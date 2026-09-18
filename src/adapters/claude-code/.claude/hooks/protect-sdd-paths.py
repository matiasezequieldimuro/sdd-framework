#!/usr/bin/env python3
"""Enforce the SDD write boundary for Claude Code file-editing tools."""

import json  # Parse hook input and write the JSON decision.
import os  # Read the project root from the environment.
import sys  # Read stdin and write stdout.
from pathlib import Path  # Handle filesystem paths safely.


def deny(reason: str) -> None:
    # Return the Claude Code protocol response that denies the tool call.
    json.dump(
        {
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",  # Identify the hook lifecycle event.
                "permissionDecision": "deny",  # Prevent the requested operation.
                "permissionDecisionReason": reason,  # Explain the denial to the developer.
            }
        },
        sys.stdout,
    )


def is_within(path: Path, directory: Path) -> bool:
    # Check whether path is directory itself or one of its descendants.
    try:
        path.relative_to(directory)
        return True
    except ValueError:
        return False


def is_allowed_sdd_write(path: Path, sdd_root: Path) -> bool:
    # These top-level SDD directories are writable by design.
    for directory in ("context", "research", "specs"):
        if is_within(path, sdd_root / directory):
            return True

    # Active work items may only be changed through their artifacts directory.
    active_root = sdd_root / "work-items" / "active"
    if not is_within(path, active_root):
        return False

    # Inspect the path below the active work-items directory.
    relative = path.relative_to(active_root)
    # Require a work-item name followed by the artifacts directory.
    return len(relative.parts) >= 3 and relative.parts[1] == "artifacts"


def main() -> int:
    # Use Claude's project directory, falling back to the current directory.
    project_root = Path(os.environ.get("CLAUDE_PROJECT_DIR", os.getcwd())).resolve()

    try:
        # Read the hook request supplied by Claude Code.
        hook_input = json.load(sys.stdin)
    except json.JSONDecodeError:
        deny("Blocked: the SDD protection hook could not parse the tool input.")
        return 0

    # Extract the target path used by file and notebook editing tools.
    tool_input = hook_input.get("tool_input", {})
    raw_path = tool_input.get("file_path") or tool_input.get("notebook_path")
    if not isinstance(raw_path, str) or not raw_path:
        deny("Blocked: the SDD protection hook could not identify the target path.")
        return 0

    try:
        # Normalize relative paths against the project root.
        target = Path(raw_path)
        if not target.is_absolute():
            target = project_root / target
        target = target.resolve(strict=False)
    except (OSError, ValueError):
        deny("Blocked: the SDD protection hook could not safely resolve the target path.")
        return 0

    # Keep adapter configuration read-only to agents.
    claude_root = project_root / ".claude"
    if is_within(target, claude_root):
        deny("Blocked: .claude/ is adapter configuration and is read-only to agents.")
        return 0

    # Enforce the narrower write policy inside the SDD directory.
    sdd_root = project_root / ".sdd"
    if is_within(target, sdd_root) and not is_allowed_sdd_write(target, sdd_root):
        deny(
            "Blocked: write within .sdd/ is limited to context/, research/, "
            "specs/, and active work-item artifacts/."
        )
        return 0

    return 0


if __name__ == "__main__":
    # Exit with the hook's result when run as a script.
    raise SystemExit(main())