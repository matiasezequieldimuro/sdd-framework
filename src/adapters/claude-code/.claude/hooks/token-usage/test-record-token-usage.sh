#!/usr/bin/env bash
# test-record-token-usage.sh
#
# Manual integration test for the record-token-usage.py hook.
# Exercises the hook against a real temporary SDD project without modifying
# any production state. Covers the edge cases requested in the plan:
#     — .active-work-item absent / invalid ID
#     — audit inactive (not_reported work item)
#     — transcript partial / empty
#     — full recording with active audit
#
# Usage (from anywhere; absolute paths are used throughout):
#   bash <path-to-this-script>
#
# Requirements:
#   - sdd-cli must be in PATH and must be built from current source
#     (go build -o ~/bin/sdd-cli . && PATH=~/bin:$PATH bash test-...)
#   - python3 must be in PATH
#   - The hook script is adjacent to this file

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$SCRIPT_DIR/record-token-usage.py"
PASS=0
FAIL=0

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

pass() { echo "  PASS: $1"; PASS=$((PASS+1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL+1)); }

# Create a minimal SDD project with a fast-change work item in TMPDIR.
# Sets PROJECT_DIR and ITEM_ID as caller-local variables.
setup_project() {
    local active="${1:-false}"
    PROJECT_DIR="$(mktemp -d)"
    ITEM_ID="hook-test-item"
    trap "rm -rf '$PROJECT_DIR'" EXIT

    sdd-cli init --dir "$PROJECT_DIR" > /dev/null 2>&1

    # Optionally activate token audit.
    if [[ "$active" == "true" ]]; then
        sed -i 's/token_usage: false/token_usage: true/; s/token_usage: optional/token_usage: true/' \
            "$PROJECT_DIR/.sdd/config.yaml"
    fi

    sdd-cli start "$ITEM_ID" \
        --dir "$PROJECT_DIR" \
        --workflow fast-change \
        --title "Hook test" \
        > /dev/null 2>&1
}

# Build a minimal transcript JSONL at TRANSCRIPT_PATH with N assistant messages.
write_transcript() {
    local path="$1"
    local count="${2:-3}"
    mkdir -p "$(dirname "$path")"
    : > "$path"
    for ((i=1; i<=count; i++)); do
        printf '{"message":{"role":"assistant","usage":{"input_tokens":%d,"output_tokens":%d,"cache_read_input_tokens":%d,"cache_creation_input_tokens":%d}}}\n' \
            $((100*i)) $((50*i)) $((10*i)) $((5*i)) >> "$path"
    done
}

# Invoke the hook with a fabricated stdin payload and capture its exit code.
run_hook() {
    local project_dir="$1"
    local session_id="${2:-ses-test-001}"
    local transcript_path="${3:-/nonexistent/transcript.jsonl}"

    local payload
    payload="$(printf '{"hook_event_name":"Stop","session_id":"%s","transcript_path":"%s","cwd":"%s"}' \
        "$session_id" "$transcript_path" "$project_dir")"

    CLAUDE_PROJECT_DIR="$project_dir" \
        python3 "$HOOK" <<< "$payload"
    # Hook always exits 0; the test inspects log files for errors.
}

# Return the number of log files in the logs directory.
log_count() {
    local project_dir="$1"
    local log_dir="$project_dir/.claude/hooks/token-usage/logs"
    [[ -d "$log_dir" ]] || { echo 0; return; }
    find "$log_dir" -name "*.log" | wc -l | tr -d ' '
}

# ---------------------------------------------------------------------------
# Test cases
# ---------------------------------------------------------------------------

echo ""
echo "=== Token usage hook — manual integration tests ==="
echo ""

# --- T1: .active-work-item absent  ---
echo "T1: .active-work-item absent"
setup_project "false"
# Do NOT create .active-work-item.
run_hook "$PROJECT_DIR"
if [[ "$(log_count "$PROJECT_DIR")" -gt 0 ]]; then
    pass "absent .active-work-item → error log created"
else
    fail "absent .active-work-item → no error log (expected one)"
fi
rm -rf "$PROJECT_DIR"
trap - EXIT

# --- T2: .active-work-item with invalid ID  ---
echo "T2: invalid ID in .active-work-item"
setup_project "false"
echo "INVALID_ID_WITH_UPPERCASE" > "$PROJECT_DIR/.active-work-item"
run_hook "$PROJECT_DIR"
if [[ "$(log_count "$PROJECT_DIR")" -gt 0 ]]; then
    pass "invalid ID → error log created"
else
    fail "invalid ID → no error log"
fi
rm -rf "$PROJECT_DIR"
trap - EXIT

# --- T3: audit inactive  ---
echo "T3: audit inactive (not_reported work item)"
setup_project "false"
echo "$ITEM_ID" > "$PROJECT_DIR/.active-work-item"
TRANSCRIPT="$PROJECT_DIR/transcript.jsonl"
write_transcript "$TRANSCRIPT" 2
run_hook "$PROJECT_DIR" "ses-001" "$TRANSCRIPT"
if [[ "$(log_count "$PROJECT_DIR")" -gt 0 ]]; then
    pass "inactive audit → error logged (token_audit_inactive)"
else
    fail "inactive audit → no error log"
fi
rm -rf "$PROJECT_DIR"
trap - EXIT

# --- T4: empty transcript → hook skips  ---
# An empty transcript yields n=0 assistant messages. The hook must NOT call
# record-tokens (which would overwrite a previous total with zeros). The
# manifest must remain in its initial "partial" state.
echo "T4: empty transcript → hook skips, manifest stays partial (F1/RN-3)"
setup_project "true"
echo "$ITEM_ID" > "$PROJECT_DIR/.active-work-item"
TRANSCRIPT="$PROJECT_DIR/empty.jsonl"
: > "$TRANSCRIPT"    # empty file
run_hook "$PROJECT_DIR" "ses-002" "$TRANSCRIPT"
if [[ "$(log_count "$PROJECT_DIR")" -eq 0 ]]; then
    pass "empty transcript → no error log (hook exited silently)"
else
    fail "empty transcript → unexpected error log"
fi
MANIFEST="$PROJECT_DIR/.sdd/work-items/active/$ITEM_ID/manifest.yaml"
if grep -q "status: partial" "$MANIFEST"; then
    pass "empty transcript → manifest stays partial (not overwritten with zeros)"
else
    fail "empty transcript → manifest status is not 'partial' (F1 guard may be broken)"
fi
rm -rf "$PROJECT_DIR"
trap - EXIT

# --- T5: happy path — active audit, real transcript  ---
echo "T5: happy path (active audit, 3-turn transcript)"
setup_project "true"
echo "$ITEM_ID" > "$PROJECT_DIR/.active-work-item"
TRANSCRIPT="$PROJECT_DIR/transcript.jsonl"
write_transcript "$TRANSCRIPT" 3
run_hook "$PROJECT_DIR" "ses-003" "$TRANSCRIPT"
if [[ "$(log_count "$PROJECT_DIR")" -eq 0 ]]; then
    pass "happy path → no error log"
else
    fail "happy path → unexpected error log"
fi
MANIFEST="$PROJECT_DIR/.sdd/work-items/active/$ITEM_ID/manifest.yaml"
if grep -q "status: recorded" "$MANIFEST"; then
    pass "happy path → manifest shows recorded"
else
    fail "happy path → manifest not recorded"
fi
# Verify total = sum of 3 turns: input 600, output 300, cache_read 60, cache_write 30 → total 990
if grep -q "total_tokens: 990" "$MANIFEST"; then
    pass "happy path → total_tokens = 990"
else
    fail "happy path → total_tokens mismatch (check manifest for actual value)"
fi
rm -rf "$PROJECT_DIR"
trap - EXIT

# --- T6: idempotency — same session_id same N → no duplicate event ---
echo "T6: idempotency (same operation-id replayed)"
setup_project "true"
echo "$ITEM_ID" > "$PROJECT_DIR/.active-work-item"
TRANSCRIPT="$PROJECT_DIR/transcript.jsonl"
write_transcript "$TRANSCRIPT" 2
run_hook "$PROJECT_DIR" "ses-004" "$TRANSCRIPT"
EVENTS_BEFORE=$(wc -l < "$PROJECT_DIR/.sdd/work-items/active/$ITEM_ID/events.jsonl")
run_hook "$PROJECT_DIR" "ses-004" "$TRANSCRIPT"    # same session, same N=2
EVENTS_AFTER=$(wc -l < "$PROJECT_DIR/.sdd/work-items/active/$ITEM_ID/events.jsonl")
if [[ "$EVENTS_BEFORE" -eq "$EVENTS_AFTER" ]]; then
    pass "idempotent replay → no duplicate event"
else
    fail "idempotent replay → event count changed ($EVENTS_BEFORE → $EVENTS_AFTER)"
fi
rm -rf "$PROJECT_DIR"
trap - EXIT

# --- T7: unreadable transcript (OSError) → error logged, manifest unchanged ---
# Simulates the async-lag scenario  where the transcript path is a
# directory (causes OSError on open). The hook must write a diagnostic log and
# exit 0 WITHOUT calling record-tokens, so the previously recorded state is
# preserved  .
echo "T7: unreadable transcript (OSError) → error logged, manifest unchanged "
setup_project "true"
echo "$ITEM_ID" > "$PROJECT_DIR/.active-work-item"
# Use a directory path as transcript to trigger OSError.
TRANSCRIPT_DIR="$PROJECT_DIR/transcript-dir.jsonl"
mkdir -p "$TRANSCRIPT_DIR"
run_hook "$PROJECT_DIR" "ses-007" "$TRANSCRIPT_DIR"
if [[ "$(log_count "$PROJECT_DIR")" -gt 0 ]]; then
    pass "unreadable transcript → error log written"
else
    fail "unreadable transcript → no error log (expected transcript_read_error)"
fi
MANIFEST="$PROJECT_DIR/.sdd/work-items/active/$ITEM_ID/manifest.yaml"
if grep -q "status: partial" "$MANIFEST"; then
    pass "unreadable transcript → manifest unchanged (stays partial)"
else
    fail "unreadable transcript → manifest was overwritten unexpectedly"
fi
rm -rf "$PROJECT_DIR"
trap - EXIT

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo "Results: $PASS passed, $FAIL failed"
echo ""
[[ "$FAIL" -eq 0 ]]
