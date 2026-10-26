package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLICompletesFastChangeLifecycle(t *testing.T) {
	binary := buildTestBinary(t)
	projectDir := t.TempDir()

	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "init")
	runJSONBinary(
		t,
		binary,
		0,
		"--json", "--dir", projectDir,
		"start", "cli-contract",
		"--workflow", "fast-change",
		"--title", "CLI contract lifecycle",
		"--operation-id", "cli:start",
	)

	manifestPath := filepath.Join(projectDir, ".sdd", "work-items", "active", "cli-contract", "manifest.yaml")
	beforeNext, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile() before next error = %v", err)
	}
	next := runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "next", "cli-contract")
	nextData := responseData(t, next)
	if nextData["phase_id"] != "plan" || nextData["status"] != "in_progress" {
		t.Fatalf("next data = %#v", nextData)
	}
	afterNext, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile() after next error = %v", err)
	}
	if !bytes.Equal(afterNext, beforeNext) {
		t.Fatal("next modified manifest.yaml")
	}

	stdout, stderr, exitCode := runTestBinary(binary, "--dir", projectDir, "status", "cli-contract")
	if exitCode != 0 || stderr != "" {
		t.Fatalf("text status exit = %d, stderr = %q", exitCode, stderr)
	}
	planIndex := strings.Index(stdout, "plan")
	implementationIndex := strings.Index(stdout, "implementation")
	if !strings.Contains(stdout, "Work Item: cli-contract [active]") ||
		planIndex == -1 ||
		implementationIndex == -1 ||
		planIndex > implementationIndex {
		t.Fatalf("unexpected text status output:\n%s", stdout)
	}

	invalid := runJSONBinary(
		t,
		binary,
		1,
		"--json", "--dir", projectDir,
		"begin", "cli-contract",
		"--phase", "plan",
	)
	if invalid.Error == nil || invalid.Error.Code != "invalid_transition" {
		t.Fatalf("invalid transition response = %#v", invalid)
	}

	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "deliver", "cli-contract", "--phase", "plan", "--operation-id", "cli:deliver:plan")
	rejected := runJSONBinary(
		t,
		binary,
		0,
		"--json", "--dir", projectDir,
		"reject", "cli-contract",
		"--phase", "plan",
		"--by", "contract-test",
		"--comment", "Adjust the plan",
		"--operation-id", "cli:reject:plan",
	)
	rejectedPhases, ok := responseData(t, rejected)["phases"].(map[string]interface{})
	if !ok {
		t.Fatalf("rejected phases = %#v", responseData(t, rejected)["phases"])
	}
	rejectedPlan, ok := rejectedPhases["plan"].(map[string]interface{})
	if !ok || rejectedPlan["status"] != "rejected" {
		t.Fatalf("rejected plan = %#v", rejectedPhases["plan"])
	}
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "begin", "cli-contract", "--phase", "plan", "--operation-id", "cli:begin:plan:rework")
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "deliver", "cli-contract", "--phase", "plan", "--operation-id", "cli:deliver:plan:rework")
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "approve", "cli-contract", "--phase", "plan", "--by", "contract-test", "--operation-id", "cli:approve:plan")
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "begin", "cli-contract", "--phase", "implementation", "--operation-id", "cli:begin:implementation")
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "deliver", "cli-contract", "--phase", "implementation", "--operation-id", "cli:deliver:implementation")
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "begin", "cli-contract", "--phase", "verification", "--operation-id", "cli:begin:verification")
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "deliver", "cli-contract", "--phase", "verification", "--operation-id", "cli:deliver:verification")
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "begin", "cli-contract", "--phase", "human-code-review", "--operation-id", "cli:begin:review")
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "deliver", "cli-contract", "--phase", "human-code-review", "--operation-id", "cli:deliver:review")
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "approve", "cli-contract", "--phase", "human-code-review", "--by", "contract-test", "--operation-id", "cli:approve:review")
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "complete", "cli-contract", "--operation-id", "cli:complete")
	runJSONBinary(
		t,
		binary,
		0,
		"--json", "--dir", projectDir,
		"record-event", "cli-contract",
		"--type", "contract.cli.completed",
		"--message", "CLI lifecycle completed",
		"--operation-id", "cli:record-event",
	)

	status := runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "status", "cli-contract")
	if got := responseData(t, status)["status"]; got != "completed" {
		t.Fatalf("final work item status = %v, want completed", got)
	}
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "validate", "cli-contract")

	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "begin", "cli-contract", "--phase", "archive", "--operation-id", "cli:begin:archive")
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "deliver", "cli-contract", "--phase", "archive", "--operation-id", "cli:deliver:archive")
	archived := runJSONBinary(
		t,
		binary,
		0,
		"--json", "--dir", projectDir,
		"archive", "cli-contract",
		"--operation-id", "cli:archive",
	)
	archiveData := responseData(t, archived)
	if archiveData["location"] != "archive" {
		t.Fatalf("archive data = %#v", archiveData)
	}
	archivePath, ok := archiveData["archive_path"].(string)
	if !ok || !strings.Contains(archivePath, "work-items/archive/") {
		t.Fatalf("archive path = %#v", archiveData["archive_path"])
	}
	if _, err := os.Stat(manifestPath); !os.IsNotExist(err) {
		t.Fatalf("active manifest still exists: %v", err)
	}
	activeWorkItemPath := filepath.Join(projectDir, ".sdd", "work-items", "active", "cli-contract")
	if _, err := os.Stat(activeWorkItemPath); !os.IsNotExist(err) {
		t.Fatalf("active work item directory still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, filepath.FromSlash(archivePath), "manifest.yaml")); err != nil {
		t.Fatalf("archived manifest missing: %v", err)
	}

	archivedStatus := runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "status", "cli-contract")
	archivedStatusData := responseData(t, archivedStatus)
	if archivedStatusData["status"] != "archived" ||
		archivedStatusData["location"] != "archive" ||
		archivedStatusData["archive_path"] != archivePath {
		t.Fatalf("archived status = %#v", archivedStatusData)
	}
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "validate", "cli-contract")
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "validate")

	archivedManifestPath := filepath.Join(projectDir, filepath.FromSlash(archivePath), "manifest.yaml")
	beforeRetry, err := os.ReadFile(archivedManifestPath)
	if err != nil {
		t.Fatalf("ReadFile() before archive retry error = %v", err)
	}
	retried := runJSONBinary(
		t,
		binary,
		0,
		"--json", "--dir", projectDir,
		"archive", "cli-contract",
		"--operation-id", "cli:archive",
	)
	if responseData(t, retried)["archive_path"] != archivePath {
		t.Fatalf("retried archive = %#v", responseData(t, retried))
	}
	afterRetry, err := os.ReadFile(archivedManifestPath)
	if err != nil {
		t.Fatalf("ReadFile() after archive retry error = %v", err)
	}
	if !bytes.Equal(beforeRetry, afterRetry) {
		t.Fatal("archive retry changed archived manifest")
	}
	alreadyArchived := runJSONBinary(
		t,
		binary,
		1,
		"--json", "--dir", projectDir,
		"archive", "cli-contract",
		"--operation-id", "cli:archive:other",
	)
	if alreadyArchived.Error == nil || alreadyArchived.Error.Code != "already_archived" {
		t.Fatalf("already archived response = %#v", alreadyArchived)
	}
	reused := runJSONBinary(
		t,
		binary,
		1,
		"--json", "--dir", projectDir,
		"start", "cli-contract",
		"--workflow", "fast-change",
		"--title", "Reused archived ID",
	)
	if reused.Error == nil || reused.Error.Code != "already_exists" {
		t.Fatalf("reused archived ID response = %#v", reused)
	}
	immutable := runJSONBinary(
		t,
		binary,
		1,
		"--json", "--dir", projectDir,
		"record-event", "cli-contract",
		"--type", "archive.mutated",
		"--message", "must not be recorded",
	)
	if immutable.Error == nil || immutable.Error.Code != "not_found" {
		t.Fatalf("archived mutation response = %#v", immutable)
	}
}

func TestCLIValidateReportsMultipleFailuresWithoutMutation(t *testing.T) {
	binary := buildTestBinary(t)
	projectDir := t.TempDir()

	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "init")
	validProject := runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "validate")
	if responseData(t, validProject)["valid"] != true {
		t.Fatalf("project validation = %#v", validProject)
	}

	runJSONBinary(
		t,
		binary,
		0,
		"--json", "--dir", projectDir,
		"start", "validate-contract",
		"--workflow", "fast-change",
		"--title", "Validate contract",
	)
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "validate", "validate-contract")

	itemDir := filepath.Join(projectDir, ".sdd", "work-items", "active", "validate-contract")
	artifactPath := filepath.Join(itemDir, "artifacts", "plan.md")
	eventsPath := filepath.Join(itemDir, "events.jsonl")
	manifestPath := filepath.Join(itemDir, "manifest.yaml")
	if err := os.WriteFile(artifactPath, []byte("# Missing front matter\n"), 0644); err != nil {
		t.Fatalf("WriteFile() artifact error = %v", err)
	}
	events, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("ReadFile() events error = %v", err)
	}
	events = append(events, []byte("{invalid-json}\n")...)
	if err := os.WriteFile(eventsPath, events, 0644); err != nil {
		t.Fatalf("WriteFile() events error = %v", err)
	}
	beforeArtifact, _ := os.ReadFile(artifactPath)
	beforeEvents, _ := os.ReadFile(eventsPath)
	beforeManifest, _ := os.ReadFile(manifestPath)

	response := runJSONBinary(t, binary, 1, "--json", "--dir", projectDir, "validate", "validate-contract")
	if response.Error == nil || response.Error.Code != "validation_failed" {
		t.Fatalf("validation response = %#v", response)
	}
	details, ok := response.Error.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("validation details = %#v", response.Error.Details)
	}
	summary, ok := details["summary"].(map[string]interface{})
	if !ok || summary["failed"].(float64) < 2 {
		t.Fatalf("validation summary = %#v", details["summary"])
	}

	afterArtifact, _ := os.ReadFile(artifactPath)
	afterEvents, _ := os.ReadFile(eventsPath)
	afterManifest, _ := os.ReadFile(manifestPath)
	if !bytes.Equal(beforeArtifact, afterArtifact) ||
		!bytes.Equal(beforeEvents, afterEvents) ||
		!bytes.Equal(beforeManifest, afterManifest) {
		t.Fatal("validate modified the persisted work item")
	}
}

func TestCLIListsAndInstallsClaudeCodeAdapter(t *testing.T) {
	binary := buildTestBinary(t)
	projectDir := t.TempDir()

	listed := runJSONBinary(t, binary, 0, "--json", "adapters", "list")
	listData := responseData(t, listed)
	adapters, ok := listData["adapters"].([]interface{})
	if !ok || len(adapters) != 1 {
		t.Fatalf("adapters list = %#v", listData["adapters"])
	}
	claude, ok := adapters[0].(map[string]interface{})
	if !ok || claude["id"] != "claude-code" {
		t.Fatalf("adapter = %#v", adapters[0])
	}

	missingSDD := runJSONBinary(
		t,
		binary,
		1,
		"--json", "--dir", projectDir,
		"adapters", "install", "claude-code",
	)
	if missingSDD.Error == nil || missingSDD.Error.Code != "invalid_input" {
		t.Fatalf("missing .sdd response = %#v", missingSDD)
	}

	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "init")
	installed := runJSONBinary(
		t,
		binary,
		0,
		"--json", "--dir", projectDir,
		"adapters", "install", "claude-code",
	)
	installData := responseData(t, installed)
	if installData["id"] != "claude-code" {
		t.Fatalf("installation = %#v", installData)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "CLAUDE.md")); err != nil {
		t.Fatalf("CLAUDE.md missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".claude", "skills", "create-plan", "SKILL.md")); err != nil {
		t.Fatalf("create-plan skill missing: %v", err)
	}

	conflict := runJSONBinary(
		t,
		binary,
		1,
		"--json", "--dir", projectDir,
		"adapters", "install", "claude-code",
	)
	if conflict.Error == nil || conflict.Error.Code != "already_exists" {
		t.Fatalf("adapter conflict response = %#v", conflict)
	}

	unknown := runJSONBinary(
		t,
		binary,
		1,
		"--json", "--dir", projectDir,
		"adapters", "install", "unknown",
	)
	if unknown.Error == nil || unknown.Error.Code != "not_found" {
		t.Fatalf("unknown adapter response = %#v", unknown)
	}
}

// TestCLIReworkAndCancelCommands exercises the rework and cancel commands end-to-end
// using the compiled CLI binary (plan D: CA-RW-01/02/12, CA-CA-01/05/07/09/10).
func TestCLIReworkReactivatedWorkItemCanBeValidatedAndArchived(t *testing.T) {
	binary := buildTestBinary(t)
	projectDir := t.TempDir()
	id := "reactivate-e2e"

	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "init")
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir,
		"start", id, "--workflow", "fast-change", "--title", "Reactivation E2E test",
	)

	// plan starts in_progress; the remaining phases must be begun first.
	runPhases := func(round string, phases []string, begin map[string]bool, approve map[string]bool) {
		for _, phase := range phases {
			if begin[phase] {
				runJSONBinary(t, binary, 0, "--json", "--dir", projectDir,
					"begin", id, "--phase", phase, "--operation-id", id+":"+round+":begin:"+phase)
			}
			runJSONBinary(t, binary, 0, "--json", "--dir", projectDir,
				"deliver", id, "--phase", phase, "--operation-id", id+":"+round+":deliver:"+phase)
			if approve[phase] {
				runJSONBinary(t, binary, 0, "--json", "--dir", projectDir,
					"approve", id, "--phase", phase, "--by", "e2e-tester", "--operation-id", id+":"+round+":approve:"+phase)
			}
		}
		runJSONBinary(t, binary, 0, "--json", "--dir", projectDir,
			"complete", id, "--operation-id", id+":"+round+":complete")
		runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "validate", id)
	}

	downstream := []string{"implementation", "verification", "human-code-review", "archive"}
	beginAll := map[string]bool{"implementation": true, "verification": true, "human-code-review": true, "archive": true}
	approvals := map[string]bool{"plan": true, "human-code-review": true}

	runPhases("first", append([]string{"plan"}, downstream...), beginAll, approvals)

	// Rework a phase of the completed item: it is reactivated and must still validate.
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir,
		"rework", id, "--phase", "implementation", "--reason", "Defect found after completion",
		"--actor-kind", "human", "--actor-id", "e2e-tester",
	)
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "validate", id)

	// Complete it again and archive it: lifecycle continuity must hold.
	runPhases("second", downstream, beginAll, approvals)
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "archive", id)
}

func TestCLIReworkAndCancelCommands(t *testing.T) {
	binary := buildTestBinary(t)
	projectDir := t.TempDir()

	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "init")

	// ---- REWORK SCENARIO ----

	// Start fast-change work item and progress through plan approval and implementation delivery.
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir,
		"start", "rework-e2e",
		"--workflow", "fast-change",
		"--title", "Rework E2E test",
		"--operation-id", "rework-e2e:start",
	)
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir,
		"deliver", "rework-e2e", "--phase", "plan", "--operation-id", "rework-e2e:deliver:plan",
	)
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir,
		"approve", "rework-e2e", "--phase", "plan", "--by", "e2e-tester", "--operation-id", "rework-e2e:approve:plan",
	)
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir,
		"begin", "rework-e2e", "--phase", "implementation", "--operation-id", "rework-e2e:begin:impl",
	)
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir,
		"deliver", "rework-e2e", "--phase", "implementation", "--operation-id", "rework-e2e:deliver:impl",
	)

	// Rework plan → exit 0, JSON work_item with plan=ready and implementation=superseded.
	reworkResult := runJSONBinary(t, binary, 0, "--json", "--dir", projectDir,
		"rework", "rework-e2e",
		"--phase", "plan",
		"--reason", "Needs rethink",
		"--operation-id", "rework-e2e:rework:plan",
	)
	reworkData := responseData(t, reworkResult)
	workItem, ok := reworkData["work_item"].(map[string]interface{})
	if !ok {
		t.Fatalf("rework result work_item = %#v", reworkData["work_item"])
	}
	phases, ok := workItem["phases"].(map[string]interface{})
	if !ok {
		t.Fatalf("rework result phases = %#v", workItem["phases"])
	}
	planPhase, ok := phases["plan"].(map[string]interface{})
	if !ok || planPhase["status"] != "ready" {
		t.Fatalf("plan phase after rework = %#v", phases["plan"])
	}
	implPhase, ok := phases["implementation"].(map[string]interface{})
	if !ok || implPhase["status"] != "superseded" {
		t.Fatalf("implementation phase after rework = %#v", phases["implementation"])
	}

	// Idempotency: retry rework with same operation-id → events file unchanged.
	reworkEventsPath := filepath.Join(projectDir, ".sdd", "work-items", "active", "rework-e2e", "events.jsonl")
	eventsBeforeRetry, err := os.ReadFile(reworkEventsPath)
	if err != nil {
		t.Fatalf("ReadFile() rework events error = %v", err)
	}
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir,
		"rework", "rework-e2e",
		"--phase", "plan",
		"--reason", "Needs rethink",
		"--operation-id", "rework-e2e:rework:plan", // same operation-id
	)
	eventsAfterRetry, err := os.ReadFile(reworkEventsPath)
	if err != nil {
		t.Fatalf("ReadFile() rework events after retry error = %v", err)
	}
	if !bytes.Equal(eventsBeforeRetry, eventsAfterRetry) {
		t.Fatal("rework retry changed the events file (not idempotent)")
	}

	// Begin plan after rework succeeds (CA-RW-02).
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir,
		"begin", "rework-e2e", "--phase", "plan", "--operation-id", "rework-e2e:begin:plan:2",
	)

	// Error: rework without --reason → invalid_arguments (ESC-RW-06).
	noReason := runJSONBinary(t, binary, 1, "--json", "--dir", projectDir,
		"rework", "rework-e2e", "--phase", "plan",
	)
	if noReason.Error == nil || noReason.Error.Code != "invalid_arguments" {
		t.Fatalf("no-reason rework = %#v", noReason)
	}

	// Error: rework with non-existent phase → not_found (ESC-RW-05).
	badPhase := runJSONBinary(t, binary, 1, "--json", "--dir", projectDir,
		"rework", "rework-e2e", "--phase", "nonexistent", "--reason", "test",
	)
	if badPhase.Error == nil || badPhase.Error.Code != "not_found" {
		t.Fatalf("bad-phase rework = %#v", badPhase)
	}

	// ---- CANCEL SCENARIO ----

	// Start a separate work item for the cancel tests.
	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir,
		"start", "cancel-e2e",
		"--workflow", "fast-change",
		"--title", "Cancel E2E test",
		"--operation-id", "cancel-e2e:start",
	)

	// Error: cancel without --reason → invalid_arguments.
	noReasonCancel := runJSONBinary(t, binary, 1, "--json", "--dir", projectDir,
		"cancel", "cancel-e2e",
	)
	if noReasonCancel.Error == nil || noReasonCancel.Error.Code != "invalid_arguments" {
		t.Fatalf("no-reason cancel = %#v", noReasonCancel)
	}

	// Successful cancel with agent actor (CA-CA-07): exit 0, text output.
	stdout, stderr, exitCode := runTestBinary(binary,
		"--dir", projectDir,
		"cancel", "cancel-e2e",
		"--reason", "No longer needed",
		"--actor-kind", "agent",
		"--actor-id", "sdd-agent",
		"--operation-id", "cancel-e2e:cancel",
	)
	if exitCode != 0 || stderr != "" {
		t.Fatalf("cancel exit = %d, stderr = %q", exitCode, stderr)
	}
	if !strings.Contains(stdout, "Work item 'cancel-e2e' cancelled.") {
		t.Fatalf("cancel text output = %q", stdout)
	}

	// Active directory must no longer exist (CA-CA-01).
	activePath := filepath.Join(projectDir, ".sdd", "work-items", "active", "cancel-e2e")
	if _, err := os.Stat(activePath); !os.IsNotExist(err) {
		t.Fatalf("active directory still exists after cancel: %v", err)
	}
	// Cancelled directory and manifest must exist (CA-CA-01).
	cancelledManifest := filepath.Join(projectDir, ".sdd", "work-items", "cancelled", "cancel-e2e", "manifest.yaml")
	if _, err := os.Stat(cancelledManifest); err != nil {
		t.Fatalf("cancelled manifest missing: %v", err)
	}

	// Idempotency: repeat cancel with same operation-id → no new events (CA-CA-10).
	cancelEventsPath := filepath.Join(projectDir, ".sdd", "work-items", "cancelled", "cancel-e2e", "events.jsonl")
	cancelEventsBeforeRetry, err := os.ReadFile(cancelEventsPath)
	if err != nil {
		t.Fatalf("ReadFile() cancel events error = %v", err)
	}
	cancelRetried := runJSONBinary(t, binary, 0, "--json", "--dir", projectDir,
		"cancel", "cancel-e2e",
		"--reason", "No longer needed",
		"--operation-id", "cancel-e2e:cancel", // same operation-id
	)
	cancelEventsAfterRetry, err := os.ReadFile(cancelEventsPath)
	if err != nil {
		t.Fatalf("ReadFile() cancel events after retry error = %v", err)
	}
	if !bytes.Equal(cancelEventsBeforeRetry, cancelEventsAfterRetry) {
		t.Fatal("cancel retry changed the events file (not idempotent)")
	}
	cancelRetryData := responseData(t, cancelRetried)
	if cancelRetryData["location"] != "cancelled" {
		t.Fatalf("cancel retry location = %#v", cancelRetryData["location"])
	}

	// ID reservation: start with the same ID → already_exists with "cancelled" in message (CA-CA-05).
	reserved := runJSONBinary(t, binary, 1, "--json", "--dir", projectDir,
		"start", "cancel-e2e",
		"--workflow", "fast-change",
		"--title", "Reused cancelled ID",
	)
	if reserved.Error == nil || reserved.Error.Code != "already_exists" {
		t.Fatalf("reserved cancelled ID = %#v", reserved)
	}
	if !strings.Contains(reserved.Error.Message, "cancel") {
		t.Fatalf("expected 'cancel' in already_exists message, got %q", reserved.Error.Message)
	}

	// Opacity: status on cancelled ID → not_found (CA-CA-09 / ESC-CA-06).
	cancelledStatus := runJSONBinary(t, binary, 1, "--json", "--dir", projectDir, "status", "cancel-e2e")
	if cancelledStatus.Error == nil || cancelledStatus.Error.Code != "not_found" {
		t.Fatalf("cancelled status = %#v", cancelledStatus)
	}

	// Opacity: next on cancelled ID → not_found.
	cancelledNext := runJSONBinary(t, binary, 1, "--json", "--dir", projectDir, "next", "cancel-e2e")
	if cancelledNext.Error == nil || cancelledNext.Error.Code != "not_found" {
		t.Fatalf("cancelled next = %#v", cancelledNext)
	}

	// Opacity: validate on cancelled ID → not_found.
	cancelledValidate := runJSONBinary(t, binary, 1, "--json", "--dir", projectDir, "validate", "cancel-e2e")
	if cancelledValidate.Error == nil || cancelledValidate.Error.Code != "not_found" {
		t.Fatalf("cancelled validate = %#v", cancelledValidate)
	}

	// Attempting to cancel again with a different operation-id → invalid_transition (already cancelled).
	alreadyCancelled := runJSONBinary(t, binary, 1, "--json", "--dir", projectDir,
		"cancel", "cancel-e2e",
		"--reason", "Duplicate cancel attempt",
		"--operation-id", "cancel-e2e:cancel:other",
	)
	if alreadyCancelled.Error == nil || alreadyCancelled.Error.Code != "invalid_transition" {
		t.Fatalf("already-cancelled response = %#v", alreadyCancelled)
	}
}

func buildTestBinary(t *testing.T) string {
	t.Helper()
	moduleDir, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}
	binary := filepath.Join(t.TempDir(), "sdd")
	command := exec.Command("go", "build", "-o", binary, ".")
	command.Dir = moduleDir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("go build error = %v\n%s", err, output)
	}
	return binary
}

func runJSONBinary(t *testing.T, binary string, wantExit int, args ...string) JSONResponse {
	t.Helper()
	stdout, stderr, exitCode := runTestBinary(binary, args...)
	if exitCode != wantExit {
		t.Fatalf("sdd %v exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", args, exitCode, wantExit, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("sdd %v stderr = %q", args, stderr)
	}
	var response JSONResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("Unmarshal() response error = %v\noutput:\n%s", err, stdout)
	}
	if response.Success != (wantExit == 0) {
		t.Fatalf("sdd %v response = %#v", args, response)
	}
	return response
}

func runTestBinary(binary string, args ...string) (string, string, int) {
	var stdout, stderr bytes.Buffer
	command := exec.Command(binary, args...)
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			return stdout.String(), stderr.String(), -1
		}
	}
	return stdout.String(), stderr.String(), command.ProcessState.ExitCode()
}

func responseData(t *testing.T, response JSONResponse) map[string]interface{} {
	t.Helper()
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("response data = %#v", response.Data)
	}
	return data
}
