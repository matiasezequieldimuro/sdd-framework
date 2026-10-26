package infra

// fs_cancel_repository.go implements WorkItemCanceller: the transactional
// movement of a work item from active/ to cancelled/.
//
// The implementation is deliberately analogous to fs_archive_repository.go
// (DD-3): same three-phase commit (stage → marker → rename), same recovery
// branches (destination published / active restored / backup pending).
// All filesystem primitives (copyDirectory, appendStagedEvents, writeAndSync,
// syncDirectory, syncTree, lockWorkItem, recoverWorkItemTransaction,
// eventOperationExists, containedPath, pathExists) are reused from
// fs_repository.go — no new primitives are introduced.
//
// Key differences from archive:
//   - Destination: cancelled/<id> (no date prefix; ID is globally unique).
//   - Marker:      .transactions/<id>.cancel.json
//   - Stage:       .transactions/<id>.cancel-stage
//   - Backup:      .transactions/<id>.cancel-backup
//   - No artifact gate (validateCancelStage is not needed).
//   - Source status must be active (not completed).

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

// cancelTransaction is the marker written atomically before the rename phase so
// that a crash between steps can be recovered deterministically.
type cancelTransaction struct {
	SchemaVersion string `json:"schema_version"`
	WorkItem      string `json:"work_item"`
	OperationID   string `json:"operation_id,omitempty"`
	Destination   string `json:"destination"`
}

// cancelDestination computes the canonical name and relative path for a
// cancelled work item directory.
//
// @param id Work item identifier (must be a valid kebab-case identifier).
// @returns (directoryName, relativePath, error)
func cancelDestination(id string) (name, relative string, err error) {
	if err := domain.ValidateIdentifier("work item id", id); err != nil {
		return "", "", err
	}
	name = id
	relative = filepath.ToSlash(filepath.Join(".sdd", "work-items", "cancelled", id))
	return name, relative, nil
}

// WorkItemCancelledExists reports whether a cancelled work item directory exists
// for the given id. It checks only for the presence of the manifest file and
// does not validate its contents.
//
// @param baseDir Absolute path to the project root.
// @param id      Work item identifier.
func (r *FSWorkItemRepository) WorkItemCancelledExists(baseDir, id string) (bool, error) {
	if err := domain.ValidateIdentifier("work item id", id); err != nil {
		return false, err
	}
	cancelledPath, err := containedPath(filepath.Join(baseDir, ".sdd"), "work-items", "cancelled", id)
	if err != nil {
		return false, err
	}
	manifestPath := filepath.Join(cancelledPath, "manifest.yaml")
	return pathExists(manifestPath)
}

// FindCancelledWorkItem recovers any in-flight cancel transaction and then
// reads the cancelled work item. Returns ErrWorkItemNotFound if the item is
// not in cancelled/.
//
// @param baseDir Absolute path to the project root.
// @param id      Work item identifier.
func (r *FSWorkItemRepository) FindCancelledWorkItem(baseDir, id string) (*ports.LocatedWorkItem, error) {
	if err := domain.ValidateIdentifier("work item id", id); err != nil {
		return nil, err
	}
	if err := r.recoverCancelIfNeeded(baseDir, id); err != nil {
		return nil, err
	}
	return r.findCancelledWorkItemNoRecovery(baseDir, id)
}

// findCancelledWorkItemNoRecovery reads the cancelled work item without
// triggering recovery. Callers that already hold the lock must use this.
func (r *FSWorkItemRepository) findCancelledWorkItemNoRecovery(baseDir, id string) (*ports.LocatedWorkItem, error) {
	cancelledPath, err := containedPath(filepath.Join(baseDir, ".sdd"), "work-items", "cancelled", id)
	if err != nil {
		return nil, err
	}
	exists, err := pathExists(cancelledPath)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, domain.ErrWorkItemNotFound
	}
	item, err := r.readWorkItemAt(baseDir, cancelledPath, id)
	if err != nil {
		return nil, err
	}
	if item.Status != domain.WorkItemCancelled {
		return nil, fmt.Errorf(
			"%w: cancelled work item %s has status %s",
			domain.ErrArchiveConflict,
			id,
			item.Status,
		)
	}
	_, relative, err := cancelDestination(id)
	if err != nil {
		return nil, err
	}
	return &ports.LocatedWorkItem{
		Item:         item,
		Location:     ports.WorkItemLocationCancelled,
		RelativePath: relative,
	}, nil
}

// CancelledOperationApplied reports whether the given operationID is already
// recorded in the events.jsonl of a cancelled work item. Returns false if the
// item is not yet cancelled or operationID is empty.
//
// @param baseDir     Absolute path to the project root.
// @param id          Work item identifier.
// @param operationID Idempotency key to look up.
func (r *FSWorkItemRepository) CancelledOperationApplied(baseDir, id, operationID string) (bool, error) {
	if operationID == "" {
		return false, nil
	}
	if err := domain.ValidateOperationID(operationID); err != nil {
		return false, err
	}
	record, err := r.FindCancelledWorkItem(baseDir, id)
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

// CancelWorkItem performs the transactional move of a work item from active/ to
// cancelled/. The operation is idempotent when an operationID is provided: if the
// cancel event is already present in the cancelled item's events.jsonl the method
// returns ErrOperationAlreadyApplied, allowing the use case to resolve by reading
// the cancelled item.
//
// Transaction phases (mirrors ArchiveWorkItem):
//  1. Stage: copy active → .cancel-stage, apply manifest + event writes.
//  2. Marker: write .cancel.json atomically.
//  3. Backup: rename active → .cancel-backup (atomic).
//  4. Publish: rename .cancel-stage → cancelled/<id> (atomic).
//  5. Cleanup: remove backup + marker.
//
// Recovery from crashes between phases is handled by recoverCancelIfNeeded.
//
// @param baseDir Absolute path to the project root.
// @param commit  Bundled mutated work item and audit event.
func (r *FSWorkItemRepository) CancelWorkItem(
	baseDir string,
	commit ports.WorkItemCancelCommit,
) (result *ports.LocatedWorkItem, resultErr error) {
	if commit.Item == nil {
		return nil, fmt.Errorf("%w: cancel commit requires a work item", domain.ErrInvalidWorkItem)
	}
	if err := domain.ValidateIdentifier("work item id", commit.Item.ID); err != nil {
		return nil, err
	}
	if err := domain.ValidateOperationID(commit.OperationID); err != nil {
		return nil, err
	}

	_, destinationRelative, err := cancelDestination(commit.Item.ID)
	if err != nil {
		return nil, err
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
	// Recover any in-flight normal commit (same mechanism used by archive).
	normalBackup := filepath.Join(transactionRoot, commit.Item.ID+".backup")
	if err := recoverWorkItemTransaction(activePath, normalBackup); err != nil {
		return nil, err
	}
	if err := r.recoverCancelTransactionLocked(baseDir, commit.Item.ID); err != nil {
		return nil, err
	}

	// Idempotency: if the item is already in cancelled/ with the same operationID,
	// signal ErrOperationAlreadyApplied so the use case can return the existing result.
	cancelled, cancelledErr := r.findCancelledWorkItemNoRecovery(baseDir, commit.Item.ID)
	if cancelledErr != nil && !errors.Is(cancelledErr, domain.ErrWorkItemNotFound) {
		return nil, cancelledErr
	}
	if cancelled != nil {
		if commit.OperationID != "" {
			applied, err := eventOperationExists(
				filepath.Join(baseDir, filepath.FromSlash(cancelled.RelativePath), "events.jsonl"),
				commit.OperationID,
			)
			if err != nil {
				return nil, err
			}
			if applied {
				return nil, domain.ErrOperationAlreadyApplied
			}
		}
		return nil, fmt.Errorf("%w: work item %s is already cancelled", domain.ErrWorkItemCannotCancel, commit.Item.ID)
	}

	// Validate preconditions against the on-disk state.
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
	if current.Status != domain.WorkItemActive {
		return nil, fmt.Errorf("%w: current status is %s", domain.ErrWorkItemCannotCancel, current.Status)
	}
	if commit.Item.Status != domain.WorkItemCancelled {
		return nil, fmt.Errorf("%w: cancel commit status is %s", domain.ErrInvalidWorkItem, commit.Item.Status)
	}

	// Persist revision increment (infra convention: domain does not increment).
	persistedItem := *commit.Item
	persistedItem.Revision = current.Revision + 1
	if err := r.validateCommit(baseDir, &persistedItem, nil, []domain.Event{commit.Event}); err != nil {
		return nil, err
	}
	if err := validateCancelEvent(commit.Event, commit.Item.ID, commit.OperationID); err != nil {
		return nil, err
	}

	cancelledRoot, err := containedPath(filepath.Join(baseDir, ".sdd"), "work-items", "cancelled")
	if err != nil {
		return nil, err
	}
	// cancelled/ is created lazily on the first cancel (spec: no .gitkeep, dir created by motor).
	if err := os.MkdirAll(cancelledRoot, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cancelled directory: %w", err)
	}
	destinationName := commit.Item.ID
	destinationPath, err := containedPath(cancelledRoot, destinationName)
	if err != nil {
		return nil, err
	}
	stagePath := filepath.Join(transactionRoot, commit.Item.ID+".cancel-stage")
	backupPath := filepath.Join(transactionRoot, commit.Item.ID+".cancel-backup")
	markerPath := filepath.Join(transactionRoot, commit.Item.ID+".cancel.json")
	for _, path := range []string{stagePath, backupPath, markerPath} {
		if err := os.RemoveAll(path); err != nil {
			return nil, fmt.Errorf("failed to prepare cancel transaction path %s: %w", path, err)
		}
	}
	if err := os.Mkdir(stagePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cancel stage: %w", err)
	}
	stageOwned := true
	defer func() {
		if stageOwned {
			resultErr = errors.Join(resultErr, os.RemoveAll(stagePath))
		}
	}()

	// Phase 1: populate the stage directory.
	if err := copyDirectory(activePath, stagePath); err != nil {
		return nil, fmt.Errorf("failed to stage active work item: %w", err)
	}
	if err := appendStagedEvents(filepath.Join(stagePath, "events.jsonl"), []domain.Event{commit.Event}); err != nil {
		return nil, err
	}
	manifestData, err := yaml.Marshal(&persistedItem)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cancelled manifest: %w", err)
	}
	if err := writeAndSync(filepath.Join(stagePath, "manifest.yaml"), manifestData, 0644); err != nil {
		return nil, fmt.Errorf("failed to stage cancelled manifest: %w", err)
	}
	if err := syncTree(stagePath); err != nil {
		return nil, fmt.Errorf("failed to sync cancel stage: %w", err)
	}

	// Phase 2: write the transaction marker (crash-safe sentinel).
	marker := cancelTransaction{
		SchemaVersion: domain.ContractSchemaVersion,
		WorkItem:      commit.Item.ID,
		OperationID:   commit.OperationID,
		Destination:   destinationName,
	}
	markerData, err := json.Marshal(marker)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cancel transaction: %w", err)
	}
	if err := writeAndSync(markerPath, markerData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write cancel transaction marker: %w", err)
	}
	if err := syncDirectory(transactionRoot); err != nil {
		return nil, fmt.Errorf("failed to sync cancel transaction marker: %w", err)
	}
	if err := r.runCommitHook("cancel_before_active_backup"); err != nil {
		return nil, errors.Join(err, rollbackCancelBeforePublish(activePath, backupPath, stagePath, markerPath))
	}

	// Phase 3: backup active → .cancel-backup (atomic rename).
	if err := os.Rename(activePath, backupPath); err != nil {
		return nil, errors.Join(
			fmt.Errorf("failed to preserve active work item: %w", err),
			rollbackCancelBeforePublish(activePath, backupPath, stagePath, markerPath),
		)
	}
	if err := syncDirectory(filepath.Dir(activePath)); err != nil {
		rollbackErr := rollbackCancelBeforePublish(activePath, backupPath, stagePath, markerPath)
		return nil, errors.Join(fmt.Errorf("failed to sync active cancel removal: %w", err), rollbackErr)
	}
	if err := r.runCommitHook("cancel_after_active_backup"); err != nil {
		rollbackErr := rollbackCancelBeforePublish(activePath, backupPath, stagePath, markerPath)
		return nil, errors.Join(err, rollbackErr)
	}

	// Phase 4: publish stage → cancelled/<id> (atomic rename).
	if err := os.Rename(stagePath, destinationPath); err != nil {
		rollbackErr := rollbackCancelBeforePublish(activePath, backupPath, stagePath, markerPath)
		return nil, errors.Join(fmt.Errorf("failed to publish cancelled work item: %w", err), rollbackErr)
	}
	stageOwned = false
	if err := r.runCommitHook("cancel_after_destination_publish"); err != nil {
		rollbackErr := rollbackPublishedCancel(activePath, destinationPath, backupPath, transactionRoot, markerPath)
		return nil, errors.Join(err, rollbackErr)
	}
	if err := syncDirectory(cancelledRoot); err != nil {
		rollbackErr := rollbackPublishedCancel(activePath, destinationPath, backupPath, transactionRoot, markerPath)
		return nil, errors.Join(fmt.Errorf("failed to sync cancelled directory: %w", err), rollbackErr)
	}
	if err := syncDirectory(filepath.Dir(activePath)); err != nil {
		rollbackErr := rollbackPublishedCancel(activePath, destinationPath, backupPath, transactionRoot, markerPath)
		return nil, errors.Join(fmt.Errorf("failed to sync active directory: %w", err), rollbackErr)
	}

	// Phase 5: cleanup (best-effort; the cancel is already committed).
	if err := os.RemoveAll(backupPath); err != nil {
		return nil, fmt.Errorf("cancel committed but backup cleanup failed: %w", err)
	}
	if err := os.Remove(markerPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("cancel committed but marker cleanup failed: %w", err)
	}
	if err := syncDirectory(transactionRoot); err != nil {
		return nil, fmt.Errorf("cancel committed but transaction cleanup sync failed: %w", err)
	}

	commit.Item.Revision = persistedItem.Revision
	return &ports.LocatedWorkItem{
		Item:         commit.Item,
		Location:     ports.WorkItemLocationCancelled,
		RelativePath: destinationRelative,
	}, nil
}

// validateCancelEvent verifies that the provided event is a well-formed
// work_item.cancelled event with the correct fields.
func validateCancelEvent(event domain.Event, id, operationID string) error {
	if event.Type != "work_item.cancelled" {
		return fmt.Errorf("%w: cancel event type is %q", domain.ErrSchemaValidation, event.Type)
	}
	if event.WorkItem != id {
		return fmt.Errorf("%w: cancel event work item is %q", domain.ErrSchemaValidation, event.WorkItem)
	}
	if event.CorrelationID != operationID {
		return fmt.Errorf("%w: cancel event correlation id does not match operation", domain.ErrSchemaValidation)
	}
	from, fromOK := event.Data["from"].(string)
	to, toOK := event.Data["to"].(string)
	reason, reasonOK := event.Data["reason"].(string)
	if !fromOK || !toOK || !reasonOK ||
		from != string(domain.WorkItemActive) ||
		to != string(domain.WorkItemCancelled) ||
		reason == "" {
		return fmt.Errorf("%w: invalid cancel event payload", domain.ErrSchemaValidation)
	}
	return nil
}

// recoverCancelIfNeeded detects and recovers an in-flight cancel transaction
// for the given work item if a .cancel.json marker exists. It acquires the
// work item lock before delegating to recoverCancelTransactionLocked.
func (r *FSWorkItemRepository) recoverCancelIfNeeded(baseDir, id string) error {
	if err := domain.ValidateIdentifier("work item id", id); err != nil {
		return err
	}
	transactionRoot, err := containedPath(filepath.Join(baseDir, ".sdd"), "work-items", ".transactions")
	if err != nil {
		return err
	}
	markerPath := filepath.Join(transactionRoot, id+".cancel.json")
	exists, err := pathExists(markerPath)
	if err != nil || !exists {
		return err
	}
	unlock, err := r.lockWorkItem(baseDir, id)
	if err != nil {
		return err
	}
	defer unlock()
	return r.recoverCancelTransactionLocked(baseDir, id)
}

// recoverCancelTransactionLocked inspects the cancel transaction marker and
// decides which recovery branch to apply:
//
//   - destination published (cancelled/<id> exists): drop backup + marker.
//   - active present (rename did not complete): drop stage + marker.
//   - only backup exists: restore backup → active, drop stage + marker.
//
// Callers must hold the work item lock before calling this method.
func (r *FSWorkItemRepository) recoverCancelTransactionLocked(baseDir, id string) error {
	transactionRoot, err := containedPath(filepath.Join(baseDir, ".sdd"), "work-items", ".transactions")
	if err != nil {
		return err
	}
	markerPath := filepath.Join(transactionRoot, id+".cancel.json")
	markerData, err := os.ReadFile(markerPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read cancel transaction marker: %w", err)
	}
	var marker cancelTransaction
	if err := json.Unmarshal(markerData, &marker); err != nil {
		return fmt.Errorf("%w: invalid cancel transaction marker: %v", domain.ErrArchiveConflict, err)
	}
	if marker.SchemaVersion != domain.ContractSchemaVersion {
		return fmt.Errorf(
			"%w: cancel transaction marker schema_version %q is not supported (requires %q); recover it with the sdd-cli version that created it",
			domain.ErrArchiveConflict, marker.SchemaVersion, domain.ContractSchemaVersion,
		)
	}
	if marker.WorkItem != id || marker.Destination != id {
		return fmt.Errorf("%w: cancel transaction marker does not match %s", domain.ErrArchiveConflict, id)
	}

	activePath, err := r.getWorkItemPath(baseDir, id)
	if err != nil {
		return err
	}
	cancelledRoot, err := containedPath(filepath.Join(baseDir, ".sdd"), "work-items", "cancelled")
	if err != nil {
		return err
	}
	destinationPath, err := containedPath(cancelledRoot, marker.Destination)
	if err != nil {
		return err
	}
	stagePath := filepath.Join(transactionRoot, id+".cancel-stage")
	backupPath := filepath.Join(transactionRoot, id+".cancel-backup")

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
		return fmt.Errorf("%w: work item %s is visible in active and cancelled", domain.ErrArchiveConflict, id)
	}
	if destinationExists {
		// Cancel was published; verify integrity and clean up leftovers.
		item, err := r.readWorkItemAt(baseDir, destinationPath, id)
		if err != nil {
			return fmt.Errorf("%w: published cancelled item is invalid: %v", domain.ErrArchiveConflict, err)
		}
		if item.Status != domain.WorkItemCancelled {
			return fmt.Errorf("%w: published cancelled item has status %s", domain.ErrArchiveConflict, item.Status)
		}
		if marker.OperationID != "" {
			applied, err := eventOperationExists(filepath.Join(destinationPath, "events.jsonl"), marker.OperationID)
			if err != nil {
				return err
			}
			if !applied {
				return fmt.Errorf(
					"%w: published cancelled item is missing operation %s",
					domain.ErrArchiveConflict,
					marker.OperationID,
				)
			}
		}
		if err := os.RemoveAll(backupPath); err != nil {
			return fmt.Errorf("failed to clean committed cancel backup: %w", err)
		}
		if err := os.RemoveAll(stagePath); err != nil {
			return fmt.Errorf("failed to clean committed cancel stage: %w", err)
		}
		if err := os.Remove(markerPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to clean committed cancel marker: %w", err)
		}
		return syncDirectory(transactionRoot)
	}
	if activeExists {
		// Cancel did not complete; active is still in place — just clean up.
		if backupExists {
			if err := os.RemoveAll(backupPath); err != nil {
				return fmt.Errorf("failed to clean rolled back cancel backup: %w", err)
			}
		}
		if err := os.RemoveAll(stagePath); err != nil {
			return fmt.Errorf("failed to clean rolled back cancel stage: %w", err)
		}
		if err := os.Remove(markerPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to clean rolled back cancel marker: %w", err)
		}
		return syncDirectory(transactionRoot)
	}
	if backupExists {
		// Crashed after active was moved to backup but before publish completed.
		// Restore active from backup.
		if err := os.Rename(backupPath, activePath); err != nil {
			return fmt.Errorf("failed to recover active work item from cancel backup: %w", err)
		}
		if err := syncDirectory(filepath.Dir(activePath)); err != nil {
			return fmt.Errorf("failed to sync recovered active work item: %w", err)
		}
		if err := os.RemoveAll(stagePath); err != nil {
			return fmt.Errorf("failed to clean recovered cancel stage: %w", err)
		}
		if err := os.Remove(markerPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to clean recovered cancel marker: %w", err)
		}
		return syncDirectory(transactionRoot)
	}
	return fmt.Errorf("%w: cancel transaction for %s has no recoverable snapshot", domain.ErrArchiveConflict, id)
}

// rollbackCancelBeforePublish reverts the transaction to the pre-rename state
// (stage + marker not yet applied to cancelled/). Mirrors rollbackArchiveBeforePublish.
func rollbackCancelBeforePublish(activePath, backupPath, stagePath, markerPath string) error {
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

// rollbackPublishedCancel reverts a cancel after the stage was published to
// cancelled/ but before cleanup was complete. Mirrors rollbackPublishedArchive.
func rollbackPublishedCancel(
	activePath, destinationPath, backupPath, transactionRoot, markerPath string,
) error {
	failedPath := filepath.Join(transactionRoot, filepath.Base(activePath)+".cancel-failed")
	var result error
	if err := os.RemoveAll(failedPath); err != nil {
		result = errors.Join(result, err)
	}
	if err := os.Rename(destinationPath, failedPath); err != nil {
		return errors.Join(result, fmt.Errorf("failed to preserve published cancel during rollback: %w", err))
	}
	if err := os.Rename(backupPath, activePath); err != nil {
		restoreErr := os.Rename(failedPath, destinationPath)
		return errors.Join(
			result,
			fmt.Errorf("failed to restore active work item: %w", err),
			fmt.Errorf("failed to restore cancelled destination: %w", restoreErr),
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

var _ ports.WorkItemCanceller = (*FSWorkItemRepository)(nil)
