package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/usecases"
)

type staticValidationInspector struct {
	checks []domain.ValidationCheck
}

func (inspector staticValidationInspector) InspectProject(_ string) ([]domain.ValidationCheck, error) {
	return inspector.checks, nil
}

func (inspector staticValidationInspector) InspectWorkItem(_, _ string) ([]domain.ValidationCheck, error) {
	return inspector.checks, nil
}

func TestArgumentErrorsUseJSONEnvelope(t *testing.T) {
	root := NewRootCommand(Application{})
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--json", "status"})

	if err := executeRoot(root); err == nil {
		t.Fatal("executeRoot() error = nil")
	}
	var response JSONResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v; output = %s", err, stdout.String())
	}
	if response.Success || response.Error == nil || response.Error.Code != "invalid_arguments" {
		t.Fatalf("response = %#v", response)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRequiredFlagErrorsUseJSONEnvelope(t *testing.T) {
	root := NewRootCommand(Application{})
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--json", "start", "missing-title"})

	if err := executeRoot(root); err == nil {
		t.Fatal("executeRoot() error = nil")
	}
	var response JSONResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v; output = %s", err, stdout.String())
	}
	if response.Error == nil || response.Error.Code != "invalid_arguments" {
		t.Fatalf("response = %#v", response)
	}
}

func TestRejectRequiredPhaseUsesJSONEnvelope(t *testing.T) {
	root := NewRootCommand(Application{})
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--json", "reject", "item-1"})

	if err := executeRoot(root); err == nil {
		t.Fatal("executeRoot() error = nil")
	}
	var response JSONResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v; output = %s", err, stdout.String())
	}
	if response.Success || response.Error == nil || response.Error.Code != "invalid_arguments" {
		t.Fatalf("response = %#v", response)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestValidateFailureUsesJSONDetails(t *testing.T) {
	application := Application{
		Validate: usecases.NewValidateUseCase(staticValidationInspector{
			checks: []domain.ValidationCheck{{
				Status: domain.CheckFailed, Category: "artifact", Code: "artifact.missing", Target: "artifact.md", Message: "missing",
			}},
		}),
	}
	root := NewRootCommand(application)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--json", "validate"})

	if err := executeRoot(root); err == nil {
		t.Fatal("executeRoot() error = nil")
	}
	var response JSONResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v; output = %s", err, stdout.String())
	}
	if response.Success || response.Error == nil || response.Error.Code != "validation_failed" || response.Error.Details == nil {
		t.Fatalf("response = %#v", response)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestValidateRejectsMultipleTargets(t *testing.T) {
	root := NewRootCommand(Application{})
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--json", "validate", "one", "two"})

	if err := executeRoot(root); err == nil {
		t.Fatal("executeRoot() error = nil")
	}
	var response JSONResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v; output = %s", err, stdout.String())
	}
	if response.Error == nil || response.Error.Code != "invalid_arguments" {
		t.Fatalf("response = %#v", response)
	}
}

func TestArchiveRequiresExactlyOneTarget(t *testing.T) {
	root := NewRootCommand(Application{})
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--json", "archive"})

	if err := executeRoot(root); err == nil {
		t.Fatal("executeRoot() error = nil")
	}
	var response JSONResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v; output = %s", err, stdout.String())
	}
	if response.Error == nil || response.Error.Code != "invalid_arguments" {
		t.Fatalf("response = %#v", response)
	}
}

func TestArchiveErrorsUseStableCodes(t *testing.T) {
	if got := errorCode(domain.ErrWorkItemAlreadyArchived); got != "already_archived" {
		t.Fatalf("already archived code = %q", got)
	}
	if got := errorCode(domain.ErrArchiveConflict); got != "archive_conflict" {
		t.Fatalf("archive conflict code = %q", got)
	}
	if got := errorCode(domain.ErrWorkItemCannotArchive); got != "invalid_transition" {
		t.Fatalf("cannot archive code = %q", got)
	}
	if got := errorCode(domain.ErrAdapterNotFound); got != "not_found" {
		t.Fatalf("adapter not found code = %q", got)
	}
	if got := errorCode(domain.ErrAdapterInstallConflict); got != "already_exists" {
		t.Fatalf("adapter conflict code = %q", got)
	}
}
