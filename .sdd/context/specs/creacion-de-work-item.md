---
schema_version: "0.1"
kind: reconstructed-spec
id: "creacion-de-work-item"
capability: "start"
status: reconstruida-desde-codigo
created_at: "2026-09-22"
created_by: { kind: "agent", id: "sdd-doc-subagent" }
sources:
  - src/cli/cmd/start.go
  - src/cli/internal/usecases/start_uc.go
  - src/cli/internal/domain/work_item.go
  - src/cli/internal/infra/artifact_manager.go
---

# Especificación: Creación de work item (`start`)

> Especificación reconstruida desde el código fuente durante el onboarding del motor
> `sdd-cli`. La fuente de verdad prioritaria es el código.

## Requisitos y comportamiento

### Propósito

`sdd-cli start <id>` crea una nueva **instancia de trabajo** (work item): genera el manifest
(`manifest.yaml`) a partir de un workflow, inicializa el estado de todas las fases, prepara
el/los artefacto(s) de la fase de entrada renderizando su template, y registra los eventos de
creación. Soporta empezar desde cero (input `user_prompt`) o desde un artefacto externo ya
escrito (`--from-artifact`), verificando su integridad con SHA-256.

### Entradas (comando y flags)

- Argumento: `<work-item-id>` (`cobra.ExactArgs(1)`), debe ser kebab-case.
- `--workflow` / `-w`: id de workflow; si se omite, se usa `defaults.workflow` de
  `.sdd/config.yaml` (p. ej. `feature-standard`).
- `--title` / `-t`: **requerido**, no vacío.
- `--summary` / `-s`: resumen del input.
- `--from-artifact`: ruta a un artefacto Markdown preexistente.
- `--phase`: fase de entrada a usar con `--from-artifact` (deben usarse **juntos**).
- `--actor-kind` (default `human`), `--actor-id` (default `user`).
- `--operation-id`: clave de idempotencia (1–128, `[A-Za-z0-9][A-Za-z0-9._:-]*`).

### Comportamiento observable

- **Validaciones previas**: id kebab-case, actor válido, operation-id válido, title no vacío,
  y XOR estricto entre `--from-artifact` y `--phase` (si solo uno está presente →
  `ErrInvalidExternalArtifact`).
- **Idempotencia**: si el work item ya existe y se pasó `--operation-id` ya aplicado
  (buscado en `events.jsonl` por `correlation_id`), devuelve el item existente; si existe y
  no coincide → `ErrWorkItemAlreadyExists`.
- **Selección de workflow**: `--workflow` o el default de config; se carga y valida el
  workflow (schema + semántica + templates).
- **Inicio normal (`user_prompt`)**:
  - `entryPhase = workflow.EntryPhaseFor("user_prompt")` (debe ser único; si hay varias o
    ninguna → `ErrInvalidEntryPoint`).
  - `NewWorkItem`: todas las fases quedan `blocked`, la fase de entrada pasa a `ready` y luego
    a `in_progress` automáticamente (`BeginPhase`); **no** requiere un `begin` manual.
  - `input.source = user_prompt`.
- **Inicio desde artefacto externo (`--from-artifact` + `--phase`)**:
  - `entryPhase = --phase`; `externalArtifactID = workflow.ExternalArtifactForEntry(phase)`
    (la fase de entrada debe aceptar exactamente un artefacto producido; si no →
    `ErrInvalidEntryPoint`).
  - `ResolveExternalArtifact`: resuelve la ruta a **absoluta**, exige archivo regular, lee su
    contenido y calcula el **SHA-256** (`fmt.Sprintf("%x", sha256)`).
  - `NewWorkItem` con `ExternalArtifact`: `input.source = external_artifact`; las fases
    **ancestro** de la fase de entrada pasan a `not_applicable`; la fase de entrada pasa a
    `accepted` (o `awaiting_approval` si su gate es `required`, agregando un `Approval` pending).
  - `ExternalArtifactReference` en el manifest: `{ artifact, path (absoluto), sha256 }`.
- **Preparación de artefactos** (`ArtifactManager.PrepareArtifactsForPhase`): por cada
  artefacto que produce la fase de entrada, lee `.sdd/templates/<template>.md`, lo renderiza
  con variables (`title`, `id`, `created_at`, `type`, `artifact_id`, `phase`,
  `created_by_kind=cli`, `created_by_id=sdd`, `sources`), verifica que no queden placeholders
  `{{`, valida el front-matter contra `artifact.schema.json` y la identidad
  (`id`/`phase`/`work_item`).
- **Import externo** (`ImportExternalArtifact`): reemplaza el cuerpo del artefacto canónico
  por el contenido externo, conservando el front-matter válido generado (descarta el
  front-matter original del archivo importado).
- **Eventos emitidos**:
  - `work_item.created` (`{workflow, title}`).
  - Si externo: `phase.bypassed_by_external_input` (`{phase, external_artifact, sha256}`).
  - `phase.transitioned` por cada transición de la mutación inicial (cause `work_item_started`;
    para desbloqueos, cause `dependencies_satisfied`).
- **Persistencia**: commit atómico y transaccional (manifest revisión 1, artefactos,
  eventos) — ver spec de trazabilidad y de transiciones para el mecanismo.
- **Salida**:
  - Texto: `Work item '<id>' successfully started with workflow '<wf>'.`
  - JSON: el objeto `WorkItem` completo (manifest) en `data`.

## Escenarios verificables

- `start x -t "T"` sin `--workflow` → usa el workflow default; fase de entrada `in_progress`;
  evento inicial `work_item.created`.
- `start x -t "T" --from-artifact ruta.md --phase specification` en `feature-standard` →
  `prd` queda `not_applicable`, `specification` queda `awaiting_approval` (gate required),
  se registra `phase.bypassed_by_external_input` con el SHA-256.
- `start` con id existente + mismo `--operation-id` → devuelve el item, sin duplicar.
- `--from-artifact` sin `--phase` (o viceversa) → error `invalid_external_artifact`.

## Reglas, contratos y restricciones

- `id` kebab-case; `title` obligatorio; XOR de `--from-artifact`/`--phase`.
- Entry point no ambiguo (`user_prompt`) o entrada externa que acepte exactamente un artefacto.
- `not_applicable` solo en ancestros del entry phase.
- SHA-256 del artefacto externo persistido y verificado luego por `validate`.
- Manifest inicial: `schema_version 0.1`, `kind work-item`, `status active`, `revision 1`,
  `traceability.events = events.jsonl`.

## Errores y códigos

- `ErrInvalidExternalArtifact` / `ErrInvalidEntryPoint` / `ErrInvalidWorkItem` /
  `ErrInvalidIdentifier` → `invalid_input`.
- `ErrWorkItemAlreadyExists` → `already_exists`.
- Workflow inexistente/ inválido → `not_found` / `invalid_input`.

## Dependencias con otras capacidades

- Depende del contrato materializado por `init` (workflows, templates, schemas, config).
- Alimenta a `begin/deliver/approve/reject`, `status/next`, `validate` y `record-event`.

## Deltas sobre baseline

No aplica (especificación reconstruida).

- **Pendiente/Desconocido**: el CLI no expone un flag para pasar `references` del input
  (`WorkItemInput.References` existe en el dominio pero `start` no lo puebla). El origen
  `imported_artifact` está modelado como valor válido de `input.source` en el dominio/schema,
  pero `start` solo produce `user_prompt` o `external_artifact` (no se detectó flujo que
  genere `imported_artifact`).

## Trazabilidad

- `src/cli/cmd/start.go` — flags y salida.
- `src/cli/internal/usecases/start_uc.go` — orquestación (validaciones, workflow, eventos, commit).
- `src/cli/internal/domain/work_item.go` — `NewWorkItem`, `BeginPhase`, `AcceptExternalPhase`.
- `src/cli/internal/infra/artifact_manager.go` — preparación e import de artefactos, SHA-256.
- `src/cli/internal/domain/workflow_validation.go` — `EntryPhaseFor`, `ExternalArtifactForEntry`, `Ancestors`.
