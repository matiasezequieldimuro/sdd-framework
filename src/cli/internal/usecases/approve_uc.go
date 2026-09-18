package usecases

import (
	"fmt"
	"time"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

type ApproveInput struct {
	WorkItemID  string
	PhaseID     string
	ApprovedBy  domain.Actor
	Comment     string
	OperationID string
}

type ApproveUseCase struct {
	workItemRepo ports.WorkItemMutationRepository
	workflowRepo ports.WorkflowRepository
	artifacts    ports.ArtifactPreparer
	clock        ports.Clock
	idGenerator  ports.IDGenerator
}

func NewApproveUseCase(
	wiRepo ports.WorkItemMutationRepository,
	wfRepo ports.WorkflowRepository,
	artifacts ports.ArtifactPreparer,
	clock ports.Clock,
	idGenerator ports.IDGenerator,
) *ApproveUseCase {
	return &ApproveUseCase{
		workItemRepo: wiRepo,
		workflowRepo: wfRepo,
		artifacts:    artifacts,
		clock:        clock,
		idGenerator:  idGenerator,
	}
}

func (uc *ApproveUseCase) Execute(baseDir string, in ApproveInput) (*domain.WorkItem, error) {
	item, err := uc.workItemRepo.GetWorkItem(baseDir, in.WorkItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to find work item: %w", err)
	}

	wf, err := uc.workflowRepo.GetWorkflow(baseDir, item.Workflow.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}
	applied, err := operationApplied(baseDir, in.WorkItemID, in.OperationID, uc.workItemRepo)
	if err != nil {
		return nil, err
	}
	if applied {
		return item, nil
	}

	mutation, err := item.ApprovePhase(
		wf,
		in.PhaseID,
		in.ApprovedBy,
		uc.clock.Now().UTC().Format(time.RFC3339),
		in.Comment,
	)
	if err != nil {
		return nil, err
	}

	artifacts, err := prepareArtifactsForTransitions(
		baseDir,
		item,
		wf,
		mutation.Unblocked,
		uc.artifacts,
		uc.clock,
	)
	if err != nil {
		return nil, err
	}

	approvalEvent, err := newOperationEvent(in.WorkItemID, "approval.recorded", in.ApprovedBy, map[string]interface{}{
		"phase":   in.PhaseID,
		"status":  domain.ApprovalApproved,
		"comment": in.Comment,
	}, in.OperationID, uc.clock, uc.idGenerator)
	if err != nil {
		return nil, fmt.Errorf("failed to generate approval event: %w", err)
	}
	transitionEvents, err := phaseMutationEvents(
		in.WorkItemID,
		mutation,
		in.ApprovedBy,
		"approval_recorded",
		in.OperationID,
		uc.clock,
		uc.idGenerator,
	)
	if err != nil {
		return nil, err
	}
	events := append([]domain.Event{approvalEvent}, transitionEvents...)

	persisted, err := commitWorkItem(baseDir, uc.workItemRepo, item, artifacts, events, in.OperationID)
	if err != nil {
		return nil, fmt.Errorf("failed to commit approval: %w", err)
	}
	return persisted, nil
}
