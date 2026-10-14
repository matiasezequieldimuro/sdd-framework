package usecases

import (
	"errors"
	"fmt"
	"strings"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

// CancelInput carries all caller-supplied parameters for the cancel operation.
type CancelInput struct {
	WorkItemID  string
	Reason      string
	Actor       domain.Actor
	OperationID string
}

// CancelResult is the output of a successful cancel operation.
type CancelResult struct {
	WorkItem *domain.WorkItem       `json:"work_item"`
	Location ports.WorkItemLocation `json:"location"`
}

// CancelUseCase orchestrates the cancel command: validating preconditions,
// applying the domain mutation, generating the audit event, and performing
// the transactional move to cancelled/.
type CancelUseCase struct {
	activeItems ports.WorkItemReader
	canceller   ports.WorkItemCanceller
	clock       ports.Clock
	ids         ports.IDGenerator
}

// NewCancelUseCase constructs a CancelUseCase with its required dependencies.
//
// @param activeItems Reader for loading the active work item (PC-CA-01).
// @param canceller   Repository for the transactional cancel operation.
// @param clock       Wall-clock provider for event timestamps.
// @param ids         UUID generator for event identifiers.
func NewCancelUseCase(
	activeItems ports.WorkItemReader,
	canceller ports.WorkItemCanceller,
	clock ports.Clock,
	ids ports.IDGenerator,
) *CancelUseCase {
	return &CancelUseCase{
		activeItems: activeItems,
		canceller:   canceller,
		clock:       clock,
		ids:         ids,
	}
}

// Execute runs the cancel use case for the given input. It follows the algorithm
// defined in the specification (§cancel) and the plan (Etapa 3B.1):
//
//  1. Validate identifier, actor, operation id, and reason.
//  2. Check idempotency via the cancelled/ repository.
//  3. Load the active work item (PC-CA-01).
//  4. Apply domain.WorkItem.Cancel (PC-CA-02).
//  5. Build the work_item.cancelled audit event.
//  6. Commit the transactional move via canceller.CancelWorkItem.
//
// @param baseDir Absolute path to the project root.
// @param input   Caller-supplied parameters for the cancel.
// @returns       The cancelled work item and its new location, or an error.
func (uc *CancelUseCase) Execute(baseDir string, input CancelInput) (*CancelResult, error) {
	if err := domain.ValidateIdentifier("work item id", input.WorkItemID); err != nil {
		return nil, err
	}
	if err := domain.ValidateActor(input.Actor); err != nil {
		return nil, err
	}
	if err := domain.ValidateOperationID(input.OperationID); err != nil {
		return nil, err
	}
	// Defense-in-depth: Cobra marks --reason as required (ESC-CA-04), but the
	// use case validates it too to guard against programmatic callers.
	if strings.TrimSpace(input.Reason) == "" {
		return nil, fmt.Errorf("%w: --reason is required and cannot be empty", domain.ErrInvalidWorkItem)
	}

	// Idempotency: if the item is already in cancelled/ and the operation was
	// applied, return the existing result without re-applying (CA-CA-10).
	cancelled, err := uc.canceller.FindCancelledWorkItem(baseDir, input.WorkItemID)
	if err == nil {
		// Work item is already in cancelled/.
		if input.OperationID != "" {
			applied, checkErr := uc.canceller.CancelledOperationApplied(
				baseDir,
				input.WorkItemID,
				input.OperationID,
			)
			if checkErr != nil {
				return nil, fmt.Errorf("failed to check cancelled operation: %w", checkErr)
			}
			if applied {
				return &CancelResult{
					WorkItem: cancelled.Item,
					Location: cancelled.Location,
				}, nil
			}
		}
		// Already cancelled but operation ID differs — conflict (PC-CA-02).
		return nil, fmt.Errorf(
			"%w: work item '%s' is already cancelled",
			domain.ErrWorkItemCannotCancel,
			input.WorkItemID,
		)
	}
	if !errors.Is(err, domain.ErrWorkItemNotFound) {
		return nil, err
	}

	// Load the active work item (PC-CA-01).
	item, err := uc.activeItems.GetWorkItem(baseDir, input.WorkItemID)
	if err != nil {
		return nil, err
	}

	// Apply domain mutation: validates PC-CA-02 (status must be active).
	if err := item.Cancel(); err != nil {
		return nil, err
	}

	eventID, err := uc.ids.NewID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate cancel event id: %w", err)
	}
	event := domain.NewEvent(
		eventID,
		uc.clock.Now(),
		input.WorkItemID,
		"work_item.cancelled",
		input.Actor,
		map[string]interface{}{
			"from":   string(domain.WorkItemActive),
			"to":     string(domain.WorkItemCancelled),
			"reason": input.Reason,
		},
	)
	event.CorrelationID = input.OperationID

	record, err := uc.canceller.CancelWorkItem(baseDir, ports.WorkItemCancelCommit{
		Item:        item,
		Event:       event,
		OperationID: input.OperationID,
	})
	if errors.Is(err, domain.ErrOperationAlreadyApplied) {
		// Race or retry: the cancel was applied between our check and the commit.
		record, err = uc.canceller.FindCancelledWorkItem(baseDir, input.WorkItemID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to cancel work item: %w", err)
	}

	return &CancelResult{
		WorkItem: record.Item,
		Location: record.Location,
	}, nil
}
