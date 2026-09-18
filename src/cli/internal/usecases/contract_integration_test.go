package usecases_test

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/infra"
	"sdd-cli/internal/usecases"
)

func TestStartUsesConfiguredDefaultWorkflowAndAllowsOverride(t *testing.T) {
	tmpDir := setupTestEnv(t)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, ".sdd", "config.yaml")
	config := `schema_version: "0.1"
defaults:
  workflow: fast-change
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	startUC := newStartUseCase()
	actor := domain.Actor{Kind: domain.ActorHuman, ID: "matias"}
	defaultItem, err := startUC.Execute(tmpDir, usecases.StartWorkItemInput{
		ID:      "fast-default",
		Title:   "Configured workflow",
		Summary: "Use config default",
		Actor:   actor,
	})
	if err != nil {
		t.Fatalf("Execute() default error = %v", err)
	}
	if defaultItem.Workflow.ID != "fast-change" || defaultItem.Workflow.EntryPhase != "plan" {
		t.Fatalf("default workflow = %#v", defaultItem.Workflow)
	}

	overrideItem, err := startUC.Execute(tmpDir, usecases.StartWorkItemInput{
		ID:         "feature-override",
		WorkflowID: "feature-standard",
		Title:      "Explicit workflow",
		Summary:    "Override config",
		Actor:      actor,
	})
	if err != nil {
		t.Fatalf("Execute() override error = %v", err)
	}
	if overrideItem.Workflow.ID != "feature-standard" {
		t.Fatalf("override workflow = %s", overrideItem.Workflow.ID)
	}
}

func TestStartUsesLocalTemplate(t *testing.T) {
	tmpDir := setupTestEnv(t)
	defer os.RemoveAll(tmpDir)

	templatePath := filepath.Join(tmpDir, ".sdd", "templates", "prd.md")
	template, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	template = append(template, []byte("\nLOCAL TEMPLATE MARKER\n")...)
	if err := os.WriteFile(templatePath, template, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err = newStartUseCase().Execute(tmpDir, usecases.StartWorkItemInput{
		ID:         "feature-local-template",
		WorkflowID: "feature-standard",
		Title:      "Local template",
		Actor:      domain.Actor{Kind: domain.ActorHuman, ID: "matias"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	artifactPath := filepath.Join(tmpDir, ".sdd", "work-items", "active", "feature-local-template", "artifacts", "prd.md")
	artifact, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("ReadFile() artifact error = %v", err)
	}
	if !strings.Contains(string(artifact), "LOCAL TEMPLATE MARKER") {
		t.Fatal("generated artifact did not use the local template")
	}
	for _, field := range []string{
		`schema_version: "0.1"`,
		`work_item: "feature-local-template"`,
		`created_by: { kind: "cli", id: "sdd" }`,
		"sources: []",
	} {
		if !strings.Contains(string(artifact), field) {
			t.Fatalf("generated artifact is missing %q", field)
		}
	}
}

func TestStartRejectsInvalidLocalTemplateBeforeWriting(t *testing.T) {
	tmpDir := setupTestEnv(t)
	defer os.RemoveAll(tmpDir)

	templatePath := filepath.Join(tmpDir, ".sdd", "templates", "prd.md")
	if err := os.WriteFile(templatePath, []byte("# Missing front matter\n"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := newStartUseCase().Execute(tmpDir, usecases.StartWorkItemInput{
		ID:         "invalid-template",
		WorkflowID: "feature-standard",
		Title:      "Invalid template",
		Actor:      domain.Actor{Kind: domain.ActorHuman, ID: "matias"},
	})
	if !errors.Is(err, domain.ErrInvalidWorkflow) {
		t.Fatalf("Execute() error = %v, want %v", err, domain.ErrInvalidWorkflow)
	}
	manifestPath := filepath.Join(tmpDir, ".sdd", "work-items", "active", "invalid-template", "manifest.yaml")
	if _, statErr := os.Stat(manifestPath); !os.IsNotExist(statErr) {
		t.Fatalf("manifest was written despite invalid template: %v", statErr)
	}
}

func TestStartValidatesExternalArtifactEntryAndRecordsHash(t *testing.T) {
	tmpDir := setupTestEnv(t)
	defer os.RemoveAll(tmpDir)

	sourcePath := filepath.Join(tmpDir, "external-plan.md")
	source := []byte("# Existing plan\n\nKeep this content.\n")
	if err := os.WriteFile(sourcePath, source, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	startUC := newStartUseCase()
	actor := domain.Actor{Kind: domain.ActorHuman, ID: "matias"}
	item, err := startUC.Execute(tmpDir, usecases.StartWorkItemInput{
		ID:           "feature-external-plan",
		WorkflowID:   "feature-standard",
		Title:        "External plan",
		FromArtifact: sourcePath,
		Phase:        "plan",
		Actor:        actor,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if item.Phases["prd"].Status != domain.PhaseNotApplicable ||
		item.Phases["specification"].Status != domain.PhaseNotApplicable ||
		item.Phases["plan"].Status != domain.PhaseAwaitingApproval {
		t.Fatalf("unexpected external entry states: %#v", item.Phases)
	}
	expectedHash := fmt.Sprintf("%x", sha256.Sum256(source))
	if item.Input.ExternalArtifact == nil || item.Input.ExternalArtifact.SHA256 != expectedHash {
		t.Fatalf("external artifact metadata = %#v", item.Input.ExternalArtifact)
	}

	importedPath := filepath.Join(tmpDir, ".sdd", "work-items", "active", item.ID, "artifacts", "plan.md")
	imported, err := os.ReadFile(importedPath)
	if err != nil {
		t.Fatalf("ReadFile() imported artifact error = %v", err)
	}
	if !strings.Contains(string(imported), `work_item: "feature-external-plan"`) ||
		!strings.Contains(string(imported), "Keep this content.") {
		t.Fatalf("imported artifact does not preserve metadata and content:\n%s", imported)
	}

	_, err = startUC.Execute(tmpDir, usecases.StartWorkItemInput{
		ID:           "feature-invalid-entry",
		WorkflowID:   "feature-standard",
		Title:        "Invalid entry",
		FromArtifact: sourcePath,
		Phase:        "implementation",
		Actor:        actor,
	})
	if !errors.Is(err, domain.ErrInvalidEntryPoint) {
		t.Fatalf("invalid entry error = %v, want %v", err, domain.ErrInvalidEntryPoint)
	}
}

func TestStartRejectsIncompleteExternalArtifactArguments(t *testing.T) {
	tmpDir := setupTestEnv(t)
	defer os.RemoveAll(tmpDir)

	startUC := newStartUseCase()
	actor := domain.Actor{Kind: domain.ActorHuman, ID: "matias"}
	tests := []usecases.StartWorkItemInput{
		{ID: "missing-phase", WorkflowID: "feature-standard", Title: "Missing phase", FromArtifact: "file.md", Actor: actor},
		{ID: "missing-artifact", WorkflowID: "feature-standard", Title: "Missing artifact", Phase: "prd", Actor: actor},
	}
	for _, input := range tests {
		if _, err := startUC.Execute(tmpDir, input); !errors.Is(err, domain.ErrInvalidExternalArtifact) {
			t.Fatalf("Execute(%s) error = %v, want %v", input.ID, err, domain.ErrInvalidExternalArtifact)
		}
	}
}

func TestStartRejectsTraversalIdentifierBeforeWriting(t *testing.T) {
	tmpDir := setupTestEnv(t)
	defer os.RemoveAll(tmpDir)

	_, err := newStartUseCase().Execute(tmpDir, usecases.StartWorkItemInput{
		ID:         "../../../escape",
		WorkflowID: "feature-standard",
		Title:      "Traversal",
		Actor:      domain.Actor{Kind: domain.ActorHuman, ID: "matias"},
	})
	if !errors.Is(err, domain.ErrInvalidIdentifier) {
		t.Fatalf("Execute() error = %v, want %v", err, domain.ErrInvalidIdentifier)
	}
	if _, statErr := os.Stat(filepath.Join(tmpDir, "escape")); !os.IsNotExist(statErr) {
		t.Fatalf("unexpected escaped path state: %v", statErr)
	}
}

func TestRecordEventAppliesEventSchema(t *testing.T) {
	tmpDir := setupTestEnv(t)
	defer os.RemoveAll(tmpDir)

	actor := domain.Actor{Kind: domain.ActorHuman, ID: "matias"}
	if _, err := newStartUseCase().Execute(tmpDir, usecases.StartWorkItemInput{
		ID:         "event-validation",
		WorkflowID: "feature-standard",
		Title:      "Event validation",
		Actor:      actor,
	}); err != nil {
		t.Fatalf("start error = %v", err)
	}
	repository := infra.NewFSWorkItemRepository()
	before, err := repository.GetWorkItem(tmpDir, "event-validation")
	if err != nil {
		t.Fatalf("GetWorkItem() before error = %v", err)
	}
	eventsPath := filepath.Join(tmpDir, ".sdd", "work-items", "active", "event-validation", "events.jsonl")
	eventsBefore, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("ReadFile() events before error = %v", err)
	}

	recordUC := usecases.NewRecordEventUseCase(
		repository,
		infra.NewSystemClock(),
		infra.NewCryptoIDGenerator(),
	)
	err = recordUC.Execute(tmpDir, usecases.RecordEventInput{
		WorkItemID: "event-validation",
		EventType:  "INVALID EVENT TYPE",
		Actor:      actor,
	})
	if !errors.Is(err, domain.ErrSchemaValidation) {
		t.Fatalf("RecordEvent() error = %v, want %v", err, domain.ErrSchemaValidation)
	}
	after, err := repository.GetWorkItem(tmpDir, "event-validation")
	if err != nil {
		t.Fatalf("GetWorkItem() after error = %v", err)
	}
	eventsAfter, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("ReadFile() events after error = %v", err)
	}
	if after.Revision != before.Revision || !reflect.DeepEqual(eventsAfter, eventsBefore) {
		t.Fatal("failed event mutation changed the persisted snapshot")
	}
}

func TestStatusRejectsStructurallyAndSemanticallyInvalidManifest(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(t *testing.T, manifestPath string)
		wantError error
	}{
		{
			name: "invalid schema",
			mutate: func(t *testing.T, manifestPath string) {
				t.Helper()
				data, err := os.ReadFile(manifestPath)
				if err != nil {
					t.Fatalf("ReadFile() error = %v", err)
				}
				data = []byte(strings.Replace(string(data), "status: active", "status: invalid", 1))
				if err := os.WriteFile(manifestPath, data, 0644); err != nil {
					t.Fatalf("WriteFile() error = %v", err)
				}
			},
			wantError: domain.ErrSchemaValidation,
		},
		{
			name: "invalid dependency state",
			mutate: func(t *testing.T, manifestPath string) {
				t.Helper()
				data, err := os.ReadFile(manifestPath)
				if err != nil {
					t.Fatalf("ReadFile() error = %v", err)
				}
				var item domain.WorkItem
				if err := yaml.Unmarshal(data, &item); err != nil {
					t.Fatalf("Unmarshal() error = %v", err)
				}
				state := item.Phases["implementation"]
				state.Status = domain.PhaseInProgress
				item.Phases["implementation"] = state
				data, err = yaml.Marshal(item)
				if err != nil {
					t.Fatalf("Marshal() error = %v", err)
				}
				if err := os.WriteFile(manifestPath, data, 0644); err != nil {
					t.Fatalf("WriteFile() error = %v", err)
				}
			},
			wantError: domain.ErrInvalidWorkItem,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := setupTestEnv(t)
			defer os.RemoveAll(tmpDir)

			if _, err := newStartUseCase().Execute(tmpDir, usecases.StartWorkItemInput{
				ID:         "manifest-validation",
				WorkflowID: "feature-standard",
				Title:      "Manifest validation",
				Actor:      domain.Actor{Kind: domain.ActorHuman, ID: "matias"},
			}); err != nil {
				t.Fatalf("start error = %v", err)
			}
			manifestPath := filepath.Join(tmpDir, ".sdd", "work-items", "active", "manifest-validation", "manifest.yaml")
			tt.mutate(t, manifestPath)

			_, err := usecases.NewStatusUseCase(
				infra.NewFSWorkItemRepository(),
				infra.NewFSWorkflowRepository(),
			).Execute(tmpDir, "manifest-validation")
			if !errors.Is(err, tt.wantError) {
				t.Fatalf("Status() error = %v, want %v", err, tt.wantError)
			}
		})
	}
}

func newStartUseCase() *usecases.StartWorkItemUseCase {
	return usecases.NewStartWorkItemUseCase(
		infra.NewFSWorkItemRepository(),
		infra.NewFSWorkflowRepository(),
		infra.NewFSConfigRepository(),
		infra.NewArtifactManager(),
		infra.NewSystemClock(),
		infra.NewCryptoIDGenerator(),
	)
}
