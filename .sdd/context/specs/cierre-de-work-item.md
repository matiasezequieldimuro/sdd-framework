---
schema_version: "0.1"
kind: reconstructed-spec
id: "cierre-de-work-item"
capability: "complete / archive"
status: reconstruida-desde-codigo
created_at: "2026-09-22"
created_by: { kind: "agent", id: "sdd-doc-subagent" }
sources:
  - src/cli/cmd/complete.go
  - src/cli/cmd/archive.go
  - src/cli/internal/usecases/complete_uc.go
  - src/cli/internal/usecases/archive_uc.go
  - src/cli/internal/infra/fs_archive_repository.go
  - src/cli/internal/domain/work_item.go
---

# Especificación: Cierre de work item (`complete` / `archive`)

> Especificación reconstruida desde el código fuente durante el onboarding del motor
> `sdd-cli`. La fuente de verdad prioritaria es el código.

## Requisitos y comportamiento

### Propósito

`complete` cierra una fase aprobada/aceptada o el work item completo; `archive` mueve el
expediente ya completado a un directorio inmutable `archive/YYYY-MM-DD-<id>/`, tras superar un
gate de validación. Juntos cierran el ciclo de vida iniciado por `start`.

### `complete`

- Flags: `--phase`/`-p` (opcional), `--actor-kind` (default `cli`), `--actor-id`
  (default `sdd`), `--operation-id`. Argumento `<id>` obligatorio.
- **`complete --phase <fase>`** (`CompletePhase`): la fase debe estar `approved` o `accepted`;
  pasa a `completed`. Es **opcional** (ambos estados ya satisfacen dependencias), pero
  normaliza el estado y desbloquea/prepara los templates de las fases dependientes. Evento
  `phase.transitioned` (cause `phase_completed`).
- **`complete` (sin `--phase`)** (`WorkItem.Complete`): cierra el work item.
  - El item debe estar `active` (si no → `ErrWorkItemCannotComplete`).
  - Cada fase **obligatoria** debe satisfacer completitud
    (`approved`/`completed`/`accepted`/`not_applicable`); si no → error indicando la fase.
  - Cada fase **opcional** puede quedar sin iniciar (`blocked`/`ready`/`not_applicable`); si
    fue iniciada, debe estar satisfecha.
  - Pasa el item a `completed`. Evento `work_item.completed`.
- Salida: JSON con el `WorkItem`; texto `Work item '<id>' completed.` o
  `Phase '<fase>' completed for work item '<id>'.`

### `archive`

- Flags: `--actor-kind` (default `cli`), `--actor-id` (default `sdd`), `--operation-id`.
  Argumento `<id>` obligatorio.
- **Precondiciones de elegibilidad** (`WorkItem.Archive` / `archiveEligibility`):
  - Item en estado `completed`.
  - Todas las fases no-`archive` satisfechas (las opcionales pueden quedar sin iniciar).
  - Si el workflow declara una fase `archive`, esta debe estar satisfecha.
- **Gate de validación previo**: se ejecuta `InspectWorkItem` (misma lógica que `validate <id>`);
  si el reporte tiene fallos → `ValidationFailure` (se imprime el reporte y sale con error
  `validation_failed`). Esto exige, entre otras cosas, que `artifacts/archive.md` (u otro
  artefacto de la fase `archive`) sea válido.
- **Idempotencia / ya archivado**: si ya existe en `archive/` y el `--operation-id` fue
  aplicado → devuelve el resultado archivado; si existe sin coincidir → `ErrWorkItemAlreadyArchived`.
- **Destino**: `.sdd/work-items/archive/<YYYY-MM-DD>-<id>/` (fecha UTC del momento de archivar).
- **Evento**: `archive.completed` (`{from: completed, to: archived, archive_path}`).
- **Movimiento transaccional** (`ArchiveWorkItem`): lock por work item + revisión optimista;
  staging con copia del expediente, append del evento y manifest con `revision+1`; validación
  del stage (incluye el artefacto de `archive`); publicación mediante `rename` de active→backup
  y stage→destino, con marker de transacción, hooks de commit, rollback y recovery ante
  interrupción (`recoverArchiveTransactionLocked`). El manifest archivado queda con
  `status: archived`.
- Salida: JSON `{ work_item, location: "archive", archive_path }`; texto
  `Work item '<id>' archived at '<path>'.`

## Escenarios verificables

- `complete` con todas las fases obligatorias satisfechas → item `completed`, evento
  `work_item.completed`.
- `complete` con una fase obligatoria pendiente → `ErrWorkItemCannotComplete`.
- `archive` de un item `completed` con validación limpia y `archive.md` válido → expediente
  en `archive/YYYY-MM-DD-<id>/`, evento `archive.completed`.
- `archive` de un item con `validate` en fallo → error `validation_failed`, sin mover nada.
- `archive` reintentado con el mismo `--operation-id` → idempotente.
- `archive` de un item no `completed` → `ErrWorkItemCannotArchive`.

## Reglas, contratos y restricciones

- El archivado es **inmutable**: no hay comando de reapertura; el destino no se sobrescribe.
- Unicidad de ubicación: un work item no puede existir a la vez en `active/` y `archive/`
  (`ErrArchiveConflict`).
- El directorio de archivo respeta el patrón `YYYY-MM-DD-<id>` (validado al leer/parsear).
- Escritura atómica con backup/recovery; concurrencia por lock (`flock`) + revisión.
- Política de archivo: `config.yaml` `archive_policy` (p. ej. `optional`); en `feature-standard`
  la fase `archive` es `optional`.

## Errores y códigos (envelope JSON)

- `ErrWorkItemCannotComplete`, `ErrWorkItemCannotArchive` → `invalid_transition`.
- `ErrWorkItemAlreadyArchived` → `already_archived`.
- `ErrArchiveConflict` → `archive_conflict`.
- `ValidationFailure` (`ErrValidationFailed`) → `validation_failed` (con el reporte en `details`).
- `ErrConcurrentModification` → `concurrent_modification`; `ErrWorkItemLocked` → `work_item_locked`.

## Dependencias con otras capacidades

- Requiere fases resueltas por `begin/deliver/approve/reject` (o `complete --phase`).
- `archive` reutiliza el inspector de `validate` como gate.
- El artefacto de la fase `archive` (`artifacts/archive.md`) lo produce el rol/agente antes de archivar.

## Deltas sobre baseline

No aplica (especificación reconstruida).

- **Pendiente/Desconocido**: `cancelled` está modelado como `WorkItemStatus` pero no hay
  comando público que lo produzca (fuera de alcance BETA).

## Trazabilidad

- `src/cli/cmd/{complete,archive}.go` — flags, defaults de actor, manejo de `ValidationFailure`.
- `src/cli/internal/usecases/{complete_uc,archive_uc}.go` — orquestación y gate de validación.
- `src/cli/internal/infra/fs_archive_repository.go` — movimiento transaccional, recovery, validación del stage.
- `src/cli/internal/domain/work_item.go` — `Complete`, `CompletePhase`, `Archive`, `archiveEligibility`.
