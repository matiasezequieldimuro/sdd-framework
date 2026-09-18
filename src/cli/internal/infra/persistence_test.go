package infra

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

func TestCommitWorkItemRollsBackCompleteSnapshotOnFailure(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := &FSWorkItemRepository{}
	item := newPersistenceTestItem(t)
	initialArtifact := []byte("initial artifact\n")
	initialEvent := persistenceEvent(item.ID, "work_item.created", "create-operation")
	if err := repository.CommitWorkItem(baseDir, ports.WorkItemCommit{
		Item: item,
		Artifacts: []ports.ArtifactWrite{{
			Path:    "artifacts/prd.md",
			Content: initialArtifact,
			Mode:    0644,
		}},
		Events:      []domain.Event{initialEvent},
		OperationID: initialEvent.CorrelationID,
	}); err != nil {
		t.Fatalf("initial CommitWorkItem() error = %v", err)
	}

	workItemPath := filepath.Join(baseDir, ".sdd", "work-items", "active", item.ID)
	before := snapshotDirectory(t, workItemPath)

	item.Title = "title that must be rolled back"
	failedEvent := persistenceEvent(item.ID, "phase.transitioned", "failed-operation")
	repository.commitHook = func(stage string) error {
		if stage == "after_backup" {
			return errors.New("injected publish failure")
		}
		return nil
	}
	err := repository.CommitWorkItem(baseDir, ports.WorkItemCommit{
		Item: item,
		Artifacts: []ports.ArtifactWrite{{
			Path:    "artifacts/prd.md",
			Content: []byte("replacement artifact\n"),
			Mode:    0644,
		}},
		Events:      []domain.Event{failedEvent},
		OperationID: failedEvent.CorrelationID,
	})
	if err == nil {
		t.Fatal("CommitWorkItem() succeeded despite injected failure")
	}

	after := snapshotDirectory(t, workItemPath)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("failed commit changed persisted snapshot\nbefore: %#v\nafter: %#v", before, after)
	}
}

func TestCommitWorkItemFailureBeforePublishLeavesNoWorkItem(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := &FSWorkItemRepository{
		commitHook: func(stage string) error {
			if stage == "before_publish" {
				return errors.New("injected staging failure")
			}
			return nil
		},
	}
	item := newPersistenceTestItem(t)
	event := persistenceEvent(item.ID, "work_item.created", "failed-create")

	err := repository.CommitWorkItem(baseDir, ports.WorkItemCommit{
		Item:        item,
		Events:      []domain.Event{event},
		OperationID: event.CorrelationID,
	})
	if err == nil {
		t.Fatal("CommitWorkItem() succeeded despite injected failure")
	}
	if item.Revision != 0 {
		t.Fatalf("in-memory revision = %d, want 0", item.Revision)
	}
	workItemPath := filepath.Join(baseDir, ".sdd", "work-items", "active", item.ID)
	if _, statErr := os.Stat(workItemPath); !os.IsNotExist(statErr) {
		t.Fatalf("failed create left work item state: %v", statErr)
	}
}

func TestCommitWorkItemDetectsStaleRevision(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := &FSWorkItemRepository{}
	item := persistInitialTestItem(t, baseDir, repository)

	first, err := repository.GetWorkItem(baseDir, item.ID)
	if err != nil {
		t.Fatalf("GetWorkItem() first error = %v", err)
	}
	stale, err := repository.GetWorkItem(baseDir, item.ID)
	if err != nil {
		t.Fatalf("GetWorkItem() stale error = %v", err)
	}

	first.Title = "first writer"
	firstEvent := persistenceEvent(item.ID, "phase.transitioned", "first-writer")
	if err := repository.CommitWorkItem(baseDir, ports.WorkItemCommit{
		Item:        first,
		Events:      []domain.Event{firstEvent},
		OperationID: firstEvent.CorrelationID,
	}); err != nil {
		t.Fatalf("first CommitWorkItem() error = %v", err)
	}

	stale.Title = "stale writer"
	staleEvent := persistenceEvent(item.ID, "phase.transitioned", "stale-writer")
	err = repository.CommitWorkItem(baseDir, ports.WorkItemCommit{
		Item:        stale,
		Events:      []domain.Event{staleEvent},
		OperationID: staleEvent.CorrelationID,
	})
	if !errors.Is(err, domain.ErrConcurrentModification) {
		t.Fatalf("stale CommitWorkItem() error = %v, want %v", err, domain.ErrConcurrentModification)
	}

	persisted, err := repository.GetWorkItem(baseDir, item.ID)
	if err != nil {
		t.Fatalf("GetWorkItem() persisted error = %v", err)
	}
	if persisted.Title != "first writer" || persisted.Revision != first.Revision {
		t.Fatalf("persisted work item = %#v", persisted)
	}
}

func TestCommitWorkItemRejectsConcurrentWriter(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := &FSWorkItemRepository{}
	item := persistInitialTestItem(t, baseDir, repository)

	unlock, err := repository.lockWorkItem(baseDir, item.ID)
	if err != nil {
		t.Fatalf("lockWorkItem() error = %v", err)
	}
	defer unlock()

	event := persistenceEvent(item.ID, "phase.transitioned", "locked-writer")
	err = repository.CommitWorkItem(baseDir, ports.WorkItemCommit{
		Item:        item,
		Events:      []domain.Event{event},
		OperationID: event.CorrelationID,
	})
	if !errors.Is(err, domain.ErrWorkItemLocked) {
		t.Fatalf("CommitWorkItem() error = %v, want %v", err, domain.ErrWorkItemLocked)
	}
}

func TestGetWorkItemRecoversInterruptedDirectorySwap(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := &FSWorkItemRepository{}
	item := persistInitialTestItem(t, baseDir, repository)

	workItemPath := filepath.Join(baseDir, ".sdd", "work-items", "active", item.ID)
	transactionRoot := filepath.Join(baseDir, ".sdd", "work-items", ".transactions")
	if err := os.MkdirAll(transactionRoot, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	backupPath := filepath.Join(transactionRoot, item.ID+".backup")
	if err := os.Rename(workItemPath, backupPath); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if err := os.Mkdir(filepath.Join(transactionRoot, item.ID+"-stale"), 0755); err != nil {
		t.Fatalf("Mkdir() stale stage error = %v", err)
	}

	recovered, err := repository.GetWorkItem(baseDir, item.ID)
	if err != nil {
		t.Fatalf("GetWorkItem() recovery error = %v", err)
	}
	if recovered.Revision != item.Revision {
		t.Fatalf("recovered revision = %d, want %d", recovered.Revision, item.Revision)
	}
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Fatalf("backup still exists after recovery: %v", err)
	}
	if _, err := os.Stat(filepath.Join(transactionRoot, item.ID+"-stale")); !os.IsNotExist(err) {
		t.Fatalf("stale stage still exists after recovery: %v", err)
	}
}

func persistenceTestBaseDir(t *testing.T) string {
	t.Helper()
	baseDir := t.TempDir()
	target := filepath.Join(baseDir, ".sdd")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	if err := copyDirectory(filepath.Join(contractBaseDir(t), ".sdd"), target); err != nil {
		t.Fatalf("copyDirectory() error = %v", err)
	}
	return baseDir
}

func newPersistenceTestItem(t *testing.T) *domain.WorkItem {
	t.Helper()
	item := readFixture(
		t,
		filepath.Join(contractBaseDir(t), ".sdd", "tests", "fixtures", "valid", "10-feature-standard-started.json"),
	)
	item.ID = "persistence-item"
	item.Title = "Persistence item"
	item.Revision = 0
	return item
}

func persistInitialTestItem(
	t *testing.T,
	baseDir string,
	repository *FSWorkItemRepository,
) *domain.WorkItem {
	t.Helper()
	item := newPersistenceTestItem(t)
	event := persistenceEvent(item.ID, "work_item.created", "initial-create")
	if err := repository.CommitWorkItem(baseDir, ports.WorkItemCommit{
		Item:        item,
		Events:      []domain.Event{event},
		OperationID: event.CorrelationID,
	}); err != nil {
		t.Fatalf("initial CommitWorkItem() error = %v", err)
	}
	return item
}

func persistenceEvent(workItemID, eventType, operationID string) domain.Event {
	event := domain.NewEvent(
		"evt_"+operationID,
		time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC),
		workItemID,
		eventType,
		domain.Actor{Kind: domain.ActorCLI, ID: "test"},
		map[string]interface{}{"operation": operationID},
	)
	event.CorrelationID = operationID
	return event
}

func snapshotDirectory(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snapshot[relative] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return snapshot
}

func TestCommitWorkItemTreatsRepeatedOperationAsApplied(t *testing.T) {
	baseDir := persistenceTestBaseDir(t)
	repository := &FSWorkItemRepository{}
	item := newPersistenceTestItem(t)
	event := persistenceEvent(item.ID, "work_item.created", "repeatable-operation")
	commit := ports.WorkItemCommit{
		Item:        item,
		Events:      []domain.Event{event},
		OperationID: event.CorrelationID,
	}
	if err := repository.CommitWorkItem(baseDir, commit); err != nil {
		t.Fatalf("first CommitWorkItem() error = %v", err)
	}
	if err := repository.CommitWorkItem(baseDir, commit); !errors.Is(err, domain.ErrOperationAlreadyApplied) {
		t.Fatalf("repeated CommitWorkItem() error = %v, want %v", err, domain.ErrOperationAlreadyApplied)
	}
}
