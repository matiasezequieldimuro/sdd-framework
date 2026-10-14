package domain_test

import (
	"errors"
	"testing"

	"sdd-cli/internal/domain"
)

// TestCancelActiveWorkItemSucceeds verifies that Cancel transitions an active
// work item to cancelled and does not modify any phase states (CA-CA-06, POST-CA-03).
func TestCancelActiveWorkItemSucceeds(t *testing.T) {
	item := &domain.WorkItem{
		Status: domain.WorkItemActive,
		Phases: map[string]domain.PhaseState{
			"plan":           {Status: domain.PhaseCompleted},
			"implementation": {Status: domain.PhaseInProgress},
		},
	}
	// Capture phase states before cancellation for comparison.
	beforePlan := item.Phases["plan"].Status
	beforeImpl := item.Phases["implementation"].Status

	if err := item.Cancel(); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if item.Status != domain.WorkItemCancelled {
		t.Fatalf("work item status = %s, want cancelled", item.Status)
	}
	// Phase states must be unchanged (POST-CA-03).
	if got := item.Phases["plan"].Status; got != beforePlan {
		t.Fatalf("plan phase changed from %s to %s after cancel", beforePlan, got)
	}
	if got := item.Phases["implementation"].Status; got != beforeImpl {
		t.Fatalf("implementation phase changed from %s to %s after cancel", beforeImpl, got)
	}
}

// TestCancelNonActiveWorkItemReturnsError verifies that Cancel rejects work items
// not in active status with ErrWorkItemCannotCancel (CA-CA-02/03).
func TestCancelNonActiveWorkItemReturnsError(t *testing.T) {
	statuses := []domain.WorkItemStatus{
		domain.WorkItemCompleted,
		domain.WorkItemArchived,
		domain.WorkItemCancelled,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			item := &domain.WorkItem{
				Status: status,
				Phases: map[string]domain.PhaseState{},
			}
			if err := item.Cancel(); !errors.Is(err, domain.ErrWorkItemCannotCancel) {
				t.Fatalf("Cancel() error = %v, want %v", err, domain.ErrWorkItemCannotCancel)
			}
			// Status must remain unchanged after the rejected cancel attempt.
			if item.Status != status {
				t.Fatalf("item status changed from %s to %s", status, item.Status)
			}
		})
	}
}

// TestCancelPreservesAllPhaseStates verifies that Cancel leaves all phases intact
// regardless of their individual states (CA-CA-06 / POST-CA-03).
func TestCancelPreservesAllPhaseStates(t *testing.T) {
	phases := map[string]domain.PhaseState{
		"prd":            {Status: domain.PhaseNotApplicable},
		"specification":  {Status: domain.PhaseCompleted},
		"plan":           {Status: domain.PhaseApproved},
		"implementation": {Status: domain.PhaseInProgress},
	}
	item := &domain.WorkItem{
		Status: domain.WorkItemActive,
		Phases: phases,
	}

	if err := item.Cancel(); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	for phaseID, before := range phases {
		if after := item.Phases[phaseID].Status; after != before.Status {
			t.Fatalf("phase %s changed from %s to %s after cancel", phaseID, before.Status, after)
		}
	}
}

// TestValidateAgainstSupersededPhaseIsExempted is a regression guard for DD-5:
// a work item whose superseded descendant's dependency is not yet satisfied must
// NOT produce a dependencies_unsatisfied validation failure. Before the fix,
// ValidateAgainst would incorrectly flag the superseded phase because its
// dependency (plan=ready) does not satisfy satisfiesDependency().
func TestValidateAgainstSupersededPhaseIsExempted(t *testing.T) {
	// Minimal two-phase linear workflow: plan → implementation.
	workflow := &domain.Workflow{
		SchemaVersion: "0.1",
		Kind:          "workflow",
		ID:            "dd5-regression",
		Title:         "DD-5 Regression",
		WorkItemType:  "dd5-regression",
		EntryPoints: []domain.EntryPoint{{
			Phase:   "plan",
			Accepts: []string{"user_prompt", "plan"},
		}},
		Phases: []domain.WorkflowPhase{
			{ID: "plan", Produces: []string{"plan"}, Approval: domain.ApprovalRequired},
			{
				ID:       "implementation",
				Requires: []string{"plan"},
				Produces: []string{"implementation"},
				Approval: domain.ApprovalNone,
			},
		},
		Artifacts: map[string]domain.WorkflowArtifact{
			"plan":           {Path: "artifacts/plan.md", Template: "plan"},
			"implementation": {Path: "artifacts/implementation-report.md", Template: "implementation-report"},
		},
	}
	// Post-rework state: plan reset to ready, implementation superseded.
	item := &domain.WorkItem{
		SchemaVersion: "0.1",
		Kind:          "work-item",
		ID:            "dd5-test-item",
		Type:          "dd5-regression", // must match workflow.WorkItemType
		Status:        domain.WorkItemActive,
		Workflow: domain.WorkItemWorkflow{
			ID:         "dd5-regression",
			Version:    "0.1",
			EntryPhase: "plan",
		},
		Input: domain.WorkItemInput{Source: "user_prompt"},
		Phases: map[string]domain.PhaseState{
			"plan":           {Status: domain.PhaseReady, Artifact: "artifacts/plan.md"},
			"implementation": {Status: domain.PhaseSuperseded, Artifact: "artifacts/implementation-report.md"},
		},
		Traceability: domain.Traceability{Events: "events.jsonl"},
	}

	if err := item.ValidateAgainst(workflow); err != nil {
		t.Fatalf(
			"ValidateAgainst() error = %v, want nil — "+
				"DD-5 exemption must suppress the dependency check for superseded phases",
			err,
		)
	}
}
