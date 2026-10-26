---
schema_version: "0.1"
kind: artifact
id: "archive-record"
work_item: "playwright-verifier-e2e-tests"
phase: "archive"
status: delivered
created_at: "2026-10-22T19:33:19Z"
created_by: { kind: "cli", id: "sdd" }
sources: 
  - artifacts/human-code-review.md
  - artifacts/verification-report.md
  - artifacts/implementation-report.md
  - artifacts/plan.md
---

# Archive: Integrar Playwright CLI + skills para verificación de UI

## Resultado de consolidación

Se entregó la capability `sdd.verify-ui` para verificar UI web con Playwright CLI, invocada de forma condicional desde `verify`:

- **Contrato:** procedimiento `procedures/verify-ui.md` y entrada en el registro (`sdd.verify-ui`, salida `ui_evidence`). Incluye preparación única por proyecto (versión fija, navegador y skills del paquete), ejecución por verificación con sesiones nombradas y `outputDir` explícito, y reglas de seguridad (sin `--filename` ni `state-save`/`state-load`, borrado del estado de sesión y control de fugas al cierre).
- **Procedimiento `verify.md`:** las comprobaciones de verificación visual y de flujo UI remiten a `sdd.verify-ui`.
- **Adapter `claude-code`:** skill `verify-ui` (wrapper fino), `sdd-verifier` la carga; `settings.json` agrega reglas `allow`, `ask` y `deny` para `playwright-cli`.
- **Pruebas:** `TestClaudeCodeSkillsReferenceRegisteredCapabilities` comprueba que las skills del adapter citen capabilities y procedimientos registrados, incluida `sdd.verify-ui`.
- **Documentación:** sección 5.4 de `docs/SDD_WORKFLOW.md`, `AGENTS.md` (corrige la regla de work items en este repo: dogfooding), investigación en `.sdd/research/playwright-cli-para-sdd.md`.
- **Cambio durante la revisión humana:** en `verify-ui.md` (Precondiciones y Preparación, paso 3), `install-browser chromium --with-deps` se reemplazó por `sudo apt-get install -y libnss3 libnspr4`. Solo afecta texto; ya está incluido en el commit del contrato.

**Verificación:** veredicto **Passed** (`artifacts/verification-report.md`). V1–V4 cumplidos; V4 se repitió en modo default y los prompts de `--filename` y `state-save` aparecieron y fueron rechazados.

**Validación previa a los commits (2026-10-22, desde `src/cli`):**
- `go generate ./...`: exit 0 (los embeds generados están ignorados por Git; no forman parte de los commits).
- `go test ./... -count=1`: exit 0. Todos los paquetes `ok` (`cmd`, `internal/domain`, `internal/infra`, `internal/usecases`).

**Specs (`.sdd/specs/`):** no aplica. El directorio solo contiene `.gitkeep` y el plan declara este pedido como "fast-change, sin especificación previa". El procedimiento pide actualizar la especificación vigente con los deltas, pero no existe una vigente que actualizar; crear un baseline a partir de este fast-change inventaría una especificación. El registro durable del cambio queda en `CHANGELOG.md` (sección Unreleased), en `verify-ui.md` y en la investigación de `.sdd/research/`. Si se quiere un baseline de la verificación de UI, es un work item aparte.

**Dogfooding:** las copias de la raíz de `.sdd/procedures/verify-ui.md`, `.sdd/procedures/verify.md` y `.sdd/registry/capabilities.yaml` son idénticas a las de `src/.sdd/` (`diff` sin diferencias). La raíz `.claude/` (no versionada) contiene la skill `verify-ui` y las reglas de `playwright-cli` en `settings.json`. No se comparó byte a byte; la sincronización sigue siendo acción del usuario hasta que exista un comando de upgrade.

## Commits

Rama `feature/playwright-verifier-e2e-tests`, sin push todavía:

| Hash | Mensaje | Contenido |
| --- | --- | --- |
| `6db0bbc` | `feat(contract): agregar capability sdd.verify-ui para verificar UI con Playwright CLI` | `verify-ui.md`, `verify.md`, `capabilities.yaml` (src y raíz) |
| `89f65b7` | `feat(adapter): agregar skill verify-ui y permisos de playwright-cli en claude-code` | skill `verify-ui`, `sdd-verifier.md`, `settings.json` |
| `7ff9742` | `test(cli): verificar que las skills del adapter referencien capabilities registradas` | `fs_adapter_repository_test.go` |
| `3e188e9` | `docs(sdd): documentar la verificacion de UI con Playwright CLI` | `CHANGELOG.md`, `docs/SDD_WORKFLOW.md`, `AGENTS.md`, `.sdd/research/playwright-cli-para-sdd.md` |
| `2864b1b` | `chore(repo): ignorar evidencia de todos los work items` | `.gitignore` |

El directorio del work item (`.sdd/work-items/active/playwright-verifier-e2e-tests/`) no se commitea en este paso. La `evidence/` está ignorada por `.gitignore` (`.sdd/work-items/*/*/evidence/`, confirmado con `git check-ignore`).

## Decisiones y gaps aceptados

Ver `artifacts/human-code-review.md`, sección "Gaps aceptados por el revisor":

- **G-R6-2:** el control 7b no revisa `evidence/` fuera de `ui/`; un `state-save` absoluto ahí no se detecta. Riesgo aceptado: requiere violar la regla y aprobar el `ask`; `evidence/` no se versiona.
- **G-R6-3:** no hay paso explícito para borrar un `state-save` absoluto escrito fuera del proyecto. Riesgo aceptado: solo ocurre con aprobación humana y queda fuera del repo.
- **G-R6-4:** falta criterio de cierre de 7b cuando quedan cambios legítimos reportados (código 1). Aceptado; mejora para un work item futuro.

Otras decisiones: `evidence/` de cada work item queda local; la investigación de referencia se versiona en `.sdd/research/`; la copia de dogfooding de la raíz la sincroniza el usuario.

## Pendientes y futuro

- **G-1:** no hay test automático del paso 7 (códigos 0/1/2, `dash`/`bash`, exclusión de `evidence/`). Work item futuro.
- **G-4:** no se probó en sesión el comportamiento `Bloqueada` del verifier con Playwright no preparado. Work item futuro.
- **G-R6-4:** criterio de cierre de 7b (ver arriba).
- Comando de upgrade para sincronizar `src/` hacia `.sdd/` y `.claude/` de la raíz (hoy lo hace el usuario por permisos).

## Acciones omitidas y pendientes de cierre

- **Pull request: omitida por indicación explícita del usuario.** No se abrió PR.
- **Push: no ejecutado en este paso.** El usuario lo planificó como segundo paso separado; la rama queda sin upstream hasta entonces.
- **Archivo físico (`sdd-cli archive`): pendiente por decisión del usuario.** El work item se completa, pero no se mueve a `archive/`. No se ejecutaron comandos mutantes de `sdd-cli`.
- **Commit de este artefacto y del expediente del work item:** pendiente en el paso posterior a que el orquestador complete el work item.
- **Worktrees:** no tocados (responsabilidad del orquestador).

## Referencias externas autorizadas

- Playwright CLI (`@playwright/cli`): versión fija y skills del paquete, según `procedures/verify-ui.md`. Se usó en la verificación de la corrida 6 (run-6). No se consultaron otras fuentes externas para este cierre.
