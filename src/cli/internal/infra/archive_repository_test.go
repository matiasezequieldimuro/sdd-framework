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

func TestArchiveWorkItemPublishesCompleteSnapshot(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := NewFSWorkItemRepository()
	item := persistedArchiveCandidate(t, baseDir, repository)
	archivedAt := time.Date(2026, time.August, 19, 23, 30, 0, 0, time.FixedZone("local", -3*60*60))
	destination := ".sdd/work-items/archive/2026-08-20-archive-item"

	if err := item.Archive(mustWorkflow(t, baseDir, item.Workflow.ID)); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	event := archiveEvent(item.ID, archivedAt, destination, "archive-operation")
	result, err := repository.ArchiveWorkItem(baseDir, ports.WorkItemArchiveCommit{
		Item:        item,
		Event:       event,
		ArchivedAt:  archivedAt,
		Destination: destination,
		OperationID: event.CorrelationID,
	})
	if err != nil {
		t.Fatalf("ArchiveWorkItem() error = %v", err)
	}
	if result.Location != ports.WorkItemLocationArchive || result.RelativePath != destination {
		t.Fatalf("archive result = %#v", result)
	}
	if result.Item.Status != domain.WorkItemArchived || result.Item.Revision != 2 {
		t.Fatalf("archived item = %#v", result.Item)
	}

	activePath := filepath.Join(baseDir, ".sdd", "work-items", "active", item.ID)
	if _, err := os.Stat(activePath); !os.IsNotExist(err) {
		t.Fatalf("active path still exists: %v", err)
	}
	archivePath := filepath.Join(baseDir, filepath.FromSlash(destination))
	if _, err := os.Stat(filepath.Join(archivePath, "artifacts", "archive.md")); err != nil {
		t.Fatalf("archive artifact missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(archivePath, "evidence")); err != nil {
		t.Fatalf("archive evidence directory missing: %v", err)
	}
	events, err := os.ReadFile(filepath.Join(archivePath, "events.jsonl"))
	if err != nil {
		t.Fatalf("ReadFile() events error = %v", err)
	}
	if strings.Count(string(events), `"type":"archive.completed"`) != 1 {
		t.Fatalf("events = %s", events)
	}

	found, err := repository.FindWorkItem(baseDir, item.ID)
	if err != nil {
		t.Fatalf("FindWorkItem() error = %v", err)
	}
	if found.Location != ports.WorkItemLocationArchive {
		t.Fatalf("FindWorkItem() location = %s", found.Location)
	}
}

func TestArchiveWorkItemTreatsRepeatedOperationAsApplied(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := NewFSWorkItemRepository()
	item := persistedArchiveCandidate(t, baseDir, repository)
	archivedAt := time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC)
	destination := ".sdd/work-items/archive/2026-08-20-archive-item"
	if err := item.Archive(mustWorkflow(t, baseDir, item.Workflow.ID)); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	event := archiveEvent(item.ID, archivedAt, destination, "repeat-archive")
	commit := ports.WorkItemArchiveCommit{
		Item:        item,
		Event:       event,
		ArchivedAt:  archivedAt,
		Destination: destination,
		OperationID: event.CorrelationID,
	}
	if _, err := repository.ArchiveWorkItem(baseDir, commit); err != nil {
		t.Fatalf("first ArchiveWorkItem() error = %v", err)
	}
	if _, err := repository.ArchiveWorkItem(baseDir, commit); !errors.Is(err, domain.ErrOperationAlreadyApplied) {
		t.Fatalf("second ArchiveWorkItem() error = %v, want %v", err, domain.ErrOperationAlreadyApplied)
	}
}

func TestArchiveWorkItemRollsBackAfterActiveBackupFailure(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := NewFSWorkItemRepository()
	item := persistedArchiveCandidate(t, baseDir, repository)
	archivedAt := time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC)
	destination := ".sdd/work-items/archive/2026-08-20-archive-item"
	if err := item.Archive(mustWorkflow(t, baseDir, item.Workflow.ID)); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	repository.commitHook = func(stage string) error {
		if stage == "archive_after_active_backup" {
			return errors.New("injected archive failure")
		}
		return nil
	}
	event := archiveEvent(item.ID, archivedAt, destination, "failed-archive")
	_, err := repository.ArchiveWorkItem(baseDir, ports.WorkItemArchiveCommit{
		Item:        item,
		Event:       event,
		ArchivedAt:  archivedAt,
		Destination: destination,
		OperationID: event.CorrelationID,
	})
	if err == nil {
		t.Fatal("ArchiveWorkItem() succeeded despite injected failure")
	}
	active, loadErr := repository.GetWorkItem(baseDir, item.ID)
	if loadErr != nil {
		t.Fatalf("GetWorkItem() after rollback error = %v", loadErr)
	}
	if active.Status != domain.WorkItemCompleted || active.Revision != 1 {
		t.Fatalf("active item after rollback = %#v", active)
	}
	if _, statErr := os.Stat(filepath.Join(baseDir, filepath.FromSlash(destination))); !os.IsNotExist(statErr) {
		t.Fatalf("archive destination exists after rollback: %v", statErr)
	}
}

func TestGetWorkItemRecoversInterruptedArchiveBeforePublish(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := NewFSWorkItemRepository()
	item := persistedArchiveCandidate(t, baseDir, repository)
	transactionRoot := filepath.Join(baseDir, ".sdd", "work-items", ".transactions")
	activePath := filepath.Join(baseDir, ".sdd", "work-items", "active", item.ID)
	backupPath := filepath.Join(transactionRoot, item.ID+".archive-backup")
	stagePath := filepath.Join(transactionRoot, item.ID+".archive-stage")
	markerPath := filepath.Join(transactionRoot, item.ID+".archive.json")
	if err := os.MkdirAll(stagePath, 0755); err != nil {
		t.Fatalf("MkdirAll() stage error = %v", err)
	}
	if err := os.Rename(activePath, backupPath); err != nil {
		t.Fatalf("Rename() backup error = %v", err)
	}
	markerData, err := json.Marshal(archiveTransaction{
		SchemaVersion: "0.1",
		WorkItem:      item.ID,
		OperationID:   "interrupted-archive",
		Destination:   "2026-08-20-" + item.ID,
	})
	if err != nil {
		t.Fatalf("Marshal() marker error = %v", err)
	}
	if err := os.WriteFile(markerPath, markerData, 0644); err != nil {
		t.Fatalf("WriteFile() marker error = %v", err)
	}

	recovered, err := repository.GetWorkItem(baseDir, item.ID)
	if err != nil {
		t.Fatalf("GetWorkItem() recovery error = %v", err)
	}
	if recovered.Status != domain.WorkItemCompleted || recovered.Revision != 1 {
		t.Fatalf("recovered item = %#v", recovered)
	}
	for _, path := range []string{backupPath, stagePath, markerPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("transaction path still exists %s: %v", path, err)
		}
	}
}

func persistedArchiveCandidate(
	t *testing.T,
	baseDir string,
	repository *FSWorkItemRepository,
) *domain.WorkItem {
	t.Helper()
	item := readFixture(
		t,
		filepath.Join(contractBaseDir(t), ".sdd", "tests", "fixtures", "valid", "21-fast-change-completed.json"),
	)
	item.ID = "archive-item"
	item.Title = "Archive item"
	item.Revision = 0
	workflow := mustWorkflow(t, baseDir, item.Workflow.ID)
	writes, err := NewArtifactManager().PrepareArtifactsForPhase(
		baseDir,
		workflow,
		"archive",
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
	event := persistenceEvent(item.ID, "work_item.created", "archive-create")
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

func mustWorkflow(t *testing.T, baseDir, id string) *domain.Workflow {
	t.Helper()
	workflow, err := NewFSWorkflowRepository().GetWorkflow(baseDir, id)
	if err != nil {
		t.Fatalf("GetWorkflow() error = %v", err)
	}
	return workflow
}

func archiveEvent(
	id string,
	at time.Time,
	destination, operationID string,
) domain.Event {
	event := domain.NewEvent(
		"evt_"+operationID,
		at,
		id,
		"archive.completed",
		domain.Actor{Kind: domain.ActorCLI, ID: "test"},
		map[string]interface{}{
			"from":         string(domain.WorkItemCompleted),
			"to":           string(domain.WorkItemArchived),
			"archive_path": destination,
		},
	)
	event.CorrelationID = operationID
	return event
}
