---
schema_version: "0.1"
kind: reconstructed-spec
id: "instalacion-de-adapters"
capability: "adapters"
status: reconstruida-desde-codigo
created_at: "2026-09-22"
created_by: { kind: "agent", id: "sdd-doc-subagent" }
sources:
  - src/cli/cmd/adapters.go
  - src/cli/internal/usecases/adapters_uc.go
  - src/cli/internal/infra/fs_adapter_repository.go
---

# Especificación: Instalación de adapters (`adapters list` / `adapters install`)

> Especificación reconstruida desde el código fuente durante el onboarding del motor
> `sdd-cli`. La fuente de verdad prioritaria es el código.

## Requisitos y comportamiento

### Propósito

El comando `adapters` gestiona el catálogo de **adapters de agente** embebidos en el binario
(`embeds.DefaultAdapterResources`, `//go:embed all:default_adapters`) y los materializa sobre
un proyecto ya inicializado. Un adapter traduce los roles del contrato SDD a una herramienta
concreta de codificación. En la BETA el único adapter provisto es **`claude-code`**
(`src/adapters/claude-code/`), que aporta `CLAUDE.md`, `.mcp.json` y el árbol `.claude/`
(agents, hooks, settings).

### `adapters list`

- **Entrada**: `sdd-cli adapters list` (sin argumentos; `cobra.NoArgs`).
- **Comportamiento**: `FSAdapterRepository.ListAdapters` lee el FS embebido `default_adapters`,
  toma cada subdirectorio (ignorando los que empiezan con `.`), lee su `adapter.yaml`
  (`id`, `title`, `description`) y verifica que `id` coincida con el nombre del directorio.
  El resultado se ordena por `id`.
- **Salida**:
  - Texto: encabezado `SUPPORTED ADAPTERS` y una línea por adapter con formato
    `%-20s %s` (ID y Description). Nota: el `title` no se imprime en texto.
  - JSON: `{ "adapters": [ { "id", "title", "description" } ] }`.

### `adapters install <adapter-id>`

- **Entrada**: `sdd-cli adapters install <adapter-id>` (`cobra.ExactArgs(1)`), más flags
  globales `--dir` / `--json`.
- **Comportamiento** (`InstallAdapterUseCase` → `FSAdapterRepository.InstallAdapter`):
  1. Valida que `<adapter-id>` sea kebab-case (`domain.ValidateIdentifier`).
  2. Lee el descriptor del adapter (existencia + coincidencia de `id`).
  3. Exige proyecto inicializado: `.sdd/` debe existir y ser directorio, o error
     "`.sdd directory not found; run sdd-cli init first`".
  4. Recolecta los archivos del adapter (excluye `adapter.yaml`), ordenados por ruta.
  5. Calcula los **roots** (primer segmento de cada ruta, p. ej. `CLAUDE.md`, `.mcp.json`,
     `.claude`) y **verifica colisiones antes de escribir**: si algún root ya existe
     (`os.Lstat`), falla con `ErrAdapterInstallConflict` **sin sobrescribir nada**. No hay
     `--force`.
  6. Escribe todo en un staging temporal `.sdd-adapter-install-*`, luego **publica cada root**
     con `os.Rename` dentro del `containedPath` del destino; si falla a mitad, revierte los
     roots ya publicados (`rollbackPublishedRoots`).
- **Salida**:
  - Texto: `Adapter "<id>" installed with <n> file(s).`
  - JSON: `{ "id": "<id>", "files": [ ...rutas relativas ordenadas... ] }`.

## Escenarios verificables

- `adapters list` en cualquier build → lista al menos `claude-code` con su descripción.
- `install claude-code` sobre proyecto con `.sdd/` y sin colisiones → materializa
  `CLAUDE.md`, `.mcp.json` y `.claude/`; devuelve la lista de archivos instalados.
- `install <id>` sin `.sdd/` previo → error indicando ejecutar `init` primero.
- `install <id>` cuando ya existe alguno de los roots → error de conflicto; no se escribe.
- `install <id>` con id inexistente → `ErrAdapterNotFound`.

## Reglas, contratos y restricciones

- Comprobación de colisiones **atómica respecto a la escritura**: primero se verifica todo,
  luego se publica; ante error se hace rollback de lo publicado.
- Todas las rutas de destino pasan por `containedPath` (no pueden escapar del proyecto).
- El catálogo es puramente derivado del FS embebido; no hay registro externo.
- Sincronización de embeds: `src/adapters/` → `src/cli/embeds/default_adapters/` vía
  `go generate` (`tools/syncadapters`).

## Errores y códigos (envelope JSON)

- `ErrAdapterNotFound` → `not_found`.
- `ErrAdapterInstallConflict` → `already_exists`.
- `.sdd/` ausente (`ErrInvalidPath`) → `invalid_input`.
- id no kebab-case (`ErrInvalidIdentifier`) → `invalid_input`.

## Deltas sobre baseline

No aplica (especificación reconstruida).

- **Pendiente/Desconocido**: el conjunto exacto de adapters disponibles se deriva en runtime
  del contenido embebido; en la BETA solo se confirma `claude-code`. La instalación **no** es
  incremental ni actualiza un adapter ya presente (falla por colisión); no se observó un flujo
  de "update/upgrade" de adapters.

## Trazabilidad

- `src/cli/cmd/adapters.go` — comando raíz + subcomandos `list` / `install`, salida dual.
- `src/cli/internal/usecases/adapters_uc.go` — `ListAdaptersUseCase`, `InstallAdapterUseCase`.
- `src/cli/internal/infra/fs_adapter_repository.go` — catálogo, colisiones, staging/publish/rollback.
- `src/cli/embeds/embeds.go` — `//go:embed all:default_adapters`.
- `src/adapters/claude-code/` — contenido del adapter provisto.
