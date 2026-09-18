# Crear plan de implementación

## Objetivo

Crear un plan de implementación detallado a partir del análisis del issue principal. El plan debe determinar si el problema necesita dividirse en fases, por ejemplo, cuando involucra funcionalidades diferentes o múltiples cambios en numerosos archivos de las distintas capas de la aplicación. Si se identifican fases, cada una debe incluir un listado claro y detallado de tareas, con el alcance y el resultado esperado, para que los desarrolladores puedan comprender qué deben hacer sin ambigüedades.

## Precondiciones

- La entrada requerida por el workflow está aceptada o aprobada.
- Se exploró el código y contexto necesario; las incertidumbres quedan documentadas.

## Instrucciones

- Consultar documentación actualizada, priorizando siempre fuentes oficiales de las tecnologías, librerías y servicios involucrados.
- Incluir snippets de código relevantes como referencia cuando ayuden a aclarar la implementación esperada o las decisiones técnicas.
- Explicitar el impacto esperado de la solución en las funcionalidades, módulos, capas y dependencias afectadas.
- Identificar los posibles riesgos, supuestos y consecuencias de la implementación, junto con las medidas de mitigación cuando corresponda.

## Reglas

Siempre seguir principios SOLID, Clean Code y Clean Architecture a la hora de diseñar una solución o proponer cambios en el proyecto. A su vez, implementar patrones de diseño para resolver problemas comunes. Es crítico.

Antes de finalizar la fase de planificación, debes revisar el documento para verificar que cumpla con dichos principios.

## Resultado

Crear `artifacts/plan.md` desde la plantilla. Incluir áreas afectadas, estrategia de pruebas, dependencias, riesgos y rollback cuando corresponda. No modificar código en esta operación.
