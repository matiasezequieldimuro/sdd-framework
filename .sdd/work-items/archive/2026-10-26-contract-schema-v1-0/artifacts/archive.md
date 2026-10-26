---
schema_version: "1.0"
kind: artifact
id: "archive-record"
work_item: "contract-schema-v1-0"
phase: "archive"
status: draft
created_at: "2026-10-26T20:27:11Z"
created_by: { kind: "cli", id: "sdd" }
sources: 
  - artifacts/human-code-review.md
---

# Archive: Migrar schema_version del contrato a 1.0

## Resultado de consolidación

**Work item:** `contract-schema-v1-0` (tipo `fast-change`, workflow `fast-change`, revisión 11).
**Objetivo:** el contrato SDD pasa de `schema_version` `0.1` a `1.0`. El motor escribe y exige `1.0`.

**Estado de fases al cierre:** plan aprobado (2026-10-26T20:10:01Z, con O1-O4 según recomendación),
implementación completada, verificación completada con veredicto **Pass** tras corregir D-1 y D-2,
revisión humana aprobada (2026-10-26T20:27:11Z, revisor `matias`).

**Evidencia de cierre:**

- `go build`, `go vet` y `go test ./...` en verde (222 tests), incluidos T1 a T8 del plan.
- Smoke de `init`, adapter, `start`, ciclo de fases, `archive` y `cancel` con el binario 1.0.
- Eventos mixtos 0.1/1.0 válidos. Rechazo explícito de manifest, config, workflow, registry y schemas 0.1.
- Verificación Pass en `artifacts/verification-report.md`. Evidencia en
  `evidence/` del work item (no versionada, ignorada por git).

**Alcance implementado:**

- `src/cli`: 18 archivos modificados y 3 archivos Go nuevos (`internal/domain/contract_version.go`,
  `internal/domain/contract_version_test.go`, `internal/infra/contract_version_test.go`). Total +124/−31 en modificados.
- `src/.sdd`: config, registry, schemas JSON, workflows, templates y fixtures migrados a `1.0`
  (excepción temporal de eventos, ver decisiones).
- `src/adapters/claude-code/adapter.yaml`.
- `.sdd/context/**` (arquitectura y specs de contexto afectadas) y `docs/GUIA_INSTALACION.md` §5.1.

**Decisiones:**

1. **Versión única `1.0`.** La constante vive en `domain.ContractSchemaVersion`. Los guards de Go no
   dependen de los schemas del proyecto, así que un proyecto con schemas viejos recibe un error explícito.
   `validate` agrega los checks `workflow.*` y `manifest.schema_version_supported`.
2. **Excepción temporal en `event.schema.json`.** Acepta el enum `["0.1", "1.0"]` para que el
   `events.jsonl` histórico quede inmutable. Se retira cuando exista un comando o script de migración
   (pendiente, ver más abajo).
3. **Migración manual por el usuario.** La migración de proyectos existentes (manifests, incluidos
   archivados y cancelados, config, registry y front-matter de artefactos) la hace el usuario a mano
   según `docs/GUIA_INSTALACION.md` §5.1. El motor no migra automáticamente.
4. **Defectos corregidos en el lugar, sin `rework`.** Con autorización explícita del usuario:
   - D-1: front-matter de `implementation-report.md` (`id: "implementation-report"`, `sources: [artifacts/plan.md]`).
   - D-2: la guía §5.1 paso 1 ya no exige que `.transactions/` esté vacío. Distingue markers de
     archive/cancel pendientes, `<id>.backup` esperable y restos inofensivos.
   - Quedaron registradas en el evento `verification.defects_fixed`.

**Commits creados (rama local `release/v1.0.0`, sin push):**

| Hash | Mensaje | Alcance |
| --- | --- | --- |
| `af99611` | `chore(sdd): archivar work item playwright-verifier-e2e-tests` | Movimiento de `active/` a `archive/2026-10-26-playwright-verifier-e2e-tests/` (7 archivos, renames) |
| `1f8920a` | `feat(contract)!: migrar schema_version del contrato a 1.0` | `src/cli`, `src/.sdd`, `src/adapters` (56 archivos, +519/−78). Incluye `BREAKING CHANGE` |
| `386d1a0` | `docs(sdd): documentar contrato 1.0 y guia de migracion manual` | `.sdd/context/**` y `docs/GUIA_INSTALACION.md` (11 archivos, +79/−23) |

No se versionaron: `.sdd/work-items/active/contract-schema-v1-0/**` (se commitea tras `sdd-cli archive`),
`evidence/` (ignorado por git) ni `src/cli/embeds/default_*` (generado, ignorado).

**Revisión humana: aprobada sin revisar el diff.** El revisor declaró que no revisó el código por falta
de tiempo y aprobó por confianza en los agentes y en la evidencia de verificación. Esto deja un riesgo
residual. **Se recomienda revisar el diff en la PR `release/v1.0.0` → `main`** antes de publicar.

**Gaps aceptados:**

- T8 (markers legacy de archive/cancel) solo tiene cobertura unitaria, sin prueba end-to-end con un marker 0.1 en disco.
- El flujo `--from-artifact` con SHA-256 modificado (guía §5.1 paso 7) no tiene prueba end-to-end.
- Config 0.1 se detecta en `validate` y `start`, pero `status` y `next` no lo leen (O-2, diseño por el que
  el registry solo se valida en `validate`).
- Escenarios 3 a 5 del plan no ejecutados en la verificación.

**Acciones omitidas por instrucción explícita del usuario:** push a remoto, creación de PR, merge y tag.
No se ejecutó ninguna acción remota.

## Baselines y changelog actualizados

- **Specs y contexto:** `.sdd/context/architecture/data-modeling.md` y `.sdd/context/specs/*` (9 specs)
  actualizados al contrato 1.0 en el commit `386d1a0`.
- **Baseline raíz `.sdd/`:** no modificado en este cierre. Sigue en `0.1` hasta la Etapa 5 (la ejecuta el usuario).
- **`CHANGELOG.md`:** no modificado. Decisión O2 del plan. Pendiente para el paso de release.
- **`docs/CLI.md:3`:** dice "contrato y CLI v0.1". Decisión O3 del plan. Pendiente de confirmación humana antes de la release.

## Referencias externas autorizadas

- Ninguna. No hubo push, PR ni acciones remotas por instrucción del usuario.
- Rama local: `release/v1.0.0`. Commits: `af99611`, `1f8920a`, `386d1a0`.

**Pendientes:**

1. **Etapa 5 del plan (usuario, después de `sdd-cli archive`):** sync del contrato raíz `.sdd/`, migración de
   `.sdd/config.yaml`, manifests y front-matter de artefactos, e instalación del binario 1.0.
2. **Push, PR y merge:** los hace el usuario manualmente.
3. **Migración entre versiones de contrato:** comando o script pendiente. Se registra en `CHANGELOG` en un paso posterior de release.
   Hasta entonces, `event.schema.json` mantiene el enum temporal `["0.1", "1.0"]`.
4. **Observación O-4 (resuelta, sin cambios):** la skill `certificate-weekly-hs` está versionada en
   `src/adapters/claude-code/.claude/{skills,commands}/` y se instala en cualquier proyecto nuevo. El usuario confirmó
   el 2026-10-26 que ese es el comportamiento buscado.
5. **Revisión del diff** en la PR de release (ver revisión humana arriba).
