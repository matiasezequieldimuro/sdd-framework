---
schema_version: "1.0"
kind: artifact
id: "implementation-report"
work_item: "playwright-verifier-e2e-tests"
phase: "implementation"
status: draft
created_at: "2026-10-22T14:54:44Z"
created_by: { kind: "cli", id: "sdd" }
sources: 
  - artifacts/plan.md
---

# Reporte de implementación: Integrar Playwright CLI + skills para verificación de UI

Implementado por `sdd-orchestrator` (inline, a pedido del usuario) sobre el plan aprobado
(revisión 2). Etapas E1 a E4 completas. **E5 (sincronización a la raíz) queda pendiente del
usuario**: las rutas destino están en `deny` para agentes.

## Iteración 6: implementación del plan revisión 3 (vigente)

Implementa la sección "Revisión 3" de `artifacts/plan.md`, aprobada por el usuario: opción B,
prevenir en lugar de recolectar. **Reemplaza** el mecanismo de evidencia y fugas de las
iteraciones 2 a 5. Esas secciones quedan abajo como historial.

| Tarea | Archivo | Cambio |
| --- | --- | --- |
| T-R3.1 | `src/.sdd/procedures/verify-ui.md` | Ejecución reescrita. Paso 1: `WI_REL`/`WI`, `EVIDENCE_DIR`, `CFG` (copia + `outputDir`) y `GIT_BEFORE` con `git status --porcelain=v1 --untracked-files=all`, que excluye la `evidence/` del work item por pathspec. Paso 5: evidencia sin argumentos; snapshot y consola con `--raw >` a ruta absoluta; prohibidos `--filename` y `state-save`/`state-load` salvo aprobación humana. Paso 7: 7a borra `storage-state-*.json`; 7b hace `git status` después + `diff` (0 sin cambios, 1 hay, 2 falta `GIT_BEFORE`). Reglas de seguridad con los residuales declarados. Se eliminaron `.session-start`, `find -newer`, `leaks.txt`, `leaks/`, `mv -n` y la lista blanca. |
| T-R3.2 | `src/adapters/claude-code/.claude/settings.json` | +4 `ask`: `*state-save*` y `*--filename*`, cada una con las dos formas. Total `playwright-cli`: 2 allow, 20 ask, 3 deny. |
| T-R3.3 | `src/adapters/claude-code/.claude/agents/sdd-verifier.md` | Bullet: no usar `--filename`, `state-save` ni `state-load`; `--raw >` con ruta absoluta; cierre en el paso 7. |

**Ajuste durante la implementación (dentro del alcance):** el `git status` del paso 1 y del 7b
excluye `$WI_REL/evidence` por pathspec. Sin esa exclusión, en un proyecto que no ignora
`evidence/`, el control marcaría la propia evidencia como cambio (falso positivo
sistemático).

**Matriz T2.3 actualizada (reglas ↔ `settings.json`):**

| Regla | Representación |
| --- | --- |
| `--filename` requiere confirmación | `ask` `*--filename*` (2 formas) |
| `state-save`/`state-load` requieren confirmación | `ask` `*state-save*`, `*state-load*` (2 formas cada una) |
| El estado nunca va a evidencia | 7a (borra `storage-state-*`) + `ask` en `state-save` |
| Detectar fugas | 7b (`git status` + `diff`); no representable como permiso |
| Resto (código arbitrario, navegador real, preparación, paquete legacy) | Sin cambios respecto de la revisión 2 |

**Autoprueba T-R3.4** con la CLI real (`@playwright/cli@0.1.22`, temporal fuera del repo con
`git init` y `evidence/` **sin** ignorar). Los bloques de los pasos 1 y 7 se extrajeron
literalmente del procedimiento.

| Check | Resultado |
| --- | --- |
| V2: `open --config`, `click` (título → "done"), `screenshot`, `snapshot --raw >`, `--raw console error >`, `tracing-start/stop`, `video-start/stop`, `pdf` | Todo en `evidence/ui/`: snapshots, png, log de consola, `traces/`, `.webm` y `.pdf`. `.playwright-cli/` no se creó. 7b código 0 en `dash` y `bash`. |
| V3: `screenshot --filename="ñ leak.png"`, `state-save sub/auth.json`, `state-save` sin argumento, `tracked.txt` modificado | 7a borró el `storage-state-*`. 7b código 1 en `dash` y `bash`, listando exactamente ` M tracked.txt`, `?? sub/auth.json` y `?? "ñ leak.png"`. Después de resolver, código 0. Sin `GIT_BEFORE`, código 2. Único `*.json` en evidencia: el config del work item. |
| `go generate`, `go vet`, `go test ./... -count=1`, `go build` | OK |
| `init` + `adapters install` + `validate` en un dir temporal | `valid: true`; `verify-ui.md` y `settings.json` embebidos idénticos a la fuente |

**Pendiente del usuario (T-R3.5):** sync de `verify-ui.md` y `sdd-verifier.md` (con `model:
haiku`) y merge de las 4 reglas `ask` nuevas en `.claude/settings.json` local.

## Iteración 5 (rework tras verificación corrida 4 Failed)

La corrida 4 encontró huecos en el control 7b: N-1 (nombres no ASCII no detectados), N-2
(no ve carpetas ignoradas, p. ej. `.playwright-cli/`), N-3 (no repetible, porque borraba el
marcador), N-4 (código de salida no confiable) y N-5 (colisión silenciosa de `mv -n`). El
usuario aprobó corregirlo.

Cambio en `src/.sdd/procedures/verify-ui.md`, paso 7b: deja de depender de Git y usa `find`
con poda de `.git`, `node_modules` y la `evidence/` del work item:

```bash
LEAKS="$(git rev-parse --show-toplevel)/$WI_REL/evidence/leaks.txt"
find . \( -name .git -o -name node_modules -o -path "./$WI_REL/evidence" \) -prune \
  -o -type f -newer "$START" -print > "$LEAKS"
cat "$LEAKS"; [ ! -s "$LEAKS" ]
```

| Hueco | Resolución |
| --- | --- |
| N-1 | `find` imprime los nombres sin comillas ni escapes. |
| N-2 | Recorre también las carpetas ignoradas por Git. |
| N-3 | El marcador se borra recién cuando 7b solo lista lo reportado para revisión humana. Hasta entonces el control se puede repetir. |
| N-4 | El código de salida es explícito (`[ ! -s "$LEAKS" ]`): 0 = sin fugas, 1 = hay fugas. El listado queda en `evidence/leaks.txt`. |
| N-5 | La evidencia filtrada se mueve a `evidence/ui/leaks/<ruta original>`. No colisiona y deja registro del origen. |

Trade-off: 7b también lista lo que la app, el dev server o el build escriban durante la
sesión (p. ej. `dist/`). Se reporta para revisión humana y no se borra. Es preferible un falso
positivo a un falso negativo.

**Prueba con la CLI real** (temporal con `git init`; los bloques de los pasos 1 y 7 se
extrajeron literalmente del procedimiento y 7 corrió con `dash`). Flujo `open --config` +
`click` (título → "done"). Fugas provocadas: `ñ-leak.png`, `.playwright-cli/ign-leak.png`,
`sub/leak.png`, `leak.png`, `state-save sub/auth.json`, `state-save auth.json` y
`state-save` sin argumento.

- **7b, primera corrida:** listó las 6 fugas y salió con código 1. 7a borró
  `storage-state-*.json`.
- **Resolución según el procedimiento:** se borraron los dos `auth.json` y las 4 capturas
  quedaron en `evidence/ui/leaks/` con su ruta original (incluidas `ñ-leak.png` y
  `.playwright-cli/ign-leak.png`).
- **7b, segunda corrida:** salió con código 0, así que es repetible.
- **Resultado:** `git status` del temporal quedó sin archivos fuera de `evidence/`.

| Check | Resultado |
| --- | --- |
| `go generate`, `go vet`, `go test ./... -count=1`, `go build` | OK |
| `init` + `adapters install` + `validate` en un dir temporal | `valid: true`; embebido idéntico a la fuente |

**Pendiente del usuario:** `cp src/.sdd/procedures/verify-ui.md .sdd/procedures/`.

## Iteración 4 (rework previo a `human-code-review`, pedido del usuario)

Pedido del usuario: la evidencia debe generarse en una **carpeta indicada explícitamente**, no
recolectarse después de varios lugares. También: cubrir subcarpetas (G-9), corregir la
redacción y las colisiones (G-11) y que `evidence/` no se versione (regla general).

Hechos confirmados con un spike de `@playwright/cli@0.1.22` en un temporal fuera del repo:

| Hecho | Resultado |
| --- | --- |
| `open --config=<cfg>` con `outputDir` | Lo aplica la CLI. Vale para toda la sesión. La salida sin `--filename` (snapshot de `open`, `screenshot`, log de consola, `state-save` sin argumento) cae en `outputDir`. No se crea `.playwright-cli/`. |
| `--config` parcial (solo `outputDir`) | **Reemplaza** `.playwright/cli.config.json`: se pierde `browser` y `open` falla ("chrome not found"). Por eso el config del work item es una copia completa con `outputDir`. |
| `--filename` relativo con `outputDir` configurado | Sigue resolviéndose contra el directorio actual: raíz o subcarpeta. |
| `snapshot` sin argumento | Imprime en pantalla y no guarda archivo. |

| Cambio | Archivo |
| --- | --- |
| Ejecución reescrita. Paso 1: `EVIDENCE_DIR`, `CFG` (copia de `.playwright/cli.config.json` + `outputDir`, generada con `node -e`) y marcador. Paso 2: `open --config="$CFG"`. Paso 5: sin `--filename` o con ruta absoluta en `EVIDENCE_DIR`; nunca relativa. Paso 7: **7a** borra `storage-state-*.json` de `EVIDENCE_DIR`; **7b** es un control con `git ls-files --others --exclude-standard` (excluye `evidence/` por pathspec y filtra con `-nt` contra el marcador). Para lo que liste 7b: evidencia con `mv -n`, estado de la sesión se borra, el resto se reporta. Se eliminan la recolección en dos lugares y `-maxdepth`. | `src/.sdd/procedures/verify-ui.md` |
| Regla de `state-save` reescrita (G-11): 7b **detecta** y borrarlo es una acción explícita. | idem, Reglas de seguridad |
| Bullet del verifier: `open --config` con `outputDir` = `evidence/ui/`, nunca `--filename` relativo, limpieza y control al cerrar. | `src/adapters/claude-code/.claude/agents/sdd-verifier.md` |
| Regla general `.sdd/work-items/*/*/evidence/`. Reemplaza las dos entradas específicas de `feat-rework-cancel`, que quedan cubiertas. | `.gitignore` (raíz) |

**Prueba con la CLI real** (temporal con `git init`, `evidence/` **sin** ignorar para probar
la exclusión por pathspec). Se siguió el procedimiento literalmente: config generado,
`open --config`, `snapshot`, `click e2` (título → "done"), `screenshot` sin filename y con
ruta absoluta, `console error`, `requests`. Casos adversariales: `screenshot
--filename=sub/leak.png`, `state-save auth.json`, `state-save` sin argumento. Luego `close` y
`list` (sin navegadores).

- Antes de 7, `evidence/ui/` tenía `AC1-abs.png`, `page-*.png`, `page-*.yml`, `console-*.log`
  y `storage-state-*.json`.
- 7a borró `storage-state-*.json`.
- 7b listó exactamente `auth.json` y `sub/leak.png`. No listó nada de `evidence/` ni el
  config.
- `.playwright-cli/` no se creó.

| Check | Resultado |
| --- | --- |
| `go generate`, `gofmt -l`, `go vet`, `go test ./... -count=1`, `go build` | OK |
| `init` + `adapters install` + `validate` en un dir temporal | `valid: true`; `verify-ui.md` y `sdd-verifier.md` embebidos idénticos a la fuente |
| `git check-ignore`: `evidence/` de active y archive ignorados; `artifacts/` no | OK |

**Desviación acumulada respecto del plan aprobado (iteraciones 2 a 4).** El mecanismo de
evidencia de DD5, P6 y la fila de `state-save` de la matriz T2.3 queda reemplazado por:
`outputDir` explícito por sesión + prohibición de rutas relativas + borrado del estado +
control con Git. `settings.json` no cambia: `open --config` ya está en `allow`, y `node -e`
pide confirmación porque no está en `allow`. **Requiere aprobación explícita en
`human-code-review`.**

**Pendiente del usuario antes de re-verificar:**
`cp src/.sdd/procedures/verify-ui.md .sdd/procedures/` y
`cp src/adapters/claude-code/.claude/agents/sdd-verifier.md .claude/agents/ && sed -i 's/^model: .*/model: haiku/' .claude/agents/sdd-verifier.md`.

## Iteración 3 (rework tras verificación corrida 2 Failed)

La corrida 2 de verificación (`evidence/run-2/ui/19`, `22`–`24`, `26`, `33`) mostró con la CLI
real que una ruta **relativa** (`screenshot --filename`, `snapshot --filename`, `state-save
<archivo>`) se resuelve contra el **directorio actual (la raíz del proyecto)** y no contra
`.playwright-cli/`. Esos archivos quedan fuera de `.gitignore`. Sin argumento, la salida va a
`.playwright-cli/`, incluido `storage-state-*.json`. El hecho P6 del plan era falso para este
uso. El usuario aprobó el rework.

| Defecto | Archivo | Corrección |
| --- | --- | --- |
| D-4 (alto) | `src/.sdd/procedures/verify-ui.md`, Ejecución 5 y 7, Reglas de seguridad | El paso 5 describe el comportamiento real y exige rutas absolutas siempre. El paso 7 ahora tiene tres partes: **7a** recolecta en `.playwright-cli/`, **7b** recolecta en la raíz con `-maxdepth 1`, ambas con la lista blanca de extensiones, y **7c** es un control que debe quedar vacío. Lo que liste 7c (p. ej. `auth.json`) no se mueve ni se commitea: si es estado de la sesión se borra; si no, se reporta para revisión humana. `state-save` debe escribirse con ruta absoluta fuera del proyecto y del work item. |
| D-5 (bajo) | Mismo archivo, paso 7a | Borra los `storage-state-*.json` creados durante la sesión en `.playwright-cli/`. |

**Desviación respecto del plan aprobado (acumulada con la de la iteración 2).** P6 y la fila
"`state-save` nunca va a `evidence/`" de la matriz T2.3 quedan reemplazados por: lista blanca
sin `*.json` + rutas absolutas obligatorias + recolección en dos ubicaciones + control 7c +
borrado del estado por defecto. Es más restrictiva que el plan y no cambia `settings.json`.
`allow Bash(playwright-cli *)` se mantiene: escribir una captura en la raíz no viola el
criterio D3 del usuario (no es código fuente) y el riesgo de credenciales lo cubre el paso 7.
**Requiere aprobación explícita en `human-code-review`.**

**Pruebas de la iteración 3** (2026-10-22):

| Check | Resultado |
| --- | --- |
| Simulación del paso 7 en un repo temporal: en `.playwright-cli/` `page-1.yml`, `page-2.png`, `trace.zip`, `storage-state-2026.json` y `session-x.json`; en la raíz `rel-check.png`, `rel-snap.yml`, `auth.json` y `my-session.json`; `site/app.log` en un subdirectorio; `README.md` y `old.png` anteriores al marcador | En evidencia quedaron los 5 de la lista blanca. Se borró `storage-state-2026.json`. 7c listó `auth.json` y `my-session.json`, que no se movieron. `session-x.json`, `README.md`, `old.png` y `site/app.log` no se tocaron. OK |
| `go generate`, `gofmt -l`, `go vet`, `go test ./... -count=1`, `go build` | OK |
| `init` + `adapters install` + `validate` en un dir temporal | `valid: true`; el embebido es idéntico a la fuente |

**Pendiente del usuario antes de re-verificar:** `cp src/.sdd/procedures/verify-ui.md .sdd/procedures/`.

## Iteración 2 (rework tras verificación Failed)

La verificación del 2026-10-22 dio **Failed** (`artifacts/verification-report.md`). El usuario
aprobó el rework de `implementation`. Correcciones:

| Defecto | Archivo | Corrección |
| --- | --- | --- |
| D-1 (crítico) | `src/.sdd/procedures/verify-ui.md`, paso 7 y Reglas de seguridad | Antes se excluía por nombre (`*state*`/`*storage*`), y un `auth.json` terminaba en `evidence/`. Ahora el paso 7 usa una **lista blanca** de extensiones (`yml`, `png`, `jpeg`, `jpg`, `webp`, `txt`, `log`, `zip`, `webm`, `pdf`) y nunca mueve `*.json`. En verificación, `state-save` se escribe con ruta absoluta fuera de `.playwright-cli/` y del work item, y se borra al terminar. |
| D-2 (medio) | `src/.sdd/procedures/verify-ui.md`, Preparación paso 6 | Agrega la prueba de arranque `-s=prep-check open about:blank` + `close`. Si `open` falla por librerías, se vuelve al paso 3. |
| D-3 (bajo) | `docs/SDD_WORKFLOW.md` § 5.4 | Queda en 7 líneas de contenido (antes 12). |

```bash
[ -d .playwright-cli ] && find .playwright-cli -type f -newer "$EVIDENCE_DIR/.session-start" \
  \( -name '*.yml' -o -name '*.png' -o -name '*.jpeg' -o -name '*.jpg' -o -name '*.webp' \
     -o -name '*.txt' -o -name '*.log' -o -name '*.zip' -o -name '*.webm' -o -name '*.pdf' \) \
  -exec mv {} "$EVIDENCE_DIR"/ \;
```

**Desviación respecto del plan aprobado.** La matriz de T2.3 del plan dice que la regla
"`state-save` nunca va a `evidence/`" se cubre con "la exclusión `*state*`/`*storage*` del paso
7". Esa mitigación resultó insuficiente (D-1). Pasa a cubrirse con la lista blanca sin
`*.json` más la ubicación obligatoria de `state-save` fuera de `.playwright-cli/`. No se edita
el plan aprobado: la desviación queda registrada aquí. Es más restrictiva que lo planificado y
no cambia `settings.json`.

**Pruebas de la iteración 2** (2026-10-22):

| Check | Resultado |
| --- | --- |
| Simulación del paso 7 con archivos falsos (`page-1.yml`, `rel.png`, `shot.jpeg`, `trace.zip`, `video.webm`, `console.log`, `doc.pdf`, `auth.json`, `my-session.json`, `storage-state.json`, `cookies.txt.json`, y un `old.png` anterior al marcador) | Se movieron los 7 de evidencia. Quedaron en `.playwright-cli/` los 4 `*.json` y `old.png`. OK |
| `go generate`, `gofmt -l`, `go vet`, `go test ./... -count=1`, `go build` | OK |
| `init` + `adapters install` + `validate` en un dir temporal | `valid: true`; el `verify-ui.md` embebido es idéntico a la fuente |
| Dependencias del navegador (las instaló el usuario con apt: `libnss3`, `libnspr4`) | `ldd` sin faltantes en `chromium-1247`, `chromium-1248` y `chromium_headless_shell-1248` |

**Pendiente del usuario antes de re-verificar:** `cp src/.sdd/procedures/verify-ui.md .sdd/procedures/`.

## Análisis previo del plan

- El plan es consistente con el código: la fuente de verdad del contrato es `src/.sdd/` y la
  del adapter es `src/adapters/claude-code/`. `go generate` los embebe (F1, F2).
- No se encontraron contradicciones que bloquearan la implementación.
- Fuentes oficiales: las que el plan ya contrastó (README y paquete publicado
  `@playwright/cli@0.1.22`, docs de permisos de Claude Code). No se agregaron decisiones
  técnicas nuevas.

## Cambios realizados

### E1. Contrato portable (`src/.sdd/`)

| Tarea | Archivo | Cambio |
| --- | --- | --- |
| T1.1 | `src/.sdd/procedures/verify-ui.md` | **Nuevo** (130 líneas). Es el borrador del plan, con un ajuste: en el paso 7, `find` tiene la guarda `[ -d .playwright-cli ] &&` para no fallar si la carpeta no existe. |
| T1.2 | `src/.sdd/registry/capabilities.yaml` | Agrega `sdd.verify-ui` después de `sdd.verify`. |
| T1.3 | `src/.sdd/procedures/verify.md` | Delegación en `sdd.verify-ui` en los puntos 5 y 6, más un bullet sobre `evidence/ui/`. |

El paso 7 usa una forma portable (sin `mv -t`, que solo existe en GNU):

```bash
[ -d .playwright-cli ] && find .playwright-cli -type f -newer "$EVIDENCE_DIR/.session-start" \
  ! -iname '*state*' ! -iname '*storage*' -exec mv {} "$EVIDENCE_DIR"/ \;
```

### E2. Adapter `claude-code` (`src/adapters/claude-code/.claude/`)

| Tarea | Archivo | Cambio |
| --- | --- | --- |
| T2.1 | `skills/verify-ui/SKILL.md` | **Nuevo.** Wrapper thin: `Capability`, `Seguir` y una línea que referencia la skill `playwright-cli` del paquete o `--help`. |
| T2.2 | `agents/sdd-verifier.md` | `skills: status, verify, verify-ui`, 2 bullets en "Live UI & Interaction Testing" (incluye el movimiento a `evidence/ui/`) y la condición de parada 4 (sesiones cerradas). `model` sin cambios. |
| T2.3 | `settings.json` | +2 `allow`, +16 `ask`, +3 `deny`, exactamente como en la tabla del plan. Se insertaron como texto para conservar el formato original del archivo. |

### E3. Tests (`src/cli/internal/infra/fs_adapter_repository_test.go`)

- T3.1: `expectedFiles` suma `.claude/skills/verify-ui/SKILL.md` y `.claude/agents/sdd-verifier.md`.
- T3.2: nuevo `TestClaudeCodeSkillsReferenceRegisteredCapabilities`, con docstring en inglés.
  Reutiliza `capabilityRegistry` y suma los imports `regexp` y `gopkg.in/yaml.v3`, que ya
  es dependencia del módulo.

### E4. Documentación y housekeeping

| Tarea | Archivo | Cambio |
| --- | --- | --- |
| T4.1 | `CHANGELOG.md` | 2 bullets en `[Unreleased]` → `Agregado`. |
| T4.2 | `docs/SDD_WORKFLOW.md` | Nueva § 5.4 "Verificación de UI (opcional)". |
| T4.3 | `.sdd/research/playwright-cli-para-sdd.md` | Movido desde la raíz sin editar. No está ignorado por Git; se trackea en el commit del work item. |
| T4.4 | `AGENTS.md` | Línea 17: este repo usa el framework para evolucionarse (dogfooding) y los cambios al motor se hacen en `./src/`. También se corrigió el typo "utilzar". |

## Decisiones relevantes o críticas

- **Implementación inline:** la hizo el orquestador y no `sdd-developer`, porque el usuario
  pidió agilidad después de la demora del planner. No cambia el alcance.
- **Formato de `settings.json`:** reserializar con `json.dumps` cambiaba el formato. Se optó
  por insertar las reglas como texto, de modo que el diff solo muestre las líneas nuevas.

## Desviaciones aprobadas del plan

Ninguna. Hay dos ajustes menores dentro del alcance, ninguno cambia comportamiento:

- Guarda `[ -d .playwright-cli ]` en el paso 7 del procedimiento.
- Corrección del typo "utilzar" en la línea modificada de `AGENTS.md`.

## Pruebas añadidas o actualizadas

| Test | Tipo | Qué protege |
| --- | --- | --- |
| `TestFSAdapterRepositoryInstallsClaudeCode` (ampliado) | Integración | Instalación de la skill `verify-ui` y del verifier. |
| `TestClaudeCodeSkillsReferenceRegisteredCapabilities` (nuevo) | Integración / contrato | Cada skill con `Capability:` apunta a una capability registrada y a su procedure, y cada procedure citado existe. |

## Pruebas realizadas

Ejecutadas el 2026-10-22 desde `src/cli`:

| Comando | Resultado |
| --- | --- |
| `go generate ./...` | OK (61 archivos de contrato, 51 de adapter) |
| `gofmt -l $(git ls-files '*.go') internal/infra/fs_adapter_repository_test.go` | Sin salida (OK) |
| `go vet ./...` | OK |
| `go test ./... -count=1` | OK: `cmd`, `internal/domain`, `internal/infra`, `internal/usecases` |
| `go test ./internal/infra -run '…InstallsClaudeCode\|…ReferenceRegisteredCapabilities' -v` | 2/2 PASS |
| `go build -o /tmp/sdd-verify-ui/sdd-cli .` | OK |
| `init` + `adapters install claude-code` + `validate` en un dir temporal | `success: true`, `valid: true`. `verify-ui.md` y `skills/verify-ui/SKILL.md` presentes. `settings.json` válido. |
| `python3 -m json.tool src/adapters/claude-code/.claude/settings.json` | OK |

No se ejecutó en esta fase (corresponde a verificación):

- El smoke real de Playwright CLI, autorizado en D6.
- `sdd-cli validate` del repo después de E5.

## Pendiente antes de verificación (E5, acción del usuario)

Los comandos exactos están en `artifacts/plan.md`, Etapa 5:

1. Copiar al contrato versionado `.sdd/` (`procedures/` y `registry/`).
2. Copiar la skill y el verifier al `.claude/` local, conservando `model: haiku`.
3. Agregar con el script del plan las reglas de `playwright-cli` al `settings.json` local.
4. Validar y reiniciar Claude Code.
