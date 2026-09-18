#!/usr/bin/env python3
"""Block direct attempts to read protected files or list environment variables."""

import json  # Parse hook input and write the JSON decision.
import os  # Read the project root from the environment.
import re  # Match protected paths and shell commands.
import sys  # Read stdin and write stdout.
from pathlib import Path  # Handle filesystem paths safely.


FILE_NAME_PATTERNS = (
    re.compile(r"^\.env(?:\..+)?$"),  # Match .env and its variants.
    re.compile(r"^\.envrc$"),  # Match the direnv configuration file.
    re.compile(r"^\.(?:npmrc|pypirc|netrc|git-credentials)$"),  # Match credential config files.
    re.compile(r".*\.(?:pem|key|p12|pfx|kdbx)$", re.IGNORECASE),  # Match common key and certificate files.
)
PROTECTED_DIRECTORY_NAMES = {"secrets", "credentials", ".ssh", ".aws", ".gnupg"}  # Block sensitive directories.
ENVIRONMENT_FILE_PATTERN = re.compile(r"(?:^|/)(?:proc/[^/]+/environ)(?:/|$)")  # Block process environment files.
ENVIRONMENT_COMMAND_PATTERN = re.compile(
    # Match env and printenv at shell command boundaries.
    r"(?:^|&&|\|\||;|\||&|\n)\s*(?:command\s+)?(?:env|printenv)(?:\s|$)"
)
ENVIRONMENT_LISTING_PATTERN = re.compile(
    # Match set and export when used to list all variables.
    r"(?:^|&&|\|\||;|\||&|\n)\s*(?:command\s+)?(?:set|export)\s*"
    r"(?:-p\s*)?(?:&&|\|\||;|\||&|\n|$)"
)
ENVIRONMENT_SUBSHELL_PATTERN = re.compile(
    # Match env and printenv inside command substitution.
    r"(?:\$\(|`)\s*(?:command\s+)?(?:env|printenv)(?:\s|\)|`|$)"
)
SHELL_COMMAND_PATTERN = re.compile(
    # Match shell -c commands that list environment variables.
    r"\b(?:bash|sh|zsh)\s+(?:-[^\s]+\s+)*-c\s+.*\b(?:env|printenv|set|export)\b"
)
SENSITIVE_COMMAND_PATH_PATTERN = re.compile(
    # Match protected filenames or directories in shell commands.
    r"(?i)(?:\.env(?:\.[^\s/]+)?|\.envrc|\.npmrc|\.pypirc|\.netrc|"
    r"\.git-credentials|\.(?:pem|key|p12|pfx|kdbx)\b|"
    r"(?:^|/)(?:secrets|credentials|\.ssh|\.aws|\.gnupg)(?:/|$))"
)


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


def is_protected_path(path: Path) -> bool:
    # Block direct reads of a process environment file.
    if ENVIRONMENT_FILE_PATTERN.search(path.as_posix()):
        return True
    # Block any path nested below a sensitive directory.
    if any(part in PROTECTED_DIRECTORY_NAMES for part in path.parts):
        return True
    # Block protected filenames regardless of their parent directory.
    return any(pattern.fullmatch(path.name) for pattern in FILE_NAME_PATTERNS)


def resolve_path(raw_path: str, project_root: Path) -> Path:
    # Convert the tool-provided string into a filesystem path.
    path = Path(raw_path)
    if not path.is_absolute():
        # Resolve relative paths from the project root.
        path = project_root / path
    # Normalize .. segments without requiring the target to exist.
    return path.resolve(strict=False)


def extract_path(tool_name: str, tool_input: dict):
    # Read uses file_path, while search tools use path.
    if tool_name == "Read":
        return tool_input.get("file_path")
    if tool_name in {"Grep", "Glob"}:
        return tool_input.get("path")
    return None


def main() -> int:
    # Use Claude's project directory, falling back to the current directory.
    project_root = Path(os.environ.get("CLAUDE_PROJECT_DIR", os.getcwd())).resolve()

    try:
        # Read the hook request supplied by Claude Code.
        hook_input = json.load(sys.stdin)
    except json.JSONDecodeError:
        deny("Blocked: the secret protection hook could not parse the tool input.")
        return 0

    # Extract the tool name and its arguments from the request.
    tool_name = hook_input.get("tool_name", "")
    tool_input = hook_input.get("tool_input", {})
    if not isinstance(tool_input, dict):
        deny("Blocked: the secret protection hook received an invalid tool input.")
        return 0

    if tool_name == "Bash":
        # Inspect shell commands for environment enumeration or secret paths.
        command = tool_input.get("command", "")
        if isinstance(command, str) and (
            ENVIRONMENT_COMMAND_PATTERN.search(command)
            or ENVIRONMENT_LISTING_PATTERN.search(command)
            or ENVIRONMENT_SUBSHELL_PATTERN.search(command)
            or SHELL_COMMAND_PATTERN.search(command)
        ):
            deny("Blocked: listing or exporting environment variables is not allowed.")
        elif isinstance(command, str) and SENSITIVE_COMMAND_PATH_PATTERN.search(command):
            deny("Blocked: direct shell access to a protected secret file is not allowed.")
        return 0

    # Inspect file paths for Read, Grep, and Glob operations.
    raw_path = extract_path(tool_name, tool_input)
    if not isinstance(raw_path, str) or not raw_path:
        return 0

    try:
        # Normalize the requested path before checking its components.
        target = resolve_path(raw_path, project_root)
    except (OSError, ValueError):
        deny("Blocked: the secret protection hook could not safely resolve the target path.")
        return 0

    if is_protected_path(target):
        # Deny access when the normalized path matches a protected resource.
        deny("Blocked: reading protected secrets or credentials is not allowed.")

    return 0


if __name__ == "__main__":
    # Exit with the hook's result when run as a script.
    raise SystemExit(main())