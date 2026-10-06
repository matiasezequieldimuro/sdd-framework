---
schema_version: "0.1"
kind: artifact
id: "prd"
work_item: "token-usage-audit"
phase: "prd"
status: draft
created_at: "2026-09-28T21:10:00Z"
created_by: { kind: "cli", id: "sdd" }
sources: []
---

# PRD: Auditoría y registro de consumo de tokens

## Problema y objetivo

### Problema

El SDD Framework en su estado actual (`v0.1.0-beta`) no registra ni expone información sobre
el consumo de tokens que genera la operación de un work item. Esto impide saber:

- **Cuánto cuesta una feature** en términos de recursos de LLM, acumulado a lo largo de todas
  sus fases (PRD, especificación, plan, implementación, verificación, etc.).
- **Cuánto costó el proyecto completo** una vez finalizado: sin datos por work item no hay
  forma de estimar el gasto total.

El manifest y el schema del motor ya modelan la estructura `observability.token_usage` como
campo opcional/futuro, pero ningún componente la alimenta con datos reales. La funcionalidad
queda declarada pero completamente inactiva.

### Objetivo

Habilitar la **auditoría y registro del consumo de tokens** (de entrada, de salida, de caché
y total) por work item, acumulado a lo largo de todas sus fases, con las siguientes metas:

1. Permitir estimar el costo de recursos de LLM de una feature/escenario completo.
2. Permitir estimar el gasto total aproximado del proyecto al momento de su cierre.
3. Ofrecer trazabilidad del agente de código que generó el consumo.
4. Proveer un mecanismo de registro que sea accionable tanto automáticamente (vía adapter)
   como manualmente (vía comando del motor).

### Beneficiarios

| Perfil | Beneficio |
| --- | --- |
| Desarrollador que usa SDD | Conoce el costo en tokens de cada feature que entrega. |
| Responsable del proyecto | Puede estimar el gasto total acumulado al cierre. |
| Autor/mantenedor del framework | Tiene evidencia para evaluar la eficiencia del proceso SDD en términos de uso de LLM. |

---

## Alcance y no alcance

### En alcance

- Registro del consumo de tokens (input, output, cache, total) por work item, acumulado
  a lo largo de todas sus fases.
- Identificación del agente de código que generó el consumo (p. ej., Claude Code).
- Mecanismo de activación/desactivación de la auditoría mediante configuración del proyecto.
- Comportamiento diferenciado según estado de configuración:
  - Auditoría activa → el campo de consumo se inicializa en cero y se actualiza.
  - Auditoría inactiva → el campo de consumo queda en `null`/no reportado.
- Comando público del motor para registrar consumo de tokens de forma manual, como alternativa
  o respaldo al registro automático del adapter.
- Registro de errores o fallas del mecanismo de captura, accesible para diagnóstico posterior.
- Mecanismo para que el adapter identifique a qué work item pertenece la sesión activa.
- Captura realizada desde el **adapter de Claude Code** (adapter en BETA), tomando el consumo de
  **toda la sesión**: agente principal, orquestador y subagentes.

### Fuera de alcance

- Cálculo de costo monetario (conversión de tokens a dólares u otra moneda).
- Dashboards, visualizaciones o reportes gráficos de consumo.
- Integración con APIs de facturación de proveedores de LLM.
- Soporte de auditoría para adapters distintos a Claude Code (otros adapters son futura
  consideración; este work item cubre exclusivamente el adapter de Claude Code).
- Contabilización de actividad ocurrida fuera de la sesión auditada (otras sesiones, otras ramas/
  worktrees): sólo cuenta lo que sucede dentro de la sesión atada al work item.
- Auditoría de sesiones anteriores a la activación de la funcionalidad (los datos históricos
  pre-feature quedan como `not_reported`).

---

## Reglas de negocio

**RN-1 — Motor limpio, sin acople al adapter.**
El motor `sdd-cli` debe permanecer libre de lógica específica de ningún adapter ni agente de
código. La inteligencia de captura (cuándo y cómo leer el consumo del LLM) reside exclusivamente
en el adapter; el motor expone únicamente un contrato público para que cualquier adapter pueda
informarle el consumo.

**RN-2 — Configurabilidad obligatoria.**
La auditoría de tokens debe poder activarse o desactivarse a nivel de configuración del proyecto
(en `config.yaml`, campo `observability.token_usage`). Un proyecto con auditoría desactivada no
debe almacenar datos de consumo ni intentar capturarlos.

**RN-3 — Registro acumulativo por recálculo total (pisa, no adiciona).**
El consumo registrado debe representar el total acumulado del work item hasta el momento del
registro. El mecanismo es **acumulativo por recálculo**: en cada registro se recalcula el total
de la sesión completa disponible y se **sobrescribe** (pisa) el valor previo; **no** se suma
únicamente el delta del último turno. Esta decisión es deliberada: si un turno no se registró por
cualquier motivo, el siguiente registro recupera el total correcto de toda la sesión y no deja una
brecha silenciosa. Como consecuencia directa, la eventual falta de registro de un turno aislado no
constituye una pérdida de datos permanente.

**RN-4 — Granularidad de registro.**
Se deben registrar por separado: tokens de entrada (input), tokens de salida (output), tokens de
caché (cache) y el total. Adicionalmente, debe quedar registrado el identificador del agente de
código que generó el consumo.

**RN-5 — Sesión exclusiva por work item (responsabilidad del usuario).**
La auditoría toma el consumo de **toda la sesión** —agente principal, orquestador y subagentes—;
todo lo que ocurre fuera de la sesión no se contabiliza. Para que la atribución sea correcta, cada
sesión debe estar dedicada a un único work item. Garantizarlo es **responsabilidad del usuario** y
se sostiene sobre el supuesto operativo de que **cada sesión y cada work item viven en una rama/
worktree distinta** (aislamiento ya provisto por el flujo SDD). Esta condición de uso debe quedar
documentada.

**RN-6 — Recuperabilidad ante fallas del mecanismo de captura.**
Cuando el mecanismo automático de captura falle (error de hook, validación del motor, etc.), el
error debe quedar registrado de forma diagnosticable. Además, el comando público del motor permite
registrar consumo manualmente como mecanismo de respaldo.

**RN-7 — Identificación del work item activo vía `.active-work-item`.**
El adapter identifica a qué work item corresponde la sesión activa leyendo un archivo
`.active-work-item` dentro del worktree. Como cada sesión/work item vive en su propia rama/worktree
(RN-5), ese archivo resuelve sin ambigüedad el destino del registro, incluso con varias sesiones
abiertas en paralelo. Sin esta identificación, el registro no puede asociarse al work item correcto.

---

## Criterios de aceptación

Los siguientes criterios definen cuándo esta feature se considera satisfecha desde una perspectiva
de producto. Los criterios técnicos detallados (condiciones exactas, valores de borde, errores
esperados) corresponden a la fase de especificación.

**CA-1.** Dado un work item con auditoría activa, al finalizar trabajo en cualquiera de sus fases,
el campo de consumo del work item refleja el total acumulado de tokens (input, output, cache,
total) y el agente de código utilizado.

**CA-2.** Dado un work item con auditoría desactivada, el campo de consumo permanece en `null` y
no se registra ningún dato.

**CA-3.** Un operador del proyecto puede consultar el consumo de tokens de cualquier work item
activo o archivado sin necesidad de leer directamente los manifests YAML.

**CA-4.** Es posible registrar consumo de tokens manualmente mediante un comando del motor,
sin depender del mecanismo automático del adapter.

**CA-5.** Cuando el mecanismo de captura falla, el error queda registrado en un log diagnosticable,
y el work item no queda en un estado inconsistente.

**CA-6.** Activar o desactivar la auditoría en `config.yaml` cambia el comportamiento del sistema
para los work items creados a partir de ese momento, sin requerir cambios en el código del motor.

---

## Riesgos y preguntas abiertas

### Riesgos y limitaciones de alto nivel

**R-1 — Sesión atada a un único work item (condición de uso aceptada).**
La atribución correcta requiere que una sesión opere sobre un único work item. Esto se **acepta
formalmente como condición de uso** y es responsabilidad del usuario, apoyada en el aislamiento
por rama/worktree que ya provee el flujo SDD (RN-5). No es una limitación abierta sino una premisa
de diseño; debe quedar documentada como tal.

**R-2 — Falta de registro de un turno aislado (mitigado por diseño).**
El registro acumulativo por recálculo total (RN-3) **elimina** este riesgo: cada registro recalcula
el total de toda la sesión y sobrescribe el valor previo, por lo que un turno no registrado se
recupera en el registro siguiente. No hay pérdida permanente de datos por este motivo.

**R-3 — Múltiples sesiones abiertas simultáneamente (resuelto por diseño).**
Bajo la premisa de una sesión/work item por rama/worktree (RN-5), cada sesión resuelve su destino
leyendo el `.active-work-item` de su propio worktree (RN-7), sin ambigüedad entre sesiones
paralelas. Riesgo residual bajo, acotado a que el usuario respete la condición de uso R-1.

### Preguntas abiertas

**P-1.** ¿Cómo almacena Claude Code las transcripciones de sesión y el consumo de tokens?
¿Es posible consultarlo de forma fiable desde un hook post-turno? Esta pregunta debe ser
investigada y respondida con evidencia empírica en la fase de plan, antes de diseñar cualquier
mecanismo de captura. (Directiva explícita del orquestador: `note.orchestration-directives`.)

**P-2.** ¿Un "turno del agente" comprende toda la ejecución desde el mensaje del usuario hasta
la respuesta completa, incluyendo llamadas a herramientas intermedias? ¿Qué ocurre si en el medio
del turno el usuario debe tomar una decisión interactiva (p. ej., aprobar un permiso)? La
respuesta afecta la granularidad y completitud del registro. (A investigar en la fase de plan;
dado el registro acumulativo por recálculo total (RN-3), su impacto sobre el total final es acotado.)

_Decisiones cerradas por el usuario (previamente P-3 y P-4):_ la eventual falta de registro de un
turno queda **mitigada por diseño** vía RN-3 (registro acumulativo por recálculo); y la exclusividad
sesión↔work item se **acepta como condición de uso** documentada, responsabilidad del usuario (RN-5),
sin necesidad de un mecanismo de detección/alerta en v1.

### Supuestos

**S-1.** El adapter de Claude Code soporta hooks ejecutables al finalizar cada turno del LLM,
tanto para el agente principal como para el orquestador. (Supuesto; debe confirmarse en la
fase de plan mediante investigación técnica.)

**S-2.** La estructura `observability.token_usage` existente en el manifest y en el schema
(`not_reported` | `partial` | `recorded`, con contadores de input/output/cache) es compatible
con los requisitos funcionales de esta feature y puede extenderse o utilizarse sin una ruptura
de schema mayor. (Supuesto basado en la documentación de arquitectura; a verificar en la fase
de especificación.)

**S-3.** El campo de configuración `observability.token_usage` en `config.yaml` (actualmente
modelado como `optional`) puede evolucionar a un control binario on/off sin introducir
incompatibilidades con proyectos existentes. (Supuesto; a validar en especificación.)

