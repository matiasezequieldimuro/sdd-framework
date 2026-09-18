package usecases

import (
	"errors"
	"fmt"
	"path/filepath"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

type ArchiveInput struct {
	WorkItemID  string
	Actor       domain.Actor
	OperationID string
}

type ArchiveResult struct {
	WorkItem    *domain.WorkItem       `json:"work_item"`
	Location    ports.WorkItemLocation `json:"location"`
	ArchivePath string                 `json:"archive_path"`
}

type ArchiveUseCase struct {
	activeItems ports.WorkItemReader
	workflows   ports.WorkflowRepository
	validator   ports.ValidationInspector
	archiver    ports.WorkItemArchiver
	clock       ports.Clock
	ids         ports.IDGenerator
}

func NewArchiveUseCase(
	activeItems ports.WorkItemReader,
	workflows ports.WorkflowRepository,
	validator ports.ValidationInspector,
	archiver ports.WorkItemArchiver,
	clock ports.Clock,
	ids ports.IDGenerator,
) *ArchiveUseCase {
	return &ArchiveUseCase{
		activeItems: activeItems,
		workflows:   workflows,
		validator:   validator,
		archiver:    archiver,
		clock:       clock,
		ids:         ids,
	}
}

func (uc *ArchiveUseCase) Execute(baseDir string, input ArchiveInput) (*ArchiveResult, error) {
	if err := domain.ValidateIdentifier("work item id", input.WorkItemID); err != nil {
		return nil, err
	}
	if err := domain.ValidateActor(input.Actor); err != nil {
		return nil, err
	}
	if err := domain.ValidateOperationID(input.OperationID); err != nil {
		return nil, err
	}

	archived, err := uc.archiver.FindArchivedWorkItem(baseDir, input.WorkItemID)
	if err == nil {
		if input.OperationID != "" {
			applied, checkErr := uc.archiver.ArchivedOperationApplied(
				baseDir,
				input.WorkItemID,
				input.OperationID,
			)
			if checkErr != nil {
				return nil, fmt.Errorf("failed to check archived operation: %w", checkErr)
			}
			if applied {
				return archiveResult(archived), nil
			}
		}
		return nil, domain.ErrWorkItemAlreadyArchived
	}
	if !errors.Is(err, domain.ErrWorkItemNotFound) {
		return nil, err
	}

	item, workflow, err := loadWorkItemAndWorkflow(
		baseDir,
		input.WorkItemID,
		uc.activeItems,
		uc.workflows,
	)
	if err != nil {
		return nil, err
	}
	candidate := *item
	if err := candidate.Archive(workflow); err != nil {
		return nil, err
	}

	checks, err := uc.validator.InspectWorkItem(baseDir, input.WorkItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate work item before archive: %w", err)
	}
	report := buildValidationReport(ValidationScopeWorkItem, input.WorkItemID, checks)
	if !report.Valid {
		return nil, &ValidationFailure{Report: report}
	}

	archivedAt := uc.clock.Now().UTC()
	destination := filepath.ToSlash(filepath.Join(
		".sdd",
		"work-items",
		"archive",
		archivedAt.Format("2006-01-02")+"-"+input.WorkItemID,
	))
	if err := item.Archive(workflow); err != nil {
		return nil, err
	}
	eventID, err := uc.ids.NewID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate archive event id: %w", err)
	}
	event := domain.NewEvent(
		eventID,
		archivedAt,
		input.WorkItemID,
		"archive.completed",
		input.Actor,
		map[string]interface{}{
			"from":         string(domain.WorkItemCompleted),
			"to":           string(domain.WorkItemArchived),
			"archive_path": destination,
		},
	)
	event.CorrelationID = input.OperationID

	record, err := uc.archiver.ArchiveWorkItem(baseDir, ports.WorkItemArchiveCommit{
		Item:        item,
		Event:       event,
		ArchivedAt:  archivedAt,
		Destination: destination,
		OperationID: input.OperationID,
	})
	if errors.Is(err, domain.ErrOperationAlreadyApplied) {
		record, err = uc.archiver.FindArchivedWorkItem(baseDir, input.WorkItemID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to archive work item: %w", err)
	}
	return archiveResult(record), nil
}

func archiveResult(record *ports.LocatedWorkItem) *ArchiveResult {
	return &ArchiveResult{
		WorkItem:    record.Item,
		Location:    record.Location,
		ArchivePath: record.RelativePath,
	}
}
