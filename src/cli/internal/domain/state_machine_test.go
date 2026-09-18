package domain_test

import (
	"errors"
	"testing"

	"sdd-cli/internal/domain"
)

func TestDeliverPhaseAppliesApprovalPolicy(t *testing.T) {
	tests := []struct {
		name            string
		policy          domain.ApprovalPolicy
		requestApproval bool
		wantStatus      domain.PhaseStatus
		wantErr         error
	}{
		{name: "required requests approval", policy: domain.ApprovalRequired, wantStatus: domain.PhaseAwaitingApproval},
		{name: "optional completes by default", policy: domain.ApprovalOptional, wantStatus: domain.PhaseCompleted},
		{name: "optional can request approval", policy: domain.ApprovalOptional, requestApproval: true, wantStatus: domain.PhaseAwaitingApproval},
		{name: "none completes", policy: domain.ApprovalNone, wantStatus: domain.PhaseCompleted},
		{name: "none rejects approval request", policy: domain.ApprovalNone, requestApproval: true, wantErr: domain.ErrApprovalNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow := workflowWithPhase("phase", tt.policy, false)
			item := workItemWithPhase("phase", domain.PhaseInProgress)

			_, err := item.DeliverPhase(workflow, "phase", tt.requestApproval)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("DeliverPhase() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if got := item.Phases["phase"].Status; got != tt.wantStatus {
				t.Fatalf("DeliverPhase() status = %s, want %s", got, tt.wantStatus)
			}
		})
	}
}

func TestBeginPhaseTransitionMatrix(t *testing.T) {
	tests := []struct {
		status  domain.PhaseStatus
		wantErr bool
	}{
		{status: domain.PhaseReady},
		{status: domain.PhaseRejected},
		{status: domain.PhaseSuperseded},
		{status: domain.PhaseBlocked, wantErr: true},
		{status: domain.PhaseInProgress, wantErr: true},
		{status: domain.PhaseAwaitingApproval, wantErr: true},
		{status: domain.PhaseApproved, wantErr: true},
		{status: domain.PhaseCompleted, wantErr: true},
		{status: domain.PhaseAccepted, wantErr: true},
		{status: domain.PhaseNotApplicable, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			workflow := workflowWithPhase("phase", domain.ApprovalNone, false)
			item := workItemWithPhase("phase", tt.status)

			_, err := item.BeginPhase(workflow, "phase")
			if tt.wantErr {
				if !errors.Is(err, domain.ErrInvalidTransition) {
					t.Fatalf("BeginPhase() error = %v, want %v", err, domain.ErrInvalidTransition)
				}
				return
			}
			if err != nil {
				t.Fatalf("BeginPhase() error = %v", err)
			}
			if got := item.Phases["phase"].Status; got != domain.PhaseInProgress {
				t.Fatalf("BeginPhase() status = %s, want %s", got, domain.PhaseInProgress)
			}
		})
	}
}

func TestCompletePhaseTransitionMatrix(t *testing.T) {
	tests := []struct {
		status  domain.PhaseStatus
		wantErr bool
	}{
		{status: domain.PhaseApproved},
		{status: domain.PhaseAccepted},
		{status: domain.PhaseBlocked, wantErr: true},
		{status: domain.PhaseReady, wantErr: true},
		{status: domain.PhaseInProgress, wantErr: true},
		{status: domain.PhaseAwaitingApproval, wantErr: true},
		{status: domain.PhaseCompleted, wantErr: true},
		{status: domain.PhaseRejected, wantErr: true},
		{status: domain.PhaseSuperseded, wantErr: true},
		{status: domain.PhaseNotApplicable, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			workflow := workflowWithPhase("phase", domain.ApprovalRequired, false)
			item := workItemWithPhase("phase", tt.status)

			_, err := item.CompletePhase(workflow, "phase")
			if tt.wantErr {
				if !errors.Is(err, domain.ErrInvalidTransition) {
					t.Fatalf("CompletePhase() error = %v, want %v", err, domain.ErrInvalidTransition)
				}
				return
			}
			if err != nil {
				t.Fatalf("CompletePhase() error = %v", err)
			}
			if got := item.Phases["phase"].Status; got != domain.PhaseCompleted {
				t.Fatalf("CompletePhase() status = %s, want %s", got, domain.PhaseCompleted)
			}
		})
	}
}

func TestApprovePhaseRequiresHumanAndPendingApproval(t *testing.T) {
	workflow := workflowWithPhase("phase", domain.ApprovalRequired, false)

	t.Run("agent cannot approve", func(t *testing.T) {
		item := workItemWithPhase("phase", domain.PhaseAwaitingApproval)
		_, err := item.ApprovePhase(workflow, "phase", domain.Actor{Kind: domain.ActorAgent, ID: "copilot"}, "2026-08-18T00:00:00Z", "")
		if !errors.Is(err, domain.ErrHumanActorRequired) {
			t.Fatalf("ApprovePhase() error = %v, want %v", err, domain.ErrHumanActorRequired)
		}
	})

	t.Run("in progress cannot be approved", func(t *testing.T) {
		item := workItemWithPhase("phase", domain.PhaseInProgress)
		_, err := item.ApprovePhase(workflow, "phase", domain.Actor{Kind: domain.ActorHuman, ID: "matias"}, "2026-08-18T00:00:00Z", "")
		if !errors.Is(err, domain.ErrPhaseNotAwaitingApproval) {
			t.Fatalf("ApprovePhase() error = %v, want %v", err, domain.ErrPhaseNotAwaitingApproval)
		}
	})
}

func TestRejectPhaseRequiresHumanAndPendingApproval(t *testing.T) {
	workflow := workflowWithPhase("phase", domain.ApprovalRequired, false)

	t.Run("agent cannot reject", func(t *testing.T) {
		item := workItemWithPhase("phase", domain.PhaseAwaitingApproval)
		item.Approvals = []domain.Approval{{Phase: "phase", Status: domain.ApprovalPending}}

		_, err := item.RejectPhase(
			workflow,
			"phase",
			domain.Actor{Kind: domain.ActorAgent, ID: "copilot"},
			"2026-08-19T22:00:00Z",
			"needs changes",
		)
		if !errors.Is(err, domain.ErrHumanActorRequired) {
			t.Fatalf("RejectPhase() error = %v, want %v", err, domain.ErrHumanActorRequired)
		}
		if got := item.Phases["phase"].Status; got != domain.PhaseAwaitingApproval {
			t.Fatalf("phase status = %s, want %s", got, domain.PhaseAwaitingApproval)
		}
		if got := item.Approvals[0].Status; got != domain.ApprovalPending {
			t.Fatalf("approval status = %s, want %s", got, domain.ApprovalPending)
		}
	})

	t.Run("in progress cannot be rejected", func(t *testing.T) {
		item := workItemWithPhase("phase", domain.PhaseInProgress)
		_, err := item.RejectPhase(
			workflow,
			"phase",
			domain.Actor{Kind: domain.ActorHuman, ID: "matias"},
			"2026-08-19T22:00:00Z",
			"",
		)
		if !errors.Is(err, domain.ErrPhaseNotAwaitingApproval) {
			t.Fatalf("RejectPhase() error = %v, want %v", err, domain.ErrPhaseNotAwaitingApproval)
		}
	})

	t.Run("phase without approval cannot be rejected", func(t *testing.T) {
		item := workItemWithPhase("phase", domain.PhaseAwaitingApproval)
		_, err := item.RejectPhase(
			workflowWithPhase("phase", domain.ApprovalNone, false),
			"phase",
			domain.Actor{Kind: domain.ActorHuman, ID: "matias"},
			"2026-08-19T22:00:00Z",
			"",
		)
		if !errors.Is(err, domain.ErrApprovalNotAllowed) {
			t.Fatalf("RejectPhase() error = %v, want %v", err, domain.ErrApprovalNotAllowed)
		}
	})
}

func TestRejectPhaseRecordsDecision(t *testing.T) {
	workflow := workflowWithPhase("plan", domain.ApprovalRequired, false)
	item := workItemWithPhase("plan", domain.PhaseAwaitingApproval)
	item.Approvals = []domain.Approval{{Phase: "plan", Status: domain.ApprovalPending}}
	actor := domain.Actor{Kind: domain.ActorHuman, ID: "matias"}

	mutation, err := item.RejectPhase(
		workflow,
		"plan",
		actor,
		"2026-08-19T22:00:00Z",
		"Adjust rollback steps",
	)
	if err != nil {
		t.Fatalf("RejectPhase() error = %v", err)
	}
	if mutation.Transition.Phase != "plan" ||
		mutation.Transition.From != domain.PhaseAwaitingApproval ||
		mutation.Transition.To != domain.PhaseRejected {
		t.Fatalf("transition = %#v", mutation.Transition)
	}
	if len(mutation.Unblocked) != 0 {
		t.Fatalf("unblocked transitions = %#v, want none", mutation.Unblocked)
	}
	if got := item.Phases["plan"].Status; got != domain.PhaseRejected {
		t.Fatalf("phase status = %s, want %s", got, domain.PhaseRejected)
	}
	approval := item.Approvals[0]
	if approval.Status != domain.ApprovalRejected ||
		approval.By == nil ||
		*approval.By != actor ||
		approval.At != "2026-08-19T22:00:00Z" ||
		approval.Comment != "Adjust rollback steps" {
		t.Fatalf("approval = %#v", approval)
	}
}

func TestRejectedPhasePreservesApprovalHistoryAcrossRework(t *testing.T) {
	workflow := workflowWithPhase("plan", domain.ApprovalRequired, false)
	item := workItemWithPhase("plan", domain.PhaseAwaitingApproval)
	item.Approvals = []domain.Approval{{Phase: "plan", Status: domain.ApprovalPending}}
	actor := domain.Actor{Kind: domain.ActorHuman, ID: "matias"}

	if _, err := item.RejectPhase(workflow, "plan", actor, "2026-08-19T22:00:00Z", "Revise"); err != nil {
		t.Fatalf("RejectPhase() error = %v", err)
	}
	if _, err := item.BeginPhase(workflow, "plan"); err != nil {
		t.Fatalf("BeginPhase() error = %v", err)
	}
	if _, err := item.DeliverPhase(workflow, "plan", false); err != nil {
		t.Fatalf("DeliverPhase() error = %v", err)
	}

	if len(item.Approvals) != 2 {
		t.Fatalf("approvals = %#v, want two iterations", item.Approvals)
	}
	if item.Approvals[0].Status != domain.ApprovalRejected ||
		item.Approvals[1].Status != domain.ApprovalPending {
		t.Fatalf("approval history = %#v", item.Approvals)
	}
}

func TestAcceptExternalPhaseRequiresConfiguredReview(t *testing.T) {
	workflow := workflowWithPhase("phase", domain.ApprovalRequired, false)
	item := workItemWithPhase("phase", domain.PhaseBlocked)

	if _, err := item.AcceptExternalPhase(workflow, "phase"); err != nil {
		t.Fatalf("AcceptExternalPhase() error = %v", err)
	}
	if got := item.Phases["phase"].Status; got != domain.PhaseAwaitingApproval {
		t.Fatalf("phase status = %s, want %s", got, domain.PhaseAwaitingApproval)
	}
	if len(item.Approvals) != 1 || item.Approvals[0].Status != domain.ApprovalPending {
		t.Fatalf("pending approval was not recorded: %#v", item.Approvals)
	}
}

func TestRejectedPhaseCanBeginRework(t *testing.T) {
	workflow := workflowWithPhase("phase", domain.ApprovalRequired, false)
	item := workItemWithPhase("phase", domain.PhaseAwaitingApproval)
	item.Approvals = []domain.Approval{{Phase: "phase", Status: domain.ApprovalPending}}
	human := domain.Actor{Kind: domain.ActorHuman, ID: "matias"}

	if _, err := item.RejectPhase(workflow, "phase", human, "2026-08-18T00:00:00Z", "needs changes"); err != nil {
		t.Fatalf("RejectPhase() error = %v", err)
	}
	if _, err := item.BeginPhase(workflow, "phase"); err != nil {
		t.Fatalf("BeginPhase() after rejection error = %v", err)
	}
	if got := item.Phases["phase"].Status; got != domain.PhaseInProgress {
		t.Fatalf("phase status = %s, want %s", got, domain.PhaseInProgress)
	}
}

func TestApprovedPhaseUnlocksDependents(t *testing.T) {
	workflow := &domain.Workflow{
		Phases: []domain.WorkflowPhase{
			{ID: "plan", Approval: domain.ApprovalRequired},
			{ID: "implementation", Requires: []string{"plan"}, Approval: domain.ApprovalNone, Produces: []string{"report"}},
		},
		Artifacts: map[string]domain.WorkflowArtifact{
			"report": {Path: "artifacts/implementation-report.md"},
		},
	}
	item := &domain.WorkItem{
		Status: domain.WorkItemActive,
		Phases: map[string]domain.PhaseState{
			"plan":           {Status: domain.PhaseAwaitingApproval},
			"implementation": {Status: domain.PhaseBlocked},
		},
		Approvals: []domain.Approval{{Phase: "plan", Status: domain.ApprovalPending}},
	}

	mutation, err := item.ApprovePhase(
		workflow,
		"plan",
		domain.Actor{Kind: domain.ActorHuman, ID: "matias"},
		"2026-08-18T00:00:00Z",
		"",
	)
	if err != nil {
		t.Fatalf("ApprovePhase() error = %v", err)
	}
	if len(mutation.Unblocked) != 1 {
		t.Fatalf("unblocked transitions = %d, want 1", len(mutation.Unblocked))
	}
	state := item.Phases["implementation"]
	if state.Status != domain.PhaseReady {
		t.Fatalf("implementation status = %s, want %s", state.Status, domain.PhaseReady)
	}
	if state.Artifact != "artifacts/implementation-report.md" {
		t.Fatalf("implementation artifact = %q", state.Artifact)
	}
}

func TestCompleteWorkItemAllowsOmittedOptionalPhase(t *testing.T) {
	workflow := &domain.Workflow{
		Phases: []domain.WorkflowPhase{
			{ID: "review", Approval: domain.ApprovalRequired},
			{ID: "archive", Approval: domain.ApprovalNone, Optional: true},
		},
	}
	item := &domain.WorkItem{
		Status: domain.WorkItemActive,
		Phases: map[string]domain.PhaseState{
			"review":  {Status: domain.PhaseApproved},
			"archive": {Status: domain.PhaseReady},
		},
	}

	if err := item.Complete(workflow); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if item.Status != domain.WorkItemCompleted {
		t.Fatalf("work item status = %s, want %s", item.Status, domain.WorkItemCompleted)
	}
}

func TestCompleteWorkItemRejectsStartedOptionalPhase(t *testing.T) {
	workflow := &domain.Workflow{
		Phases: []domain.WorkflowPhase{
			{ID: "review", Approval: domain.ApprovalRequired},
			{ID: "archive", Approval: domain.ApprovalNone, Optional: true},
		},
	}
	item := &domain.WorkItem{
		Status: domain.WorkItemActive,
		Phases: map[string]domain.PhaseState{
			"review":  {Status: domain.PhaseApproved},
			"archive": {Status: domain.PhaseInProgress},
		},
	}

	if err := item.Complete(workflow); !errors.Is(err, domain.ErrWorkItemCannotComplete) {
		t.Fatalf("Complete() error = %v, want %v", err, domain.ErrWorkItemCannotComplete)
	}
}

func TestOptionalPhaseCanRunAfterWorkItemCompletion(t *testing.T) {
	workflow := workflowWithPhase("archive", domain.ApprovalNone, true)
	item := workItemWithPhase("archive", domain.PhaseReady)
	item.Status = domain.WorkItemCompleted

	if _, err := item.BeginPhase(workflow, "archive"); err != nil {
		t.Fatalf("BeginPhase() error = %v", err)
	}
	if _, err := item.DeliverPhase(workflow, "archive", false); err != nil {
		t.Fatalf("DeliverPhase() error = %v", err)
	}
	if got := item.Phases["archive"].Status; got != domain.PhaseCompleted {
		t.Fatalf("archive status = %s, want %s", got, domain.PhaseCompleted)
	}
}

func TestRequiredPhaseCannotMutateAfterWorkItemCompletion(t *testing.T) {
	workflow := workflowWithPhase("review", domain.ApprovalRequired, false)
	item := workItemWithPhase("review", domain.PhaseReady)
	item.Status = domain.WorkItemCompleted

	if _, err := item.BeginPhase(workflow, "review"); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("BeginPhase() error = %v, want %v", err, domain.ErrInvalidTransition)
	}
}

func TestArchiveCompletedWorkItemWithSatisfiedArchivePhase(t *testing.T) {
	workflow := workflowWithPhase("archive", domain.ApprovalNone, true)
	item := workItemWithPhase("archive", domain.PhaseCompleted)
	item.Status = domain.WorkItemCompleted

	if err := item.Archive(workflow); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	if item.Status != domain.WorkItemArchived {
		t.Fatalf("work item status = %s, want %s", item.Status, domain.WorkItemArchived)
	}
}

func TestArchiveRejectsUnsatisfiedArchivePhase(t *testing.T) {
	workflow := workflowWithPhase("archive", domain.ApprovalNone, true)
	item := workItemWithPhase("archive", domain.PhaseReady)
	item.Status = domain.WorkItemCompleted

	if err := item.Archive(workflow); !errors.Is(err, domain.ErrWorkItemCannotArchive) {
		t.Fatalf("Archive() error = %v, want %v", err, domain.ErrWorkItemCannotArchive)
	}
	if item.Status != domain.WorkItemCompleted {
		t.Fatalf("work item status = %s, want %s", item.Status, domain.WorkItemCompleted)
	}
}

func TestArchiveRejectsActiveWorkItem(t *testing.T) {
	workflow := workflowWithPhase("archive", domain.ApprovalNone, true)
	item := workItemWithPhase("archive", domain.PhaseCompleted)

	if err := item.Archive(workflow); !errors.Is(err, domain.ErrWorkItemCannotArchive) {
		t.Fatalf("Archive() error = %v, want %v", err, domain.ErrWorkItemCannotArchive)
	}
}

func TestArchiveAllowsWorkflowWithoutArchivePhase(t *testing.T) {
	workflow := workflowWithPhase("review", domain.ApprovalRequired, false)
	item := workItemWithPhase("review", domain.PhaseApproved)
	item.Status = domain.WorkItemCompleted

	if err := item.Archive(workflow); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
}

func workflowWithPhase(id string, policy domain.ApprovalPolicy, optional bool) *domain.Workflow {
	return &domain.Workflow{
		Phases: []domain.WorkflowPhase{{
			ID:       id,
			Approval: policy,
			Optional: optional,
		}},
	}
}

func workItemWithPhase(id string, status domain.PhaseStatus) *domain.WorkItem {
	return &domain.WorkItem{
		Status: domain.WorkItemActive,
		Phases: map[string]domain.PhaseState{
			id: {Status: status},
		},
	}
}
