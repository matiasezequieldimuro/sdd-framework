package usecases_test

import (
	"errors"
	"os"
	"testing"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/infra"
	"sdd-cli/internal/usecases"
)

// setupTokenTestEnv creates a temp project with a single fast-change work item
// and returns the base dir and work item ID. The caller controls audit
// activation via the config.yaml in the project.
func setupTokenTestEnv(t *testing.T, auditActive bool) (baseDir, itemID string) {
	t.Helper()
	baseDir = setupTestEnv(t)
	t.Cleanup(func() { os.RemoveAll(baseDir) })
	itemID = "token-test-item"

	// Patch config.yaml: activate or deactivate audit as requested.
	// Handles both old ("optional") and new ("false") template defaults.
	configPath := baseDir + "/.sdd/config.yaml"
	content, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("ReadFile config: %v", readErr)
	}
	updated := string(content)
	if auditActive {
		updated = replaceInString(updated, "token_usage: false", "token_usage: true")
		updated = replaceInString(updated, "token_usage: optional", "token_usage: true")
	}
	if err := os.WriteFile(configPath, []byte(updated), 0644); err != nil {
		t.Fatalf("WriteFile config: %v", err)
	}

	actor := domain.Actor{Kind: domain.ActorHuman, ID: "tester"}
	_, err := usecases.NewStartWorkItemUseCase(
		infra.NewFSWorkItemRepository(),
		infra.NewFSWorkflowRepository(),
		infra.NewFSConfigRepository(),
		infra.NewArtifactManager(),
		infra.NewSystemClock(),
		infra.NewCryptoIDGenerator(),
	).Execute(baseDir, usecases.StartWorkItemInput{
		ID:         itemID,
		WorkflowID: "fast-change",
		Title:      "Token test item",
		Actor:      actor,
	})
	if err != nil {
		t.Fatalf("StartWorkItemUseCase.Execute() error = %v", err)
	}
	return baseDir, itemID
}

// replaceInString replaces the first occurrence of old with new in s.
func replaceInString(s, old, replacement string) string {
	idx := 0
	for i := 0; i <= len(s)-len(old); i++ {
		if s[i:i+len(old)] == old {
			idx = i
			return s[:idx] + replacement + s[idx+len(old):]
		}
	}
	return s
}

func newRecordTokensUC() *usecases.RecordTokensUseCase {
	return usecases.NewRecordTokensUseCase(
		infra.NewFSWorkItemRepository(),
		infra.NewSystemClock(),
		infra.NewCryptoIDGenerator(),
	)
}

// TestRecordTokensUseCase_Success verifies the happy path (E-01): counters are
// persisted and total is the sum of the four inputs.
func TestRecordTokensUseCase_Success(t *testing.T) {
	baseDir, itemID := setupTokenTestEnv(t, true)
	uc := newRecordTokensUC()

	err := uc.Execute(baseDir, usecases.RecordTokensInput{
		WorkItemID:       itemID,
		InputTokens:      500,
		OutputTokens:     300,
		CacheReadTokens:  100,
		CacheWriteTokens: 50,
		Source:           "claude-code",
		Actor:            domain.Actor{Kind: domain.ActorAgent, ID: "agent"},
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	// Verify persistence.
	item, err := infra.NewFSWorkItemRepository().GetWorkItem(baseDir, itemID)
	if err != nil {
		t.Fatalf("GetWorkItem() error = %v", err)
	}
	tu := item.Observability.TokenUsage
	if tu.Status != "recorded" {
		t.Errorf("Status = %q, want recorded", tu.Status)
	}
	wantTotal := 950
	if tu.TotalTokens == nil || *tu.TotalTokens != wantTotal {
		t.Errorf("TotalTokens = %v, want %d", tu.TotalTokens, wantTotal)
	}
}

// TestRecordTokensUseCase_Idempotent verifies RC-9: replaying with the same
// operation-id does not duplicate the event.
func TestRecordTokensUseCase_Idempotent(t *testing.T) {
	baseDir, itemID := setupTokenTestEnv(t, true)
	uc := newRecordTokensUC()
	actor := domain.Actor{Kind: domain.ActorAgent, ID: "agent"}

	input := usecases.RecordTokensInput{
		WorkItemID:       itemID,
		InputTokens:      100,
		OutputTokens:     50,
		CacheReadTokens:  0,
		CacheWriteTokens: 0,
		Source:           "claude-code",
		Actor:            actor,
		OperationID:      "tokens:ses001:5",
	}

	if err := uc.Execute(baseDir, input); err != nil {
		t.Fatalf("first Execute() error = %v", err)
	}
	// Count events after first call.
	eventsPath := baseDir + "/.sdd/work-items/active/" + itemID + "/events.jsonl"
	first, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("ReadFile() events error = %v", err)
	}

	// Replay with same operation-id.
	if err := uc.Execute(baseDir, input); err != nil {
		t.Fatalf("replayed Execute() error = %v", err)
	}
	second, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("ReadFile() events after replay error = %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("replaying with same operation-id added a duplicate event")
	}
}

// TestRecordTokensUseCase_InactiveAudit verifies E-12: executing on an item
// with inactive audit returns ErrTokenAuditInactive.
func TestRecordTokensUseCase_InactiveAudit(t *testing.T) {
	baseDir, itemID := setupTokenTestEnv(t, false)
	uc := newRecordTokensUC()

	err := uc.Execute(baseDir, usecases.RecordTokensInput{
		WorkItemID:       itemID,
		InputTokens:      100,
		OutputTokens:     50,
		CacheReadTokens:  0,
		CacheWriteTokens: 0,
		Source:           "claude-code",
		Actor:            domain.Actor{Kind: domain.ActorAgent, ID: "agent"},
	})
	if !errors.Is(err, domain.ErrTokenAuditInactive) {
		t.Fatalf("Execute() error = %v, want ErrTokenAuditInactive", err)
	}
}
