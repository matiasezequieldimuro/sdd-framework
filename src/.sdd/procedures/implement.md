# Implementar cambio aprobado

## Objetivo

Aplicar fielmente los cambios detallados en el plan aprobado y registrar lo realizado.

## Reglas de implementación

- Analizar primero el plan aprobado para comprender el issue, el alcance, las tareas propuestas, las restricciones y los criterios de aceptación.
- Contrastar el plan con la especificación, el código existente y las pruebas relevantes antes de codificar. Identificar cualquier incongruencia, ambigüedad o error en el plan o en la especificación.
- No comenzar a codificar si existe una contradicción que impida determinar el comportamiento esperado. Registrar el bloqueo y solicitar la resolución correspondiente.
- Consultar fuentes y documentación oficial de las tecnologías, librerías y APIs involucradas antes de implementar decisiones técnicas relevantes.
- Aplicar siempre los principios SOLID y las prácticas de Clean Code y Clean Architecture, respetando las responsabilidades y los límites de las capas existentes.
- Verificar que cada cambio propuesto sea consistente con el layering, las dependencias, las responsabilidades y las convenciones arquitectónicas del codebase existente. No introducir acoplamiento entre capas, violaciones de límites o responsabilidades fuera de lugar; si el plan aprobado requiere una excepción, documentarla y obtener su aprobación explícita.
- Implementar siempre que sea posible -y tenga sentido- patrones de diseño. Cuando se debe resolver un problema complejo, validar si no se puede resolver con un patrón conocido.
- Implementar únicamente lo necesario para cumplir el plan aprobado. No introducir refactors, dependencias o cambios de alcance no autorizados.
- Validar los cambios con las pruebas, verificaciones y criterios de aceptación definidos en el plan y registrar sus resultados.
- Antes de finalizar una fase o la implementación completa, ejecutar los tests, la compilación y las verificaciones disponibles para confirmar que los cambios funcionan y no rompen el comportamiento existente. Resolver los fallos o dejar documentado el bloqueo antes de continuar.

## Reglas del reporte de implementación

- Organizar los cambios por fase cuando corresponda y explicar brevemente qué se hizo, cómo se resolvió y dónde se implementó: archivos, módulos o componentes relevantes.
- Incluir snippets de código cortos cuando ayuden a entender la solución sin que el lector tenga que inferirla o revisar todo el código.
- Documentar de forma breve las decisiones técnicas, trade-offs y su impacto cuando sean relevantes o críticos.
- Registrar las desviaciones aprobadas del plan e indicar "Ninguna" cuando no existan.
- Documentar las pruebas añadidas o actualizadas, cómo se ejecutaron y sus resultados.
- Documentar en una sección separada las pruebas realizadas antes del cierre, incluyendo tests, compilación, verificaciones ejecutadas y sus resultados.
- Incluir un diagrama Mermaid únicamente cuando facilite entender el flujo o las interacciones; mantener el reporte simple y conciso.

## Reglas de documentación del código

- Agregar un docstring conciso o un comentario de documentación idiomático del lenguaje a cada clase y función implementada.
- Escribir los docstrings y comentarios de documentación en inglés.
- Documentar brevemente qué es, qué hace, para qué sirve y por qué es importante cuando ese contexto no sea evidente a partir del código.
- Mantener la documentación corta, precisa y enfocada en el comportamiento, la intención, los contratos y las restricciones relevantes. No repetir la implementación.
- Documentar los parámetros, valores de retorno, errores, excepciones, efectos secundarios e invariantes importantes utilizando las convenciones del lenguaje o de las herramientas del proyecto.
- Usar anotaciones o tags de documentación como `@param`, `@returns`, `@throws`, `@raises`, `@deprecated` y equivalentes cuando estén soportados y aporten información útil.
- Mantener la documentación sincronizada con el código y actualizarla cuando cambie el contrato documentado.

## Precondiciones

- El motor habilitó la fase `implementation` tras aprobar el plan.
- El adaptador obtuvo los permisos de escritura necesarios.

## Resultado

Modificar únicamente el alcance autorizado. Crear `artifacts/implementation-report.md` con el análisis previo del plan, las fuentes oficiales consultadas, los cambios, los archivos relevantes, las pruebas ejecutadas, sus resultados y cualquier desviación aprobada. No marcar una fase como completada: esa transición la realiza el motor.
