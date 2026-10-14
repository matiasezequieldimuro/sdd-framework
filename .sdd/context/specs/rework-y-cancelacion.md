---
schema_version: "0.1"
kind: reconstructed-spec
id: "rework-y-cancelacion"
capability: "rework / cancel"
status: consolidada-desde-spec-aprobada-y-codigo
created_at: "2026-10-14"
created_by: { kind: "agent", id: "sdd-functional-analyst" }
sources:
  - src/cli/cmd/rework.go
  - src/cli/cmd/cancel.go
  - src/cli/cmd/root.go
  - src/cli/internal/usecases/rework_uc.go
  - src/cli/internal/usecases/cancel_uc.go
  - src/cli/internal/usecases/transition_helpers.go
  - src/cli/internal/infra/fs_cancel_repository.go
  - src/cli/internal/infra/fs_archive_repository.go
  - src/cli/internal/infra/fs_repository.go
  - src/cli/internal/infra/fs_validation_inspector.go
  - src/cli/internal/domain/work_item.go
  - src/cli/internal/domain/work_item_validation.go
  - src/cli/internal/domain/workflow_validation.go
  - src/cli/internal/domain/errors.go
---

# Especificación: Retrabajo de fases y cancelación de work items (`rework` / `cancel`)

> Especificación consolidada desde la especificación aprobada del work item
> `feat-rework-cancel` y verificada contra el código fuente. Cuando ambos divergen, se
> documenta el comportamiento del código (fuente de verdad prioritaria) y se señala la
> divergencia.

## Requisitos y comportamiento

### Propósito

- `rework` devuelve una fase ya resuelta a `ready` e invalida (`superseded`) o vuelve a bloquear
  (`blocked`) sus fases descendientes, sin borrar historial. Permite reabrir también un work
  item `completed`.
- `cancel` termina de forma permanente un work item `active`: lo mueve de forma transaccional
  a `.sdd/work-items/cancelled/<id>/` y **reserva su ID**.

### `rework <work-item-id>`

- Flags: `--phase`/`-p` (**requerido**), `--reason` (**requerido**, no vacío tras `TrimSpace`),
  `--actor-kind` (default `agent`), `--actor-id` (default `agent`), `--operation-id`.
- Precondiciones (`WorkItem.ReworkPhase`):
  - El work item debe existir en `active/` y su estado debe ser `active` o `completed`
    (`archived`/`cancelled` → `ErrInvalidTransition`).
  - La fase debe existir en el workflow y en el manifest (`ErrPhaseNotFound`).
  - La fase objetivo debe estar en `completed`, `approved` o `accepted`; cualquier otro estado
    → `ErrInvalidTransition`.
- Efectos, en este orden:
  1. Si el item estaba `completed`, pasa a `active` (reactivación).
  2. La fase objetivo pasa a `ready`.
  3. Cada **descendiente transitivo** (en orden topológico) se transforma así:
     - `completed`, `approved`, `accepted`, `in_progress`, `awaiting_approval`, `rejected`
       → `superseded`.
     - `ready` → `blocked` (estaba desbloqueada sin iniciar; se vuelve a bloquear).
     - `blocked`, `not_applicable`, `superseded` → sin cambio.
- No modifica artefactos, registros `Approval` ni fases no descendientes. La `revision` se
  incrementa en 1 al commitear (capa de infraestructura).
- Tras el rework, `begin` sobre la fase objetivo (`ready`) la lleva a `in_progress`; las fases
  `superseded` se reinician con `begin` solo cuando sus dependencias vuelven a estar
  satisfechas (antes, `begin` falla con `invalid_transition`). `next` reporta la primera fase
  `superseded` reiniciable cuando no hay fases `awaiting_approval`, `in_progress` ni `ready`.
- Eventos (todos con `data.reason` = texto de `--reason` y `correlation_id` = `--operation-id`),
  en orden:
  1. `work_item.reactivated` (`{from: completed, to: active, reason}`), solo si hubo reactivación.
  2. `phase.transitioned` de la fase objetivo (cause `rework_requested`).
  3. `phase.transitioned` por cada descendiente transicionado (cause `rework_superseded`),
     incluidos los `ready → blocked`.
- Salida:
  - JSON: `{ work_item, superseded (cantidad de descendientes transicionados), reactivated }`.
  - Texto: `Work item '<id>' reactivated and phase '<fase>' reworked. N phase(s) superseded.`
    si hubo reactivación; si no, `Work item '<id>': phase '<fase>' reworked. N phase(s) superseded.`
- Idempotencia: con `--operation-id` ya aplicado (evento con ese `correlation_id`) devuelve el
  estado persistido actual sin mutar; en ese caso `superseded` es 0 y `reactivated` es false
  en la respuesta (no se reconstruye el resumen original).

### `cancel <work-item-id>`

- Flags: `--reason` (**requerido**, no vacío), `--actor-kind` (default `agent`), `--actor-id`
  (default `agent`), `--operation-id`.
- Precondiciones: el work item debe existir en `active/` con estado `active`. Un item
  `completed` no se puede cancelar (`ErrWorkItemCannotCancel`); uno ya cancelado con distinto
  `--operation-id` → `ErrWorkItemCannotCancel`; uno archivado o inexistente → `ErrWorkItemNotFound`.
- Efectos: el estado pasa a `cancelled`; las fases **no se modifican**; `revision` +1. El
  directorio completo (manifest, artefactos, `events.jsonl`) se mueve a
  `.sdd/work-items/cancelled/<id>/` (sin prefijo de fecha). `cancelled/` se crea de forma lazy
  en el primer cancel.
- Evento único `work_item.cancelled` (`{from: active, to: cancelled, reason}`,
  `correlation_id` = `--operation-id`), agregado al `events.jsonl` del expediente movido.
- Movimiento transaccional (análogo a `archive`): stage (`.transactions/<id>.cancel-stage`) →
  marcador (`.transactions/<id>.cancel.json`) → backup del activo (`.cancel-backup`) →
  publicación (rename a `cancelled/<id>`) → limpieza. Toma el lock del work item y valida la
  `revision` esperada (`ErrConcurrentModification`). No hay gate de artefactos ni de `validate`.
- Recuperación ante crash (`recoverCancelIfNeeded`, ejecutada al leer el item o cancelar):
  - destino publicado → valida estado `cancelled` y la operación en el log; limpia backup/stage/marcador;
  - activo presente sin publicar → descarta stage/backup/marcador (rollback);
  - solo backup → restaura el activo desde el backup y limpia;
  - activo y destino visibles a la vez, marcador inválido o inconsistente → `ErrArchiveConflict`.
- Salida: JSON `{ work_item, location: "cancelled" }`; texto `Work item '<id>' cancelled.`
- Idempotencia: si el item ya está en `cancelled/` y el `--operation-id` figura en su log,
  devuelve el resultado existente sin reaplicar.

### Reserva de ID y opacidad

- Un ID cancelado queda **reservado** de forma permanente: `start` con ese ID falla
  (`ErrWorkItemAlreadyExists`, ver `creacion-de-work-item`).
- Los expedientes cancelados son opacos para lecturas generales: `status`, `next` y `validate`
  reportan `ErrWorkItemCancelled` (código `not_found`) sin exponer su contenido (ver
  `consultas-de-estado` y `validacion-de-integridad`). Cualquier comando que opere sobre el
  expediente `active/` (incluidos `rework` y `begin`) lo ve como inexistente.
- Invariante de ubicación única: un ID no puede estar en más de una de `active/`, `archive/`
  y `cancelled/`; de lo contrario `ErrArchiveConflict` (`archive_conflict`).

## Escenarios verificables

- `rework x -p plan --reason "..."` con `plan` aprobado, `implementation` completed y
  `verification` ready → `plan=ready`, `implementation=superseded`, `verification=blocked`;
  `superseded: 2`; eventos `rework_requested` + 2 × `rework_superseded`.
- `rework` sobre un item `completed` → `reactivated: true`, item `active`, primer evento
  `work_item.reactivated`.
- `rework` sobre fase en `in_progress`/`ready`/`blocked`/`not_applicable`/`superseded`/
  `rejected`/`awaiting_approval` → `invalid_transition`, sin mutación.
- `rework` con `--phase` inexistente → `not_found`; sin `--reason` → `invalid_arguments`.
- Reintento de `rework` / `cancel` con el mismo `--operation-id` → sin eventos duplicados.
- `cancel x --reason "..."` → `cancelled/x/` existe, `active/x/` no; estado `cancelled`; evento
  `work_item.cancelled`; `start x` posterior → `already_exists`.
- `cancel` de un item `completed` → `invalid_transition`; de uno inexistente/archivado → `not_found`.

## Reglas, contratos y restricciones

- `--reason` es obligatorio en ambos comandos (validado por cobra y de nuevo en el use case) y
  se registra en cada evento del lote.
- Actor: cualquier `kind` válido (`human|agent|cli|system`); a diferencia de `approve`/`reject`
  no se exige actor humano.
- `rework` no reaprueba ni borra aprobaciones: los registros `Approval` se conservan.
- Las fases `superseded` están exentas de `work_item.dependencies_unsatisfied` en `validate`.
- `cancel` es irreversible: no existe comando de reactivación de un item cancelado.

## Errores y códigos (envelope JSON)

- `ErrInvalidTransition`, `ErrWorkItemCannotCancel` → `invalid_transition`.
- `ErrWorkItemNotFound`, `ErrPhaseNotFound`, `ErrWorkItemCancelled` → `not_found`.
- `ErrConcurrentModification` → `concurrent_modification`; `ErrWorkItemLocked` → `work_item_locked`.
- `ErrArchiveConflict` → `archive_conflict`.
- Actor/operation-id inválidos, `--reason` vacío por use case → `invalid_input`; flag
  requerido ausente (`--phase`, `--reason`) → `invalid_arguments`.

## Dependencias con otras capacidades

- Usa la persistencia transaccional, locking e idempotencia descritas en
  `trazabilidad-por-eventos` y `transiciones-de-fase-y-gates`.
- Comparte el patrón de movimiento transaccional con `cierre-de-work-item`.

## Deltas sobre baseline

Capacidad nueva; los efectos en otras especificaciones se aplicaron como deltas en
`transiciones-de-fase-y-gates`, `consultas-de-estado`, `creacion-de-work-item`,
`validacion-de-integridad`, `trazabilidad-por-eventos` y `cierre-de-work-item`.

- **Divergencias aceptadas** (revisor humano, 2026-10-14) respecto de la spec aprobada: (1) `--reason` ausente produce `invalid_arguments`
  (error de cobra por flag requerido) y no `invalid_input`; (2) `rework` sobre un item
  cancelado devuelve `not_found` (el expediente no es visible en `active/`) y no
  `invalid_transition`; la guarda de dominio existe pero es inalcanzable por el CLI.
- **Validación de items reactivados**: el inspector de eventos (`fs_validation_inspector.go`)
  reconoce `work_item.reactivated` como transición `completed → active`, por lo que un item
  reactivado pasa `validate` y puede volver a completarse y archivarse (cubierto por
  `TestCLIReworkReactivatedWorkItemCanBeValidatedAndArchived`).
- **Recovery de `cancel`**: rollback ante fallos tras el backup y tras la publicación, y recovery
  de crash en las tres ramas, cubiertos por `internal/infra/cancel_repository_test.go`.

## Trazabilidad

- `src/cli/cmd/{rework,cancel}.go` — flags, defaults y salida.
- `src/cli/internal/usecases/{rework_uc,cancel_uc}.go` — orquestación, idempotencia.
- `src/cli/internal/usecases/transition_helpers.go` — `reworkEvents`.
- `src/cli/internal/domain/work_item.go` — `ReworkPhase`, `Cancel`, `ReworkMutation`.
- `src/cli/internal/domain/workflow_validation.go` — `Descendants`.
- `src/cli/internal/infra/fs_cancel_repository.go` — movimiento transaccional y recovery.
- `src/cli/internal/infra/{fs_archive_repository,fs_repository}.go` — unicidad de ubicación, opacidad.
