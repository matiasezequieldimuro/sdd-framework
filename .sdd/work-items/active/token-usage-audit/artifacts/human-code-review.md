---
schema_version: "0.1"
kind: artifact
id: "code-review-record"
work_item: "token-usage-audit"
phase: "human-code-review"
status: draft
created_at: "2026-09-29T15:23:58Z"
created_by: { kind: "cli", id: "sdd" }
sources: 
  - artifacts/verification-report.md
---

# Revisión humana de código: Auditoría y registro de consumo de tokens

## Decisión

Revisión asistida ejecutada, hallazgos corregidos y re-verificados. El artefacto se somete al
**gate de aprobación humana** (`sdd-cli approve/reject`) para la firma explícita del revisor. La
decisión formal queda registrada en `events.jsonl` mediante el gate.

## Observaciones

### Proceso de revisión
- Se ejecutó una **revisión de código asistida** sobre el diff de la feature (sólo `src/.sdd`,
  `src/cli`, `src/adapters/claude-code`; la `/.sdd` raíz no forma parte de la feature — el cambio
  de `id/name` en `config.yaml` raíz se conservó por decisión del usuario).
- Build (`go build ./...`), `go vet` y `go test ./...` en verde durante toda la revisión.

### Hallazgos y resolución
| # | Sev. | Hallazgo | Resolución |
| --- | --- | --- | --- |
| F1 | ALTA | El hook pisaba el total con ceros si el transcript era ilegible (`OSError`) o sin mensajes de asistente → violaba RN-3/REQ-3 (pérdida de acumulado). | **Corregido.** Guard `if n == 0: return`; `OSError` se re-lanza y se captura en `main()` → log `transcript_read_error` + `exit 0` **sin** registrar. E-09 (ceros vía CLI directo) intacto. Tests T4 y T7 añadidos. |
| F2 | MEDIA | `start` cargaba/validaba config siempre, aun con `--workflow` explícito → regresión para proyectos sin `defaults.workflow`. | **Corregido.** Con `--workflow` explícito, un fallo de config es no-fatal (auditoría → inactiva). Sin `--workflow`, se preserva el comportamiento previo. Test nuevo `TestStartUseCaseExplicitWorkflowToleratesConfigFailure`. |
| F4 | BAJA | El use case recomputaba `total` para el evento (posible divergencia manifest ↔ event-log). | **Corregido.** El evento lee `item.Observability.TokenUsage.TotalTokens` (fuente única, D-a). |
| F3 | — | Reabrir una sesión **nueva** sobre el mismo work item sub-cuenta el total. | **Limitación aceptada** (una sesión por work item, RN-5/R-1). Documentada en `git-worktree/SKILL.md` y `sdd-orchestrator.md`. Sin cambio de código. |
| F5 | ALTA | **Detectado en prueba manual del usuario:** el adapter registraba el hook `Stop` en `settings.json` con `"matcher": "*"`. El evento `Stop` **no usa `matcher`** (sólo los tool-events lo usan); la estructura era inválida y el hook no disparaba en una sesión real de Claude Code. Los tests automatizados no lo detectaron porque invocan el script del hook directamente, salteándose el dispatch de hooks. | **Corregido.** Se quitó `matcher` del bloque `Stop` en `src/adapters/claude-code/.claude/settings.json`; `go generate` propagó el embed (`Stop[0] keys=['hooks']`). Pendiente: test-guardrail que asegure que los eventos sin tool no lleven `matcher`. |

### Limitaciones menores conocidas (v1, documentadas)
- Contadores en auditoría inactiva se serializan como campos omitidos (equivalente a `null`).
- Log del hook con timestamp de precisión de segundos (dos errores en el mismo segundo se pisan).
- Retención de logs sin rotación.

### Re-verificación post-fix (orquestador)
- `go build ./...` OK; `go test ./... -count=1` → 4 paquetes en verde, 0 regresiones.
- Script del hook `test-record-token-usage.sh` → **11/11** (incluye T4 transcript vacío → manifest
  queda `partial`; T7 transcript ilegible → log de error + manifest intacto).

## Revisor y fecha

Revisor: matias — 2026-09-29. Firma formal vía gate `sdd-cli approve/reject`.
