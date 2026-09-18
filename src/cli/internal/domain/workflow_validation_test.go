package domain_test

import (
	"errors"
	"testing"

	"sdd-cli/internal/domain"
)

func TestWorkflowValidationRejectsInvalidGraphs(t *testing.T) {
	tests := []struct {
		name     string
		workflow *domain.Workflow
	}{
		{
			name: "cycle",
			workflow: testWorkflow(
				[]domain.WorkflowPhase{
					{ID: "one", Requires: []string{"two"}, Produces: []string{"one"}, Approval: domain.ApprovalNone},
					{ID: "two", Requires: []string{"one"}, Produces: []string{"two"}, Approval: domain.ApprovalNone},
				},
				[]domain.EntryPoint{{Phase: "one", Accepts: []string{"user_prompt", "one"}}},
			),
		},
		{
			name: "unknown dependency",
			workflow: testWorkflow(
				[]domain.WorkflowPhase{{ID: "one", Requires: []string{"missing"}, Produces: []string{"one"}, Approval: domain.ApprovalNone}},
				[]domain.EntryPoint{{Phase: "one", Accepts: []string{"user_prompt", "one"}}},
			),
		},
		{
			name: "invalid entry input",
			workflow: testWorkflow(
				[]domain.WorkflowPhase{{ID: "one", Produces: []string{"one"}, Approval: domain.ApprovalNone}},
				[]domain.EntryPoint{{Phase: "one", Accepts: []string{"other"}}},
			),
		},
		{
			name: "traversing artifact path",
			workflow: &domain.Workflow{
				SchemaVersion: "0.1",
				Kind:          "workflow",
				ID:            "test-workflow",
				Title:         "Test",
				WorkItemType:  "test",
				EntryPoints:   []domain.EntryPoint{{Phase: "one", Accepts: []string{"user_prompt", "one"}}},
				Phases:        []domain.WorkflowPhase{{ID: "one", Produces: []string{"one"}, Approval: domain.ApprovalNone}},
				Artifacts: map[string]domain.WorkflowArtifact{
					"one": {Path: "../outside.md", Template: "one"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.workflow.ValidateSemantics(); !errors.Is(err, domain.ErrInvalidWorkflow) {
				t.Fatalf("ValidateSemantics() error = %v, want %v", err, domain.ErrInvalidWorkflow)
			}
		})
	}
}

func TestOrderedPhasesDoesNotDependOnDeclarationOrder(t *testing.T) {
	workflow := testWorkflow(
		[]domain.WorkflowPhase{
			{ID: "verification", Requires: []string{"implementation"}, Produces: []string{"verification"}, Approval: domain.ApprovalNone},
			{ID: "plan", Produces: []string{"plan"}, Approval: domain.ApprovalRequired},
			{ID: "implementation", Requires: []string{"plan"}, Produces: []string{"implementation"}, Approval: domain.ApprovalNone},
		},
		[]domain.EntryPoint{{Phase: "plan", Accepts: []string{"user_prompt", "plan"}}},
	)

	ordered, err := workflow.OrderedPhases()
	if err != nil {
		t.Fatalf("OrderedPhases() error = %v", err)
	}
	got := []string{ordered[0].ID, ordered[1].ID, ordered[2].ID}
	want := []string{"plan", "implementation", "verification"}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("OrderedPhases() = %v, want %v", got, want)
		}
	}
}

func TestWorkflowValidationAccumulatesViolations(t *testing.T) {
	workflow := testWorkflow(
		[]domain.WorkflowPhase{
			{ID: "one", Requires: []string{"missing"}, Produces: []string{"one"}, Approval: domain.ApprovalNone},
			{ID: "one", Produces: []string{"two"}, Approval: domain.ApprovalNone},
		},
		[]domain.EntryPoint{{Phase: "unknown", Accepts: []string{"other"}}},
	)
	violations := workflow.SemanticViolations()
	if len(violations) < 3 {
		t.Fatalf("SemanticViolations() = %#v", violations)
	}
	if err := workflow.ValidateSemantics(); err == nil {
		t.Fatal("ValidateSemantics() error = nil")
	}
}

func testWorkflow(phases []domain.WorkflowPhase, entries []domain.EntryPoint) *domain.Workflow {
	artifacts := make(map[string]domain.WorkflowArtifact)
	for _, phase := range phases {
		for _, artifactID := range phase.Produces {
			artifacts[artifactID] = domain.WorkflowArtifact{
				Path:     "artifacts/" + artifactID + ".md",
				Template: artifactID,
			}
		}
	}
	return &domain.Workflow{
		SchemaVersion: "0.1",
		Kind:          "workflow",
		ID:            "test-workflow",
		Title:         "Test",
		WorkItemType:  "test",
		EntryPoints:   entries,
		Phases:        phases,
		Artifacts:     artifacts,
	}
}
