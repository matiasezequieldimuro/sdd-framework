# Modelado de datos

## Resumen

<!-- Describir cómo se representan y persisten los datos principales. -->

El motor **no usa base de datos**: todo el estado se representa como archivos sobre el
filesystem dentro de `.sdd/`. Las estructuras de dominio se definen en Go
(`src/cli/internal/domain/`) y se validan contra JSON Schemas en `.sdd/schemas/`. Hay dos
grandes familias de datos:

1. **Contrato (datos declarativos, versionados con el repo)**: `Workflow`, `Config`,
   templates, procedures y `capabilities`. Definen *cómo* es el proceso.
2. **Estado de ejecución (datos por instancia de trabajo)**: `WorkItem` (manifest),
   `Artifact` (documentos Markdown con front-matter) y `Event` (log append-only). Definen
   *en qué punto* está cada trabajo.

Cada work item concentra su verdad en tres archivos con roles distintos:
`manifest.yaml` (estado actual), `artifacts/*.md` (evidencia) y `events.jsonl` (historial
inmutable).

## Entidades y estructuras

| Entidad o estructura | Propósito | Fuente de verdad | Observaciones |
| --- | --- | --- | --- |
| `WorkItem` (manifest) | Instancia concreta de un workflow: estado de cada fase, aprobaciones, trazabilidad | `.sdd/work-items/<loc>/<id>/manifest.yaml` | `schema_version: "1.0"`, `kind: work-item`; validado por `work-item.schema.json`; `id` kebab-case; `revision` ≥ 1 |
| `WorkItemWorkflow` | Referencia embebida al workflow que rige el item | dentro del manifest | `id`, `version`, `entry_phase` |
| `WorkItemInput` | Origen del trabajo | dentro del manifest | `source`: `user_prompt` \| `external_artifact` \| `imported_artifact`; `summary`; `external_artifact` opcional |
| `ExternalArtifactReference` | Metadatos del artefacto externo importado | dentro del manifest | `artifact`, `path`, `sha256` (64 hex) |
| `PhaseState` | Estado de una fase del item | dentro del manifest (`phases` map) | `status` (10 valores) + `artifact` (ruta `artifacts/*.md`) |
| `Approval` | Registro de aprobación/rechazo de una fase con gate | dentro del manifest (`approvals`) | `phase`, `status` (`pending`/`approved`/`rejected`/`superseded`), `by` (Actor), `at`, `comment`; ningún comando produce hoy `superseded` (`rework` conserva los registros tal cual) |
| `Traceability` | Enlaces del item con eventos y otros items | dentro del manifest | `events: events.jsonl`, `related_work_items`, `baseline_specs` |
| `Observability` / `TokenUsage` | Consumo de tokens (modelado, opcional) | dentro del manifest | `token_usage.status`: `not_reported`/`partial`/`recorded` + contadores |
| `Workflow` | Plantilla declarativa del proceso (fases, dependencias, gates, artefactos) | `.sdd/workflows/<id>.workflow.yaml` | `kind: workflow`; validado por `workflow.schema.json` + reglas semánticas (DAG) |
| `WorkflowPhase` | Una fase del workflow | dentro del workflow | `id`, `requires[]`, `produces[]` (exactamente 1 en v1.0), `procedure`, `approval` (`none`/`required`/`optional`), `optional`, `effects[]` |
| `EntryPoint` | Fase desde la que se puede empezar y qué inputs acepta | dentro del workflow | `phase`, `accepts[]` (`user_prompt` o un artefacto producido) |
| `WorkflowArtifact` | Mapea un artefacto lógico a su ruta y template | dentro del workflow (`artifacts` map) | `path` (`artifacts/*.md`), `template` |
| `Event` | Registro inmutable de algo que pasó | `.sdd/work-items/<loc>/<id>/events.jsonl` | `kind` implícito; validado por `event.schema.json`; `type` en `snake.dotted-form`; `correlation_id` = operation-id |
| `Actor` | Quién realizó una acción | embebido en Event/Approval/WorkItem | `kind`: `human`/`agent`/`cli`/`system`, `id` no vacío |
| `Config` | Config del proyecto SDD | `.sdd/config.yaml` | `defaults.workflow` (fallback de `start`), más `artifact_language`, `archive_policy`, `interaction.mode`, `observability` |
| `Artifact` (metadata) | Front-matter YAML de cada documento de fase | encabezado de `artifacts/*.md` | validado por `artifact.schema.json`; `status`: `draft`/`delivered`/`approved`/`rejected`/`superseded`; `sources[]` |
| `Capabilities` (registry) | Procedimientos portables (skills) por capability | `.sdd/registry/capabilities.yaml` | `id` (`sdd.*`, 15 en v1.0.0, incluida `sdd.verify-ui`), `title`, `procedure` (opcional: `sdd.status` no tiene), `inputs`, `outputs` |
| `ValidationCheck` / `ContractViolation` | Resultado de diagnóstico | en memoria (salida de `validate`) | `status` (`passed`/`warning`/`failed`), `category`, `code`, `target`, `message` |

Estados posibles de una fase (`PhaseStatus`): `not_applicable`, `blocked`, `ready`,
`in_progress`, `awaiting_approval`, `approved`, `completed`, `rejected`, `accepted`,
`superseded` (lo produce `rework` en las fases descendientes). Estados de work item
(`WorkItemStatus`): `active`, `completed`, `archived` (lo produce `archive`) y `cancelled`
(lo produce `cancel`; terminal e irreversible).

## Relaciones

<!-- Relaciones, cardinalidades y reglas relevantes. -->

- `Workflow` **1 — N** `WorkflowPhase`; cada fase produce **exactamente 1** artefacto en
  el contrato v1.0 (`workflow.produced_artifact` / regla `phase_artifact_count_invalid`).
- `Workflow` **1 — N** `WorkflowArtifact`; cada artefacto tiene ruta única y es producido
  por exactamente una fase (`artifact_producer_missing`/`_duplicate`).
- Dependencias entre fases (`requires`) forman un **DAG** (grafo dirigido acíclico); se
  ordena topológicamente y toda fase debe ser alcanzable desde algún entry point.
- `WorkItem` **1 — 1** `Workflow` (por `workflow.id` + `version`); el manifest debe tener
  una `PhaseState` por cada fase del workflow (cardinalidad exacta, `phase_count_mismatch`).
- `WorkItem` **1 — N** `Approval` (histórico; puede haber varias iteraciones por fase, con
  a lo sumo una `pending` por fase).
- `WorkItem` **1 — N** `Event` (append-only en `events.jsonl`); cada evento referencia el
  `work_item` por id.
- `WorkItem` **1 — N** `Artifact` (uno por fase que produce, más `evidence/` para el
  verificador; en este repo `evidence/` está ignorada por Git vía `.gitignore`
  `.sdd/work-items/*/*/evidence/`). El `artifact.sources[]` enlaza cada documento con los
  artefactos de sus fases predecesoras.
- `EntryPoint.accepts` referencia `user_prompt` o un artefacto producido por esa fase
  (regla `entry_input_unknown`/`entry_input_ambiguous`).

## Persistencia

- Almacenamientos: filesystem del proyecto, bajo `.sdd/`.
  - Contrato: `.sdd/{workflows,schemas,templates,procedures,registry}/`, `.sdd/config.yaml`.
  - Estado: `.sdd/work-items/active/<id>/`; tras archivar,
    `.sdd/work-items/archive/YYYY-MM-DD-<id>/`; tras cancelar,
    `.sdd/work-items/cancelled/<id>/` (sin prefijo de fecha; el directorio se crea de forma
    lazy en el primer `cancel`). Un ID vive en una sola de las tres ubicaciones.
  - Internos del motor: `.sdd/work-items/.locks/<id>.lock` y
    `.sdd/work-items/.transactions/` (staging `<id>-*`, `<id>.backup`, `<id>.failed`; para
    archive/cancel, `<id>.{archive,cancel}-stage`, `-backup`, `-failed` y el marker
    `<id>.{archive,cancel}.json`). `init` crea `.sdd/work-items/.gitignore` que excluye
    `.locks/` y `.transactions/` de Git.
- Formatos y esquemas:
  - `manifest.yaml` — YAML (serializado con `yaml.v3`), validado contra
    `work-item.schema.json`.
  - `events.jsonl` — JSON Lines (un evento por línea), validado contra `event.schema.json`.
  - `artifacts/*.md` — Markdown con front-matter YAML validado contra
    `artifact.schema.json`.
  - Workflows — YAML validado contra `workflow.schema.json` + reglas semánticas del
    dominio.
- Migraciones o versionado: versionado por **`schema_version: "1.0"`** en work items,
  workflows, eventos, artefactos, config, registry y markers de transacciones;
  `WorkItem.Workflow.Version` debe coincidir con el `schema_version` del workflow
  (`workflow_version_mismatch`). La fuente única en Go es `domain.ContractSchemaVersion`
  (`internal/domain/contract_version.go`). Corte duro: el motor escribe y exige `"1.0"` y
  rechaza `"0.1"` con un error explícito mediante `domain.ValidateContractSchemaVersion`.
  Los guards no dependen de los schemas del proyecto: lectura de manifest y workflow
  (`requireContractSchemaVersion`), metadata de artefactos (`validateArtifactMetadata`),
  markers de archive/cancel y las checks de `validate` `*.schema_version_supported`.
  Única excepción temporal: `event.schema.json` acepta `enum ["0.1","1.0"]` para que los
  `events.jsonl` históricos sigan inmutables; los eventos nuevos se escriben en `"1.0"`.
  `Revision` (entero monótono ≥ 1) sirve al control de concurrencia optimista, no al
  versionado de schema.
- Retención y eliminación: los work items no se borran; se **archivan** moviéndolos a
  `archive/YYYY-MM-DD-<id>/`, donde quedan inmutables, o se **cancelan** moviéndolos a
  `cancelled/<id>/` (opacos para `status`/`next`/`validate`; su ID queda reservado).
  `events.jsonl` es append-only (nunca se reescribe). Git conserva el histórico.

## Validación e integridad

<!-- Restricciones, invariantes y validaciones conocidas. -->

- **Validación estructural (JSON Schema)**: `SchemaValidator` compila con
  `AssertFormat: true` y valida manifests, eventos, workflows y front-matter de artefactos
  contra los schemas de `.sdd/schemas/`.
- **Validación semántica del workflow** (`domain/workflow_validation.go`): identificadores
  kebab-case, sin fases duplicadas, política de aprobación válida, exactamente 1 artefacto
  por fase, artefactos con path único y productor único, dependencias existentes y sin
  auto-dependencia, entry points no ambiguos, **DAG sin ciclos** (orden topológico) y todas
  las fases alcanzables desde algún entry point.
- **Validación del work item contra su workflow** (`domain/work_item_validation.go`):
  coincidencia de workflow/tipo/versión, entry point declarado, coherencia de
  `external_artifact` con el source, una `PhaseState` por fase, rutas de artefacto
  correctas, `not_applicable` solo en ancestros del entry phase, dependencias satisfechas
  antes de avanzar, estados de aprobación coherentes con la política y con el histórico de
  `approvals` (a lo sumo una `pending` por fase, decisión con actor humano y timestamp
  RFC3339 válido).
- **Invariantes de la máquina de estados**: ninguna fase se saltea; transiciones válidas
  solo desde estados permitidos; `approve`/`reject` exigen actor `human` y fase
  `awaiting_approval`; un item solo `complete` cuando todas las fases obligatorias están
  satisfechas; `archive` solo sobre item `completed` con la fase `archive` (si existe)
  satisfecha; `rework` solo sobre una fase `completed`/`approved`/`accepted` de un item
  `active` o `completed` (lo reactiva); `cancel` solo sobre un item `active`.
- **Integridad transaccional y de paths**: commit atómico (staging+rename+backup),
  concurrencia por lock + revisión optimista, idempotencia por `operation-id`, y rutas
  contenidas dentro de `.sdd/` (sin `..`, sin symlinks que escapen, artefactos solo bajo
  `artifacts/` y `.md`). Los artefactos externos verifican integridad por SHA-256.

## Datos sensibles

<!-- Clasificación, protección y acceso. Indicar "No identificado" si aplica. -->

No identificado. El motor no gestiona credenciales ni datos personales; opera sobre
documentos de proceso del propio proyecto. El adapter de Claude Code incluye un hook
`protect-secrets.py` y mantiene `settings.local.json` fuera de Git como salvaguarda, pero
el motor en sí no almacena secretos.

## Pendientes y fuentes

- Pendientes o supuestos:
  - `ApprovalStatus: superseded` está modelado pero ningún flujo lo produce (`rework` no
    modifica los registros `Approval`).
  - `observability.token_usage` está modelado en dominio y schema pero no se recolecta
    realmente (Pendiente).
  - No hay motor de migración de `schema_version`; el paso 0.1→1.0 es manual (ver
    `docs/GUIA_INSTALACION.md` §5.1). Pendiente: comando/script de migración (permitiría
    retirar la excepción `"0.1"` de `event.schema.json`).
- Fuentes verificadas:
  - `src/cli/internal/domain/{work_item.go, workflow.go, event.go, config.go,
    validation.go, workflow_validation.go, work_item_validation.go, diagnostic.go,
    template.go, contract_version.go}`.
  - `.sdd/schemas/{work-item,workflow,event,artifact}.schema.json`, `.sdd/config.yaml`,
    `.sdd/registry/capabilities.yaml`, `.sdd/workflows/*.workflow.yaml`.
  - `src/cli/internal/infra/{fs_repository.go, fs_archive_repository.go,
    fs_cancel_repository.go, project_initializer.go, artifact_manager.go,
    schema_validator.go, path_security.go}`.
