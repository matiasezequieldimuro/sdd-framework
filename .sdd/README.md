# Núcleo portable SDD

Esta carpeta contiene el contrato portable del motor SDD. Al instalar el template en un proyecto objetivo, su contenido debe ubicarse en la raíz del proyecto como `.sdd/`.

No incluye adaptadores, agentes, skills, comandos ni CLI. Esos componentes consumirán este contrato sin duplicar workflows ni estado.

`registry/capabilities.yaml` es el índice portable de habilidades disponibles.
Cuando una capability requiere instrucciones, referencia un archivo de
`procedures/`. Los adapters específicos pueden exponer skills propias de la
plataforma, pero deben actuar como wrappers finos de este registry y sus
procedures.
