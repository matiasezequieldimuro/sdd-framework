---
schema_version: "0.1"
kind: reconstructed-spec
id: "inicializacion-y-contrato-embebido"
capability: "init"
status: reconstruida-desde-codigo
created_at: "2026-09-22"
created_by: { kind: "agent", id: "sdd-doc-subagent" }
sources:
  - src/cli/cmd/init.go
  - src/cli/internal/usecases/init_uc.go
  - src/cli/internal/infra/project_initializer.go
  - src/cli/embeds/embeds.go
---

# Especificación: Inicialización y contrato embebido (`init`)

> Especificación reconstruida desde el código fuente durante el onboarding del motor
> `sdd-cli`. La fuente de verdad prioritaria es el código; no describe un work item nuevo.

## Requisitos y comportamiento

### Propósito

El comando `sdd-cli init` materializa el **contrato declarativo `.sdd/`** —embebido en el
binario vía `go:embed`— dentro del directorio destino, dejando el proyecto listo para
operar workflows SDD sin necesidad de red ni de otro runtime. El binario es autosuficiente:
lleva su propia copia del contrato en `embeds.DefaultSDDResources` (`//go:embed all:default_sdd`),
que se sincroniza desde `src/.sdd/` con `go generate` (`tools/syncsdd`).

### Comportamiento observable

- **Entrada**: comando `sdd-cli init`. No tiene flags propios; usa los flags globales del
  root (`--dir`, por defecto `.`, y `--json`). El directorio destino es `options.targetDir`.
- **Idempotencia / overwrite**: **no sobrescribe**. Si ya existe `.sdd/` en el destino,
  falla con `.sdd directory already exists in target path`. No hay `--force`. El comando no
  es idempotente en el sentido de "reintentable sin efecto": una segunda ejecución sobre un
  proyecto inicializado siempre falla.
- **Materialización** (`FSProjectInitializer.Initialize`):
  1. Verifica que `.sdd/` no exista (o error si `os.Stat` falla por otra causa).
  2. Crea un directorio de staging temporal `.sdd-init-*` dentro del destino.
  3. Recorre (`fs.WalkDir`) el FS embebido `default_sdd`, **excluyendo** las rutas
     `work-items` y `work-items/…` y `tests` y `tests/…` (fixtures de prueba). Cada archivo
     se escribe con `writeInitializedFile` (open `O_CREATE|O_EXCL|O_WRONLY`, write, `fsync`).
  4. Crea explícitamente `work-items/active/` y `work-items/archive/`, cada uno con un
     archivo `.gitkeep` (para que Git preserve los directorios vacíos).
  5. **Publica** el staging con un `os.Rename` atómico a `.sdd/`.
  6. Ante cualquier fallo, el staging se limpia (`os.RemoveAll` diferido).
- **Estructura creada** (según el contrato embebido `src/.sdd/`): `config.yaml`,
  `schemas/`, `workflows/`, `templates/`, `procedures/`, `registry/capabilities.yaml`,
  `context/` y `work-items/{active,archive}/.gitkeep`. Quedan fuera `work-items/` con
  contenido y `tests/`.
- **Salida**:
  - Texto: `Successfully initialized .sdd directory framework in '<dir>'`.
  - JSON (`--json`): envelope `{ "success": true, "data": "Successfully initialized .sdd directory framework" }`.
- **No** inicializa Git ni instala adapters (eso es responsabilidad de `adapters install`).

## Escenarios verificables

- Proyecto sin `.sdd/` → `init` crea la estructura completa y devuelve éxito; el rename
  final es atómico (no deja `.sdd/` a medias).
- Proyecto con `.sdd/` preexistente → error "already exists"; no se modifica nada.
- El contrato materializado **no** contiene `work-items/<id>/` ni `tests/` (fixtures),
  pero sí `work-items/active/.gitkeep` y `work-items/archive/.gitkeep`.
- `--dir <ruta>` → la estructura se crea bajo `<ruta>/.sdd`.

## Reglas, contratos y restricciones

- Escritura de archivos con `O_EXCL` + `fsync`: no se pisan archivos existentes en staging.
- Publicación atómica por `rename` de directorio; limpieza del staging ante error.
- El contrato embebido es la **única** fuente para `init`: no se descarga nada.
- Sincronización de embeds: `src/.sdd/` → `src/cli/embeds/default_sdd/` vía `go generate`;
  si no se regeneran los embeds, `init` materializa una versión desactualizada del contrato.

## Errores y casos borde conocidos

- `.sdd/` ya existe → error `.sdd directory already exists in target path`.
  Nota: es un `fmt.Errorf` sin error de dominio envuelto, por lo que en `--json` el `code`
  resultante es `internal_error` (no hay un código específico tipo `already_exists`).
- Fallo al leer el FS embebido, crear staging o publicar → errores envueltos con contexto;
  el staging se elimina.

## Deltas sobre baseline

No aplica (especificación reconstruida, sin baseline previo).

- **Pendiente/Desconocido**: `init` no ofrece `--force` ni migración de un `.sdd/` existente
  (confirmado: no hay flag ni código para ello). El detalle exacto del contenido embebido
  depende de `src/.sdd/` en el momento de `go generate` (no se enumeró archivo por archivo).

## Trazabilidad

- `src/cli/cmd/init.go` — comando Cobra y salida dual.
- `src/cli/internal/usecases/init_uc.go` — `InitUseCase.Execute`.
- `src/cli/internal/infra/project_initializer.go` — `Initialize`, exclusiones, staging, publish.
- `src/cli/embeds/embeds.go` — `//go:embed all:default_sdd`.
- `src/cli/cmd/root.go` — flags globales `--dir` / `--json`, envelope JSON.
