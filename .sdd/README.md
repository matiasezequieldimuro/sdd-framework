# Núcleo portable SDD

Esta carpeta es el contrato portable del motor SDD (`schema_version` 1.0). `sdd-cli init`
la instala en la raíz del proyecto como `.sdd/`. `sdd-cli` es la autoridad del estado: no
edites a mano manifests, eventos, locks ni work items archivados.

No incluye adapters, agentes, skills, comandos ni la CLI. Los adapters (por ejemplo
`claude-code`, que se instala con `sdd-cli adapters install`) consumen este contrato sin
duplicar workflows ni estado.

## Contenido

| Ruta | Propósito |
| --- | --- |
| `config.yaml` | Workflow por defecto, idioma de artefactos y modo de interacción |
| `workflows/` | Fases, dependencias, artefactos, entry points y gates de cada workflow |
| `schemas/` | JSON Schemas con los que el motor valida manifests, eventos, workflows y artefactos |
| `templates/` | Plantillas de los artefactos de cada fase |
| `registry/capabilities.yaml` | Índice portable de capabilities y su procedure |
| `procedures/` | Instrucciones de cada capability |
| `context/` | Conocimiento durable del proyecto (`project.md`, `architecture/`, `domain-language.md`) |
| `specs/` | Specs vigentes (baseline) por funcionalidad |
| `research/` | Investigaciones fechadas (evidencia exploratoria, no requisitos aprobados) |
| `work-items/` | `active/`, `archive/` y `cancelled/` (se crea con la primera cancelación). Lo gestiona solo `sdd-cli`; `.locks/` y `.transactions/` son estado de runtime ignorado por Git |

## Capabilities y procedures

`registry/capabilities.yaml` es el índice portable de habilidades disponibles.
Cuando una capability requiere instrucciones, referencia un archivo de
`procedures/`. Los adapters específicos pueden exponer skills propias de la
plataforma, pero deben actuar como wrappers finos de este registry y sus
procedures.
