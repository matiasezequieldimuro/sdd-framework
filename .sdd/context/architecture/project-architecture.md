# Arquitectura del proyecto

## Estructura del repositorio

```text
sdd-framework/
├── AGENTS.md                     # Rol/objetivo del asistente en el diseño del framework
├── README.md                    # Visión general, filosofía, instalación, desarrollo
├── CHANGELOG.md                 # Scope funcional de la BETA
├── .mcp.json                    # Config MCP del propio repo
├── docs/
│   ├── CLI.md                   # Referencia de comandos y contrato de salida
│   ├── SDD_WORKFLOW.md          # Escenarios, fases, gates, artefactos
│   ├── GUIA_INSTALACION.md      # Instalación por OS
│   └── internal/                # Documentación de diseño interna
├── scripts/
│   ├── install.sh               # Instalador macOS/Linux
│   └── install.ps1              # Instalador Windows
├── .github/workflows/
│   ├── ci.yml                   # Formato, vet, tests, build
│   └── release.yml              # Binarios multiplataforma + SHA256SUMS
├── .sdd/                        # Instancia del contrato en este repo (onboarding actual)
│   ├── config.yaml
│   ├── workflows/*.workflow.yaml
│   ├── schemas/*.schema.json
│   ├── templates/*.md
│   ├── procedures/*.md
│   ├── registry/capabilities.yaml
│   ├── context/                 # Docs de contexto (este conjunto)
│   └── work-items/{active,archive}/
└── src/
    ├── .sdd/                    # Fuente canónica del contrato (template)
    ├── adapters/
    │   └── claude-code/         # Adapter: CLAUDE.md, .mcp.json, .claude/ (agents, hooks)
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
el binario, generada por `go generate` → `tools/syncsdd`) y `.sdd/` en la raíz (la
instancia que este propio repo usa). Lo mismo aplica al adapter (`src/adapters/` →
`src/cli/embeds/default_adapters/`).

## Organización por capas o módulos

<!-- Capas, módulos y responsabilidades de cada uno. -->

| Capa o módulo | Responsabilidad | Dependencias principales |
| --- | --- | --- |
| `internal/domain` | Entidades, máquina de estados de fases/work items, validación semántica del contrato (DAG, manifests, approvals), errores del dominio, render de templates | Ninguna interna; solo stdlib (`fmt`, `regexp`, `sort`, `time`, `path/filepath`) |
| `internal/ports` | Interfaces que abstraen persistencia y servicios: repositorios, `ArtifactService`, `ValidationInspector`, adapters, `Clock`, `IDGenerator` | `internal/domain` |
| `internal/usecases` | Un caso de uso por operación; validan input, coordinan dominio + puertos, arman eventos y commitean | `internal/domain`, `internal/ports` |
| `internal/infra` | Implementaciones concretas de los puertos: `FSWorkItemRepository`, `FSWorkflowRepository`, `FSConfigRepository`, `ArtifactManager`, `SchemaValidator`, `FSProjectInitializer`, `FSAdapterRepository`, `FSValidationInspector`, `path_security`, `runtime` (clock/id) | `internal/domain`, `internal/ports`, `embeds`, libs externas (`flock`, `jsonschema`, `yaml`) |
| `cmd` | Presentación Cobra, parseo de flags, composition root (DI), salida dual texto/JSON y mapeo de errores | `internal/usecases`, `internal/infra`, `internal/domain`, `cobra` |
| `embeds` | Empaqueta `default_sdd/` y `default_adapters/` en el binario vía `go:embed` | stdlib `embed` |
| `tools/syncsdd`, `tools/syncadapters` | Generadores que copian `src/.sdd` y `src/adapters` a `embeds/` | stdlib |

## Puntos de entrada

- Aplicación: `src/cli/main.go` → `cmd.Execute()` → `NewRootCommand(NewProductionApplication())`.
- CLI o jobs: comandos Cobra en `src/cli/cmd/` — `init`, `adapters` (`list`/`install`),
  `start`, `status`, `next`, `validate`, `begin`, `deliver`, `approve`, `reject`,
  `complete`, `archive`, `record-event`, `version`.
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
  `WorkflowRepository`, `ConfigRepository`, `WorkItemArchiver` en `ports`, implementados
  sobre filesystem en `infra`.
- **Composition Root / Dependency Injection manual**: `cmd/composition.go`
  (`NewProductionApplication`) cablea las implementaciones de `infra` en los casos de uso;
  los tests inyectan dobles.
- **State Machine**: transiciones de fase y work item en `domain/work_item.go`
  (`BeginPhase`, `DeliverPhase`, `ApprovePhase`, `RejectPhase`, `CompletePhase`, etc.).
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
  `workflow_validation.go`, `work_item_validation.go`, `diagnostic.go`, `template.go`);
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
  - No se inspeccionó exhaustivamente el contenido de `docs/internal/`; se asume
    documentación de diseño (Pendiente de confirmar detalle).
  - Detalle fino de `tools/syncsdd` y `tools/syncadapters` inferido por nombre y
    `generate.go` (no se leyó su implementación completa).
- Fuentes verificadas:
  - Árbol de archivos observado (`find src`, `find .sdd`).
  - `src/cli/{main.go, generate.go, go.mod}`, `src/cli/cmd/{root.go, composition.go}`,
    `src/cli/embeds/embeds.go`.
  - `src/cli/internal/{domain, ports, usecases, infra}/*.go` (múltiples archivos).
