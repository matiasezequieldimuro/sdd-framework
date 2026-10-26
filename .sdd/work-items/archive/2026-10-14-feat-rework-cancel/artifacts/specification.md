---
schema_version: "1.0"
kind: artifact
id: "specification"
work_item: "feat-rework-cancel"
phase: "specification"
status: draft
created_at: "2026-10-07T00:36:03Z"
created_by: { kind: "cli", id: "sdd" }
sources:
  - artifacts/prd.md
---

# Especificación: Comandos `rework` y `cancel` para work items

## Requisitos y comportamiento

### Contexto del dominio

`sdd-cli` gobierna el ciclo de vida de los work items como una máquina de estados sobre el
filesystem. El estado persiste en `manifest.yaml` (estado actual), `events.jsonl` (historial
inmutable, append-only) y `artifacts/*.md` (evidencia). Toda mutación es atómica: lock por
work item + staging en `.sdd/work-items/.transactions/` + rename + recovery.

Los estados de una fase (`PhaseStatus`): `not_applicable`, `blocked`, `ready`, `in_progress`,
`awaiting_approval`, `approved`, `completed`, `rejected`, `accepted`, `superseded`.

Los estados de un work item (`WorkItemStatus`): `active`, `completed`, `archived`, `cancelled`.

El directorio de trabajo de un work item activo es `.sdd/work-items/active/<id>/`.

---

### Comando `sdd-cli rework <id> --phase <fase> --reason <texto>`

#### Interfaz

| Elemento | Tipo | Requerido | Descripción |
|---|---|---|---|
| `<id>` | argumento | sí | ID del work item (kebab-case válido) |
| `--phase` / `-p` | flag string | sí | ID de la fase objetivo a regresar a `ready` |
| `--reason` | flag string | sí | Texto libre que documenta el motivo del retrabajo |
| `--actor-kind` | flag string | no | Tipo de actor (default: `agent`) |
| `--actor-id` | flag string | no | ID del actor (default: `agent`) |
| `--operation-id` | flag string | no | ID de operación para idempotencia |

#### Precondiciones

**PC-RW-01.** El `<id>` debe corresponder a un work item existente en `active/` o en una
ubicación reconocible por el motor. Si no existe, el motor falla con `not_found`.

**PC-RW-02.** El work item no debe estar en estado `archived` ni `cancelled`. Ambos son
estados finales irreversibles. Cualquiera de los dos produce error `invalid_transition`.

**PC-RW-03.** El work item debe estar en estado `active` o `completed`. Si está `completed`,
el comando lo reactivará antes de proceder.

**PC-RW-04.** El ID de fase provisto en `--phase` debe existir en el manifest del work item.
Si no existe, el motor falla con `not_found`.

**PC-RW-05.** La fase objetivo debe estar en estado `completed`, `approved` o `accepted`.
Cualquier otro estado (`ready`, `in_progress`, `not_applicable`, `blocked`, `superseded`,
`rejected`, `awaiting_approval`) produce error `invalid_transition`.

**PC-RW-06.** El flag `--reason` con texto no vacío es obligatorio. Su ausencia o su valor
vacío produce error `invalid_input`.

#### Algoritmo de ejecución

1. **Idempotencia**: si se proveyó `--operation-id` y ya existe un evento con ese
   `correlation_id` en `events.jsonl`, el motor devuelve el estado actual del work item sin
   re-transicionar. Salida: JSON del work item actual + código HTTP equivalente 200.

2. **Lock**: adquirir el lock de archivo del work item (`.sdd/work-items/.locks/<id>.lock`).
   Si el lock está tomado, falla con `work_item_locked`.

3. **Recovery**: ejecutar recovery de transacciones incompletas si existe marker en
   `.transactions/<id>.*`.

4. **Carga**: leer `manifest.yaml` del work item.

5. **Validación de revisión optimista**: si se proveyó `--revision` o el motor exige una
   revisión esperada, verificar que coincida con la actual. Si no coincide, falla con
   `concurrent_modification`.

6. **Verificación de precondiciones PC-RW-01 a PC-RW-06** (ver arriba). Falla antes de
   cualquier mutación si alguna se viola.

7. **Reactivación del work item** (solo si estaba `completed`):
   - Cambiar `item.Status` de `completed` a `active`.
   - Preparar evento `work_item.reactivated` (ver sección Eventos).

8. **Transición de la fase objetivo** a `ready`:
   - Cambiar el estado de la fase indicada en `--phase` de su estado actual a `ready`.
   - Preparar evento `phase.transitioned` con cause `rework_requested` para esta fase.

9. **Propagación selectiva sobre descendientes transitivos**:
   - Recorrer el DAG del workflow desde la fase objetivo hacia adelante (topológicamente).
   - Para cada fase descendiente (directa o transitiva), aplicar la siguiente tabla:

   | Estado del descendiente | Acción | Evento |
   |---|---|---|
   | `completed`, `approved`, `accepted`, `in_progress`, `awaiting_approval`, `rejected` | → `superseded` (fue trabajada) | `phase.transitioned` cause `rework_superseded` |
   | `ready` | → `blocked` (desbloqueada pero no iniciada) | `phase.transitioned` cause `rework_superseded` |
   | `blocked` | sin cambio | ninguno |
   | `not_applicable` | sin cambio | ninguno |
   | `superseded` | sin cambio (idempotente) | ninguno |

10. **Preparar commit transaccional**:
    - Incrementar `item.Revision` en 1.
    - Escribir `manifest.yaml` actualizado en el stage.
    - Agregar todos los eventos preparados a `events.jsonl` en el stage (append).
    - Publicar el stage mediante rename atómico.

11. **Liberar lock** y emitir salida.

#### Postcondiciones

**POST-RW-01.** La fase indicada en `--phase` queda en estado `ready`. Sus artefactos no se
modifican por el comando (quedan en el filesystem; el agente los sobreescribe durante el nuevo
`begin`/`deliver`).

**POST-RW-02.** Los descendientes transitivos de la fase objetivo quedan en el estado que
corresponda según su estado previo: las fases trabajadas (`completed`, `approved`, `accepted`,
`in_progress`, `awaiting_approval`, `rejected`) pasan a `superseded`; las fases en `ready`
pasan a `blocked`; las fases en `blocked`, `not_applicable` o `superseded` no cambian.

**POST-RW-03.** Si el work item estaba `completed`, queda en estado `active`.

**POST-RW-04.** Los registros de `Approval` existentes en el manifest no son modificados ni
eliminados. El historial de decisiones es inmutable.

**POST-RW-05.** `events.jsonl` contiene, por cada transición producida, un evento auditable
que incluye el `reason` del invocador.

**POST-RW-06.** `item.Revision` se incrementó exactamente en 1 respecto al valor previo.

#### Salida

- **Texto**: `Work item '<id>': phase '<fase>' reworked. <n> phase(s) superseded.`
  (Si el work item fue reactivado: `Work item '<id>' reactivated and phase '<fase>' reworked.`)
- **JSON** (`--json`): envelope estándar con el `WorkItem` actualizado en `data`.

---

### Comando `sdd-cli cancel <id> --reason <texto>`

#### Interfaz

| Elemento | Tipo | Requerido | Descripción |
|---|---|---|---|
| `<id>` | argumento | sí | ID del work item a cancelar (kebab-case válido) |
| `--reason` | flag string | sí | Texto libre que documenta el motivo de la cancelación |
| `--actor-kind` | flag string | no | Tipo de actor (default: `agent`) |
| `--actor-id` | flag string | no | ID del actor (default: `agent`) |
| `--operation-id` | flag string | no | ID de operación para idempotencia |

No se requiere actor de tipo `human`. Cualquier tipo de actor es válido.

#### Precondiciones

**PC-CA-01.** El `<id>` debe corresponder a un work item existente en `active/`. Si no existe
en `active/`, el motor verifica si existe en `archive/` o en `cancelled/` para proveer un
error diagnóstico. Si el item no existe en ninguna ubicación, falla con `not_found`.

**PC-CA-02.** El work item debe estar en estado `active`. Si está en `completed`, `archived` o
`cancelled`, el motor falla con `invalid_transition` indicando el estado actual.

**PC-CA-03.** El flag `--reason` con texto no vacío es obligatorio. Su ausencia o valor vacío
produce error `invalid_input`.

#### Algoritmo de ejecución

1. **Idempotencia**: si se proveyó `--operation-id` y el work item ya existe en `cancelled/`
   con un evento con ese `correlation_id`, el motor devuelve el estado actual sin re-aplicar la
   cancelación.

2. **Lock**: adquirir el lock de archivo del work item. Si el lock está tomado, falla con
   `work_item_locked`.

3. **Recovery**: ejecutar recovery de transacciones incompletas si existe marker de transacción
   de cancelación en `.transactions/<id>.cancel.json`.

4. **Carga**: leer `manifest.yaml` del work item desde `active/<id>/`.

5. **Verificación de precondiciones PC-CA-01 a PC-CA-03**.

6. **Preparar el manifest cancelado**:
   - Cambiar `item.Status` a `cancelled`.
   - No modificar el estado de ninguna fase individual.
   - Incrementar `item.Revision` en 1.

7. **Preparar evento `work_item.cancelled`** (ver sección Eventos).

8. **Movimiento transaccional** (análogo a `ArchiveWorkItem`, destino diferente):
   a. Calcular destino: `.sdd/work-items/cancelled/<id>/` (sin prefijo de fecha).
   b. Crear directorio de stage en `.transactions/<id>.cancel-stage/`.
   c. Copiar el directorio del work item activo al stage.
   d. Escribir el manifest actualizado en el stage.
   e. Agregar el evento a `events.jsonl` en el stage.
   f. Escribir marker de transacción en `.transactions/<id>.cancel.json` con:
      `{ schema_version, work_item, operation_id, destination }`.
   g. Mover el stage al destino final `cancelled/<id>/` mediante `rename` atómico.
   h. Mover `active/<id>/` a `.transactions/<id>.cancel-backup/` (backup atómico).
   i. Eliminar el marker de transacción y el backup.

9. **Liberar lock**.

10. **Recovery ante interrupción**: si el proceso es interrumpido entre los pasos f y i y se
    reintenta el comando (o cualquier operación sobre el mismo ID), el motor detecta el marker
    `.transactions/<id>.cancel.json` y recupera el estado consistente:
    - Si el destino `cancelled/<id>/` ya existe y el backup también: eliminar backup, eliminar
      marker.
    - Si el destino `cancelled/<id>/` no existe y el backup existe: restaurar backup a
      `active/<id>/`, eliminar marker.
    - Si el destino `cancelled/<id>/` existe y el backup no: eliminar marker (cancelación
      completada).

#### Postcondiciones

**POST-CA-01.** El directorio `active/<id>/` ya no existe.

**POST-CA-02.** El directorio `.sdd/work-items/cancelled/<id>/` existe y contiene el manifest,
los artefactos y `events.jsonl` del work item, con `status: cancelled` en el manifest y el
evento `work_item.cancelled` como último evento en `events.jsonl`.

**POST-CA-03.** El estado de cada fase individual en el manifest cancelado es idéntico al
estado que tenían antes de la cancelación. `cancel` no modifica fases.

**POST-CA-04.** El ID queda reservado permanentemente. Ningún comando futuro puede crear un
nuevo work item con el mismo ID.

**POST-CA-05.** `events.jsonl` del expediente cancelado contiene el evento `work_item.cancelled`
con `reason`, `actor` y `at`.

#### Salida

- **Texto**: `Work item '<id>' cancelled.`
- **JSON** (`--json`): envelope estándar con `{ "work_item": <WorkItem>, "location": "cancelled" }`.

---

## Contrato de errores (envelope JSON)

Todos los errores siguen el envelope estándar:
```json
{ "error": { "code": "<código>", "message": "<texto>", "details": <objeto-o-null> } }
```

### Errores de `rework`

| Condición | Código | Mensaje ilustrativo |
|---|---|---|
| Work item no encontrado | `not_found` | `work item '<id>' not found` |
| Work item en `archived` | `invalid_transition` | `cannot rework work item '<id>': status is archived` |
| Work item en `cancelled` | `invalid_transition` | `cannot rework work item '<id>': status is cancelled` |
| Fase no existe en el manifest | `not_found` | `phase '<fase>' not found in work item '<id>'` |
| Fase en estado inválido (no `completed`/`approved`/`accepted`) | `invalid_transition` | `cannot rework phase '<fase>': current status is '<estado>'; expected completed, approved or accepted` |
| `--reason` ausente o vacío | `invalid_input` | `--reason is required and cannot be empty` |
| Lock tomado | `work_item_locked` | `work item '<id>' is locked by another operation` |
| Revisión desactualizada | `concurrent_modification` | `work item '<id>' was modified concurrently` |
| Operación ya aplicada (idempotencia) | `200 OK` (no es error) | — |

### Errores de `cancel`

| Condición | Código | Mensaje ilustrativo |
|---|---|---|
| Work item no encontrado (en ninguna ubicación) | `not_found` | `work item '<id>' not found` |
| Work item en `completed` | `invalid_transition` | `cannot cancel work item '<id>': status is completed` |
| Work item en `archived` | `invalid_transition` | `cannot cancel work item '<id>': status is archived` |
| Work item ya en `cancelled` | `invalid_transition` | `cannot cancel work item '<id>': status is cancelled` |
| `--reason` ausente o vacío | `invalid_input` | `--reason is required and cannot be empty` |
| Lock tomado | `work_item_locked` | `work item '<id>' is locked by another operation` |

### Errores en comandos existentes cuando el ID apunta a `cancelled/`

| Comando | Condición | Código |
|---|---|---|
| `sdd-cli status <id>` | ID en `cancelled/` | `invalid_transition` o `not_found` con mensaje que indica que el item fue cancelado |
| `sdd-cli validate <id>` | ID en `cancelled/` | mismo que `status` |
| `sdd-cli next <id>` | ID en `cancelled/` | mismo que `status` |
| `sdd-cli start <id>` | ID en `cancelled/` (reservado) | `already_exists` con mensaje que indica que el ID fue cancelado y está reservado |

---

## Eventos emitidos

Los eventos siguen el schema `event.schema.json`:
```
schema_version, id (UUID), at (RFC3339Nano UTC), work_item, type, actor, data, correlation_id
```
`correlation_id` es el valor de `--operation-id` cuando se provee.

### Eventos de `rework`

#### `work_item.reactivated` (solo si el work item estaba `completed`)

```json
{
  "schema_version": "0.1",
  "id": "<uuid>",
  "at": "<RFC3339Nano>",
  "work_item": "<id>",
  "type": "work_item.reactivated",
  "actor": { "kind": "<actor-kind>", "id": "<actor-id>" },
  "data": {
    "from": "completed",
    "to": "active",
    "reason": "<texto del --reason>"
  },
  "correlation_id": "<operation-id o vacío>"
}
```

#### `phase.transitioned` — fase objetivo (cause `rework_requested`)

```json
{
  "schema_version": "0.1",
  "id": "<uuid>",
  "at": "<RFC3339Nano>",
  "work_item": "<id>",
  "type": "phase.transitioned",
  "actor": { "kind": "<actor-kind>", "id": "<actor-id>" },
  "data": {
    "phase": "<fase-objetivo>",
    "from": "<estado-previo>",
    "to": "ready",
    "cause": "rework_requested",
    "reason": "<texto del --reason>"
  },
  "correlation_id": "<operation-id o vacío>"
}
```

#### `phase.transitioned` — cada descendiente transitado a `superseded` (cause `rework_superseded`)

Un evento por cada fase que transite a `superseded` (las ya en `superseded` o en
`not_applicable` no generan evento):

```json
{
  "schema_version": "0.1",
  "id": "<uuid>",
  "at": "<RFC3339Nano>",
  "work_item": "<id>",
  "type": "phase.transitioned",
  "actor": { "kind": "<actor-kind>", "id": "<actor-id>" },
  "data": {
    "phase": "<fase-descendiente>",
    "from": "<estado-previo>",
    "to": "superseded",
    "cause": "rework_superseded",
    "reason": "<texto del --reason>"
  },
  "correlation_id": "<operation-id o vacío>"
}
```

**Orden de eventos en `events.jsonl`**: primero `work_item.reactivated` (si aplica), luego
`phase.transitioned` de la fase objetivo, luego `phase.transitioned` de cada descendiente en
orden topológico del DAG.

### Evento de `cancel`

#### `work_item.cancelled`

```json
{
  "schema_version": "0.1",
  "id": "<uuid>",
  "at": "<RFC3339Nano>",
  "work_item": "<id>",
  "type": "work_item.cancelled",
  "actor": { "kind": "<actor-kind>", "id": "<actor-id>" },
  "data": {
    "from": "active",
    "to": "cancelled",
    "reason": "<texto del --reason>"
  },
  "correlation_id": "<operation-id o vacío>"
}
```

---

## Garantías de persistencia y atomicidad

### Para `rework`

- El commit es atómico: staging en `.transactions/<id>.backup` + rename + publicación.
- Si el proceso se interrumpe antes del rename final, el estado del work item queda intacto
  (se observa el estado previo al rework).
- Recovery: al reconectarse, si existe `.transactions/<id>.backup`, el motor evalúa si el
  destino (el directorio `active/<id>/`) ya fue actualizado; si no, restaura desde el backup.
- Solo un proceso puede operar sobre el work item a la vez (lock por flock).
- Revisión optimista: si otro proceso modificó el work item concurrentemente, el motor falla
  con `concurrent_modification` antes de escribir.

### Para `cancel`

- El directorio `cancelled/` se crea bajo `.sdd/work-items/` como peer de `active/` y
  `archive/`. No existe subdirectorio por fecha.
- El movimiento del expediente se modela exactamente como `ArchiveWorkItem` en
  `fs_archive_repository.go`, adaptando la ruta de destino a `cancelled/<id>` y el marker a
  `.cancel.json`.
- Marker de transacción `.transactions/<id>.cancel.json` escrito antes del rename; eliminado
  después de la publicación exitosa.
- Recovery al reintentar: el motor detecta el marker y determina en qué etapa se interrumpió
  el movimiento, recuperando el estado consistente (ver Algoritmo, paso 10).
- Un work item cancelado nunca aparece en `active/`; el directorio `active/<id>/` es destruido
  por el rename.

---

## Impacto sobre comandos existentes

### `sdd-cli start <id>`

El comando debe inspeccionar, en orden, las siguientes ubicaciones antes de crear un nuevo
work item:

1. `.sdd/work-items/active/<id>/` — si existe, falla con `id_already_exists`.
2. `.sdd/work-items/archive/` — si existe una entrada con ese ID, falla con `id_already_exists`
   (o `already_archived`).
3. `.sdd/work-items/cancelled/<id>/` — **nuevo**: si existe, falla con `already_exists` y mensaje
   que indica que el ID fue previamente cancelado.

Sin este cambio, se podría reutilizar un ID cancelado silenciosamente, rompiendo la trazabilidad.

### `sdd-cli status <id>`, `sdd-cli validate <id>`, `sdd-cli next <id>`

Estos comandos son de solo lectura. Actualmente inspeccionan `active/` y `archive/`.

Con esta feature, deben además detectar si el ID apunta a `cancelled/<id>/` y, en ese caso,
devolver un error con código `not_found` (o `invalid_transition`) con un mensaje diagnóstico
claro: `work item '<id>' was cancelled`. No exponen el contenido del expediente cancelado.

### `sdd-cli begin`, `sdd-cli deliver`, `sdd-cli approve`, `sdd-cli reject`, `sdd-cli complete`, `sdd-cli archive`

No se modifican estructuralmente. Seguirán fallando con `not_found` si el ID apunta a
`cancelled/`, porque no verán el work item en `active/`.

### `sdd-cli rework` después de `rework`

Un segundo `rework` sobre el mismo work item (distinta fase) es válido, siempre que la nueva
fase objetivo cumpla las precondiciones de estado. Los descendientes de la nueva fase objetivo
se propagan a `superseded` por las mismas reglas.

### `sdd-cli begin` después de `rework`

La fase en estado `ready` producida por `rework` acepta `begin` directamente (transición
`ready → in_progress`). Esta transición ya es válida según la spec de transiciones existente
(`BeginPhase` acepta `ready`, `rejected` o `superseded`). No requiere cambios en `begin`.

---

## Invariantes del dominio

**INV-01.** Un work item no puede existir simultáneamente en más de una ubicación
(`active/`, `archive/`, `cancelled/`).

**INV-02.** Los `Approval` records son inmutables una vez registrados. `rework` no los elimina
ni los modifica. El historial de decisiones refleja todas las iteraciones previas.

**INV-03.** El archivo `events.jsonl` es append-only. Ningún evento se elimina ni se modifica.

**INV-04.** La propagación en `rework` es transitiva sobre el DAG hasta las hojas. Solo las
fases efectivamente trabajadas (`completed`, `approved`, `accepted`, `in_progress`,
`awaiting_approval`, `rejected`) pasan a `superseded`. Las fases en `ready` pasan a `blocked`.
Las fases en `blocked`, `not_applicable` o `superseded` no se modifican.

**INV-05.** Un work item en `cancelled/` es opaco para la interfaz pública del motor. Solo
`start` lo detecta (para rechazar reutilización del ID).

**INV-06.** El `reason` se incluye en todos los eventos de `rework` (tanto en la fase objetivo
como en los descendientes) y en el evento de `cancel`. Es el registro auditable de la decisión.

**INV-07.** `cancel` no modifica el estado de ninguna fase. El snapshot del manifest cancelado
refleja el estado exacto del work item en el momento de la cancelación.

---

## Escenarios verificables

### Escenarios de `rework`

#### ESC-RW-01 — Retrabajo básico desde fase aprobada (work item activo)

**Precondición**: work item `mi-feature` en `active`; fase `plan` en `approved`; fases
`implementation` y `verification` en `completed`.

**Acción**: `sdd-cli rework mi-feature --phase plan --reason "cambio de alcance aprobado"`

**Resultado esperado**:
- Fase `plan` queda en `ready`.
- Fase `implementation` queda en `superseded`.
- Fase `verification` queda en `superseded`.
- Work item permanece `active`.
- `events.jsonl` contiene:
  - `phase.transitioned {phase: plan, from: approved, to: ready, cause: rework_requested}`
  - `phase.transitioned {phase: implementation, from: completed, to: superseded, cause: rework_superseded}`
  - `phase.transitioned {phase: verification, from: completed, to: superseded, cause: rework_superseded}`
- Revision incrementada en 1.
- `Approvals` no modificados.

**Verificación subsiguiente**: `sdd-cli begin mi-feature --phase plan` debe tener éxito
(transición `ready → in_progress`).

---

#### ESC-RW-02 — Retrabajo con reactivación (work item completed)

**Precondición**: work item `mi-feature` en `completed`; fase `prd` en `completed`.

**Acción**: `sdd-cli rework mi-feature --phase prd --reason "PRD necesita revisión"`

**Resultado esperado**:
- Work item pasa a `active`.
- Fase `prd` queda en `ready`.
- Todas las fases descendientes de `prd` quedan en `superseded`.
- `events.jsonl` contiene `work_item.reactivated` como primer evento del lote.
- Mensaje de salida indica reactivación.

---

#### ESC-RW-03 — Fase en estado inválido como target

**Precondición**: work item `mi-feature` en `active`; fase `plan` en `in_progress`.

**Acción**: `sdd-cli rework mi-feature --phase plan --reason "prueba"`

**Resultado esperado**: error `invalid_transition`, mensaje indica estado actual `in_progress`.
El work item no muta.

---

#### ESC-RW-04 — Work item archivado

**Precondición**: work item `mi-feature` en `archived` (en `archive/YYYY-MM-DD-mi-feature/`).

**Acción**: `sdd-cli rework mi-feature --phase plan --reason "prueba"`

**Resultado esperado**: error `invalid_transition`, mensaje indica estado `archived`.

---

#### ESC-RW-05 — Work item cancelado

**Precondición**: work item `mi-feature` en `cancelled/`.

**Acción**: `sdd-cli rework mi-feature --phase plan --reason "prueba"`

**Resultado esperado**: error `invalid_transition` (o `not_found`), mensaje indica que el
item fue cancelado.

---

#### ESC-RW-06 — `--reason` ausente

**Acción**: `sdd-cli rework mi-feature --phase plan`

**Resultado esperado**: error `invalid_input` de Cobra por flag requerido. No se aplica ninguna
transición.

---

#### ESC-RW-07 — Fase no existe

**Acción**: `sdd-cli rework mi-feature --phase fase-inexistente --reason "x"`

**Resultado esperado**: error `not_found`, mensaje indica que la fase no existe en el work item.

---

#### ESC-RW-08 — Descendiente ya en `superseded` (idempotencia parcial)

**Precondición**: work item `mi-feature` con fase `implementation` ya en `superseded` antes
del `rework`.

**Acción**: `sdd-cli rework mi-feature --phase plan --reason "nuevo alcance"`

**Resultado esperado**:
- Fase `implementation` permanece en `superseded` sin evento adicional.
- Solo la fase `plan` (objetivo) recibe evento `rework_requested`.
- Otras fases descendientes en estados distintos de `superseded` y `not_applicable` reciben
  `rework_superseded`.

---

#### ESC-RW-09 — Descendiente en `not_applicable` no se afecta

**Precondición**: work item `mi-feature` iniciado desde un entry point intermedio; fases
anteriores en `not_applicable`. Suponer que alguna fase descendiente también tiene
`not_applicable` (escenario teórico o workflow extendido).

**Resultado esperado**: ninguna fase en `not_applicable` recibe `superseded`. El motor las omite.

---

#### ESC-RW-10 — Idempotencia con `--operation-id`

**Acción (primera vez)**: `sdd-cli rework mi-feature --phase plan --reason "x" --operation-id abc-123`

**Acción (segunda vez, mismos argumentos)**: `sdd-cli rework mi-feature --phase plan --reason "x" --operation-id abc-123`

**Resultado esperado**: la segunda invocación devuelve el estado actual sin eventos duplicados.
`events.jsonl` contiene exactamente un conjunto de eventos con `correlation_id: abc-123`.

---

### Escenarios de `cancel`

#### ESC-CA-01 — Cancelación exitosa

**Precondición**: work item `mi-feature` en `active`, con fases en varios estados.

**Acción**: `sdd-cli cancel mi-feature --reason "proyecto discontinuado"`

**Resultado esperado**:
- `active/mi-feature/` ya no existe.
- `cancelled/mi-feature/` existe y contiene `manifest.yaml` con `status: cancelled`.
- Todas las fases conservan sus estados previos.
- `events.jsonl` en `cancelled/mi-feature/` contiene `work_item.cancelled` con `reason`.
- Exit code: 0.
- Mensaje: `Work item 'mi-feature' cancelled.`

---

#### ESC-CA-02 — Cancelación de work item `completed`

**Precondición**: work item `mi-feature` en `completed`.

**Acción**: `sdd-cli cancel mi-feature --reason "prueba"`

**Resultado esperado**: error `invalid_transition`, mensaje indica que el item está `completed`.

---

#### ESC-CA-03 — Cancelación de work item `archived`

**Precondición**: work item `mi-feature` en `archived`.

**Acción**: `sdd-cli cancel mi-feature --reason "prueba"`

**Resultado esperado**: error `invalid_transition`, mensaje indica que el item está `archived`.

---

#### ESC-CA-04 — `--reason` ausente

**Acción**: `sdd-cli cancel mi-feature`

**Resultado esperado**: error `invalid_input`. No se aplica ninguna mutación.

---

#### ESC-CA-05 — ID reservado tras cancelación

**Precondición**: work item `mi-feature` ya cancelado (en `cancelled/mi-feature/`).

**Acción**: `sdd-cli start mi-feature --title "nuevo proyecto"`

**Resultado esperado**: error `already_exists`, mensaje indica que el ID `mi-feature` fue
previamente cancelado y no puede reutilizarse.

---

#### ESC-CA-06 — `status` sobre ID cancelado

**Precondición**: work item `mi-feature` en `cancelled/`.

**Acción**: `sdd-cli status mi-feature`

**Resultado esperado**: error con código `not_found` (o equivalente) y mensaje que indica que
el work item `mi-feature` fue cancelado. No se expone el contenido del expediente.

---

#### ESC-CA-07 — Actor `agent` puede cancelar

**Acción**: `sdd-cli cancel mi-feature --reason "..." --actor-kind agent --actor-id my-agent`

**Resultado esperado**: cancelación exitosa. No se requiere `--actor-kind human`.

---

#### ESC-CA-08 — Recovery ante interrupción

**Precondición**: proceso interrumpido en mitad del movimiento transaccional; marker
`.transactions/mi-feature.cancel.json` existe; `cancelled/mi-feature/` puede o no existir.

**Acción**: `sdd-cli cancel mi-feature --reason "..." --operation-id <mismo-id>`

**Resultado esperado**: el motor detecta el marker, evalúa el estado y recupera la consistencia.
El resultado final es equivalente a una cancelación exitosa: `cancelled/mi-feature/` existe,
`active/mi-feature/` no existe, estado `cancelled`, evento `work_item.cancelled` en
`events.jsonl`.

---

#### ESC-CA-09 — Idempotencia con `--operation-id`

**Acción (primera vez)**: `sdd-cli cancel mi-feature --reason "x" --operation-id xyz-456`

**Acción (segunda vez)**: `sdd-cli cancel mi-feature --reason "x" --operation-id xyz-456`

**Resultado esperado**: la segunda invocación detecta que el evento `work_item.cancelled` con
`correlation_id: xyz-456` ya existe en `events.jsonl` del expediente cancelado, y devuelve el
estado actual sin re-aplicar la cancelación.

---

## Reglas, contratos y restricciones

### Restricciones estructurales

- El directorio `.sdd/work-items/cancelled/` es un directorio de primer nivel bajo
  `work-items/`, análogo a `active/` y `archive/`. El motor lo crea al cancelar el primer work
  item si no existe.
- El nombre del directorio de un expediente cancelado es exactamente `<id>` (sin prefijo de
  fecha ni sufijo). El ID es único por diseño.
- Los expedientes en `cancelled/` no tienen `.gitkeep`; el directorio se crea con el primer
  `cancel`.

### Restricciones de transición

- `rework` solo acepta como estado de origen de la fase objetivo: `completed`, `approved`,
  `accepted`. Cualquier otro estado es rechazado con `invalid_transition`.
- `cancel` solo acepta work items en estado `active`. Estados `completed`, `archived` y
  `cancelled` son rechazados con `invalid_transition`.
- Ninguno de los dos comandos puede ser aplicado a un work item `archived` (`rework` ni
  `cancel` abren un expediente archivado).

### Restricciones de auditoría

- El campo `reason` es propagado a todos los eventos de una misma invocación de `rework` o
  `cancel`. No puede quedar vacío en ningún evento si fue provisto en el comando.
- El campo `correlation_id` del evento coincide con el `--operation-id` provisto, o queda vacío
  si no se proveyó.

### Invariantes de idempotencia

- Si un `--operation-id` ya fue aplicado, el motor devuelve el estado actual (éxito) sin
  re-transicionar. La respuesta JSON es el work item en su estado post-operación.
- No existe distinción de comportamiento entre "operación ya aplicada" y "operación completada
  en esta invocación" desde la perspectiva del caller.

---

## Criterios de aceptación

Los criterios de aceptación derivan directamente de los requisitos del PRD aprobado y los
refinan con detalle suficiente para verificación independiente.

### `rework`

| ID | Criterio | Referencia PRD |
|---|---|---|
| CA-RW-01 | `rework <id> --phase <fase> --reason <texto>` con work item `active` y fase en `completed`/`approved`/`accepted`: la fase indicada queda en `ready`; sus descendientes transitivos quedan en `superseded` (excepto `not_applicable` y ya `superseded`); el work item permanece `active`; revision +1. | CA-RW-01 |
| CA-RW-02 | Tras `rework`, `begin <id> --phase <fase>` sobre la fase objetivo tiene éxito y transiciona a `in_progress`. | CA-RW-02 |
| CA-RW-03 | `rework <id> --phase <fase> --reason <texto>` con work item `completed`: el work item pasa a `active` y se aplican las mismas transiciones de fase; el evento `work_item.reactivated` precede al de la fase objetivo en `events.jsonl`. | CA-RW-03 |
| CA-RW-04 | `rework <id> --phase <fase>` sin `--reason`: el motor rechaza con error `invalid_input`; el work item no muta. | CA-RW-04 |
| CA-RW-05 | `rework <id> --phase <fase-inexistente> --reason <texto>`: el motor rechaza con error `not_found`; el work item no muta. | CA-RW-05 |
| CA-RW-06 | `rework <id> --phase <fase> --reason <texto>` con work item `archived` o `cancelled`: el motor rechaza con error `invalid_transition` indicando el estado actual; el work item no muta. | CA-RW-06 |
| CA-RW-07 | Los `Approval` records presentes antes del `rework` no son modificados ni eliminados del manifest resultante. | CA-RW-07 |
| CA-RW-08 | `events.jsonl` contiene exactamente un evento por cada fase transicionada, todos con el campo `reason` igual al texto provisto en `--reason`. | CA-RW-08 |
| CA-RW-09 | Fases descendientes ya en estado `superseded` antes del `rework` no generan un nuevo evento de transición y permanecen en `superseded`. | CA-RW-09 |
| CA-RW-10 | `rework <id> --phase <fase> --reason <texto>` con la fase objetivo en estado `ready`, `in_progress`, `not_applicable`, `blocked`, `superseded`, `rejected` o `awaiting_approval`: el motor rechaza con error `invalid_transition`. | CA-RW-10 |
| CA-RW-11 | Fases en estado `not_applicable` que son descendientes transitivas de la fase objetivo no reciben `superseded` y no generan evento de transición. | CA-RW-11 |
| CA-RW-12 | `rework` con `--operation-id` ya registrado devuelve el estado actual sin duplicar eventos. | AS-04 |

### `cancel`

| ID | Criterio | Referencia PRD |
|---|---|---|
| CA-CA-01 | `cancel <id> --reason <texto>` con work item `active`: el expediente existe en `.sdd/work-items/cancelled/<id>/`, el manifest tiene `status: cancelled`, se emitió `work_item.cancelled` con `reason`. | CA-CA-01 |
| CA-CA-02 | `cancel <id> --reason <texto>` con work item `completed`: error `invalid_transition` indicando el estado `completed`. | CA-CA-02 |
| CA-CA-03 | `cancel <id> --reason <texto>` con work item `archived`: error `invalid_transition` indicando el estado `archived`. | CA-CA-03 |
| CA-CA-04 | `cancel <id>` sin `--reason`: error `invalid_input`; el work item no muta. | CA-CA-04 |
| CA-CA-05 | Tras `cancel <id>`, `sdd-cli start <id>` falla con error `already_exists` indicando que el ID fue cancelado y está reservado. | CA-CA-05 |
| CA-CA-06 | Las fases del work item cancelado conservan sus estados originales en el manifest del expediente cancelado. | CA-CA-06 |
| CA-CA-07 | `cancel` con `--actor-kind agent` tiene éxito. No se requiere actor `human`. | CA-CA-07 |
| CA-CA-08 | Si el proceso es interrumpido durante el movimiento y se reintenta, el motor recupera un estado consistente (sin expediente parcialmente movido). | CA-CA-08 |
| CA-CA-09 | `sdd-cli status <id>`, `validate <id>` y `next <id>` ejecutados sobre un work item cancelado devuelven error con mensaje que indica que el ID corresponde a un work item cancelado. | CA-CA-09 |
| CA-CA-10 | `cancel` con `--operation-id` ya registrado en el expediente cancelado devuelve el estado actual sin duplicar eventos. | AS-04 |

---

## Deltas sobre baseline

Los comandos `rework` y `cancel` son nuevos. No existe baseline que modifiquen estructuralmente.
Los deltas sobre las specs existentes son:

### Adiciones

- Nuevo comando `sdd-cli rework` con su use case, dominio y handler CLI.
- Nuevo comando `sdd-cli cancel` con su use case, dominio, handler CLI y repositorio de
  cancelación (análogo a `fs_archive_repository.go`).
- Nuevo directorio de persistencia `.sdd/work-items/cancelled/` creado por el motor en el
  primer `cancel`.
- Nuevos tipos de evento: `work_item.reactivated`, `work_item.cancelled`.
- Nueva causa de transición de fase: `rework_requested`, `rework_superseded`.
- Error code reutilizado: `already_exists` para `start` con ID cancelado (con mensaje que distingue el caso de cancelación del de colisión activa).

### Modificaciones en comandos existentes

- `sdd-cli start`: debe inspeccionar `cancelled/<id>/` además de `active/` y `archive/` para
  reserva de IDs.
- `sdd-cli status`, `validate`, `next`: deben detectar IDs en `cancelled/` y devolver un error
  diagnóstico en lugar de `not_found` genérico.
- `domain/work_item.go`: nuevos métodos `ReworkPhase` y `Cancel` (o equivalentes) con las
  transiciones de estado correspondientes.
- `infra/fs_archive_repository.go` (o nuevo `fs_cancel_repository.go`): movimiento transaccional
  a `cancelled/`.

### Sin cambio

- El schema `manifest.yaml` ya modela `WorkItemStatus: cancelled` — no requiere cambio de
  schema.
- El schema `event.schema.json` valida `type` contra un pattern abierto (`^[a-z_]+(?:[.-][a-z_]+)*$`), no una enumeración cerrada — los nuevos tipos `work_item.reactivated` y `work_item.cancelled` son válidos sin cambio de schema.
- Los comandos `begin`, `deliver`, `approve`, `reject`, `complete`, `archive` no requieren
  cambios de lógica central (solo `start` requiere inspección adicional).
- Los `Approval` records y su estructura no se modifican.

---

## Trazabilidad

| Especificación | Fuente |
|---|---|
| CA-RW-01 a CA-RW-12 | PRD `artifacts/prd.md`, sección Criterios de aceptación `rework` |
| CA-CA-01 a CA-CA-10 | PRD `artifacts/prd.md`, sección Criterios de aceptación `cancel` |
| INV-01 a INV-07 | PRD, sección Reglas de negocio; `.sdd/context/specs/cierre-de-work-item.md` |
| Contrato de errores | `.sdd/context/specs/transiciones-de-fase-y-gates.md`; `.sdd/context/specs/cierre-de-work-item.md` |
| Modelo de datos | `.sdd/context/architecture/data-modeling.md` |
| Flujo transaccional | `.sdd/context/architecture/data-and-process-flow.md`; `src/cli/internal/infra/fs_archive_repository.go` |
| Estados de dominio | `src/cli/internal/domain/work_item.go` |
| Supuesto AS-01 (artefactos no modificados) | PRD, sección Supuestos declarados |
| Supuesto AS-02 (estructura `cancelled/`) | PRD, sección Supuestos declarados |
| Supuesto AS-03 (mecanismo transaccional) | PRD, sección Supuestos declarados |
| Supuesto AS-04 (idempotencia `--operation-id`) | PRD, sección Supuestos declarados |
