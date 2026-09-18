package infra

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"sdd-cli/internal/domain"
)

func TestContractFixtures(t *testing.T) {
	baseDir := contractBaseDir(t)
	validator := NewSchemaValidator()
	workflowRepo := NewFSWorkflowRepository()

	t.Run("valid fixtures", func(t *testing.T) {
		fixtures := fixtureFiles(t, filepath.Join(baseDir, ".sdd", "tests", "fixtures", "valid"))
		for _, fixturePath := range fixtures {
			t.Run(filepath.Base(fixturePath), func(t *testing.T) {
				item := readFixture(t, fixturePath)
				if err := validator.ValidateValue(baseDir, "work-item.schema.json", item); err != nil {
					t.Fatalf("schema validation failed: %v", err)
				}
				workflow, err := workflowRepo.GetWorkflow(baseDir, item.Workflow.ID)
				if err != nil {
					t.Fatalf("GetWorkflow() error = %v", err)
				}
				if err := item.ValidateAgainst(workflow); err != nil {
					t.Fatalf("semantic validation failed: %v", err)
				}
			})
		}
	})

	t.Run("invalid fixtures", func(t *testing.T) {
		fixtures := fixtureFiles(t, filepath.Join(baseDir, ".sdd", "tests", "fixtures", "invalid"))
		for _, fixturePath := range fixtures {
			t.Run(filepath.Base(fixturePath), func(t *testing.T) {
				item := readFixture(t, fixturePath)
				err := validator.ValidateValue(baseDir, "work-item.schema.json", item)
				if err == nil {
					workflow, workflowErr := workflowRepo.GetWorkflow(baseDir, item.Workflow.ID)
					if workflowErr != nil {
						err = workflowErr
					} else {
						err = item.ValidateAgainst(workflow)
					}
				}
				if err == nil {
					t.Fatal("invalid fixture passed structural and semantic validation")
				}
			})
		}
	})
}

func TestSchemaValidatorReturnsAllLeafViolations(t *testing.T) {
	validator := NewSchemaValidator()
	violations, err := validator.ValidateValueAll(
		contractBaseDir(t),
		"workflow.schema.json",
		map[string]interface{}{
			"schema_version": "9",
			"kind":           "invalid",
			"id":             "INVALID",
			"title":          "",
			"work_item_type": "",
			"entry_points":   []interface{}{},
			"phases":         []interface{}{},
			"artifacts":      map[string]interface{}{},
		},
	)
	if err != nil {
		t.Fatalf("ValidateValueAll() error = %v", err)
	}
	if len(violations) < 2 {
		t.Fatalf("violations = %#v", violations)
	}
	for index := 1; index < len(violations); index++ {
		if violations[index-1].InstancePath > violations[index].InstancePath {
			t.Fatalf("violations are not ordered: %#v", violations)
		}
	}
	for _, schemaFile := range requiredSchemas {
		if err := validator.Compile(contractBaseDir(t), schemaFile); err != nil {
			t.Fatalf("Compile(%s) error = %v", schemaFile, err)
		}
	}
}

func TestValidationInspectorDoesNotRecoverTransactions(t *testing.T) {
	baseDir := t.TempDir()
	if err := NewFSProjectInitializer().Initialize(baseDir); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, ".sdd", "work-items", "active", ".gitkeep"), nil, 0644); err != nil {
		t.Fatalf("WriteFile() .gitkeep error = %v", err)
	}
	backupPath := filepath.Join(baseDir, ".sdd", "work-items", ".transactions", "unrelated.backup")
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	markerPath := filepath.Join(backupPath, "marker")
	if err := os.WriteFile(markerPath, []byte("preserve"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	checks, err := NewFSValidationInspector().InspectProject(baseDir)
	if err != nil {
		t.Fatalf("InspectProject() error = %v", err)
	}
	for _, check := range checks {
		if check.Status == domain.CheckFailed {
			t.Fatalf("unexpected failed check = %#v", check)
		}
	}
	data, err := os.ReadFile(markerPath)
	if err != nil || string(data) != "preserve" {
		t.Fatalf("transaction marker changed: data=%q err=%v", data, err)
	}
}

func TestValidationInspectorValidatesCapabilityProcedureReferences(t *testing.T) {
	baseDir := t.TempDir()
	if err := NewFSProjectInitializer().Initialize(baseDir); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	registryPath := filepath.Join(baseDir, ".sdd", "registry", "capabilities.yaml")
	registry := `schema_version: "0.1"
capabilities:
  - id: sdd.missing-procedure
    title: Missing procedure
    procedure: procedures/missing.md
`
	if err := os.WriteFile(registryPath, []byte(registry), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	checks, err := NewFSValidationInspector().InspectProject(baseDir)
	if err != nil {
		t.Fatalf("InspectProject() error = %v", err)
	}
	for _, check := range checks {
		if check.Code == "registry.procedure_file_exists" && check.Status == domain.CheckFailed {
			return
		}
	}
	t.Fatalf("missing capability procedure was not reported: %#v", checks)
}

func TestContainedPathRejectsTraversalAndSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	if _, err := containedPath(root, "..", "outside"); !errors.Is(err, domain.ErrInvalidPath) {
		t.Fatalf("containedPath() traversal error = %v, want %v", err, domain.ErrInvalidPath)
	}

	outside := t.TempDir()
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	if _, err := containedPath(root, "link", "file.md"); !errors.Is(err, domain.ErrInvalidPath) {
		t.Fatalf("containedPath() symlink error = %v, want %v", err, domain.ErrInvalidPath)
	}
}

func TestRepositoryRejectsInvalidIdentifier(t *testing.T) {
	repository := NewFSWorkItemRepository()
	if _, err := repository.WorkItemExists(t.TempDir(), "../../../escape"); !errors.Is(err, domain.ErrInvalidIdentifier) {
		t.Fatalf("WorkItemExists() error = %v, want %v", err, domain.ErrInvalidIdentifier)
	}
}

func TestRepositoryRejectsSymlinkedWorkItemsRoot(t *testing.T) {
	baseDir := t.TempDir()
	sddDir := filepath.Join(baseDir, ".sdd")
	if err := os.MkdirAll(filepath.Join(sddDir, "work-items"), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(sddDir, "work-items", "active")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	if _, err := NewFSWorkItemRepository().WorkItemExists(baseDir, "safe-id"); !errors.Is(err, domain.ErrInvalidPath) {
		t.Fatalf("WorkItemExists() error = %v, want %v", err, domain.ErrInvalidPath)
	}
}

func contractBaseDir(t *testing.T) string {
	t.Helper()
	baseDir, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}
	return baseDir
}

func fixtureFiles(t *testing.T, directory string) []string {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	var files []string
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			files = append(files, filepath.Join(directory, entry.Name()))
		}
	}
	if len(files) == 0 {
		t.Fatalf("no JSON fixtures found in %s", directory)
	}
	return files
}

func readFixture(t *testing.T, path string) *domain.WorkItem {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var item domain.WorkItem
	if err := json.Unmarshal(data, &item); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	return &item
}
