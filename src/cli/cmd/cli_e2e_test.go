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
