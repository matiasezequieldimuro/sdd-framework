# Verificar implementación

## Objetivo

Obtener evidencia real, reproducible y proporcional al riesgo de que la implementación cumple la especificación y el plan aprobado.

La verificación no consiste sólo en ejecutar la suite existente. Debe comprobar que:

- los cambios implementados corresponden a la especificación aprobada y a los criterios de aceptación;
- se ejecutaron las tareas y verificaciones previstas en el plan, o se documentó por qué una desviación razonada es mejor que el plan original;
- el comportamiento funciona en el nivel aplicable: código, API, proceso completo y, cuando exista UI, presentación e interacción.

## Fuentes que se deben contrastar

Leer, como mínimo:

- la especificación aprobada, especialmente los criterios de aceptación;
- el plan aprobado y su estrategia de pruebas;
- `artifacts/implementation-report.md`;
- el diff y los archivos modificados;
- la configuración y documentación necesaria para levantar el entorno.

No tratar el reporte de implementación, el chat ni la existencia de tests como prueba suficiente. La evidencia debe provenir de una ejecución o inspección comprobable.

## Trazabilidad y desviaciones

Antes de ejecutar las pruebas, construir una matriz que relacione cada criterio de aceptación con:

`criterio -> tarea del plan -> cambio implementado -> prueba o inspección -> evidencia -> resultado`

La matriz debe identificar criterios cubiertos, parcialmente cubiertos y no cubiertos. Para cada desviación del plan:

- explicar qué cambió y por qué;
- justificar por qué la alternativa es técnicamente más adecuada o reduce un riesgo;
- indicar el impacto sobre alcance, comportamiento, compatibilidad y pruebas;
- registrar si requiere decisión o aprobación humana.

Una desviación inteligente no se considera automáticamente un incumplimiento, pero tampoco se acepta sin justificación y evidencia.

## Estrategia de verificación

Seleccionar las comprobaciones aplicables al cambio y declarar explícitamente las que no aplican:

1. **Validación estructural:** ejecutar el formateo, lint, compilación, typecheck y validaciones del proyecto que correspondan.
2. **Pruebas automatizadas:** ejecutar primero las pruebas unitarias y luego las de integración o contrato relacionadas con el alcance. Ejecutar la suite de regresión disponible cuando el riesgo o el plan lo requieran.
3. **Smoke test:** levantar el sistema en un entorno reproducible y comprobar el camino mínimo de funcionamiento, incluyendo arranque, configuración esencial y una operación representativa.
4. **API testing:** cuando se modifique o consuma una API, probar los endpoints o contratos afectados con entradas válidas, respuestas esperadas y, cuando sea relevante, errores, autenticación, validación y límites. Conservar requests, responses o reportes sanitizados bajo `evidence/`.
5. **Verificación visual:** cuando exista UI, levantarla en el entorno indicado por el proyecto y comprobar que la pantalla modificada luce como lo esperado según la especificación. Registrar viewport, navegador, estado utilizado y capturas antes/después cuando aporten valor.
6. **Flujo UI/RPA:** cuando exista interacción de usuario, ejecutar el flujo principal mediante clicks, escritura, selección, navegación y acciones equivalentes a un usuario real. Comprobar estados intermedios, mensajes, navegación, persistencia y resultado final; conservar screenshots, video o logs reproducibles cuando estén disponibles.

No inventar pruebas visuales o de API para componentes que no las tienen. Declarar `No aplica` junto con la razón y la cobertura alternativa.

## Ejecución y evidencia

- Registrar para cada comprobación el comando o procedimiento exacto, entorno, fecha, resultado y causa de cualquier fallo.
- No ocultar fallos por ejecutar comandos parciales ni reemplazar una prueba fallida por una inspección manual sin explicarlo.
- Sanitizar secretos, tokens, datos personales y credenciales antes de guardar evidencia.
- Guardar bajo `evidence/` los resultados crudos que permitan reproducir o auditar la conclusión: logs, JSON/JUnit, capturas, videos, reportes de cobertura y resultados de smoke tests.
- Si el entorno no puede levantarse o una prueba no puede ejecutarse, marcarla como `Bloqueada`, explicar el bloqueo y no afirmar que el criterio está validado.

## Criterios de conclusión

La implementación sólo puede declararse verificada cuando:

- todos los criterios aplicables tienen resultado `Pass`, o una desviación justificada y aceptada cubre el comportamiento esperado;
- las pruebas fallidas, bloqueadas o parciales están visibles en el reporte;
- la evidencia permite repetir las comprobaciones principales;
- no quedan discrepancias sin explicar entre especificación, plan, implementación y comportamiento observado.

Si no se cumple alguna condición, el resultado debe ser `Failed`, `Blocked` o `Partially verified`, nunca un éxito implícito.

## Precondiciones

- La implementación está completada según el motor.
- Se dispone de los criterios de aceptación y de un entorno de verificación aplicable.

## Resultado

Crear `artifacts/verification-report.md` con la matriz de trazabilidad, desviaciones, entorno, comprobaciones ejecutadas, resultados y enlaces a la evidencia. Conservar resultados crudos reproducibles bajo `evidence/` cuando aporten valor. Declarar fallos, bloqueos, cobertura incompleta y pruebas no aplicables; no asumir éxito ni marcar una fase como completada: esa transición la realiza el motor.
