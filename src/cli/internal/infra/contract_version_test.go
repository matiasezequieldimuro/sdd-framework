package infra

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"sdd-cli/internal/domain"
)

// readYAMLMap decodes a YAML file into a generic map.
func readYAMLMap(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	var document map[string]interface{}
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("Unmarshal(%s) error = %v", path, err)
	}
	return document
}

// replaceInFile rewrites a file replacing the first occurrence of old with new.
func replaceInFile(t *testing.T, path, old, new string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	content := string(data)
	if !strings.Contains(content, old) {
		t.Fatalf("%s does not contain %q", path, old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(content, old, new, 1)), 0644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

// findChecks returns the checks with the given code.
func findChecks(checks []domain.ValidationCheck, code string) []domain.ValidationCheck {
	var found []domain.ValidationCheck
	for _, check := range checks {
		if check.Code == code {
			found = append(found, check)
		}
	}
	return found
}

func assertCheckFailed(t *testing.T, checks []domain.ValidationCheck, code string) {
	t.Helper()
	found := findChecks(checks, code)
	if len(found) == 0 {
		t.Fatalf("check %s not found", code)
	}
	for _, check := range found {
		if check.Status != domain.CheckFailed {
			t.Fatalf("check %s status = %s, want failed", code, check.Status)
		}
	}
}

// T3: the source contract data must agree with domain.ContractSchemaVersion.
func TestContractSchemaVersionIsConsistent(t *testing.T) {
	root := contractBaseDir(t)
	sdd := filepath.Join(root, ".sdd")

	var yamlFiles []string
	yamlFiles = append(yamlFiles,
		filepath.Join(sdd, "config.yaml"),
		filepath.Join(sdd, "registry", "capabilities.yaml"),
		filepath.Join(root, "adapters", "claude-code", "adapter.yaml"),
	)
	workflows, err := filepath.Glob(filepath.Join(sdd, "workflows", "*.workflow.yaml"))
	if err != nil || len(workflows) == 0 {
		t.Fatalf("workflows glob = %v, err = %v", workflows, err)
	}
	yamlFiles = append(yamlFiles, workflows...)
	for _, path := range yamlFiles {
		if got := readYAMLMap(t, path)["schema_version"]; got != domain.ContractSchemaVersion {
			t.Errorf("%s schema_version = %v, want %q", path, got, domain.ContractSchemaVersion)
		}
	}

	templates, err := filepath.Glob(filepath.Join(sdd, "templates", "*.md"))
	if err != nil || len(templates) == 0 {
		t.Fatalf("templates glob = %v, err = %v", templates, err)
	}
	for _, path := range templates {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", path, err)
		}
		metadata, err := extractFrontMatter(string(data))
		if err != nil {
			t.Fatalf("extractFrontMatter(%s) error = %v", path, err)
		}
		var header struct {
			SchemaVersion string `yaml:"schema_version"`
		}
		if err := yaml.Unmarshal(metadata, &header); err != nil {
			t.Fatalf("Unmarshal(%s) error = %v", path, err)
		}
		if header.SchemaVersion != domain.ContractSchemaVersion {
			t.Errorf("%s schema_version = %q, want %q", path, header.SchemaVersion, domain.ContractSchemaVersion)
		}
	}

	for _, name := range []string{"artifact", "work-item", "workflow", "event"} {
		data, err := os.ReadFile(filepath.Join(sdd, "schemas", name+".schema.json"))
		if err != nil {
			t.Fatalf("ReadFile(%s schema) error = %v", name, err)
		}
		var schema struct {
			Properties struct {
				SchemaVersion struct {
					Const interface{}   `json:"const"`
					Enum  []interface{} `json:"enum"`
				} `json:"schema_version"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatalf("Unmarshal(%s schema) error = %v", name, err)
		}
		property := schema.Properties.SchemaVersion
		if name == "event" {
			want := []interface{}{"0.1", domain.ContractSchemaVersion}
			if !reflect.DeepEqual(property.Enum, want) {
				t.Errorf("event schema enum = %v, want %v", property.Enum, want)
			}
			continue
		}
		if property.Const != domain.ContractSchemaVersion {
			t.Errorf("%s schema const = %v, want %q", name, property.Const, domain.ContractSchemaVersion)
		}
	}
}

// T4: a legacy manifest is rejected with an explicit message.
func TestGetWorkItemRejectsLegacyManifestVersion(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := NewFSWorkItemRepository()
	item := persistInitialTestItem(t, baseDir, repository)
	manifest := filepath.Join(baseDir, ".sdd", "work-items", "active", item.ID, "manifest.yaml")

	replaceInFile(t, manifest, `schema_version: "1.0"`, `schema_version: "0.1"`)
	_, err := repository.GetWorkItem(baseDir, item.ID)
	if !errors.Is(err, domain.ErrSchemaValidation) {
		t.Fatalf("GetWorkItem() error = %v, want ErrSchemaValidation", err)
	}
	for _, want := range []string{"manifest", `"0.1"`, `"1.0"`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err.Error(), want)
		}
	}

	replaceInFile(t, manifest, `schema_version: "0.1"`, `schema_version: "1.0"`)
	replaceInFile(t, manifest, `version: "1.0"`, `version: "0.1"`)
	if _, err := repository.GetWorkItem(baseDir, item.ID); err == nil {
		t.Fatal("GetWorkItem() succeeded with legacy workflow.version")
	}
}

// T5: legacy workflow and template metadata are rejected.
func TestGetWorkflowRejectsLegacyVersions(t *testing.T) {
	repository := NewFSWorkflowRepository()

	baseDir := persistenceTestBaseDir(t)
	workflowPath := filepath.Join(baseDir, ".sdd", "workflows", "fast-change.workflow.yaml")
	replaceInFile(t, workflowPath, `schema_version: "1.0"`, `schema_version: "0.1"`)
	_, err := repository.GetWorkflow(baseDir, "fast-change")
	if !errors.Is(err, domain.ErrSchemaValidation) || !strings.Contains(err.Error(), "workflow fast-change") {
		t.Fatalf("GetWorkflow() error = %v, want explicit version error", err)
	}

	baseDir = persistenceTestBaseDir(t)
	templatePath := filepath.Join(baseDir, ".sdd", "templates", "plan.md")
	replaceInFile(t, templatePath, `schema_version: "1.0"`, `schema_version: "0.1"`)
	_, err = repository.GetWorkflow(baseDir, "fast-change")
	if err == nil || !strings.Contains(err.Error(), "template plan") {
		t.Fatalf("GetWorkflow() error = %v, want legacy template error", err)
	}
}

// The artifact metadata guard must not depend on the project's artifact.schema.json.
func TestValidateArtifactMetadataRejectsLegacyVersion(t *testing.T) {
	legacy := []byte("schema_version: \"0.1\"\nid: plan\nphase: plan\nwork_item: item\n")
	err := validateArtifactMetadata(legacy, "plan", "plan", "item")
	if !errors.Is(err, domain.ErrSchemaValidation) || !strings.Contains(err.Error(), `"0.1"`) {
		t.Fatalf("validateArtifactMetadata() error = %v, want explicit version error", err)
	}
	current := []byte("schema_version: \"1.0\"\nid: plan\nphase: plan\nwork_item: item\n")
	if err := validateArtifactMetadata(current, "plan", "plan", "item"); err != nil {
		t.Fatalf("validateArtifactMetadata() current error = %v", err)
	}
}

// T6: events.jsonl may mix 0.1 and 1.0 lines.
func TestInspectWorkItemAcceptsMixedEventVersions(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := NewFSWorkItemRepository()
	item := persistInitialTestItem(t, baseDir, repository)
	events := filepath.Join(baseDir, ".sdd", "work-items", "active", item.ID, "events.jsonl")
	replaceInFile(t, events, `"schema_version":"1.0"`, `"schema_version":"0.1"`)

	checks, err := NewFSValidationInspector().InspectWorkItem(baseDir, item.ID)
	if err != nil {
		t.Fatalf("InspectWorkItem() error = %v", err)
	}
	for _, check := range checks {
		if check.Status == domain.CheckFailed && check.Category == "event" {
			t.Fatalf("unexpected failed event check = %#v", check)
		}
	}
	if len(findChecks(checks, "event.schema_valid")) == 0 {
		t.Fatal("event.schema_valid was not evaluated")
	}
}

// T7: validate reports legacy files through *.schema_version_supported checks.
func TestInspectorReportsLegacySchemaVersions(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := NewFSWorkItemRepository()
	item := persistInitialTestItem(t, baseDir, repository)
	inspector := NewFSValidationInspector()

	manifest := filepath.Join(baseDir, ".sdd", "work-items", "active", item.ID, "manifest.yaml")
	replaceInFile(t, manifest, `schema_version: "1.0"`, `schema_version: "0.1"`)
	checks, err := inspector.InspectWorkItem(baseDir, item.ID)
	if err != nil {
		t.Fatalf("InspectWorkItem() error = %v", err)
	}
	assertCheckFailed(t, checks, "manifest.schema_version_supported")

	replaceInFile(t, filepath.Join(baseDir, ".sdd", "workflows", "fast-change.workflow.yaml"), `schema_version: "1.0"`, `schema_version: "0.1"`)
	replaceInFile(t, filepath.Join(baseDir, ".sdd", "config.yaml"), `schema_version: "1.0"`, `schema_version: "0.1"`)
	checks, err = inspector.InspectProject(baseDir)
	if err != nil {
		t.Fatalf("InspectProject() error = %v", err)
	}
	failedWorkflow := 0
	for _, check := range findChecks(checks, "workflow.schema_version_supported") {
		if check.Status == domain.CheckFailed && strings.Contains(check.Target, "fast-change") {
			failedWorkflow++
		}
	}
	if failedWorkflow != 1 {
		t.Fatalf("failed workflow.schema_version_supported for fast-change = %d, want 1", failedWorkflow)
	}
	assertCheckFailed(t, checks, "config.schema_version_supported")
	if message := findChecks(checks, "config.schema_version_supported")[0].Message; !strings.Contains(message, `requires "1.0"`) {
		t.Fatalf("config check message = %q", message)
	}
}
