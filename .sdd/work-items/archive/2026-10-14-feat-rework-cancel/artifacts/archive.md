---
schema_version: "0.1"
kind: artifact
id: "archive-record"
work_item: "feat-rework-cancel"
phase: "archive"
status: delivered
created_at: "2026-10-14T15:41:53Z"
created_by: { kind: "cli", id: "sdd" }
sources: 
  - artifacts/human-code-review.md
---

# Archive: Comandos rework y cancel para work items

## Resultado de consolidación

Feature cerrada tras la aprobación humana de `human-code-review` (matias, 2026-10-14, ciclo 3).

- `sdd-cli rework <id> --phase <fase> --reason <motivo>`: reabre una fase cerrada, propaga
  `superseded`/`blocked` a descendientes y reactiva work items `completed`.
- `sdd-cli cancel <id> --reason <motivo>`: movimiento transaccional a
  `.sdd/work-items/cancelled/<id>/` con reserva de ID y recovery ante crash.
- Adapter Claude Code: comandos `/sdd:rework` y `/sdd:cancel`; skill `sdd-cli` con la sección
  "Rework and cancel".
- Correcciones detectadas durante el cierre (vía `rework` sobre este mismo work item): validación
  de items reactivados (F1), `next` sobre fases `superseded` (F2), guarda de dependencias en
  `begin` (F3), tests de crash-recovery de `cancel` (G1) y `.sdd/work-items/.gitignore` en `init`.
- Desviaciones aceptadas: D1 (`rework` sobre item cancelado → `not_found`) y D2 (sin `--reason`
  → `invalid_arguments`).
- Pendientes para work items posteriores: brechas G4–G8 de tests de conflicto del marker de
  `cancel`, `-race` en un entorno con gcc, y `next` no reporta fases `rejected`.

## Baselines y changelog actualizados

- Nueva: `.sdd/context/specs/rework-y-cancelacion.md`.
- Deltas: `transiciones-de-fase-y-gates.md`, `consultas-de-estado.md`,
  `creacion-de-work-item.md`, `validacion-de-integridad.md`, `trazabilidad-por-eventos.md`,
  `cierre-de-work-item.md`, `inicializacion-y-contrato-embebido.md`.
- `CHANGELOG.md`: sección `[Unreleased]` (Agregado / Corregido).

## Referencias externas autorizadas

Autorizadas por el usuario (matias, 2026-10-14), ejecutadas después del archive físico:

1. Commit en `feature/feat-rework-cancel` (skill `git-commit`, Conventional Commits). No se
   versionan `.sdd/work-items/.locks/`, `.sdd/work-items/.transactions/` ni la evidencia bruta
   de este work item (decisión del revisor).
2. Push de `feature/feat-rework-cancel` a `origin` (GitHub).
3. Pull request hacia `development`, tras la revisión del resumen por parte del usuario.

El merge no forma parte del archive y requiere confirmación explícita aparte. Los hashes de
commit y el enlace de la PR se registran en la descripción de la PR y en el historial de Git,
ya que el expediente archivado es inmutable.
