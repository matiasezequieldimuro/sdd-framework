package infra

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

// Crash-recovery and rollback coverage for the transactional cancel (CA-CA-08).

func TestCancelWorkItemPublishesSnapshotAndCleansTransaction(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := NewFSWorkItemRepository()
	item := persistedCancelCandidate(t, baseDir, repository)
	paths := cancelTestPaths(baseDir, item.ID)

	if err := item.Cancel(); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	result, err := repository.CancelWorkItem(baseDir, cancelCommit(item, "cancel-operation"))
	if err != nil {
		t.Fatalf("CancelWorkItem() error = %v", err)
	}
	if result.Location != ports.WorkItemLocationCancelled || result.Item.Revision != 2 {
		t.Fatalf("cancel result = %#v", result)
	}
	if _, err := os.Stat(paths.active); !os.IsNotExist(err) {
		t.Fatalf("active path still exists: %v", err)
	}
	events, err := os.ReadFile(filepath.Join(paths.destination, "events.jsonl"))
	if err != nil {
		t.Fatalf("ReadFile() events error = %v", err)
	}
	if strings.Count(string(events), `"type":"work_item.cancelled"`) != 1 {
		t.Fatalf("events = %s", events)
	}
	assertCancelTransactionClean(t, paths)
}

func TestCancelWorkItemRollsBackAfterActiveBackupFailure(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := NewFSWorkItemRepository()
	item := persistedCancelCandidate(t, baseDir, repository)
	paths := cancelTestPaths(baseDir, item.ID)
	if err := item.Cancel(); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	repository.commitHook = failAtStage("cancel_after_active_backup")

	if _, err := repository.CancelWorkItem(baseDir, cancelCommit(item, "failed-cancel")); err == nil {
		t.Fatal("CancelWorkItem() succeeded despite injected failure")
	}
	assertActiveRestored(t, baseDir, repository, item.ID)
	if _, err := os.Stat(paths.destination); !os.IsNotExist(err) {
		t.Fatalf("cancelled destination exists after rollback: %v", err)
	}
	assertCancelTransactionClean(t, paths)
}

func TestCancelWorkItemRollsBackAfterDestinationPublishFailure(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := NewFSWorkItemRepository()
	item := persistedCancelCandidate(t, baseDir, repository)
	paths := cancelTestPaths(baseDir, item.ID)
	if err := item.Cancel(); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	repository.commitHook = failAtStage("cancel_after_destination_publish")

	if _, err := repository.CancelWorkItem(baseDir, cancelCommit(item, "failed-publish")); err == nil {
		t.Fatal("CancelWorkItem() succeeded despite injected failure")
	}
	assertActiveRestored(t, baseDir, repository, item.ID)
	if _, err := os.Stat(paths.destination); !os.IsNotExist(err) {
		t.Fatalf("cancelled destination exists after rollback: %v", err)
	}
	assertCancelTransactionClean(t, paths)
}

// Crash after the marker was written but before active was moved to backup.
func TestGetWorkItemRecoversInterruptedCancelBeforeBackup(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := NewFSWorkItemRepository()
	item := persistedCancelCandidate(t, baseDir, repository)
	paths := cancelTestPaths(baseDir, item.ID)
	if err := os.MkdirAll(paths.stage, 0755); err != nil {
		t.Fatalf("MkdirAll() stage error = %v", err)
	}
	writeCancelMarker(t, paths, item.ID, "interrupted-before-backup")

	assertActiveRestored(t, baseDir, repository, item.ID)
	assertCancelTransactionClean(t, paths)
}

// Crash after active was moved to backup but before the stage was published.
func TestGetWorkItemRecoversInterruptedCancelBeforePublish(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := NewFSWorkItemRepository()
	item := persistedCancelCandidate(t, baseDir, repository)
	paths := cancelTestPaths(baseDir, item.ID)
	if err := os.MkdirAll(paths.stage, 0755); err != nil {
		t.Fatalf("MkdirAll() stage error = %v", err)
	}
	if err := os.Rename(paths.active, paths.backup); err != nil {
		t.Fatalf("Rename() backup error = %v", err)
	}
	writeCancelMarker(t, paths, item.ID, "interrupted-before-publish")

	assertActiveRestored(t, baseDir, repository, item.ID)
	assertCancelTransactionClean(t, paths)
}

// Crash after the stage was published but before backup and marker cleanup.
func TestFindCancelledWorkItemRecoversInterruptedCancelAfterPublish(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := NewFSWorkItemRepository()
	item := persistedCancelCandidate(t, baseDir, repository)
	paths := cancelTestPaths(baseDir, item.ID)
	if err := item.Cancel(); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if _, err := repository.CancelWorkItem(baseDir, cancelCommit(item, "published-cancel")); err != nil {
		t.Fatalf("CancelWorkItem() error = %v", err)
	}
	// Re-create the leftovers of a crash between publish and cleanup.
	if err := copyDirectory(paths.destination, paths.backup); err != nil {
		t.Fatalf("copyDirectory() backup error = %v", err)
	}
	writeCancelMarker(t, paths, item.ID, "published-cancel")

	found, err := repository.FindCancelledWorkItem(baseDir, item.ID)
	if err != nil {
		t.Fatalf("FindCancelledWorkItem() recovery error = %v", err)
	}
	if found.Item.Status != domain.WorkItemCancelled || found.Location != ports.WorkItemLocationCancelled {
		t.Fatalf("recovered cancelled item = %#v", found)
	}
	if _, err := os.Stat(paths.active); !os.IsNotExist(err) {
		t.Fatalf("active path exists after recovery: %v", err)
	}
	assertCancelTransactionClean(t, paths)
}

// A published cancel whose events lack the marker operation is a conflict, not a silent cleanup.
func TestFindCancelledWorkItemRejectsPublishedCancelMissingOperation(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := NewFSWorkItemRepository()
	item := persistedCancelCandidate(t, baseDir, repository)
	paths := cancelTestPaths(baseDir, item.ID)
	if err := item.Cancel(); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if _, err := repository.CancelWorkItem(baseDir, cancelCommit(item, "real-operation")); err != nil {
		t.Fatalf("CancelWorkItem() error = %v", err)
	}
	writeCancelMarker(t, paths, item.ID, "other-operation")

	if _, err := repository.FindCancelledWorkItem(baseDir, item.ID); !errors.Is(err, domain.ErrArchiveConflict) {
		t.Fatalf("FindCancelledWorkItem() error = %v, want %v", err, domain.ErrArchiveConflict)
	}
}

type cancelPaths struct {
	active, destination, stage, backup, marker string
}

func cancelTestPaths(baseDir, id string) cancelPaths {
	workItems := filepath.Join(baseDir, ".sdd", "work-items")
	transactions := filepath.Join(workItems, ".transactions")
	return cancelPaths{
		active:      filepath.Join(workItems, "active", id),
		destination: filepath.Join(workItems, "cancelled", id),
		stage:       filepath.Join(transactions, id+".cancel-stage"),
		backup:      filepath.Join(transactions, id+".cancel-backup"),
		marker:      filepath.Join(transactions, id+".cancel.json"),
	}
}

func persistedCancelCandidate(t *testing.T, baseDir string, repository *FSWorkItemRepository) *domain.WorkItem {
	t.Helper()
	item := readFixture(
		t,
		filepath.Join(contractBaseDir(t), ".sdd", "tests", "fixtures", "valid", "20-fast-change-started-at-plan.json"),
	)
	item.ID = "cancel-item"
	item.Title = "Cancel item"
	item.Revision = 0
	workflow := mustWorkflow(t, baseDir, item.Workflow.ID)
	writes, err := NewArtifactManager().PrepareArtifactsForPhase(
		baseDir,
		workflow,
		"plan",
		item.ID,
		map[string]string{
			"title":      item.Title,
			"id":         item.ID,
			"created_at": item.CreatedAt,
			"type":       item.Type,
		},
	)
	if err != nil {
		t.Fatalf("PrepareArtifactsForPhase() error = %v", err)
	}
	event := persistenceEvent(item.ID, "work_item.created", "cancel-create")
	if err := repository.CommitWorkItem(baseDir, ports.WorkItemCommit{
		Item:        item,
		Artifacts:   writes,
		Events:      []domain.Event{event},
		OperationID: event.CorrelationID,
	}); err != nil {
		t.Fatalf("CommitWorkItem() error = %v", err)
	}
	return item
}

func cancelCommit(item *domain.WorkItem, operationID string) ports.WorkItemCancelCommit {
	event := domain.NewEvent(
		"evt_"+operationID,
		time.Date(2026, time.October, 14, 0, 0, 0, 0, time.UTC),
		item.ID,
		"work_item.cancelled",
		domain.Actor{Kind: domain.ActorHuman, ID: "test"},
		map[string]interface{}{
			"from":   string(domain.WorkItemActive),
			"to":     string(domain.WorkItemCancelled),
			"reason": "No longer needed",
		},
	)
	event.CorrelationID = operationID
	return ports.WorkItemCancelCommit{Item: item, Event: event, OperationID: operationID}
}

func writeCancelMarker(t *testing.T, paths cancelPaths, id, operationID string) {
	t.Helper()
	data, err := json.Marshal(cancelTransaction{
		SchemaVersion: domain.ContractSchemaVersion,
		WorkItem:      id,
		OperationID:   operationID,
		Destination:   id,
	})
	if err != nil {
		t.Fatalf("Marshal() marker error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.marker), 0755); err != nil {
		t.Fatalf("MkdirAll() transactions error = %v", err)
	}
	if err := os.WriteFile(paths.marker, data, 0644); err != nil {
		t.Fatalf("WriteFile() marker error = %v", err)
	}
}

func failAtStage(target string) func(string) error {
	return func(stage string) error {
		if stage == target {
			return errors.New("injected cancel failure at " + target)
		}
		return nil
	}
}

func assertActiveRestored(t *testing.T, baseDir string, repository *FSWorkItemRepository, id string) {
	t.Helper()
	repository.commitHook = nil
	active, err := repository.GetWorkItem(baseDir, id)
	if err != nil {
		t.Fatalf("GetWorkItem() after recovery error = %v", err)
	}
	if active.Status != domain.WorkItemActive || active.Revision != 1 {
		t.Fatalf("active item after recovery = %#v", active)
	}
}

func assertCancelTransactionClean(t *testing.T, paths cancelPaths) {
	t.Helper()
	for _, path := range []string{paths.stage, paths.backup, paths.marker} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("transaction path still exists %s: %v", path, err)
		}
	}
}

// T8: a legacy cancel marker is rejected with an explicit version message.
func TestGetWorkItemRejectsLegacyCancelMarker(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := NewFSWorkItemRepository()
	item := persistedCancelCandidate(t, baseDir, repository)
	paths := cancelTestPaths(baseDir, item.ID)
	if err := os.MkdirAll(filepath.Dir(paths.marker), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	marker := `{"schema_version":"0.1","work_item":"` + item.ID + `","operation_id":"legacy","destination":"` + item.ID + `"}`
	if err := os.WriteFile(paths.marker, []byte(marker), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := repository.GetWorkItem(baseDir, item.ID)
	if !errors.Is(err, domain.ErrArchiveConflict) || !strings.Contains(err.Error(), `schema_version "0.1" is not supported`) {
		t.Fatalf("GetWorkItem() error = %v, want legacy marker conflict", err)
	}
}
