---
schema_version: "0.1"
kind: artifact
id: "specification"
work_item: "token-usage-audit"
phase: "specification"
status: draft
created_at: "2026-09-28T21:29:30Z"
created_by: { kind: "cli", id: "sdd" }
sources: 
  - artifacts/prd.md
---

# Especificación: Auditoría y registro de consumo de tokens

## Requisitos y comportamiento

### REQ-1 — Contrato público del motor para registro de tokens
_Trazabilidad: RN-1, CA-4_

El motor `sdd-cli` debe exponer un comando público que permita registrar el consumo de
tokens de un work item. Ese comando:

- Recibe como argumentos los contadores de consumo acumulado de la sesión completa:
  `input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_write_tokens` (todos enteros
  >= 0), más el identificador del agente de código (`source`, string no vacío).
- Recibe el identificador del work item destino como argumento o flag (no lo infiere del
  entorno; quien invoca el comando es responsable de proveerlo).
- Persiste los valores recibidos en `observability.token_usage` del manifest del work item,
  sobrescribiendo el valor previo de forma transaccional (atómica).
- No contiene lógica de captura del entorno del LLM: no sabe cómo leer tokens de Claude
  Code ni cuándo ejecutarse. Esa responsabilidad reside exclusivamente en el adapter.
- Valida que los valores cumplen el schema (`work-item.schema.json`) antes de persistir.
  Si la validación falla, retorna un error y el manifest permanece sin cambios.

### REQ-2 — Configurabilidad binaria de la auditoría
_Trazabilidad: RN-2, CA-2, CA-6_

El campo `observability.token_usage` en `.sdd/config.yaml` controla la auditoría con
semántica binaria (activa / inactiva):

- **Auditoría activa:** al crear un work item, su manifest se inicializa con
  `observability.token_usage` presente, `status: "partial"`, todos los contadores
  (`input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_write_tokens`,
  `total_tokens`) en `0` y `source: null`.
- **Auditoría inactiva:** el manifest se inicializa con `observability.token_usage`
  presente, `status: "not_reported"`, todos los contadores en `null` y `source: null`.
  Todo intento de registro sobre un item con auditoría inactiva se rechaza con un error
  manejable (ver E-12 / REQ-6) y no modifica los contadores.

La configuración en vigor al momento de crear el work item es la que rige para toda la
vida del item. Un cambio posterior en config.yaml no afecta work items ya existentes
(RC-5).

**Nota de schema:** el campo actualmente tiene el valor `optional` en `config.yaml`. Por
decisión del usuario (D-3), la activación/desactivación se realiza **editando manualmente**
la propiedad en `config.yaml` (semántica `true`/`false`); no se requiere un mecanismo de
migración automática. El cambio de valor no debe romper el parseo de config existente.

### REQ-3 — Registro acumulativo por recálculo total
_Trazabilidad: RN-3, CA-1_

Cada invocación exitosa al comando de registro debe:

1. Recibir el total acumulado de **toda la sesión** disponible hasta ese momento (no el
   delta del último turno).
2. Sobrescribir (pisar) los contadores previos en el manifest con los valores recibidos.
3. Garantizar que si un turno no fue registrado (fallo del hook), el siguiente registro
   correcto restaure el total de sesión completo, sin pérdida permanente de datos.

**Invariante:** el valor almacenado en `observability.token_usage` siempre representa
el total acumulado conocido hasta el último registro exitoso. Nunca es una suma parcial
de deltas. El adapter es responsable de calcular ese total antes de invocar el motor.

### REQ-4 — Granularidad de registro
_Trazabilidad: RN-4, CA-1_

Cada registro debe persistir los siguientes campos:

| Campo | Tipo | Restricción |
| --- | --- | --- |
| `input_tokens` | entero | >= 0 |
| `output_tokens` | entero | >= 0 |
| `cache_read_tokens` | entero | >= 0 |
| `cache_write_tokens` | entero | >= 0 |
| `total_tokens` | entero | >= 0; igual a la suma de los cuatro contadores |
| `source` | string | no vacío; identifica al agente de código |

El **total de consumo** (`total_tokens`) se **almacena** como campo del schema (decisión del
usuario, D-1) y debe ser igual a `input_tokens + output_tokens + cache_read_tokens +
cache_write_tokens`. La adición del campo al schema debe ser **aditiva** (sin ruptura). Con
auditoría inactiva, `total_tokens` (como el resto de contadores) es `null` por defecto.

### REQ-5 — Alcance de sesión y condición de uso
_Trazabilidad: RN-5_

La auditoría captura el consumo de **toda la sesión** (agente principal, orquestador y
subagentes que operen dentro de la misma sesión de Claude Code). Lo que ocurre fuera de
esa sesión —otras sesiones, otras ramas/worktrees— no se contabiliza.

El sistema no verifica ni impone la exclusividad sesión/work item. Esta condición es
**responsabilidad del usuario** y se sustenta en el aislamiento por rama/worktree que
ya provee el flujo SDD. Debe quedar documentada como condición de uso en el adapter.

### REQ-6 — Recuperabilidad ante fallas del mecanismo de captura
_Trazabilidad: RN-6, CA-5_

Cuando el mecanismo automático de captura falla (error en el hook del adapter, rechazo
de validación del motor, archivo `.active-work-item` inválido, u otro):

- El error debe quedar registrado en un archivo de diagnóstico persistente dentro del
  worktree, con información suficiente para identificar causa, momento y contexto
  del fallo. La ubicación exacta y el formato del archivo son decisiones del plan (D-6).
- El manifest del work item no debe quedar en estado inconsistente: o el registro se
  completa exitosamente, o el manifest permanece con los valores previos intactos.
- El usuario puede invocar el comando público del motor (REQ-1) manualmente para
  registrar el consumo como mecanismo de respaldo.

### REQ-7 — Identificación del work item activo vía `.active-work-item`
_Trazabilidad: RN-7_

El adapter resuelve el work item destino de cada registro leyendo el archivo
`.active-work-item` ubicado en la **raíz del worktree** de la sesión activa. Este archivo:

- Contiene el identificador (ID) del work item activo, en formato kebab-case.
- Debe existir antes de que el adapter intente cualquier registro.
- Debe ser creado al iniciarse el trabajo en ese worktree (su creación y gestión es
  responsabilidad del flujo SDD y del adapter, no del motor).

Si el archivo no existe, está vacío, o contiene un ID que no corresponde a ningún work
item activo en el proyecto, el adapter no debe intentar el registro y debe emitir un
error diagnosticable (REQ-6).

### REQ-8 — Ciclo de vida del `status` de la auditoría
_Trazabilidad: RN-2, RN-6, CA-1, CA-5_

El campo `observability.token_usage.status` describe el estado de **la auditoría** del work
item (no el estado de fase/ciclo de vida del work item). Su semántica es:

| `status` | Condición |
| --- | --- |
| `not_reported` | Auditoría inactiva (no aplica); contadores en `null`. |
| `partial` | Auditoría activa, pero el último ciclo de captura registró un error (o aún no hubo un registro exitoso desde la creación). Los datos pueden estar incompletos. |
| `recorded` | Auditoría activa y el último registro se completó sin error. Los contadores reflejan el total acumulado consistente de la sesión. |

Transiciones: un registro exitoso lleva el `status` a `recorded`; un error del mecanismo de
captura (hook o validación) lo lleva (o lo mantiene) en `partial`. Con auditoría inactiva el
`status` permanece en `not_reported` durante toda la vida del work item.

---

## Escenarios verificables

### E-01 — Registro exitoso: los contadores se sobrescriben correctamente

**Dado** que la auditoría está activa, existe un work item activo identificado por
`.active-work-item`, y el manifest contiene `input_tokens=200`, `output_tokens=80`,
`cache_read_tokens=0`, `cache_write_tokens=0`,

**Cuando** se invoca el comando público del motor con `input_tokens=500`,
`output_tokens=300`, `cache_read_tokens=100`, `cache_write_tokens=50`,
`source="claude_code"`,

**Entonces** el manifest del work item refleja exactamente: `input_tokens=500`,
`output_tokens=300`, `cache_read_tokens=100`, `cache_write_tokens=50`,
`source="claude_code"`. Los valores anteriores han sido completamente sobrescritos.

### E-02 — Inicialización del campo al crear un work item con auditoría activa

**Dado** que la auditoría está activa en config.yaml,

**Cuando** se crea un nuevo work item (`sdd-cli start`),

**Entonces** el manifest del nuevo work item incluye `observability.token_usage` con
`status: "partial"`, todos los contadores (incluido `total_tokens`) en `0` y `source: null`.
El work item queda en condiciones de recibir registros de consumo desde el primer turno.

### E-03 — Auditoría inactiva: ningún campo de consumo se crea ni se modifica

**Dado** que la auditoría está inactiva en config.yaml,

**Cuando** se crea un work item y luego se intenta registrar tokens (vía comando del
motor o vía adapter),

**Entonces** el manifest incluye `observability.token_usage` con `status: "not_reported"`
y todos los contadores en `null`; el intento de registro se rechaza con un **error
manejable** (ver E-12) y los contadores permanecen en `null`.

### E-04 — Recuperación de turno no registrado por el diseño acumulativo

**Dado** que un work item tiene `input_tokens=200`, `output_tokens=100` (total acumulado
registrado al turno 2), y el turno 3 no fue registrado por un fallo del hook,

**Cuando** el adapter procesa el turno 4 y registra el total acumulado real de la sesión
completa con `input_tokens=600`, `output_tokens=350`,

**Entonces** el manifest refleja `input_tokens=600`, `output_tokens=350`, recuperando
los valores correctos de los 4 turnos sin pérdida permanente. No existe brecha silenciosa
en el total final.

### E-05 — Registro manual como mecanismo de respaldo

**Dado** que el hook automático falló y generó un log de error diagnosticable,

**Cuando** el usuario invoca manualmente el comando público del motor con los valores
correctos del consumo acumulado de la sesión,

**Entonces** el manifest se actualiza correctamente con los nuevos valores, y el work
item no queda en estado inconsistente.

### E-06 — Fallo del motor ante valores inválidos: manifest queda intacto

**Dado** que la auditoría está activa y el work item tiene `input_tokens=200`,

**Cuando** se intenta registrar un valor negativo (p. ej. `input_tokens=-10`) o un
tipo de dato incorrecto,

**Entonces** el motor rechaza el registro con un error de validación (código
`validation_failed`), el manifest permanece con `input_tokens=200`, y el adapter
registra el error en el archivo de diagnóstico (REQ-6).

### E-07 — Múltiples sesiones paralelas: aislamiento sin ambigüedad

**Dado** que existen dos sesiones activas, cada una en su propio worktree con su
respectivo `.active-work-item` (worktree A → work item `feature-a`; worktree B →
work item `feature-b`),

**Cuando** cada sesión registra tokens de forma independiente,

**Entonces** los tokens de la sesión A se persistente exclusivamente en el manifest de
`feature-a`, y los de la sesión B exclusivamente en el de `feature-b`. No existe
contaminación cruzada entre work items.

### E-08 — Archivo `.active-work-item` ausente o ID inválido

**Dado** que el archivo `.active-work-item` no existe en la raíz del worktree, o contiene
un ID que no corresponde a ningún work item activo,

**Cuando** el adapter intenta registrar tokens,

**Entonces** el registro no se realiza, se genera un error diagnosticable (archivo de log
con causa y timestamp), y ningún manifest es modificado.

### E-09 — Registro con todos los contadores en cero es válido

**Dado** que la auditoría está activa y existe un work item activo,

**Cuando** se registran tokens con `input_tokens=0`, `output_tokens=0`,
`cache_read_tokens=0`, `cache_write_tokens=0`,

**Entonces** el manifest persiste esos valores (estado válido). El motor no rechaza ni
trata como error un registro de cero.

### E-10 — Consulta del consumo de tokens sin acceso directo al manifest

**Dado** que un work item tiene consumo de tokens registrado,

**Cuando** se consulta el estado del work item (vía `sdd-cli status` o equivalente),

**Entonces** los valores de `input_tokens`, `output_tokens`, `cache_read_tokens`,
`cache_write_tokens` y `source` son visibles en la salida del comando, sin necesidad
de leer directamente el YAML del manifest.

### E-11 — Activar auditoría no afecta work items preexistentes

**Dado** que la auditoría estaba inactiva y existen work items activos sin campo
`observability.token_usage`,

**Cuando** se activa la auditoría en config.yaml,

**Entonces** los work items existentes no se modifican retroactivamente. Solo los work
items creados a partir de ese momento se inicializan con el campo de auditoría.

### E-12 — Registro sobre work item con auditoría inactiva: error o no-op

**Dado** que un work item fue creado con auditoría inactiva (manifest sin campo
`observability.token_usage`),

**Cuando** el adapter o el usuario intenta registrar tokens en ese work item,

**Entonces** la CLI devuelve un **error manejable** (no una excepción no controlada) y no
modifica el manifest. Como el script del adapter no expone su salida al usuario, el adapter
debe volcar ese error al **archivo de logs** (RC-7), del mismo modo que cualquier otro error
manejable, para que quede visible/diagnosticable (decisión del usuario, D-7).

---

## Reglas, contratos y restricciones

### RC-1 — Schema del campo `observability.token_usage`

El campo debe cumplir el schema existente de `work-item.schema.json` (definición
`observability > token_usage`):

| Campo | Tipo | Requerido | Restricción |
| --- | --- | --- | --- |
| `status` | string | sí | `"not_reported"` \| `"partial"` \| `"recorded"` |
| `source` | string \| null | no | — |
| `input_tokens` | integer \| null | no | >= 0 |
| `output_tokens` | integer \| null | no | >= 0 |
| `cache_read_tokens` | integer \| null | no | >= 0 |
| `cache_write_tokens` | integer \| null | no | >= 0 |
| `total_tokens` | integer \| null | no | >= 0; suma de los cuatro contadores |

`total_tokens` se **agrega** al schema (D-1) como campo aditivo (no rompe manifests
existentes, al ser opcional/nullable). Los contadores nunca pueden ser negativos: el schema
lo prohíbe y el motor lo valida antes de persistir. Cuando la auditoría está inactiva, todos
los contadores son `null` y `status` es `not_reported` (ver REQ-2 y REQ-8).

### RC-2 — Invariante de sobrescritura total

Cada llamada exitosa al comando de registro resulta en un manifest donde los contadores
reflejan **exclusivamente** los valores recibidos en esa llamada. El motor no acumula
internamente: el adapter es quien calcula el total acumulado de la sesión y lo pasa como
parámetro.

### RC-3 — Motor libre de lógica de captura del entorno LLM

El motor (`sdd-cli`) no puede contener código que lea métricas de consumo del entorno
de Claude Code, interprete transcripciones de sesión, ni determine cuándo disparar un
registro. Esa lógica reside exclusivamente en el adapter. El motor solo persiste los
valores que recibe.

### RC-4 — Atomicidad e integridad transaccional del registro

La escritura en el manifest al registrar tokens usa el mecanismo transaccional existente
del motor (staging + fsync + rename + backup/recovery). O el valor se persiste de forma
completa, o el manifest permanece con el valor previo. Un manifest parcialmente
actualizado es un estado inválido que no debe ser posible.

### RC-5 — La configuración rige al momento de creación del work item

El valor de `observability.token_usage` en `config.yaml` vigente al crear el work item
determina si ese item tendrá o no auditoría activa. Un cambio posterior en config.yaml
no afecta retroactivamente a work items ya existentes.

### RC-6 — Resolución del work item por `.active-work-item`

El archivo `.active-work-item` debe ubicarse en la **raíz del worktree** (mismo nivel
que `.sdd/`). Su contenido debe ser el ID del work item en formato kebab-case
(`^[a-z0-9]+(?:-[a-z0-9]+)*$`). Ante cualquier condición inválida (archivo ausente,
contenido vacío, ID malformado, ID inexistente en work items activos), el adapter debe
abortar el registro y emitir un error diagnosticable.

### RC-7 — Registro de errores diagnosticable

Los errores del mecanismo de captura deben quedar en archivos de log persistentes dentro
del worktree con, al mínimo: timestamp, tipo o código de error, y contexto del fallo. El
archivo de log no debe interferir con el estado del work item ni con el manifest. La
ubicación exacta, formato y política de rotación son decisiones de implementación del
plan (D-6).

### RC-8 — Identificador del agente (`source`)

El campo `source` debe ser un string no vacío que identifique al agente de código. Debe
ser consistente entre todos los registros de una misma sesión. El valor concreto para el
adapter de Claude Code es una decisión del plan.

### RC-9 — Idempotencia por `operation-id`

El comando de registro debe respetar el mecanismo de idempotencia del motor
(`--operation-id`): si se reintenta una llamada con el mismo `operation-id`, el motor
debe reconocer la operación ya ejecutada y no duplicar el evento en `events.jsonl`.

---

## Deltas sobre baseline

Esta especificación introduce comportamiento nuevo sobre una estructura ya modelada en el
schema y el dominio pero completamente inactiva. No modifica ni elimina comportamiento
observable existente.

### Añadido

| ID | Descripción |
| --- | --- |
| A-1 | Comando público del motor para registrar consumo de tokens en un work item |
| A-2 | Inicialización de `observability.token_usage` al crear un work item con auditoría activa (contadores en 0) |
| A-3 | Validación del campo `observability.token_usage` en config.yaml como control binario |
| A-4 | Mecanismo de captura automática en el adapter de Claude Code (hook post-turno) |
| A-5 | Archivo `.active-work-item` para resolución del work item destino por sesión/worktree |
| A-6 | Registro de errores diagnosticable del mecanismo de captura en el adapter |
| A-7 | Exposición del consumo de tokens en la salida de `sdd-cli status` (CA-3) |
| A-8 | Campo `total_tokens` (aditivo, nullable) en el schema de `observability.token_usage` (D-1) |

### Modificado

| ID | Descripción | Baseline |
| --- | --- | --- |
| M-1 | Campo `observability.token_usage` en config.yaml: de `optional` a control binario (`true`/`false` o semántica equivalente) | `observability.token_usage: optional` en `.sdd/config.yaml` |
| M-2 | Comportamiento de `sdd-cli start`: inicializa `observability.token_usage` si la auditoría está activa | Actualmente no toca el campo de observabilidad al crear un work item |

### Eliminado

Ninguno. No se elimina comportamiento existente.

---

## Dependencias y preguntas abiertas para la fase de plan

El usuario cerró **D-1, D-3, D-4, D-5 y D-7** (ya incorporadas arriba). Quedan abiertas
**D-2 y D-6**, pendientes de investigación empírica o decisión de diseño en la fase de plan.
Las cerradas ya están reflejadas en los requisitos; las abiertas no bloquean la especificación,
pero bloquean el inicio de la implementación si no se responden.

**D-1 — Campo `total_tokens` en el schema. ✅ CERRADA (usuario).**
Se **agrega `total_tokens` al schema** como campo aditivo/nullable (ver REQ-4, RC-1). Con
auditoría inactiva es `null`. El plan sólo debe implementar la adición confirmando que es
retrocompatible.

**D-2 — Supuesto S-1: soporte de hooks post-turno en el adapter de Claude Code (P-1
del PRD).**
El mecanismo de captura automática asume que el adapter soporta hooks ejecutables al
finalizar cada turno del agente (principal y orquestador). Esta capacidad debe verificarse
empíricamente antes de diseñar el mecanismo. Si los hooks no son viables, el diseño de
captura debe redefinirse.

**D-3 — Compatibilidad del cambio de config (M-1). ✅ CERRADA (usuario).**
No se implementa mecanismo de migración automática: el usuario **edita manualmente** la
propiedad `observability.token_usage` en `config.yaml` (`true`/`false`). El plan sólo debe
asegurar que el parser de config acepta el valor binario sin romper el parseo existente.

**D-4 — Granularidad de un "turno del agente" (P-2 del PRD). ✅ CERRADA (usuario).**
No requiere definición precisa para garantizar la correctitud: sea cual sea el alcance de un
"turno", el script **siempre recalcula y sobrescribe el total acumulado de la sesión** (REQ-3/
RC-2), de modo que **nunca se pierden tokens** ni queda una brecha permanente. El plan elige
un punto de disparo del hook por conveniencia, pero la integridad de los datos no depende de
esa elección.

**D-5 — Transición al estado `recorded`. ✅ CERRADA (usuario).**
`status` es un estado de **la auditoría** (no del work item). Semántica definida en REQ-8:
`recorded` si el último registro fue exitoso; `partial` si hubo error de captura (o aún no
hubo registro exitoso, con auditoría activa); `not_reported` si la auditoría está inactiva.

**D-6 — Ubicación y formato del log de errores (REQ-6, RC-7).**
La spec requiere que los errores del mecanismo de captura queden en un archivo de
diagnóstico persistente dentro del worktree. La fase de plan debe definir: ruta exacta
(raíz del worktree, `.sdd/`, otro directorio), formato del archivo (texto plano, JSON),
esquema de nombre (p. ej. con timestamp y nombre del adapter), y política de retención.

**D-7 — Registro en work item con auditoría inactiva (E-12). ✅ CERRADA (usuario).**
La CLI devuelve un **error manejable** (no una excepción no controlada) y no modifica el
manifest. Como la salida del script no la ve el usuario, el adapter debe volcar ese error al
**archivo de logs** (RC-7), igual que cualquier otro error manejable. El plan define la ruta/
formato del log (D-6).

---

## Trazabilidad

| Ítem de especificación | Criterio PRD | Regla de negocio |
| --- | --- | --- |
| REQ-1, A-1, RC-9 | CA-4 | RN-1 |
| REQ-2, A-2, A-3, M-1, M-2, RC-5, E-02, E-03, E-11 | CA-2, CA-6 | RN-2 |
| REQ-3, RC-2, E-01, E-04 | CA-1 | RN-3 |
| REQ-4, RC-1, RC-8, A-8, E-09 | CA-1 | RN-4 |
| REQ-8 | CA-1, CA-5 | RN-2, RN-6 |
| REQ-5 | — | RN-5 |
| REQ-6, A-6, RC-7, E-05, E-06, E-08 | CA-5 | RN-6 |
| REQ-7, A-5, RC-6, E-07, E-08 | CA-1, CA-5 | RN-7 |
| A-7, E-10 | CA-3 | — |
| RC-3 | — | RN-1 |
| RC-4 | CA-5 | RN-6 |
| E-12 | — | RN-2 (frontera de borde) |

Artefacto fuente: `artifacts/prd.md` (aprobado, fase `prd` del work item
`token-usage-audit`).
