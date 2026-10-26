---
schema_version: "1.0"
kind: artifact
id: "implementation-report"
work_item: "feat-rework-cancel"
phase: "implementation"
status: delivered
created_at: "2026-10-07T12:48:00Z"
created_by: { kind: "agent", id: "sdd-developer" }
sources: 
  - artifacts/plan.md
---

# Implementation Report — feat-rework-cancel

## Summary of Changes

### New Files

| File | Purpose |
|---|---|
| `src/cli/internal/infra/fs_cancel_repository.go` | Transactional cancel: stage → marker → rename for `active/` → `cancelled/`. Full recovery coverage (three branches). |
| `src/cli/internal/usecases/rework_uc.go` | `ReworkUseCase`: validate → load → idempotency → `ReworkPhase` → `reworkEvents` → `commitWorkItem`. |
| `src/cli/internal/usecases/cancel_uc.go` | `CancelUseCase`: validate → idempotency via `FindCancelledWorkItem` → `Cancel()` → `CancelWorkItem`. |
| `src/cli/cmd/rework.go` | Cobra command wiring for `sdd-cli rework`. |
| `src/cli/cmd/cancel.go` | Cobra command wiring for `sdd-cli cancel`. |
| `src/cli/internal/domain/work_item_rework_test.go` | 10 domain unit tests for `ReworkPhase`: transition matrix, work item status matrix, transitive propagation, re-blocking, idempotent skipping, not_applicable, approval preservation, topological order. |
| `src/cli/internal/domain/work_item_cancel_test.go` | 4 domain unit tests for `Cancel`: success, status matrix, phase preservation, DD-5 regression guard. |

### Modified Files

| File | Change |
|---|---|
| `src/cli/internal/domain/errors.go` | Added `ErrWorkItemCannotCancel` and `ErrWorkItemCancelled` sentinels. |
| `src/cli/internal/domain/work_item.go` | Added `ReworkMutation` struct, `ReworkPhase` method, `Cancel` method. |
| `src/cli/internal/domain/workflow_validation.go` | Added `Descendants` method (mirror of `Ancestors`, forward DAG traversal). |
| `src/cli/internal/domain/work_item_validation.go` | Added `superseded` exemption (DD-5) to `ViolationsAgainst`. |
| `src/cli/internal/ports/repository.go` | Added `WorkItemLocationCancelled`, `WorkItemCancelCommit`, `CancelledWorkItemChecker`, `WorkItemCanceller` interfaces. |
| `src/cli/internal/usecases/transition_helpers.go` | Added `reworkEvents` function generating ordered audit events for rework. |
| `src/cli/internal/usecases/start_uc.go` | Added `cancelledChecker` field; differentiated error message for cancelled IDs (DD-6, CA-CA-05). |
| `src/cli/internal/usecases/next_uc.go` | Added `cancelledChecker` field; returns `ErrWorkItemCancelled` diagnostic when item is cancelled. |
| `src/cli/internal/usecases/validate_uc.go` | Added `cancelledChecker` field; returns `ErrWorkItemCancelled` diagnostic before `InspectWorkItem`. |
| `src/cli/internal/infra/fs_repository.go` | `recoverIfNeeded`: calls `recoverCancelIfNeeded`; `WorkItemExists`: checks all three locations; `GetWorkItem`: unchanged (active only). |
| `src/cli/internal/infra/fs_archive_repository.go` | `FindWorkItem`: returns `ErrWorkItemCancelled` when ID exists only in `cancelled/` (DD-7, INV-05). |
| `src/cli/cmd/composition.go` | Wired `Rework` and `Cancel` use cases; passed `workItems` as `CancelledWorkItemChecker` to `Start`, `Next`, `Validate`. |
| `src/cli/cmd/root.go` | Registered `rework` and `cancel` commands; added `ErrWorkItemCancelled → not_found` and `ErrWorkItemCannotCancel → invalid_transition` error code mappings. |
| `src/cli/internal/usecases/workflow_lifecycle_test.go` | Added 6 integration tests: `TestReworkUseCaseSupersededDescendants`, `TestReworkUseCaseReactivatesCompletedItem`, `TestReworkUseCaseIsIdempotent`, `TestCancelUseCaseMovesToCancelled`, `TestCancelUseCaseIsIdempotent`, `TestCancelUseCaseOpacity`. |
| `src/cli/cmd/cli_e2e_test.go` | Added `TestCLIReworkAndCancelCommands` covering rework success/idempotency/errors and cancel success/idempotency/ID-reservation/opacity/errors. |

---

## Mapping to Plan

| Plan Task | Addressed By |
|---|---|
| 1A.1 — `Descendants` in `workflow_validation.go` | `workflow_validation.go`: BFS over forward edges |
| 1A.2 — `ReworkMutation` struct | `work_item.go`: `ReworkMutation{Reactivated, Target, Superseded}` |
| 1A.3 — `ReworkPhase` method | `work_item.go`: propagation rules (ready→blocked, worked→superseded, skip no-change) |
| 1A.4 — `superseded` exemption (DD-5) | `work_item_validation.go`: `continue` before dependency check |
| 1A.5 — New sentinels | `errors.go`: `ErrWorkItemCannotCancel`, `ErrWorkItemCancelled` |
| 1A.6 — Domain tests for `ReworkPhase` | `work_item_rework_test.go` (8 tests) |
| 1B.1 — `Cancel` method | `work_item.go`: `Cancel()` validates `WorkItemActive` |
| 1B.2 — Domain tests for `Cancel` | `work_item_cancel_test.go` (4 tests, including DD-5 regression) |
| 2.1 — `fs_cancel_repository.go` | New file: `cancelTransaction`, `CancelWorkItem`, `recoverCancelIfNeeded`, rollback branches |
| 2.2 — `WorkItemExists` / `recoverIfNeeded` | `fs_repository.go`: three-location check, cancel recovery call |
| 2.3 — `FindWorkItem` opacity | `fs_archive_repository.go`: `ErrWorkItemCancelled` for cancelled-only IDs |
| 2.4 — Ports | `repository.go`: `CancelledWorkItemChecker`, `WorkItemCanceller`, `WorkItemCancelCommit` |
| 3A.1 — `reworkEvents` | `transition_helpers.go`: ordered events (reactivated?, rework_requested, rework_superseded×N) |
| 3A.2 — `rework_uc.go` | New use case with idempotency guard and `commitWorkItem` |
| 3A.3 — `cmd/rework.go` | New Cobra command: `--phase`, `--reason`, `--actor-kind`, `--actor-id`, `--operation-id` |
| 3B.1 — `cancel_uc.go` | New use case with idempotency via `FindCancelledWorkItem` |
| 3B.2 — `cmd/cancel.go` | New Cobra command: `--reason`, `--actor-kind`, `--actor-id`, `--operation-id` |
| 3B.3 — `start_uc.go` differentiated message | `start_uc.go`: cancelled check before generic `ErrWorkItemAlreadyExists` |
| 3B.4 — `next_uc.go` diagnostic | `next_uc.go`: `cancelledChecker` after `ErrWorkItemNotFound` |
| 3B.5 — `validate_uc.go` diagnostic | `validate_uc.go`: `cancelledChecker` before `InspectWorkItem` |
| 4.1 — Composition root | `composition.go`: wired all use cases and checkers |
| 4.2 — `root.go` error mapping | `root.go`: two new entries in `errorCode` |
| 5 (C) — Lifecycle integration tests | `workflow_lifecycle_test.go`: 6 new test functions |
| 5 (D) — E2E binary tests | `cli_e2e_test.go`: `TestCLIReworkAndCancelCommands` |

---

## Design and Architecture Notes

### DD-1: Opacity of cancelled items
`FindWorkItem` (used by `StatusUseCase`) returns `ErrWorkItemCancelled` when the ID exists only in `cancelled/`. This maps to `not_found` in `errorCode`, surfacing "was cancelled" without exposing manifest content. `GetWorkItem` (used by phase mutation use cases) retains its active-only lookup; cancelled items correctly produce `ErrWorkItemNotFound` there, which is caught by the cancelled checkers in `next_uc.go` and `validate_uc.go`.

### DD-2: `ReworkMutation.Superseded` holds both superseded and re-blocked transitions
The field name is "representative, not exclusive." A `ready → blocked` transition is included alongside `* → superseded` transitions. The count returned to callers (`ReworkResult.Superseded`) reflects the total, enabling accurate text output.

### DD-3: Cancel shares the transactional pattern with archive
`fs_cancel_repository.go` replicates the three-phase commit (stage → marker → rename) and all three recovery branches from `fs_archive_repository.go`. Differences: destination is `cancelled/<id>` (no date prefix), marker is `.cancel.json`, and there is no artifact gate. This duplication is explicitly tracked as future refactoring debt (generic transactional mover).

### DD-4: Variadic checker parameters preserve backward compatibility
`NewStartWorkItemUseCase`, `NewNextUseCase`, and `NewValidateUseCase` accept `cancelledCheckers ...ports.CancelledWorkItemChecker` as a variadic last parameter. All existing call sites (unit tests, integration tests) that do not pass a checker continue to compile and behave identically.

### DD-5: Superseded exemption in validation
`ViolationsAgainst` now skips dependency checks for phases in `superseded` status. Without this, post-rework manifests would fail validation because superseded descendants have unsatisfied dependencies (the reworked phase is back to `ready`, which does not satisfy `satisfiesDependency()`).

### DD-6: Differentiated `already_exists` message for cancelled IDs
`start_uc.go` checks `WorkItemCancelledExists` when a work item already exists; if cancelled, it wraps `ErrWorkItemAlreadyExists` with a message containing "reserved by a prior cancellation". Both cases return `already_exists` error code.

### DD-7: `ErrWorkItemCancelled` sentinel
The sentinel allows callers to distinguish "not found" from "found but cancelled." CLI error mapping routes it to `not_found` for opacity at the API surface, while internally the distinction drives the differentiated diagnostics in `next_uc.go` and `validate_uc.go`.

---

## Verification Performed

### Build

```
/home/linuxbrew/.linuxbrew/bin/go build ./...
# (no output — clean build)
```

### Tests

```
/home/linuxbrew/.linuxbrew/bin/go clean -testcache
/home/linuxbrew/.linuxbrew/bin/go test ./...
```

Results:
```
ok  sdd-cli/cmd               5.348s
ok  sdd-cli/internal/domain   0.011s
ok  sdd-cli/internal/infra    3.483s
ok  sdd-cli/internal/usecases 16.316s
```

All packages pass. No build errors, no lint errors, no type errors.

---

## Deviations from Plan

None. The implementation follows the approved plan strictly. All design decisions (DD-1 through DD-7) were applied as specified. The variadic checker parameter strategy (RK-4 mitigation) was applied to all three affected constructors.

## Rework 2026-10-14 — validación de work items reactivados

Motivo (cuatro ciclos de rework): durante la fase `archive` se detectó (y se reprodujo con el binario) que el inspector de
eventos no reconocía `work_item.reactivated`. Un work item `completed` al que se le aplicaba
`rework` fallaba `validate` (`event.lifecycle_matches_manifest`) y, al volver a completarse,
`event.lifecycle_continuity_valid`, lo que impedía archivarlo. Al hacer rework sobre este mismo work item se detectó además que
`next` no guiaba el reinicio de fases `superseded` y que `begin` permitía reiniciarlas con
dependencias insatisfechas.

El tercer ciclo agrega los tests de crash-recovery de `cancel` señalados como brecha G1 por la
re-verificación.

Desviaciones aceptadas por el revisor humano (matias, 2026-10-14), sin cambio de código:

- D1: `rework` sobre un work item cancelado devuelve `not_found` genérico (la spec pedía un error
  que indique la cancelación).
- D2: la ausencia de `--reason` devuelve `invalid_arguments` (error estándar de Cobra), no
  `invalid_input`.

Fuera de alcance (comportamiento previo, sin cambios): `next` tampoco reporta fases `rejected`.

### Cambios

| Archivo | Cambio |
|---|---|
| `src/cli/internal/infra/fs_validation_inspector.go` | Ciclo de vida de eventos: `work_item.reactivated` (exige `completed` → `active`) y `work_item.cancelled` (exige `active` → `cancelled`). El segundo es defensivo: hoy `validate` no inspecciona `cancelled/`. |
| `src/cli/cmd/cli_e2e_test.go` | `TestCLIReworkReactivatedWorkItemCanBeValidatedAndArchived`: completar → `rework` → `validate` → re-completar → `validate` → `archive`. Falla sin el fix y pasa con él. |
| `src/cli/internal/domain/work_item.go` | `BeginPhase`: una fase `superseded` solo se reinicia con sus dependencias satisfechas (antes se podía saltear el DAG tras un rework). `NextPhase`: tras `awaiting_approval`, `in_progress` y `ready`, reporta la primera fase `superseded` con dependencias satisfechas (antes `next` decía "No active phases pending"). |
| `src/cli/internal/domain/work_item_rework_test.go` | `TestReworkSupersededPhaseRestartRequiresDependencies` y `TestNextPhaseReportsSupersededPhaseWithSatisfiedDependencies`. |
| `src/cli/internal/infra/cancel_repository_test.go` (nuevo) | Cobertura de CA-CA-08 (ciclo 3): publicación y limpieza; rollback por fallo tras el backup y tras la publicación (hooks `cancel_after_active_backup` / `cancel_after_destination_publish`); recovery de crash antes del backup, antes de publicar y después de publicar; conflicto cuando el expediente publicado no contiene la operación del marker. Prueba de mutación: desactivar `recoverCancelTransactionLocked` hace fallar los 4 tests de recovery. |
| `src/cli/internal/infra/project_initializer.go` | Ciclo 4 (pedido del revisor): `init` escribe `.sdd/work-items/.gitignore` con `.locks/` y `.transactions/`, el estado de runtime del motor. |
| `src/cli/internal/infra/project_initializer_test.go` (nuevo) | `TestInitializeIgnoresEngineRuntimeDirectories`. |
| `.gitignore` (raíz) | Ignora `.sdd/work-items/.locks/` y `.sdd/work-items/.transactions/` en este repositorio. |
| `src/cli/cmd/composition.go`, `src/cli/internal/usecases/workflow_lifecycle_test.go` | Solo `gofmt`. |

Revisión humana previa (mismo día): comandos `/sdd:rework` y `/sdd:cancel` del adapter, skill
`sdd-cli` actualizado, embeds regenerados y test de instalación ampliado (ver
`artifacts/human-code-review.md` del ciclo anterior).

### Verificación

- `go test ./...`, `go vet ./...` y `gofmt -l .` en `src/cli`: sin errores.
- Ciclo 4: proyecto temporal con `git init` + `sdd-cli init` + `start`: `validate` pasa
  (301/301) y `git status` no lista `.locks/` ni `.transactions/`.
