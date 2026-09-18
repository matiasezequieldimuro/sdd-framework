package usecases_test

import (
	"errors"
	"testing"
	"time"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
	"sdd-cli/internal/usecases"
)

type memoryArchiver struct {
	archived         *ports.LocatedWorkItem
	operationApplied bool
	findErr          error
	archiveErr       error
	archiveCalls     int
	commit           ports.WorkItemArchiveCommit
}

func (archiver *memoryArchiver) FindArchivedWorkItem(_, _ string) (*ports.LocatedWorkItem, error) {
	if archiver.findErr != nil {
		return nil, archiver.findErr
	}
	if archiver.archived == nil {
		return nil, domain.ErrWorkItemNotFound
	}
	return archiver.archived, nil
}

func (archiver *memoryArchiver) ArchivedOperationApplied(_, _, _ string) (bool, error) {
	return archiver.operationApplied, nil
}

func (archiver *memoryArchiver) ArchiveWorkItem(
	_ string,
	commit ports.WorkItemArchiveCommit,
) (*ports.LocatedWorkItem, error) {
	archiver.archiveCalls++
	archiver.commit = commit
	if archiver.archiveErr != nil {
		return nil, archiver.archiveErr
	}
	commit.Item.Revision++
	archiver.archived = &ports.LocatedWorkItem{
		Item:         commit.Item,
		Location:     ports.WorkItemLocationArchive,
		RelativePath: commit.Destination,
	}
	return archiver.archived, nil
}

func TestArchiveUseCaseArchivesValidatedCompletedItem(t *testing.T) {
	workflow, item := archiveUseCaseState(domain.PhaseCompleted)
	active := &memoryWorkItemRepository{item: item}
	archiver := &memoryArchiver{}
	clock := fixedClock{value: time.Date(2026, time.August, 19, 23, 45, 0, 0, time.FixedZone("local", -3*60*60))}
	ids := &sequenceIDGenerator{}
	validator := &memoryValidationInspector{itemChecks: []domain.ValidationCheck{{
		Status: domain.CheckPassed, Category: "artifact", Code: "artifact.valid", Target: "archive.md", Message: "valid",
	}}}

	result, err := usecases.NewArchiveUseCase(
		active,
		staticWorkflowRepository{workflow: workflow},
		validator,
		archiver,
		clock,
		ids,
	).Execute("project", usecases.ArchiveInput{
		WorkItemID:  item.ID,
		Actor:       domain.Actor{Kind: domain.ActorCLI, ID: "sdd"},
		OperationID: "archive-operation",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if archiver.archiveCalls != 1 || validator.itemCalls != 1 {
		t.Fatalf("calls = archive:%d validation:%d", archiver.archiveCalls, validator.itemCalls)
	}
	if result.WorkItem.Status != domain.WorkItemArchived || result.WorkItem.Revision != 4 {
		t.Fatalf("result = %#v", result)
	}
	if result.ArchivePath != ".sdd/work-items/archive/2026-08-20-archive-use-case" {
		t.Fatalf("archive path = %q", result.ArchivePath)
	}
	if archiver.commit.Event.Type != "archive.completed" ||
		archiver.commit.Event.CorrelationID != "archive-operation" ||
		archiver.commit.Event.At != "2026-08-20T02:45:00Z" {
		t.Fatalf("archive event = %#v", archiver.commit.Event)
	}
}

func TestArchiveUseCaseReturnsValidationFailureBeforeMutation(t *testing.T) {
	workflow, item := archiveUseCaseState(domain.PhaseCompleted)
	archiver := &memoryArchiver{}
	validator := &memoryValidationInspector{itemChecks: []domain.ValidationCheck{{
		Status: domain.CheckFailed, Category: "artifact", Code: "artifact.invalid", Target: "archive.md", Message: "invalid",
	}}}

	_, err := usecases.NewArchiveUseCase(
		&memoryWorkItemRepository{item: item},
		staticWorkflowRepository{workflow: workflow},
		validator,
		archiver,
		fixedClock{value: time.Now()},
		&sequenceIDGenerator{},
	).Execute("project", usecases.ArchiveInput{
		WorkItemID: item.ID,
		Actor:      domain.Actor{Kind: domain.ActorCLI, ID: "sdd"},
	})
	var failure *usecases.ValidationFailure
	if !errors.As(err, &failure) {
		t.Fatalf("Execute() error = %v, want ValidationFailure", err)
	}
	if archiver.archiveCalls != 0 || item.Status != domain.WorkItemCompleted {
		t.Fatalf("archive calls = %d, item status = %s", archiver.archiveCalls, item.Status)
	}
}

func TestArchiveUseCaseReturnsIdempotentArchivedResult(t *testing.T) {
	workflow, item := archiveUseCaseState(domain.PhaseCompleted)
	item.Status = domain.WorkItemArchived
	record := &ports.LocatedWorkItem{
		Item:         item,
		Location:     ports.WorkItemLocationArchive,
		RelativePath: ".sdd/work-items/archive/2026-08-20-archive-use-case",
	}
	archiver := &memoryArchiver{archived: record, operationApplied: true}

	result, err := usecases.NewArchiveUseCase(
		&memoryWorkItemRepository{},
		staticWorkflowRepository{workflow: workflow},
		&memoryValidationInspector{},
		archiver,
		fixedClock{},
		&sequenceIDGenerator{},
	).Execute("project", usecases.ArchiveInput{
		WorkItemID:  item.ID,
		Actor:       domain.Actor{Kind: domain.ActorCLI, ID: "sdd"},
		OperationID: "archive-operation",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ArchivePath != record.RelativePath || archiver.archiveCalls != 0 {
		t.Fatalf("result = %#v, archive calls = %d", result, archiver.archiveCalls)
	}
}

func archiveUseCaseState(archiveStatus domain.PhaseStatus) (*domain.Workflow, *domain.WorkItem) {
	workflow := &domain.Workflow{
		SchemaVersion: "0.1",
		ID:            "archive-workflow",
		WorkItemType:  "test",
		Phases: []domain.WorkflowPhase{
			{ID: "review", Approval: domain.ApprovalRequired},
			{ID: "archive", Requires: []string{"review"}, Approval: domain.ApprovalNone, Optional: true},
		},
	}
	item := &domain.WorkItem{
		ID:       "archive-use-case",
		Revision: 3,
		Type:     "test",
		Status:   domain.WorkItemCompleted,
		Workflow: domain.WorkItemWorkflow{ID: workflow.ID, Version: workflow.SchemaVersion},
		Phases: map[string]domain.PhaseState{
			"review":  {Status: domain.PhaseApproved},
			"archive": {Status: archiveStatus},
		},
	}
	return workflow, item
}
