package usecases

import (
	"errors"
	"fmt"
	"time"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

func loadWorkItemAndWorkflow(
	baseDir, workItemID string,
	workItemRepo ports.WorkItemReader,
	workflowRepo ports.WorkflowRepository,
) (*domain.WorkItem, *domain.Workflow, error) {
	item, err := workItemRepo.GetWorkItem(baseDir, workItemID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get work item: %w", err)
	}

	workflow, err := workflowRepo.GetWorkflow(baseDir, item.Workflow.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	return item, workflow, nil
}

func prepareArtifactsForTransitions(
	baseDir string,
	item *domain.WorkItem,
	workflow *domain.Workflow,
	transitions []domain.PhaseTransition,
	artifactService ports.ArtifactPreparer,
	clock ports.Clock,
) ([]ports.ArtifactWrite, error) {
	templateVars := map[string]string{
		"title":      item.Title,
		"id":         item.ID,
		"created_at": clock.Now().UTC().Format(time.RFC3339),
		"type":       item.Type,
	}

	var writes []ports.ArtifactWrite
	for _, transition := range transitions {
		if transition.To != domain.PhaseReady {
			continue
		}
		phaseWrites, err := artifactService.PrepareArtifactsForPhase(
			baseDir,
			workflow,
			transition.Phase,
			item.ID,
			templateVars,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare artifacts for phase %s: %w", transition.Phase, err)
		}
		writes = append(writes, phaseWrites...)
	}

	return writes, nil
}

func operationApplied(
	baseDir, workItemID, operationID string,
	repository ports.OperationTracker,
) (bool, error) {
	if err := domain.ValidateOperationID(operationID); err != nil {
		return false, err
	}
	if operationID == "" {
		return false, nil
	}
	applied, err := repository.OperationApplied(baseDir, workItemID, operationID)
	if err != nil {
		return false, fmt.Errorf("failed to check operation id: %w", err)
	}
	return applied, nil
}

func commitWorkItem(
	baseDir string,
	repository ports.WorkItemMutationRepository,
	item *domain.WorkItem,
	artifacts []ports.ArtifactWrite,
	events []domain.Event,
	operationID string,
) (*domain.WorkItem, error) {
	err := repository.CommitWorkItem(baseDir, ports.WorkItemCommit{
		Item:        item,
		Artifacts:   artifacts,
		Events:      events,
		OperationID: operationID,
	})
	if errors.Is(err, domain.ErrOperationAlreadyApplied) {
		persisted, loadErr := repository.GetWorkItem(baseDir, item.ID)
		if loadErr != nil {
			return nil, errors.Join(err, fmt.Errorf("failed to load idempotent result: %w", loadErr))
		}
		return persisted, nil
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func newOperationEvent(
	workItemID, eventType string,
	actor domain.Actor,
	data map[string]interface{},
	operationID string,
	clock ports.Clock,
	idGenerator ports.IDGenerator,
) (domain.Event, error) {
	id, err := idGenerator.NewID()
	if err != nil {
		return domain.Event{}, err
	}
	event := domain.NewEvent(id, clock.Now(), workItemID, eventType, actor, data)
	event.CorrelationID = operationID
	return event, nil
}

// reworkEvents constructs the ordered audit events for a rework operation:
//  1. work_item.reactivated (only when mutation.Reactivated is true)
//  2. phase.transitioned with cause rework_requested for the target phase
//  3. phase.transitioned with cause rework_superseded for each descendant, in
//     topological order (covering both superseded and re-blocked transitions)
//
// All events carry the reason from the caller (INV-06).
//
// @param workItemID Work item identifier.
// @param mutation   Result of domain.WorkItem.ReworkPhase.
// @param actor      Actor who requested the rework.
// @param reason     Free-text justification (required, non-empty).
// @param operationID Idempotency key (may be empty).
// @returns Ordered slice of events ready to be committed, or an error.
func reworkEvents(
	workItemID string,
	mutation domain.ReworkMutation,
	actor domain.Actor,
	reason, operationID string,
	clock ports.Clock,
	idGenerator ports.IDGenerator,
) ([]domain.Event, error) {
	var events []domain.Event

	if mutation.Reactivated {
		ev, err := newOperationEvent(
			workItemID,
			"work_item.reactivated",
			actor,
			map[string]interface{}{
				"from":   "completed",
				"to":     "active",
				"reason": reason,
			},
			operationID,
			clock,
			idGenerator,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, ev)
	}

	target := mutation.Target
	targetEv, err := newOperationEvent(
		workItemID,
		"phase.transitioned",
		actor,
		map[string]interface{}{
			"phase":  target.Phase,
			"from":   target.From,
			"to":     target.To,
			"cause":  "rework_requested",
			"reason": reason,
		},
		operationID,
		clock,
		idGenerator,
	)
	if err != nil {
		return nil, err
	}
	events = append(events, targetEv)

	for _, t := range mutation.Superseded {
		ev, err := newOperationEvent(
			workItemID,
			"phase.transitioned",
			actor,
			map[string]interface{}{
				"phase":  t.Phase,
				"from":   t.From,
				"to":     t.To,
				"cause":  "rework_superseded",
				"reason": reason,
			},
			operationID,
			clock,
			idGenerator,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, ev)
	}

	return events, nil
}

func phaseMutationEvents(
	workItemID string,
	mutation domain.PhaseMutation,
	actor domain.Actor,
	cause, operationID string,
	clock ports.Clock,
	idGenerator ports.IDGenerator,
) ([]domain.Event, error) {
	transitions := append([]domain.PhaseTransition{mutation.Transition}, mutation.Unblocked...)
	events := make([]domain.Event, 0, len(transitions))
	for _, transition := range transitions {
		data := map[string]interface{}{
			"phase": transition.Phase,
			"from":  transition.From,
			"to":    transition.To,
			"cause": cause,
		}
		if transition.From == domain.PhaseBlocked && transition.To == domain.PhaseReady {
			data["cause"] = "dependencies_satisfied"
		}
		event, err := newOperationEvent(
			workItemID,
			"phase.transitioned",
			actor,
			data,
			operationID,
			clock,
			idGenerator,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to generate transition event: %w", err)
		}
		events = append(events, event)
	}
	return events, nil
}
