# Changelog

Todas las versiones relevantes de este proyecto se documentan en este archivo.
El formato sigue, a grandes rasgos, [Keep a Changelog](https://keepachangelog.com/es/1.1.0/)
y el versionado [SemVer](https://semver.org/lang/es/).

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

[0.1.0-beta]: https://github.com/matiasezequieldimuro/sdd-framework/releases/tag/v0.1.0-beta
