package domain_test

import (
	"errors"
	"testing"

	"sdd-cli/internal/domain"
)

// reworkWorkflow returns a 4-phase linear DAG (plan → implementation → verification → review)
// suitable for rework propagation tests.
func reworkWorkflow() *domain.Workflow {
	return &domain.Workflow{
		SchemaVersion: "0.1",
		Kind:          "workflow",
		ID:            "rework-test",
		Title:         "Rework Test",
		WorkItemType:  "rework-test",
		EntryPoints: []domain.EntryPoint{{
			Phase:   "plan",
			Accepts: []string{"user_prompt", "plan"},
		}},
		Phases: []domain.WorkflowPhase{
			{ID: "plan", Produces: []string{"plan"}, Approval: domain.ApprovalRequired},
			{ID: "implementation", Requires: []string{"plan"}, Produces: []string{"implementation"}, Approval: domain.ApprovalNone},
			{ID: "verification", Requires: []string{"implementation"}, Produces: []string{"verification"}, Approval: domain.ApprovalNone},
			{ID: "review", Requires: []string{"verification"}, Produces: []string{"review"}, Approval: domain.ApprovalRequired},
		},
		Artifacts: map[string]domain.WorkflowArtifact{
			"plan":           {Path: "artifacts/plan.md", Template: "plan"},
			"implementation": {Path: "artifacts/implementation-report.md", Template: "implementation-report"},
			"verification":   {Path: "artifacts/verification-report.md", Template: "verification-report"},
			"review":         {Path: "artifacts/review.md", Template: "review"},
		},
	}
}

// reworkWorkItemAllDone returns a work item where all phases of reworkWorkflow
// are in a post-work terminal state valid as a rework precondition.
func reworkWorkItemAllDone() *domain.WorkItem {
	return &domain.WorkItem{
		Status: domain.WorkItemActive,
		Phases: map[string]domain.PhaseState{
			"plan":           {Status: domain.PhaseCompleted},
			"implementation": {Status: domain.PhaseCompleted},
			"verification":   {Status: domain.PhaseCompleted},
			"review":         {Status: domain.PhaseApproved},
		},
	}
}

// TestReworkPhaseTargetTransitionMatrix verifies which source states allow
// reworking (completed, approved, accepted) and which are rejected with
// ErrInvalidTransition (CA-RW-01, PC-RW-05).
func TestReworkPhaseTargetTransitionMatrix(t *testing.T) {
	tests := []struct {
		status  domain.PhaseStatus
		wantErr error
	}{
		{status: domain.PhaseCompleted},
		{status: domain.PhaseApproved},
		{status: domain.PhaseAccepted},
		{status: domain.PhaseReady, wantErr: domain.ErrInvalidTransition},
		{status: domain.PhaseInProgress, wantErr: domain.ErrInvalidTransition},
		{status: domain.PhaseAwaitingApproval, wantErr: domain.ErrInvalidTransition},
		{status: domain.PhaseBlocked, wantErr: domain.ErrInvalidTransition},
		{status: domain.PhaseRejected, wantErr: domain.ErrInvalidTransition},
		{status: domain.PhaseNotApplicable, wantErr: domain.ErrInvalidTransition},
		{status: domain.PhaseSuperseded, wantErr: domain.ErrInvalidTransition},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			workflow := workflowWithPhase("plan", domain.ApprovalRequired, false)
			item := &domain.WorkItem{
				Status: domain.WorkItemActive,
				Phases: map[string]domain.PhaseState{
					"plan": {Status: tt.status},
				},
			}

			mutation, err := item.ReworkPhase(workflow, "plan")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ReworkPhase() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if mutation.Target.Phase != "plan" ||
				mutation.Target.From != tt.status ||
				mutation.Target.To != domain.PhaseReady {
				t.Fatalf("target transition = %#v", mutation.Target)
			}
			if got := item.Phases["plan"].Status; got != domain.PhaseReady {
				t.Fatalf("phase status = %s, want ready", got)
			}
		})
	}
}

// TestReworkPhaseWorkItemStatusMatrix verifies which work item statuses allow
// reworking and which are rejected. Completed items are reactivated (CA-RW-03).
func TestReworkPhaseWorkItemStatusMatrix(t *testing.T) {
	tests := []struct {
		status      domain.WorkItemStatus
		wantErr     error
		reactivated bool
	}{
		{status: domain.WorkItemActive},
		{status: domain.WorkItemCompleted, reactivated: true},
		{status: domain.WorkItemArchived, wantErr: domain.ErrInvalidTransition},
		{status: domain.WorkItemCancelled, wantErr: domain.ErrInvalidTransition},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			workflow := workflowWithPhase("plan", domain.ApprovalRequired, false)
			item := &domain.WorkItem{
				Status: tt.status,
				Phases: map[string]domain.PhaseState{
					"plan": {Status: domain.PhaseCompleted},
				},
			}

			mutation, err := item.ReworkPhase(workflow, "plan")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ReworkPhase() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if mutation.Reactivated != tt.reactivated {
				t.Fatalf("Reactivated = %v, want %v", mutation.Reactivated, tt.reactivated)
			}
			if tt.reactivated && item.Status != domain.WorkItemActive {
				t.Fatalf("work item status after reactivation = %s, want active", item.Status)
			}
		})
	}
}

// TestReworkPhaseReturnsErrPhaseNotFoundForUnknownPhase verifies that reworking
// a phase not declared in the workflow returns ErrPhaseNotFound (CA-RW-05).
func TestReworkPhaseReturnsErrPhaseNotFoundForUnknownPhase(t *testing.T) {
	workflow := workflowWithPhase("plan", domain.ApprovalRequired, false)
	item := &domain.WorkItem{
		Status: domain.WorkItemActive,
		Phases: map[string]domain.PhaseState{
			"plan": {Status: domain.PhaseCompleted},
		},
	}

	if _, err := item.ReworkPhase(workflow, "nonexistent"); !errors.Is(err, domain.ErrPhaseNotFound) {
		t.Fatalf("ReworkPhase() error = %v, want %v", err, domain.ErrPhaseNotFound)
	}
}

// TestReworkPhaseTransitivePropagation verifies that reworking plan supersedes
// every descendant that was in a worked state (CA-RW-01, RB-RW-02).
func TestReworkPhaseTransitivePropagation(t *testing.T) {
	workflow := reworkWorkflow()
	item := reworkWorkItemAllDone()

	mutation, err := item.ReworkPhase(workflow, "plan")
	if err != nil {
		t.Fatalf("ReworkPhase() error = %v", err)
	}

	if got := item.Phases["plan"].Status; got != domain.PhaseReady {
		t.Fatalf("plan status = %s, want ready", got)
	}
	for _, phaseID := range []string{"implementation", "verification", "review"} {
		if got := item.Phases[phaseID].Status; got != domain.PhaseSuperseded {
			t.Fatalf("phase %s status = %s, want superseded", phaseID, got)
		}
	}
	if got := len(mutation.Superseded); got != 3 {
		t.Fatalf("len(mutation.Superseded) = %d, want 3", got)
	}
}

// TestReworkPhaseReadyDescendantIsReblocked verifies that a descendant currently
// in ready (unlocked but not started) is re-blocked rather than superseded (RB-RW-02).
func TestReworkPhaseReadyDescendantIsReblocked(t *testing.T) {
	workflow := reworkWorkflow()
	item := &domain.WorkItem{
		Status: domain.WorkItemActive,
		Phases: map[string]domain.PhaseState{
			"plan":           {Status: domain.PhaseCompleted},
			"implementation": {Status: domain.PhaseReady},
			"verification":   {Status: domain.PhaseBlocked},
			"review":         {Status: domain.PhaseBlocked},
		},
	}

	mutation, err := item.ReworkPhase(workflow, "plan")
	if err != nil {
		t.Fatalf("ReworkPhase() error = %v", err)
	}

	if got := item.Phases["implementation"].Status; got != domain.PhaseBlocked {
		t.Fatalf("implementation status = %s, want blocked", got)
	}
	// verification and review were already blocked → unchanged, not in Superseded
	if got := item.Phases["verification"].Status; got != domain.PhaseBlocked {
		t.Fatalf("verification status = %s, want blocked", got)
	}
	// Only implementation should appear in Superseded (re-blocked from ready)
	if got := len(mutation.Superseded); got != 1 {
		t.Fatalf("len(mutation.Superseded) = %d, want 1", got)
	}
	if mutation.Superseded[0].Phase != "implementation" ||
		mutation.Superseded[0].From != domain.PhaseReady ||
		mutation.Superseded[0].To != domain.PhaseBlocked {
		t.Fatalf("Superseded[0] = %#v", mutation.Superseded[0])
	}
}

// TestReworkPhaseAlreadySupersededDescendantIsSkipped verifies that a descendant
// already in superseded is not re-added to mutation.Superseded (CA-RW-09).
func TestReworkPhaseAlreadySupersededDescendantIsSkipped(t *testing.T) {
	workflow := reworkWorkflow()
	item := &domain.WorkItem{
		Status: domain.WorkItemActive,
		Phases: map[string]domain.PhaseState{
			"plan":           {Status: domain.PhaseCompleted},
			"implementation": {Status: domain.PhaseSuperseded},
			"verification":   {Status: domain.PhaseBlocked},
			"review":         {Status: domain.PhaseBlocked},
		},
	}

	mutation, err := item.ReworkPhase(workflow, "plan")
	if err != nil {
		t.Fatalf("ReworkPhase() error = %v", err)
	}

	for _, tr := range mutation.Superseded {
		if tr.Phase == "implementation" {
			t.Fatalf("already-superseded descendant 'implementation' must not appear in Superseded: %#v", tr)
		}
	}
	// implementation remains superseded
	if got := item.Phases["implementation"].Status; got != domain.PhaseSuperseded {
		t.Fatalf("implementation status changed from superseded to %s", got)
	}
}

// TestReworkPhaseNotApplicableDescendantUnchanged verifies that a descendant in
// not_applicable is left untouched (CA-RW-11).
func TestReworkPhaseNotApplicableDescendantUnchanged(t *testing.T) {
	workflow := reworkWorkflow()
	item := &domain.WorkItem{
		Status: domain.WorkItemActive,
		Phases: map[string]domain.PhaseState{
			"plan":           {Status: domain.PhaseCompleted},
			"implementation": {Status: domain.PhaseNotApplicable},
			"verification":   {Status: domain.PhaseBlocked},
			"review":         {Status: domain.PhaseBlocked},
		},
	}

	mutation, err := item.ReworkPhase(workflow, "plan")
	if err != nil {
		t.Fatalf("ReworkPhase() error = %v", err)
	}

	if got := item.Phases["implementation"].Status; got != domain.PhaseNotApplicable {
		t.Fatalf("not_applicable phase changed to %s", got)
	}
	for _, tr := range mutation.Superseded {
		if tr.Phase == "implementation" {
			t.Fatalf("not_applicable descendant must not appear in Superseded: %#v", tr)
		}
	}
}

// TestReworkPhaseApprovalHistoryPreserved verifies that existing approval records
// are not modified by rework (CA-RW-07).
func TestReworkPhaseApprovalHistoryPreserved(t *testing.T) {
	workflow := workflowWithPhase("plan", domain.ApprovalRequired, false)
	actor := domain.Actor{Kind: domain.ActorHuman, ID: "matias"}
	item := &domain.WorkItem{
		Status: domain.WorkItemActive,
		Phases: map[string]domain.PhaseState{
			"plan": {Status: domain.PhaseApproved},
		},
		Approvals: []domain.Approval{
			{Phase: "plan", Status: domain.ApprovalApproved, By: &actor, At: "2026-08-01T00:00:00Z", Comment: "LGTM"},
		},
	}

	if _, err := item.ReworkPhase(workflow, "plan"); err != nil {
		t.Fatalf("ReworkPhase() error = %v", err)
	}

	if len(item.Approvals) != 1 ||
		item.Approvals[0].Status != domain.ApprovalApproved ||
		item.Approvals[0].By == nil ||
		*item.Approvals[0].By != actor ||
		item.Approvals[0].Comment != "LGTM" {
		t.Fatalf("approvals modified by rework: %#v", item.Approvals)
	}
}

// TestReworkPhaseSupersededOrderIsTopological verifies that mutation.Superseded
// lists descendants in topological order (plan → implementation → verification → review).
func TestReworkPhaseSupersededOrderIsTopological(t *testing.T) {
	workflow := reworkWorkflow()
	item := reworkWorkItemAllDone()

	mutation, err := item.ReworkPhase(workflow, "plan")
	if err != nil {
		t.Fatalf("ReworkPhase() error = %v", err)
	}

	want := []string{"implementation", "verification", "review"}
	if len(mutation.Superseded) != len(want) {
		t.Fatalf("len(Superseded) = %d, want %d", len(mutation.Superseded), len(want))
	}
	for i, phase := range want {
		if mutation.Superseded[i].Phase != phase {
			t.Fatalf("Superseded[%d].Phase = %s, want %s", i, mutation.Superseded[i].Phase, phase)
		}
	}
}

// TestReworkSupersededPhaseRestartRequiresDependencies verifies that a superseded
// descendant cannot be restarted before the reworked predecessor is satisfied again,
// and that it can be restarted once it is.
func TestReworkSupersededPhaseRestartRequiresDependencies(t *testing.T) {
	workflow := reworkWorkflow()
	item := reworkWorkItemAllDone()

	if _, err := item.ReworkPhase(workflow, "plan"); err != nil {
		t.Fatalf("ReworkPhase() error = %v", err)
	}
	if _, err := item.BeginPhase(workflow, "implementation"); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("BeginPhase(implementation) with plan ready error = %v, want %v", err, domain.ErrInvalidTransition)
	}

	plan := item.Phases["plan"]
	plan.Status = domain.PhaseApproved
	item.Phases["plan"] = plan

	if _, err := item.BeginPhase(workflow, "implementation"); err != nil {
		t.Fatalf("BeginPhase(implementation) with plan approved error = %v", err)
	}
	if got := item.Phases["implementation"].Status; got != domain.PhaseInProgress {
		t.Fatalf("implementation status = %s, want in_progress", got)
	}
}

// TestNextPhaseReportsSupersededPhaseWithSatisfiedDependencies verifies that next
// guides the restart of superseded phases in topological order.
func TestNextPhaseReportsSupersededPhaseWithSatisfiedDependencies(t *testing.T) {
	workflow := reworkWorkflow()
	item := reworkWorkItemAllDone()

	if _, err := item.ReworkPhase(workflow, "implementation"); err != nil {
		t.Fatalf("ReworkPhase() error = %v", err)
	}
	impl := item.Phases["implementation"]
	impl.Status = domain.PhaseCompleted
	item.Phases["implementation"] = impl

	next, err := item.NextPhase(workflow)
	if err != nil {
		t.Fatalf("NextPhase() error = %v", err)
	}
	if next == nil || next.Definition.ID != "verification" || next.State.Status != domain.PhaseSuperseded {
		t.Fatalf("NextPhase() = %#v, want superseded verification", next)
	}
}
