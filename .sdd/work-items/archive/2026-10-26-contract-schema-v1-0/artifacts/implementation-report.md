---
schema_version: "0.1"
kind: artifact
id: "implementation-report"
work_item: "contract-schema-v1-0"
phase: "implementation"
status: draft
created_at: "2026-10-26T20:30:00Z"
created_by: { kind: "agent", id: "sdd-developer" }
sources: ["artifacts/plan.md"]
---

# Reporte de implementación: Migrar schema_version del contrato a 1.0

> El front-matter queda en `"0.1"` a propósito: el binario instalado 0.1 opera este work item.

## Análisis previo del plan

- Se leyó el plan aprobado (O1–O4 resueltas como recomendadas). Alcance ejecutado: Etapas 1, 2 y 3. No se ejecutó la Etapa 4 (la hace el verifier) ni la Etapa 5 (la hace el usuario).
- Sin contradicciones entre plan y código: los números de línea y los literales coincidían con `git grep`.
- Fuentes oficiales: no se consultó documentación externa. No hay APIs nuevas; solo se usan `gopkg.in/yaml.v3` y `fmt`, ya presentes en el proyecto. El plan ya había verificado que `$comment` es una keyword estándar de draft 2020-12.

## Cambios realizados

### Etapa 1: fuente única y contrato de datos
- 1.1 `src/cli/internal/domain/contract_version.go` (nuevo): `ContractSchemaVersion = "1.0"` y `ValidateContractSchemaVersion(subject, version)` (error que envuelve `ErrSchemaValidation`, nombra el sujeto y las dos versiones).
- 1.2–1.4 `event.go`, `work_item.go` usan la constante. `workflow_validation.go` formatea "contract v%s" con la constante.
- 1.5–1.6 Schemas `artifact`, `work-item` y `workflow` con `const "1.0"`. `event.schema.json` con `enum ["0.1","1.0"]` y `$comment` de excepción temporal.
- 1.7–1.10 `config.yaml`, `capabilities.yaml`, 5 workflows, 10 templates y `adapter.yaml` en `"1.0"` (todo en `src/`).
- 1.11 Fixtures: 7 valid y 6 invalid, `schema_version` y `workflow.version` en `"1.0"`.
- 1.12 Tests existentes: literales de struct pasan a `domain.ContractSchemaVersion`. YAML embebido en strings pasa a `"1.0"`. `contract_integration_test.go` compara contra la constante.

### Etapa 2: enforcement (guards independientes de los schemas del proyecto)
- 2.1 `config_repository.go`: usa `ValidateContractSchemaVersion("config.yaml", ...)`.
- 2.2 `fs_repository.go`: helper `requireContractSchemaVersion(subject, data)`. Se llama antes de `ValidateYAML` en `readWorkItemAt` y en `GetWorkflow`. Si el YAML está mal formado devuelve `nil` y mantiene el error actual.
- 2.3 `artifact_manager.go`: `validateArtifactMetadata` lee `schema_version` y lo valida (cubre begin, templates, archive, `template.identity_valid` y `artifact.identity_valid`).
- 2.4–2.5 Markers de archive y cancel: se escribe la constante. La recuperación separa el chequeo de versión y devuelve `ErrArchiveConflict` con el mensaje de D5.
- 2.6–2.7 `fs_validation_inspector.go`: las checks de registry y config usan la función común (mensaje de pass de config: `schema version 1.0 is supported`).
- 2.8–2.9 Checks nuevas `workflow.schema_version_supported` y `manifest.schema_version_supported`. La de manifest no corta el resto de las checks.

### Etapa 3: documentación
- `.sdd/context/specs/*.md` (9): front-matter `"1.0"`.
- `trazabilidad-por-eventos.md` (enum con excepción temporal), `creacion-de-work-item.md` (1.0 y `workflow.version`), `validacion-de-integridad.md` (v1.0, dos checks nuevas y escenario de manifest 0.1).
- `.sdd/context/architecture/data-modeling.md`: v1.0, constante, guards, excepción de eventos, migración manual como pendiente.
- `docs/GUIA_INSTALACION.md`: nota de excepción en §5 y nueva §5.1 (texto del Anexo A).

## Decisiones relevantes o críticas
- Una sola constante en dominio. Infra depende de dominio y no al revés. No se creó una lista legacy en Go (YAGNI): la excepción de eventos vive solo en el schema y la fija un test de consistencia.
- Los guards no dependen de `.sdd/schemas/` del proyecto, así que un proyecto con schemas viejos igual recibe un error explícito.

## Desviaciones aprobadas del plan

Ninguna de fondo. Dos ajustes menores de ubicación y forma:
1. Los tests T3–T7 de infra se agruparon en un archivo nuevo `infra/contract_version_test.go`, en lugar de repartirlos entre `validation_test.go` y `persistence_test.go`. Es el mismo contenido, más cohesivo. T8 sí está en `archive_repository_test.go` y `cancel_repository_test.go`, como decía el plan.
2. T5 (variante de template): con los schemas del proyecto actualizados, el error sale antes por `artifact.schema.json` (`value must be "1.0"`). Por eso el test del `GetWorkflow` verifica el rechazo y que nombre `template plan`, y el guard de `validateArtifactMetadata` se prueba aparte con un test directo (`TestValidateArtifactMetadataRejectsLegacyVersion`).

## Pruebas añadidas o actualizadas
- `domain/contract_version_test.go` (nuevo): T1 (`"1.0"` ok; `"0.1"`, `""` y `"1.0.0"` fallan con mensaje), T2 (`NewEvent` y `NewWorkItem`).
- `infra/contract_version_test.go` (nuevo): T3 `TestContractSchemaVersionIsConsistent`, T4 manifest legacy (`schema_version` y `workflow.version`), T5 workflow y template legacy más el guard de metadata, T6 eventos mixtos 0.1/1.0 sin fallos de categoría `event`, T7 checks `*.schema_version_supported` en `failed`.
- `infra/archive_repository_test.go` y `infra/cancel_repository_test.go`: T8, marker legacy rechazado con `ErrArchiveConflict`.
- Actualizados los tests listados en 1.12.

## Pruebas realizadas

Desde `/home/matiasddimuro/personal/tools/sdd-framework/sdd-framework/src/cli`:

| Comando | Resultado |
| --- | --- |
| `go generate ./...` | OK (contrato y adapter sincronizados a `embeds/`) |
| `gofmt -l .` | sin salida |
| `go vet ./...` | OK |
| `go test ./... -count=1` | OK: `cmd`, `domain`, `infra` y `usecases` en verde |
| `go build ./...` y `go build -o /tmp/sdd-cli-1.0 .` | OK (binario en `/tmp`; `/usr/local/bin/sdd-cli` no se tocó) |
| `sdd-cli validate --dir <root>` (instalado 0.1) | pasa: 1373 checks, 0 failed (el `.sdd/` raíz sigue en 0.1) |
| `git grep '"0\.1"' -- src docs .sdd/context` (sin tests) | solo la excepción de eventos y su documentación, más la guía de migración |
| `git grep -i 'v0\.1\b\|contract v0' -- src .sdd/context` | solo `version.go` (producto), `project.md` (BETA, fuera de alcance) y `verify-ui.md` |

Una ejecución previa a los tests nuevos mostró fallos esperados (markers, config) mientras faltaba la Etapa 2. Quedaron resueltos al implementarla.

## Pendientes para el usuario o el verifier
- **Etapa 5 (usuario)**: sync del contrato raíz, migración de `.sdd/config.yaml`, manifests y front-matter de artefactos, instalación del binario 1.0. No se tocó nada del `.sdd/` raíz, de manifests, eventos ni items archivados.
- **Etapa 4 (verifier)**: smoke con `/tmp/sdd-cli-1.0` en un directorio temporal. Los escenarios 3 a 5 del plan no se ejecutaron aquí.
- **No tocado por decisión**: `CHANGELOG.md` (O2), `docs/CLI.md:3` (O3), wording BETA y `cmd/version.go`.
- Nada quedó bloqueado por permisos. Las ediciones en `.sdd/context/**` se permitieron.
- Nota para el verifier: `src/cli/sdd-cli` aparece en el árbol (binario ignorado por git); no se modificó.
