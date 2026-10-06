package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// activateTokenAudit patches config.yaml in projectDir to enable the token
// usage audit (token_usage: true). It handles both the old "optional" value
// and the new "false" default.
func activateTokenAudit(t *testing.T, projectDir string) {
	t.Helper()
	configPath := filepath.Join(projectDir, ".sdd", "config.yaml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile config.yaml: %v", err)
	}
	updated := strings.ReplaceAll(string(content), "token_usage: false", "token_usage: true")
	updated = strings.ReplaceAll(updated, "token_usage: optional", "token_usage: true")
	if err := os.WriteFile(configPath, []byte(updated), 0644); err != nil {
		t.Fatalf("WriteFile config.yaml: %v", err)
	}
}

// TestCLIRecordTokens_ActiveAudit_SuccessAndOverwrite verifies E-01 + E-05:
// happy path recording and overwrite on an active-audit work item.
func TestCLIRecordTokens_ActiveAudit_SuccessAndOverwrite(t *testing.T) {
	binary := buildTestBinary(t)
	projectDir := t.TempDir()

	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "init")
	activateTokenAudit(t, projectDir)

	runJSONBinary(
		t, binary, 0,
		"--json", "--dir", projectDir,
		"start", "active-item",
		"--workflow", "fast-change",
		"--title", "Active audit item",
	)

	// First recording.
	runJSONBinary(
		t, binary, 0,
		"--json", "--dir", projectDir,
		"record-tokens", "active-item",
		"--input-tokens", "200",
		"--output-tokens", "80",
		"--cache-read-tokens", "0",
		"--cache-write-tokens", "0",
		"--source", "claude-code",
		"--operation-id", "tokens:ses001:3",
	)

	// Overwrite (E-01): second recording replaces the first.
	resp := runJSONBinary(
		t, binary, 0,
		"--json", "--dir", projectDir,
		"record-tokens", "active-item",
		"--input-tokens", "500",
		"--output-tokens", "300",
		"--cache-read-tokens", "100",
		"--cache-write-tokens", "50",
		"--source", "claude-code",
		"--operation-id", "tokens:ses001:7",
	)
	_ = resp // success: exit 0 asserted by runJSONBinary

	// Verify via status --json.
	status := runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "status", "active-item")
	statusData := responseData(t, status)
	obs, _ := statusData["observability"].(map[string]interface{})
	if obs == nil {
		t.Fatal("observability missing from status JSON")
	}
	tu, _ := obs["token_usage"].(map[string]interface{})
	if tu == nil {
		t.Fatal("token_usage missing from observability")
	}
	if tu["status"] != "recorded" {
		t.Errorf("token_usage status = %v, want recorded", tu["status"])
	}
	if tu["total_tokens"].(float64) != 950 {
		t.Errorf("total_tokens = %v, want 950", tu["total_tokens"])
	}
	if tu["input_tokens"].(float64) != 500 {
		t.Errorf("input_tokens = %v, want 500", tu["input_tokens"])
	}
}

// TestCLIRecordTokens_ActiveAudit_CreatedWithZeros verifies E-02: after init
// with active audit, the manifest starts with status=partial and counters=0.
func TestCLIRecordTokens_ActiveAudit_CreatedWithZeros(t *testing.T) {
	binary := buildTestBinary(t)
	projectDir := t.TempDir()

	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "init")
	activateTokenAudit(t, projectDir)
	runJSONBinary(
		t, binary, 0,
		"--json", "--dir", projectDir,
		"start", "audit-init-item",
		"--workflow", "fast-change",
		"--title", "Audit init",
	)

	status := runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "status", "audit-init-item")
	statusData := responseData(t, status)
	obs, _ := statusData["observability"].(map[string]interface{})
	if obs == nil {
		t.Fatal("observability missing")
	}
	tu, _ := obs["token_usage"].(map[string]interface{})
	if tu == nil {
		t.Fatal("token_usage missing")
	}
	if tu["status"] != "partial" {
		t.Errorf("initial status = %v, want partial", tu["status"])
	}
	if tu["total_tokens"].(float64) != 0 {
		t.Errorf("initial total_tokens = %v, want 0", tu["total_tokens"])
	}
}

// TestCLIRecordTokens_InactiveAudit_ReturnsError verifies E-03 / E-12:
// recording on an inactive-audit item returns token_audit_inactive.
func TestCLIRecordTokens_InactiveAudit_ReturnsError(t *testing.T) {
	binary := buildTestBinary(t)
	projectDir := t.TempDir()

	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "init")
	// Do NOT activate audit — default is inactive.
	runJSONBinary(
		t, binary, 0,
		"--json", "--dir", projectDir,
		"start", "inactive-item",
		"--workflow", "fast-change",
		"--title", "Inactive audit item",
	)

	resp := runJSONBinary(
		t, binary, 1, // expect non-zero exit
		"--json", "--dir", projectDir,
		"record-tokens", "inactive-item",
		"--input-tokens", "100",
		"--output-tokens", "50",
		"--cache-read-tokens", "0",
		"--cache-write-tokens", "0",
		"--source", "claude-code",
	)
	if resp.Error == nil || resp.Error.Code != "token_audit_inactive" {
		t.Fatalf("inactive audit response = %#v, want token_audit_inactive", resp)
	}

	// Verify manifest is untouched (counters still nil).
	status := runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "status", "inactive-item")
	statusData := responseData(t, status)
	obs, _ := statusData["observability"].(map[string]interface{})
	if obs == nil {
		t.Fatal("observability missing")
	}
	tu, _ := obs["token_usage"].(map[string]interface{})
	if tu == nil {
		t.Fatal("token_usage missing")
	}
	if tu["status"] != "not_reported" {
		t.Errorf("status after reject = %v, want not_reported", tu["status"])
	}
}

// TestCLIRecordTokens_NegativeInput_ManifestIntact verifies E-06: a negative
// counter value is rejected and the manifest stays unchanged.
func TestCLIRecordTokens_NegativeInput_ManifestIntact(t *testing.T) {
	binary := buildTestBinary(t)
	projectDir := t.TempDir()

	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "init")
	activateTokenAudit(t, projectDir)
	runJSONBinary(
		t, binary, 0,
		"--json", "--dir", projectDir,
		"start", "neg-item",
		"--workflow", "fast-change",
		"--title", "Negative test item",
	)
	// Record valid values first.
	runJSONBinary(
		t, binary, 0,
		"--json", "--dir", projectDir,
		"record-tokens", "neg-item",
		"--input-tokens", "200",
		"--output-tokens", "100",
		"--cache-read-tokens", "0",
		"--cache-write-tokens", "0",
		"--source", "claude-code",
	)

	manifestPath := filepath.Join(projectDir, ".sdd", "work-items", "active", "neg-item", "manifest.yaml")
	before, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile manifest: %v", err)
	}

	// Attempt invalid recording with negative input.
	resp := runJSONBinary(
		t, binary, 1,
		"--json", "--dir", projectDir,
		"record-tokens", "neg-item",
		"--input-tokens", "-10",
		"--output-tokens", "100",
		"--cache-read-tokens", "0",
		"--cache-write-tokens", "0",
		"--source", "claude-code",
	)
	if resp.Error == nil || resp.Error.Code != "validation_failed" {
		t.Fatalf("negative input response = %#v, want validation_failed", resp)
	}

	// Manifest must be unchanged.
	after, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile manifest after rejection: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("manifest was modified despite validation failure")
	}
}

// TestCLIRecordTokens_AllZero_Valid verifies E-09: recording all counters as 0
// is accepted and status becomes "recorded".
func TestCLIRecordTokens_AllZero_Valid(t *testing.T) {
	binary := buildTestBinary(t)
	projectDir := t.TempDir()

	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "init")
	activateTokenAudit(t, projectDir)
	runJSONBinary(
		t, binary, 0,
		"--json", "--dir", projectDir,
		"start", "zero-item",
		"--workflow", "fast-change",
		"--title", "Zero tokens",
	)

	runJSONBinary(
		t, binary, 0,
		"--json", "--dir", projectDir,
		"record-tokens", "zero-item",
		"--input-tokens", "0",
		"--output-tokens", "0",
		"--cache-read-tokens", "0",
		"--cache-write-tokens", "0",
		"--source", "claude-code",
	)

	status := runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "status", "zero-item")
	statusData := responseData(t, status)
	obs, _ := statusData["observability"].(map[string]interface{})
	tu, _ := obs["token_usage"].(map[string]interface{})
	if tu["status"] != "recorded" {
		t.Errorf("status after zero recording = %v, want recorded", tu["status"])
	}
	if tu["total_tokens"].(float64) != 0 {
		t.Errorf("total_tokens = %v, want 0", tu["total_tokens"])
	}
}

// TestCLIRecordTokens_Idempotent verifies RC-9: same operation-id does not
// duplicate events; different operation-id creates a new event.
func TestCLIRecordTokens_Idempotent(t *testing.T) {
	binary := buildTestBinary(t)
	projectDir := t.TempDir()

	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "init")
	activateTokenAudit(t, projectDir)
	runJSONBinary(
		t, binary, 0,
		"--json", "--dir", projectDir,
		"start", "idem-item",
		"--workflow", "fast-change",
		"--title", "Idempotent item",
	)

	eventsPath := filepath.Join(projectDir, ".sdd", "work-items", "active", "idem-item", "events.jsonl")

	runJSONBinary(
		t, binary, 0,
		"--json", "--dir", projectDir,
		"record-tokens", "idem-item",
		"--input-tokens", "100",
		"--output-tokens", "50",
		"--cache-read-tokens", "0",
		"--cache-write-tokens", "0",
		"--source", "claude-code",
		"--operation-id", "tokens:ses:5",
	)
	after1, _ := os.ReadFile(eventsPath)

	// Replay same operation-id — must not add event.
	runJSONBinary(
		t, binary, 0,
		"--json", "--dir", projectDir,
		"record-tokens", "idem-item",
		"--input-tokens", "100",
		"--output-tokens", "50",
		"--cache-read-tokens", "0",
		"--cache-write-tokens", "0",
		"--source", "claude-code",
		"--operation-id", "tokens:ses:5",
	)
	after2, _ := os.ReadFile(eventsPath)
	if string(after1) != string(after2) {
		t.Fatal("idempotent replay added a duplicate event")
	}

	// Different operation-id — must add a new event.
	runJSONBinary(
		t, binary, 0,
		"--json", "--dir", projectDir,
		"record-tokens", "idem-item",
		"--input-tokens", "200",
		"--output-tokens", "80",
		"--cache-read-tokens", "0",
		"--cache-write-tokens", "0",
		"--source", "claude-code",
		"--operation-id", "tokens:ses:9",
	)
	after3, _ := os.ReadFile(eventsPath)
	if string(after2) == string(after3) {
		t.Fatal("different operation-id did not add a new event")
	}
}

// TestCLIStatus_TextIncludesTokenUsage verifies E-10: the text output of
// "status" includes the Token Usage block when the audit is active.
func TestCLIStatus_TextIncludesTokenUsage(t *testing.T) {
	binary := buildTestBinary(t)
	projectDir := t.TempDir()

	runJSONBinary(t, binary, 0, "--json", "--dir", projectDir, "init")
	activateTokenAudit(t, projectDir)
	runJSONBinary(
		t, binary, 0,
		"--json", "--dir", projectDir,
		"start", "status-item",
		"--workflow", "fast-change",
		"--title", "Status token item",
	)
	runJSONBinary(
		t, binary, 0,
		"--json", "--dir", projectDir,
		"record-tokens", "status-item",
		"--input-tokens", "300",
		"--output-tokens", "150",
		"--cache-read-tokens", "50",
		"--cache-write-tokens", "20",
		"--source", "claude-code",
	)

	stdout, stderr, exitCode := runTestBinary(binary, "--dir", projectDir, "status", "status-item")
	if exitCode != 0 || stderr != "" {
		t.Fatalf("text status exit=%d stderr=%q", exitCode, stderr)
	}
	for _, expected := range []string{"Token Usage:", "recorded", "claude-code", "520"} {
		if !strings.Contains(stdout, expected) {
			t.Errorf("text status missing %q\nfull output:\n%s", expected, stdout)
		}
	}
}
