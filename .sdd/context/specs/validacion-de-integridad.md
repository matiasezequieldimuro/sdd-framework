---
schema_version: "0.1"
kind: reconstructed-spec
id: "validacion-de-integridad"
capability: "validate"
status: reconstruida-desde-codigo
created_at: "2026-09-22"
created_by: { kind: "agent", id: "sdd-doc-subagent" }
sources:
  - src/cli/cmd/validate.go
  - src/cli/internal/usecases/validate_uc.go
  - src/cli/internal/domain/workflow_validation.go
  - src/cli/internal/domain/work_item_validation.go
  - src/cli/internal/domain/diagnostic.go
  - src/cli/internal/domain/validation.go
  - src/cli/internal/infra/schema_validator.go
  - src/cli/internal/infra/fs_validation_inspector.go
---

# Especificación: Validación de integridad (`validate`)

> Especificación reconstruida desde el código fuente durante el onboarding del motor
> `sdd-cli`. La fuente de verdad prioritaria es el código.

## Requisitos y comportamiento

### Propósito

`sdd-cli validate [work-item-id]` es el **chequeo de salud** del contrato y de los work items.
Combina validación **estructural** (JSON Schema) con validación **semántica** (workflow como
DAG y coherencia work-item↔workflow). Es una consulta pura: no muta estado, no crea locks ni
eventos, y puede repetirse libremente.

### Entradas y alcance

- Sin argumento → **scope `project`**: `InspectProject`.
- Con `<work-item-id>` → **scope `work_item`**: `InspectWorkItem` (id validado kebab-case).
- `cobra.MaximumNArgs(1)`. Flags globales `--dir` / `--json`.

### Salida y exit code

- Devuelve un `ValidationReport`: `{ scope, target, valid, summary { total, passed, warnings,
  failed }, checks[] }`. `valid = (failed == 0)`. Los checks se ordenan de forma estable por
  `target`, `category`, `code`, `status`, `message`.
- Si `valid` es false → se devuelve un `validationReportError` (envuelve `ValidationFailure`),
  que imprime el reporte y sale con **exit code 1**; en `--json` el `code` es `validation_failed`
  y `details` contiene el reporte completo. Las **warnings no** cambian el exit code.
- Cada `ValidationCheck`: `{ status (passed|warning|failed), category, code, target, message }`.

### Categorías (`category`)

`project`, `config`, `registry`, `schema`, `workflow`, `template`, `procedure`, `manifest`,
`work_item`, `artifact`, `event`, `reference`.

### Validación estructural (JSON Schema)

`SchemaValidator` compila con `AssertFormat: true` y valida contra los schemas de
`.sdd/schemas/`: `artifact.schema.json`, `event.schema.json`, `work-item.schema.json`,
`workflow.schema.json`. Comprueba que existan y **compilen** (`schema.file_exists`,
`schema.compiles`), y valida cada documento (manifest, eventos, workflows, front-matter de
artefactos y de templates renderizados) reportando cada violación como `*.schema_valid`.

### Validación semántica del workflow (DAG)

`Workflow.SemanticViolations` emite `ContractViolation` con códigos estables, entre ellos:
`workflow.identifier_invalid`, `workflow.phases_missing`, `workflow.entry_points_missing`,
`workflow.phase_identifier_invalid`, `workflow.phase_duplicate`,
`workflow.approval_policy_invalid`, `workflow.phase_artifact_count_invalid` (exactamente 1
artefacto por fase en v0.1), `workflow.artifact_identifier_invalid`,
`workflow.template_identifier_invalid`, `workflow.artifact_path_invalid`,
`workflow.artifact_path_duplicate`, `workflow.phase_self_dependency`,
`workflow.phase_dependency_unknown`, `workflow.produced_artifact_unknown`,
`workflow.artifact_producer_duplicate`, `workflow.artifact_producer_missing`,
`workflow.entry_phase_unknown`, `workflow.entry_phase_duplicate`,
`workflow.entry_input_ambiguous`, `workflow.entry_input_unknown`, `workflow.graph_cycle`
(orden topológico sin ciclos) y `workflow.phase_unreachable` (alcanzabilidad desde algún
entry point). Además se verifica `workflow.filename_matches_id` y la presencia/renderizado de
templates y procedures (`template.file_exists`, `template.placeholders_resolved`,
`template.front_matter_valid`, `template.identity_valid`, `procedure.file_exists`).

### Coherencia work-item↔workflow

`WorkItem.ViolationsAgainst` emite códigos como: `work_item.workflow_mismatch`,
`work_item.workflow_version_mismatch`, `work_item.type_mismatch`,
`work_item.entry_point_invalid`, `work_item.external_artifact_missing` /
`_mismatch` / `_unexpected`, `work_item.phase_count_mismatch`, `work_item.phase_state_missing`,
`work_item.artifact_path_mismatch`, `work_item.not_applicable_invalid`,
`work_item.phase_blocked_invalid`, `work_item.dependencies_unsatisfied`,
`work_item.approval_state_invalid`, `work_item.accepted_state_invalid`,
`work_item.phase_unknown`, `work_item.completion_invalid`, `work_item.archive_invalid`.
Las aprobaciones se validan con `approval.*`: `phase_unknown`, `policy_invalid`,
`pending_metadata_invalid`, `actor_invalid` (decisión requiere actor humano),
`timestamp_invalid`, `pending_duplicate` (≤1 pending por fase), `latest_mismatch`
(coherencia estado de fase ↔ última aprobación).

### Validación de eventos y referencias

- Eventos (`inspectEvents`): `event.file_exists`, `event.line_json_valid`, `event.schema_valid`,
  `event.first_is_creation` (el primer evento debe ser `work_item.created`),
  `event.work_item_matches`, `event.id_unique`, `event.correlation_id_valid`,
  `event.transition_payload_valid`, `event.transition_continuity_valid`,
  `event.approval_payload_valid`, `event.lifecycle_continuity_valid`,
  `event.archive_payload_valid`, `event.transition_matches_manifest`, `event.archive_unique`,
  `event.lifecycle_matches_manifest`.
- Referencias (`inspectReferences`): artefacto externo verificado por SHA-256
  (`reference.external_hash_matches`), disponibilidad (`reference.external_available` —
  **warning** si el archivo no está en esta máquina), `reference.external_regular`,
  `reference.related_work_item_id_valid`, `reference.baseline_spec_exists` (specs bajo `specs/`).
- Nivel proyecto: existencia de directorios/archivos requeridos, `config.yaml`
  (`config.yaml_valid`, `config.schema_version_supported`, `config.default_workflow_valid`,
  `config.default_workflow_exists`), registry de capabilities (`registry.*`), y unicidad de
  ubicación de cada work item entre `active/` y `archive/` (`work_item.location_unique`,
  `archive.entry_valid`, `archive.id_unique`).

## Escenarios verificables

- Workflow con ciclo en `requires` → check `workflow.graph_cycle` en `failed`, exit 1.
- Manifest cuyo `revision`/estados violan el schema → `manifest.schema_valid` en `failed`.
- Artefacto externo cuyo hash ya no coincide → `reference.external_hash_matches` en `failed`.
- Artefacto externo ausente en la máquina → `reference.external_available` en `warning`
  (no cambia el exit code).
- Proyecto sano → reporte `valid: true`, exit 0.

## Reglas, contratos y restricciones

- Estructural + semántica: un documento puede pasar el schema y fallar la semántica del DAG.
- `AssertFormat: true`: se validan formatos (p. ej. `date-time`).
- Consulta pura y determinista; salida ordenada de forma estable.
- Warnings ≠ fallos: solo `failed > 0` marca `valid: false` / exit 1.

## Dependencias con otras capacidades

- `archive` reutiliza `InspectWorkItem` como gate previo (ver spec de cierre).
- La lectura de work items (`GetWorkItem`) valida también contra el schema y la semántica al
  cargar (`readWorkItemAt`), por lo que la validación de integridad es transversal.

## Deltas sobre baseline

No aplica (especificación reconstruida).

- **Pendiente/Desconocido**: no se enumeraron exhaustivamente los checks a nivel `project`
  (existen más `project.*`); el detalle completo vive en `fs_validation_inspector.go`. Las
  reglas del schema `workflow.schema.json`/`work-item.schema.json` no se transcribieron aquí.

## Trazabilidad

- `src/cli/cmd/validate.go` — comando, `validationReportError`, impresión del reporte.
- `src/cli/internal/usecases/validate_uc.go` — `ValidationReport`, `ValidationFailure`, `buildValidationReport`.
- `src/cli/internal/domain/{workflow_validation,work_item_validation,diagnostic,validation}.go` — reglas y códigos.
- `src/cli/internal/infra/schema_validator.go` — compilación/validación JSON Schema.
- `src/cli/internal/infra/fs_validation_inspector.go` — inspección de proyecto y work item.
