package usecases_test

import (
	"errors"
	"testing"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/usecases"
)

type memoryValidationInspector struct {
	projectChecks []domain.ValidationCheck
	itemChecks    []domain.ValidationCheck
	err           error
	projectCalls  int
	itemCalls     int
}

func (inspector *memoryValidationInspector) InspectProject(_ string) ([]domain.ValidationCheck, error) {
	inspector.projectCalls++
	return inspector.projectChecks, inspector.err
}

func (inspector *memoryValidationInspector) InspectWorkItem(_, _ string) ([]domain.ValidationCheck, error) {
	inspector.itemCalls++
	return inspector.itemChecks, inspector.err
}

func TestValidateUseCaseBuildsDeterministicSummary(t *testing.T) {
	source := []domain.ValidationCheck{
		{Status: domain.CheckWarning, Category: "reference", Code: "reference.warning", Target: "z", Message: "warning"},
		{Status: domain.CheckFailed, Category: "artifact", Code: "artifact.failed", Target: "a", Message: "failed"},
		{Status: domain.CheckPassed, Category: "manifest", Code: "manifest.passed", Target: "a", Message: "passed"},
	}
	inspector := &memoryValidationInspector{projectChecks: source}
	report, err := usecases.NewValidateUseCase(inspector).Execute("project", "")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if inspector.projectCalls != 1 || inspector.itemCalls != 0 {
		t.Fatalf("calls = project:%d item:%d", inspector.projectCalls, inspector.itemCalls)
	}
	if report.Scope != usecases.ValidationScopeProject || report.Target != "." || report.Valid {
		t.Fatalf("report = %#v", report)
	}
	if report.Summary.Total != 3 || report.Summary.Passed != 1 ||
		report.Summary.Warnings != 1 || report.Summary.Failed != 1 {
		t.Fatalf("summary = %#v", report.Summary)
	}
	if report.Checks[0].Target != "a" || report.Checks[0].Category != "artifact" {
		t.Fatalf("checks are not ordered: %#v", report.Checks)
	}
	if source[0].Target != "z" {
		t.Fatal("Execute() mutated adapter checks")
	}
}

func TestValidateUseCaseSelectsWorkItemScope(t *testing.T) {
	inspector := &memoryValidationInspector{
		itemChecks: []domain.ValidationCheck{{
			Status: domain.CheckPassed, Category: "manifest", Code: "manifest.valid", Target: "manifest.yaml", Message: "valid",
		}},
	}
	report, err := usecases.NewValidateUseCase(inspector).Execute("project", "valid-item")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if inspector.itemCalls != 1 || inspector.projectCalls != 0 {
		t.Fatalf("calls = project:%d item:%d", inspector.projectCalls, inspector.itemCalls)
	}
	if report.Scope != usecases.ValidationScopeWorkItem || report.Target != "valid-item" || !report.Valid {
		t.Fatalf("report = %#v", report)
	}
}

func TestValidateUseCaseRejectsInvalidIDBeforeInspector(t *testing.T) {
	inspector := &memoryValidationInspector{}
	if _, err := usecases.NewValidateUseCase(inspector).Execute("project", "../invalid"); !errors.Is(err, domain.ErrInvalidIdentifier) {
		t.Fatalf("Execute() error = %v, want %v", err, domain.ErrInvalidIdentifier)
	}
	if inspector.itemCalls != 0 {
		t.Fatalf("item calls = %d", inspector.itemCalls)
	}
}

func TestValidateUseCasePropagatesInspectorFailure(t *testing.T) {
	adapterErr := errors.New("adapter failure")
	inspector := &memoryValidationInspector{err: adapterErr}
	if _, err := usecases.NewValidateUseCase(inspector).Execute("project", "valid-item"); !errors.Is(err, adapterErr) {
		t.Fatalf("Execute() error = %v, want %v", err, adapterErr)
	}
}
