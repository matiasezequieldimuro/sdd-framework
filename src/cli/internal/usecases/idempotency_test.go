package usecases_test

import (
	"os"
	"path/filepath"
	"testing"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/infra"
	"sdd-cli/internal/usecases"
)

func TestMutatingUseCasesAreIdempotentWithOperationID(t *testing.T) {
	baseDir := setupTestEnv(t)
	defer os.RemoveAll(baseDir)

	repository := infra.NewFSWorkItemRepository()
	workflowRepository := infra.NewFSWorkflowRepository()
	start := usecases.NewStartWorkItemUseCase(
		repository,
		workflowRepository,
		infra.NewFSConfigRepository(),
		infra.NewArtifactManager(),
		infra.NewSystemClock(),
		infra.NewCryptoIDGenerator(),
	)
	actor := domain.Actor{Kind: domain.ActorHuman, ID: "matias"}
	input := usecases.StartWorkItemInput{
		ID:          "idempotent-item",
		WorkflowID:  "feature-standard",
		Title:       "Idempotent item",
		Actor:       actor,
		OperationID: "run:start:001",
	}

	first, err := start.Execute(baseDir, input)
	if err != nil {
		t.Fatalf("first start error = %v", err)
	}
	eventsPath := filepath.Join(baseDir, ".sdd", "work-items", "active", input.ID, "events.jsonl")
	eventsAfterFirstStart, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("ReadFile() first start events error = %v", err)
	}
	replayed, err := start.Execute(baseDir, input)
	if err != nil {
		t.Fatalf("replayed start error = %v", err)
	}
	eventsAfterReplay, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("ReadFile() replayed start events error = %v", err)
	}
	if replayed.Revision != first.Revision || string(eventsAfterReplay) != string(eventsAfterFirstStart) {
		t.Fatal("replayed start changed revision or events")
	}

	deliver := usecases.NewDeliverPhaseUseCase(
		repository,
		workflowRepository,
		infra.NewArtifactManager(),
		infra.NewSystemClock(),
		infra.NewCryptoIDGenerator(),
	)
	deliverInput := usecases.DeliverPhaseInput{
		WorkItemID:  input.ID,
		PhaseID:     "prd",
		Actor:       actor,
		OperationID: "run:deliver:001",
	}
	delivered, err := deliver.Execute(baseDir, deliverInput)
	if err != nil {
		t.Fatalf("first deliver error = %v", err)
	}
	eventsAfterDeliver, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("ReadFile() deliver events error = %v", err)
	}
	replayedDelivery, err := deliver.Execute(baseDir, deliverInput)
	if err != nil {
		t.Fatalf("replayed deliver error = %v", err)
	}
	eventsAfterDeliveryReplay, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("ReadFile() replayed deliver events error = %v", err)
	}
	if replayedDelivery.Revision != delivered.Revision ||
		string(eventsAfterDeliveryReplay) != string(eventsAfterDeliver) {
		t.Fatal("replayed delivery changed revision or events")
	}

	reject := usecases.NewRejectUseCase(
		repository,
		workflowRepository,
		infra.NewSystemClock(),
		infra.NewCryptoIDGenerator(),
	)
	rejectInput := usecases.RejectInput{
		WorkItemID:  input.ID,
		PhaseID:     "prd",
		RejectedBy:  actor,
		Comment:     "Needs revision",
		OperationID: "run:reject:001",
	}
	rejected, err := reject.Execute(baseDir, rejectInput)
	if err != nil {
		t.Fatalf("first reject error = %v", err)
	}
	eventsAfterReject, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("ReadFile() reject events error = %v", err)
	}
	replayedRejection, err := reject.Execute(baseDir, rejectInput)
	if err != nil {
		t.Fatalf("replayed reject error = %v", err)
	}
	eventsAfterRejectReplay, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("ReadFile() replayed reject events error = %v", err)
	}
	if replayedRejection.Revision != rejected.Revision ||
		string(eventsAfterRejectReplay) != string(eventsAfterReject) {
		t.Fatal("replayed rejection changed revision or events")
	}
}
