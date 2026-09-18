package usecases

import (
	"fmt"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

type CompleteInput struct {
	WorkItemID  string
	PhaseID     string
	Actor       domain.Actor
	OperationID string
}

type CompleteUseCase struct {
	workItemRepo ports.WorkItemMutationRepository
	workflowRepo ports.WorkflowRepository
	artifacts    ports.ArtifactPreparer
	clock        ports.Clock
	idGenerator  ports.IDGenerator
}

func NewCompleteUseCase(
	workItemRepo ports.WorkItemMutationRepository,
	workflowRepo ports.WorkflowRepository,
	artifacts ports.ArtifactPreparer,
	clock ports.Clock,
	idGenerator ports.IDGenerator,
) *CompleteUseCase {
	return &CompleteUseCase{
		workItemRepo: workItemRepo,
		workflowRepo: workflowRepo,
		artifacts:    artifacts,
		clock:        clock,
		idGenerator:  idGenerator,
	}
}

func (uc *CompleteUseCase) Execute(baseDir string, input CompleteInput) (*domain.WorkItem, error) {
	if err := domain.ValidateActor(input.Actor); err != nil {
		return nil, err
	}
	item, workflow, err := loadWorkItemAndWorkflow(baseDir, input.WorkItemID, uc.workItemRepo, uc.workflowRepo)
	if err != nil {
		return nil, err
	}
	applied, err := operationApplied(baseDir, input.WorkItemID, input.OperationID, uc.workItemRepo)
	if err != nil {
		return nil, err
	}
	if applied {
		return item, nil
	}

	var artifacts []ports.ArtifactWrite
	var events []domain.Event

	if input.PhaseID == "" {
		if err := item.Complete(workflow); err != nil {
			return nil, err
		}
		event, err := newOperationEvent(
			input.WorkItemID,
			"work_item.completed",
			input.Actor,
			map[string]interface{}{},
			input.OperationID,
			uc.clock,
			uc.idGenerator,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to generate completion event: %w", err)
		}
		events = []domain.Event{event}
	} else {
		mutation, err := item.CompletePhase(workflow, input.PhaseID)
		if err != nil {
			return nil, err
		}
		artifacts, err = prepareArtifactsForTransitions(
			baseDir,
			item,
			workflow,
			mutation.Unblocked,
			uc.artifacts,
			uc.clock,
		)
		if err != nil {
			return nil, err
		}
		events, err = phaseMutationEvents(
			input.WorkItemID,
			mutation,
			input.Actor,
			"phase_completed",
			input.OperationID,
			uc.clock,
			uc.idGenerator,
		)
		if err != nil {
			return nil, err
		}
	}

	persisted, err := commitWorkItem(baseDir, uc.workItemRepo, item, artifacts, events, input.OperationID)
	if err != nil {
		return nil, fmt.Errorf("failed to commit completion: %w", err)
	}

	return persisted, nil
}
