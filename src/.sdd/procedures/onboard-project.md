# Documentar proyecto existente

## Objetivo

Construir contexto estable y verificable de un repositorio existente para reducir exploraciones repetidas.

## Precondiciones

- El contrato SDD está inicializado.
- Se dispone del código del proyecto o de la mayor parte accesible de este.
- Las fuentes faltantes, contradictorias o todavía desconocidas pueden declararse explícitamente.

## Resultado de nuevos artefactos

Actualizar los siguientes documentos bajo `.sdd/context/`:

- `project.md`: propósito, alcance, límites, comandos y referencias.
- `architecture/software-architecture.md`: arquitectura general, sistemas conectados, autenticación, plataforma y despliegue.
- `architecture/project-architecture.md`: estructura concreta del repositorio, capas, convenciones y patrones implementados.
- `architecture/data-modeling.md`: entidades, relaciones, persistencia, validaciones y migraciones conocidas.
- `architecture/data-and-process-flow.md`: flujos de datos, procesos principales y diagramas relevantes.

Priorizar el codebase y las fuentes verificables. No inventar contexto ausente: marcar como pendiente o desconocido lo que no pueda confirmarse. No completar `domain-language.md`; ese documento se construye progresivamente durante el trabajo con el proyecto.

## Proyectos ya avanzados: especificar funcionalidades existentes

Si el proyecto ya tiene funcionalidad implementada de forma significativa, además de los documentos de contexto anteriores, tras la exploración generar **specs de las funcionalidades existentes**.

Motivo: SDD apunta cada cambio de una feature a su especificación. Al onboardear un proyecto avanzado, si luego hay que ajustar una funcionalidad ya implementada (por ejemplo con `change-request`), no existiría ninguna spec a la cual apuntar. Estas specs cubren ese vacío reconstruyendo la especificación a partir del código.

Cómo hacerlo:

- Identificar las funcionalidades o casos de uso cohesivos del proyecto y generar **una spec por cada uno**, no una única spec monolítica. Ejemplo: un servicio CAP con casos de uso para analizar facturas mediante IA y para registrar asientos contables en SAP genera dos specs separadas.
- Reconstruir cada spec desde el codebase como fuente de verdad prioritaria: propósito, comportamiento observable, entradas y salidas, reglas y dependencias. Marcar como pendiente o desconocido lo que no pueda confirmarse.
- Guardarlas bajo `.sdd/context/specs/`, una por funcionalidad, con nombres descriptivos.

Criterio de granularidad: si una funcionalidad puede evolucionar o ajustarse de forma independiente, merece su propia spec.
