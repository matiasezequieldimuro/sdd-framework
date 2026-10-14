package usecases

import (
	"fmt"
	"strings"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

// ReworkInput carries all caller-supplied parameters for the rework operation.
type ReworkInput struct {
	WorkItemID  string
	PhaseID     string
	Reason      string
	Actor       domain.Actor
	OperationID string
}

// ReworkResult is the output of a successful rework operation, bundling the
// updated work item together with the number of superseded phases and whether
// the work item was reactivated from completed to active.
type ReworkResult struct {
	WorkItem    *domain.WorkItem `json:"work_item"`
	Superseded  int              `json:"superseded"`
	Reactivated bool             `json:"reactivated"`
}

// ReworkUseCase orchestrates the rework command: validating preconditions,
// applying the domain mutation, generating audit events, and committing the
// result transactionally.
type ReworkUseCase struct {
	workItemRepo ports.WorkItemMutationRepository
	workflowRepo ports.WorkflowRepository
	clock        ports.Clock
	idGenerator  ports.IDGenerator
}

// NewReworkUseCase constructs a ReworkUseCase with its required dependencies.
//
// @param workItemRepo Repository for reading and committing work items.
// @param workflowRepo Repository for loading workflow definitions.
// @param clock        Wall-clock provider for event timestamps.
// @param idGenerator  UUID generator for event identifiers.
func NewReworkUseCase(
	workItemRepo ports.WorkItemMutationRepository,
	workflowRepo ports.WorkflowRepository,
	clock ports.Clock,
	idGenerator ports.IDGenerator,
) *ReworkUseCase {
	return &ReworkUseCase{
		workItemRepo: workItemRepo,
		workflowRepo: workflowRepo,
		clock:        clock,
		idGenerator:  idGenerator,
	}
}

// Execute runs the rework use case for the given input. It follows the algorithm
// defined in the specification (§rework) and the plan (Etapa 3A):
//
//  1. Validate actor, reason, and operation id.
//  2. Load work item and workflow (PC-RW-01).
//  3. Check idempotency: if operation already applied, return current state.
//  4. Apply domain.WorkItem.ReworkPhase (PC-RW-02 to PC-RW-05).
//  5. Build audit events via reworkEvents.
//  6. Commit transactionally via commitWorkItem.
//
// @param baseDir Absolute path to the project root.
// @param input   Caller-supplied parameters for the rework.
// @returns       The updated work item and rework summary, or an error.
func (uc *ReworkUseCase) Execute(baseDir string, input ReworkInput) (*ReworkResult, error) {
	if err := domain.ValidateActor(input.Actor); err != nil {
		return nil, err
	}
	// Defense-in-depth: Cobra marks --reason as required (ESC-RW-06), but the
	// use case also validates it to ensure no caller can bypass the flag.
	if strings.TrimSpace(input.Reason) == "" {
		return nil, fmt.Errorf("%w: --reason is required and cannot be empty", domain.ErrInvalidWorkItem)
	}
	if err := domain.ValidateOperationID(input.OperationID); err != nil {
		return nil, err
	}

	item, workflow, err := loadWorkItemAndWorkflow(baseDir, input.WorkItemID, uc.workItemRepo, uc.workflowRepo)
	if err != nil {
		return nil, err
	}

	// Idempotency guard (CA-RW-12 / ESC-RW-10).
	applied, err := operationApplied(baseDir, input.WorkItemID, input.OperationID, uc.workItemRepo)
	if err != nil {
		return nil, err
	}
	if applied {
		// Re-read the persisted item so the caller sees the post-rework state.
		persisted, loadErr := uc.workItemRepo.GetWorkItem(baseDir, input.WorkItemID)
		if loadErr != nil {
			return nil, loadErr
		}
		return &ReworkResult{WorkItem: persisted}, nil
	}

	mutation, err := item.ReworkPhase(workflow, input.PhaseID)
	if err != nil {
		return nil, err
	}

	events, err := reworkEvents(
		input.WorkItemID,
		mutation,
		input.Actor,
		input.Reason,
		input.OperationID,
		uc.clock,
		uc.idGenerator,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate rework events: %w", err)
	}

	// Rework never writes artifact files (AS-01: artifacts are not modified).
	persisted, err := commitWorkItem(baseDir, uc.workItemRepo, item, nil, events, input.OperationID)
	if err != nil {
		return nil, fmt.Errorf("failed to commit rework: %w", err)
	}

	return &ReworkResult{
		WorkItem:    persisted,
		Superseded:  len(mutation.Superseded),
		Reactivated: mutation.Reactivated,
	}, nil
}
