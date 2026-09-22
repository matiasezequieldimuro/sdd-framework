---
schema_version: "0.1"
kind: reconstructed-spec
id: "trazabilidad-por-eventos"
capability: "record-event / events.jsonl"
status: reconstruida-desde-codigo
created_at: "2026-09-22"
created_by: { kind: "agent", id: "sdd-doc-subagent" }
sources:
  - src/cli/cmd/record_event.go
  - src/cli/internal/usecases/record_event_uc.go
  - src/cli/internal/usecases/transition_helpers.go
  - src/cli/internal/domain/event.go
  - src/cli/internal/infra/fs_repository.go
  - .sdd/schemas/event.schema.json
---

# Especificación: Trazabilidad por eventos (`record-event` y `events.jsonl`)

> Especificación reconstruida desde el código fuente durante el onboarding del motor
> `sdd-cli`. La fuente de verdad prioritaria es el código.

## Requisitos y comportamiento

### Propósito

Cada work item mantiene un log **append-only** `events.jsonl` (JSON Lines, un evento por
línea) que constituye su historial inmutable. Toda operación mutadora agrega uno o más
eventos; `record-event` permite además registrar eventos **custom** desde un agente. Junto al
`manifest.yaml` (estado) y a `artifacts/*.md` (evidencia), `events.jsonl` es una de las tres
fuentes de verdad del work item.

### Estructura de un evento (`domain.Event` / `event.schema.json`)

Campos: `schema_version` (const `"0.1"`), `id` (string no vacío), `at` (RFC3339Nano UTC,
formato `date-time`), `work_item` (kebab-case), `type` (patrón
`^[a-z_]+(?:[.-][a-z_]+)*$`), `actor` (`{ kind: human|agent|cli|system, id }`), `data`
(objeto) y `correlation_id` (opcional). `additionalProperties: false`. El `id` lo produce el
`IDGenerator`; `at` se sella con el `Clock`.

### `record-event <work-item-id>`

- Flags: `--type`/`-t` (**requerido**), `--message`/`-m`, `--actor-kind` (default `agent`),
  `--actor-id` (default `agent`), `--operation-id`.
- Comportamiento (`RecordEventUseCase`): valida el actor, obtiene el work item **activo**
  (`GetWorkItem`), aplica el chequeo de idempotencia y, si no fue aplicado, construye el `data`
  (agrega `message` si se pasó `--message`), genera el evento (con `correlation_id = operation-id`)
  y lo commitea (append a `events.jsonl` + manifest con `revision+1`, transaccional).
- Salida: JSON envelope con `data: "Event recorded successfully"`; texto
  `Successfully recorded event '<type>' for work item '<id>'.`

### Tipos de evento emitidos por el motor

- `work_item.created` — creación (primer evento; `event.first_is_creation` lo exige).
- `phase.bypassed_by_external_input` — inicio desde artefacto externo (`{phase, external_artifact, sha256}`).
- `phase.transitioned` — toda transición de fase (`{phase, from, to, cause}`); el `cause`
  refleja la operación (`work_item_started`, `phase_begun`, `phase_delivered`,
  `approval_recorded`, `approval_rejected`, `phase_completed`, o `dependencies_satisfied` en
  desbloqueos).
- `approval.requested` — al entrar una fase en `awaiting_approval`.
- `approval.recorded` — decisión humana (`{phase, status, comment}`).
- `work_item.completed` — cierre del work item.
- `archive.completed` — archivado (`{from: completed, to: archived, archive_path}`).
- Eventos **custom** vía `record-event` (cualquier `type` que respete el patrón del schema).

### `correlation_id` e idempotencia

- `--operation-id` (1–128, `[A-Za-z0-9][A-Za-z0-9._:-]*`) se persiste como `correlation_id`.
- Antes de commitear, el motor escanea `events.jsonl` buscando un evento con el mismo
  `correlation_id` (`eventOperationExists` / `OperationApplied`). Si ya existe, la operación se
  considera aplicada: los use cases devuelven el estado actual sin re-ejecutar la transición, y
  el commit rechaza duplicados con `ErrOperationAlreadyApplied` (que el helper `commitWorkItem`
  resuelve devolviendo el item persistido). Así, reintentar un comando con el mismo
  `operation-id` no duplica eventos ni reaplica efectos.

### Inmutabilidad y persistencia

- `events.jsonl` es **append-only**: los eventos se agregan (`O_APPEND`) y se hace `fsync`;
  nunca se reescriben líneas existentes.
- El commit es transaccional (staging + `rename` + backup + recovery): manifest, artefactos y
  eventos se confirman como "todo o nada" (ver `fs_repository.go`).
- `validateCommit` valida cada evento contra `event.schema.json` y que `event.work_item`
  coincida con el id antes de publicar.
- Los expedientes archivados conservan su `events.jsonl` inmutable en `archive/`.

## Escenarios verificables

- `record-event x -t custom.note -m "hola"` → agrega una línea a `events.jsonl` con
  `type: custom.note`, `data.message: "hola"`, actor `agent`.
- Reintento de `record-event` con el mismo `--operation-id` → no agrega un segundo evento.
- Un `--type` que no respeta el patrón del schema → falla la validación al commitear
  (`schema validation failed` → `invalid_input`).
- El primer evento de todo work item es `work_item.created` (lo verifica `validate`).

## Reglas, contratos y restricciones

- Log append-only, una línea JSON por evento, con `fsync`.
- `type` restringido por patrón; `schema_version` const `0.1`; `actor.kind` acotado al enum.
- `correlation_id` opcional pero, si se usa, garantiza idempotencia por búsqueda en el log.
- `record-event` opera solo sobre work items **activos** (usa `GetWorkItem`); los archivados
  son inmutables.

## Errores y códigos

- Actor inválido / `type` inválido / schema → `invalid_input`.
- `ErrWorkItemNotFound` → `not_found`.
- `ErrOperationAlreadyApplied` se maneja internamente (retorna el estado existente).

## Dependencias con otras capacidades

- Todas las capacidades mutadoras (`start`, transiciones, cierre) emiten eventos por esta vía.
- `validate` audita la coherencia del log (primer evento, continuidad de transiciones,
  correspondencia con el manifest, unicidad de archivado).

## Deltas sobre baseline

No aplica (especificación reconstruida).

- **Pendiente/Desconocido**: `record-event` no expone un flag para poblar `data` arbitrario:
  el `RecordEventInput.Data` existe en el use case pero el comando solo inyecta `message`. La
  observabilidad de tokens/costos (`observability.token_usage`) está modelada en el manifest y
  el schema pero no se recolecta como eventos reales en la BETA.

## Trazabilidad

- `src/cli/cmd/record_event.go` — flags y salida.
- `src/cli/internal/usecases/record_event_uc.go` — construcción del evento e idempotencia.
- `src/cli/internal/usecases/transition_helpers.go` — `newOperationEvent`, `phaseMutationEvents`, `commitWorkItem`.
- `src/cli/internal/domain/event.go` — `Event`, `NewEvent`.
- `src/cli/internal/infra/fs_repository.go` — append transaccional, `eventOperationExists`, `validateCommit`.
- `.sdd/schemas/event.schema.json` — contrato estructural del evento.
