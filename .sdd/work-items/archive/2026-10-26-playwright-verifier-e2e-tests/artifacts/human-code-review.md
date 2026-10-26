---
schema_version: "1.0"
kind: artifact
id: "code-review-record"
work_item: "playwright-verifier-e2e-tests"
phase: "human-code-review"
status: delivered
created_at: "2026-10-22T17:20:46Z"
created_by: { kind: "cli", id: "sdd" }
sources: 
  - artifacts/verification-report.md
---

# Revisión humana de código: Integrar Playwright CLI + skills para verificación de UI

## Decisión

**Aprobado** por el revisor humano (matias), 2026-10-22. Se autoriza pasar a `archive` con
commit y push. No se pide pull request.

## Observaciones

### Diseño aprobado

- Rige la revisión 3 del plan (`artifacts/plan.md`, sección "Revisión 3", opción B).
  Prevención en lugar de recolección:
  - `outputDir` explícito por sesión con `open --config`;
  - prohibición de `--filename` y `state-save`/`state-load`, reforzada con `ask` en el
    adapter;
  - cierre en dos pasos: borrar `storage-state-*.json` y comparar `git status` antes y
    después con `diff`.
- La desviación respecto de la revisión 2 queda formalizada en la revisión 3 aprobada.
- La verificación de la corrida 6 dio **Passed** (V1–V4). V4 se repitió en modo default: los
  pedidos de confirmación de `--filename` y `state-save` aparecieron y fueron rechazados.

### Cambio durante la revisión (sin reabrir el work item, a pedido del revisor)

- `verify-ui.md`, Precondiciones y Preparación paso 3: se reemplazó
  `install-browser chromium --with-deps` (26 paquetes) por
  `sudo apt-get install -y libnss3 libnspr4`, las librerías que faltaron en WSL (Ubuntu).
  Se agregaron el síntoma y el diagnóstico con `ldd`.
- Es solo texto: no afecta el flujo ni los permisos verificados en V1–V4.
- Después del cambio, `go test ./... -count=1` pasó.
- El revisor sincronizó la copia de la raíz (`.sdd/procedures/verify-ui.md`).
- Queda registrado como evento `review.change` en `events.jsonl`.

### Gaps aceptados por el revisor

| # | Gap | Disposición |
| --- | --- | --- |
| G-R6-2 | El control 7b no mira `evidence/` fuera de `ui/`. Un `state-save` absoluto ahí no se detecta. | Riesgo aceptado: requiere violar la regla y aprobar el `ask`; `evidence/` no se versiona |
| G-R6-3 | No hay un paso explícito para borrar un `state-save` absoluto escrito fuera del proyecto. | Riesgo aceptado: solo pasa con aprobación humana; queda fuera del repo |
| G-R6-4 | Falta un criterio de cierre de 7b cuando quedan cambios legítimos reportados (código 1). | Aceptado; mejora para un work item futuro |
| G-1 | No hay test automático del paso 7 (códigos 0/1/2, `dash`/`bash`, exclusión de `evidence/`). | Work item futuro |
| G-4 | No se probó en sesión el comportamiento `Bloqueada` del verifier con Playwright no preparado. | Work item futuro |

### Otras decisiones registradas en el work item

- `evidence/` de todo work item queda local (`.gitignore`: `.sdd/work-items/*/*/evidence/`).
- La investigación de referencia se trackea en `.sdd/research/playwright-cli-para-sdd.md`.
- La copia de dogfooding en la raíz (`.sdd/`, `.claude/`) la sincroniza el usuario mientras
  no exista un comando de upgrade.

## Revisor y fecha

- Revisor: matias (humano)
- Fecha: 2026-10-22
