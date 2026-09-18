package infra

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofrs/flock"
	"gopkg.in/yaml.v3"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

type FSWorkItemRepository struct {
	commitHook func(stage string) error
}

func NewFSWorkItemRepository() *FSWorkItemRepository {
	return &FSWorkItemRepository{}
}

func (r *FSWorkItemRepository) getWorkItemPath(baseDir, id string) (string, error) {
	if err := domain.ValidateIdentifier("work item id", id); err != nil {
		return "", err
	}
	return containedPath(filepath.Join(baseDir, ".sdd"), "work-items", "active", id)
}

func (r *FSWorkItemRepository) getManifestPath(baseDir, id string) (string, error) {
	workItemPath, err := r.getWorkItemPath(baseDir, id)
	if err != nil {
		return "", err
	}
	return containedPath(workItemPath, "manifest.yaml")
}

func (r *FSWorkItemRepository) getEventsPath(baseDir, id string) (string, error) {
	workItemPath, err := r.getWorkItemPath(baseDir, id)
	if err != nil {
		return "", err
	}
	return containedPath(workItemPath, "events.jsonl")
}

func (r *FSWorkItemRepository) WorkItemExists(baseDir, id string) (bool, error) {
	if err := r.recoverIfNeeded(baseDir, id); err != nil {
		return false, err
	}
	path, err := r.getManifestPath(baseDir, id)
	if err != nil {
		return false, err
	}
	activeExists, err := pathExists(path)
	if err != nil {
		return false, fmt.Errorf("failed to inspect manifest.yaml: %w", err)
	}
	archived, err := r.findArchivedWorkItemNoRecovery(baseDir, id)
	if err != nil && !errors.Is(err, domain.ErrWorkItemNotFound) {
		return false, err
	}
	if activeExists && archived != nil {
		return false, fmt.Errorf("%w: work item %s exists in active and archive", domain.ErrArchiveConflict, id)
	}
	return activeExists || archived != nil, nil
}

func (r *FSWorkItemRepository) GetWorkItem(baseDir, id string) (*domain.WorkItem, error) {
	if err := r.recoverIfNeeded(baseDir, id); err != nil {
		return nil, err
	}
	manifestPath, err := r.getManifestPath(baseDir, id)
	if err != nil {
		return nil, err
	}
	return r.readWorkItemAt(baseDir, filepath.Dir(manifestPath), id)
}

func (r *FSWorkItemRepository) readWorkItemAt(baseDir, workItemPath, id string) (*domain.WorkItem, error) {
	data, err := os.ReadFile(filepath.Join(workItemPath, "manifest.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrWorkItemNotFound
		}
		return nil, fmt.Errorf("failed to read manifest.yaml: %w", err)
	}
	if err := NewSchemaValidator().ValidateYAML(baseDir, "work-item.schema.json", data); err != nil {
		return nil, err
	}

	var item domain.WorkItem
	if err := yaml.Unmarshal(data, &item); err != nil {
		return nil, fmt.Errorf("failed to parse manifest.yaml: %w", err)
	}
	if item.ID != id {
		return nil, fmt.Errorf("%w: manifest id %s does not match requested id %s", domain.ErrInvalidWorkItem, item.ID, id)
	}
	workflow, err := NewFSWorkflowRepository().GetWorkflow(baseDir, item.Workflow.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate work item workflow: %w", err)
	}
	if err := item.ValidateAgainst(workflow); err != nil {
		return nil, err
	}

	return &item, nil
}

func (r *FSWorkItemRepository) OperationApplied(baseDir, id, operationID string) (bool, error) {
	if operationID == "" {
		return false, nil
	}
	if err := r.recoverIfNeeded(baseDir, id); err != nil {
		return false, err
	}
	eventsPath, err := r.getEventsPath(baseDir, id)
	if err != nil {
		return false, err
	}
	return eventOperationExists(eventsPath, operationID)
}

func (r *FSWorkItemRepository) CommitWorkItem(baseDir string, commit ports.WorkItemCommit) (resultErr error) {
	if commit.Item == nil {
		return fmt.Errorf("%w: commit requires a work item", domain.ErrInvalidWorkItem)
	}
	if err := domain.ValidateIdentifier("work item id", commit.Item.ID); err != nil {
		return err
	}

	unlock, err := r.lockWorkItem(baseDir, commit.Item.ID)
	if err != nil {
		return err
	}
	defer unlock()

	workItemPath, err := r.getWorkItemPath(baseDir, commit.Item.ID)
	if err != nil {
		return err
	}
	transactionRoot, err := containedPath(filepath.Join(baseDir, ".sdd"), "work-items", ".transactions")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(transactionRoot, 0755); err != nil {
		return fmt.Errorf("failed to create transaction directory: %w", err)
	}
	backupPath := filepath.Join(transactionRoot, commit.Item.ID+".backup")
	if err := recoverWorkItemTransaction(workItemPath, backupPath); err != nil {
		return err
	}
	if err := cleanupTransactionStages(transactionRoot, commit.Item.ID); err != nil {
		return err
	}

	exists, err := pathExists(workItemPath)
	if err != nil {
		return err
	}
	if commit.OperationID != "" && exists {
		applied, err := eventOperationExists(filepath.Join(workItemPath, "events.jsonl"), commit.OperationID)
		if err != nil {
			return err
		}
		if applied {
			return domain.ErrOperationAlreadyApplied
		}
	}

	expectedRevision := commit.Item.Revision
	switch {
	case !exists && expectedRevision != 0:
		return domain.ErrWorkItemNotFound
	case exists && expectedRevision == 0:
		return domain.ErrWorkItemAlreadyExists
	case exists:
		current, err := r.readWorkItemForCommit(baseDir, commit.Item.ID)
		if err != nil {
			return err
		}
		if current.Revision != expectedRevision {
			return fmt.Errorf(
				"%w: expected revision %d, current revision %d",
				domain.ErrConcurrentModification,
				expectedRevision,
				current.Revision,
			)
		}
	}

	persistedItem := *commit.Item
	persistedItem.Revision = expectedRevision + 1
	if err := r.validateCommit(baseDir, &persistedItem, commit.Artifacts, commit.Events); err != nil {
		return err
	}

	stagePath, err := os.MkdirTemp(transactionRoot, commit.Item.ID+"-")
	if err != nil {
		return fmt.Errorf("failed to create transaction staging directory: %w", err)
	}
	stageOwned := true
	defer func() {
		if stageOwned {
			resultErr = errors.Join(resultErr, os.RemoveAll(stagePath))
		}
	}()

	if exists {
		if err := copyDirectory(workItemPath, stagePath); err != nil {
			return fmt.Errorf("failed to stage current work item: %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Join(stagePath, "artifacts"), 0755); err != nil {
		return fmt.Errorf("failed to stage artifacts directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(stagePath, "evidence"), 0755); err != nil {
		return fmt.Errorf("failed to stage evidence directory: %w", err)
	}
	for _, artifact := range commit.Artifacts {
		if err := writeStagedArtifact(stagePath, artifact); err != nil {
			return err
		}
	}
	if err := appendStagedEvents(filepath.Join(stagePath, "events.jsonl"), commit.Events); err != nil {
		return err
	}
	manifestData, err := yaml.Marshal(&persistedItem)
	if err != nil {
		return fmt.Errorf("failed to marshal work item manifest: %w", err)
	}
	if err := writeAndSync(filepath.Join(stagePath, "manifest.yaml"), manifestData, 0644); err != nil {
		return fmt.Errorf("failed to stage manifest.yaml: %w", err)
	}
	if err := syncTree(stagePath); err != nil {
		return fmt.Errorf("failed to sync staged work item: %w", err)
	}
	if err := r.runCommitHook("before_publish"); err != nil {
		return err
	}

	if exists {
		if err := os.Rename(workItemPath, backupPath); err != nil {
			return fmt.Errorf("failed to preserve current work item: %w", err)
		}
		if err := r.runCommitHook("after_backup"); err != nil {
			if rollbackErr := os.Rename(backupPath, workItemPath); rollbackErr != nil {
				return errors.Join(err, fmt.Errorf("failed to roll back work item: %w", rollbackErr))
			}
			return err
		}
	}

	if err := os.Rename(stagePath, workItemPath); err != nil {
		if exists {
			if rollbackErr := os.Rename(backupPath, workItemPath); rollbackErr != nil {
				return errors.Join(
					fmt.Errorf("failed to publish work item: %w", err),
					fmt.Errorf("failed to roll back work item: %w", rollbackErr),
				)
			}
		}
		return fmt.Errorf("failed to publish work item: %w", err)
	}
	stageOwned = false

	activePath := filepath.Dir(workItemPath)
	if err := syncDirectory(activePath); err != nil {
		rollbackErr := rollbackPublishedWorkItem(workItemPath, backupPath, transactionRoot, exists)
		return errors.Join(fmt.Errorf("failed to sync published work item: %w", err), rollbackErr)
	}

	commit.Item.Revision = persistedItem.Revision
	return nil
}

func (r *FSWorkItemRepository) recoverIfNeeded(baseDir, id string) error {
	if err := r.recoverArchiveIfNeeded(baseDir, id); err != nil {
		return err
	}
	workItemPath, err := r.getWorkItemPath(baseDir, id)
	if err != nil {
		return err
	}
	exists, err := pathExists(workItemPath)
	if err != nil || exists {
		return err
	}
	transactionRoot, err := containedPath(filepath.Join(baseDir, ".sdd"), "work-items", ".transactions")
	if err != nil {
		return err
	}
	backupPath := filepath.Join(transactionRoot, id+".backup")
	backupExists, err := pathExists(backupPath)
	if err != nil || !backupExists {
		return err
	}

	unlock, err := r.lockWorkItem(baseDir, id)
	if err != nil {
		return err
	}
	defer unlock()
	if err := recoverWorkItemTransaction(workItemPath, backupPath); err != nil {
		return err
	}
	return cleanupTransactionStages(transactionRoot, id)
}

func (r *FSWorkItemRepository) validateCommit(
	baseDir string,
	item *domain.WorkItem,
	artifacts []ports.ArtifactWrite,
	events []domain.Event,
) error {
	workflow, err := NewFSWorkflowRepository().GetWorkflow(baseDir, item.Workflow.ID)
	if err != nil {
		return fmt.Errorf("failed to validate work item workflow: %w", err)
	}
	if err := item.ValidateAgainst(workflow); err != nil {
		return err
	}
	if err := NewSchemaValidator().ValidateValue(baseDir, "work-item.schema.json", item); err != nil {
		return err
	}
	for _, artifact := range artifacts {
		if err := validateArtifactWrite(artifact); err != nil {
			return err
		}
	}
	for _, event := range events {
		if event.WorkItem != item.ID {
			return fmt.Errorf("%w: event work item %s does not match %s", domain.ErrSchemaValidation, event.WorkItem, item.ID)
		}
		if err := domain.ValidateActor(event.Actor); err != nil {
			return err
		}
		if err := NewSchemaValidator().ValidateValue(baseDir, "event.schema.json", event); err != nil {
			return err
		}
	}
	return nil
}

func (r *FSWorkItemRepository) readWorkItemForCommit(baseDir, id string) (*domain.WorkItem, error) {
	manifestPath, err := r.getManifestPath(baseDir, id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read current manifest: %w", err)
	}
	var item domain.WorkItem
	if err := yaml.Unmarshal(data, &item); err != nil {
		return nil, fmt.Errorf("failed to parse current manifest: %w", err)
	}
	return &item, nil
}

func (r *FSWorkItemRepository) lockWorkItem(baseDir, id string) (func(), error) {
	lockPath, err := containedPath(filepath.Join(baseDir, ".sdd"), "work-items", ".locks", id+".lock")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create lock directory: %w", err)
	}
	fileLock := flock.New(lockPath)
	locked, err := fileLock.TryLock()
	if err != nil {
		return nil, fmt.Errorf("failed to lock work item: %w", err)
	}
	if !locked {
		return nil, domain.ErrWorkItemLocked
	}
	return func() {
		fileLock.Close()
	}, nil
}

func (r *FSWorkItemRepository) runCommitHook(stage string) error {
	if r.commitHook == nil {
		return nil
	}
	return r.commitHook(stage)
}

func validateArtifactWrite(artifact ports.ArtifactWrite) error {
	cleaned := filepath.Clean(artifact.Path)
	if filepath.IsAbs(cleaned) || cleaned == "." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%w: invalid artifact path %q", domain.ErrInvalidPath, artifact.Path)
	}
	if !strings.HasPrefix(filepath.ToSlash(cleaned), "artifacts/") || filepath.Ext(cleaned) != ".md" {
		return fmt.Errorf("%w: artifact path %q must be under artifacts/ and end in .md", domain.ErrInvalidPath, artifact.Path)
	}
	if artifact.Mode == 0 {
		return fmt.Errorf("%w: artifact %s has no file mode", domain.ErrInvalidPath, artifact.Path)
	}
	return nil
}

func writeStagedArtifact(stagePath string, artifact ports.ArtifactWrite) error {
	targetPath, err := containedPath(stagePath, artifact.Path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create staged artifact directory: %w", err)
	}
	if err := writeAndSync(targetPath, artifact.Content, artifact.Mode); err != nil {
		return fmt.Errorf("failed to stage artifact %s: %w", artifact.Path, err)
	}
	return nil
}

func appendStagedEvents(path string, events []domain.Event) error {
	if len(events) == 0 {
		return nil
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open staged events.jsonl: %w", err)
	}
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			closeErr := file.Close()
			return errors.Join(fmt.Errorf("failed to marshal event to JSON: %w", err), closeErr)
		}
		if _, err := file.Write(append(data, '\n')); err != nil {
			closeErr := file.Close()
			return errors.Join(fmt.Errorf("failed to write staged event: %w", err), closeErr)
		}
	}
	if err := file.Sync(); err != nil {
		closeErr := file.Close()
		return errors.Join(fmt.Errorf("failed to sync staged events.jsonl: %w", err), closeErr)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close staged events.jsonl: %w", err)
	}
	return nil
}

func eventOperationExists(path, operationID string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read events.jsonl: %w", err)
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		var event domain.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			closeErr := file.Close()
			return false, errors.Join(fmt.Errorf("failed to parse events.jsonl: %w", err), closeErr)
		}
		if event.CorrelationID == operationID {
			if err := file.Close(); err != nil {
				return false, fmt.Errorf("failed to close events.jsonl: %w", err)
			}
			return true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		closeErr := file.Close()
		return false, errors.Join(fmt.Errorf("failed to scan events.jsonl: %w", err), closeErr)
	}
	if err := file.Close(); err != nil {
		return false, fmt.Errorf("failed to close events.jsonl: %w", err)
	}
	return false, nil
}

func recoverWorkItemTransaction(workItemPath, backupPath string) error {
	backupExists, err := pathExists(backupPath)
	if err != nil || !backupExists {
		return err
	}
	currentExists, err := pathExists(workItemPath)
	if err != nil {
		return err
	}
	if currentExists {
		if err := os.RemoveAll(backupPath); err != nil {
			return fmt.Errorf("failed to clean committed work item backup: %w", err)
		}
		return nil
	}
	if err := os.Rename(backupPath, workItemPath); err != nil {
		return fmt.Errorf("failed to recover previous work item: %w", err)
	}
	return nil
}

func cleanupTransactionStages(transactionRoot, workItemID string) error {
	entries, err := os.ReadDir(transactionRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to inspect transaction directory: %w", err)
	}
	prefix := workItemID + "-"
	failedPath := workItemID + ".failed"
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) && entry.Name() != failedPath {
			continue
		}
		if err := os.RemoveAll(filepath.Join(transactionRoot, entry.Name())); err != nil {
			return fmt.Errorf("failed to clean stale transaction %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func rollbackPublishedWorkItem(workItemPath, backupPath, transactionRoot string, hadPrevious bool) error {
	if !hadPrevious {
		if err := os.RemoveAll(workItemPath); err != nil {
			return fmt.Errorf("failed to remove uncommitted work item: %w", err)
		}
		return syncDirectory(filepath.Dir(workItemPath))
	}

	failedPath := filepath.Join(transactionRoot, filepath.Base(workItemPath)+".failed")
	if err := os.RemoveAll(failedPath); err != nil {
		return fmt.Errorf("failed to prepare rollback directory: %w", err)
	}
	if err := os.Rename(workItemPath, failedPath); err != nil {
		return fmt.Errorf("failed to preserve uncommitted work item during rollback: %w", err)
	}
	if err := os.Rename(backupPath, workItemPath); err != nil {
		restoreErr := os.Rename(failedPath, workItemPath)
		return errors.Join(
			fmt.Errorf("failed to restore previous work item: %w", err),
			fmt.Errorf("failed to restore published work item after rollback failure: %w", restoreErr),
		)
	}
	if err := syncDirectory(filepath.Dir(workItemPath)); err != nil {
		return fmt.Errorf("failed to sync rolled back work item: %w", err)
	}
	if err := os.RemoveAll(failedPath); err != nil {
		return fmt.Errorf("previous work item restored but rollback cleanup failed: %w", err)
	}
	return nil
}

func copyDirectory(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		targetPath := filepath.Join(target, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symlink %s is not allowed in work item state", domain.ErrInvalidPath, path)
		}
		if entry.IsDir() {
			return os.MkdirAll(targetPath, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%w: non-regular file %s is not allowed in work item state", domain.ErrInvalidPath, path)
		}
		return copyFile(path, targetPath, info.Mode().Perm())
	})
}

func copyFile(source, target string, mode fs.FileMode) error {
	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	targetFile, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		closeErr := sourceFile.Close()
		return errors.Join(err, closeErr)
	}
	_, copyErr := io.Copy(targetFile, sourceFile)
	syncErr := targetFile.Sync()
	targetCloseErr := targetFile.Close()
	sourceCloseErr := sourceFile.Close()
	return errors.Join(copyErr, syncErr, targetCloseErr, sourceCloseErr)
}

func writeAndSync(path string, data []byte, mode fs.FileMode) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		closeErr := file.Close()
		return errors.Join(err, closeErr)
	}
	if err := file.Sync(); err != nil {
		closeErr := file.Close()
		return errors.Join(err, closeErr)
	}
	return file.Close()
}

func syncTree(root string) error {
	var directories []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			directories = append(directories, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for i := len(directories) - 1; i >= 0; i-- {
		if err := syncDirectory(directories[i]); err != nil {
			return err
		}
	}
	return nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	if err := directory.Sync(); err != nil {
		closeErr := directory.Close()
		return errors.Join(err, closeErr)
	}
	return directory.Close()
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

type FSWorkflowRepository struct{}

func NewFSWorkflowRepository() ports.WorkflowRepository {
	return &FSWorkflowRepository{}
}

func (r *FSWorkflowRepository) GetWorkflow(baseDir, workflowID string) (*domain.Workflow, error) {
	if err := domain.ValidateIdentifier("workflow id", workflowID); err != nil {
		return nil, err
	}
	path, err := containedPath(filepath.Join(baseDir, ".sdd"), "workflows", workflowID+".workflow.yaml")
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("failed to read workflow file: %w", err)
	}
	if err := NewSchemaValidator().ValidateYAML(baseDir, "workflow.schema.json", data); err != nil {
		return nil, err
	}

	var wf domain.Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("failed to parse workflow YAML: %w", err)
	}
	if wf.ID != workflowID {
		return nil, fmt.Errorf("%w: workflow id %s does not match filename %s", domain.ErrInvalidWorkflow, wf.ID, workflowID)
	}
	if err := wf.ValidateSemantics(); err != nil {
		return nil, err
	}
	if err := validateWorkflowTemplates(baseDir, &wf); err != nil {
		return nil, err
	}

	return &wf, nil
}

func validateWorkflowTemplates(baseDir string, workflow *domain.Workflow) error {
	sddRoot := filepath.Join(baseDir, ".sdd")
	for artifactID, artifact := range workflow.Artifacts {
		templatePath, err := containedPath(sddRoot, "templates", artifact.Template+".md")
		if err != nil {
			return err
		}
		info, err := os.Stat(templatePath)
		if err != nil {
			return fmt.Errorf("%w: artifact %s template %s: %v", domain.ErrInvalidWorkflow, artifactID, artifact.Template, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%w: artifact %s template %s is not a regular file", domain.ErrInvalidWorkflow, artifactID, artifact.Template)
		}
		content, err := os.ReadFile(templatePath)
		if err != nil {
			return fmt.Errorf("%w: read template %s: %v", domain.ErrInvalidWorkflow, artifact.Template, err)
		}
		phaseID := ""
		var phase domain.WorkflowPhase
		for _, candidate := range workflow.Phases {
			for _, producedID := range candidate.Produces {
				if producedID == artifactID {
					phaseID = candidate.ID
					phase = candidate
					break
				}
			}
			if phaseID != "" {
				break
			}
		}
		rendered := domain.RenderTemplate(string(content), map[string]string{
			"title":           "Template validation",
			"id":              "template-validation",
			"type":            workflow.WorkItemType,
			"artifact_id":     artifactID,
			"phase":           phaseID,
			"created_at":      "2026-08-18T00:00:00Z",
			"created_by_kind": string(domain.ActorCLI),
			"created_by_id":   "sdd",
			"sources":         artifactSources(workflow, phase),
		})
		if strings.Contains(rendered, "{{") {
			return fmt.Errorf("%w: template %s contains unresolved placeholders", domain.ErrInvalidWorkflow, artifact.Template)
		}
		metadata, err := extractFrontMatter(rendered)
		if err != nil {
			return fmt.Errorf("%w: template %s: %v", domain.ErrInvalidWorkflow, artifact.Template, err)
		}
		if err := NewSchemaValidator().ValidateYAML(baseDir, "artifact.schema.json", metadata); err != nil {
			return fmt.Errorf("%w: template %s: %v", domain.ErrInvalidWorkflow, artifact.Template, err)
		}
		if err := validateArtifactMetadata(metadata, artifactID, phaseID, "template-validation"); err != nil {
			return fmt.Errorf("%w: template %s: %v", domain.ErrInvalidWorkflow, artifact.Template, err)
		}
	}
	return nil
}
