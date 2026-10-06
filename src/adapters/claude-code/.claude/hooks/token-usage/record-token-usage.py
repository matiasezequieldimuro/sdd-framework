#!/usr/bin/env python3
"""
Record cumulative token usage for the active SDD work item at the end of each
Claude Code turn (Stop hook).

Design decisions (from the approved plan):
  — ADAPTER_ID is the single constant that identifies this adapter. Its
          value matches the id in adapter.yaml ("claude-code"). It is reused for
          --source and for the error log filename; no literals elsewhere.
  — operation-id = "tokens:<session_id>:<N>" where N = number of assistant
          messages counted in the transcript. Same N → idempotent; new turn →
          new N → overwrites previous total.
  — Errors are written to:
          ${CLAUDE_PROJECT_DIR}/.claude/hooks/token-usage/logs/
          token-usage-error-<UTC-ISO8601>-<ADAPTER_ID>.log
    — The hook sums the entire transcript on every call (acumulative
          recalculation). Missing the last turn is recovered on the next Stop.
    — The hook contains all LLM-capture logic; the engine (sdd-cli) only
          persists what it receives.
    — Work item ID is read from ${CLAUDE_PROJECT_DIR}/.active-work-item.
          Invalid/absent → log + exit 0 (never block the session).
  Never blocks: always exits with 0; only exit 2 blocks Claude, and we avoid it.
"""

import json
import os
import re
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

# ---------------------------------------------------------------------------
# Adapter identity: single constant, reused for --source and log names.
# ---------------------------------------------------------------------------
ADAPTER_ID = "claude-code"

# Regex for valid kebab-case work item identifiers (RC-6).
WORK_ITEM_ID_RE = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")


# ---------------------------------------------------------------------------
# Logging helpers
# ---------------------------------------------------------------------------

def _log_dir(project_dir: Path) -> Path:
    """Return the directory where error logs for this adapter are written."""
    return project_dir / ".claude" / "hooks" / "token-usage" / "logs"


def write_error_log(
    project_dir: Path,
    hook_event: str,
    session_id: str,
    work_item: str,
    error_kind: str,
    message: str,
    extra: str = "",
) -> None:
    """
    Persist a diagnostic entry to a time-stamped log file .

    Parameters
    ----------
    project_dir : Path
        Root of the worktree (value of ${CLAUDE_PROJECT_DIR}).
    hook_event : str
        Name of the hook lifecycle event (e.g. "Stop").
    session_id : str
        Claude Code session identifier from stdin.
    work_item : str
        Resolved work item ID, or a description of why resolution failed.
    error_kind : str
        Short error code / category for quick triage.
    message : str
        Human-readable error description.
    extra : str
        Optional additional context (e.g. sdd-cli JSON output).
    """
    try:
        log_directory = _log_dir(project_dir)
        log_directory.mkdir(parents=True, exist_ok=True)
        timestamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
        filename = f"token-usage-error-{timestamp}-{ADAPTER_ID}.log"
        log_path = log_directory / filename
        lines = [
            f"timestamp:   {timestamp}",
            f"hook_event:  {hook_event}",
            f"session_id:  {session_id}",
            f"work_item:   {work_item}",
            f"error_kind:  {error_kind}",
            f"message:     {message}",
        ]
        if extra:
            lines.append(f"extra:       {extra}")
        log_path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    except Exception as exc:  # pragma: no cover — best-effort logging
        # If even logging fails we still must not block the session.
        print(f"[token-usage hook] logging failed: {exc}", file=sys.stderr)


# ---------------------------------------------------------------------------
# Work item resolution
# ---------------------------------------------------------------------------

def resolve_work_item(project_dir: Path) -> str:
    """
    Read and validate the work item ID from .active-work-item.

    Returns
    -------
    str
        Validated kebab-case work item ID.

    Raises
    ------
    ValueError
        If the file is absent, empty, or contains an invalid ID.
    """
    active_file = project_dir / ".active-work-item"
    if not active_file.exists():
        raise ValueError(".active-work-item not found in project root")
    work_item_id = active_file.read_text(encoding="utf-8").strip()
    if not work_item_id:
        raise ValueError(".active-work-item is empty")
    if not WORK_ITEM_ID_RE.match(work_item_id):
        raise ValueError(
            f".active-work-item contains invalid ID: {work_item_id!r} "
            "(expected kebab-case: ^[a-z0-9]+(?:-[a-z0-9]+)*$)"
        )
    return work_item_id


# ---------------------------------------------------------------------------
# Transcript parsing
# ---------------------------------------------------------------------------

def sum_token_usage(transcript_path: str) -> tuple[int, int, int, int, int]:
    """
    Parse the transcript JSONL and sum token counters across all assistant
    messages, including sidechain (sub-agent) messages.

    All four counter fields default to 0 when absent from a usage block.

    Parameters
    ----------
    transcript_path : str
        Absolute path to the session transcript JSONL file.

    Returns
    -------
    tuple[int, int, int, int, int]
        (input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, N)
        where N is the count of assistant messages (used for operation-id D-f).
    """
    input_tokens = 0
    output_tokens = 0
    cache_read_tokens = 0
    cache_write_tokens = 0
    assistant_message_count = 0

    try:
        with open(transcript_path, encoding="utf-8") as fh:
            for line in fh:
                line = line.strip()
                if not line:
                    continue
                try:
                    entry = json.loads(line)
                except json.JSONDecodeError:
                    continue  # Skip malformed lines (transcript may be partial).

                # We only care about assistant messages with usage data.
                message = entry.get("message", {})
                if not isinstance(message, dict):
                    continue
                if message.get("role") != "assistant":
                    continue

                usage = message.get("usage", {})
                if not isinstance(usage, dict):
                    continue

                # isSidechain: True for sub-agent messages — still counted (REQ-5).
                assistant_message_count += 1
                input_tokens += usage.get("input_tokens", 0) or 0
                output_tokens += usage.get("output_tokens", 0) or 0
                cache_read_tokens += usage.get("cache_read_input_tokens", 0) or 0
                cache_write_tokens += usage.get("cache_creation_input_tokens", 0) or 0

    except OSError:
        # Transcript file not yet written or locked (async lag).
        # Re-raise to the caller so it can log a diagnostic and skip the
        # record-tokens call without overwriting the previous total.
        raise

    return input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, assistant_message_count


# ---------------------------------------------------------------------------
# Main hook logic
# ---------------------------------------------------------------------------

def main() -> int:
    """
    Entry point for the Stop hook. Always returns 0 to avoid blocking the
    Claude Code session.
    """
    project_dir_str = os.environ.get("CLAUDE_PROJECT_DIR", os.getcwd())
    project_dir = Path(project_dir_str).resolve()

    # Parse the hook payload from stdin.
    try:
        hook_input = json.load(sys.stdin)
    except (json.JSONDecodeError, ValueError) as exc:
        write_error_log(
            project_dir,
            hook_event="Stop",
            session_id="unknown",
            work_item="unknown",
            error_kind="stdin_parse_error",
            message=str(exc),
        )
        return 0

    hook_event = hook_input.get("hook_event_name", "Stop")
    session_id = hook_input.get("session_id", "unknown")
    transcript_path = hook_input.get("transcript_path", "")

    # 1. Resolve work item .
    try:
        work_item_id = resolve_work_item(project_dir)
    except ValueError as exc:
        write_error_log(
            project_dir,
            hook_event=hook_event,
            session_id=session_id,
            work_item="unresolved",
            error_kind="work_item_resolution_failed",
            message=str(exc),
        )
        return 0

    # 2. Parse transcript and accumulate counters.
    # On OSError (transcript not yet written, async lag) log and skip —
    # never overwrite a previous total with zeros.
    try:
        input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, n = (
            sum_token_usage(transcript_path)
        )
    except OSError as exc:
        write_error_log(
            project_dir,
            hook_event=hook_event,
            session_id=session_id,
            work_item=work_item_id,
            error_kind="transcript_read_error",
            message=f"transcript not readable (async lag or permission error): {exc}",
        )
        return 0

    # Guard: if no assistant messages were found the sum is 0/0/0/0.
    # Calling record-tokens with all-zero counters would overwrite the previously
    # persisted total with zeros. Skip silently — the next Stop
    # turn will recover the correct cumulative total from a readable transcript.
    # NOTE: a direct CLI call with explicit zero counters is still valid;
    # this guard applies only when the hook finds no session data at all.
    if n == 0:
        return 0

    # 3. Build idempotency key (D-f): "tokens:<session_id>:<N>".
    operation_id = f"tokens:{session_id}:{n}"

    # 4. Call the engine .
    sdd_cli_args = [
        "sdd-cli",
        "--dir", str(project_dir),
        "--json",
        "record-tokens", work_item_id,
        "--input-tokens", str(input_tokens),
        "--output-tokens", str(output_tokens),
        "--cache-read-tokens", str(cache_read_tokens),
        "--cache-write-tokens", str(cache_write_tokens),
        "--source", ADAPTER_ID,
        "--operation-id", operation_id,
    ]

    try:
        result = subprocess.run(
            sdd_cli_args,
            capture_output=True,
            text=True,
            timeout=30,
        )
    except FileNotFoundError:
        write_error_log(
            project_dir,
            hook_event=hook_event,
            session_id=session_id,
            work_item=work_item_id,
            error_kind="sdd_cli_not_found",
            message="sdd-cli executable not found in PATH",
        )
        return 0
    except subprocess.TimeoutExpired:
        write_error_log(
            project_dir,
            hook_event=hook_event,
            session_id=session_id,
            work_item=work_item_id,
            error_kind="sdd_cli_timeout",
            message="sdd-cli record-tokens timed out after 30 seconds",
        )
        return 0
    except Exception as exc:  # pragma: no cover
        write_error_log(
            project_dir,
            hook_event=hook_event,
            session_id=session_id,
            work_item=work_item_id,
            error_kind="sdd_cli_error",
            message=str(exc),
        )
        return 0

    # 5. Inspect engine response.
    if result.returncode != 0:
        # Parse the JSON error response for diagnostics.
        cli_output = result.stdout.strip() or result.stderr.strip()
        try:
            response = json.loads(result.stdout)
            error_code = response.get("error", {}).get("code", "unknown")
            error_message = response.get("error", {}).get("message", cli_output)
        except (json.JSONDecodeError, AttributeError):
            error_code = "cli_non_zero"
            error_message = cli_output

        write_error_log(
            project_dir,
            hook_event=hook_event,
            session_id=session_id,
            work_item=work_item_id,
            error_kind=error_code,
            message=error_message,
            extra=cli_output,
        )

    # Always return 0 — never block the session.
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
