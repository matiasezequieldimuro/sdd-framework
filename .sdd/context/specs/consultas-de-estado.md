---
schema_version: "0.1"
kind: reconstructed-spec
id: "consultas-de-estado"
capability: "status / next"
status: reconstruida-desde-codigo
created_at: "2026-09-22"
created_by: { kind: "agent", id: "sdd-doc-subagent" }
sources:
  - src/cli/cmd/status.go
  - src/cli/cmd/next.go
  - src/cli/internal/usecases/status_uc.go
  - src/cli/internal/usecases/next_uc.go
  - src/cli/internal/domain/work_item.go
---

# Especificación: Consultas de estado (`status` / `next`)

> Especificación reconstruida desde el código fuente durante el onboarding del motor
> `sdd-cli`. La fuente de verdad prioritaria es el código.

## Requisitos y comportamiento

### Propósito

`status` y `next` son **consultas puras** (no mutan estado, no crean locks ni eventos).
`status` devuelve el estado completo del work item y sus fases en orden topológico; `next`
devuelve la **siguiente acción válida** (fase activa prioritaria y su procedure).

### `status <work-item-id>`

- Argumento obligatorio; sin flags propios (usa `--dir` / `--json`).
- `FindWorkItem` localiza el item en `active/` **o** `archive/` (`WorkItemCatalogReader`),
  carga el workflow y ordena las fases topológicamente (`OrderedPhases`).
- Devuelve `StatusResult`: embebe el `WorkItem` (manifest completo) + `location`
  (`active`/`archive`) + `archive_path` (solo si está archivado) + `OrderedPhases`
  (lista `{id, status, artifact}`).
- **Salida**:
  - Texto: cabecera con `Work Item: <id> [<status>]`, `Title`, `Workflow`, `Location`,
    `Archive path` (si aplica), y una tabla `PHASE / STATUS / ARTIFACT` en orden topológico.
  - JSON: el `WorkItem` completo + `location` + `archive_path`. Nota: `OrderedPhases` está
    marcado `json:"-"` / `yaml:"-"`, por lo que **no** aparece en JSON; los consumidores leen
    el mapa `phases` del manifest (sin orden garantizado en JSON).

### `next <work-item-id>`

- Argumento obligatorio; sin flags propios.
- `GetWorkItem` lee el item **activo** (`WorkItemReader.GetWorkItem`, que resuelve la ruta
  `active/`), carga el workflow y calcula `NextPhase`.
- **Prioridad de selección** (`WorkItem.NextPhase`), recorriendo fases en orden topológico:
  `awaiting_approval` > `in_progress` > `ready`. Si no hay ninguna → acción vacía con mensaje
  `No active phases pending. Work item may be completed or archived.`
- Devuelve `NextAction`: `{ phase_id, status, procedure, artifact, needs_approval, optional,
  message }`. `needs_approval` es true si la política de la fase es `required` o si está
  `awaiting_approval`. El `message` cambia cuando la fase está `awaiting_approval`
  (indica que espera aprobación humana).
- **Salida**:
  - Texto: el `message`; si hay fase, además `Phase: <id> (Status: <status>)`,
    `Procedure: <procedure>`, `Artifact: <artifact>`.
  - JSON: el objeto `NextAction` completo.

## Escenarios verificables

- `status` de un item activo → tabla de fases en orden topológico con sus estados/artefactos.
- `status` de un item archivado → `location: archive` y `archive_path` poblado.
- `next` cuando una fase está `awaiting_approval` → `needs_approval: true` y mensaje de espera
  de aprobación humana.
- `next` sin fases activas → mensaje "No active phases pending…" y `phase_id` vacío.
- Ni `status` ni `next` crean locks, eventos ni modifican el manifest.

## Reglas, contratos y restricciones

- Consultas puras: sin efectos secundarios.
- `status` ve activos y archivados; `next` solo opera sobre el expediente **activo** (usa
  `GetWorkItem`, no el catalog reader), por lo que sobre un item archivado devolvería
  `not_found`.
- El orden de fases en `status` (texto) es topológico y estable.

## Errores y códigos

- `ErrWorkItemNotFound` → `not_found`.
- `ErrInvalidWorkflow` / ciclo en el grafo → `invalid_input` (al ordenar fases).

## Dependencias con otras capacidades

- Consumen el manifest producido por `start` y actualizado por las transiciones.
- `next.procedure` referencia procedures del contrato (`.sdd/procedures/`), que un agente usa
  para saber qué hacer a continuación.

## Deltas sobre baseline

No aplica (especificación reconstruida).

- **Desconocido/Nota**: la ausencia de `OrderedPhases` en la salida JSON es intencional
  (marcado `json:"-"`); los consumidores JSON deben derivar el orden ellos mismos desde el
  workflow o aceptar el mapa `phases` sin orden. No se detectó una salida JSON con las fases
  ya ordenadas.

## Trazabilidad

- `src/cli/cmd/status.go` — render de texto y tabla de fases.
- `src/cli/cmd/next.go` — render de la siguiente acción.
- `src/cli/internal/usecases/status_uc.go` — `StatusResult`, `OrderedPhases`, `location`/`archive_path`.
- `src/cli/internal/usecases/next_uc.go` — `NextAction` y mensajes.
- `src/cli/internal/domain/work_item.go` — `NextPhase` (prioridades).
