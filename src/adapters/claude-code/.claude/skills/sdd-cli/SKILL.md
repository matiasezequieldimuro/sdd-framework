---
name: sdd-cli
description: Use to operate or inspect the SDD engine through its deterministic CLI.
---

# SDD engine CLI

Use this skill whenever a request needs SDD state, workflow guidance, or a
state transition. The CLI is the authority for state; it does not write the
intellectual content of artifacts or implement code.

## Invocation

- Run `sdd-cli <command> --help` before using unfamiliar flags.
- Use `--json` when structured output informs a decision; never infer state from chat.
- Use `--dir <project>` only when the SDD root is not the current directory.
- Always pass `--dir` as an **absolute path**. A relative path resolves against
  the invoking process's cwd, which is not guaranteed to be the project root
  (for example, inside a subagent or a freshly created worktree), and
  `sdd-cli` fails to locate `.sdd`/`.claude`. Resolve it first (e.g. `$(pwd)`,
  `$(git rev-parse --show-toplevel)`) instead of passing `..` or `./...`.
- Reuse `--operation-id` when retrying a state-changing command after an uncertain result.

```bash
sdd-cli --dir /path/to/project status feat-export --json
```

Global flags:

| Flag | Use |
| --- | --- |
| `--dir <path>` | Select the project containing `.sdd/`; defaults to the current directory. Must be an absolute path. |
| `--json` | Return the stable JSON envelope for agent decisions and integrations. |

## Inspect first

Before selecting a phase or transition, inspect the engine:

```bash
sdd-cli status <work-item-id> --json
sdd-cli next <work-item-id> --json
sdd-cli validate [work-item-id] --json
```

| Command | Use |
| --- | --- |
| `status <id>` | Read the work item, ordered phases, workflow, approvals, and active/archive location. |
| `next <id>` | Obtain the highest-priority valid next action, its procedure, artifact, and approval need. |
| `validate [id]` | Check the entire project without an ID, or one active/archived work item with an ID. |

- `status`, `next`, and `validate` do not mutate state.
- `next` does not begin a phase. It reports the action the engine currently permits.
- A failed `validate` exits non-zero. In JSON, inspect `error.code` and
  `error.details`; do not ignore the report.

## Initialize and install

```bash
sdd-cli init --dir /path/to/project
sdd-cli adapters list --json
sdd-cli adapters install claude-code --dir /path/to/project
```

| Command | Preconditions and result |
| --- | --- |
| `init` | The target must not already contain `.sdd/`. It installs the portable contract. |
| `adapters list` | Lists adapters included in the current binary. |
| `adapters install <id>` | Requires `.sdd/`; refuses any destination collision and never overwrites. |

## Start a work item

Start from the default workflow:

```bash
sdd-cli start feat-export-csv \
  --title "Add CSV export" \
  --summary "Users need a CSV download for reports." \
  --actor-kind agent \
  --actor-id sdd-orchestrator \
  --operation-id run:feat-export-csv:start
```

Start from a workflow-specific entry artifact only when the workflow declares
that entry point:

```bash
sdd-cli start feat-export-csv \
  --workflow feature-standard \
  --title "Add CSV export" \
  --from-artifact ./approved-plan.md \
  --phase plan
```

| Flag | Meaning |
| --- | --- |
| `--title` | Required work item title. |
| `--workflow` | Optional workflow ID; defaults to `.sdd/config.yaml`. |
| `--summary` | Initial user input summary. |
| `--from-artifact` and `--phase` | Must be supplied together. The phase must be a declared workflow entry point. |
| `--actor-kind`, `--actor-id` | Identify the actor; defaults are `human` and `user` for `start`. |

Do not use an entry artifact to skip arbitrary phases. The CLI imports the
artifact, records the input, and marks only valid predecessor phases as not
applicable.

## Execute a phase

Use the engine-controlled sequence:

```text
inspect -> begin -> load procedure -> write artifact -> deliver -> gate -> complete
```

Begin only the enabled phase:

```bash
sdd-cli begin feat-export-csv \
  --phase plan \
  --actor-kind agent \
  --actor-id sdd-planner \
  --operation-id run:feat-export-csv:plan:begin
```

After `begin`, load only the procedure reported by `next`, produce the
required artifact at the engine-prepared path, then submit it:

```bash
sdd-cli deliver feat-export-csv \
  --phase plan \
  --actor-kind agent \
  --actor-id sdd-planner \
  --operation-id run:feat-export-csv:plan:deliver
```

| Command | Valid purpose |
| --- | --- |
| `begin <id> --phase <phase>` | Start a `ready`, `rejected`, or `superseded` phase. |
| `deliver <id> --phase <phase>` | Submit evidence for an `in_progress` phase. |
| `deliver ... --request-approval` | Request approval for an optional gate only. It is invalid for a `none` gate. |
| `complete <id> --phase <phase>` | Explicitly complete an `approved` or `accepted` phase. |
| `complete <id>` | Complete the work item only when all required phases are satisfied. |

`deliver` resolves the phase according to its declared gate:

| Gate | Result after delivery |
| --- | --- |
| `required` | `awaiting_approval` |
| `optional` | `completed`, unless `--request-approval` is supplied |
| `none` | `completed` |

Never manually edit a manifest, event log, approval, lock, or phase status to
make a command succeed.

## Human gates

Only execute these after an explicit human decision is present in the current
conversation:

```bash
sdd-cli approve feat-export-csv \
  --phase plan \
  --by matias \
  --comment "Approved" \
  --operation-id run:feat-export-csv:plan:approve

sdd-cli reject feat-export-csv \
  --phase plan \
  --by matias \
  --comment "Add rollback details." \
  --operation-id run:feat-export-csv:plan:reject
```

- `approve` and `reject` require a phase in `awaiting_approval`.
- `--by` identifies a human reviewer. Do not use an agent identity.
- A rejection preserves history. Rework begins with `begin --phase <phase>`;
  it does not overwrite the rejection.
- Never approve, reject, or infer approval on the user's behalf.

## Archive and audit

Archive is a two-step closure:

1. Deliver the workflow's `archive` phase when it exists.
2. Validate, then physically archive the completed work item.

```bash
sdd-cli validate feat-export-csv --json
sdd-cli archive feat-export-csv \
  --actor-kind cli \
  --actor-id sdd \
  --operation-id run:feat-export-csv:archive
```

`archive` requires a completed work item, satisfied archive phase when
declared, valid archive evidence, and no validation failures. It moves the
snapshot to `.sdd/work-items/archive/`; do not modify it afterwards.

Record an audited observation without changing phase state:

```bash
sdd-cli record-event feat-export-csv \
  --type validation.completed \
  --message "Targeted suite passed." \
  --actor-kind agent \
  --actor-id sdd-verifier \
  --operation-id run:feat-export-csv:validation
```

`record-event` requires `--type`; `--message` is optional.

## Mutations and retries

- Mutating commands include `init`, `adapters install`, `start`, `begin`,
  `deliver`, `approve`, `reject`, `complete`, `archive`, and `record-event`.
- Assign a stable, descriptive `--operation-id` before a mutation when a retry
  might be needed.
- If the result is uncertain, retry the exact same command with the same
  operation ID. Do not invent a new ID or edit files to compensate.
- Treat `invalid_transition`, `validation_failed`, `not_found`, and
  `concurrent_modification` as signals to inspect state and resolve the cause.
- The engine owns transactional persistence, locks, revisions, and recovery.