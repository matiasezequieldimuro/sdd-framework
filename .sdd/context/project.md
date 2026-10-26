# Proyecto

## Propósito

<!-- Qué problema resuelve el proyecto y para quién. -->

Este repositorio es el **motor y template del SDD Framework** (Spec-Driven Development):
un framework para trabajar con agentes de IA de forma **gobernada**. En lugar de confiar
en que un LLM "recuerde" en qué punto del proceso está, el proceso se modela como fases
explícitas con aprobaciones humanas, artefactos y trazabilidad, y una CLI determinista
(`sdd-cli`) se encarga de hacerlo cumplir.

Separa dos responsabilidades que normalmente se mezclan:

- El **agente o persona** hace el trabajo cognitivo (redacta, investiga, programa).
- La **CLI (`sdd-cli`)** gobierna el proceso: valida reglas, cambia estado, exige gates
  humanos, prepara artefactos y registra evidencia.

Está pensado para ser "agent-agnostic": el mismo contrato puede operarse desde distintas
herramientas de codificación mediante *adapters* (en la v1.0.0, Claude Code). El público
objetivo es el propio autor y desarrolladores que quieran delegar tareas a la IA con un
proceso auditable y portable.

> Estado: **Estable** (`v1.0.0`).

## Alcance y límites

- Incluye:
  - El **contrato declarativo** `.sdd/` (workflows YAML, schemas JSON, templates,
    procedures, registry de capabilities, `config.yaml`).
  - El **motor determinista `sdd-cli`** en Go (binario nativo único, contrato embebido,
    sin runtime externo), con arquitectura hexagonal/clean.
  - El **adapter de Claude Code** (`src/adapters/claude-code/`): `CLAUDE.md`, `.mcp.json`
    y `.claude/` con 7 agents, skills (wrappers finos de las capabilities/procedures
    portables, más skills operativas como `sdd-cli`, `git-*`, `pull-request` y
    `certificate-weekly-hs`), commands (`/sdd:*` y `/certificate-weekly-hs`), hooks y
    settings (incluye permisos para `playwright-cli`, usado por `verify-ui`).
  - Documentación (`docs/`), scripts de instalación multiplataforma (`scripts/`) y CI/CD
    (`.github/workflows/`).
  - Cinco workflows: `feature-standard`, `change-request`, `fast-change`,
    `bug-known-cause`, `bug-investigation`.
- No incluye:
  - El trabajo cognitivo: la CLI **no** redacta PRDs, no programa la feature ni decide
    qué modelo usar. Eso lo hacen los agentes/personas.
- Uso del propio framework (dogfooding): los cambios al motor se hacen en `src/`, pero se
  gobiernan como work items SDD de este mismo repo. La `.sdd/` de la raíz es una copia
  instalada de `src/.sdd/` más el conocimiento propio del repo (`context/`, `specs/` y
  `research/`) y sus `work-items/` (hay expedientes archivados en
  `.sdd/work-items/archive/`). Ver `AGENTS.md`.
- Límites conocidos (fuera de alcance en la v1.0.0):
  - Observabilidad de tokens y costos, memoria (Engram) y navegación de código (CodeGraph).
  - Ejecución del motor mediante MCP (Model Context Protocol); se expondrá a futuro.
  - Comando de migración de `schema_version` (el paso 0.1→1.0 es manual, ver
    `docs/GUIA_INSTALACION.md` §5.1).
  - El retrabajo (`sdd-cli rework`) y la cancelación (`sdd-cli cancel`), listados como
    fuera de alcance en `0.1.0-beta`, ya están implementados (ver `CHANGELOG.md`
    `[Unreleased]` y `.sdd/specs/rework-y-cancelacion.md`).

## Componentes principales

<!-- Resumen de los componentes o módulos relevantes. -->

| Componente | Ruta | Responsabilidad |
| --- | --- | --- |
| CLI `sdd-cli` | `src/cli/` | Motor determinista en Go: máquina de estados, validación por contrato, persistencia transaccional y salida dual (texto/JSON). |
| Contrato SDD | `src/.sdd/` (fuente) y `.sdd/` (instancia del propio repo) | Definición declarativa del proceso: workflows, schemas, templates, procedures, registry, config. |
| Recursos embebidos | `src/cli/embeds/default_sdd/` y `default_adapters/` | Copia del contrato y de los adapters embebida en el binario vía `go:embed`. Se sincroniza con `go generate`. |
| Adapter Claude Code | `src/adapters/claude-code/` | Materializa los roles del contrato como subagentes, hooks y configuración para Claude Code. |
| Documentación | `docs/` | `CLI.md`, `SDD_WORKFLOW.md`, `GUIA_INSTALACION.md`. |
| Scripts | `scripts/` | `install.sh` (macOS/Linux) e `install.ps1` (Windows). |
| CI/CD | `.github/workflows/` | `ci.yml` (formato, vet, tests, build) y `release.yml` (binarios multiplataforma + `SHA256SUMS`). |

## Repositorios y dependencias relacionadas

<!-- Repositorios, servicios o paquetes externos relevantes. -->

- **Repositorio público**: `github.com/matiasezequieldimuro/sdd-framework` (los binarios se
  publican en GitHub Releases).
- **Dependencias Go** (`src/cli/go.mod`, módulo `sdd-cli`, Go 1.22):
  - `github.com/spf13/cobra v1.8.1` — framework de CLI.
  - `github.com/gofrs/flock v0.12.1` — lock de archivos por work item.
  - `github.com/santhosh-tekuri/jsonschema/v5 v5.3.1` — validación por JSON Schema.
  - `gopkg.in/yaml.v3 v3.0.1` — parseo de YAML (manifest, workflows, config).
- No hay servicios externos ni base de datos: toda la persistencia es sobre filesystem y
  Git es la fuente de verdad última del repositorio.

## Desarrollo local

- Requisitos: **Go 1.22+** (ver `README.md` y `src/cli/go.mod`). Sin Node.js, Python ni
  otro runtime. Git recomendado. `gh` CLI para la publicación de releases.
- Instalación: desde código, dentro de `src/cli/`; para usuarios finales existen los
  instaladores en `scripts/` y los binarios en GitHub Releases.
- Ejecución (desde fuente):
  ```bash
  cd src/cli
  go generate ./...   # sincroniza recursos embebidos (.sdd y adapters)
  go build -o sdd-cli .
  ./sdd-cli --help
  ```
- Pruebas: `go test ./...` (tests unitarios `_test.go` por paquete, más e2e en
  `src/cli/cmd/cli_e2e_test.go` y tests de integración de contrato en
  `internal/usecases/contract_integration_test.go`). CI usa `go test ./... -count=1`.
- Lint y formato: `gofmt` (CI falla si hay archivos no formateados) y `go vet ./...`. No
  se detectó un linter adicional (golangci-lint) configurado.

## Documentación y referencias

<!-- Enlaces o rutas a documentación disponible. Indicar cuando no exista. -->

- `README.md` — visión general, filosofía, instalación rápida y desarrollo.
- `docs/SDD_WORKFLOW.md` — escenarios, fases, gates, artefactos y fuentes de verdad.
- `docs/CLI.md` — referencia de comandos, flags, ejemplos y contrato de salida.
- `docs/GUIA_INSTALACION.md` — instalación, actualización y desinstalación por OS.
- `CHANGELOG.md` — historial de versiones, alcance de cada release y lo que queda fuera de alcance.
- `AGENTS.md` — rol/objetivo del asistente en el diseño del framework.
- Contexto de arquitectura: `.sdd/context/architecture/` (este mismo conjunto de docs).
- Specs reconstruidas/consolidadas de las capacidades del motor: `.sdd/specs/`.

## Estado y pendientes

- Estado actual: **Estable `v1.0.0`** (primera versión mayor; sucede a `v0.1.0-beta`). El
  núcleo del motor, el contrato (`schema_version` 1.0) y el adapter de Claude Code están
  implementados y cubiertos por tests + CI.
- Pendientes o preguntas abiertas:
  - Observabilidad de tokens/costos (`observability.token_usage` está modelado en el
    manifest y el schema, pero marcado como `optional`/futuro), memoria (Engram) y
    navegación de código (CodeGraph).
  - Exposición del motor vía MCP.
  - Comando/script de migración de `schema_version`; cuando exista se podrá retirar la
    excepción `"0.1"` de `event.schema.json`.
- Resuelto respecto de `0.1.0-beta`: `rework` (fase a `ready` + descendientes a
  `superseded`/`blocked`) y `cancel` (mueve a `.sdd/work-items/cancelled/<id>/` y reserva
  el ID) existen como comandos públicos (`src/cli/cmd/{rework,cancel}.go`).
