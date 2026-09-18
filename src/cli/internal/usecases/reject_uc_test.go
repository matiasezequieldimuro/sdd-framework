package usecases_test

import (
	"errors"
	"testing"
	"time"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/usecases"
)

func TestRejectUseCaseCommitsDecisionAndTransition(t *testing.T) {
	workflow := memoryWorkflow()
	workflow.Phases[0].Approval = domain.ApprovalRequired
	item := &domain.WorkItem{
		ID:       "memory-item",
		Status:   domain.WorkItemActive,
		Workflow: domain.WorkItemWorkflow{ID: workflow.ID},
		Phases: map[string]domain.PhaseState{
			"first": {Status: domain.PhaseAwaitingApproval},
		},
		Approvals: []domain.Approval{{
			Phase:  "first",
			Status: domain.ApprovalPending,
		}},
	}
	repository := &memoryWorkItemRepository{item: item}
	clock := fixedClock{value: time.Date(2026, time.August, 19, 22, 0, 0, 0, time.UTC)}
	ids := &sequenceIDGenerator{}
	actor := domain.Actor{Kind: domain.ActorHuman, ID: "matias"}

	result, err := usecases.NewRejectUseCase(
		repository,
		staticWorkflowRepository{workflow: workflow},
		clock,
		ids,
	).Execute("unused", usecases.RejectInput{
		WorkItemID:  item.ID,
		PhaseID:     "first",
		RejectedBy:  actor,
		Comment:     "Needs changes",
		OperationID: "test:reject:first",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Phases["first"].Status != domain.PhaseRejected {
		t.Fatalf("phase status = %s, want %s", result.Phases["first"].Status, domain.PhaseRejected)
	}
	if repository.commits != 1 {
		t.Fatalf("commits = %d, want 1", repository.commits)
	}
	if len(repository.artifacts) != 0 {
		t.Fatalf("artifacts = %#v, want none", repository.artifacts)
	}
	if len(repository.events) != 2 {
		t.Fatalf("events = %#v, want two", repository.events)
	}
	approvalEvent := repository.events[0]
	transitionEvent := repository.events[1]
	if approvalEvent.Type != "approval.recorded" ||
		approvalEvent.Actor != actor ||
		approvalEvent.CorrelationID != "test:reject:first" ||
		approvalEvent.Data["phase"] != "first" ||
		approvalEvent.Data["status"] != domain.ApprovalRejected ||
		approvalEvent.Data["comment"] != "Needs changes" {
		t.Fatalf("approval event = %#v", approvalEvent)
	}
	if transitionEvent.Type != "phase.transitioned" ||
		transitionEvent.Actor != actor ||
		transitionEvent.CorrelationID != "test:reject:first" ||
		transitionEvent.Data["phase"] != "first" ||
		transitionEvent.Data["from"] != domain.PhaseAwaitingApproval ||
		transitionEvent.Data["to"] != domain.PhaseRejected ||
		transitionEvent.Data["cause"] != "approval_rejected" {
		t.Fatalf("transition event = %#v", transitionEvent)
	}
	if approvalEvent.ID == transitionEvent.ID {
		t.Fatalf("event IDs are not unique: %s", approvalEvent.ID)
	}
	approval := result.Approvals[0]
	if approval.Status != domain.ApprovalRejected ||
		approval.By == nil ||
		*approval.By != actor ||
		approval.At != "2026-08-19T22:00:00Z" ||
		approval.Comment != "Needs changes" {
		t.Fatalf("approval = %#v", approval)
	}
}

func TestRejectUseCaseReturnsAppliedOperationWithoutCommit(t *testing.T) {
	workflow := memoryWorkflow()
	workflow.Phases[0].Approval = domain.ApprovalRequired
	item := &domain.WorkItem{
		ID:       "memory-item",
		Status:   domain.WorkItemActive,
		Workflow: domain.WorkItemWorkflow{ID: workflow.ID},
		Phases: map[string]domain.PhaseState{
			"first": {Status: domain.PhaseRejected},
		},
	}
	repository := &memoryWorkItemRepository{
		item:             item,
		operationApplied: true,
	}
	ids := &sequenceIDGenerator{}

	result, err := usecases.NewRejectUseCase(
		repository,
		staticWorkflowRepository{workflow: workflow},
		fixedClock{value: time.Date(2026, time.August, 19, 22, 0, 0, 0, time.UTC)},
		ids,
	).Execute("unused", usecases.RejectInput{
		WorkItemID:  item.ID,
		PhaseID:     "first",
		RejectedBy:  domain.Actor{Kind: domain.ActorHuman, ID: "matias"},
		OperationID: "test:reject:first",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result != item {
		t.Fatalf("result = %#v, want existing item", result)
	}
	if repository.commits != 0 || len(repository.events) != 0 || ids.next != 0 {
		t.Fatalf(
			"commits = %d, events = %d, generated IDs = %d",
			repository.commits,
			len(repository.events),
			ids.next,
		)
	}
}

func TestRejectUseCasePropagatesAdapterFailures(t *testing.T) {
	adapterErr := errors.New("adapter failure")
	workflow := memoryWorkflow()
	workflow.Phases[0].Approval = domain.ApprovalRequired
	newItem := func() *domain.WorkItem {
		return &domain.WorkItem{
			ID:       "memory-item",
			Status:   domain.WorkItemActive,
			Workflow: domain.WorkItemWorkflow{ID: workflow.ID},
			Phases: map[string]domain.PhaseState{
				"first": {Status: domain.PhaseAwaitingApproval},
			},
			Approvals: []domain.Approval{{Phase: "first", Status: domain.ApprovalPending}},
		}
	}
	tests := []struct {
		name       string
		repository *memoryWorkItemRepository
		workflows  staticWorkflowRepository
		ids        *sequenceIDGenerator
	}{
		{
			name:       "work item loading",
			repository: &memoryWorkItemRepository{getErr: adapterErr},
			workflows:  staticWorkflowRepository{workflow: workflow},
			ids:        &sequenceIDGenerator{},
		},
		{
			name:       "workflow loading",
			repository: &memoryWorkItemRepository{item: newItem()},
			workflows:  staticWorkflowRepository{err: adapterErr},
			ids:        &sequenceIDGenerator{},
		},
		{
			name:       "idempotency lookup",
			repository: &memoryWorkItemRepository{item: newItem(), operationErr: adapterErr},
			workflows:  staticWorkflowRepository{workflow: workflow},
			ids:        &sequenceIDGenerator{},
		},
		{
			name:       "event id generation",
			repository: &memoryWorkItemRepository{item: newItem()},
			workflows:  staticWorkflowRepository{workflow: workflow},
			ids:        &sequenceIDGenerator{err: adapterErr},
		},
		{
			name:       "work item commit",
			repository: &memoryWorkItemRepository{item: newItem(), commitErr: adapterErr},
			workflows:  staticWorkflowRepository{workflow: workflow},
			ids:        &sequenceIDGenerator{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := usecases.NewRejectUseCase(
				tt.repository,
				tt.workflows,
				fixedClock{value: time.Date(2026, time.August, 19, 22, 0, 0, 0, time.UTC)},
				tt.ids,
			).Execute("unused", usecases.RejectInput{
				WorkItemID:  "memory-item",
				PhaseID:     "first",
				RejectedBy:  domain.Actor{Kind: domain.ActorHuman, ID: "matias"},
				OperationID: "test:reject:first",
			})
			if !errors.Is(err, adapterErr) {
				t.Fatalf("Execute() error = %v, want %v", err, adapterErr)
			}
			if result != nil {
				t.Fatalf("Execute() result = %#v, want nil", result)
			}
			if tt.repository.commits != 0 {
				t.Fatalf("commits = %d, want 0", tt.repository.commits)
			}
		})
	}
}
