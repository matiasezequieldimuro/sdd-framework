package usecases

import (
	"fmt"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

type DeliverPhaseInput struct {
	WorkItemID              string
	PhaseID                 string
	RequestOptionalApproval bool
	Actor                   domain.Actor
	OperationID             string
}

type DeliverPhaseUseCase struct {
	workItemRepo ports.WorkItemMutationRepository
	workflowRepo ports.WorkflowRepository
	artifacts    ports.ArtifactPreparer
	clock        ports.Clock
	idGenerator  ports.IDGenerator
}

func NewDeliverPhaseUseCase(
	workItemRepo ports.WorkItemMutationRepository,
	workflowRepo ports.WorkflowRepository,
	artifacts ports.ArtifactPreparer,
	clock ports.Clock,
	idGenerator ports.IDGenerator,
) *DeliverPhaseUseCase {
	return &DeliverPhaseUseCase{
		workItemRepo: workItemRepo,
		workflowRepo: workflowRepo,
		artifacts:    artifacts,
		clock:        clock,
		idGenerator:  idGenerator,
	}
}

func (uc *DeliverPhaseUseCase) Execute(baseDir string, input DeliverPhaseInput) (*domain.WorkItem, error) {
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

	mutation, err := item.DeliverPhase(workflow, input.PhaseID, input.RequestOptionalApproval)
	if err != nil {
		return nil, err
	}
	artifacts, err := prepareArtifactsForTransitions(
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

	events, err := phaseMutationEvents(
		input.WorkItemID,
		mutation,
		input.Actor,
		"phase_delivered",
		input.OperationID,
		uc.clock,
		uc.idGenerator,
	)
	if err != nil {
		return nil, err
	}

	if mutation.Transition.To == domain.PhaseAwaitingApproval {
		event, err := newOperationEvent(input.WorkItemID, "approval.requested", input.Actor, map[string]interface{}{
			"phase": input.PhaseID,
		}, input.OperationID, uc.clock, uc.idGenerator)
		if err != nil {
			return nil, fmt.Errorf("failed to generate approval event: %w", err)
		}
		events = append(events, event)
	}

	persisted, err := commitWorkItem(baseDir, uc.workItemRepo, item, artifacts, events, input.OperationID)
	if err != nil {
		return nil, fmt.Errorf("failed to commit phase delivery: %w", err)
	}
	return persisted, nil
}
