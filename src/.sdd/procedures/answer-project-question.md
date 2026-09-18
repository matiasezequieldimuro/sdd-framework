# Responder consulta del proyecto

## Objetivo

Responder una pregunta puntual, técnica o funcional, usando las fuentes de verdad necesarias y distinguiendo hechos, inferencias e incertidumbres. La consulta no requiere una precondición específica ni inicia por sí misma un workflow.

## Fuentes de verdad

Consultar las fuentes según la naturaleza de la pregunta:

- `manifest.yaml` y los logs de eventos (`events.jsonl`) para el estado actual del work item, las transiciones y las decisiones registradas.
- Specs aprobadas y artefactos de las fases para el comportamiento esperado, los criterios de aceptación y la evidencia documentada.
- `.sdd/context/` para conocimiento durable, convenciones y contexto del proyecto.
- `domain-language.md` para usar el lenguaje común del dominio y asegurar que el usuario y la IA interpreten los conceptos con el mismo significado.
- Historial de Git para la evolución de la implementación y el contexto de cambios.
- Engram, cuando esté disponible, como fuente adicional de conocimiento persistido.
- Codebase y tests para contrastar el comportamiento actualmente implementado.

No usar la conversación, nombres de archivos o prosa aislada para inferir el estado del workflow.

## Criterio de consulta

- Para una pregunta funcional, priorizar specs aprobadas, criterios de aceptación y otros artefactos relevantes; complementar con contexto, eventos y codebase cuando sea necesario.
- Para una pregunta técnica, revisar los artefactos técnicos y los documentos puntuales de los work items asociados; complementar con historial de Git, codebase y tests.
- Usar solamente las fuentes necesarias para responder y señalar qué fuente respalda cada afirmación cuando aporte valor.

## Resultado

Entregar una respuesta directa con referencias concretas cuando aporten valor. Indicar explícitamente incertidumbres, información faltante o conflictos entre fuentes. No crear un work item salvo que la consulta derive en un cambio gobernado ni documentar archivos nuevos si el usuario no lo solicita.
