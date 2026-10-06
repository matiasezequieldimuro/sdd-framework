package domain_test

import (
	"errors"
	"testing"

	"sdd-cli/internal/domain"
)

// minimalWorkflow returns a fast-change-like workflow with a single phase
// sufficient for NewWorkItem in unit tests.
func minimalWorkflow(t *testing.T) *domain.Workflow {
	t.Helper()
	wf := &domain.Workflow{
		SchemaVersion: "0.1",
		ID:            "fast-change",
		WorkItemType:  "change",
		Phases: []domain.WorkflowPhase{
			{ID: "plan", Requires: nil, Approval: domain.ApprovalRequired},
		},
	}
	return wf
}

func newTestItem(t *testing.T, auditActive bool) *domain.WorkItem {
	t.Helper()
	wf := minimalWorkflow(t)
	item, _, err := domain.NewWorkItem(wf, domain.NewWorkItemParams{
		ID:               "test-item",
		Title:            "Test item",
		Summary:          "unit test",
		EntryPhase:       "plan",
		CreatedAt:        "2026-01-01T00:00:00Z",
		CreatedBy:        domain.Actor{Kind: "human", ID: "tester"},
		TokenAuditActive: auditActive,
	})
	if err != nil {
		t.Fatalf("NewWorkItem() error = %v", err)
	}
	return item
}

// TestNewWorkItem_ActiveAudit_InitialisesCountersToZero verifies E-02: a work
// item created with an active audit has status "partial" and all counters = 0.
func TestNewWorkItem_ActiveAudit_InitialisesCountersToZero(t *testing.T) {
	item := newTestItem(t, true)

	if item.Observability == nil {
		t.Fatal("Observability is nil, want non-nil")
	}
	tu := item.Observability.TokenUsage
	if tu == nil {
		t.Fatal("TokenUsage is nil, want non-nil")
	}
	if tu.Status != "partial" {
		t.Errorf("Status = %q, want %q", tu.Status, "partial")
	}
	if tu.Source != nil {
		t.Errorf("Source = %v, want nil", tu.Source)
	}
	for name, ptr := range map[string]*int{
		"InputTokens":      tu.InputTokens,
		"OutputTokens":     tu.OutputTokens,
		"CacheReadTokens":  tu.CacheReadTokens,
		"CacheWriteTokens": tu.CacheWriteTokens,
		"TotalTokens":      tu.TotalTokens,
	} {
		if ptr == nil {
			t.Errorf("%s is nil, want pointer to 0", name)
		} else if *ptr != 0 {
			t.Errorf("%s = %d, want 0", name, *ptr)
		}
	}
}

// TestNewWorkItem_InactiveAudit_InitialisesNotReported verifies E-03: a work
// item created with an inactive audit has status "not_reported" and nil counters.
func TestNewWorkItem_InactiveAudit_InitialisesNotReported(t *testing.T) {
	item := newTestItem(t, false)

	if item.Observability == nil {
		t.Fatal("Observability is nil, want non-nil")
	}
	tu := item.Observability.TokenUsage
	if tu == nil {
		t.Fatal("TokenUsage is nil, want non-nil")
	}
	if tu.Status != "not_reported" {
		t.Errorf("Status = %q, want %q", tu.Status, "not_reported")
	}
	for name, ptr := range map[string]*int{
		"InputTokens":      tu.InputTokens,
		"OutputTokens":     tu.OutputTokens,
		"CacheReadTokens":  tu.CacheReadTokens,
		"CacheWriteTokens": tu.CacheWriteTokens,
		"TotalTokens":      tu.TotalTokens,
	} {
		if ptr != nil {
			t.Errorf("%s = %d, want nil", name, *ptr)
		}
	}
}

// TestRecordTokenUsage_Success verifies E-01: counters are overwritten and
// total_tokens equals the sum of the four counters (D-a invariant).
func TestRecordTokenUsage_Success(t *testing.T) {
	item := newTestItem(t, true)

	if err := item.RecordTokenUsage(500, 300, 100, 50, "claude-code"); err != nil {
		t.Fatalf("RecordTokenUsage() unexpected error = %v", err)
	}
	tu := item.Observability.TokenUsage
	if tu.Status != "recorded" {
		t.Errorf("Status = %q, want recorded", tu.Status)
	}
	if tu.Source == nil || *tu.Source != "claude-code" {
		t.Errorf("Source = %v, want claude-code", tu.Source)
	}
	wantTotal := 500 + 300 + 100 + 50
	checks := map[string]struct {
		got  *int
		want int
	}{
		"InputTokens":      {tu.InputTokens, 500},
		"OutputTokens":     {tu.OutputTokens, 300},
		"CacheReadTokens":  {tu.CacheReadTokens, 100},
		"CacheWriteTokens": {tu.CacheWriteTokens, 50},
		"TotalTokens":      {tu.TotalTokens, wantTotal},
	}
	for name, c := range checks {
		if c.got == nil {
			t.Errorf("%s is nil, want %d", name, c.want)
		} else if *c.got != c.want {
			t.Errorf("%s = %d, want %d", name, *c.got, c.want)
		}
	}
}

// TestRecordTokenUsage_Overwrite verifies that a second recording fully
// overwrites the first (RC-2 / REQ-3).
func TestRecordTokenUsage_Overwrite(t *testing.T) {
	item := newTestItem(t, true)
	if err := item.RecordTokenUsage(100, 50, 0, 0, "claude-code"); err != nil {
		t.Fatalf("first RecordTokenUsage() error = %v", err)
	}
	if err := item.RecordTokenUsage(200, 80, 20, 10, "claude-code"); err != nil {
		t.Fatalf("second RecordTokenUsage() error = %v", err)
	}
	tu := item.Observability.TokenUsage
	if tu.InputTokens == nil || *tu.InputTokens != 200 {
		t.Errorf("InputTokens after overwrite = %v, want 200", tu.InputTokens)
	}
	expectedTotal := 200 + 80 + 20 + 10
	if tu.TotalTokens == nil || *tu.TotalTokens != expectedTotal {
		t.Errorf("TotalTokens after overwrite = %v, want %d", tu.TotalTokens, expectedTotal)
	}
}

// TestRecordTokenUsage_AllZero verifies E-09: recording all counters as 0 is valid.
func TestRecordTokenUsage_AllZero(t *testing.T) {
	item := newTestItem(t, true)
	if err := item.RecordTokenUsage(0, 0, 0, 0, "claude-code"); err != nil {
		t.Fatalf("RecordTokenUsage() with zeros: unexpected error = %v", err)
	}
	tu := item.Observability.TokenUsage
	if tu.TotalTokens == nil || *tu.TotalTokens != 0 {
		t.Errorf("TotalTokens = %v, want 0", tu.TotalTokens)
	}
	if tu.Status != "recorded" {
		t.Errorf("Status = %q, want recorded", tu.Status)
	}
}

// TestRecordTokenUsage_InactiveAudit verifies E-12: recording on an inactive
// item returns ErrTokenAuditInactive and leaves counters untouched.
func TestRecordTokenUsage_InactiveAudit(t *testing.T) {
	item := newTestItem(t, false)
	err := item.RecordTokenUsage(100, 50, 0, 0, "claude-code")
	if !errors.Is(err, domain.ErrTokenAuditInactive) {
		t.Fatalf("RecordTokenUsage() error = %v, want ErrTokenAuditInactive", err)
	}
	// Counters must remain nil.
	tu := item.Observability.TokenUsage
	if tu.InputTokens != nil {
		t.Errorf("InputTokens = %d after rejected recording, want nil", *tu.InputTokens)
	}
	if tu.Status != "not_reported" {
		t.Errorf("Status = %q after rejected recording, want not_reported", tu.Status)
	}
}

// TestRecordTokenUsage_NegativeValues verifies E-06: negative counters are
// rejected with ErrValidationFailed.
func TestRecordTokenUsage_NegativeValues(t *testing.T) {
	cases := []struct {
		name                              string
		in, out, cacheRead, cacheWrite    int
	}{
		{"negative input", -1, 0, 0, 0},
		{"negative output", 0, -1, 0, 0},
		{"negative cache_read", 0, 0, -1, 0},
		{"negative cache_write", 0, 0, 0, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := newTestItem(t, true)
			err := item.RecordTokenUsage(tc.in, tc.out, tc.cacheRead, tc.cacheWrite, "claude-code")
			if !errors.Is(err, domain.ErrValidationFailed) {
				t.Errorf("RecordTokenUsage() error = %v, want ErrValidationFailed", err)
			}
		})
	}
}

// TestRecordTokenUsage_EmptySource verifies that an empty source is rejected.
func TestRecordTokenUsage_EmptySource(t *testing.T) {
	item := newTestItem(t, true)
	err := item.RecordTokenUsage(10, 5, 0, 0, "")
	if !errors.Is(err, domain.ErrValidationFailed) {
		t.Errorf("RecordTokenUsage() with empty source: error = %v, want ErrValidationFailed", err)
	}
}
