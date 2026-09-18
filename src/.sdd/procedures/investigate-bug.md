# Investigar bug

## Objetivo

Evaluar la causa probable o raíz de un issue usando evidencia de código, ejecución, logs y documentación disponible.

La investigación debe reducir la incertidumbre de forma reproducible. No consiste únicamente en buscar una línea que "parezca" sospechosa ni en proponer un fix antes de distinguir síntomas, hechos e hipótesis.

## Precondiciones

- El issue está documentado y el workflow habilitó exploración o debugging.
- Las herramientas usadas respetan los permisos del proyecto.

## Método de investigación

### 1. Definir el fallo observable

Antes de inspeccionar o modificar el código:

- leer `artifacts/issue.md`, la especificación vigente y el contexto relevante;
- resumir el síntoma, el resultado esperado, el resultado real, el impacto y el entorno;
- identificar si el fallo es reproducible y cuáles son las condiciones mínimas para observarlo;
- separar datos aportados por quien reportó el issue de inferencias todavía no verificadas.

Si faltan datos esenciales, intentar una reproducción controlada o registrar el bloqueo y las preguntas concretas que deben resolverse. No completar los pasos faltantes inventando comportamiento.

### 2. Formular hipótesis y elegir experimentos

Construir una lista corta de hipótesis ordenadas por evidencia e impacto. Para cada una, registrar:

| Hipótesis | Evidencia a favor | Evidencia en contra | Experimento discriminante | Resultado |
| --- | --- | --- | --- | --- |
| Qué podría explicar el síntoma | Hechos observados | Hechos incompatibles | Prueba que separa esta hipótesis de otra | Pendiente/confirmada/descartada |

Priorizar experimentos baratos, reversibles y que diferencien hipótesis. Un experimento puede ser leer una ruta de código, ejecutar un test, variar una entrada, inspeccionar una request, reproducir con otra configuración o comparar una versión conocida.

No cambiar código de producción para "probar" una hipótesis durante esta fase. Si hace falta un probe temporal, aislarlo, documentarlo y eliminarlo antes de concluir; cualquier fix permanente pertenece al plan y a la fase de implementación.

### 3. Investigar por capas

Avanzar desde la capa más cercana al síntoma hacia sus dependencias. Saltar una capa sólo cuando no sea aplicable.

1. **Configuración y entorno:** versiones, flags, variables de entorno no secretas, datos de entrada, permisos, feature flags, dependencias y diferencias entre el entorno que funciona y el que falla.
2. **Código estático:** punto de entrada, flujo de datos, validaciones, manejo de errores, concurrencia, serialización, límites y cambios recientes. Seguir la evidencia hasta el primer punto donde el estado observado deja de coincidir con el esperado.
3. **Ejecución y backend:** reproducir con el comando o request exacto; revisar stdout/stderr, logs estructurados, stack traces, status codes, payloads sanitizados, consultas, tiempos y efectos persistidos. Comparar una ejecución correcta con una fallida cuando sea posible.
4. **API o integración:** inspeccionar contrato, headers relevantes no sensibles, request, response, retries, timeouts, autenticación, colas y servicios externos. Determinar si el fallo se origina localmente o es una condición devuelta por otra dependencia.
5. **UI y navegador:** cuando el issue tenga interfaz, levantar la aplicación en el entorno indicado y reproducir el flujo como usuario. Usar las herramientas disponibles para inspeccionar consola, errores uncaught, warnings relevantes, red, status codes, payloads, tiempos, almacenamiento, cookies no sensibles, estado del DOM, eventos y layout. Verificar también estados intermedios, navegación, loading, errores y persistencia; no limitarse a la pantalla final.
6. **DevTools y automatización:** si el adaptador dispone de una capacidad de navegador/DevTools mediante MCP, usarla para abrir la URL, fijar viewport y estado inicial, capturar screenshots, inspeccionar consola/red/performance y ejecutar el flujo reproducible. Si esa capacidad no está instalada o autorizada, marcar la verificación UI como `Bloqueada` o `No ejecutada`, conservar lo que sí pudo comprobarse y declarar qué herramienta falta. No afirmar que se inspeccionó la UI sólo porque el código fue leído.

La investigación puede usar herramientas de terminal, test runners, clientes HTTP, navegador, DevTools u otros MCPs según el registro de capabilities y los permisos del proyecto. La procedure es portable: no debe depender de un proveedor concreto ni asumir que una capacidad está disponible.

### 4. Confirmar la causa

Una causa se considera confirmada sólo cuando:

- explica el síntoma y las condiciones de reproducción;
- está respaldada por una observación directa o por una cadena de evidencia verificable;
- un experimento la distingue de las hipótesis alternativas razonables;
- permite indicar qué componente, estado o interacción debe cambiarse, sin prescribir todavía una implementación detallada.

Si sólo se conoce una causa probable, nombrarla como tal. Si las hipótesis no pueden distinguirse con la evidencia disponible, declarar el límite y los siguientes experimentos necesarios.

## Evidencia y seguridad

- Guardar bajo `evidence/` los insumos crudos que permitan repetir la investigación: comandos, logs, requests/responses sanitizados, dumps relevantes, trazas, screenshots, videos, exports de DevTools o reportes de herramientas.
- En `artifacts/exploration.md`, enlazar cada afirmación importante con su evidencia y registrar fecha, entorno, versión/commit, herramienta y resultado.
- Sanitizar secretos, tokens, cookies, headers sensibles, datos personales y payloads confidenciales antes de persistir cualquier salida.
- No ejecutar acciones destructivas, mutaciones contra sistemas reales ni comandos fuera del alcance autorizado. Para integraciones externas, usar mocks, fixtures o entornos de prueba cuando estén disponibles.
- Mantener intacto el estado del repositorio salvo que el workflow habilite explícitamente una modificación. La exploración no aprueba ni aplica cambios.

## Resultado

Crear `artifacts/exploration.md` con, como mínimo:

- resumen del issue y resultado esperado versus observado;
- entorno y pasos de reproducción, incluyendo si no fue posible reproducirlo;
- hechos observados y fuentes de evidencia;
- hipótesis consideradas y experimentos ejecutados;
- análisis por las capas aplicables, incluida UI/DevTools cuando corresponda;
- causa raíz confirmada, causa probable o límite de la investigación;
- próximos pasos o preguntas abiertas, especialmente capacidades MCP faltantes;
- enlaces a los archivos relevantes bajo `evidence/`.

La exploración no genera un plan de implementación, no aplica un fix y no marca una fase como completada: esas acciones las gobiernan el workflow y el motor.
