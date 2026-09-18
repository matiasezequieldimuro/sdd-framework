package usecases

import (
	"fmt"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

type RecordEventInput struct {
	WorkItemID  string
	EventType   string
	Message     string
	Actor       domain.Actor
	Data        map[string]interface{}
	OperationID string
}

type RecordEventUseCase struct {
	workItemRepo ports.WorkItemMutationRepository
	clock        ports.Clock
	idGenerator  ports.IDGenerator
}

func NewRecordEventUseCase(
	repo ports.WorkItemMutationRepository,
	clock ports.Clock,
	idGenerator ports.IDGenerator,
) *RecordEventUseCase {
	return &RecordEventUseCase{
		workItemRepo: repo,
		clock:        clock,
		idGenerator:  idGenerator,
	}
}

func (uc *RecordEventUseCase) Execute(baseDir string, in RecordEventInput) error {
	if err := domain.ValidateActor(in.Actor); err != nil {
		return err
	}
	item, err := uc.workItemRepo.GetWorkItem(baseDir, in.WorkItemID)
	if err != nil {
		return err
	}
	applied, err := operationApplied(baseDir, in.WorkItemID, in.OperationID, uc.workItemRepo)
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	data := make(map[string]interface{}, len(in.Data)+1)
	for key, value := range in.Data {
		data[key] = value
	}
	if in.Message != "" {
		data["message"] = in.Message
	}

	event, err := newOperationEvent(
		in.WorkItemID,
		in.EventType,
		in.Actor,
		data,
		in.OperationID,
		uc.clock,
		uc.idGenerator,
	)
	if err != nil {
		return fmt.Errorf("failed to generate event: %w", err)
	}
	if _, err := commitWorkItem(baseDir, uc.workItemRepo, item, nil, []domain.Event{event}, in.OperationID); err != nil {
		return fmt.Errorf("failed to commit event: %w", err)
	}

	return nil
}
