package usecases_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/infra"
	"sdd-cli/internal/usecases"
)

func TestEveryWorkflowCompletesItsMandatoryLifecycle(t *testing.T) {
	baseDir := setupTestEnv(t)
	defer os.RemoveAll(baseDir)

	for _, workflowID := range configuredWorkflowIDs(t) {
		t.Run(workflowID, func(t *testing.T) {
			repository := infra.NewFSWorkItemRepository()
			workflows := infra.NewFSWorkflowRepository()
			artifacts := infra.NewArtifactManager()
			clock := infra.NewSystemClock()
			ids := infra.NewCryptoIDGenerator()
			actor := domain.Actor{Kind: domain.ActorHuman, ID: "contract-test"}
			workItemID := workflowID + "-contract"

			workflow, err := workflows.GetWorkflow(baseDir, workflowID)
			if err != nil {
				t.Fatalf("GetWorkflow() error = %v", err)
			}
			ordered, err := workflow.OrderedPhases()
			if err != nil {
				t.Fatalf("OrderedPhases() error = %v", err)
			}

			item, err := usecases.NewStartWorkItemUseCase(
				repository,
				workflows,
				infra.NewFSConfigRepository(),
				artifacts,
				clock,
				ids,
			).Execute(baseDir, usecases.StartWorkItemInput{
				ID:          workItemID,
				WorkflowID:  workflowID,
				Title:       "Contract lifecycle for " + workflowID,
				Summary:     "Exercise every mandatory phase",
				Actor:       actor,
				OperationID: "contract:" + workflowID + ":start",
			})
			if err != nil {
				t.Fatalf("StartWorkItemUseCase.Execute() error = %v", err)
			}

			begin := usecases.NewBeginPhaseUseCase(repository, workflows, clock, ids)
			deliver := usecases.NewDeliverPhaseUseCase(repository, workflows, artifacts, clock, ids)
			approve := usecases.NewApproveUseCase(repository, workflows, artifacts, clock, ids)
			complete := usecases.NewCompleteUseCase(repository, workflows, artifacts, clock, ids)

			for _, phase := range ordered {
				if phase.Optional {
					continue
				}

				state := item.Phases[phase.ID]
				assertArtifactExists(t, baseDir, item.ID, state.Artifact)
				if state.Status == domain.PhaseReady {
					item, err = begin.Execute(baseDir, usecases.BeginPhaseInput{
						WorkItemID:  item.ID,
						PhaseID:     phase.ID,
						Actor:       actor,
						OperationID: "contract:" + workflowID + ":begin:" + phase.ID,
					})
					if err != nil {
						t.Fatalf("BeginPhaseUseCase.Execute(%s) error = %v", phase.ID, err)
					}
				}
				if got := item.Phases[phase.ID].Status; got != domain.PhaseInProgress {
					t.Fatalf("phase %s status before delivery = %s, want %s", phase.ID, got, domain.PhaseInProgress)
				}

				item, err = deliver.Execute(baseDir, usecases.DeliverPhaseInput{
					WorkItemID:  item.ID,
					PhaseID:     phase.ID,
					Actor:       actor,
					OperationID: "contract:" + workflowID + ":deliver:" + phase.ID,
				})
				if err != nil {
					t.Fatalf("DeliverPhaseUseCase.Execute(%s) error = %v", phase.ID, err)
				}

				if phase.Approval == domain.ApprovalRequired {
					if got := item.Phases[phase.ID].Status; got != domain.PhaseAwaitingApproval {
						t.Fatalf("phase %s status after delivery = %s, want %s", phase.ID, got, domain.PhaseAwaitingApproval)
					}
					item, err = approve.Execute(baseDir, usecases.ApproveInput{
						WorkItemID:  item.ID,
						PhaseID:     phase.ID,
						ApprovedBy:  actor,
						Comment:     "Contract approval",
						OperationID: "contract:" + workflowID + ":approve:" + phase.ID,
					})
					if err != nil {
						t.Fatalf("ApproveUseCase.Execute(%s) error = %v", phase.ID, err)
					}
					if got := item.Phases[phase.ID].Status; got != domain.PhaseApproved {
						t.Fatalf("phase %s status after approval = %s, want %s", phase.ID, got, domain.PhaseApproved)
					}
				} else if got := item.Phases[phase.ID].Status; got != domain.PhaseCompleted {
					t.Fatalf("phase %s status after delivery = %s, want %s", phase.ID, got, domain.PhaseCompleted)
				}
			}

			item, err = complete.Execute(baseDir, usecases.CompleteInput{
				WorkItemID:  item.ID,
				Actor:       actor,
				OperationID: "contract:" + workflowID + ":complete",
			})
			if err != nil {
				t.Fatalf("CompleteUseCase.Execute() error = %v", err)
			}
			if item.Status != domain.WorkItemCompleted {
				t.Fatalf("work item status = %s, want %s", item.Status, domain.WorkItemCompleted)
			}
			for _, phase := range ordered {
				if phase.Optional {
					if got := item.Phases[phase.ID].Status; got != domain.PhaseReady {
						t.Fatalf("optional phase %s status = %s, want %s", phase.ID, got, domain.PhaseReady)
					}
				}
			}

			persisted, err := repository.GetWorkItem(baseDir, item.ID)
			if err != nil {
				t.Fatalf("GetWorkItem() error = %v", err)
			}
			if err := persisted.ValidateAgainst(workflow); err != nil {
				t.Fatalf("persisted work item validation error = %v", err)
			}
			assertLifecycleEvents(t, baseDir, persisted, workflow)
		})
	}
}

func configuredWorkflowIDs(t *testing.T) []string {
	t.Helper()
	contractDir, err := filepath.Abs(filepath.Join("..", "..", "..", ".sdd", "workflows"))
	if err != nil {
		t.Fatalf("Abs() workflows error = %v", err)
	}
	entries, err := os.ReadDir(contractDir)
	if err != nil {
		t.Fatalf("ReadDir() workflows error = %v", err)
	}
	var workflowIDs []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".workflow.yaml") {
			continue
		}
		workflowIDs = append(workflowIDs, strings.TrimSuffix(entry.Name(), ".workflow.yaml"))
	}
	if len(workflowIDs) == 0 {
		t.Fatal("no workflows found")
	}
	return workflowIDs
}

func assertArtifactExists(t *testing.T, baseDir, workItemID, relativePath string) {
	t.Helper()
	if relativePath == "" {
		t.Fatal("phase artifact path is empty")
	}
	path := filepath.Join(baseDir, ".sdd", "work-items", "active", workItemID, relativePath)
	if info, err := os.Stat(path); err != nil {
		t.Fatalf("artifact %s error = %v", relativePath, err)
	} else if info.IsDir() {
		t.Fatalf("artifact %s is a directory", relativePath)
	}
}

func assertLifecycleEvents(
	t *testing.T,
	baseDir string,
	item *domain.WorkItem,
	workflow *domain.Workflow,
) {
	t.Helper()
	path := filepath.Join(baseDir, ".sdd", "work-items", "active", item.ID, item.Traceability.Events)
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() events error = %v", err)
	}
	defer file.Close()

	validator := infra.NewSchemaValidator()
	eventIDs := make(map[string]struct{})
	transitionedPhases := make(map[string]struct{})
	completed := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event domain.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatalf("Unmarshal() event error = %v", err)
		}
		if err := validator.ValidateValue(baseDir, "event.schema.json", event); err != nil {
			t.Fatalf("event %s schema validation error = %v", event.ID, err)
		}
		if _, duplicate := eventIDs[event.ID]; duplicate {
			t.Fatalf("duplicate event id %s", event.ID)
		}
		eventIDs[event.ID] = struct{}{}
		if event.CorrelationID == "" {
			t.Fatalf("event %s has no correlation id", event.ID)
		}
		if event.Type == "phase.transitioned" {
			phase, ok := event.Data["phase"].(string)
			if !ok {
				t.Fatalf("transition event %s has invalid phase data: %#v", event.ID, event.Data)
			}
			transitionedPhases[phase] = struct{}{}
		}
		if event.Type == "work_item.completed" {
			completed = true
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan events error = %v", err)
	}
	if len(eventIDs) == 0 {
		t.Fatal("lifecycle produced no events")
	}
	if !completed {
		t.Fatal("lifecycle did not record work_item.completed")
	}
	for _, phase := range workflow.Phases {
		if _, exists := transitionedPhases[phase.ID]; !exists {
			t.Fatalf("phase %s has no transition event", phase.ID)
		}
	}
}
