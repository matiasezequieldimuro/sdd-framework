package usecases_test

import (
	"os"
	"path/filepath"
	"testing"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/infra"
	"sdd-cli/internal/usecases"
)

func setupTestEnv(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "sdd-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	initUC := usecases.NewInitUseCase(infra.NewFSProjectInitializer())
	if err := initUC.Execute(tmpDir); err != nil {
		t.Fatalf("InitUseCase failed: %v", err)
	}

	return tmpDir
}

func TestFullWorkItemLifecycle(t *testing.T) {
	tmpDir := setupTestEnv(t)
	defer os.RemoveAll(tmpDir)

	wiRepo := infra.NewFSWorkItemRepository()
	wfRepo := infra.NewFSWorkflowRepository()
	configRepo := infra.NewFSConfigRepository()

	actor := domain.Actor{Kind: "human", ID: "matias"}

	// 1. Start Work Item
	startUC := usecases.NewStartWorkItemUseCase(
		wiRepo,
		wfRepo,
		configRepo,
		infra.NewArtifactManager(),
		infra.NewSystemClock(),
		infra.NewCryptoIDGenerator(),
	)
	item, err := startUC.Execute(tmpDir, usecases.StartWorkItemInput{
		ID:         "feat-test-lifecycle",
		WorkflowID: "feature-standard",
		Title:      "Test Lifecycle Feature",
		Summary:    "Testing the end to end engine CLI lifecycle",
		Actor:      actor,
	})
	if err != nil {
		t.Fatalf("StartWorkItemUseCase failed: %v", err)
	}

	if item.ID != "feat-test-lifecycle" {
		t.Errorf("Expected ID 'feat-test-lifecycle', got '%s'", item.ID)
	}
	if item.Phases["prd"].Status != "in_progress" {
		t.Errorf("Expected prd status 'in_progress', got '%s'", item.Phases["prd"].Status)
	}
	if item.Phases["implementation"].Artifact != "artifacts/implementation-report.md" {
		t.Errorf("Expected workflow artifact path, got '%s'", item.Phases["implementation"].Artifact)
	}

	// 2. Status Check
	statusUC := usecases.NewStatusUseCase(wiRepo, wfRepo)
	statusItem, err := statusUC.Execute(tmpDir, "feat-test-lifecycle")
	if err != nil {
		t.Fatalf("StatusUseCase failed: %v", err)
	}
	if statusItem.Title != "Test Lifecycle Feature" {
		t.Errorf("Expected title 'Test Lifecycle Feature', got '%s'", statusItem.Title)
	}

	// 3. Deliver PRD phase for approval
	deliverUC := usecases.NewDeliverPhaseUseCase(
		wiRepo,
		wfRepo,
		infra.NewArtifactManager(),
		infra.NewSystemClock(),
		infra.NewCryptoIDGenerator(),
	)
	deliveredItem, err := deliverUC.Execute(tmpDir, usecases.DeliverPhaseInput{
		WorkItemID: "feat-test-lifecycle",
		PhaseID:    "prd",
		Actor:      actor,
	})
	if err != nil {
		t.Fatalf("DeliverPhaseUseCase failed: %v", err)
	}
	if deliveredItem.Phases["prd"].Status != domain.PhaseAwaitingApproval {
		t.Errorf("Expected prd status 'awaiting_approval', got '%s'", deliveredItem.Phases["prd"].Status)
	}

	// 4. Approve PRD phase
	approveUC := usecases.NewApproveUseCase(
		wiRepo,
		wfRepo,
		infra.NewArtifactManager(),
		infra.NewSystemClock(),
		infra.NewCryptoIDGenerator(),
	)
	approvedItem, err := approveUC.Execute(tmpDir, usecases.ApproveInput{
		WorkItemID: "feat-test-lifecycle",
		PhaseID:    "prd",
		ApprovedBy: actor,
		Comment:    "PRD looks great!",
	})
	if err != nil {
		t.Fatalf("ApproveUseCase failed: %v", err)
	}
	if approvedItem.Phases["prd"].Status != "approved" {
		t.Errorf("Expected prd status 'approved', got '%s'", approvedItem.Phases["prd"].Status)
	}
	if approvedItem.Phases["specification"].Status != "ready" {
		t.Errorf("Expected specification status 'ready', got '%s'", approvedItem.Phases["specification"].Status)
	}

	// 5. Next Action Check
	nextUC := usecases.NewNextUseCase(wiRepo, wfRepo)
	nextAction, err := nextUC.Execute(tmpDir, "feat-test-lifecycle")
	if err != nil {
		t.Fatalf("NextUseCase failed: %v", err)
	}
	if nextAction.PhaseID != "specification" {
		t.Errorf("Expected next phase 'specification', got '%s'", nextAction.PhaseID)
	}
	if nextAction.Status != string(domain.PhaseReady) {
		t.Errorf("Expected next phase status 'ready', got '%s'", nextAction.Status)
	}
	persistedItem, err := wiRepo.GetWorkItem(tmpDir, "feat-test-lifecycle")
	if err != nil {
		t.Fatalf("GetWorkItem failed after next: %v", err)
	}
	if persistedItem.Phases["specification"].Status != domain.PhaseReady {
		t.Errorf("Expected next to leave specification 'ready', got '%s'", persistedItem.Phases["specification"].Status)
	}

	// 6. Begin specification explicitly
	beginUC := usecases.NewBeginPhaseUseCase(
		wiRepo,
		wfRepo,
		infra.NewSystemClock(),
		infra.NewCryptoIDGenerator(),
	)
	begunItem, err := beginUC.Execute(tmpDir, usecases.BeginPhaseInput{
		WorkItemID: "feat-test-lifecycle",
		PhaseID:    "specification",
		Actor:      actor,
	})
	if err != nil {
		t.Fatalf("BeginPhaseUseCase failed: %v", err)
	}
	if begunItem.Phases["specification"].Status != domain.PhaseInProgress {
		t.Errorf("Expected specification status 'in_progress', got '%s'", begunItem.Phases["specification"].Status)
	}

	// 7. Complete remaining mandatory phases
	for _, phaseID := range []string{"specification", "plan"} {
		deliveredItem, err = deliverUC.Execute(tmpDir, usecases.DeliverPhaseInput{
			WorkItemID: "feat-test-lifecycle",
			PhaseID:    phaseID,
			Actor:      actor,
		})
		if err != nil {
			t.Fatalf("DeliverPhaseUseCase failed for %s: %v", phaseID, err)
		}
		if deliveredItem.Phases[phaseID].Status != domain.PhaseAwaitingApproval {
			t.Fatalf("Expected %s awaiting approval, got %s", phaseID, deliveredItem.Phases[phaseID].Status)
		}

		approvedItem, err = approveUC.Execute(tmpDir, usecases.ApproveInput{
			WorkItemID: "feat-test-lifecycle",
			PhaseID:    phaseID,
			ApprovedBy: actor,
		})
		if err != nil {
			t.Fatalf("ApproveUseCase failed for %s: %v", phaseID, err)
		}

		nextPhaseID := "plan"
		if phaseID == "plan" {
			nextPhaseID = "implementation"
		}
		begunItem, err = beginUC.Execute(tmpDir, usecases.BeginPhaseInput{
			WorkItemID: "feat-test-lifecycle",
			PhaseID:    nextPhaseID,
			Actor:      actor,
		})
		if err != nil {
			t.Fatalf("BeginPhaseUseCase failed for %s: %v", nextPhaseID, err)
		}
	}

	for _, phaseID := range []string{"implementation", "verification"} {
		deliveredItem, err = deliverUC.Execute(tmpDir, usecases.DeliverPhaseInput{
			WorkItemID: "feat-test-lifecycle",
			PhaseID:    phaseID,
			Actor:      actor,
		})
		if err != nil {
			t.Fatalf("DeliverPhaseUseCase failed for %s: %v", phaseID, err)
		}
		if deliveredItem.Phases[phaseID].Status != domain.PhaseCompleted {
			t.Fatalf("Expected %s completed, got %s", phaseID, deliveredItem.Phases[phaseID].Status)
		}

		nextPhaseID := "verification"
		if phaseID == "verification" {
			nextPhaseID = "human-code-review"
		}
		begunItem, err = beginUC.Execute(tmpDir, usecases.BeginPhaseInput{
			WorkItemID: "feat-test-lifecycle",
			PhaseID:    nextPhaseID,
			Actor:      actor,
		})
		if err != nil {
			t.Fatalf("BeginPhaseUseCase failed for %s: %v", nextPhaseID, err)
		}
	}

	if _, err = deliverUC.Execute(tmpDir, usecases.DeliverPhaseInput{
		WorkItemID: "feat-test-lifecycle",
		PhaseID:    "human-code-review",
		Actor:      actor,
	}); err != nil {
		t.Fatalf("DeliverPhaseUseCase failed for human-code-review: %v", err)
	}
	approvedItem, err = approveUC.Execute(tmpDir, usecases.ApproveInput{
		WorkItemID: "feat-test-lifecycle",
		PhaseID:    "human-code-review",
		ApprovedBy: actor,
	})
	if err != nil {
		t.Fatalf("ApproveUseCase failed for human-code-review: %v", err)
	}
	if approvedItem.Phases["archive"].Status != domain.PhaseReady {
		t.Fatalf("Expected optional archive ready, got %s", approvedItem.Phases["archive"].Status)
	}

	completeUC := usecases.NewCompleteUseCase(
		wiRepo,
		wfRepo,
		infra.NewArtifactManager(),
		infra.NewSystemClock(),
		infra.NewCryptoIDGenerator(),
	)
	completedItem, err := completeUC.Execute(tmpDir, usecases.CompleteInput{
		WorkItemID: "feat-test-lifecycle",
		Actor:      actor,
	})
	if err != nil {
		t.Fatalf("CompleteUseCase failed: %v", err)
	}
	if completedItem.Status != domain.WorkItemCompleted {
		t.Fatalf("Expected work item completed, got %s", completedItem.Status)
	}

	// 8. Record Event
	recordEventUC := usecases.NewRecordEventUseCase(
		wiRepo,
		infra.NewSystemClock(),
		infra.NewCryptoIDGenerator(),
	)
	err = recordEventUC.Execute(tmpDir, usecases.RecordEventInput{
		WorkItemID: "feat-test-lifecycle",
		EventType:  "test.completed",
		Message:    "Unit test verification passed",
		Actor:      actor,
	})
	if err != nil {
		t.Fatalf("RecordEventUseCase failed: %v", err)
	}
}

func TestRejectedPhaseCanBeReworkedAndApproved(t *testing.T) {
	tmpDir := setupTestEnv(t)
	defer os.RemoveAll(tmpDir)

	repository := infra.NewFSWorkItemRepository()
	workflows := infra.NewFSWorkflowRepository()
	artifacts := infra.NewArtifactManager()
	clock := infra.NewSystemClock()
	ids := infra.NewCryptoIDGenerator()
	actor := domain.Actor{Kind: domain.ActorHuman, ID: "matias"}

	item, err := usecases.NewStartWorkItemUseCase(
		repository,
		workflows,
		infra.NewFSConfigRepository(),
		artifacts,
		clock,
		ids,
	).Execute(tmpDir, usecases.StartWorkItemInput{
		ID:          "reworked-plan",
		WorkflowID:  "fast-change",
		Title:       "Rework rejected plan",
		Actor:       actor,
		OperationID: "rework:start",
	})
	if err != nil {
		t.Fatalf("StartWorkItemUseCase.Execute() error = %v", err)
	}

	deliver := usecases.NewDeliverPhaseUseCase(repository, workflows, artifacts, clock, ids)
	item, err = deliver.Execute(tmpDir, usecases.DeliverPhaseInput{
		WorkItemID:  item.ID,
		PhaseID:     "plan",
		Actor:       actor,
		OperationID: "rework:deliver:first",
	})
	if err != nil {
		t.Fatalf("DeliverPhaseUseCase.Execute() first error = %v", err)
	}

	reject := usecases.NewRejectUseCase(repository, workflows, clock, ids)
	item, err = reject.Execute(tmpDir, usecases.RejectInput{
		WorkItemID:  item.ID,
		PhaseID:     "plan",
		RejectedBy:  actor,
		Comment:     "Add rollback details",
		OperationID: "rework:reject",
	})
	if err != nil {
		t.Fatalf("RejectUseCase.Execute() error = %v", err)
	}
	if item.Phases["plan"].Status != domain.PhaseRejected {
		t.Fatalf("plan status = %s, want %s", item.Phases["plan"].Status, domain.PhaseRejected)
	}
	if item.Phases["implementation"].Status != domain.PhaseBlocked {
		t.Fatalf(
			"implementation status = %s, want %s",
			item.Phases["implementation"].Status,
			domain.PhaseBlocked,
		)
	}

	begin := usecases.NewBeginPhaseUseCase(repository, workflows, clock, ids)
	item, err = begin.Execute(tmpDir, usecases.BeginPhaseInput{
		WorkItemID:  item.ID,
		PhaseID:     "plan",
		Actor:       actor,
		OperationID: "rework:begin",
	})
	if err != nil {
		t.Fatalf("BeginPhaseUseCase.Execute() error = %v", err)
	}
	item, err = deliver.Execute(tmpDir, usecases.DeliverPhaseInput{
		WorkItemID:  item.ID,
		PhaseID:     "plan",
		Actor:       actor,
		OperationID: "rework:deliver:second",
	})
	if err != nil {
		t.Fatalf("DeliverPhaseUseCase.Execute() second error = %v", err)
	}
	if len(item.Approvals) != 2 ||
		item.Approvals[0].Status != domain.ApprovalRejected ||
		item.Approvals[1].Status != domain.ApprovalPending {
		t.Fatalf("approval history after redelivery = %#v", item.Approvals)
	}

	item, err = usecases.NewApproveUseCase(
		repository,
		workflows,
		artifacts,
		clock,
		ids,
	).Execute(tmpDir, usecases.ApproveInput{
		WorkItemID:  item.ID,
		PhaseID:     "plan",
		ApprovedBy:  actor,
		Comment:     "Rollback is clear",
		OperationID: "rework:approve",
	})
	if err != nil {
		t.Fatalf("ApproveUseCase.Execute() error = %v", err)
	}
	if item.Phases["plan"].Status != domain.PhaseApproved ||
		item.Phases["implementation"].Status != domain.PhaseReady {
		t.Fatalf("phases after approval = %#v", item.Phases)
	}
	if len(item.Approvals) != 2 ||
		item.Approvals[0].Status != domain.ApprovalRejected ||
		item.Approvals[1].Status != domain.ApprovalApproved {
		t.Fatalf("final approval history = %#v", item.Approvals)
	}
}

func TestBypassModeStart(t *testing.T) {
	tmpDir := setupTestEnv(t)
	defer os.RemoveAll(tmpDir)

	wiRepo := infra.NewFSWorkItemRepository()
	wfRepo := infra.NewFSWorkflowRepository()
	configRepo := infra.NewFSConfigRepository()

	// Create dummy external artifact
	extArtPath := filepath.Join(tmpDir, "external-prd.md")
	if err := os.WriteFile(extArtPath, []byte("# My External PRD"), 0644); err != nil {
		t.Fatalf("Failed to create external artifact: %v", err)
	}

	actor := domain.Actor{Kind: "human", ID: "matias"}

	startUC := usecases.NewStartWorkItemUseCase(
		wiRepo,
		wfRepo,
		configRepo,
		infra.NewArtifactManager(),
		infra.NewSystemClock(),
		infra.NewCryptoIDGenerator(),
	)
	item, err := startUC.Execute(tmpDir, usecases.StartWorkItemInput{
		ID:           "feat-bypass",
		WorkflowID:   "feature-standard",
		Title:        "Bypass Test Feature",
		Summary:      "Starting from existing PRD",
		FromArtifact: extArtPath,
		Phase:        "prd",
		Actor:        actor,
	})
	if err != nil {
		t.Fatalf("StartWorkItemUseCase bypass mode failed: %v", err)
	}

	if item.Phases["prd"].Status != domain.PhaseAwaitingApproval {
		t.Errorf("Expected prd status 'awaiting_approval', got '%s'", item.Phases["prd"].Status)
	}
	if item.Phases["specification"].Status != domain.PhaseBlocked {
		t.Errorf("Expected specification status 'blocked', got '%s'", item.Phases["specification"].Status)
	}

	// Verify external artifact file was copied
	copiedArtPath := filepath.Join(tmpDir, ".sdd", "work-items", "active", "feat-bypass", "artifacts", "prd.md")
	if _, err := os.Stat(copiedArtPath); err != nil {
		t.Errorf("Expected copied artifact at %s, but file not found: %v", copiedArtPath, err)
	}

	approveUC := usecases.NewApproveUseCase(
		wiRepo,
		wfRepo,
		infra.NewArtifactManager(),
		infra.NewSystemClock(),
		infra.NewCryptoIDGenerator(),
	)
	approvedItem, err := approveUC.Execute(tmpDir, usecases.ApproveInput{
		WorkItemID: "feat-bypass",
		PhaseID:    "prd",
		ApprovedBy: actor,
	})
	if err != nil {
		t.Fatalf("ApproveUseCase failed for external artifact: %v", err)
	}
	if approvedItem.Phases["specification"].Status != domain.PhaseReady {
		t.Errorf("Expected specification status 'ready' after approval, got '%s'", approvedItem.Phases["specification"].Status)
	}
}
