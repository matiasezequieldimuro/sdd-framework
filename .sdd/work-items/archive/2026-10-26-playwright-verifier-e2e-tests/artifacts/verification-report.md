---
schema_version: "1.0"
kind: artifact
id: "verification-report"
work_item: "playwright-verifier-e2e-tests"
phase: "verification"
status: draft
created_at: "2026-10-22T14:57:21Z"
created_by: { kind: "cli", id: "sdd" }
sources:
  - artifacts/plan.md
  - artifacts/implementation-report.md
---

# Reporte de verificación: Integrar Playwright CLI + skills para verificación de UI

Corrida 6 (verificación acotada a V1–V4 de la definición de terminado de la revisión 3 de `artifacts/plan.md`, sobre la iteración 6 de implementación). Fecha: 2026-10-22.

Evidencia de esta corrida: `evidence/run-6/`. Evidencia de corridas previas (sin modificar): `evidence/run-2/`, `evidence/run-3/`, `evidence/run-4/`, `evidence/run-5/`, y `evidence/cli`, `evidence/contract`, `evidence/ui` (corrida 1).

Rutas sanitizadas: `<TMP>` es el directorio temporal de la prueba (ya borrado), `<REPO>` la raíz del repo, `<HOME>` el home del usuario.

## Veredicto

**Passed** (actualizado el 2026-10-22, después de repetir V4 en una sesión con permisos interactivos; ver "V4 (repetición)").

- V1, V2 y V3 pasan con evidencia reproducible (corrida 6 del `sdd-verifier`).
- **V4 pasa en la repetición.** El orquestador la ejecutó en modo default, con la persona usuaria a cargo de la decisión. `-s=v4 screenshot --filename=/tmp/v4.png` y `-s=v4 state-save` **pidieron confirmación** y la persona usuaria **rechazó ambas**. Se valida la regla `ask` con `-s=` delante del subcomando (S5 / G-2). La causa de la corrida original queda confirmada: la sesión estaba en modo auto, que resuelve los `ask` sin prompt.
- Gaps abiertos no bloqueantes: G-R6-2, G-R6-3, G-R6-4, G-1 y G-4. Quedan para decisión de la persona usuaria en `human-code-review`.

**Veredicto original de la corrida 6 (antes de la repetición de V4): Blocked.**

- V1, V2 y V3 pasan con evidencia reproducible.
- V4 (permisos en vivo, S5 / G-2) no se pudo validar. Los dos comandos con regla `ask` (`--filename` y `state-save`, con `-s=` delante del subcomando) se ejecutaron **sin prompt**. El usuario no tuvo ocasión de rechazarlos. Tampoco apareció prompt para `install` (regla `ask` `*install*`) durante V2, ni para una regla `ask` ajena a Playwright (`git remote *`, en modo solo lectura). La causa exacta no se pudo determinar desde el verificador: en el contexto de la sesión figura un aviso de modo auto activo, que es una explicación probable pero no confirmada.
- Sin V4 cumplido, la definición de terminado de la revisión 3 (§6) no está completa, así que no se puede marcar Passed.
- AC-b no se viola: en el repo real no se generó ningún archivo de estado y en la evidencia de la corrida no hay `*.json` de estado. Por eso el veredicto no pasa a Failed por esa vía.

**Qué hace falta para cerrar.** Repetir V4 en una sesión con permisos interactivos (sin modo auto), con la persona usuaria rechazando ambos prompts. Si el prompt aparece y se rechaza, V4 pasa y el veredicto puede pasar a Passed (con los gaps de la sección correspondiente). Si en esa sesión tampoco aparece prompt, el veredicto pasa a **Failed**: la barrera `ask` de la revisión 3 no se aplica y eso es un defecto de enforcement que el plan usaba como mitigación de los residuales de §2.

## Resumen del diseño vigente (revisión 3, opción B)

- **Prevenir, no recolectar.** `open --config` con un config del work item (copia de `.playwright/cli.config.json` con `outputDir` = `evidence/ui/`). Todo lo que la CLI escribe sin `--filename` cae en `evidence/ui/`.
- **Sin vías de fuga.** No usar `--filename` ni `state-save`/`state-load` salvo aprobación humana. Snapshot y consola con `snapshot --raw > <ruta absoluta>`.
- **Paso 7.** 7a borra `storage-state-*.json` de `evidence/ui/`. 7b compara `git status --porcelain=v1 --untracked-files=all` antes y después, con pathspec que excluye `evidence/` del work item. Código 0 sin cambios, 1 con cambios, 2 si falta `GIT_BEFORE`.
- **Adapter.** Cuatro reglas `ask` nuevas (`*--filename*` y `*state-save*`, cada una en sus dos formas). Total `playwright-cli`: 2 allow, 20 ask, 3 deny, igual en la fuente y en el `.claude/settings.json` local.
- **Definición de terminado (§6).** V1 a V4. Hallazgos fuera de la lista son gaps y no cambian el veredicto, salvo que violen AC-b.
- **Residuales aceptados (§2).** Escrituras en rutas ignoradas que violan la regla (mitigación: `ask`), evasión deliberada (`sh -c`), archivos sin trackear preexistentes a la sesión y `video-start <archivo>`. No son defectos.

## Entorno y comandos

| Componente | Valor |
| --- | --- |
| Host | WSL2, kernel 6.18.40.1-microsoft-standard-WSL2 |
| Node / npm | v24.21.0 / 11.19.0 |
| Go | go1.27.2 linux/amd64 |
| Python | 3.14.4 (ejecutado con `-I`) |
| Git | 2.53.0 |
| Shell | `/bin/sh` = dash (`/bin/dash`); bash para los bloques |
| Playwright CLI | `@playwright/cli@0.1.22` con `--save-dev --save-exact`, en `<TMP>/proj` (fuera del repo) |
| Navegador | Chromium de la caché `~/.cache/ms-playwright` (chromium-1248) |
| Servidor de prueba | `python3 -I -m http.server 8765 --bind 127.0.0.1` sobre `<TMP>/site`. Página sintética (heading "pending", botón "Confirmar", `console.error`). HTTP 200. Detenido al cierre |

**Comandos de V1** (en `src/cli` o en la raíz, según corresponda):

```
go test ./... -count=1                                                  # rc=0, 4 paquetes con pruebas OK
sdd-cli validate --dir <REPO> --json                                    # valid true, 1192/1192
diff -q .sdd/procedures/verify-ui.md src/.sdd/procedures/verify-ui.md   # IDENTICAL
diff -q .sdd/procedures/verify.md src/.sdd/procedures/verify.md         # IDENTICAL
diff -rq .claude/skills/verify-ui src/adapters/claude-code/.claude/skills/verify-ui   # IDENTICAL
diff .claude/agents/sdd-verifier.md src/adapters/claude-code/.claude/agents/sdd-verifier.md   # solo línea 8: model haiku vs sonnet (esperado)
```

**Comandos de V2 y V3** (temporal fuera del repo, con `git init`, `evidence/` sin ignorar y `tracked.txt` versionado):

```
npm install --save-dev --save-exact @playwright/cli@0.1.22; npx --no-install playwright-cli --version   # 0.1.22
npx --no-install playwright-cli install                                 # rc=0; .gitignore += .playwright-cli/
bash <TMP>/scripts/step1.sh                                             # bloque literal del paso 1 (líneas 81-91), rc=0
npx --no-install playwright-cli -s=playwright-verifier-e2e-tests open --config="$CFG" http://127.0.0.1:8765/index.html
npx --no-install playwright-cli -s=... snapshot / click e3 / snapshot --raw > ... / screenshot / --raw console error > ...
npx --no-install playwright-cli -s=... tracing-start / tracing-stop / video-start / video-stop / pdf    # sin argumentos
npx --no-install playwright-cli -s=... close ; ... list
bash <TMP>/scripts/step7-body.sh   y   dash <TMP>/scripts/step7-body.sh   # bloque literal del paso 7 (líneas 124-128)
```

Notas de método (registradas, no son defectos del diseño):

- Los bloques literales se extrajeron de `src/.sdd/procedures/verify-ui.md` con `sed` (se quitó la sangría de markdown y se sustituyó `<work-item-id>` por el id). En el paso 1 se agregó una línea `printf` que guarda las variables en `<TMP>/vars.env`, porque la shell de la herramienta no conserva variables entre llamadas. El paso 7 carga ese archivo antes de correr el bloque sin cambios.
- En V2, el snapshot `--raw` previo al click se escribió en `evidence/run-6/ui/` del repo (copia de trabajo) y no en `evidence/ui/` del temporal. El contenido es el mismo que el generado en el temporal.
- En V3, las fugas se provocaron con `<TMP>/scripts/v3-leaks.sh` (ejecutado con `bash`), según la indicación del plan para no depender del prompt de permisos. Por eso V3 no ejercita en vivo la regla `ask` de `--filename` y `state-save`. Eso queda cubierto por V4.
- En V2, `install` (regla `ask`) se ejecutó sin prompt. El `install` estaba autorizado por la briefing y se hizo solo en el temporal.

## Matriz de trazabilidad

### Criterios de aceptación (plan §2)

Cadena: Criterio → Tarea del plan → Cambio implementado → Check (run-6) → Evidencia → Estado.

| AC | Tarea del plan | Cambio implementado | Check (run-6) | Evidencia | Estado |
| --- | --- | --- | --- | --- | --- |
| AC-a: toda la evidencia en `evidence/ui/` sin recolección | T-R3.1 | Paso 1 con config `outputDir`; sin `--filename` | V2: screenshot, snapshot, consola, tracing, video, pdf y `state-save` (no usado en V2) caen en `evidence/ui/`. `.playwright-cli/` no se creó. 7b rc=0 con dash y bash | `ui/smoke-transcript.log`, `ui/v2-artifacts-listing.txt`, `ui/v2-step7-bash.log`, `ui/v2-step7-dash.log` | Pass |
| AC-b: ningún estado (cookies o tokens) en evidencia ni en el repo sin intervención humana | T-R3.1, T-R3.2 | 7a borra `storage-state-*.json`; `ask` en `state-save`; estado nunca citado | V3: `state-save` sin argumento cae en `evidence/ui/` y 7a lo borró (0 restantes). `state-save sub/auth.json` (relativo) fue detectado por 7b y resuelto. Repo real: sin archivos de estado (mismas 16 entradas de `git status` que el snapshot inicial; `find` sin resultados). Ningún `*.json` de estado en `run-6/` | `ui/v3-leaks-transcript.log`, `ui/v3-step7-*.log`, `ui/v4-live-permissions.log` | Pass (con advertencia: la barrera `ask` no se observó en V4; AC-b depende del procedimiento y de 7a/7b) |
| AC-c: el cierre detecta cualquier cambio visible para Git, con código confiable que falla cerrado | T-R3.1 | 7b con `git status` antes/después, pathspec que excluye `evidence/` del work item, `diff` | V3: 7b rc=1 con `" M tracked.txt"`, `"?? sub/auth.json"` y `"?? \"ñ leak.png\""` (raíz, subcarpeta y no ASCII), con dash y bash. Tras resolver, rc=0 con ambos. Sin `GIT_BEFORE`, rc=2 con ambos | `ui/v3-step7-leaks-*.log`, `ui/v3-step7-resuelto-*.log`, `ui/v3-step7-sin-before-*.log` | Pass |

### Requisitos R1–R13 (estado vigente)

Base: `evidence/run-6/` para lo repetido en esta corrida. Lo no repetido cita la corrida vigente; el código de esa parte no cambió en la iteración 6.

| ID | Criterio | Tarea del plan | Check y evidencia | Estado |
| --- | --- | --- | --- | --- |
| R1 | Capability agnóstica que explica el uso de Playwright CLI | T1.1, T1.2 | Procedimiento y embebido idénticos (V1). Registry sin cambios en la iteración 6; última comprobación de identidad en `run-5/contract/sync-checks.log` | Pass |
| R2 | Skill del adapter thin que delega en el procedimiento | T2.1, T3.2 | `diff -rq` de la skill sin diferencias (V1). `go test` OK, incluido `TestClaudeCodeSkillsReferenceRegisteredCapabilities` (`run-6/cli/go-test.log`) | Pass |
| R3 | Instalación con versión fija, navegador, dependencias e `install --skills` | T1.1 (Preparación 1 a 6) | `--version` = 0.1.22; `install` rc=0 (V2). `install --skills`, prueba de arranque y `install-browser --with-deps` no se repitieron: vigentes en `run-3/ui/22-prep-check-step6.log` y `run-3/ui/24-install-skills-tmp.log`; dry-run de `install-browser` en `run-1` | Pass (parcial, lo no repetido es vigente) |
| R4 | Referenciar, no copiar, la skill y el `--help` | T1.1, T2.1 | Sin cambios en la iteración 6. Vigente: `run-2/ui/02-prep-skills-and-check.log` | Pass (vigente, corrida 2) |
| R5 | Sesiones nombradas, snapshot y refs, evidencia en `evidence/`, cierre | T1.1 (Ejecución 1 a 6) | `open --config`, `snapshot` (refs e2, e3), `click e3` con cambio verificado ("pending" a "done"), `screenshot`, `console`, `tracing`, `video`, `pdf`, `close`, `list` (V2) | Pass |
| R6 | Integración con `verify` y `sdd-verifier`; cita de evidencia | T1.3, T2.2, T-R3.3 | `verify.md` idéntico a la fuente (V1). Bullet de `sdd-verifier.md` idéntico a la fuente salvo `model` (V1). El bullet exige sin `--filename`, sin `state-save`/`state-load`, `--raw >` con ruta absoluta y cierre en el paso 7 | Pass |
| R7 | Permisos y seguridad: `.gitignore`, solo ambientes locales, sin credenciales | T2.3, T1.1 (Reglas de seguridad) | `.gitignore` de `install` agrega `.playwright-cli/` (V2). `.sdd/work-items/*/*/evidence/` ignorado en el repo (`.gitignore:41`, `check-ignore` de `run-6/`). Sitio sintético, sin credenciales. Reglas `ask` presentes (V1), pero no observadas en vivo (V4) | Bloqueada (parcial: la parte de enforcement de `ask` está pendiente de V4) |
| R8 | Tests de consistencia y comandos de validación | T3.1, T3.2, T3.3 | `go test ./... -count=1` rc=0 en 4 paquetes; `validate` valid true, 1192/1192 (V1) | Pass |
| R9 | Docs mínimas: CHANGELOG y workflow | T4.1, T4.2 | Sin cambios en la iteración 6. Búsqueda de referencias al mecanismo anterior (`7c`, `.session-start`, `leaks.txt`, `-newer`, `mv -n`) en `docs/`, `CHANGELOG.md`, `AGENTS.md` y procedimientos: sin resultados. Vigente: `run-2/contract/e5-sync-and-text-checks.log` | Pass |
| R10 | Evidencia de `playwright-cli` generada en `evidence/` por defecto | T1.1 (Ejecución 5 y paso 7) | Todo en `evidence/ui/` (V2). Las fugas relativas se detectan y no se mueven (V3) | Pass |
| R11 | Reglas de seguridad reflejadas en `settings.json` o con motivo explícito | T2.3, T-R3.2 | Las 4 reglas `ask` nuevas están en la fuente y en el local (conteo en V1). Su aplicación en vivo no se observó (V4) | Bloqueada (parcial: V4) |
| R12 | Investigación trackeada en `.sdd/research/`; `AGENTS.md` deja de prohibir work items | T4.3, T4.4 | `AGENTS.md` sin cambios en la iteración 6. El archivo `.sdd/research/playwright-cli-para-sdd.md` existe, pero aparece como `??` en `git status`: falta el commit, que decide la persona usuaria. Vigente: `run-2/contract/e5-sync-and-text-checks.log` | Pass (con observación: pendiente de commit humano) |
| R13 | Skills y procedimientos usables en este repo (sync `src/` a `.claude/` y `.sdd/`) | T-R3.5 | Procedimiento, skill y verifier sincronizados (V1). Embebido `src/cli/embeds/default_sdd/procedures/verify-ui.md` idéntico a la fuente (V1) | Pass |

### Hechos del CLI confirmados en esta corrida

| Hecho | Descripción | Resultado (run-6) |
| --- | --- | --- |
| H1 | `outputDir` de `open --config` aplica a toda la sesión | Confirmado: snapshot de `open`, `screenshot`, consola, `state-save` sin argumento, tracing, video y pdf caen en `evidence/ui/` |
| H2 | `--config` reemplaza, no suma | Confirmado: el config del work item es copia completa con `outputDir` (paso 1) |
| H3 | `--filename` y `state-save <relativo>` se resuelven contra el cwd | Confirmado: `./ñ leak.png` en la raíz del temporal y `sub/auth.json` relativo al cwd |
| H4 | `snapshot --raw` devuelve solo el árbol | Confirmado: `v2-01-snapshot-antes.yml` y `v2-02-despues-click.yml` |
| H5 | `git status` con `--untracked-files=all` detecta archivos nuevos y modificados, con nombres no ASCII y en subcarpetas; código 0/1/2 confiable en dash y bash | Confirmado (V2 y V3) |

## Resultados V1–V4

| Check | Descripción | Resultado | Evidencia |
| --- | --- | --- | --- |
| V1 | Regresión: `go test`, `validate`, `diff` de sincronización | **Pass** | `run-6/cli/go-test.log` (rc=0); `run-6/contract/validate-output.txt`, `validate-summary.txt` (1192/1192); `run-6/contract/sync-checks.log` |
| V2 | Smoke feliz en temporal, bloques literales de los pasos 1 y 7 en `dash` y `bash` | **Pass** | `run-6/ui/smoke-transcript.log`; `run-6/ui/v2-artifacts-listing.txt`; `run-6/ui/v2-artifacts/`; `run-6/ui/v2-step7-bash.log` (STEP7_RC=0); `run-6/ui/v2-step7-dash.log` (STEP7_RC=0); `run-6/ui/scripts/step1.sh`, `step7-body.sh` |
| V3 | Fugas: `screenshot --filename`, `state-save` relativo, `state-save` sin argumento, archivo trackeado modificado; `GIT_BEFORE` ausente | **Pass** | `run-6/ui/v3-leaks-transcript.log`; `run-6/ui/v3-step7-leaks-{bash,dash}.log` (STEP7_RC=1 con las tres fugas); `run-6/ui/v3-step7-resuelto-{bash,dash}.log` (STEP7_RC=0); `run-6/ui/v3-step7-sin-before-{bash,dash}.log` (STEP7_RC=2); `run-6/ui/scripts/v3-leaks.sh` |
| V4 | Permisos en vivo (S5 / G-2): `-s=` delante del subcomando con `ask` | **No cumplido: Blocked** (ningún prompt apareció; el usuario no tuvo ocasión de rechazar) | `run-6/ui/v4-live-permissions.log` |

### V1 (detalle)

- `go test ./... -count=1`: `sdd-cli/cmd`, `internal/domain`, `internal/infra`, `internal/usecases` con `ok`. rc=0.
- `sdd-cli validate --dir <REPO> --json`: valid true, 1192/1192, sin fallos ni advertencias.
- Sincronización: procedimiento raíz vs `src/` idénticos; `verify.md` idéntico; skill `verify-ui` idéntica (`diff -rq`); embebido idéntico; `sdd-verifier` raíz vs fuente solo en `model` (haiku local, sonnet fuente, esperado por la persona usuaria).
- Permisos: `.claude/settings.json` local y fuente tienen igual número de reglas `playwright-cli` (allow 2, ask 20, deny 3).

### V2 (detalle)

| Paso | Comando | Resultado observado |
| --- | --- | --- |
| Instalación | `npm install --save-dev --save-exact @playwright/cli@0.1.22`; `--version` | 0.1.22 |
| Preparación 2 | `install` | rc=0. Creó `.playwright/cli.config.json` y agregó `.playwright-cli/` al `.gitignore` |
| Paso 1 | `bash step1.sh` (literal) | rc=0. `GIT_BEFORE` capturado tras `install` |
| Ejecución 2 | `open --config="$CFG" http://127.0.0.1:8765/index.html` | rc=0. Snapshot y consola automáticos en `evidence/ui/` |
| Ejecución 3 | `snapshot` | `heading "pending" [e2]`, `button "Confirmar" [e3]` |
| Ejecución 3 | `click e3` (`getByRole('button', {name: 'Confirmar'})`) | rc=0. Título "done" |
| Ejecución 3 y 5 | `snapshot --raw > ...` | `heading "done" [e4]`: cambio de estado verificado |
| Ejecución 5 | `screenshot`; `--raw console error > ...` | rc=0. Consola: 2 errores (el `console.error` de la página y el 404 de `favicon.ico`) |
| Ejecución 5 | `tracing-start`; `tracing-stop`; `video-start`; `video-stop`; `pdf` | rc=0 en todos. Creados `traces/`, `.webm` y `.pdf` en `evidence/ui/` |
| Ejecución 6 | `close`; `list` | "Browser ... closed"; `list`: "(no browsers)" |
| Paso 7 | `bash step7-body.sh` y `dash step7-body.sh` (literales) | STEP7_RC=0 con ambos. `git-status.after` sin entradas de `evidence/`. `.playwright-cli/` no existe |

### V3 (detalle)

| Paso | Qué hizo | Resultado |
| --- | --- | --- |
| Paso 1 de nuevo | `step1.sh` | rc=0 |
| Sesión `v3` | `open --config`, `screenshot --filename="ñ leak.png"`, `state-save sub/auth.json`, `state-save` (sin argumento), `echo >> tracked.txt`, `close` | Todos rc=0. Capturas: `./ñ leak.png`; estado: `sub/auth.json` (relativo) y `evidence/ui/storage-state-*.json` |
| 7 con fugas (bash y dash) | 7a y 7b | 7a borró `storage-state-*.json` (0 restantes). 7b: `" M tracked.txt"`, `"?? sub/auth.json"`, `"?? \"ñ leak.png\""`, rc=1 con ambos |
| Resolución | `rm sub/auth.json`, `rmdir sub`, `rm "ñ leak.png"`, `git checkout -- tracked.txt` (solo en el temporal) | Aplicada |
| 7 resuelto (bash y dash) | 7a y 7b | rc=0 con ambos |
| Estado en evidencia | `find evidence -name '*.json'` | Solo `playwright-cli.config.json` (config del work item, sin estado) |
| `GIT_BEFORE` ausente (bash y dash) | Se movió `git-status.before` y se corrió 7 | `diff: No such file or directory`, rc=2 con ambos. Archivo restaurado después |

### V4 (detalle)

| Caso | Comando (desde la herramienta Bash, en el temporal, sin script) | Esperado | Observado |
| --- | --- | --- | --- |
| V4a | `npx --no-install playwright-cli -s=v4 screenshot --filename=/tmp/v4.png` | Prompt; el usuario rechaza | Sin prompt. Ejecutado. Salida "The browser 'v4' is not open, please run open first"; rc=1. No escribió nada |
| V4b | `npx --no-install playwright-cli -s=v4 state-save` | Prompt; el usuario rechaza | Sin prompt. Ejecutado. Salida "The browser 'v4' is not open, please run open first"; rc=1. No escribió estado |
| Sonda de control | `git remote -v` en el temporal (regla `ask` `Bash(git remote *)`, solo lectura) | Prompt | Sin prompt. rc=0 |

Interpretación. Los comandos no escribieron nada porque no había sesión abierta, así que el resultado no prueba la regla en sí. Lo que sí queda probado: la regla `ask` no mostró prompt ni para `playwright-cli` ni para una regla ajena. Por eso S5 / G-2 quedan **sin validar**.

### V4 (repetición en modo default, 2026-10-22)

Causa del Blocked: la sesión estaba en **modo auto**, que resuelve las reglas `ask` sin prompt. Lo confirmó el contexto de la sesión del orquestador. La persona usuaria pasó Claude Code a modo default y el orquestador ejecutó los comandos desde la herramienta Bash, en la raíz del repo, donde `@playwright/cli` no está instalado. Así, una aprobación accidental no tendría efecto.

| Caso | Comando | Observado | Resultado |
| --- | --- | --- | --- |
| V4a (intento 1) | `npx --no-install playwright-cli -s=v4 screenshot --filename=/tmp/v4.png` | **Prompt mostrado.** La persona usuaria lo aprobó por error y lo confirmó en el chat. Se ejecutó y falló sin efecto (`npm error could not determine executable to run`): no escribió nada | Prompt confirmado |
| V4a (intento 2) | Mismo comando | **Prompt mostrado y rechazado** por la persona usuaria | Pass |
| V4b | `npx --no-install playwright-cli -s=v4 state-save` | **Prompt mostrado y rechazado** por la persona usuaria | Pass |

Conclusión: las reglas `ask` `Bash(npx --no-install playwright-cli *--filename*)` y `Bash(npx --no-install playwright-cli *state-save*)` se aplican con `-s=<sesión>` delante del subcomando. **G-R6-1, G-R6-6 y G-2 quedan cerrados.** G-6 pasa a residual mitigado: la regla `ask` se observó en vivo. Evidencia: esta tabla, más el registro de la conversación del orquestador; no hay archivos, porque los comandos fueron rechazados o no tuvieron efecto.

## Gaps (no cambian el veredicto, salvo que afecten AC-b)

| ID | Valor | Gap | Estado | Recomendación |
| --- | --- | --- | --- | --- |
| G-R6-1 | Alto (bloquea V4) | La regla `ask` de `--filename` y `state-save` no mostró prompt en esta sesión (ni `install`, ni una regla ajena). S5 / G-2 sin validar | Abierto, bloquea el veredicto | Repetir V4 en una sesión con permisos interactivos, con rechazo humano de ambos prompts. Si no aparece prompt, abrir defecto de enforcement |
| G-R6-2 | Medio | 7b excluye `evidence/` del work item por pathspec. Una escritura absoluta a `$WI/evidence/` fuera de `ui/` no se detecta por 7b. Solo cubre 7a (nombres `storage-state-*.json`). La regla pide rutas absolutas fuera del proyecto, así que no es un caso permitido. No está en §2 como residual | Abierto | Declarar el caso en el procedimiento o agregar `*.json` de `evidence/ui/` a 7a (ya señalado como G-R5-5) |
| G-R6-3 | Bajo | Durante la verificación de `state-save` absoluto, quién borra el archivo sigue sin definirse (G-R5-4) | Abierto | Indicar el comando de borrado al cerrar la sesión |
| G-R6-4 | Bajo | 7b devuelve 1 mientras haya archivos reportados que legítimamente deben quedar (G-R5-1). El procedimiento no define cómo cerrar ese caso sin borrar el marcador | Abierto, sin cambio en la revisión 3 | Definir cómo se aceptan archivos reportados o declarar que rc=1 es "requiere revisión humana" |
| G-R6-5 | Informativo | En un proyecto sin `.gitignore` para `node_modules/`, `GIT_BEFORE` lista miles de líneas. El control funciona, pero el archivo es grande | Informativo | Documentar la condición |
| G-R6-6 | Medio (no probado) | Las reglas `ask` no se verifican en una sesión de agente real con permisos interactivos (esto es V4) | Abierto | Ver G-R6-1 |
| G-R6-7 | Bajo | V3 provocó las fugas con script `.sh`, no en vivo. La regla `ask` de `--filename` y `state-save` no se ejercitó en V3 | Registrado como método | Cubierto por V4 |
| G-R6-8 | Informativo | `.sdd/research/playwright-cli-para-sdd.md` sin commit (`??`) | Pendiente de commit humano | La persona usuaria decide el commit (R12) |
| G-2 | Medio (arrastrado) | S5: `ask` con `-s=` delante del subcomando | Abierto, ver G-R6-1 | V4 |
| G-1 | Medio (arrastrado) | El procedimiento no tiene test automatizado de sus cláusulas (paso 7, marcador, código de salida) | Abierto | Test de contrato sobre el paso 7 con dash y bash, incluidos `GIT_BEFORE` ausente y el caso con `evidence/` |
| G-4 | Medio (arrastrado) | Comportamiento de `sdd-verifier` con `Bloqueada` no ejecutado en sesión de agente | Abierto, fuera de V1–V4 | Prueba en sesión con Playwright no preparado |
| G-6 | Bajo (arrastrado) | `allow Bash(playwright-cli *)` permite `state-save` y `screenshot --filename` relativos sin confirmación; hoy los frena solo la regla `ask` (no observada) y el procedimiento | Abierto, residual | Mismo criterio que G-R6-1 |
| G-8 | Informativo (arrastrado) | Conteo de `validate`: 1117 (corrida 5) y 1192 (corrida 6), sin fallos | Informativo | Revisar si el crecimiento viene de archivos del work item |
| G-12 | Informativo | `sdd-verifier` local con `model: haiku`, fuente `sonnet` | Cerrado por decisión de la persona usuaria | Ninguna |

Observaciones que ya no aplican por el rediseño de la revisión 3: G-R5-2 (carpetas vacías tras `mv`), G-R5-3 (lazo de resolución con `mv`), N-5 (colisión de `mv -n`), N-6 (7b con `find` fallido) y N-7 (`node_modules` podado). El mecanismo que los originaba se eliminó (`leaks/`, `mv -n`, `find -newer`, marcador `.session-start`). Su cierre se valida por diseño, no por corrida.

**Pasos sugeridos antes de `human-code-review`.**

1. Persona usuaria: repetir V4 en sesión con permisos interactivos (rechazar ambos prompts). Es la única condición pendiente para Passed.
2. Orquestador: decidir si G-R6-2 y G-R6-4 se corrigen antes de cerrar o se aceptan como residuales explícitos.
3. Orquestador: considerar un test de contrato para el paso 7 (G-1) como mejora posterior, no como condición de esta fase.

## Historial de corridas (1 a 5)

| Corrida | Veredicto | Defectos | Qué cambió antes de la siguiente corrida |
| --- | --- | --- | --- |
| 1 | Failed | D-1 (crítico: `auth.json` terminaba en `evidence/`), D-2 (prueba de arranque ausente), D-3 (documentación excesiva) | Lista blanca sin `*.json`, rutas absolutas para `state-save`, prueba de arranque, documentación reducida |
| 2 | Failed | D-4 (alto: `--filename` y `state-save` relativos caían en la raíz del repo, fuera de `.gitignore`), D-5 (bajo: `storage-state-*.json` en `.playwright-cli/`) | Procedimiento describe el comportamiento real; paso 7 dividido en 7a, 7b y 7c |
| 3 | Passed | Ninguno bloqueante para los casos probados. Cerró D-4 y D-5. Observaciones G-9 a G-11 | `outputDir` explícito por sesión, prohibición de rutas relativas, 7a borra estado, 7b por Git con pathspec y marcador |
| 4 | Failed | N-1 (alto acotado: nombres no ASCII escapan 7b), N-2 (medio: carpetas ignoradas escapan 7b), N-3 (medio: 7b no repetible), N-4 (bajo: código de salida no confiable), N-5 (bajo-medio: colisión de `mv -n`) | 7b con `find`, poda, código de salida explícito, marcador que se borra al final, `leaks/<ruta original>` |
| 5 | Failed | N-6 (alto, condicional: 7b da 0 con `find` fallido), N-7 (medio: `node_modules/` podado sin declarar) | Pendiente: corrección de 7b. Se decidió rediseñar en lugar de parchear |

**Por qué se re-planificó (revisión 3).** Cinco corridas, cuatro Failed, con el mismo punto de fallo: la red de seguridad del paso 7 (recolectar y detectar fugas después de generarlas). Cada corrección abrió casos borde nuevos, y la corrida 3 "Passed" no había probado nombres no ASCII ni carpetas ignoradas. Causas raíz registradas en el plan (CR1 a CR4): se diseñó detección en vez de prevención, la red es shell embebido sin pruebas, un hecho del CLI se asumió sin ejecutarlo (P6) y no había definición de terminado. La revisión 3 eligió prevenir (`outputDir` explícito, sin `--filename` ni `state-save`) y dejar a Git solo como control mínimo, con una definición de terminado cerrada (§6).

## Evaluación de la desviación respecto de la revisión 2

La revisión 2 pedía que `state-save` y la evidencia nunca salieran de `evidence/`, mediante recolección y movimiento posterior. Las iteraciones 2 a 5 la implementaron así y fallaron en cada corrida. La revisión 3 cambia el mecanismo a prevención. Es una desviación de la revisión 2 y queda formalizada en la revisión 3 aprobada (decisiones R3-D1 a R3-D3).

- **Lo que se verificó en esta corrida.** La prevención funciona en el camino del procedimiento: la evidencia cae en `evidence/ui/` (V2), y el control por Git detecta fugas relativas, en subcarpetas, no ASCII y en archivos trackeados, con código de salida confiable en dash y bash (V3). La exclusión de `evidence/` por pathspec (ajuste de la iteración 6, fuera del texto literal del plan §4) evita falsos positivos: en V2, el control dio 0 con `evidence/` sin ignorar.
- **Lo que no se verificó.** La barrera de permisos (`ask`), que la revisión 3 usa como mitigación de los residuales de §2 para `--filename` y `state-save`. Sin esa barrera, la protección de estado depende de que el agente siga el procedimiento, más 7a y 7b como detección.
- **Conclusión.** La desviación respecto de la revisión 2 es aceptable como diseño, porque cumple AC-a, AC-b y AC-c en lo verificado. Su cierre depende de V4. Este verificador no decide sobre la aceptación de los residuales de §2; eso lo decide la persona usuaria.

## Evidencia

Base: `.sdd/work-items/active/playwright-verifier-e2e-tests/evidence/`. Sanitizada: `<TMP>`, `<REPO>` y `<HOME>`. No contiene ningún `*.json` de estado (los `storage-state-*.json` de V2 y V3 se borraron por 7a antes de copiar nada; `validate-output.json` duplicado se eliminó). `evidence/run-6/` está ignorado por `.gitignore:41`.

- `run-6/cli/go-test.log`: `go test ./... -count=1` (rc=0).
- `run-6/contract/sync-checks.log`: diferencias de sincronización (V1).
- `run-6/contract/validate-output.txt`, `validate-summary.txt`: `sdd-cli validate` (1192/1192).
- `run-6/ui/smoke-transcript.log`: transcripción sanitizada de V2.
- `run-6/ui/v2-artifacts/`: artefactos de V2 (snapshots `page-*.yml`, consola `console-*.log`, `v2-console-error.txt`, `v2-02-despues-click.yml`, captura `page-*.png`, `page-*.pdf`, `video-*.webm`). Los `traces/` no se copiaron; su listado está en `run-6/ui/v2-artifacts-listing.txt`.
- `run-6/ui/v2-01-snapshot-antes.yml`: snapshot raw previo al click.
- `run-6/ui/v2-step7-{bash,dash}.log`: salida del paso 7 en V2.
- `run-6/ui/v3-leaks-transcript.log`: transcripción sanitizada de V3.
- `run-6/ui/v3-step7-{leaks,resuelto,sin-before}-{bash,dash}.log`: salidas del paso 7 en V3.
- `run-6/ui/v4-live-permissions.log`: transcripción de V4 y de la sonda de control.
- `run-6/ui/config-summary.txt`: resumen del config del work item (claves, sin JSON completo).
- `run-6/ui/git-status-{before,after}.count.txt`: número de líneas de los snapshots de Git de V2.
- `run-6/ui/scripts/step1.sh`, `step7-body.sh`, `v3-leaks.sh`: bloques literales extraídos de `src/.sdd/procedures/verify-ui.md` y el script de fugas (rutas sanitizadas).

**Corridas previas (sin modificar)**

- `evidence/run-5/`: corrida 5. Relevantes para R1 y R9: `contract/sync-checks.log`, `contract/checks-run5-extra.log`.
- `evidence/run-3/`: corrida 3. Relevantes para R3 y R7: `ui/22-prep-check-step6.log`, `ui/24-install-skills-tmp.log`, `ui/15` (`state-save` absoluto fuera del proyecto).
- `evidence/run-2/`: corrida 2. Relevantes para R4, R9 y R12: `ui/02-prep-skills-and-check.log`, `contract/e5-sync-and-text-checks.log`.
- `evidence/cli`, `evidence/contract`, `evidence/ui`: corrida 1.

## Limpieza

Temporal `<TMP>` borrado. Servidor de prueba detenido (puerto 8765 sin respuesta). Sesiones `playwright-verifier-e2e-tests` y `v3` cerradas. `list` sin navegadores. Sesión `v4` nunca se abrió. Ningún proceso del CLI residual. En el repo: `git status` con las mismas 16 entradas que al inicio de la conversación, más `evidence/run-6/` (ignorado). No se modificó código ni docs del repo, no se usó `sdd-cli` mutante y no se hizo commit.
