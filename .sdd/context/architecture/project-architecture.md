# Arquitectura del proyecto

## Estructura del repositorio

```text
sdd-framework/
├── AGENTS.md                     # Rol/objetivo del asistente en el diseño del framework
├── README.md                    # Visión general, filosofía, instalación, desarrollo
├── CHANGELOG.md                 # Historial de versiones y alcance
├── .mcp.json                    # Config MCP del propio repo
├── .gitignore                   # Ignora embeds generados, binarios, /.claude, /CLAUDE.md, runtime y evidence/ de .sdd
├── CLAUDE.md, .claude/          # Adapter instalado localmente (ignorados por Git)
├── docs/
│   ├── CLI.md                   # Referencia de comandos y contrato de salida
│   ├── SDD_WORKFLOW.md          # Escenarios, fases, gates, artefactos
│   ├── GUIA_INSTALACION.md      # Instalación por OS
│   └── internal/                # Docs de trabajo internas (ignoradas por Git; pueden no existir)
├── scripts/
│   ├── install.sh               # Instalador macOS/Linux
│   └── install.ps1              # Instalador Windows
├── .github/workflows/
│   ├── ci.yml                   # Formato, vet, tests, build
│   └── release.yml              # Binarios multiplataforma + SHA256SUMS
├── .sdd/                        # Contrato instalado en este repo (dogfooding)
│   ├── README.md, config.yaml
│   ├── workflows/*.workflow.yaml
│   ├── schemas/*.schema.json
│   ├── templates/*.md
│   ├── procedures/*.md
│   ├── registry/capabilities.yaml
│   ├── context/                 # Docs de contexto (este conjunto)
│   ├── research/                # Investigaciones fechadas (evidencia exploratoria)
│   ├── specs/                   # Specs vigentes/baseline (reconstruidas por funcionalidad)
│   └── work-items/{active,archive}/ (+ cancelled/ lazy, .locks/, .transactions/)
└── src/
    ├── .sdd/                    # Fuente canónica del contrato (template; incluye tests/fixtures)
    ├── adapters/
    │   └── claude-code/         # Adapter: adapter.yaml, CLAUDE.md, .mcp.json, .claude/
    └── cli/                     # Motor sdd-cli (Go)
        ├── main.go              # Punto de entrada
        ├── go.mod / go.sum
        ├── generate.go          # //go:generate de sync de embeds
        ├── cmd/                 # Comandos Cobra + composition root + tests e2e
        ├── embeds/              # Contrato y adapters embebidos (go:embed)
        │   ├── default_sdd/
        │   └── default_adapters/
        ├── tools/               # syncsdd, syncadapters (generadores de embeds)
        └── internal/
            ├── domain/          # Entidades, máquina de estados, reglas, errores
            ├── ports/           # Interfaces (puertos)
            ├── usecases/        # Casos de uso (orquestación)
            └── infra/           # Adaptadores concretos (filesystem, schema, etc.)
```

<!-- Describir solamente la estructura observada en el repositorio. -->

Observación relevante: el contrato existe en **tres lugares sincronizados**:
`src/.sdd/` (fuente canónica editable), `src/cli/embeds/default_sdd/` (copia embebida en
el binario, generada por `go generate` → `tools/syncsdd`, ignorada por Git) y `.sdd/` en la
raíz (la instancia que este propio repo usa para gobernar sus cambios como work items). Lo
mismo aplica al adapter (`src/adapters/` →
`src/cli/embeds/default_adapters/`).

Contenido del adapter `claude-code` (`src/adapters/claude-code/.claude/`):

- `agents/`: `sdd-orchestrator`, `sdd-functional-analyst`, `sdd-planner`, `sdd-developer`,
  `sdd-verifier`, `sdd-debugger`, `sdd-archivist`.
- `skills/`: wrappers de capabilities (`generate-prd`, `generate-change-request`,
  `generate-specification`, `create-plan`, `document-issue`, `investigate-bug`,
  `implement`, `verify`, `verify-ui`, `archive`, `status`, `setup-project`,
  `onboard-project`, `answer-project-question`, `research-topic`) y skills operativas sin
  capability (`sdd-cli`, `git-commit`, `git-worktree`, `pull-request`,
  `certificate-weekly-hs`).
- `commands/`: `sdd/{feature,change-request,fast-change,bug-known-cause,bug-investigation,
  status,next,approve,reject,rework,cancel,archive}.md` y `certificate-weekly-hs.md`.
- `hooks/`: `protect-secrets.py` (PreToolUse `Read|Grep|Glob|Bash`) y
  `protect-sdd-paths.py` (PreToolUse `Edit|Write|NotebookEdit`).
- `settings.json` (permisos `allow`/`ask`/`deny`, incluidos los de `playwright-cli`),
  `settings.local.json.example`, `.gitignore` (ignora `settings.local.json`) y `rules/`
  (vacío).
- `TestClaudeCodeSkillsReferenceRegisteredCapabilities`
  (`internal/infra/fs_adapter_repository_test.go`) verifica que cada skill que declara
  una capability la tenga registrada, que el procedure citado exista y coincida con el de
  la capability, y que `sdd.verify-ui` esté cubierta por la skill `verify-ui`.

## Organización por capas o módulos

<!-- Capas, módulos y responsabilidades de cada uno. -->

| Capa o módulo | Responsabilidad | Dependencias principales |
| --- | --- | --- |
| `internal/domain` | Entidades, máquina de estados de fases/work items (incluye rework y cancel), validación semántica del contrato (DAG, manifests, approvals), versión del contrato, errores del dominio, render de templates | Ninguna interna; solo stdlib (`errors`, `fmt`, `path/filepath`, `regexp`, `sort`, `strings`, `time`) |
| `internal/ports` | Interfaces que abstraen persistencia y servicios: repositorios (incl. `WorkItemArchiver`, `WorkItemCanceller`, `CancelledWorkItemChecker`), `ArtifactService`, `ValidationInspector`, `ProjectInitializer`, adapters, `Clock`, `IDGenerator` | `internal/domain` |
| `internal/usecases` | Un caso de uso por operación; validan input, coordinan dominio + puertos, arman eventos y commitean | `internal/domain`, `internal/ports` |
| `internal/infra` | Implementaciones concretas de los puertos: `FSWorkItemRepository` (también archive y cancel transaccionales, en `fs_archive_repository.go` / `fs_cancel_repository.go`), `FSWorkflowRepository`, `FSConfigRepository`, `ArtifactManager`, `SchemaValidator`, `FSProjectInitializer`, `FSAdapterRepository`, `FSValidationInspector`, `path_security`, `runtime` (clock/id) | `internal/domain`, `internal/ports`, `embeds`, libs externas (`flock`, `jsonschema`, `yaml`) |
| `cmd` | Presentación Cobra, parseo de flags, composition root (DI), salida dual texto/JSON y mapeo de errores | `internal/usecases`, `internal/infra`, `internal/domain`, `cobra` |
| `embeds` | Empaqueta `default_sdd/` y `default_adapters/` en el binario vía `go:embed` | stdlib `embed` |
| `tools/syncsdd`, `tools/syncadapters` | Generadores que copian `src/.sdd` y `src/adapters` a `embeds/` | stdlib |

## Puntos de entrada

- Aplicación: `src/cli/main.go` → `cmd.Execute()` → `NewRootCommand(NewProductionApplication())`.
- CLI o jobs: comandos Cobra en `src/cli/cmd/` — `init`, `adapters` (`list`/`install`),
  `start`, `status`, `next`, `validate`, `begin`, `deliver`, `approve`, `reject`,
  `complete`, `archive`, `rework`, `cancel`, `record-event`, `version` (más `completion` y
  `help`, generados por Cobra).
- APIs o handlers: no hay servidor HTTP ni handlers de red. La "API" es la superficie de
  comandos + el envelope JSON (`--json`).
- Otros: `generate.go` (`go generate`) sincroniza los recursos embebidos antes de compilar.

## Patrones implementados

<!-- Registrar patrones realmente presentes y dónde se aplican. -->

- **Clean / Hexagonal Architecture (Ports & Adapters)**: dependencias apuntando al
  dominio; `internal/ports` define interfaces y `internal/infra` las implementa.
- **Use Case pattern**: cada operación es un tipo `*XxxUseCase` con `Execute(...)` en
  `internal/usecases` (p. ej. `StartWorkItemUseCase`).
- **Repository pattern**: `WorkItemReader/Committer/CreationRepository`,
  `WorkflowRepository`, `ConfigRepository`, `WorkItemArchiver`, `WorkItemCanceller` en
  `ports`, implementados sobre filesystem en `infra`.
- **Composition Root / Dependency Injection manual**: `cmd/composition.go`
  (`NewProductionApplication`) cablea las implementaciones de `infra` en los casos de uso;
  los tests inyectan dobles.
- **State Machine**: transiciones de fase y work item en `domain/work_item.go`
  (`BeginPhase`, `DeliverPhase`, `ApprovePhase`, `RejectPhase`, `CompletePhase`,
  `ReworkPhase`, `Cancel`, etc.).
- **Embedded resources (`go:embed`)**: contrato y adapters dentro del binario
  (`embeds/embeds.go`), sincronizados con `go generate`.
- **Transactional filesystem / atomic write**: staging + fsync + rename + backup/recovery
  y rollback en `fs_repository.go`.
- **Optimistic concurrency + file locking**: revisión monótona (`Revision`) + `flock`.
- **Idempotency key**: `--operation-id` persistido como `correlation_id` en eventos.
- **Envelope de resultado**: `JSONResponse{success,data,error}` en `cmd/root.go`.

## Convenciones

- Nomenclatura: identificadores de dominio (work item id, workflow id, fase, artefacto,
  template) en **kebab-case** (`^[a-z0-9]+(?:-[a-z0-9]+)*$`, `domain.ValidateIdentifier`).
  Tipos Go en PascalCase; constructores `NewXxx`; interfaces con sufijos de rol
  (`WorkItemReader`, `WorkItemCommitter`). Errores exportados con prefijo `Err`
  (`domain/errors.go`).
- Organización de archivos: un paquete por capa; archivos por entidad/tema en `domain`
  (`work_item.go`, `workflow.go`, `event.go`, `config.go`, `validation.go`,
  `workflow_validation.go`, `work_item_validation.go`, `diagnostic.go`, `template.go`,
  `contract_version.go`, `errors.go`);
  un archivo por comando en `cmd/` y un archivo por caso de uso en `usecases/`.
- Manejo de errores: errores centinela en `domain/errors.go`, envueltos con `%w` y
  comparados con `errors.Is`. Violaciones de contrato con código estable vía
  `ContractViolation` (`code`, `message`, `cause`) en `domain/diagnostic.go`. `cmd/root.go`
  traduce cada error a un `code` del envelope JSON.
- Configuración: `.sdd/config.yaml` (`schema_version`, `defaults.workflow`,
  `artifact_language`, `archive_policy`, `interaction.mode`, `observability.token_usage`).
  Flags globales `--dir` y `--json`.
- Pruebas: tests `_test.go` colocados junto al código; e2e en `cmd/cli_e2e_test.go`;
  integración de contrato en `usecases/contract_integration_test.go`; fixtures válidas e
  inválidas en `src/.sdd/tests/fixtures/{valid,invalid}/` (excluidas del `init`).

## Dependencias internas

<!-- Relaciones importantes entre módulos y reglas para modificarlas. -->

- Regla de dependencia (hacia el dominio): `cmd → usecases → ports ← infra`, y todos
  dependen de `domain`. `domain` **no** debe importar `ports`, `usecases`, `infra` ni
  librerías de I/O. Mantener el dominio libre de dependencias externas es la invariante
  central.
- Para agregar una operación nueva: definir/extender interfaces en `ports`, implementar en
  `infra`, crear el caso de uso en `usecases`, exponer el comando en `cmd/` y cablearlo en
  `composition.go`.
- Cambiar el contrato (`src/.sdd/`) o el adapter (`src/adapters/`) requiere re-ejecutar
  `go generate ./...` para regenerar `embeds/` antes de compilar/testear (CI lo hace).
- `infra` es el único punto que toca filesystem, `flock`, `jsonschema` y `yaml`; el resto
  de las capas trabaja con tipos del dominio y puertos.

## Pendientes y fuentes

- Pendientes o supuestos:
  - `docs/internal/` está excluido por `.gitignore` ("Internal working docs"): no forma
    parte del repo versionado y su contenido no se documenta aquí.
  - Detalle fino de `tools/syncsdd` y `tools/syncadapters` inferido por nombre y
    `generate.go` (no se leyó su implementación completa).
- Fuentes verificadas:
  - Árbol de archivos observado (`find src`, `find .sdd`).
  - `src/cli/{main.go, generate.go, go.mod}`, `src/cli/cmd/{root.go, composition.go}`,
    `src/cli/embeds/embeds.go`.
  - `src/cli/internal/{domain, ports, usecases, infra}/*.go` (múltiples archivos).
  - `src/adapters/claude-code/` (árbol completo), `.gitignore`, `AGENTS.md`.
