# Arquitectura de software

## Resumen

<!-- Describir la arquitectura general y sus principales responsabilidades. -->

El motor `sdd-cli` es una **CLI en Go con arquitectura hexagonal / clean architecture**.
El código vive en `src/cli/` y separa responsabilidades en capas concéntricas con la
regla de dependencia apuntando siempre hacia el dominio:

- **`internal/domain`** — núcleo puro: entidades (`WorkItem`, `Workflow`, `Event`,
  `Config`), la máquina de estados de fases/work items, las reglas semánticas del
  contrato (validación de workflows como DAG, validación de manifests) y los errores del
  dominio. No depende de ninguna otra capa ni de librerías de infraestructura.
- **`internal/ports`** — interfaces (puertos) que el dominio/casos de uso necesitan:
  repositorios de work items, workflows y config; servicio de artefactos; inspector de
  validación; catálogo/instalador de adapters; `Clock` e `IDGenerator`.
- **`internal/usecases`** — orquestación de cada operación (start, begin, deliver,
  approve, reject, complete, archive, status, next, validate, record-event, init,
  adapters). Coordinan dominio + puertos sin conocer detalles de filesystem.
- **`internal/infra`** — adaptadores concretos de los puertos: repositorio sobre
  filesystem con transacciones atómicas, validador de JSON Schema, gestor de artefactos,
  inicializador de proyecto, repositorio de adapters, seguridad de paths, reloj y
  generador de IDs.
- **`cmd/`** — capa de presentación con Cobra: define los comandos, parsea flags, arma la
  *composition root* (`composition.go`, `NewProductionApplication`) inyectando las
  implementaciones de `infra` en los casos de uso, y traduce resultados/errores a la
  salida dual (texto humano o envelope JSON).
- **`main.go`** — punto de entrada mínimo: llama a `cmd.Execute()`.

El contrato `.sdd/` (workflows, schemas, templates, procedures, registry, config) se
distribuye **embebido en el binario** (`embeds/`, vía `go:embed`) y se materializa en el
proyecto destino con `init` / `adapters install`. Así el binario es autosuficiente: no
requiere red ni runtime externo.

## Sistemas y servicios conectados

| Sistema o servicio | Propósito | Dirección de integración | Datos intercambiados |
| --- | --- | --- | --- |
| Filesystem del proyecto (`.sdd/`) | Fuente de verdad del estado: manifests, artefactos, eventos, locks, transacciones | Lectura y escritura | `manifest.yaml`, `artifacts/*.md`, `events.jsonl`, `.locks/`, `.transactions/` |
| Git | Versionado y fuente de verdad última del repositorio | Indirecta (el usuario/adapter commitea; la CLI **no** ejecuta Git) | Archivos de `.sdd/` versionados |
| GitHub Releases | Distribución de binarios multiplataforma | Salida (publicación en release) / Entrada (instaladores descargan) | Binarios `sdd-cli`, `SHA256SUMS` |
| Adapters de agente (Claude Code) | Materializar roles del contrato en una herramienta concreta | Salida (la CLI instala archivos) | `CLAUDE.md`, `.mcp.json`, `.claude/` (agents, hooks, settings) |
| Agente/integración consumidora | Operar el motor y leer su estado | Bidireccional vía CLI + envelope JSON | Comandos con flags + respuestas `{success, data, error}` |
| Artefacto externo (`--from-artifact`) | Empezar un workflow desde un documento ya hecho | Entrada (import con verificación SHA-256) | Archivo Markdown local + su hash |

## Plataforma y ejecución

- Plataforma o sistema operativo: multiplataforma. El release compila para
  `linux/amd64`, `darwin/amd64`, `darwin/arm64` y `windows/amd64` (`CGO_ENABLED=0`,
  binarios estáticos).
- Runtime y versiones: **Go 1.22** (módulo `sdd-cli`). Binario nativo único, sin runtime
  externo (ni Node.js ni Python).
- Infraestructura: ninguna a nivel servidor. Todo corre localmente sobre el filesystem
  del proyecto. CI/CD en GitHub Actions.
- Ambientes: no aplica (herramienta de línea de comandos que se ejecuta en la máquina del
  desarrollador o del agente).
- Despliegue: versionado manual por tag `v*`; `release.yml` compila, genera `SHA256SUMS`
  y crea el GitHub Release (marca *prerelease* si el tag es alpha/beta/rc). Instalación
  vía `scripts/install.sh` / `install.ps1` o build desde fuente.

## Autenticación y autorización

<!-- Mecanismos de identidad, credenciales, permisos y secretos. -->

No hay autenticación de red ni gestión de credenciales en el motor: es una herramienta
local. La única noción de "autorización" es de **proceso**, no de identidad de sistema:

- Cada operación registra un **actor** (`kind`: `human` | `agent` | `cli` | `system`, más
  un `id`), informado por flags `--actor-kind` / `--actor-id`.
- Los **gates de aprobación** (`approve` / `reject`) exigen que el actor sea `human`
  (regla en el dominio: `ErrHumanActorRequired`). Un agente no puede aprobar su propio
  trabajo.
- El adapter de Claude Code publica `.claude/settings.local.json.example`; el archivo
  privado real queda ignorado por Git. Los hooks (`protect-sdd-paths.py`,
  `protect-secrets.py`) actúan como salvaguardas del lado del agente.

## Integraciones y comunicación

<!-- Protocolos, contratos, APIs, colas y dependencias externas. -->

- **Interfaz principal**: CLI construida con Cobra (`cmd/`), autodescriptiva vía
  `--help`. Flags globales `--dir` (proyecto con `.sdd/`) y `--json`.
- **Salida dual**: texto legible para humanos (stdout en éxito, stderr en error) o un
  **envelope JSON** único (`{ "success": bool, "data": ..., "error": {code, message,
  details} }`) para agentes/integraciones. Los códigos de error se mapean desde los
  errores del dominio (`not_found`, `invalid_transition`, `validation_failed`,
  `concurrent_modification`, `work_item_locked`, etc.). Exit code `0` en éxito, `1` en
  error.
- **Contrato embebido**: el binario no llama a servicios; interpreta el contrato `.sdd/`
  (embebido y/o instalado en el proyecto). La validación por JSON Schema
  (`santhosh-tekuri/jsonschema`) se hace contra los schemas en `.sdd/schemas/`.
- **Idempotencia**: `--operation-id` (1–128 chars, `[A-Za-z0-9._:-]`) permite reintentar
  un comando sin duplicar eventos ni reaplicar la transición.
- **Sin MCP en la BETA**: la ejecución del motor vía Model Context Protocol está fuera de
  alcance por ahora (el `.mcp.json` del adapter configura otros MCPs, no expone la CLI).

## Observabilidad y operación

- Logs: no hay logger tradicional. La observabilidad del proceso se basa en **eventos
  append-only** en `events.jsonl` por work item (cada transición y las operaciones
  custom via `record-event` quedan registradas con actor, timestamp, tipo, data y
  `correlation_id`).
- Métricas: modeladas pero no activas. El manifest y `work-item.schema.json` contemplan
  `observability.token_usage` (`not_reported` | `partial` | `recorded`, con tokens de
  input/output/cache); marcado como opcional/futuro en `config.yaml` y `CHANGELOG.md`.
- Trazas: la trazabilidad se logra combinando las **tres fuentes de verdad** por work
  item: `manifest.yaml` (estado actual), `artifacts/*.md` (evidencia) y `events.jsonl`
  (historial inmutable). `traceability` en el manifest referencia eventos y work items
  relacionados.
- Alertas y operación: no aplica (no es un servicio). El comando `validate` funciona como
  chequeo de salud del contrato y de cada work item, con estados `passed`/`warning`/
  `failed` y exit code `1` ante fallos.

## Seguridad y restricciones

<!-- Controles, límites técnicos o regulatorios conocidos. -->

- **Seguridad de paths** (`internal/infra/path_security.go`): todas las rutas se resuelven
  con `containedPath`, que verifica —resolviendo también symlinks— que el candidato quede
  **contenido dentro de su root** (`.sdd/...`), rechazando escapes (`..`), rutas absolutas
  y symlinks fuera del árbol. Los artefactos deben ser rutas relativas normalizadas dentro
  de `artifacts/` y con extensión `.md`.
- **Escrituras atómicas / transaccionales** (`internal/infra/fs_repository.go`): el commit
  de un work item confirma manifest + artefactos + eventos como "todo o nada" mediante
  *staging* en `.transactions/`, `fsync`, `rename`, *backup* del estado previo y
  *recovery* automático ante interrupciones. Rollback restaura el estado anterior si la
  publicación falla.
- **Concurrencia segura**: un solo escritor por work item mediante lock de archivo
  (`gofrs/flock`, `.locks/<id>.lock`) más **control de revisión optimista**: si la
  revisión cambió por debajo se devuelve `concurrent_modification`.
- **Integridad de artefactos externos**: `--from-artifact` calcula y persiste el
  **SHA-256** del documento importado; el manifest exige `sha256` con patrón de 64 hex.
- **Validación por contrato**: JSON Schema (`AssertFormat: true`) + reglas semánticas del
  dominio (DAG sin ciclos, entry points no ambiguos, dependencias existentes, un artefacto
  por fase, paths contenidos, templates existentes y renderizables sin placeholders).
- **Consultas puras**: `status`, `next` y `validate` nunca mutan estado ni crean locks.
- Restricciones regulatorias: no identificadas (herramienta local de proceso).

## Pendientes y fuentes

- Pendientes o supuestos:
  - Observabilidad de tokens/costos: modelada pero no implementada como recolección real
    (Pendiente, confirmado en `CHANGELOG.md`).
  - Ejecución vía MCP: Pendiente (fuera de alcance BETA).
  - No se identificó un mecanismo de logging estructurado más allá de `events.jsonl`.
- Fuentes verificadas:
  - `src/cli/main.go`, `src/cli/go.mod`, `src/cli/cmd/root.go`, `src/cli/cmd/composition.go`.
  - `src/cli/internal/infra/{fs_repository.go, path_security.go, schema_validator.go,
    project_initializer.go}`.
  - `src/cli/internal/domain/{work_item.go, workflow.go, event.go, validation.go}`.
  - `.sdd/schemas/*.json`, `.sdd/config.yaml`.
  - `docs/CLI.md`, `docs/SDD_WORKFLOW.md`, `README.md`, `CHANGELOG.md`,
    `.github/workflows/{ci.yml, release.yml}`.
