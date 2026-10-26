---
schema_version: "0.1"
kind: artifact
id: "verification-report"
work_item: "contract-schema-v1-0"
phase: "verification"
status: draft
created_at: "2026-10-26T20:14:34Z"
created_by: { kind: "cli", id: "sdd" }
sources:
  - artifacts/implementation-report.md
---

# Reporte de verificación: Migrar schema_version del contrato a 1.0

> El front-matter queda en `"0.1"` a propósito: el binario instalado 0.1 opera este work item.

## Resultado

**Veredicto global (tras correcciones del 2026-10-26): Pass.**

> **Correcciones post-verificación** (orquestador, con autorización explícita del usuario para
> corregir en el lugar, sin `rework`):
>
> - **D-1 resuelto.** Front-matter de `artifacts/implementation-report.md`: `id: "implementation-report"`
>   y `sources: ["artifacts/plan.md"]`. Evidencia: `sdd-cli validate --dir <repo> --json` con el
>   binario instalado 0.1 → `valid: true`, 1401/1401 checks passed, 0 failed (R8 en Pass).
> - **D-2 resuelto.** `docs/GUIA_INSTALACION.md` §5.1 paso 1 ya no exige `.transactions/` vacío:
>   distingue markers `<id>.archive.json`/`<id>.cancel.json` (en curso; `status` con 0.x los
>   recupera vía `recoverArchiveIfNeeded`/`recoverCancelIfNeeded`), `<id>.backup` sin item en
>   `active/` (guardado interrumpido; `status` lo restaura) y `<id>.backup` con item presente
>   (resto inofensivo que el motor elimina en la próxima escritura, `recoverWorkItemTransaction`).
> - Los cambios no tocan código; los resultados de build, vet y tests siguen vigentes.
>
> El texto original del veredicto se conserva debajo como historial.

**Veredicto original: Failed.**

Motivo principal: `sdd-cli validate` con el binario instalado 0.1 (criterio R8 de la Etapa 4) falla con 3 checks sobre `artifacts/implementation-report.md`. La causa está en el front-matter de ese artefacto (`id` y `sources`), no en el código del contrato. Ver defecto D-1.

Lo verificado funciona: build, vet, gofmt, 222 tests en verde (incluidos T1 a T8), smoke completo de init, adapter, start, ciclo de fases, archive y cancel en `"1.0"`, eventos mixtos 0.1/1.0 válidos, rechazo explícito de manifest, config, workflow, registry y schemas viejos, y el grep de literales residuales limitado a la allowlist.

Fallos y defectos:

| ID | Severidad | Descripción | Bloquea verificación |
| --- | --- | --- | --- |
| D-1 | Alta | Front-matter de `implementation-report.md` incorrecto: `id: "implementation"` (debe ser `"implementation-report"`, el `produces` del workflow) y `sources: ["plan"]` (debe ser una ruta `artifacts/plan.md`). Provoca `artifact.front_matter_schema_valid`, `artifact.identity_valid` y `artifact.source_declared` en `failed` con el binario instalado. | Sí |
| D-2 | Media-baja | La guía `docs/GUIA_INSTALACION.md` §5.1 paso 1 exige que `.sdd/work-items/.transactions/` esté vacío y dice que `sdd-cli status <id>` lo recupera. En la práctica no se vacía: tras cualquier guardado normal queda `<id>.backup`, y `status` y `validate` no lo eliminan. El comportamiento es idéntico en el binario 0.1 (pre-existente), pero la afirmación de la guía no se sostiene. | No (documental) |

## Criterios cubiertos

Estado: `Pass` cubierto con evidencia; `Parcial` cubierto con observaciones; `Fail` no cumple; `Fuera de alcance` lo ejecuta el usuario en Etapa 5.

| Criterio de aceptación | Tarea del plan | Cambio implementado | Prueba o inspección | Evidencia | Resultado |
| --- | --- | --- | --- | --- | --- |
| R1: el motor escribe `"1.0"` en manifest, `workflow.version`, eventos, artefactos y markers | Etapas 1 y 2 | `domain.ContractSchemaVersion = "1.0"`; `NewEvent`, `NewWorkItem`, render de templates | Smoke 03, 14, 15, 16: manifest, eventos, `plan.md`, archive y cancel con `"1.0"`. Markers transitorios: cubiertos por T8 | `evidence/smoke/03-cycle-plan.log`, `evidence/smoke/14-full-cycle.log`, `evidence/smoke/15-archive-flow.log`, `evidence/smoke/16-cancel-flow.log`, `evidence/go/go-gate.log` (T2) | Pass |
| R2: el motor exige `"1.0"` y rechaza `"0.1"` con error explícito | Etapa 2 | `ValidateContractSchemaVersion`; guards en `fs_repository`, `artifact_manager`, `config_repository`, `fs_validation_inspector`, markers | Manifest 0.1 rechazado (`invalid_input`, mensaje explícito); `workflow.version` 0.1 rechazado; config 0.1 rechazado en `validate` y en `start` sin `--workflow`; workflow 0.1 rechazado; registry 0.1 rechazado en `validate`; schemas viejos no evitan el guard | `evidence/smoke/06-manifest-schema-0.1.log`, `evidence/smoke/07-manifest-workflow-version-0.1.log`, `evidence/smoke/08-config-0.1.log`, `evidence/smoke/09-workflow-0.1.log`, `evidence/smoke/10-registry-0.1.log`, `evidence/smoke/11-stale-schemas-manifest0.1.log`, `evidence/smoke/13-start-with-legacy-config-registry.log` | Parcial (ver O-2) |
| R3: `event.schema.json` acepta `["0.1","1.0"]` con excepción documentada; eventos nuevos `"1.0"` | Etapa 1 | `event.schema.json` con `enum` y `$comment` | Enum presente. `events.jsonl` mixto validate pasa. Evento `0.2` rechazado por `event.schema_valid`. Eventos nuevos todos `"1.0"` | `evidence/smoke/05-validate-mixed-events.json`, `evidence/smoke/05c-validate-item-mixed.log`, `evidence/smoke/17-adversarial-versions.log`, `evidence/smoke/03-cycle-plan.log` | Pass |
| R4: contrato fuente `src/.sdd` y `adapter.yaml` en `"1.0"`; fixtures y tests actualizados | Etapa 1 | 7 `valid` y 6 `invalid` fixtures; `adapter.yaml`; `go generate` sincroniza `embeds/` | `grep` de literales residuales (sólo allowlist); `TestContractSchemaVersionIsConsistent` (T3) en verde; `TestContractFixtures` en verde; diff `src/.sdd` vs `embeds/default_sdd` coincide salvo `work-items` (excluido por diseño de `syncsdd`) | `evidence/grep/git-grep-0.1-residuals.log`, `evidence/go/go-gate.log` | Pass |
| R5: documentación y specs actualizadas; wording BETA excluido | Etapa 3 | 9 specs, `trazabilidad-por-eventos.md`, `creacion-de-work-item.md`, `validacion-de-integridad.md`, `data-modeling.md`, `GUIA_INSTALACION.md` | Revisión manual contra código (ver sección de consistencia). `docs/CLI.md:3` ("contrato y CLI v0.1") no se tocó por decisión O3 | `evidence/grep/git-grep-0.1-residuals.log` | Parcial (O-3) |
| R6: checklist de migración manual y texto de CHANGELOG con "migración" como Pendiente | Etapa 3 | `GUIA_INSTALACION.md` §5.1 (checklist de 9 pasos); CHANGELOG diferido al release (O2) | §5.1 existe y coincide con el código salvo paso 1 (ver D-2). CHANGELOG no tocado en este item | `evidence/smoke/18-transactions-leftover.log`, `evidence/repo/installed-validate-repo.json` | Fail (D-2) |
| R7: dogfooding (sincronizar `.sdd/` raíz y migrar datos del repo) | Etapa 5 | No ejecutado (lo hace el usuario tras el archive) | Repo sigue en 0.1 y el binario instalado opera correctamente salvo D-1 | `evidence/repo/installed-status.txt`, `evidence/repo/new-binary-validate-repo.txt` | Fuera de alcance |
| R8: build, vet, test, smoke, eventos mixtos, manifest 0.1 rechazado, grep sin residuales; `validate` instalado pasa | Etapa 4 | Todo lo anterior | Build, vet, gofmt, tests, smoke y grep pasan. `validate` instalado falla (D-1) | `evidence/repo/installed-binary-validate-repo.txt`, `evidence/repo/installed-validate-repo.json` | Fail (D-1) |
| Escenario 1 del plan (init, adapter, grep sin literales en el dir nuevo) | Etapa 4 | Adapter y contrato embebido | `init` y `adapters install claude-code` en dir temporal; sólo `event.schema.json` contiene `"0.1"` en `.sdd` | `evidence/smoke/01-init.log`, `evidence/smoke/02-adapters.log` | Pass |
| Escenario 2 del plan (ciclo start, plan, approve `tester`, validate) | Etapa 4 | Engine | `start` deja `plan` en `in_progress` (no hace falta `begin`; el primer `begin` fue rechazado como esperado). `deliver`, `approve --by tester` y `validate` pasan. Ciclo completo a `completed`, `archive` y `cancel` también | `evidence/smoke/03-cycle-plan.log`, `evidence/smoke/04-validate-clean.json`, `evidence/smoke/14-full-cycle.log`, `evidence/smoke/15-archive-flow.log`, `evidence/smoke/16-cancel-flow.log` | Pass |
| Escenario 3 del plan (evento 0.1 inyectado en mitad, validate pasa) | Etapa 4 | `event.schema.json` | Línea 1 con `"0.1"` y resto `"1.0"`: `validate` pasa con 0 fallos de `event` | `evidence/smoke/05-validate-mixed-events.json`, `evidence/smoke/05b-status-mixed.log` | Pass |
| Escenario 4 del plan (manifest 0.1 falla con `invalid_input`; validate con `manifest.schema_version_supported`) | Etapa 2 | Guard `readWorkItemAt` y check de `validate` | `status --json` devuelve `invalid_input` y mensaje explícito; `validate` marca `manifest.schema_version_supported` en `failed`. Repetido con `1.0.0`, vacío y `1.1` | `evidence/smoke/06-manifest-schema-0.1.log`, `evidence/smoke/17-adversarial-versions.log` | Pass |
| Escenario 5 del plan (schemas del proyecto viejos no evitan el guard) | Etapa 2 | Guards en Go independientes de `.sdd/schemas` | `work-item.schema.json` con `const "0.1"` y manifest 0.1: `status` falla por el guard con mensaje explícito | `evidence/smoke/11-stale-schemas-manifest0.1.log` | Pass |
| T1 `ValidateContractSchemaVersion` | Etapa 1 | `domain/contract_version.go` | `TestValidateContractSchemaVersion` | `evidence/go/go-gate.log` | Pass |
| T2 `NewEvent` y `NewWorkItem` en `"1.0"` | Etapa 1 | `event.go`, `work_item.go` | `TestNewEventUsesContractSchemaVersion`, `TestNewWorkItemUsesContractSchemaVersion` | `evidence/go/go-gate.log` | Pass |
| T3 consistencia de datos del contrato | Etapa 2 | `infra/contract_version_test.go` (desvío: no en `validation_test.go`, ver Desviaciones) | `TestContractSchemaVersionIsConsistent` | `evidence/go/go-gate.log` | Pass |
| T4 manifest legacy rechazado | Etapa 2 | `infra/contract_version_test.go` (desvío de ubicación) | `TestGetWorkItemRejectsLegacyManifestVersion` | `evidence/go/go-gate.log` | Pass |
| T5 workflow y template legacy rechazados | Etapa 2 | `infra/contract_version_test.go` | `TestGetWorkflowRejectsLegacyVersions`, `TestValidateArtifactMetadataRejectsLegacyVersion` | `evidence/go/go-gate.log` | Pass |
| T6 eventos mixtos sin fallos de `event` | Etapa 2 | `infra/contract_version_test.go` | `TestInspectWorkItemAcceptsMixedEventVersions` (confirmado también en smoke 05) | `evidence/go/go-gate.log`, `evidence/smoke/05-validate-mixed-events.json` | Pass |
| T7 checks `*.schema_version_supported` en `failed` | Etapa 2 | `infra/contract_version_test.go` | `TestInspectorReportsLegacySchemaVersions` | `evidence/go/go-gate.log` | Pass |
| T8 marker legacy de archive y cancel rechazado con `ErrArchiveConflict` | Etapa 2 | `archive_repository_test.go`, `cancel_repository_test.go` | `TestGetWorkItemRejectsLegacyArchiveMarker`, `TestGetWorkItemRejectsLegacyCancelMarker` | `evidence/go/go-gate.log` | Pass (sólo unitario, ver Limitaciones) |
| Allowlist de `git grep` | Etapa 4 | Sólo tests legacy, enum de eventos, docs de migración y producto | Ninguna ocurrencia fuera de la allowlist en `src/` (salvo O-3 y producto/BETA) | `evidence/grep/git-grep-0.1-residuals.log` | Pass |

## Desviaciones del plan

1. **Ubicación de T3 a T7.** El implementador las agrupó en `infra/contract_version_test.go` en vez de repartirlas entre `validation_test.go` y `persistence_test.go`. Es el mismo contenido y es más cohesivo. Impacto: ninguno sobre alcance ni comportamiento. No requiere decisión humana.
2. **T5 variante de template.** Con los schemas del proyecto ya en `"1.0"`, el rechazo se produce antes por `artifact.schema.json`. Se agregó `TestValidateArtifactMetadataRejectsLegacyVersion` para el guard de metadata. Impacto: ninguno. No requiere decisión humana.
3. **Escenario 2 sin `begin` inicial.** Plan dice `begin`, `deliver`, `approve`. `start` deja `plan` en `in_progress`, así que `begin` se rechaza (comportamiento correcto del motor). Impacto: ninguno; es un error del procedimiento de la prueba, no del producto.
4. **`docs/CLI.md:3` sin cambios** (decisión O3 del plan). Sigue diciendo "contrato y CLI v0.1". Requiere confirmación humana (O3).

No hay desviaciones de fondo respecto del plan aprobado.

## Comandos, entorno y resultados

Entorno: WSL2 Linux 6.18, `go1.27.2 linux/amd64`, rama `release/v1.0.0`, raíz `/home/matiasddimuro/personal/tools/sdd-framework/sdd-framework`. Binario de verificación `/tmp/sdd-cli-1.0-verify` (`-ldflags "-X sdd-cli/cmd.version=v1.0.0-dev"`). Binario instalado `/usr/local/bin/sdd-cli` (v0.1.0-dev), no modificado. Directorios de prueba en `/tmp/sdd-verify-*` (fuera del repo, copias de trabajo).

### Validación estructural y pruebas automatizadas

Comandos ejecutados en `src/cli`:

- `go generate ./...`: OK (sincroniza `embeds/`, 61 archivos del contrato y 51 del adapter).
- `gofmt -l .`: sin salida.
- `go vet ./...`: OK.
- `go build ./...`: OK.
- `go test ./... -count=1 -v`: 222 ejecutados, 222 PASS, 0 FAIL, 0 SKIP. Paquetes `cmd`, `domain`, `infra` y `usecases` en `ok`. Log completo en `evidence/go/go-gate.log`.

Veredicto de esta sección: Pass.

### Smoke test

Procedimiento en `/tmp` con `--dir` absoluto:

1. `init` y `adapters install claude-code` (49 archivos del adapter).
2. `start smoke-item --workflow fast-change`, `begin` de `plan` rechazado (esperado), `deliver plan`, `approve plan --by tester`, `validate` pasa (110 checks de item y 350 de proyecto, 0 fallos).
3. Ciclo completo: implementation, verification, human-code-review (aprobado), `complete`, `archive` (fase `archive` begin/deliver y luego `archive`), archivado en `archive/2026-10-26-smoke-item` con manifest y 23 eventos en `"1.0"`.
4. Cancel de otro item con `--reason`: `cancelled/cancel-me` con manifest y 3 eventos en `"1.0"`. Segundo cancel rechazado con mensaje "already cancelled".
5. Validate post-archive y post-cancel: `valid: true`.

Veredicto: Pass.

### API testing

No aplica. El producto es una CLI sin API HTTP, gRPC ni servicio. La cobertura equivalente son los comandos CLI y sus códigos de salida (`rc`) y JSON (`--json`) que quedan en `evidence/smoke/`.

### UI visual

No aplica. El proyecto no tiene UI web en el alcance de este cambio. No se usó `sdd.verify-ui`.

### Flujo UI/RPA

No aplica. Mismo motivo que UI visual.

### Escenarios de versión (smoke y adversarial)

| Caso | Entrada | Resultado observado |
| --- | --- | --- |
| Eventos mixtos | Línea 1 con `"0.1"`, resto `"1.0"` | `validate` pasa, 0 fallos |
| Evento 0.2 | Línea agregada con `"0.2"` | `validate` falla (`event.schema_valid` entre otros) |
| Evento 0.1 nuevo | Línea agregada con `"0.1"` | Aceptado por diseño (enum) |
| Manifest `schema_version` 0.1 | Smoke 06 | `status --json` `invalid_input` con mensaje explícito; `validate` `manifest.schema_version_supported` en `failed` |
| Manifest `workflow.version` 0.1 | Smoke 07 | `work_item.workflow_version_mismatch`; `status` rc=1 |
| Manifest `1.0.0`, vacío, `1.1` | Smoke 17 | Rechazados por `manifest.schema_version_supported` |
| Config 0.1 | Smoke 08 | `validate` falla con `config.schema_version_supported`. `start` sin `--workflow` falla con mensaje explícito. `status` de un item existente no lee config y pasa (ver O-2) |
| Workflow 0.1 | Smoke 09 | `workflow.schema_version_supported` en `failed`; `status` falla por el guard |
| Registry 0.1 | Smoke 10, 13 | `validate` falla con `registry.schema_version_supported`. `status`, `next` y `start` no leen el registry y pasan (ver O-2) |
| Schemas viejos + manifest 0.1 | Smoke 11 | `status` falla por el guard de Go |

### Revisión de consistencia (diff, código y docs)

- Constante Go, schemas, contrato embebido, adapter y fixtures: consistentes. `schema_version` de los archivos fuente en `"1.0"` salvo `event.schema.json` (enum intencional). `src/.sdd` y `embeds/default_sdd` coinciden.
- Mensajes de error: todos citan la versión recibida, la requerida y `docs/GUIA_INSTALACION.md`, como especifica D1/D3.
- Specs `validacion-de-integridad.md` (escenario manifest 0.1 con `invalid_input`, checks `*.schema_version_supported`): verificado en smoke 06 y 17.
- `data-modeling.md` y `trazabilidad-por-eventos.md` (excepción de eventos, migración manual pendiente): consistentes con el código.
- Afirmación de la guía §5.1 paso 7 (hash SHA-256 de `--from-artifact` en manifest y `validate` falla si el archivo cambia): soportada por `ExternalArtifactReference.SHA256` y `reference.external_hash_matches`.
- Afirmación de la guía §5.1 paso 1 (`.transactions/` vacío; `status` recupera): NO soportada. Ver D-2 y evidencia `evidence/smoke/18-transactions-leftover.log`.
- Afirmación de §5.1 paso 9 (checks `*.schema_valid`, `artifact.identity_valid` señalan archivos pendientes): soportada (ver salida del binario 1.0 sobre el repo en `evidence/repo/new-binary-validate-repo.txt`).

### Validación del repo con el binario instalado (R8)

Comando: `/usr/local/bin/sdd-cli validate --dir /home/matiasddimuro/personal/tools/sdd-framework/sdd-framework --json`

Resultado: rc=1. Total 1400 checks, 1397 pass, 3 fail. Los 3 fallos están en `artifacts/implementation-report.md`:

- `artifact.front_matter_schema_valid`: `/sources/0` no cumple el patrón `^artifacts/...\.md$` (valor `plan`).
- `artifact.identity_valid`: `artifact metadata mismatch: id=implementation phase=implementation`, esperado `implementation-report`.
- `artifact.source_declared`: `source "plan" is not declared by workflow`.

El artefacto de comparación `archive/2026-10-26-playwright-verifier-e2e-tests/` usa `id: "implementation-report"` y `sources: [artifacts/plan.md]`, lo que confirma la regla. La salida de `validate` del binario 1.0 sobre el mismo repo falla de forma esperada por datos 0.1 (config, registry, templates, artefactos), que es la señal de migración pendiente de Etapa 5.

Veredicto: Fail (D-1).

### Estado del work item (read-only, binario instalado)

`status`: fase `verification` en `in_progress`, `plan` approved, `implementation` completed. `next --json`: procedimiento `verify`. Evidencia en `evidence/repo/installed-status.txt` y `evidence/repo/installed-next.json`.

## Evidencia asociada

Ruta base: `.sdd/work-items/active/contract-schema-v1-0/evidence/` (no versionada, ignorada por `.gitignore`).

- Go: `go/go-gate.log` (generate, gofmt, vet, build, test -v).
- Smoke: `smoke/01-init.log`, `02-adapters.log`, `03-cycle-plan.log`, `04-validate-clean.json`, `05-validate-mixed-events.json`, `05b-status-mixed.log`, `05c-validate-item-mixed.log`, `06-manifest-schema-0.1.log`, `07-manifest-workflow-version-0.1.log`, `08-config-0.1.log`, `09-workflow-0.1.log`, `10-registry-0.1.log`, `11-stale-schemas-manifest0.1.log`, `12-config-registry-read-paths.log`, `13-start-with-legacy-config-registry.log`, `14-full-cycle.log`, `15-archive-flow.log`, `15-validate-after-archive.json`, `16-cancel-flow.log`, `16-validate-after-cancel.json`, `17-adversarial-versions.log`, `18-transactions-leftover.log`.
- Repo con binario instalado: `repo/installed-validate-repo.json`, `repo/installed-binary-validate-repo.txt`, `repo/new-binary-validate-repo.txt`, `repo/installed-status.txt`, `repo/installed-next.json`.
- Grep de residuales: `grep/git-grep-0.1-residuals.log`.

Los logs no contienen secretos ni datos personales. Las rutas absolutas de `/tmp` aparecen en logs de prueba, sin contenido sensible.

## Gaps y defectos

### Defectos

- **D-1 (Alta, bloquea).** Front-matter de `artifacts/implementation-report.md`: `id` y `sources` incorrectos. Corrección recomendada por el orquestador/desarrollador: `id: "implementation-report"` y `sources: [artifacts/plan.md]`, manteniendo `schema_version: "0.1"`. Después volver a ejecutar `validate` con el binario instalado. No se corrigió en esta verificación porque el verifier no modifica artefactos de otras fases.
- **D-2 (Media-baja, documental).** `docs/GUIA_INSTALACION.md` §5.1 paso 1. Reescribir para decir que tras guardados normales puede quedar `.transactions/<id>.backup` (esperado), y que lo que sí hay que resolver antes de actualizar son markers de archive o cancel pendientes (`*.archive-backup`, markers), que `status` recupera. Hoy la guía exige algo que no ocurre en un proyecto sano.

### Observaciones (no son defectos de este cambio)

- **O-1 Backup leftover pre-existente.** `.transactions/<id>.backup` persiste tras cualquier guardado normal en ambos binarios (0.1 y 1.0). No es regresión. Se podría tratar como item aparte.
- **O-2 Enforcement de config y registry en runtime.** El registry sólo se lee en `validate` (ningún comando de runtime lo carga), así que el guard de registry es de `validate` por diseño. Config 0.1 sólo se detecta cuando se resuelve el workflow por defecto (`start` sin `--workflow`, `validate`); `status` y `next` no lo leen. R2 se considera cumplido con esta limitación, que conviene documentar. Si el usuario quiere que un proyecto con config 0.1 falle en todo comando, requiere un cambio de diseño.
- **O-3 `docs/CLI.md:3`** dice "contrato y CLI v0.1". Queda por decisión O3 del plan. Requiere confirmación humana antes de la release.
- **O-4 Skill `certificate-weekly-hs` en el adapter.** `src/adapters/claude-code/.claude/{skills,commands}/certificate-weekly-hs*` está versionado y `adapters install claude-code` lo instala en cualquier proyecto nuevo. Parece un artefacto específico del repo que se filtró al adapter. Pre-existente y fuera de este item. Recomiendo revisarlo antes de publicar.
- **O-5 Hashes de `--from-artifact`.** Verificado sólo por inspección del código, no por ejecución de un caso con `--from-artifact`. Ver gaps.

### Gaps de cobertura

- Markers legacy de archive y cancel (T8): cubiertos sólo por tests unitarios. No hay prueba de extremo a extremo con un marker 0.1 real en disco. Recomendación: agregar un caso de integración que simule una transacción interrumpida con marker 0.1 y compruebe el mensaje "recover it with the sdd-cli version that created it".
- Flujo `--from-artifact` con SHA-256 modificado (guía §5.1 paso 7): sin prueba de extremo a extremo en esta verificación.
- Config 0.1 en `status`: comportamiento aceptado pero no cubierto por test. Recomendación: documentar o decidir.

## Limitaciones o pruebas no aplicables

- Etapa 5 (corte del repo, dogfooding, migración de `.sdd/` raíz y work items) no se ejecutó. Es responsabilidad del usuario tras el archive. Por eso el criterio R7 queda fuera de alcance y el `validate` del repo seguirá fallando con el binario 1.0 hasta esa etapa.
- No se ejecutaron mutaciones sobre el repo. Todas las pruebas de mutación usaron copias en `/tmp`. El único cambio en el repo fue la sincronización de `src/cli/embeds/` por `go generate` (ignorado por git) y los archivos bajo `evidence/`.
- No se verificó `go test -race` ni pruebas de concurrencia de los nuevos guards.
- No se verificó el CHANGELOG (diferido por decisión O2 al paso de release).
- Los binarios de release (goreleaser, `release.yml`) no se construyeron; sólo el binario de verificación local.
