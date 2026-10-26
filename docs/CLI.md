# Referencia de la CLI (`sdd-cli`)

> Estado: **Estable** (CLI `v1.0.0`, contrato `schema_version` 1.0).
> Audiencia: usuarios y agentes que operan el motor SDD.
> Alcance: comandos disponibles, flags, ejemplos y contrato de salida.

Para entender *cuándo* usar cada comando dentro del proceso, ver
[SDD_WORKFLOW.md](./SDD_WORKFLOW.md). Para instalar el binario, ver
[GUIA_INSTALACION.md](./GUIA_INSTALACION.md).

---

## 1. Modelo mental

`sdd-cli` gobierna el proceso; no redacta contenido ni ejecuta commits/PRs por sí solo.
El agente hace el trabajo cognitivo y le informa a la CLI; la CLI valida, cambia estado
y deja evidencia. Todo el estado vive en `.sdd/` dentro del proyecto.

```text
El agente redacta el plan.
El agente ejecuta   sdd-cli deliver ... --phase plan
La CLI verifica que 'plan' estaba en progreso y lo deja esperando aprobación.
Una persona ejecuta sdd-cli approve ... --phase plan
La CLI registra la aprobación y desbloquea 'implementation'.
```

Ayuda integrada (la CLI es autodescriptiva vía Cobra):

```bash
sdd-cli --help
sdd-cli <comando> --help
```

---

## 2. Flags globales

| Flag | Default | Función |
| --- | --- | --- |
| `--dir` | `.` | Proyecto que contiene `.sdd/`. |
| `--json` | `false` | Devuelve un envelope JSON para integraciones/agentes. |

Muchos comandos mutadores aceptan además:

| Flag | Función |
| --- | --- |
| `--actor-kind` | Tipo de actor: `human`, `agent`, `cli`, `system`. |
| `--actor-id` | Identificador del actor (ej. `copilot`, `user`). |
| `--operation-id` | Clave de idempotencia para reintentar sin duplicar. |

Los defaults de actor varían por comando: `start` usa `human`/`user`; `begin`, `deliver`,
`rework`, `cancel` y `record-event` usan `agent`/`agent`; `complete` y `archive` usan
`cli`/`sdd`. `approve` y `reject` no aceptan `--actor-kind`: registran siempre un actor
humano cuyo ID se pasa con `--by` (default `human`).

---

## 3. Matriz de comandos

| Comando | Tipo | Función | Muta estado |
| --- | --- | --- | --- |
| `init` | Inicialización | Instala `.sdd/` en el proyecto | Sí |
| `adapters list` | Catálogo | Lista adapters incluidos en el binario | No |
| `adapters install` | Inicialización | Instala un adapter en un proyecto SDD | Sí |
| `start` | Creación | Crea un work item | Sí |
| `status` | Consulta | Muestra manifest y fases | No |
| `next` | Consulta | Informa la próxima acción | No |
| `validate` | Diagnóstico | Valida proyecto o work item | No |
| `begin` | Transición | Inicia una fase habilitada | Sí |
| `deliver` | Transición | Entrega el resultado de una fase | Sí |
| `approve` | Gate humano | Aprueba una fase | Sí |
| `reject` | Gate humano | Rechaza una fase para retrabajo | Sí |
| `complete` | Transición | Completa una fase o el work item | Sí |
| `archive` | Cierre | Mueve el expediente a `archive/` | Sí |
| `rework` | Retrabajo | Reabre una fase cerrada y reemplaza sus descendientes | Sí |
| `cancel` | Cierre | Cancela un work item activo y lo mueve a `cancelled/` | Sí |
| `record-event` | Observabilidad | Agrega un evento custom | Sí |
| `version` | Info | Muestra versión, commit y fecha de build | No |

---

## 4. Inicialización

### `sdd-cli init`

Instala la estructura `.sdd/` (contrato: workflows, schemas, templates, procedures).

```bash
sdd-cli init
sdd-cli init --dir /ruta/al/proyecto
```

- Verifica que `.sdd/` no exista (no sobrescribe una instalación previa).
- Copia los recursos embebidos, excluyendo fixtures de test.
- Crea `work-items/active/` y `work-items/archive/` (con `.gitkeep`) y
  `work-items/.gitignore`, que excluye del control de versiones el estado de runtime del
  motor (`.locks/` y `.transactions/`).
- No inicializa Git.

### `sdd-cli adapters list`

```bash
sdd-cli adapters list
sdd-cli adapters list --json
```

Lista los adapters incluidos en el binario. La v1.0.0 expone solo `claude-code`.

### `sdd-cli adapters install <id>`

```bash
sdd-cli adapters install claude-code --dir /ruta/al/proyecto
```

- Exige una instalación `.sdd/` previa.
- Comprueba todas las colisiones antes de escribir: falla con `already_exists`, sin
  sobrescribir nada, si ya existe alguna de las raíces que publica (`CLAUDE.md`,
  `.mcp.json` o el directorio `.claude/`, aunque esté vacío). No hay `--force` en la v1.0.0.
- Publica `CLAUDE.md`, `.mcp.json` y `.claude/` (agents, skills, commands, hooks, rules).
  El inventario está en [src/adapters/README.md](../src/adapters/README.md).
- Instala `.claude/settings.local.json.example`; el archivo privado real queda ignorado.

---

## 5. Crear un work item

### `sdd-cli start <id>`

```bash
# Inicio normal
sdd-cli start feat-add-coupons \
  --title "Agregar cupones" \
  --summary "Permitir descuentos en checkout"

# Con workflow explícito
sdd-cli start bug-payment-timeout \
  --workflow bug-investigation \
  --title "Timeout de pago"

# Desde un artefacto existente
sdd-cli start feat-add-coupons \
  --title "Agregar cupones" \
  --from-artifact ./plan-aprobado.md \
  --phase plan
```

Flags:

| Flag | Requerido | Default |
| --- | --- | --- |
| `--title`, `-t` | Sí | — |
| `--workflow`, `-w` | No | El de `.sdd/config.yaml` |
| `--summary`, `-s` | No | Vacío |
| `--from-artifact` | Junto con `--phase` | Vacío |
| `--phase` | Junto con `--from-artifact` | Vacío |
| `--actor-kind` | No | `human` |
| `--actor-id` | No | `user` |
| `--operation-id` | No | Vacío |

El ID debe cumplir `^[a-z0-9]+(?:-[a-z0-9]+)*$` (ej. `feat-add-coupons`, `bug-123`).
Un ID usado por un work item cancelado queda reservado: `start` falla con
`already_exists`. En un inicio normal, la fase de entrada que acepta `user_prompt`
(`prd`, `change-request`, `issue` o `plan`, según el workflow) arranca automáticamente en
`in_progress`: no hace falta un `begin` para ella. Ver
[SDD_WORKFLOW.md §7](./SDD_WORKFLOW.md) para el inicio desde artefacto externo.

---

## 6. Consultas (no mutan estado)

### `sdd-cli status <id>`

```bash
sdd-cli status feat-add-coupons
sdd-cli status feat-add-coupons --json
```

Devuelve el estado general, ubicación (`active`/`archive`), workflow, revisión,
aprobaciones y el estado + artefacto de cada fase. En texto, las fases salen en orden
topológico; en JSON, `phases` es un mapa sin orden garantizado.

### `sdd-cli next <id>`

```bash
sdd-cli next feat-add-coupons
```

Informa la próxima acción, priorizando: una fase `awaiting_approval`, luego
`in_progress`, luego `ready` y, por último, una fase `superseded` cuyas dependencias ya
están satisfechas (tras un `rework`). Devuelve phase ID, estado, procedure, artefacto y si
necesita aprobación. **No inicia la fase** ni aumenta la revisión.

### `sdd-cli validate [id]`

```bash
sdd-cli validate                    # todo el proyecto
sdd-cli validate feat-add-coupons   # un work item
```

| Invocación | Alcance |
| --- | --- |
| Sin ID | Config, registry de capabilities, schemas, workflows, templates, procedures y work items de `active/` y `archive/`. |
| Con ID | Manifest, workflow, approvals, artefactos, eventos y referencias. |

Cada check reporta `status | category | code | target | message`, con estados
`passed`, `warning` o `failed`. Un resultado inválido usa exit code `1`. Las
advertencias no cambian el exit code. Es una consulta pura: no crea locks ni eventos.

- Los checks `config.`, `registry.`, `workflow.` y `manifest.schema_version_supported`
  fallan si el documento no declara el contrato `schema_version: "1.0"`.
- Los work items de `cancelled/` no se inspeccionan. `status`, `next` y `validate` con el
  ID de un work item cancelado fallan con `not_found` ("work item was cancelled").

---

## 7. Transiciones de fase

El ciclo normal de una fase: `begin → deliver → approve → complete`.

### `sdd-cli begin <id> --phase <fase>`

```bash
sdd-cli begin feat-add-coupons \
  --phase specification \
  --actor-kind agent --actor-id copilot
```

Permite `ready | rejected | superseded → in_progress`. Una fase `blocked` no puede
comenzar, y una `superseded` solo puede reiniciarse cuando sus dependencias vuelven a
estar satisfechas.

### `sdd-cli deliver <id> --phase <fase>`

```bash
sdd-cli deliver feat-add-coupons --phase specification --actor-id copilot
```

El destino depende de la política de aprobación de la fase:

| Política | Resultado |
| --- | --- |
| `required` | `awaiting_approval` |
| `optional` | `completed` |
| `optional` + `--request-approval` | `awaiting_approval` |
| `none` | `completed` |
| `none` + `--request-approval` | Error |

Al satisfacer dependencias, la CLI desbloquea las fases siguientes y prepara sus
templates dentro del mismo commit.

### `sdd-cli approve <id> --phase <fase>`

```bash
sdd-cli approve feat-add-coupons --phase plan --by matias --comment "Aprobado"
```

- La fase debe estar `awaiting_approval` y la política no puede ser `none`.
- El actor debe ser **humano** (la regla vive en el dominio).
- La fase pasa a `approved` y se desbloquean dependencias satisfechas.

### `sdd-cli reject <id> --phase <fase>`

```bash
sdd-cli reject feat-add-coupons --phase plan --by matias \
  --comment "Falta detallar el rollback"
```

La fase pasa a `rejected` sin desbloquear dependencias. El retrabajo se reabre con
`begin`; la nueva entrega crea otra iteración de aprobación sin borrar el rechazo previo.

### `sdd-cli complete <id> [--phase <fase>]`

```bash
sdd-cli complete feat-add-coupons --phase plan   # normaliza approved/accepted -> completed
sdd-cli complete feat-add-coupons                # cierra el work item
```

- Con `--phase`: lleva una fase `approved | accepted → completed`. No es obligatorio
  para desbloquear dependencias (approved/accepted ya las satisfacen); sirve para
  explicitar que la fase fue aplicada.
- Sin `--phase`: cierra el work item. Solo pasa a `completed` cuando todas las fases
  obligatorias están satisfechas, las opcionales no iniciadas se pueden omitir y toda
  fase opcional iniciada también terminó.
- Con el work item `completed`, solo las fases opcionales (por ejemplo `archive`) admiten
  `begin`/`deliver`/`approve`/`reject`/`complete --phase`.

### `sdd-cli rework <id> --phase <fase> --reason <motivo>`

```bash
sdd-cli rework feat-add-coupons --phase specification \
  --reason "Cambió el alcance de los cupones" \
  --actor-kind agent --actor-id copilot \
  --operation-id run:feat-add-coupons:spec:rework:001
```

| Flag | Requerido | Default |
| --- | --- | --- |
| `--phase`, `-p` | Sí | — |
| `--reason` | Sí (no vacío) | — |
| `--actor-kind` / `--actor-id` | No | `agent` / `agent` |
| `--operation-id` | No | Vacío |

- Precondiciones: work item en `active/` con estado `active` o `completed`, y fase en
  `completed`, `approved` o `accepted`; si la fase está en otro estado, falla con
  `invalid_transition`. Un work item archivado o cancelado no se encuentra (`not_found`).
- La fase vuelve a `ready` (requiere un nuevo `begin`). Cada descendiente, en orden
  topológico: `completed | approved | accepted | in_progress | awaiting_approval | rejected
  → superseded`; `ready → blocked`; `blocked | not_applicable | superseded` sin cambios.
- Si el work item estaba `completed`, vuelve a `active` (evento `work_item.reactivated`).
- No modifica artefactos ni borra el historial de aprobaciones. Registra eventos
  `phase.transitioned` con `cause` `rework_requested`/`rework_superseded` y el `reason`.
- `data` en JSON: `{ "work_item": {...}, "superseded": <n>, "reactivated": <bool> }`, donde
  `superseded` cuenta los descendientes modificados (incluye los re-bloqueados).

---

## 8. Cierre y observabilidad

### `sdd-cli archive <id>`

```bash
sdd-cli archive feat-add-coupons \
  --actor-kind cli --actor-id sdd \
  --operation-id run:feat-add-coupons:archive
```

Precondiciones: work item `completed`, fase `archive` satisfecha cuando el workflow la
declara, `validate <id>` sin failures, `artifacts/archive.md` válido y destino ausente.
Resultado:

```text
.sdd/work-items/active/<id>  ->  .sdd/work-items/archive/YYYY-MM-DD-<id>
```

El ID lógico no cambia; `status`/`validate` resuelven la ubicación archivada y el
expediente queda inmutable. Archivar un work item ya archivado falla con
`already_archived`. Ver la distinción fase `archive` vs comando `archive` en
[SDD_WORKFLOW.md §8](./SDD_WORKFLOW.md).

### `sdd-cli cancel <id> --reason <motivo>`

```bash
sdd-cli cancel feat-add-coupons --reason "Se descartó la iniciativa" \
  --actor-kind human --actor-id matias \
  --operation-id run:feat-add-coupons:cancel
```

| Flag | Requerido | Default |
| --- | --- | --- |
| `--reason` | Sí (no vacío) | — |
| `--actor-kind` / `--actor-id` | No | `agent` / `agent` |
| `--operation-id` | No | Vacío |

- Precondición: work item `active`. Uno `completed` o ya cancelado falla con
  `invalid_transition`; uno inexistente o archivado, con `not_found`.
- Resultado (irreversible): el work item pasa a `cancelled`, sin cambiar el estado de sus
  fases, y se mueve transaccionalmente:

  ```text
  .sdd/work-items/active/<id>  ->  .sdd/work-items/cancelled/<id>
  ```

- Registra el evento `work_item.cancelled` con el `reason`. El ID queda reservado (`start`
  lo rechaza) y el expediente cancelado es opaco para `status`, `next` y `validate`.
- Repetir con el mismo `--operation-id` devuelve el resultado ya aplicado.
- `data` en JSON: `{ "work_item": {...}, "location": "cancelled" }`.

### `sdd-cli record-event <id>`

```bash
sdd-cli record-event feat-add-coupons \
  --type validation.completed --message "Suite verde" \
  --actor-kind agent --actor-id verifier
```

Agrega un evento custom sin alterar fases. Aumenta la revisión, participa del commit
transaccional, valida actor y schema, y puede ser idempotente con `--operation-id`.

### `sdd-cli version`

```bash
sdd-cli version
# sdd-cli v1.0.0 (commit abc1234, built 2026-10-26T15:03:14Z)
```

---

## 9. Idempotencia (`--operation-id`)

Un agente puede perder la respuesta de un comando aunque la CLI ya lo haya aplicado.
Repetir el comando con el mismo `--operation-id` no duplica eventos, no vuelve a
incrementar la revisión y no reejecuta la transición: la CLI reconoce el intento y
devuelve el estado persistido.

Formato: 1 a 128 caracteres — letras, números, `.`, `:`, `_` y `-`.

```text
run:feat-x:plan:deliver:001
agent-session-42.approve.plan
```

---

## 10. Contrato de salida

### Modo humano (sin `--json`)

Éxitos por stdout, errores por stderr:

```text
Phase 'plan' delivered for work item 'feat-add-coupons'.
```

### Modo agente (`--json`)

stdout contiene un único envelope:

```json
{ "success": true, "data": {} }
```

Error:

```json
{
  "success": false,
  "error": {
    "code": "validation_failed",
    "message": "validation found 2 error(s)",
    "details": { "scope": "work_item", "target": "feat-add-coupons", "valid": false }
  }
}
```

### Códigos de error

| Código | Significado |
| --- | --- |
| `invalid_arguments` | Args, flags o comando incorrectos. |
| `invalid_input` | ID, actor, path, schema o contrato inválido. |
| `not_found` | Work item, workflow, fase o adapter inexistente, o work item cancelado. |
| `already_exists` | Colisión al crear (incluye ID reservado por una cancelación y raíces del adapter ya existentes). |
| `already_archived` | El work item ya está archivado. |
| `archive_conflict` | Ubicación ambigua (mismo ID en más de una ubicación) o transacción de archive/cancel inconsistente. |
| `invalid_transition` | Operación no permitida por el estado. |
| `validation_failed` | El diagnóstico encontró inconsistencias. |
| `concurrent_modification` | Revisión obsoleta. |
| `work_item_locked` | Otro escritor posee el lock. |
| `internal_error` | Error no clasificado. |

Exit code `0` en éxito, `1` en error.

---

## 11. Ejemplo completo: `fast-change`

```bash
SDD=sdd-cli
PROJECT=/tmp/example-project

$SDD init --dir "$PROJECT"

$SDD start fast-update-copy \
  --dir "$PROJECT" --workflow fast-change \
  --title "Actualizar copy de checkout" \
  --summary "Corregir mensaje de validación" \
  --operation-id run:start:001

# 'plan' arranca automáticamente in_progress al crear el item.
$SDD deliver fast-update-copy --dir "$PROJECT" --phase plan \
  --operation-id run:plan:deliver:001

$SDD approve fast-update-copy --dir "$PROJECT" --phase plan \
  --by matias --operation-id run:plan:approve:001

$SDD begin fast-update-copy --dir "$PROJECT" --phase implementation \
  --actor-id copilot --operation-id run:impl:begin:001
$SDD deliver fast-update-copy --dir "$PROJECT" --phase implementation \
  --actor-id copilot --operation-id run:impl:deliver:001

# Repetir begin/deliver para verification.
# human-code-review requiere begin -> deliver -> approve (gate humano).

$SDD complete fast-update-copy --dir "$PROJECT" \
  --operation-id run:work-item:complete:001
```

La CLI gobierna estados y evidencia. El contenido del plan, la implementación y la
verificación lo produce el agente siguiendo el `procedure` de cada fase.
