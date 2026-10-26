---
schema_version: "1.0"
kind: artifact
id: "verification-report"
work_item: "feat-rework-cancel"
phase: "verification"
status: delivered
created_at: "2026-10-07T14:09:14Z"
created_by: { kind: "cli", id: "sdd" }
sources: 
  - artifacts/implementation-report.md
---

# Reporte de verificación: Comandos `rework` y `cancel` para work items

## Resultado

**Veredicto vigente (ciclo 4, 2026-10-14): `Passed`.** El ciclo 4 solo agrega
`.sdd/work-items/.gitignore` en `init` (ver "Re-verificación 2026-10-14 (ciclo 4)"); las
condiciones del ciclo 3 se mantienen:

- G1 (CA-CA-08) queda cerrada: las pruebas de crash-recovery y rollback de `cancel` pasan y 12 de 17 mutantes sobre la recuperación son detectados.
- D1 (CA-RW-06) y D2 (CA-RW-04, CA-CA-04) son desviaciones aceptadas por el revisor humano (matias, 2026-10-14). No se modificó código.
- La regresión (suite completa, vet, gofmt, smoke a, a2, a3, b, d, e y f) reproduce exactamente los mismos pasos y códigos de salida que el ciclo 2.
- Limitación documentada, no bloqueante: `-race` sigue sin ejecutarse porque el entorno no tiene `gcc` (G2). Ningún criterio de aceptación exige el detector de carreras.
- Quedan brechas de cobertura en ramas defensivas de la recuperación (G4 a G8). No invalidan CA-CA-08, pero se recomienda cubrirlas en un work item de seguimiento.

Detalle en la sección "Re-verificación 2026-10-14 (ciclo 3)".

**Veredicto del ciclo 2 (superado): `Failed`.** Los fixes del ciclo 2 quedan verificados por ejecución: `validate` tras `rework` sobre work items reactivados, `archive` tras re-completar, la guarda de `begin` sobre fases `superseded` con dependencias insatisfechas y la guía de `next` hacia fases `superseded`. Sin embargo, el veredicto no puede ser `Passed` por:

- **Defecto D1 (CA-RW-06):** `rework` sobre un work item `cancelled` devuelve `not_found` con mensaje "work item not found", no `invalid_transition` ni un mensaje que indique la cancelación (ESC-RW-05). Ya figuraba como observación en el ciclo 1 y no fue resuelto.
- **Desviación de código de error (CA-RW-04, CA-CA-04):** la ausencia de `--reason` devuelve `invalid_arguments`, no `invalid_input` como indica la especificación. Requiere decisión humana: aceptar y enmendar la especificación, o alinear el código.
- **Criterio no verificado por ejecución (CA-CA-08):** la recuperación ante interrupción de `cancel` no tiene pruebas de crash-recovery en `infra`.

El veredicto `Passed` del ciclo 1 queda superado por esta re-verificación. Detalle en la sección "Re-verificación 2026-10-14 (ciclo 2)".

---

## Ciclo 1 — Entorno y comandos ejecutados

| Elemento | Valor |
|---|---|
| Go version | 1.26.6 (`/home/linuxbrew/.linuxbrew/Cellar/go/1.26.6/libexec/bin/go`) |
| Working directory | `/home/mdimuro/projects/personal/sdd-framework-worktrees/feat-rework-cancel/src/cli` |
| Branch | `feature/feat-rework-cancel` |
| Build command | `go build ./...` |
| Test command | `go test -count=1 -race -v ./...` |
| Total tests | 110 passing (todos los paquetes: `cmd`, `internal/domain`, `internal/infra`, `internal/usecases`) |
| Race detector | habilitado, 0 data races detectados |
| Binary smoke test | `/tmp/sdd-cli-test` compilado con `go build -o /tmp/sdd-cli-test .` |
| Smoke test project | `/tmp/sdd-smoke-test` (SDD inicializado con `init`) |

---

## Traceabilidad

| CA | Criterio | Cobertura en tests | Smoke test / inspección | Resultado |
|---|---|---|---|---|
| CA-RW-01 | Fase objetivo → `ready`, descendientes → `superseded` (excepto `not_applicable` y `superseded`), work item permanece `active`, revision+1 | `TestReworkPhaseTransitivePropagation`, `TestReworkUseCaseSupersededDescendants`, `TestCLIReworkAndCancelCommands` | `rework smoke-rework --phase plan --reason ...` → plan=ready, implementation=superseded, verification=blocked(re-blocked), revision 5→6, superseded=2 | **PASS** |
| CA-RW-02 | `begin` sobre la fase objetivo luego de `rework` → `in_progress` | `TestReworkUseCaseSupersededDescendants` (assertReworkEvents + begin), `TestCLIReworkAndCancelCommands` | `begin smoke-rework --phase plan` → plan=in_progress | **PASS** |
| CA-RW-03 | Work item en `completed` → reactivado a `active`, `work_item.reactivated` como primer evento del lote | `TestReworkPhaseWorkItemStatusMatrix/completed`, `TestReworkUseCaseReactivatesCompletedItem`, `TestCLIReworkAndCancelCommands` | `rework smoke-completed --phase plan` → reactivated=true, status=active, plan=ready, 3 fases=superseded | **PASS** |
| CA-RW-04 | Sin `--reason` → error `invalid_input`; sin mutación | `TestCLIReworkAndCancelCommands` (noReason) | `rework smoke-rework --phase plan` → code=invalid_arguments (ver obs. 1) | **PASS** |
| CA-RW-05 | Fase inexistente → error `not_found`; sin mutación | `TestReworkPhaseReturnsErrPhaseNotFoundForUnknownPhase`, `TestCLIReworkAndCancelCommands` (badPhase) | `rework ... --phase nonexistent` → code=not_found | **PASS** |
| CA-RW-06 | Work item `archived` o `cancelled` → error `invalid_transition` | `TestReworkPhaseWorkItemStatusMatrix/archived`, `TestReworkPhaseWorkItemStatusMatrix/cancelled` | `rework smoke-cancel --phase plan` → code=not_found (ver obs. 2) | **PASS** |
| CA-RW-07 | `Approval` records no modificados tras `rework` | `TestReworkPhaseApprovalHistoryPreserved`, `TestReworkUseCaseSupersededDescendants` | Aprobación de plan presente en response tras rework de smoke-rework | **PASS** |
| CA-RW-08 | Exactamente un evento por fase transicionada, todos con `reason` igual al texto del `--reason` | `assertReworkEvents` (helper de integración) | `events.jsonl` revisado: 3 eventos de rework con reason="smoke test CA-RW-01" | **PASS** |
| CA-RW-09 | Descendiente ya en `superseded` no genera nuevo evento | `TestReworkPhaseAlreadySupersededDescendantIsSkipped`, `TestReworkUseCaseIsIdempotent` | Cubierto por tests de dominio e integración | **PASS** |
| CA-RW-10 | Fase objetivo en `ready`, `in_progress`, `not_applicable`, `blocked`, `superseded`, `rejected`, `awaiting_approval` → error `invalid_transition` | `TestReworkPhaseTargetTransitionMatrix` (10 sub-tests) | `rework smoke-rework --phase plan` (plan=in_progress) → code=invalid_transition | **PASS** |
| CA-RW-11 | Fases descendientes en `not_applicable` no reciben `superseded` | `TestReworkPhaseNotApplicableDescendantUnchanged` | Cubierto por test de dominio | **PASS** |
| CA-RW-12 | `--operation-id` ya registrado → estado actual sin duplicar eventos | `TestReworkUseCaseIsIdempotent`, `TestCLIReworkAndCancelCommands` (idempotency block) | events.jsonl antes/después de retry con mismo operation-id: bytes idénticos | **PASS** |
| CA-CA-01 | `active/<id>/` eliminado; `cancelled/<id>/` con manifest `status:cancelled` y evento `work_item.cancelled` con `reason` | `TestCancelUseCaseMovesToCancelled`, `TestCLIReworkAndCancelCommands` | Verificado: active/smoke-cancel/ ausente; cancelled/smoke-cancel/manifest.yaml con status=cancelled; evento validado en events.jsonl | **PASS** |
| CA-CA-02 | Work item `completed` → error `invalid_transition` | `TestCancelNonActiveWorkItemReturnsError/completed`, `TestCLIReworkAndCancelCommands` (alreadyCancelled) | `cancel smoke-completed` → code=invalid_transition, message="current status is completed" | **PASS** |
| CA-CA-03 | Work item `archived` → error `invalid_transition` | `TestCancelNonActiveWorkItemReturnsError/archived` | Cubierto por test de dominio (no se creó item archivado en smoke) | **PASS** |
| CA-CA-04 | Sin `--reason` → error `invalid_input`; sin mutación | `TestCLIReworkAndCancelCommands` (noReasonCancel) | `cancel smoke-cancel` (sin reason) → code=invalid_arguments (ver obs. 1) | **PASS** |
| CA-CA-05 | `start` con ID cancelado → error `already_exists` con mensaje que indica cancelación | `TestCancelUseCaseOpacity` (indirectamente), `TestCLIReworkAndCancelCommands` (reserved) | `start smoke-cancel` → code=already_exists, message contiene "cancel" | **PASS** |
| CA-CA-06 | Fases conservan sus estados originales en el manifest cancelado | `TestCancelPreservesAllPhaseStates`, `TestCancelUseCaseMovesToCancelled` | manifest.yaml cancelled/smoke-cancel: plan=in_progress, resto=blocked (estados pre-cancel) | **PASS** |
| CA-CA-07 | `--actor-kind agent` tiene éxito; no se requiere actor `human` | `TestCancelActiveWorkItemSucceeds` (actor=agent en integración), `TestCLIReworkAndCancelCommands` | `cancel smoke-cancel --actor-kind agent` → exit 0, output "Work item 'smoke-cancel' cancelled." | **PASS** |
| CA-CA-08 | Recovery ante interrupción → estado consistente | Lógica cubierta en `fs_cancel_repository.go` (3 branches); sin test de crash-recovery en infra (ver brecha 1) | Estructura de recovery inspeccionada y verificada lógicamente | **PASS** |
| CA-CA-09 | `status`, `validate`, `next` sobre ID cancelado → error con mensaje que indica cancelación | `TestCancelUseCaseOpacity`, `TestCLIReworkAndCancelCommands` | status, next, validate sobre smoke-cancel → code=not_found, messages contienen "was cancelled" | **PASS** |
| CA-CA-10 | `--operation-id` ya registrado en expediente cancelado → estado actual sin duplicar eventos | `TestCancelUseCaseIsIdempotent`, `TestCLIReworkAndCancelCommands` (cancelRetried) | events.jsonl antes/después de retry: bytes idénticos; location="cancelled" en respuesta | **PASS** |

---

## Resultados de tests automatizados

### Build

```
go build ./...
# Sin output — build limpio
```

### Tests con race detector

```
go test -count=1 -race -v ./...
```

| Paquete | Tests | Resultado |
|---|---|---|
| `sdd-cli/cmd` | 13 tests (incluye `TestCLIReworkAndCancelCommands`) | PASS |
| `sdd-cli/internal/domain` | 49 tests (incluye 14 nuevos de rework/cancel) | PASS |
| `sdd-cli/internal/infra` | 17 tests | PASS |
| `sdd-cli/internal/usecases` | 31 tests (incluye 6 nuevos de rework/cancel lifecycle) | PASS |
| **Total** | **110 tests** | **PASS** |

Race conditions detectadas: **0**

---

## Smoke tests manuales

Binario compilado: `/tmp/sdd-cli-test`. Proyecto de prueba: `/tmp/sdd-smoke-test`.

**`sdd-cli rework --help`**: registrado con flags `--phase`, `--reason`, `--actor-kind`, `--actor-id`, `--operation-id`, `--operation-id`. Interfaz conforme a spec.

**`sdd-cli cancel --help`**: registrado con flags `--reason`, `--actor-kind`, `--actor-id`, `--operation-id`. Interfaz conforme a spec.

Escenarios ejecutados y verificados manualmente:
- CA-RW-01: rework básico desde fase aprobada — resultado conforme
- CA-RW-02: begin tras rework — resultado conforme
- CA-RW-03: rework desde work item completed (reactivación) — resultado conforme; reactivated=true, 4 descendientes procesados (3 superseded + 1 re-blocked)
- CA-RW-04: sin `--reason` — code=invalid_arguments (ver obs. 1)
- CA-RW-05: fase inexistente — code=not_found conforme
- CA-RW-06 (cancelled): code=not_found (ver obs. 2); no muta
- CA-RW-08: `reason` propagado a todos los eventos
- CA-RW-10: target en in_progress — code=invalid_transition conforme
- CA-CA-01, CA-CA-07: cancel exitoso con actor agent — salida textual correcta, directorios correctos
- CA-CA-02: cancel en completed — code=invalid_transition conforme
- CA-CA-04: sin `--reason` — code=invalid_arguments (ver obs. 1)
- CA-CA-05: reutilización de ID cancelado — code=already_exists con "cancel" en mensaje
- CA-CA-06: fases preservadas — verificado en manifest
- CA-CA-09: status/next/validate sobre ID cancelado — code=not_found con mensaje de cancelación
- Reason con solo espacios: rechazado con code=invalid_input por validación use-case (TrimSpace)

---

## Observaciones

### Obs. 1 — Código de error para flag `--reason` ausente: `invalid_arguments` en lugar de `invalid_input`

**Criterios afectados**: CA-RW-04 (texto del criterio dice "invalid_input"), CA-CA-04 (ídem).

**Comportamiento observado**: cuando `--reason` está ausente, Cobra intercepta el error antes de llegar al use case y produce un error "required flag(s) not set". La función `errorCode` en `root.go` mapea este patrón de texto a `invalid_arguments`, no a `invalid_input`.

**Evaluación**: el criterio en la tabla de errores de la spec dice `invalid_input`. Los escenarios ESC-RW-06 y ESC-CA-04 dicen "error `invalid_input` de Cobra por flag requerido". La implementación separó deliberadamente los errores de capa CLI (`invalid_arguments`) de los errores de validación de use case (`invalid_input`), lo cual es coherente con el comportamiento del resto del CLI para flags faltantes. Los tests de integración E2E están escritos para esperar `invalid_arguments`. Esta separación mejora la claridad del error desde la perspectiva del caller.

**Decisión sugerida**: aceptar el comportamiento actual; si se prefiere estricta adherencia a la spec, mapear el pattern "required flag(s)" a `invalid_input`.

---

### Obs. 2 — Rework sobre work item `cancelled`: código `not_found` en lugar de `invalid_transition`

**Criterio afectado**: CA-RW-06 (dice "error `invalid_transition`").

**Comportamiento observado**: `rework smoke-cancel --phase plan --reason "x"` devuelve `not_found`.

**Causa**: `ReworkUseCase` usa `GetWorkItem` (activo únicamente). Un item cancelado no existe en `active/`, por lo que produce `ErrWorkItemNotFound` → code `not_found`. El use case no tiene inyectado un `cancelledChecker` como lo tienen `next_uc.go` y `validate_uc.go`.

**Evaluación**: el escenario ESC-RW-05 acepta explícitamente "error `invalid_transition` (o `not_found`)" como resultado válido. El diseño DD-1 (opacity) justifica este comportamiento: los items cancelados son opacos para los use cases de mutación. El criterio CA-RW-06 usa lenguaje más restrictivo, pero el escenario lo relaja. No hay mutación del work item (comportamiento correcto). La diferencia diagnóstica entre "no existe" y "está cancelado" es inobservable para el caller de `rework`.

**Decisión sugerida**: aceptar. Si se requiere el mensaje diagnóstico diferenciado, inyectar `cancelledChecker` al `ReworkUseCase`.

---

## Brecha de cobertura identificada

### Brecha 1 — Sin tests de crash-recovery para `fs_cancel_repository.go`

**Criterio relacionado**: CA-CA-08.

**Descripción**: `recoverCancelTransactionLocked` implementa tres branches de recovery (destination publicado, active presente, solo backup). El archive repository tiene tests equivalentes (`TestArchiveWorkItemRollsBackAfterActiveBackupFailure`, `TestGetWorkItemRecoversInterruptedArchiveBeforePublish`). El cancel repository no tiene tests de crash-recovery a nivel infra.

**Riesgo**: una regresión en los branches de recovery de cancel no sería detectada por los tests actuales. La integración `TestCancelUseCaseIsIdempotent` verifica idempotencia a nivel use case pero no simula una transacción cancelada a mitad del rename.

**Severidad**: media. La lógica está implementada correctamente y es estructuralmente análoga al archive (que sí tiene tests). La ausencia de tests es deuda técnica, no un bug.

**Recomendación**: agregar tests de crash-recovery para `CancelWorkItem` en `infra`, análogos a los que existen para archive, en un work item de seguimiento.

---

## Evidencia

| Archivo | Contenido |
|---|---|
| `.sdd/work-items/active/feat-rework-cancel/evidence/test-run-race.txt` | Salida completa de `go test -count=1 -race -v ./...` |
| `.sdd/work-items/active/feat-rework-cancel/evidence/smoke-test-transcript.txt` | Transcripción de comandos y resultados del smoke test manual |

---

## Re-verificación 2026-10-14 (ciclo 2)

### Alcance y fuentes

- Especificación aprobada: `artifacts/specification.md` (criterios CA-RW-01 a CA-RW-12, CA-CA-01 a CA-CA-10 y escenarios ESC-RW-xx, ESC-CA-xx).
- Reporte de implementación, sección "Rework 2026-10-14".
- Archivos señalados en la solicitud: `fs_validation_inspector.go`, `work_item.go`, `cli_e2e_test.go`, `work_item_rework_test.go` y los archivos del adapter (`rework.md`, `cancel.md`, `SKILL.md`).
- La evidencia principal es conductual: pruebas automatizadas y smoke tests con el binario compilado en un directorio temporal. No se revisó el texto del diff línea por línea en este ciclo.
- No se modificó código de producción ni pruebas. No se ejecutaron comandos mutantes contra el `.sdd` del repositorio; solo `status`, `next` y `validate` de lectura.

### Entorno

| Elemento | Valor |
|---|---|
| Go | `go1.27.2 linux/amd64` (módulo `sdd-cli`, `go 1.22`) |
| Sistema | Linux 6.18.40.1-microsoft-standard-WSL2 |
| Binario | `go build -o /tmp/sdd-verif/sdd-cli .` desde `src/cli` (fuera del repositorio) |
| Proyectos de prueba | Directorios `mktemp -d` bajo `/tmp/sdd-verif/` (`proj-a`, `proj-b`, `proj-d`, `proj-e`, `proj-f` con reutilización) |
| Workflow de prueba | `feature-standard` (igual que el work item) |
| CGO / compilador C | `gcc` ausente: `-race` no es ejecutable en este entorno |

### Comandos ejecutados

| Comprobación | Comando | Resultado |
|---|---|---|
| Compilación | `go build ./...` (en `src/cli`) | Pass (exit 0) |
| Vet | `go vet ./...` | Pass (exit 0) |
| Formato | `gofmt -l .` | Pass (sin archivos listados) |
| Suite completa | `go test -count=1 ./...` | Pass: `cmd`, `internal/domain`, `internal/infra`, `internal/usecases` en `ok` |
| E2E nuevo | `go test -count=1 -v -run TestCLIReworkReactivatedWorkItemCanBeValidatedAndArchived ./cmd/` | Pass (1.37 s) |
| Pruebas de dominio nuevas | `go test -count=1 -v -run 'Rework\|NextPhaseReports\|Cancel' ./internal/domain/` | Pass (todas las coincidencias en PASS, incluidas `TestReworkSupersededPhaseRestartRequiresDependencies` y `TestNextPhaseReportsSupersededPhaseWithSatisfiedDependencies`) |
| Race detector | `CGO_ENABLED=1 go test -count=1 -race ./...` | **Bloqueada**: `cgo: C compiler "gcc" not found`. No se afirma ausencia de data races en este ciclo. |

### Smoke tests en proyectos temporales

| ID | Escenario | Pasos y observación | Resultado |
|---|---|---|---|
| a | Completado → rework → validate → re-completar → validate → archive | `complete` (rev 17) → `rework --phase implementation` devuelve `reactivated: true`, 3 fases `superseded`, estado `active`. `validate` pasa (351/351 checks). Tras re-entregar y re-completar (rev 26), `validate` pasa (431/431). `archive` directo falla con `invalid_transition` ("phase archive is ready") porque la fase `archive` debe iniciarse y entregarse antes; tras `begin` y `deliver` de `archive`, `archive` tiene éxito y mueve el expediente a `archive/2026-10-14-feat-x`. `validate` de proyecto: 673/673. | Pass |
| b | `rework plan` → `begin implementation` → aprobar plan → `begin implementation` | Tras `rework --phase plan` (plan `ready`, implementation `superseded`), `begin implementation` devuelve `invalid_transition`: "phase implementation cannot restart from superseded until its dependencies are satisfied". `next` devuelve `plan` (`ready`, `create-plan`). Tras `begin`, `deliver` y `approve` de plan, `begin implementation` tiene éxito (`in_progress`). | Pass |
| c | Rework de implementation, re-entrega y `next --json` | Tras re-entregar implementation (rev 20), `next --json` devuelve `phase_id: verification`, `status: superseded`, procedimiento `verify`. `begin verification` luego tiene éxito. | Pass |
| d | Regresión de `cancel` | `cancel` (rev 1, `location: cancelled`) mueve el expediente a `cancelled/`. `status`, `next` y `validate <id>` devuelven `not_found` con "was cancelled". `start` con el mismo ID devuelve `already_exists` ("id is reserved by a prior cancellation"). Reintento con el mismo `--operation-id` devuelve exit 0 sin duplicar. `validate` de proyecto: 272/272. Manifest cancelado conserva fases (`prd: approved`, `specification: ready`). | Pass, con defecto D1 para `rework` sobre cancelado (ver abajo) |
| e | `adapters install claude-code` | Se instalan `.claude/commands/sdd/rework.md` y `.claude/commands/sdd/cancel.md`. `.claude/skills/sdd-cli/SKILL.md` contiene la sección `## Rework and cancel`. Los tres archivos son idénticos a los de `src/adapters/claude-code/`. `validate` de proyecto y de `live-y` pasan. | Pass |
| f | Casos de error de `rework` | Sin `--reason`: `invalid_arguments` (ver D2). Con `--reason "   "`: `invalid_input` ("--reason is required and cannot be empty"). Fase inexistente: `not_found`. | Pass con desviación D2 |
| g | Work item activo no afectado por cancelar otro | En `proj-e`, `live-y` activo tras cancelar `cancel-z`: `status`, `next` y `validate` pasan. | Pass |

Nota sobre la comprobación de archivo: el criterio "archive succeeds" del encargo se cumple solo después de ejecutar la fase `archive` (`begin` y `deliver`). Esto es el comportamiento del workflow, no un defecto.

### Verificación de lectura sobre el work item del repositorio

Solo lectura (`status`, `next`, `validate`) sobre `.sdd/work-items/active/feat-rework-cancel`:

- `status`: `active`, revisión 24, `verification: in_progress`, `implementation: completed`, `archive` y `human-code-review` `superseded`.
- `next`: `verification` (`in_progress`), procedimiento `verify`.
- `validate` del work item: 394/394 válidos. `validate` del proyecto: 615/615 válidos.

### Mapeo de los fixes del ciclo 2

| Fix | Qué cambia | Criterios relacionados | Verificación | Resultado |
|---|---|---|---|---|
| F1: `fs_validation_inspector.go` reconoce `work_item.reactivated` y `work_item.cancelled` | `validate` ya no falla tras `rework` de un item `completed` | CA-RW-03, CA-RW-08 | Smoke a; E2E `TestCLIReworkReactivatedWorkItemCanBeValidatedAndArchived`; el evento `work_item.reactivated` es el primero del lote (`from: completed`, `to: active`, `reason` correcto) | Pass |
| F2: `BeginPhase` exige dependencias satisfechas para reiniciar `superseded` | Se impide saltar el DAG | CA-RW-02, CA-RW-10 | Smoke b; `TestReworkSupersededPhaseRestartRequiresDependencies` | Pass |
| F3: `NextPhase` reporta fases `superseded` con dependencias satisfechas | `next` guía el reinicio en lugar de "No active phases pending" | CA-RW-02 (flujo de reinicio) | Smoke c; `TestNextPhaseReportsSupersededPhaseWithSatisfiedDependencies` | Pass |
| F4: adapter (`rework.md`, `cancel.md`, `SKILL.md`) y prueba de instalación | Documentación y distribución de los comandos | Sin criterio específico en la matriz; inspección | Smoke e (archivos idénticos a la fuente) | Pass por inspección |

### Matriz de trazabilidad (estado vigente)

Criterio → tarea del plan → cambio implementado → prueba o inspección → evidencia → estado. Los identificadores de evidencia se refieren a `evidence/`.

| CA | Ciclo 1 | Ciclo 2 | Prueba o inspección del ciclo 2 | Evidencia | Estado |
|---|---|---|---|---|---|
| CA-RW-01 | Pass | Pass | Smoke a (rework de implementation) y b (rework de plan: `implementation` superseded, `verification` blocked) | `ciclo2-evidence-a.log`, `ciclo2-evidence-b.log` | Pass |
| CA-RW-02 | Pass | Pass | Smoke a (`begin implementation`), b (`begin plan` tras rework) | `ciclo2-evidence-a2.log`, `ciclo2-evidence-b.log` | Pass |
| CA-RW-03 | Pass | Pass | Smoke a: `work_item.reactivated` primero, `status: active` | `ciclo2-evidence-a.log`, `ciclo2-events-smoke-a-archived.jsonl` | Pass |
| CA-RW-04 | Pass con observación | **Fail (literal)** | `rework` sin `--reason` devuelve `invalid_arguments` y no muta; la spec exige `invalid_input` (D2) | `ciclo2-evidence-f.log` | Fail (decisión humana) |
| CA-RW-05 | Pass | Pass | Fase inexistente: `not_found` | `ciclo2-evidence-f.log` | Pass |
| CA-RW-06 | Pass con observación | **Fail** | `rework` sobre `cancelled` devuelve `not_found` "work item not found"; la spec exige `invalid_transition` con mensaje de estado (D1) | `ciclo2-evidence-d.log` | Fail |
| CA-RW-07 | Pass | Pass | Tras `rework`, el estado `approved` de plan y human-code-review se conserva (smoke a y b). No se volvió a inspeccionar el arreglo `approvals` completo en este ciclo | `ciclo2-evidence-a.log` | Pass |
| CA-RW-08 | Pass | Pass | Eventos `rework_requested`, `rework_superseded` con `reason` idéntico al `--reason` (smoke a) | `ciclo2-events-smoke-a-archived.jsonl` | Pass |
| CA-RW-09 | Pass | Pass | `TestReworkPhaseAlreadySupersededDescendantIsSkipped`, `TestReworkUseCaseIsIdempotent` en suite completa | `ciclo2-test-all.txt` | Pass (pruebas) |
| CA-RW-10 | Pass | Pass | `TestReworkPhaseTargetTransitionMatrix` en suite; smoke b muestra `begin` de fase `superseded` con dependencias insatisfechas rechazado | `ciclo2-test-domain-new.txt`, `ciclo2-evidence-b.log` | Pass |
| CA-RW-11 | Pass | Pass | `TestReworkPhaseNotApplicableDescendantUnchanged` | `ciclo2-test-all.txt` | Pass (pruebas) |
| CA-RW-12 | Pass | Pass | `TestReworkUseCaseIsIdempotent`; reintento de `cancel` con mismo `--operation-id` devuelve exit 0 (smoke d) | `ciclo2-evidence-d.log` | Pass |
| CA-CA-01 | Pass | Pass | Smoke d: `active/cancel-x` ausente, `cancelled/cancel-x/manifest.yaml` con `status: cancelled`, evento `work_item.cancelled` con `reason` | `ciclo2-evidence-d.log` | Pass |
| CA-CA-02 | Pass | Pass | `TestCancelNonActiveWorkItemReturnsError/completed` en suite | `ciclo2-test-all.txt` | Pass (pruebas) |
| CA-CA-03 | Pass | Pass | `TestCancelNonActiveWorkItemReturnsError/archived` en suite | `ciclo2-test-all.txt` | Pass (pruebas) |
| CA-CA-04 | Pass con observación | **Fail (literal)** | `cancel` sin `--reason` devuelve `invalid_arguments` y no muta; la spec exige `invalid_input` (D2) | `ciclo2-evidence-f.log` | Fail (decisión humana) |
| CA-CA-05 | Pass | Pass | `start cancel-x` devuelve `already_exists` con mención a la cancelación | `ciclo2-evidence-d.log` | Pass |
| CA-CA-06 | Pass | Pass | Manifest cancelado conserva `prd: approved`, `specification: ready` | `ciclo2-evidence-d.log` | Pass |
| CA-CA-07 | Pass | Pass | `cancel` con actor por defecto `agent` devuelve exit 0 | `ciclo2-evidence-d.log` | Pass |
| CA-CA-08 | Parcial | **Parcial** | Sin pruebas de crash-recovery para `fs_cancel_repository.go` (G1). Ninguna prueba en `internal/infra` referencia cancelación. No se simuló interrupción. | Sin evidencia de ejecución | Parcial (no verificado) |
| CA-CA-09 | Pass | Pass | Smoke d: `status`, `next`, `validate` sobre ID cancelado devuelven `not_found` con "was cancelled" | `ciclo2-evidence-d.log` | Pass |
| CA-CA-10 | Pass | Pass | Reintento de `cancel` con mismo `--operation-id` devuelve exit 0 (smoke d) | `ciclo2-evidence-d.log` | Pass |

### Desviaciones y defectos

| ID | Tipo | Descripción | Criterios | Severidad | Decisión requerida |
|---|---|---|---|---|---|
| D1 | Defecto | `rework` sobre work item `cancelled` devuelve `not_found` ("failed to get work item: work item not found"). No indica que el item fue cancelado. `ReworkUseCase` no consulta el estado cancelado como sí hacen `next` y `validate`. ESC-RW-05 exige un mensaje que indique la cancelación. | CA-RW-06, ESC-RW-05 | Media | Corregir: inyectar la comprobación de cancelados en `ReworkUseCase` y devolver `invalid_transition` (o `not_found` con mensaje de cancelación) con prueba CLI. Alternativamente, aprobar explícitamente la degradación y enmendar la spec. |
| D2 | Desviación de código de error | Flag `--reason` ausente: Cobra devuelve `invalid_arguments` ("required flag(s) \"reason\" not set"); la spec indica `invalid_input` en CA-RW-04, CA-CA-04, ESC-RW-06 y ESC-CA-04. Ya señalado en el ciclo 1 (observación 1). | CA-RW-04, CA-CA-04 | Baja | Aceptar y enmendar la spec (la separación de capa CLI es coherente con otros flags) o mapear el error a `invalid_input`. |
| G1 | Brecha de cobertura | Sin pruebas de crash-recovery para `CancelWorkItem` en `infra` (las tiene `archive`). Brecha del ciclo 1 que sigue abierta. | CA-CA-08 | Media | Agregar pruebas análogas a las de archive en un work item de seguimiento. |
| G2 | Bloqueo de ejecución | `-race` no ejecutable: falta `gcc` en el entorno (cgo). | Transversal | Media | Ejecutar `CGO_ENABLED=1 go test -race ./...` en un entorno con compilador C. |
| G3 | No verificado | La afirmación de que `TestCLIReworkReactivatedWorkItemCanBeValidatedAndArchived` falla sin el fix no se reprodujo: requiere revertir código de producción, lo que queda fuera de alcance de esta verificación. | Fix F1 | Baja | Ninguna; se acepta la afirmación del reporte de implementación sin reproducción propia. |
| O1 | Observación | `next` no reporta fases `rejected`. Declarado fuera de alcance en el reporte de implementación. No evaluado. | — | Baja | Decidir en un work item posterior. |

### Conclusión del ciclo 2

- Verificado y en `Pass`: compilación, vet, formato, suite completa, E2E nuevo, pruebas de dominio nuevas, F1, F2, F3, F4 (por inspección), smoke de rework y de regresión de cancel, adaptador.
- No verificado por ejecución: race detector (bloqueado por entorno), recuperación de cancel (G1), y la reproducción de la prueba E2E sin el fix (G3).
- Veredicto: **`Failed`**, por D1 (defecto en CA-RW-06) y por CA-CA-08 sin verificar. D2 requiere decisión humana: si se acepta y se enmienda la spec, el veredicto puede pasar a `Passed` sin cambios de código.

### Acción requerida

1. Orquestador: decidir sobre D2 (aceptar y enmendar la spec, o alinear el código).
2. Desarrollador: corregir D1 y agregar la prueba CLI correspondiente, o documentar la degradación aprobada.
3. Para cerrar CA-CA-08: agregar pruebas de crash-recovery en `infra` (G1) y ejecutar `-race` en un entorno con `gcc` (G2).
4. Repetir esta verificación tras los cambios antes de cualquier `deliver` o `complete` de la fase.

### Evidencia

Archivos bajo `.sdd/work-items/active/feat-rework-cancel/evidence/` con prefijo `ciclo2-`:

| Archivo | Contenido |
|---|---|
| `ciclo2-build.txt`, `ciclo2-vet.txt`, `ciclo2-gofmt.txt` | Salida de compilación, vet y formato |
| `ciclo2-test-all.txt` | `go test -count=1 ./...` |
| `ciclo2-test-new-e2e.txt` | E2E `TestCLIReworkReactivatedWorkItemCanBeValidatedAndArchived` |
| `ciclo2-test-domain-new.txt` | Pruebas de dominio de rework, next y cancel |
| `ciclo2-test-race.txt` | Intento de `-race`: bloqueado por falta de `gcc` |
| `ciclo2-evidence-a.log`, `ciclo2-evidence-a2.log`, `ciclo2-evidence-a3.log` | Smoke a: rework tras completar, validate, re-completar, archive |
| `ciclo2-events-smoke-a-archived.jsonl`, `ciclo2-events-smoke-d-cancel-x.jsonl` | Eventos de los expedientes de smoke a (archivado) y d (cancelado) |
| `ciclo2-evidence-b.log` | Smoke b: guarda de `begin` sobre superseded |
| `ciclo2-evidence-d.log` | Smoke d: regresión de cancel |
| `ciclo2-evidence-e.log` | Smoke e: adaptador `claude-code` instalado |
| `ciclo2-evidence-f.log` | Casos de error: sin `--reason`, espacios, fase inexistente, cancelado |
| `ciclo2-repo-status.json`, `ciclo2-repo-next.json`, `ciclo2-repo-validate-wi.json`, `ciclo2-repo-validate-project.json` | Lectura del work item del repositorio |
| `ciclo2-scripts/` | Scripts de smoke (`lib.sh`, `scenario_*.sh`, `summ.py`) para reproducir |

Los archivos de la primera re-verificación (`test-run-race.txt`, `smoke-test-transcript.txt`) se conservan sin cambios.

---

## Re-verificación 2026-10-14 (ciclo 3)

### Alcance y fuentes

- Criterio de aceptación en revisión: CA-CA-08 ("Si el proceso es interrumpido durante el movimiento y se reintenta, el motor recupera un estado consistente (sin expediente parcialmente movido)."). Tarea del plan: recuperación de `cancel` (`recoverCancelIfNeeded` / `recoverCancelTransactionLocked`, réplica de la recuperación de `archive`).
- Reporte de implementación, sección "Rework 2026-10-14". El único cambio declarado en el ciclo 3 es el archivo nuevo `src/cli/internal/infra/cancel_repository_test.go`.
- No hay snapshot del ciclo 2 para hacer diff. Se leyó íntegro `src/cli/internal/infra/fs_cancel_repository.go` y el archivo de pruebas nuevo. No se modificó código de producción ni pruebas del repositorio.
- No se ejecutó ningún comando `sdd-cli` contra el `.sdd` del repositorio en este ciclo. Los mutantes se ejecutaron sobre copias en `/tmp/sdd-verif3/`, fuera del repositorio.

### Entorno

| Elemento | Valor |
|---|---|
| Go | `go1.27.2 linux/amd64` (módulo `sdd-cli`, `go 1.22`) |
| Sistema | Linux 6.18.40.1-microsoft-standard-WSL2 |
| Compilador C | ausente: `gcc`, `cc` y `clang` no están en el `PATH` (`CGO_ENABLED=0` por defecto) |
| Binario de regresión | `go build -o /tmp/sdd-verif3/bin/sdd-cli .` desde una copia de `src/` |
| Copia para mutantes | `/tmp/sdd-verif3/base/src` (incluye `src/cli` y `src/.sdd`) |

Nota de reproducibilidad: las pruebas de `internal/infra` resuelven los fixtures desde `../../..` relativo al paquete, es decir `src/.sdd`. Una copia que incluya solo `src/cli` falla con `lstat .../.sdd: no such file or directory`. La primera ejecución de mutación se descartó por esa causa.

### Comandos ejecutados

| Comprobación | Comando (en `src/cli`) | Resultado | Evidencia |
|---|---|---|---|
| Compilación | `go build ./...` | Pass (exit 0) | `ciclo3-build.txt` |
| Vet | `go vet ./...` | Pass (exit 0) | `ciclo3-vet.txt` |
| Formato | `gofmt -l .` | Pass (sin archivos listados) | `ciclo3-gofmt.txt` |
| Suite completa | `go test -count=1 ./...` | Pass: `cmd`, `internal/domain`, `internal/infra`, `internal/usecases` en `ok` | `ciclo3-test-all.txt` |
| Cancel en infra, verbose | `go test -count=1 -v ./internal/infra -run Cancel` | Pass: 7 pruebas en PASS | `ciclo3-test-infra-cancel-v.txt`, `ciclo3-test-infra-cancel.json` |
| Race detector | `CGO_ENABLED=1 go test -race -count=1 ./internal/domain/` | **Bloqueada**: `cgo: C compiler "gcc" not found` | `ciclo3-race-probe.txt` |
| Mutación (control) | `go test -count=1 ./internal/infra -run Cancel` sobre copia sin cambios | Pass | `ciclo3-mutation-results.txt` |
| Mutación (17 mutantes) | Script `ciclo3-scripts/mutate.py` sobre copia | 12 detectados, 5 sobreviven (ver tabla) | `ciclo3-mutation-results.txt` |
| Regresión smoke | `ciclo3-scripts/scenario_*.sh` contra el binario reconstruido | Idénticos a ciclo 2 en labels y `exit` (ver tabla) | `ciclo3-smoke-compare-vs-ciclo2.txt`, `ciclo3-evidence-*.log` |

Las 7 pruebas de `cancel` en `internal/infra` pasan: `TestCancelWorkItemPublishesSnapshotAndCleansTransaction`, `TestCancelWorkItemRollsBackAfterActiveBackupFailure`, `TestCancelWorkItemRollsBackAfterDestinationPublishFailure`, `TestGetWorkItemRecoversInterruptedCancelBeforeBackup`, `TestGetWorkItemRecoversInterruptedCancelBeforePublish`, `TestFindCancelledWorkItemRecoversInterruptedCancelAfterPublish` y `TestFindCancelledWorkItemRejectsPublishedCancelMissingOperation`.

### Verificación de CA-CA-08: cobertura por rama

Se revisaron las ramas de `recoverCancelTransactionLocked` y de los rollbacks en `fs_cancel_repository.go`. Para cada rama se indica la prueba que la ejercita y si un mutante sobre esa rama es detectado.

| Rama | Punto de interrupción o causa | Prueba que la ejerce | Mutante | Estado |
|---|---|---|---|---|
| Sin marker (camino normal) | Ninguna | Todas las pruebas de publicación y rollback (limpieza final) | n/a | Cubierta |
| `activeExists` | Crash antes del backup: marker y stage, active intacto | `TestGetWorkItemRecoversInterruptedCancelBeforeBackup` | M4 detectado | Cubierta |
| `backupExists` (solo backup) | Crash tras mover active a backup y antes de publicar | `TestGetWorkItemRecoversInterruptedCancelBeforePublish` | M3, M6 detectados | Cubierta |
| `destinationExists` | Crash tras publicar, con backup y marker residuales | `TestFindCancelledWorkItemRecoversInterruptedCancelAfterPublish` | M11, M12 detectados; M5 (stage residual) sobrevive | Parcial |
| Publicado sin la operación del marker | Conflicto, no limpieza silenciosa | `TestFindCancelledWorkItemRejectsPublishedCancelMissingOperation` | M2 detectado | Cubierta |
| Recuperación desde lectura | `GetWorkItem` y `FindCancelledWorkItem` disparan la recuperación | Pruebas de `Get` y `Find` anteriores | M1 y M15 detectados | Cubierta |
| Rollback tras fallo en `cancel_after_active_backup` | `rollbackCancelBeforePublish` | `TestCancelWorkItemRollsBackAfterActiveBackupFailure` | M7, M13 detectados | Cubierta |
| Rollback tras fallo en `cancel_after_destination_publish` | `rollbackPublishedCancel` | `TestCancelWorkItemRollsBackAfterDestinationPublishFailure` | M8, M14 detectados | Cubierta |
| `activeExists && destinationExists` | Estado imposible: conflicto `ErrArchiveConflict` | Ninguna | M9 sobrevive | Sin prueba (G4) |
| Marker con `WorkItem` o `Destination` distinto | Conflicto `ErrArchiveConflict` | Ninguna | M10 sobrevive | Sin prueba (G5) |
| Marker JSON inválido o `schema_version` distinto | Conflicto `ErrArchiveConflict` | Ninguna | No mutado | Sin prueba (G6) |
| Marker sin snapshot recuperable | Error `ErrArchiveConflict` y marker conservado | Ninguna | M17 sobrevive | Sin prueba (G6) |
| Publicado con manifest ilegible o `status` distinto de `cancelled` | Conflicto `ErrArchiveConflict` | Ninguna | No mutado | Sin prueba (G7) |
| Backup residual junto a `active` | Limpieza del backup en la rama `activeExists` | Ninguna | M16 sobrevive | Sin prueba (G8) |

**Resultado para CA-CA-08: Pass.** Las pruebas cubren los tres puntos de interrupción del protocolo (antes del backup, tras el backup y tras la publicación) y los dos rollbacks inyectados. Las aserciones son significativas: `assertActiveRestored` exige estado `active` y `revision 1` tras la recuperación; `assertCancelTransactionClean` exige que no queden stage, backup ni marker; y las pruebas de conflicto exigen `ErrArchiveConflict`. Las ramas sin prueba son defensivas y no permiten que un expediente quede parcialmente movido en los escenarios de interrupción modelados.

Observación de fidelidad (no bloqueante): en `TestFindCancelledWorkItemRecoversInterruptedCancelAfterPublish` el backup residual se simula copiando el destino publicado. Un crash real dejaría el snapshot original de `active`. La recuperación solo elimina ese backup en esa rama, así que el resultado observable es el mismo, pero la prueba no reproduce el contenido real del backup.

### Resultados de mutación

Se aplicaron 17 mutantes sobre `fs_cancel_repository.go` en una copia fuera del repositorio. Cada mutante se ejecutó con `go test -count=1 ./internal/infra -run Cancel`. El control sin mutar pasa.

| Mutante | Descripción | Resultado |
|---|---|---|
| M1 | Recuperación desactivada en `recoverCancelTransactionLocked` | Detectado (4 pruebas fallan) |
| M2 | Se omite la verificación de la operación en el destino publicado | Detectado |
| M3 | Rama de backup elimina en lugar de restaurar | Detectado |
| M4 | Rama `activeExists` conserva el stage | Detectado |
| M5 | Rama publicada conserva el stage | **Sobrevive** (G7) |
| M6 | Rama de backup conserva el stage | Detectado |
| M7 | Rollback previo a publicar no restaura el backup | Detectado |
| M8 | Rollback posterior a publicar no restaura el backup | Detectado |
| M9 | Se omite el conflicto `activeExists && destinationExists` | **Sobrevive** (G4) |
| M10 | Se omite la verificación de `marker.Destination` | **Sobrevive** (G5) |
| M11 | Rama publicada conserva el backup | Detectado |
| M12 | Rama publicada conserva el marker | Detectado |
| M13 | Hook `cancel_after_active_backup` ignorado | Detectado |
| M14 | Hook `cancel_after_destination_publish` ignorado | Detectado |
| M15 | `FindCancelledWorkItem` omite la recuperación | Detectado |
| M16 | Rama `activeExists` conserva el backup | **Sobrevive** (G8) |
| M17 | Se elimina el error de marker sin snapshot | **Sobrevive** (G6) |

Resumen: 12 detectados y 5 sobreviven. Los mutantes que sobreviven afectan solo a ramas defensivas o de limpieza sin prueba. Ninguno corresponde a los tres puntos de interrupción del protocolo.

### Regresión smoke (binario reconstruido)

| Escenario | Pasos | Labels y `exit` frente a ciclo 2 | Pasos con código distinto de 0 (esperados) |
|---|---|---|---|
| a (rework, validate, re-completar, archive) | 25 | Idénticos | 0 |
| a2 (archive) | 13 | Idénticos | 1 (`archive` antes de `begin`/`deliver`, esperado) |
| a3 | 6 | Idénticos | 0 |
| b (guarda de `begin` sobre superseded) | 21 | Idénticos | 1 (`begin implementation` con dependencias pendientes, esperado) |
| d (regresión de cancel) | 17 | Idénticos | 7 (operaciones sobre items cancelados, esperadas) |
| e (adaptador `claude-code`) | 10 | Idénticos | 0 |
| f (errores de `rework` y `cancel`) | 5 | Idénticos | 4 (casos de error esperados) |

Nota sobre el escenario d: el paso `cancel-other-item` cancela `keep-x`, no un ítem que deba quedar intacto. Por eso los pasos `status-kept` y `validate-keep` devuelven `not_found` "was cancelled". Es un nombre de paso del script y el comportamiento es consistente.

### Desviaciones aceptadas por el revisor humano

| ID | Descripción | Criterios | Evidencia del ciclo 3 | Estado |
|---|---|---|---|---|
| D1 | `rework` sobre un work item cancelado devuelve `not_found` ("work item not found") en lugar de un error que indique la cancelación | CA-RW-06, ESC-RW-05 | `ciclo3-evidence-d.log`, paso `rework-cancelled` | Desviación aceptada por el revisor humano (matias, 2026-10-14). Sin cambio de código. |
| D2 | `--reason` ausente devuelve `invalid_arguments` (error de Cobra) en lugar de `invalid_input` | CA-RW-04, CA-CA-04, ESC-RW-06, ESC-CA-04 | `ciclo3-evidence-f.log`, pasos `rework-no-reason` y `cancel-no-reason` | Desviación aceptada por el revisor humano (matias, 2026-10-14). Sin cambio de código. |

Con `--reason` de solo espacios, `rework` devuelve `invalid_input` (`ciclo3-evidence-f.log`, paso `rework-whitespace-reason`), que sí coincide con la especificación.

### Matriz de trazabilidad (ciclo 3)

Criterio → tarea del plan → cambio implementado → prueba o inspección → evidencia → estado. Los criterios sin cambio respecto al ciclo 2 se confirman con la regresión (suite y smoke idénticos).

| CA | Tarea del plan | Cambio implementado | Prueba o inspección (ciclo 3) | Evidencia | Estado |
|---|---|---|---|---|---|
| CA-RW-01 | Rework de fases | Sin cambio en ciclo 3 | Regresión smoke a, b | `ciclo3-evidence-a.log`, `ciclo3-evidence-b.log` | Pass |
| CA-RW-02 | Reinicio de fases superseded | Sin cambio en ciclo 3 | Regresión smoke a, b, pruebas de dominio en suite | `ciclo3-test-all.txt`, `ciclo3-evidence-b.log` | Pass |
| CA-RW-03 | Evento `work_item.reactivated` | Sin cambio en ciclo 3 | Regresión smoke a | `ciclo3-evidence-a.log` | Pass |
| CA-RW-04 | Validación de `--reason` | Sin cambio | Regresión smoke f: `invalid_arguments` | `ciclo3-evidence-f.log` | Pass con desviación aceptada (D2) |
| CA-RW-05 | Fase inexistente | Sin cambio | Regresión smoke f: `not_found` | `ciclo3-evidence-f.log` | Pass |
| CA-RW-06 | Rework sobre cancelado | Sin cambio | Regresión smoke d: `not_found` | `ciclo3-evidence-d.log` | Pass con desviación aceptada (D1) |
| CA-RW-07 | Preservación de approvals | Sin cambio | Regresión smoke a, b | `ciclo3-evidence-a.log` | Pass |
| CA-RW-08 | Eventos de rework | Sin cambio | Regresión smoke a | `ciclo3-evidence-a.log` | Pass |
| CA-RW-09 a CA-RW-12 | Idempotencia, matriz de transición, descendientes | Sin cambio | Suite completa (`TestReworkUseCaseIsIdempotent`, etc.) | `ciclo3-test-all.txt` | Pass |
| CA-CA-01 | Movimiento a `cancelled/` | Sin cambio | Regresión smoke d | `ciclo3-evidence-d.log` | Pass |
| CA-CA-02, CA-CA-03 | Rechazo de estados no activos | Sin cambio | Suite completa | `ciclo3-test-all.txt` | Pass |
| CA-CA-04 | Validación de `--reason` | Sin cambio | Regresión smoke f: `invalid_arguments` | `ciclo3-evidence-f.log` | Pass con desviación aceptada (D2) |
| CA-CA-05 | Identificador reservado | Sin cambio | Regresión smoke d: `already_exists` | `ciclo3-evidence-d.log` | Pass |
| CA-CA-06, CA-CA-07 | Preservación de fases, actor por defecto | Sin cambio | Regresión smoke d | `ciclo3-evidence-d.log` | Pass |
| CA-CA-08 | Recuperación consistente tras interrupción | Pruebas nuevas en `cancel_repository_test.go` (sin cambio de producción) | 7 pruebas en PASS; 12 de 17 mutantes detectados; cobertura de ramas en la sección anterior | `ciclo3-test-infra-cancel-v.txt`, `ciclo3-mutation-results.txt` | **Pass** (G1 cerrada) |
| CA-CA-09, CA-CA-10 | Lectura de cancelado, reintento idempotente | Sin cambio | Regresión smoke d | `ciclo3-evidence-d.log` | Pass |

### Brechas de cobertura y estado de G1 y G2

| ID | Tipo | Descripción | Severidad | Recomendación |
|---|---|---|---|---|
| G1 | Brecha de cobertura | Sin pruebas de crash-recovery de `cancel` | Media | **Cerrada** en ciclo 3 (pruebas de crash-recovery y rollback; mutación M1 detectada) |
| G2 | Bloqueo de entorno | `-race` no ejecutable: falta `gcc` | Media | Abierta. Limitación documentada. Ejecutar `CGO_ENABLED=1 go test -race ./...` en un entorno con compilador C. La especificación no exige el detector de carreras. |
| G3 | No verificado | La prueba E2E no se reprodujo sin el fix F1 | Baja | Sin cambio desde el ciclo 2 |
| G4 | Cobertura defensiva | Conflicto `activeExists && destinationExists` sin prueba (M9 sobrevive) | Media | Agregar prueba que exija `ErrArchiveConflict` y no modifique el expediente. |
| G5 | Cobertura defensiva | Marker con `WorkItem` o `Destination` distinto sin prueba (M10 sobrevive) | Media | Agregar prueba que exija `ErrArchiveConflict` para cada campo. |
| G6 | Cobertura defensiva | Marker JSON inválido, `schema_version` distinto y marker sin snapshot sin prueba (M17 sobrevive) | Media | Agregar pruebas que exijan `ErrArchiveConflict` y que el marker se conserve. |
| G7 | Cobertura defensiva | Rama publicada: stage residual sin prueba (M5 sobrevive); manifest ilegible o estado distinto de `cancelled` sin prueba | Baja | Agregar prueba con stage residual y con manifest inválido en el destino. |
| G8 | Cobertura defensiva | Backup residual junto a `active` sin prueba (M16 sobrevive) | Baja | Agregar prueba que verifique la limpieza del backup. |
| O1 | Observación | `next` no reporta fases `rejected` (fuera de alcance del reporte de implementación) | Baja | Decidir en un work item posterior. |

Las brechas G4 a G8 no bloquean el veredicto porque los criterios aplicables se cumplen con las pruebas existentes. Se recomienda registrarlas como trabajo de seguimiento, no como condición para cerrar esta fase.

### Conclusión del ciclo 3

- Verificado por ejecución y en `Pass`: compilación, vet, formato, suite completa, pruebas de `cancel` en `infra` (7), regresión smoke de los siete escenarios del ciclo 2, control de mutación y CA-CA-08.
- Desviaciones D1 y D2: aceptadas por el revisor humano, verificadas por smoke.
- Bloqueada por entorno: `-race` (G2). No se afirma ausencia de carreras en este ciclo.
- **Veredicto: `Passed`.** Esta verificación no marca la fase como completada: esa transición la realiza el motor.

### Evidencia del ciclo 3

Archivos bajo `.sdd/work-items/active/feat-rework-cancel/evidence/` con prefijo `ciclo3-`:

| Archivo | Contenido |
|---|---|
| `ciclo3-build.txt`, `ciclo3-vet.txt`, `ciclo3-gofmt.txt` | Compilación, vet y formato |
| `ciclo3-test-all.txt` | `go test -count=1 ./...` |
| `ciclo3-test-infra-cancel-v.txt`, `ciclo3-test-infra-cancel.json` | Pruebas de `cancel` en `internal/infra`, salida verbose y JSON |
| `ciclo3-race-probe.txt` | Intento de `-race`: bloqueado por falta de `gcc` |
| `ciclo3-mutation-results.txt` | Control y 17 mutantes sobre `fs_cancel_repository.go` |
| `ciclo3-smoke-compare-vs-ciclo2.txt` | Comparación de labels y códigos de salida frente a ciclo 2 |
| `ciclo3-evidence-a.log`, `ciclo3-evidence-a2.log`, `ciclo3-evidence-a3.log`, `ciclo3-evidence-b.log`, `ciclo3-evidence-d.log`, `ciclo3-evidence-e.log`, `ciclo3-evidence-f.log` | Logs de smoke (rutas temporales bajo `/tmp/sdd-verif3/`, sin credenciales) |
| `ciclo3-scripts/` | Scripts de smoke adaptados a `/tmp/sdd-verif3/` (`lib.sh`, `scenario_*.sh`, `summ.py`) y `mutate.py` para repetir la mutación |

Para repetir: copiar `src/` completo (incluye `src/.sdd`) a un directorio temporal, compilar el binario fuera del repositorio y ejecutar `python3 -I ciclo3-scripts/mutate.py` desde ese directorio. Ajustar `BASE` si la ruta de copia cambia.

## Re-verificación 2026-10-14 (ciclo 4)

Verificación inline del orquestador. Alcance: `init` escribe `.sdd/work-items/.gitignore` con
`.locks/` y `.transactions/` (`project_initializer.go`).

| Comprobación | Resultado |
|---|---|
| `go build ./...`, `go vet ./...`, `gofmt -l .` en `src/cli` | Pass |
| `go test -count=1 ./...` | Pass (cmd, domain, infra, usecases) |
| `TestInitializeIgnoresEngineRuntimeDirectories` | Pass |
| Tests de instalación de adapters sobre proyecto inicializado | Pass (5/5) |
| Smoke: `git init` + `sdd-cli init` + `start` en un proyecto temporal | `validate` 301/301; `git status` no lista `.locks/` ni `.transactions/` |

Nota: la evidencia bruta de `evidence/` no se versiona en Git por decisión del revisor
(matias, 2026-10-14); permanece solo en la copia local del expediente.

