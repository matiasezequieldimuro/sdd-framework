package usecases

import (
	"fmt"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

type NextAction struct {
	PhaseID       string `json:"phase_id" yaml:"phase_id"`
	Status        string `json:"status" yaml:"status"`
	Procedure     string `json:"procedure,omitempty" yaml:"procedure,omitempty"`
	Artifact      string `json:"artifact,omitempty" yaml:"artifact,omitempty"`
	NeedsApproval bool   `json:"needs_approval" yaml:"needs_approval"`
	Optional      bool   `json:"optional" yaml:"optional"`
	Message       string `json:"message" yaml:"message"`
}

type NextUseCase struct {
	workItemRepo ports.WorkItemReader
	workflowRepo ports.WorkflowRepository
}

func NewNextUseCase(wiRepo ports.WorkItemReader, wfRepo ports.WorkflowRepository) *NextUseCase {
	return &NextUseCase{
		workItemRepo: wiRepo,
		workflowRepo: wfRepo,
	}
}

func (uc *NextUseCase) Execute(baseDir, id string) (*NextAction, error) {
	item, err := uc.workItemRepo.GetWorkItem(baseDir, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get work item: %w", err)
	}

	wf, err := uc.workflowRepo.GetWorkflow(baseDir, item.Workflow.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	next, err := item.NextPhase(wf)
	if err != nil {
		return nil, err
	}
	if next == nil {
		return &NextAction{
			Message: "No active phases pending. Work item may be completed or archived.",
		}, nil
	}

	message := fmt.Sprintf("Next active phase is '%s' (%s). Follow procedure '%s'.", next.Definition.ID, next.State.Status, next.Definition.Procedure)
	if next.State.Status == domain.PhaseAwaitingApproval {
		message = fmt.Sprintf("Phase '%s' is awaiting human approval before proceeding.", next.Definition.ID)
	}

	return &NextAction{
		PhaseID:       next.Definition.ID,
		Status:        string(next.State.Status),
		Procedure:     next.Definition.Procedure,
		Artifact:      next.State.Artifact,
		NeedsApproval: next.Definition.Approval == domain.ApprovalRequired || next.State.Status == domain.PhaseAwaitingApproval,
		Optional:      next.Definition.Optional,
		Message:       message,
	}, nil
}
