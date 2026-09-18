package infra

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

const archiveDateLayout = "2006-01-02"

type archiveTransaction struct {
	SchemaVersion string `json:"schema_version"`
	WorkItem      string `json:"work_item"`
	OperationID   string `json:"operation_id,omitempty"`
	Destination   string `json:"destination"`
}

func (r *FSWorkItemRepository) FindWorkItem(baseDir, id string) (*ports.LocatedWorkItem, error) {
	if err := domain.ValidateIdentifier("work item id", id); err != nil {
		return nil, err
	}
	if err := r.recoverIfNeeded(baseDir, id); err != nil {
		return nil, err
	}

	activePath, err := r.getWorkItemPath(baseDir, id)
	if err != nil {
		return nil, err
	}
	activeExists, err := pathExists(activePath)
	if err != nil {
		return nil, err
	}
	archived, archivedErr := r.findArchivedWorkItemNoRecovery(baseDir, id)
	if archivedErr != nil && !errors.Is(archivedErr, domain.ErrWorkItemNotFound) {
		return nil, archivedErr
	}
	if activeExists && archived != nil {
		return nil, fmt.Errorf("%w: work item %s exists in active and archive", domain.ErrArchiveConflict, id)
	}
	if activeExists {
		item, err := r.readWorkItemAt(baseDir, activePath, id)
		if err != nil {
			return nil, err
		}
		return &ports.LocatedWorkItem{
			Item:         item,
			Location:     ports.WorkItemLocationActive,
			RelativePath: filepath.ToSlash(filepath.Join(".sdd", "work-items", "active", id)),
		}, nil
	}
	if archived != nil {
		return archived, nil
	}
	return nil, domain.ErrWorkItemNotFound
}

func (r *FSWorkItemRepository) FindArchivedWorkItem(baseDir, id string) (*ports.LocatedWorkItem, error) {
	if err := domain.ValidateIdentifier("work item id", id); err != nil {
		return nil, err
	}
	if err := r.recoverIfNeeded(baseDir, id); err != nil {
		return nil, err
	}
	return r.findArchivedWorkItemNoRecovery(baseDir, id)
}

func (r *FSWorkItemRepository) findArchivedWorkItemNoRecovery(
	baseDir, id string,
) (*ports.LocatedWorkItem, error) {
	archiveRoot, err := containedPath(filepath.Join(baseDir, ".sdd"), "work-items", "archive")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(archiveRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrWorkItemNotFound
		}
		return nil, fmt.Errorf("failed to inspect archive directory: %w", err)
	}

	var matchPath string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		entryID, parseErr := parseArchiveDirectoryName(entry.Name())
		if parseErr != nil || entryID != id {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return nil, fmt.Errorf("failed to inspect archived work item %s: %w", entry.Name(), infoErr)
		}
		if !entry.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%w: archive entry %s is not a directory", domain.ErrArchiveConflict, entry.Name())
		}
		if matchPath != "" {
			return nil, fmt.Errorf("%w: multiple archive entries found for %s", domain.ErrArchiveConflict, id)
		}
		matchPath = filepath.Join(archiveRoot, entry.Name())
	}
	if matchPath == "" {
		return nil, domain.ErrWorkItemNotFound
	}

	item, err := r.readWorkItemAt(baseDir, matchPath, id)
	if err != nil {
		return nil, err
	}
	if item.Status != domain.WorkItemArchived {
		return nil, fmt.Errorf(
			"%w: archived work item %s has status %s",
			domain.ErrArchiveConflict,
			id,
			item.Status,
		)
	}
	return &ports.LocatedWorkItem{
		Item:         item,
		Location:     ports.WorkItemLocationArchive,
		RelativePath: filepath.ToSlash(filepath.Join(".sdd", "work-items", "archive", filepath.Base(matchPath))),
	}, nil
}

func parseArchiveDirectoryName(name string) (string, error) {
	if len(name) < len(archiveDateLayout)+2 || name[len(archiveDateLayout)] != '-' {
		return "", fmt.Errorf("%w: invalid archive directory %q", domain.ErrArchiveConflict, name)
	}
	if _, err := time.Parse(archiveDateLayout, name[:len(archiveDateLayout)]); err != nil {
		return "", fmt.Errorf("%w: invalid archive date in %q", domain.ErrArchiveConflict, name)
	}
	id := name[len(archiveDateLayout)+1:]
	if err := domain.ValidateIdentifier("work item id", id); err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrArchiveConflict, err)
	}
	return id, nil
}

func archiveDestination(id string, archivedAt time.Time) (name, relative string, err error) {
	if err := domain.ValidateIdentifier("work item id", id); err != nil {
		return "", "", err
	}
	name = archivedAt.UTC().Format(archiveDateLayout) + "-" + id
	relative = filepath.ToSlash(filepath.Join(".sdd", "work-items", "archive", name))
	return name, relative, nil
}

func (r *FSWorkItemRepository) ArchivedOperationApplied(
	baseDir, id, operationID string,
) (bool, error) {
	if operationID == "" {
		return false, nil
	}
	if err := domain.ValidateOperationID(operationID); err != nil {
		return false, err
	}
	record, err := r.FindArchivedWorkItem(baseDir, id)
	if errors.Is(err, domain.ErrWorkItemNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return eventOperationExists(
		filepath.Join(baseDir, filepath.FromSlash(record.RelativePath), "events.jsonl"),
		operationID,
	)
}

func (r *FSWorkItemRepository) ArchiveWorkItem(
	baseDir string,
	commit ports.WorkItemArchiveCommit,
) (result *ports.LocatedWorkItem, resultErr error) {
	if commit.Item == nil {
		return nil, fmt.Errorf("%w: archive commit requires a work item", domain.ErrInvalidWorkItem)
	}
	if err := domain.ValidateIdentifier("work item id", commit.Item.ID); err != nil {
		return nil, err
	}
	if err := domain.ValidateOperationID(commit.OperationID); err != nil {
		return nil, err
	}

	destinationName, destinationRelative, err := archiveDestination(commit.Item.ID, commit.ArchivedAt)
	if err != nil {
		return nil, err
	}
	if commit.Destination != destinationRelative {
		return nil, fmt.Errorf(
			"%w: archive destination %q does not match %q",
			domain.ErrInvalidPath,
			commit.Destination,
			destinationRelative,
		)
	}

	unlock, err := r.lockWorkItem(baseDir, commit.Item.ID)
	if err != nil {
		return nil, err
	}
	defer unlock()

	activePath, err := r.getWorkItemPath(baseDir, commit.Item.ID)
	if err != nil {
		return nil, err
	}
	transactionRoot, err := containedPath(filepath.Join(baseDir, ".sdd"), "work-items", ".transactions")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(transactionRoot, 0755); err != nil {
		return nil, fmt.Errorf("failed to create transaction directory: %w", err)
	}
	normalBackup := filepath.Join(transactionRoot, commit.Item.ID+".backup")
	if err := recoverWorkItemTransaction(activePath, normalBackup); err != nil {
		return nil, err
	}
	if err := r.recoverArchiveTransactionLocked(baseDir, commit.Item.ID); err != nil {
		return nil, err
	}

	archived, archivedErr := r.findArchivedWorkItemNoRecovery(baseDir, commit.Item.ID)
	if archivedErr != nil && !errors.Is(archivedErr, domain.ErrWorkItemNotFound) {
		return nil, archivedErr
	}
	if archived != nil {
		if commit.OperationID != "" {
			applied, err := eventOperationExists(
				filepath.Join(baseDir, filepath.FromSlash(archived.RelativePath), "events.jsonl"),
				commit.OperationID,
			)
			if err != nil {
				return nil, err
			}
			if applied {
				return nil, domain.ErrOperationAlreadyApplied
			}
		}
		return nil, domain.ErrWorkItemAlreadyArchived
	}

	current, err := r.readWorkItemAt(baseDir, activePath, commit.Item.ID)
	if err != nil {
		return nil, err
	}
	if current.Revision != commit.Item.Revision {
		return nil, fmt.Errorf(
			"%w: expected revision %d, current revision %d",
			domain.ErrConcurrentModification,
			commit.Item.Revision,
			current.Revision,
		)
	}
	if current.Status != domain.WorkItemCompleted {
		return nil, fmt.Errorf("%w: current status is %s", domain.ErrWorkItemCannotArchive, current.Status)
	}
	if commit.Item.Status != domain.WorkItemArchived {
		return nil, fmt.Errorf("%w: archive commit status is %s", domain.ErrInvalidWorkItem, commit.Item.Status)
	}

	persistedItem := *commit.Item
	persistedItem.Revision = current.Revision + 1
	if err := r.validateCommit(baseDir, &persistedItem, nil, []domain.Event{commit.Event}); err != nil {
		return nil, err
	}
	if err := validateArchiveEvent(commit.Event, commit.Item.ID, destinationRelative, commit.OperationID); err != nil {
		return nil, err
	}

	archiveRoot, err := containedPath(filepath.Join(baseDir, ".sdd"), "work-items", "archive")
	if err != nil {
		return nil, err
	}
	destinationPath, err := containedPath(archiveRoot, destinationName)
	if err != nil {
		return nil, err
	}
	stagePath := filepath.Join(transactionRoot, commit.Item.ID+".archive-stage")
	backupPath := filepath.Join(transactionRoot, commit.Item.ID+".archive-backup")
	markerPath := filepath.Join(transactionRoot, commit.Item.ID+".archive.json")
	for _, path := range []string{stagePath, backupPath, markerPath} {
		if err := os.RemoveAll(path); err != nil {
			return nil, fmt.Errorf("failed to prepare archive transaction path %s: %w", path, err)
		}
	}
	if err := os.Mkdir(stagePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create archive stage: %w", err)
	}
	stageOwned := true
	defer func() {
		if stageOwned {
			resultErr = errors.Join(resultErr, os.RemoveAll(stagePath))
		}
	}()
	if err := copyDirectory(activePath, stagePath); err != nil {
		return nil, fmt.Errorf("failed to stage active work item: %w", err)
	}
	if err := appendStagedEvents(filepath.Join(stagePath, "events.jsonl"), []domain.Event{commit.Event}); err != nil {
		return nil, err
	}
	manifestData, err := yaml.Marshal(&persistedItem)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal archived manifest: %w", err)
	}
	if err := writeAndSync(filepath.Join(stagePath, "manifest.yaml"), manifestData, 0644); err != nil {
		return nil, fmt.Errorf("failed to stage archived manifest: %w", err)
	}
	if err := r.validateArchiveStage(baseDir, stagePath, &persistedItem); err != nil {
		return nil, err
	}
	if err := syncTree(stagePath); err != nil {
		return nil, fmt.Errorf("failed to sync archive stage: %w", err)
	}

	marker := archiveTransaction{
		SchemaVersion: "0.1",
		WorkItem:      commit.Item.ID,
		OperationID:   commit.OperationID,
		Destination:   destinationName,
	}
	markerData, err := json.Marshal(marker)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal archive transaction: %w", err)
	}
	if err := writeAndSync(markerPath, markerData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write archive transaction marker: %w", err)
	}
	if err := syncDirectory(transactionRoot); err != nil {
		return nil, fmt.Errorf("failed to sync archive transaction marker: %w", err)
	}
	if err := r.runCommitHook("archive_before_active_backup"); err != nil {
		return nil, errors.Join(err, rollbackArchiveBeforePublish(activePath, backupPath, stagePath, markerPath))
	}

	if err := os.Rename(activePath, backupPath); err != nil {
		return nil, errors.Join(
			fmt.Errorf("failed to preserve active work item: %w", err),
			rollbackArchiveBeforePublish(activePath, backupPath, stagePath, markerPath),
		)
	}
	if err := syncDirectory(filepath.Dir(activePath)); err != nil {
		rollbackErr := rollbackArchiveBeforePublish(activePath, backupPath, stagePath, markerPath)
		return nil, errors.Join(fmt.Errorf("failed to sync active archive removal: %w", err), rollbackErr)
	}
	if err := r.runCommitHook("archive_after_active_backup"); err != nil {
		rollbackErr := rollbackArchiveBeforePublish(activePath, backupPath, stagePath, markerPath)
		return nil, errors.Join(err, rollbackErr)
	}

	if err := os.Rename(stagePath, destinationPath); err != nil {
		rollbackErr := rollbackArchiveBeforePublish(activePath, backupPath, stagePath, markerPath)
		return nil, errors.Join(fmt.Errorf("failed to publish archived work item: %w", err), rollbackErr)
	}
	stageOwned = false
	if err := r.runCommitHook("archive_after_destination_publish"); err != nil {
		rollbackErr := rollbackPublishedArchive(activePath, destinationPath, backupPath, transactionRoot, markerPath)
		return nil, errors.Join(err, rollbackErr)
	}
	if err := syncDirectory(archiveRoot); err != nil {
		rollbackErr := rollbackPublishedArchive(activePath, destinationPath, backupPath, transactionRoot, markerPath)
		return nil, errors.Join(fmt.Errorf("failed to sync archive directory: %w", err), rollbackErr)
	}
	if err := syncDirectory(filepath.Dir(activePath)); err != nil {
		rollbackErr := rollbackPublishedArchive(activePath, destinationPath, backupPath, transactionRoot, markerPath)
		return nil, errors.Join(fmt.Errorf("failed to sync active directory: %w", err), rollbackErr)
	}
	if err := os.RemoveAll(backupPath); err != nil {
		return nil, fmt.Errorf("archive committed but backup cleanup failed: %w", err)
	}
	if err := os.Remove(markerPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("archive committed but marker cleanup failed: %w", err)
	}
	if err := syncDirectory(transactionRoot); err != nil {
		return nil, fmt.Errorf("archive committed but transaction cleanup sync failed: %w", err)
	}

	commit.Item.Revision = persistedItem.Revision
	return &ports.LocatedWorkItem{
		Item:         commit.Item,
		Location:     ports.WorkItemLocationArchive,
		RelativePath: destinationRelative,
	}, nil
}

func validateArchiveEvent(event domain.Event, id, destination, operationID string) error {
	if event.Type != "archive.completed" {
		return fmt.Errorf("%w: archive event type is %q", domain.ErrSchemaValidation, event.Type)
	}
	if event.WorkItem != id {
		return fmt.Errorf("%w: archive event work item is %q", domain.ErrSchemaValidation, event.WorkItem)
	}
	if event.CorrelationID != operationID {
		return fmt.Errorf("%w: archive event correlation id does not match operation", domain.ErrSchemaValidation)
	}
	from, fromOK := event.Data["from"].(string)
	to, toOK := event.Data["to"].(string)
	path, pathOK := event.Data["archive_path"].(string)
	if !fromOK || !toOK || !pathOK || from != string(domain.WorkItemCompleted) ||
		to != string(domain.WorkItemArchived) || path != destination {
		return fmt.Errorf("%w: invalid archive event payload", domain.ErrSchemaValidation)
	}
	return nil
}

func (r *FSWorkItemRepository) validateArchiveStage(
	baseDir, stagePath string,
	item *domain.WorkItem,
) error {
	workflow, err := NewFSWorkflowRepository().GetWorkflow(baseDir, item.Workflow.ID)
	if err != nil {
		return err
	}
	phase, exists := workflow.Phase("archive")
	if !exists {
		return nil
	}
	if len(phase.Produces) == 0 {
		return fmt.Errorf("%w: archive phase has no artifact", domain.ErrInvalidWorkflow)
	}
	artifactID := phase.Produces[0]
	artifact, exists := workflow.Artifacts[artifactID]
	if !exists {
		return fmt.Errorf("%w: archive artifact %s is not declared", domain.ErrInvalidWorkflow, artifactID)
	}
	artifactPath, err := containedPath(stagePath, filepath.FromSlash(artifact.Path))
	if err != nil {
		return err
	}
	info, err := os.Stat(artifactPath)
	if err != nil {
		return fmt.Errorf("%w: archive artifact %s is unavailable: %v", domain.ErrSchemaValidation, artifact.Path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: archive artifact %s is not a regular file", domain.ErrInvalidPath, artifact.Path)
	}
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		return fmt.Errorf("failed to read archive artifact: %w", err)
	}
	metadata, err := extractFrontMatter(string(data))
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrSchemaValidation, err)
	}
	if err := NewSchemaValidator().ValidateYAML(baseDir, "artifact.schema.json", metadata); err != nil {
		return err
	}
	if err := validateArtifactMetadata(metadata, artifactID, "archive", item.ID); err != nil {
		return err
	}
	return nil
}

func (r *FSWorkItemRepository) recoverArchiveIfNeeded(baseDir, id string) error {
	if err := domain.ValidateIdentifier("work item id", id); err != nil {
		return err
	}
	transactionRoot, err := containedPath(filepath.Join(baseDir, ".sdd"), "work-items", ".transactions")
	if err != nil {
		return err
	}
	markerPath := filepath.Join(transactionRoot, id+".archive.json")
	exists, err := pathExists(markerPath)
	if err != nil || !exists {
		return err
	}
	unlock, err := r.lockWorkItem(baseDir, id)
	if err != nil {
		return err
	}
	defer unlock()
	return r.recoverArchiveTransactionLocked(baseDir, id)
}

func (r *FSWorkItemRepository) recoverArchiveTransactionLocked(baseDir, id string) error {
	transactionRoot, err := containedPath(filepath.Join(baseDir, ".sdd"), "work-items", ".transactions")
	if err != nil {
		return err
	}
	markerPath := filepath.Join(transactionRoot, id+".archive.json")
	markerData, err := os.ReadFile(markerPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read archive transaction marker: %w", err)
	}
	var marker archiveTransaction
	if err := json.Unmarshal(markerData, &marker); err != nil {
		return fmt.Errorf("%w: invalid archive transaction marker: %v", domain.ErrArchiveConflict, err)
	}
	if marker.SchemaVersion != "0.1" || marker.WorkItem != id {
		return fmt.Errorf("%w: archive transaction marker does not match %s", domain.ErrArchiveConflict, id)
	}
	parsedID, err := parseArchiveDirectoryName(marker.Destination)
	if err != nil || parsedID != id {
		return fmt.Errorf("%w: archive transaction destination is invalid", domain.ErrArchiveConflict)
	}

	activePath, err := r.getWorkItemPath(baseDir, id)
	if err != nil {
		return err
	}
	archiveRoot, err := containedPath(filepath.Join(baseDir, ".sdd"), "work-items", "archive")
	if err != nil {
		return err
	}
	destinationPath, err := containedPath(archiveRoot, marker.Destination)
	if err != nil {
		return err
	}
	stagePath := filepath.Join(transactionRoot, id+".archive-stage")
	backupPath := filepath.Join(transactionRoot, id+".archive-backup")

	activeExists, err := pathExists(activePath)
	if err != nil {
		return err
	}
	destinationExists, err := pathExists(destinationPath)
	if err != nil {
		return err
	}
	backupExists, err := pathExists(backupPath)
	if err != nil {
		return err
	}

	if activeExists && destinationExists {
		return fmt.Errorf("%w: work item %s is visible in active and archive", domain.ErrArchiveConflict, id)
	}
	if destinationExists {
		item, err := r.readWorkItemAt(baseDir, destinationPath, id)
		if err != nil {
			return fmt.Errorf("%w: published archive is invalid: %v", domain.ErrArchiveConflict, err)
		}
		if item.Status != domain.WorkItemArchived {
			return fmt.Errorf("%w: published archive has status %s", domain.ErrArchiveConflict, item.Status)
		}
		if marker.OperationID != "" {
			applied, err := eventOperationExists(filepath.Join(destinationPath, "events.jsonl"), marker.OperationID)
			if err != nil {
				return err
			}
			if !applied {
				return fmt.Errorf("%w: published archive is missing operation %s", domain.ErrArchiveConflict, marker.OperationID)
			}
		}
		if err := os.RemoveAll(backupPath); err != nil {
			return fmt.Errorf("failed to clean committed archive backup: %w", err)
		}
		if err := os.RemoveAll(stagePath); err != nil {
			return fmt.Errorf("failed to clean committed archive stage: %w", err)
		}
		if err := os.Remove(markerPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to clean committed archive marker: %w", err)
		}
		return syncDirectory(transactionRoot)
	}
	if activeExists {
		if backupExists {
			if err := os.RemoveAll(backupPath); err != nil {
				return fmt.Errorf("failed to clean rolled back archive backup: %w", err)
			}
		}
		if err := os.RemoveAll(stagePath); err != nil {
			return fmt.Errorf("failed to clean rolled back archive stage: %w", err)
		}
		if err := os.Remove(markerPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to clean rolled back archive marker: %w", err)
		}
		return syncDirectory(transactionRoot)
	}
	if backupExists {
		if err := os.Rename(backupPath, activePath); err != nil {
			return fmt.Errorf("failed to recover active work item from archive backup: %w", err)
		}
		if err := syncDirectory(filepath.Dir(activePath)); err != nil {
			return fmt.Errorf("failed to sync recovered active work item: %w", err)
		}
		if err := os.RemoveAll(stagePath); err != nil {
			return fmt.Errorf("failed to clean recovered archive stage: %w", err)
		}
		if err := os.Remove(markerPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to clean recovered archive marker: %w", err)
		}
		return syncDirectory(transactionRoot)
	}
	return fmt.Errorf("%w: archive transaction for %s has no recoverable snapshot", domain.ErrArchiveConflict, id)
}

func rollbackArchiveBeforePublish(activePath, backupPath, stagePath, markerPath string) error {
	var result error
	if exists, err := pathExists(backupPath); err != nil {
		result = errors.Join(result, err)
	} else if exists {
		result = errors.Join(result, os.Rename(backupPath, activePath))
	}
	result = errors.Join(result, os.RemoveAll(stagePath))
	if err := os.Remove(markerPath); err != nil && !os.IsNotExist(err) {
		result = errors.Join(result, err)
	}
	result = errors.Join(result, syncDirectory(filepath.Dir(activePath)))
	result = errors.Join(result, syncDirectory(filepath.Dir(markerPath)))
	return result
}

func rollbackPublishedArchive(
	activePath, destinationPath, backupPath, transactionRoot, markerPath string,
) error {
	failedPath := filepath.Join(transactionRoot, filepath.Base(activePath)+".archive-failed")
	var result error
	if err := os.RemoveAll(failedPath); err != nil {
		result = errors.Join(result, err)
	}
	if err := os.Rename(destinationPath, failedPath); err != nil {
		return errors.Join(result, fmt.Errorf("failed to preserve published archive during rollback: %w", err))
	}
	if err := os.Rename(backupPath, activePath); err != nil {
		restoreErr := os.Rename(failedPath, destinationPath)
		return errors.Join(
			result,
			fmt.Errorf("failed to restore active work item: %w", err),
			fmt.Errorf("failed to restore archived destination: %w", restoreErr),
		)
	}
	result = errors.Join(result, syncDirectory(filepath.Dir(activePath)))
	result = errors.Join(result, syncDirectory(filepath.Dir(destinationPath)))
	result = errors.Join(result, os.RemoveAll(failedPath))
	if err := os.Remove(markerPath); err != nil && !os.IsNotExist(err) {
		result = errors.Join(result, err)
	}
	result = errors.Join(result, syncDirectory(transactionRoot))
	return result
}

var _ ports.WorkItemCatalogReader = (*FSWorkItemRepository)(nil)
var _ ports.WorkItemArchiver = (*FSWorkItemRepository)(nil)
