package usecases

import (
	"fmt"
	"time"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

type StartWorkItemInput struct {
	ID           string
	WorkflowID   string
	Title        string
	Summary      string
	FromArtifact string
	Phase        string
	Actor        domain.Actor
	OperationID  string
}

type StartWorkItemUseCase struct {
	workItemRepo ports.WorkItemCreationRepository
	workflowRepo ports.WorkflowRepository
	configRepo   ports.ConfigRepository
	artifacts    ports.ArtifactService
	clock        ports.Clock
	idGenerator  ports.IDGenerator
}

func NewStartWorkItemUseCase(
	wiRepo ports.WorkItemCreationRepository,
	wfRepo ports.WorkflowRepository,
	configRepo ports.ConfigRepository,
	artifacts ports.ArtifactService,
	clock ports.Clock,
	idGenerator ports.IDGenerator,
) *StartWorkItemUseCase {
	return &StartWorkItemUseCase{
		workItemRepo: wiRepo,
		workflowRepo: wfRepo,
		configRepo:   configRepo,
		artifacts:    artifacts,
		clock:        clock,
		idGenerator:  idGenerator,
	}
}

func (uc *StartWorkItemUseCase) Execute(baseDir string, in StartWorkItemInput) (*domain.WorkItem, error) {
	if err := domain.ValidateIdentifier("work item id", in.ID); err != nil {
		return nil, err
	}
	if err := domain.ValidateActor(in.Actor); err != nil {
		return nil, err
	}
	if err := domain.ValidateOperationID(in.OperationID); err != nil {
		return nil, err
	}
	if in.Title == "" {
		return nil, fmt.Errorf("%w: title cannot be empty", domain.ErrInvalidWorkItem)
	}
	if (in.FromArtifact == "") != (in.Phase == "") {
		return nil, fmt.Errorf("%w: --from-artifact and --phase must be used together", domain.ErrInvalidExternalArtifact)
	}
	exists, err := uc.workItemRepo.WorkItemExists(baseDir, in.ID)
	if err != nil {
		return nil, err
	}
	if exists {
		if in.OperationID != "" {
			applied, err := operationApplied(baseDir, in.ID, in.OperationID, uc.workItemRepo)
			if err != nil {
				return nil, err
			}
			if applied {
				return uc.workItemRepo.GetWorkItem(baseDir, in.ID)
			}
		}
		return nil, domain.ErrWorkItemAlreadyExists
	}

	workflowID := in.WorkflowID
	if workflowID == "" {
		config, err := uc.configRepo.GetConfig(baseDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load default workflow: %w", err)
		}
		workflowID = config.Defaults.Workflow
	}
	wf, err := uc.workflowRepo.GetWorkflow(baseDir, workflowID)
	if err != nil {
		return nil, fmt.Errorf("invalid workflow: %w", err)
	}

	var (
		externalSource     ports.ExternalArtifact
		externalArtifactID string
	)
	entryPhase, err := wf.EntryPhaseFor("user_prompt")
	if in.FromArtifact != "" {
		entryPhase = in.Phase
		externalArtifactID, err = wf.ExternalArtifactForEntry(entryPhase)
		if err != nil {
			return nil, err
		}
		externalSource, err = uc.artifacts.ResolveExternalArtifact(in.FromArtifact)
	}
	if err != nil {
		return nil, err
	}

	createdAt := uc.clock.Now().UTC().Format(time.RFC3339)
	var externalReference *domain.ExternalArtifactReference
	if in.FromArtifact != "" {
		externalReference = &domain.ExternalArtifactReference{
			Artifact: externalArtifactID,
			Path:     externalSource.Path,
			SHA256:   externalSource.SHA256,
		}
	}
	item, mutation, err := domain.NewWorkItem(wf, domain.NewWorkItemParams{
		ID:               in.ID,
		Title:            in.Title,
		Summary:          in.Summary,
		EntryPhase:       entryPhase,
		CreatedAt:        createdAt,
		CreatedBy:        in.Actor,
		ExternalArtifact: externalReference,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create work item: %w", err)
	}

	templateVars := map[string]string{
		"title":      in.Title,
		"id":         in.ID,
		"created_at": item.CreatedAt,
		"type":       item.Type,
	}
	artifacts, err := uc.artifacts.PrepareArtifactsForPhase(baseDir, wf, entryPhase, in.ID, templateVars)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare artifacts for entry phase: %w", err)
	}

	if in.FromArtifact != "" {
		artifacts, err = uc.artifacts.ImportExternalArtifact(
			wf,
			entryPhase,
			externalArtifactID,
			externalSource,
			artifacts,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to import external artifact: %w", err)
		}
	}

	createdEvent, err := newOperationEvent(in.ID, "work_item.created", in.Actor, map[string]interface{}{
		"workflow": wf.ID,
		"title":    in.Title,
	}, in.OperationID, uc.clock, uc.idGenerator)
	if err != nil {
		return nil, fmt.Errorf("failed to generate creation event: %w", err)
	}
	events := []domain.Event{createdEvent}
	if in.FromArtifact != "" {
		bypassEvent, err := newOperationEvent(in.ID, "phase.bypassed_by_external_input", in.Actor, map[string]interface{}{
			"phase":             in.Phase,
			"external_artifact": externalSource.Path,
			"sha256":            externalSource.SHA256,
		}, in.OperationID, uc.clock, uc.idGenerator)
		if err != nil {
			return nil, fmt.Errorf("failed to generate external input event: %w", err)
		}
		events = append(events, bypassEvent)
	}
	transitionEvents, err := phaseMutationEvents(
		in.ID,
		mutation,
		in.Actor,
		"work_item_started",
		in.OperationID,
		uc.clock,
		uc.idGenerator,
	)
	if err != nil {
		return nil, err
	}
	events = append(events, transitionEvents...)

	persisted, err := commitWorkItem(baseDir, uc.workItemRepo, item, artifacts, events, in.OperationID)
	if err != nil {
		return nil, fmt.Errorf("failed to commit work item: %w", err)
	}
	return persisted, nil
}
