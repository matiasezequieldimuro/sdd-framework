package usecases

import (
	"fmt"
	"time"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

type RejectInput struct {
	WorkItemID  string
	PhaseID     string
	RejectedBy  domain.Actor
	Comment     string
	OperationID string
}

type RejectUseCase struct {
	workItemRepo ports.WorkItemMutationRepository
	workflowRepo ports.WorkflowRepository
	clock        ports.Clock
	idGenerator  ports.IDGenerator
}

func NewRejectUseCase(
	workItemRepo ports.WorkItemMutationRepository,
	workflowRepo ports.WorkflowRepository,
	clock ports.Clock,
	idGenerator ports.IDGenerator,
) *RejectUseCase {
	return &RejectUseCase{
		workItemRepo: workItemRepo,
		workflowRepo: workflowRepo,
		clock:        clock,
		idGenerator:  idGenerator,
	}
}

func (uc *RejectUseCase) Execute(baseDir string, input RejectInput) (*domain.WorkItem, error) {
	item, workflow, err := loadWorkItemAndWorkflow(
		baseDir,
		input.WorkItemID,
		uc.workItemRepo,
		uc.workflowRepo,
	)
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

	mutation, err := item.RejectPhase(
		workflow,
		input.PhaseID,
		input.RejectedBy,
		uc.clock.Now().UTC().Format(time.RFC3339),
		input.Comment,
	)
	if err != nil {
		return nil, err
	}

	approvalEvent, err := newOperationEvent(
		input.WorkItemID,
		"approval.recorded",
		input.RejectedBy,
		map[string]interface{}{
			"phase":   input.PhaseID,
			"status":  domain.ApprovalRejected,
			"comment": input.Comment,
		},
		input.OperationID,
		uc.clock,
		uc.idGenerator,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate rejection event: %w", err)
	}

	transitionEvents, err := phaseMutationEvents(
		input.WorkItemID,
		mutation,
		input.RejectedBy,
		"approval_rejected",
		input.OperationID,
		uc.clock,
		uc.idGenerator,
	)
	if err != nil {
		return nil, err
	}
	events := append([]domain.Event{approvalEvent}, transitionEvents...)

	persisted, err := commitWorkItem(
		baseDir,
		uc.workItemRepo,
		item,
		nil,
		events,
		input.OperationID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to commit rejection: %w", err)
	}

	return persisted, nil
}
