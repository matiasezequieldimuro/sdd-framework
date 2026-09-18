package usecases

import (
	"fmt"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

type BeginPhaseInput struct {
	WorkItemID  string
	PhaseID     string
	Actor       domain.Actor
	OperationID string
}

type BeginPhaseUseCase struct {
	workItemRepo ports.WorkItemMutationRepository
	workflowRepo ports.WorkflowRepository
	clock        ports.Clock
	idGenerator  ports.IDGenerator
}

func NewBeginPhaseUseCase(
	workItemRepo ports.WorkItemMutationRepository,
	workflowRepo ports.WorkflowRepository,
	clock ports.Clock,
	idGenerator ports.IDGenerator,
) *BeginPhaseUseCase {
	return &BeginPhaseUseCase{
		workItemRepo: workItemRepo,
		workflowRepo: workflowRepo,
		clock:        clock,
		idGenerator:  idGenerator,
	}
}

func (uc *BeginPhaseUseCase) Execute(baseDir string, input BeginPhaseInput) (*domain.WorkItem, error) {
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

	mutation, err := item.BeginPhase(workflow, input.PhaseID)
	if err != nil {
		return nil, err
	}

	events, err := phaseMutationEvents(
		input.WorkItemID,
		mutation,
		input.Actor,
		"phase_begun",
		input.OperationID,
		uc.clock,
		uc.idGenerator,
	)
	if err != nil {
		return nil, err
	}

	persisted, err := commitWorkItem(baseDir, uc.workItemRepo, item, nil, events, input.OperationID)
	if err != nil {
		return nil, fmt.Errorf("failed to commit phase transition: %w", err)
	}
	return persisted, nil
}
