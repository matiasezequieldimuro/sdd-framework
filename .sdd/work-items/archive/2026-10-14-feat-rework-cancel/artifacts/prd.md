---
schema_version: "1.0"
kind: artifact
id: "prd"
work_item: "feat-rework-cancel"
phase: "prd"
status: draft
created_at: "2026-10-07T00:20:17Z"
created_by: { kind: "cli", id: "sdd" }
sources: []
---

# PRD: Comandos `rework` y `cancel` para work items

## Problema y objetivo

### Problema

`sdd-cli` gobierna el ciclo de vida de los work items como una máquina de estados. Hoy, los
mecanismos disponibles para deshacer o redirigir el trabajo son limitados:

- `reject` retrocede una única fase desde `awaiting_approval` a `rejected`, para que el agente
  la rehaga. Pero no resuelve el caso en que la decisión de retrabajo llega **antes** de que la
  fase llegue al gate, o cuando el punto de retorno necesario es una fase anterior a la que está
  en curso.
- No existe ningún mecanismo para **abandonar formalmente** un work item. El dominio modela
  `cancelled` como un estado válido de `WorkItemStatus` y `superseded` como un estado válido de
  `PhaseStatus`, pero ningún comando público los produce. Dejar un work item activo sin cerrar
  genera ruido en el registry y no queda registro auditable de la decisión.

Estos dos huecos obligan a los equipos a editar manifests a mano (prohibido por las reglas de
operación del motor) o a abandonar work items sin cierre formal, degradando la trazabilidad.

### Objetivo

Exponer los dos mecanismos de control de ciclo de vida que el dominio ya contempla pero que
carecen de interfaz pública:

1. **`rework`**: permite retomar un work item desde una fase anterior, invalidando el trabajo
   descendiente de forma controlada y auditable.
2. **`cancel`**: permite cerrar formalmente un work item que no se va a completar, dejando un
   registro inmutable de la decisión y liberando el registry de expedientes activos obsoletos.

Ninguno de los dos comandos implica borrar historial ni silenciar decisiones previas.

---

## Alcance y no alcance

### En alcance

- Comando `sdd-cli rework <id> --phase <fase> --reason <texto>`.
- Comando `sdd-cli cancel <id> --reason <texto>`.
- Emisión de eventos de auditoría para cada transición producida por ambos comandos.
- Reserva permanente del ID de un work item cancelado (el motor debe rechazar futuros `start`
  con el mismo ID).
- Comportamiento ante work items en estado `completed` para `rework` (reactivación).

### Fuera de alcance

- Reapertura de work items `archived`. Un work item archivado es inmutable; si se requiere
  retrabajo, se abre un nuevo work item vinculado al archivado. Esta restricción es análoga a
  la que aplica para `archive` y preserva la inmutabilidad del historial archivado.
- Reapertura de work items `cancelled`. Una vez cancelado, el expediente no se reactiva.
- Propagación automática de `superseded` sin comando explícito (se resuelve con este work item).
- Modificación retroactiva de registros de aprobación (`Approval` records).
- Nuevo workflow o fase específica para gestionar el retrabajo o la cancelación.
- Exposición de estos comandos vía MCP (fuera de alcance de la BETA, según `CHANGELOG.md`).

---

## Reglas de negocio

### `rework`

**RB-RW-01. Fase objetivo pasa a `ready`.**
La fase indicada en `--phase` pasa al estado `ready`. No pasa directamente a `in_progress`.
El agente debe ejecutar `begin` explícitamente para iniciar el trabajo. Esto es consistente
con el ciclo de estados establecido y preserva la semántica de `begin` como inicio voluntario.

**RB-RW-01b. Estados de origen válidos para la fase objetivo.**
`rework` solo es válido si la fase objetivo se encuentra en estado `completed`, `approved`
o `accepted` — es decir, fases que fueron efectivamente trabajadas y completadas en algún
grado. Si la fase está en cualquier otro estado (`ready`, `in_progress`, `not_applicable`,
`blocked`, `superseded`, `rejected`, `awaiting_approval`), el motor falla con un error de
transición inválida. Una fase `not_applicable` nunca fue trabajada, por lo tanto tampoco es
un target válido de `rework`.

**RB-RW-02. Propagación selectiva de `superseded` hacia los descendientes.**
Para cada fase descendiente transitiva (directa e indirecta) en el DAG, el resultado depende
de su estado al momento del `rework`:

| Estado del descendiente | Resultado |
|---|---|
| `completed`, `approved`, `accepted`, `in_progress`, `awaiting_approval`, `rejected` | → `superseded` (fue trabajada) |
| `ready` | → `blocked` (desbloqueada pero no iniciada; se re-bloquea) |
| `blocked` | queda `blocked` (sin cambio) |
| `not_applicable` | queda `not_applicable` (sin cambio; fue bypassed intencionalmente) |
| `superseded` | queda `superseded` (idempotente) |

Solo pasan a `superseded` las fases sobre las que el agente efectivamente actuó. Las fases
que nunca fueron iniciadas no se marcan como supersedidas; en su lugar, `ready` retrocede a
`blocked` para reflejar que su dependencia ya no está satisfecha.

**RB-RW-03. Los registros de aprobación no se modifican.**
Los `Approval` records existentes en el manifest se preservan íntegros. El historial de
decisiones anteriores permanece visible. El retrabajo no borra evidencia, la supersede.

**RB-RW-04. `--reason` es obligatorio.**
El flag `--reason` con texto libre es requerido. El motor rechaza la invocación sin él.
El texto se incluye en los eventos de auditoría emitidos.

**RB-RW-05. Reactivación desde `completed`.**
Si el work item estaba en estado `completed`, el comando `rework` lo devuelve al estado
`active` antes de aplicar las transiciones de fase. Un work item `completed` puede ser
objeto de retrabajo.

**RB-RW-06. Work items `archived` y `cancelled` no se reabren.**
Si el work item está en estado `archived` o `cancelled`, el comando falla con un error claro.
Ambos son estados finales e irreversibles. El retrabajo sobre trabajo archivado o cancelado
requiere abrir un nuevo work item.

**RB-RW-07. Auditoría por transición.**
El comando emite al menos un evento auditable por cada fase que transiciona (`superseded`,
`ready` y, si aplica, la reactivación del work item). El campo `reason` del invocador se
propaga a los eventos.

**RB-RW-08. La fase objetivo debe existir en el workflow.**
Si la fase indicada en `--phase` no existe en el manifest del work item, el motor falla con
un error de validación.

---

### `cancel`

**RB-CA-01. Solo se cancela un work item `active`.**
El comando solo opera sobre work items en estado `active`. Si el item está en `completed` o
`archived`, el motor rechaza la invocación con un error que indica el estado actual.

**RB-CA-02. Cualquier actor puede cancelar.**
No se requiere actor `human`. Un agente puede ejecutar `cancel`. Esto contrasta con `approve`
y `reject`, que exigen actor humano, porque la cancelación es una decisión de abandono, no
una resolución de gate de calidad.

**RB-CA-03. `--reason` es obligatorio.**
El flag `--reason` con texto libre es requerido. El motor rechaza la invocación sin él.
El texto se incluye en el evento de auditoría.

**RB-CA-04. Las fases en curso permanecen en su estado actual.**
Al cancelar, el estado de cada fase individual no se modifica. Las fases que estaban
`in_progress`, `awaiting_approval`, `ready`, etc., quedan en ese estado en el snapshot
del manifest cancelado. No se fuerza ninguna transición de fase como parte del cancel.

**RB-CA-05. El expediente se mueve a `.sdd/work-items/cancelled/<id>/`.**
El directorio físico del work item se mueve desde `active/<id>/` al nuevo directorio
`cancelled/<id>/`. El patrón es análogo al de `archive/` pero sin fecha en el nombre
(porque el ID es único y el directorio de cancelados no necesita desambiguación temporal).
Este movimiento debe ser transaccional (con lock y recovery ante interrupción).

**RB-CA-06. El ID queda reservado permanentemente.**
Una vez cancelado, el ID del work item queda reservado. Un futuro `sdd-cli start` con el
mismo ID debe fallar con un error que indique que el ID ya fue utilizado y está cancelado.
El motor debe inspeccionar tanto `active/`, `archive/` como `cancelled/` al validar unicidad.

**RB-CA-07. Se emite evento `work_item.cancelled`.**
El motor emite un único evento de auditoría `work_item.cancelled` que incluye el `reason`,
el actor y el timestamp. El evento queda en el `events.jsonl` del expediente cancelado.

---

## Criterios de aceptación

### `rework`

| ID | Criterio |
|---|---|
| CA-RW-01 | `rework <id> --phase <fase> --reason <texto>` con work item `active` y fase existente: la fase objetivo queda en `ready`; descendientes en estado trabajado (`completed`/`approved`/`accepted`/`in_progress`/`awaiting_approval`/`rejected`) quedan en `superseded`; descendientes en `ready` quedan en `blocked`; descendientes en `blocked` o `not_applicable` no cambian; el work item permanece `active`. |
| CA-RW-02 | Tras `rework`, ejecutar `begin --phase <fase>` sobre la fase objetivo es posible y la transiciona a `in_progress`. |
| CA-RW-03 | `rework <id> --phase <fase> --reason <texto>` con work item `completed`: el work item pasa a `active` y se aplican las mismas transiciones de fase que en CA-RW-01. |
| CA-RW-04 | `rework <id> --phase <fase>` sin `--reason`: el motor rechaza con error de argumento requerido. |
| CA-RW-05 | `rework <id> --phase <fase-inexistente> --reason <texto>`: el motor rechaza con error de fase no encontrada. |
| CA-RW-06 | `rework <id> --phase <fase> --reason <texto>` con work item `archived` o `cancelled`: el motor rechaza con error que indica el estado actual (ambos son estados finales). |
| CA-RW-10 | `rework <id> --phase <fase> --reason <texto>` con la fase objetivo en estado `ready`, `in_progress`, `not_applicable`, `blocked`, `superseded`, `rejected` o `awaiting_approval`: el motor rechaza con error de transición inválida. Solo `completed`, `approved` y `accepted` son estados de origen válidos. |
| CA-RW-11 | Fases en estado `not_applicable` o `blocked` que sean descendientes de la fase objetivo no reciben `superseded`; permanecen en su estado actual. Fases en `ready` retroceden a `blocked`. |
| CA-RW-07 | Los registros de aprobación (`Approval` records) presentes antes del `rework` no son modificados ni eliminados del manifest. |
| CA-RW-08 | El manifest resultante contiene eventos auditables para cada fase transicionada, incluyendo el `reason`. |
| CA-RW-09 | Fases descendientes ya en estado `superseded` antes del `rework` no generan un segundo evento de transición (idempotencia). |

### `cancel`

| ID | Criterio |
|---|---|
| CA-CA-01 | `cancel <id> --reason <texto>` con work item `active`: el expediente se mueve a `.sdd/work-items/cancelled/<id>/`, el manifest queda con `status: cancelled`, y se emite el evento `work_item.cancelled` con el `reason`. |
| CA-CA-02 | `cancel <id> --reason <texto>` con work item `completed`: el motor rechaza con error que indica que el item está `completed`. |
| CA-CA-03 | `cancel <id> --reason <texto>` con work item `archived`: el motor rechaza con error que indica que el item está `archived`. |
| CA-CA-04 | `cancel <id>` sin `--reason`: el motor rechaza con error de argumento requerido. |
| CA-CA-05 | Tras `cancel <id>`, `sdd-cli start <id>` falla con error que indica que el ID está reservado (cancelado). |
| CA-CA-06 | Las fases del work item cancelado conservan sus estados originales en el manifest (ninguna fase cambia de estado por efecto del cancel). |
| CA-CA-07 | El cancel puede ser ejecutado con actor `agent` (no requiere `human`). |
| CA-CA-08 | Si el proceso es interrumpido durante el movimiento del expediente, el motor recupera un estado consistente (sin expediente parcialmente movido) al reintentar. |
| CA-CA-09 | `sdd-cli status <id>` ejecutado sobre un work item cancelado falla con error que indica que el ID corresponde a un work item cancelado. El expediente en `cancelled/` es opaco para los comandos de lectura. |

---

## Riesgos y preguntas abiertas

### Decisiones de diseño resueltas

**DD-01. Visibilidad de work items cancelados vía comandos de lectura.**
`status`, `validate` y `next` devuelven error si el ID apunta a un expediente en `cancelled/`.
Un work item cancelado es opaco para los comandos de lectura — no se inspecciona ni se reporta
su contenido. El ID sigue reservado (detectable por `start`), pero el expediente en sí no es
accesible vía la interfaz pública del motor.

**DD-02. Estados de origen válidos para la fase objetivo de `rework`.**
`rework` solo acepta fases en estado `completed`, `approved` o `accepted` (fases efectivamente
trabajadas). Cualquier otro estado — incluyendo `ready`, `in_progress`, `not_applicable`,
`blocked`, `superseded`, `rejected`, `awaiting_approval` — provoca un error de transición
inválida. Una fase `not_applicable` nunca fue trabajada, por lo tanto no es un target válido.

**DD-03. Propagación de `superseded` sobre fases `not_applicable`.**
Las fases `not_applicable` nunca reciben la propagación de `superseded` por `rework`.
Fueron bypassed intencionalmente al iniciar el work item desde un entry point posterior, y
`rework` no modifica esa condición. En la práctica, en los workflows lineales actuales las
fases `not_applicable` son ancestros del entry point y nunca aparecen como descendientes de
una fase trabajada; pero la regla aplica en general.

**DD-04. `cancelled` como estado final irreversible.**
`cancelled` es un estado final al mismo nivel que `archived`. `rework` falla con error si el
work item está `cancelled`. Un work item cancelado no se reactiva; si se requiere continuar
ese trabajo, se abre un nuevo work item.

### Supuestos declarados

**AS-01.** El comando `rework` opera exclusivamente sobre fases; no modifica ni elimina artifacts
escritos por fases previas. Los artefactos quedan en el filesystem y el agente es responsable de
sobreescribirlos durante el nuevo `begin`/`deliver`.

**AS-02.** La estructura de `cancelled/` es un nuevo directorio de primer nivel bajo
`.sdd/work-items/`, paralelo a `active/` y `archive/`. No se introduce ningún subdirectorio
adicional (ej. sin subcarpeta por fecha).

**AS-03.** El movimiento de un expediente cancelado usa el mismo mecanismo transaccional (lock +
staging + rename atómico + recovery) que el de `archive`, adaptado a la ruta de destino distinta.

**AS-04.** Los comandos `rework` y `cancel` son idempotentes vía `--operation-id` con la misma
semántica que los comandos existentes: si el evento ya fue registrado con ese `correlation_id`,
el motor devuelve el estado actual sin re-transicionar.

**AS-05.** El archivo `docs/internal/9-guia-tecnica-cli.md` referenciado en el briefing del
orquestador no existe en el worktree en la fecha de redacción de este PRD. El contenido
relevante de ese documento (secciones 12, 13, 14, 34) no pudo consultarse. Este PRD se basa
en las specs reconstruidas disponibles en `.sdd/context/specs/` y los requerimientos acordados
con el usuario provistos por el orquestador.

### Riesgos

**RK-01. Integridad del DAG en `rework`.**
La propagación transitiva de `superseded` requiere recorrer el DAG de dependencias del
workflow. Si el DAG tiene ciclos (no deberían existir por diseño) o si la relación de
dependencias no está completamente representada en el manifest, la propagación puede ser
incompleta. Riesgo mitigado por la validación de workflows al momento de `start`.

**RK-02. Reserva de IDs cancelados en comandos existentes.**
El comando `start` deberá inspeccionar `cancelled/` además de `active/` y `archive/` para
detectar IDs reservados. Si esta verificación no se incorpora correctamente, un `start` con
un ID previamente cancelado podría silenciosamente reutilizarlo. Riesgo de corrupción de
trazabilidad.

**RK-03. Consistencia del movimiento transaccional de `cancel`.**
Si el proceso es interrumpido durante el movimiento del expediente (entre el rename de
`active/<id>` y la escritura en `cancelled/<id>`), el expediente podría quedar en un estado
inconsistente. El mecanismo de recovery debe contemplar este directorio de destino.
