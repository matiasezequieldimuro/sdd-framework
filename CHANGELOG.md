# Changelog

Todas las versiones relevantes de este proyecto se documentan en este archivo.
El formato sigue, a grandes rasgos, [Keep a Changelog](https://keepachangelog.com/es/1.1.0/)
y el versionado [SemVer](https://semver.org/lang/es/).

---

## [Unreleased]

---

## [1.0.0] — 2026-10-26 — Primera versión estable

Primera versión mayor. Incorpora retrabajo y cancelación de work items y la verificación
de UI con Playwright CLI, y pasa el contrato a `schema_version` 1.0. **Rompe
compatibilidad con proyectos 0.1**: requiere una migración manual (ver más abajo).

### Cambiado (BREAKING)

- Contrato SDD `schema_version` `"0.1"` → `"1.0"` en config, registry, workflows, adapter,
  schemas, manifests (incluido `workflow.version`), front-matter de artefactos y markers de
  transacciones. `sdd-cli` 1.x escribe y exige `"1.0"`, y rechaza documentos 0.1 con un
  error explícito, aunque los `.sdd/schemas` del proyecto hayan quedado viejos.
- Excepción temporal: `event.schema.json` acepta `"0.1"` y `"1.0"` para mantener inmutable
  el histórico de `events.jsonl`. Los eventos nuevos se escriben en `"1.0"`.
- **Migración manual** de proyectos existentes: ver
  [`docs/GUIA_INSTALACION.md` §5.1](./docs/GUIA_INSTALACION.md).
- Las specs vigentes (baseline) se ubican en `.sdd/specs/`. `onboard-project` ya no usa
  `.sdd/context/specs/`. Si tu proyecto tiene specs ahí, movelas a `.sdd/specs/`.

### Agregado

- `sdd-cli rework <id> --phase <fase> --reason <motivo>`: reabre una fase cerrada y marca
  como `superseded` (o `blocked`) sus fases descendientes; reactiva work items `completed`.
- `sdd-cli cancel <id> --reason <motivo>`: cancela un work item activo, lo mueve a
  `.sdd/work-items/cancelled/<id>/` y reserva su ID.
- `sdd-cli init` crea `.sdd/work-items/.gitignore` para excluir `.locks/` y `.transactions/`.
- `validate`: checks `workflow.schema_version_supported` y `manifest.schema_version_supported`,
  que señalan cada archivo pendiente de migrar.
- Capability `sdd.verify-ui` y procedimiento `verify-ui.md`: verificación de UI con
  Playwright CLI (versión fija, sesiones nombradas y evidencia en el work item), invocada de
  forma condicional desde `verify`.
- Adapter Claude Code:
  - comandos `/sdd:rework` y `/sdd:cancel`, y skill `sdd-cli` actualizada;
  - skill `verify-ui` (wrapper fino), cargada por `sdd-verifier`, y permisos
    `allow`/`ask`/`deny` para `playwright-cli`;
  - skill y comando `certificate-weekly-hs`: redacta tareas de certificación de horas en
    Azure DevOps a partir del trabajo reciente del repositorio;
  - los agentes pueden escribir en `evidence/` de work items activos (permiso y hook
    `protect-sdd-paths.py`), y `.sdd/work-items/cancelled/**` queda protegido contra edición.

### Corregido

- `validate` acepta el ciclo de vida de work items reactivados por `rework`.
- `next` reporta fases `superseded` reiniciables y `begin` exige sus dependencias satisfechas.
- La skill `git-worktree` toma las ramas estables de `CLAUDE.md` en lugar de asumir `develop`.
- Los scripts de instalación apuntan a `docs/GUIA_INSTALACION.md` en sus mensajes de error.

### Documentación

- `docs/CLI.md`, `docs/SDD_WORKFLOW.md`, `README.md`, `.sdd/context/` y las specs quedan
  alineados con el código: `rework`, `cancel`, layout de `work-items/`, códigos de error,
  checks de `validate`, fases de los workflows y contenido del adapter.

### Problemas conocidos

- `sdd-cli init` sobre un `.sdd/` existente devuelve `internal_error` en lugar de
  `already_exists`.
- El texto de salida de `rework` cuenta como "superseded" también las fases que vuelven a
  `blocked`.
- `status` y `next` pueden completar o revertir una transacción interrumpida, así que no son
  estrictamente de solo lectura en ese caso.
- Un `config.yaml` 0.1 lo detectan `validate` y `start`, pero no `status` ni `next`.

### Pendiente / fuera de alcance

- Comando o script de migración entre versiones del contrato. Permitirá retirar la
  excepción `"0.1"` de `event.schema.json`.
- Observabilidad de tokens y costos, memoria (Engram) y navegación de código (CodeGraph).
- Ejecución del motor mediante MCP (Model Context Protocol).

---

## [0.1.0-beta] — Primera versión pública (BETA)

Primera versión funcional del framework **SDD (Spec-Driven Development)**. Incluye el
contrato del proceso, el motor determinista `sdd-cli` y un adapter para Claude Code.
El objetivo de esta BETA es validar el núcleo del motor y su ergonomía con uso real.

### Scope funcional

**Motor de proceso (`sdd-cli`)**

- Binario nativo único en Go, sin runtime externo, con el contrato `.sdd/` embebido.
- Máquina de estados para work items y fases, con transiciones validadas y sin saltos
  de fase arbitrarios.
- Gates de aprobación humana obligatorios (Human In The Loop) en las fases que lo
  requieren.
- Inicio de un work item desde cero o desde un artefacto externo (`--from-artifact`),
  con verificación de integridad por SHA-256.
- Persistencia transaccional sobre filesystem (commit atómico de manifest + artefactos
  + eventos, con staging, rename, backup y recovery).
- Concurrencia segura: lock por work item + control de revisión optimista.
- Idempotencia de comandos mediante `--operation-id`.
- Validación por JSON Schema y reglas semánticas (DAG sin ciclos, entry points, paths
  contenidos, templates válidos).
- Trazabilidad completa mediante eventos append-only (`events.jsonl`).
- Salida dual: texto para humanos y envelope JSON para agentes/integraciones.

**Comandos disponibles**

- `init`, `adapters list`, `adapters install`
- `start`, `status`, `next`, `validate`
- `begin`, `deliver`, `approve`, `reject`, `complete`, `archive`
- `record-event`, `version`

**Workflows incluidos**

- `feature-standard` (nueva feature)
- `change-request` (actualización de feature)
- `fast-change` (ajuste rápido)
- `bug-known-cause` (debug con causa conocida)
- `bug-investigation` (debug con investigación)

**Adapter Claude Code**

- Instalable con `sdd-cli adapters install claude-code`.
- Provee `CLAUDE.md`, `.mcp.json` y `.claude/` con agentes, skills (wrappers finos de
  las capabilities/procedures portables), comandos, hooks y reglas.

**Documentación**

- `README.md`, `docs/GUIA_INSTALACION.md`, `docs/SDD_WORKFLOW.md` y `docs/CLI.md`.
- Scripts de instalación multiplataforma (`scripts/install.sh`, `scripts/install.ps1`).
- CI/CD: validación en cada push/PR y publicación de binarios multiplataforma al taggear
  una versión `v*`.

### Fuera de alcance en esta BETA

- Operación de retrabajo semántico e invalidación transitoria (`superseded` no se
  propaga automáticamente hacia atrás; hoy requiere intervención manual del manifest).
- Comando de cancelación explícita (`cancelled` está modelado pero sin comando público).
- Observabilidad de tokens y costos, memoria (Engram) y navegación de código (CodeGraph).
- Ejecución del motor mediante MCP (Model Context Protocol); se expondrá en ese
  protocolo en una versión futura.

[Unreleased]: https://github.com/matiasezequieldimuro/sdd-framework/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/matiasezequieldimuro/sdd-framework/releases/tag/v1.0.0
[0.1.0-beta]: https://github.com/matiasezequieldimuro/sdd-framework/releases/tag/v0.1.0-beta
