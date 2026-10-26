---
name: certificate-weekly-hs
description: Usar para redactar tareas de certificación de horas en Azure DevOps a partir del trabajo reciente del repositorio.
---

## Relevamiento

1. Analizar el historial de git del período (`git log --since`, commits, merges y
   diffs relevantes), incluyendo ramas de worktrees si existen.
2. Revisar el código modificado para entender qué funcionalidad cambió.
3. Leer los work items de `.sdd/` (artifacts, archive) y las specs en `.sdd/specs/`
   relacionados, para usar su lenguaje funcional.
4. Agrupar el trabajo en exactamente N tareas coherentes, cada una centrada en una
   funcionalidad o caso de uso agregado/modificado, sin repetir contenido.

## Reglas de redacción

- Markdown, priorizando headings y bullets.
- Mínimo 350 palabras por tarea.
- Tono intermedio entre funcional y técnico; lenguaje sencillo y claro.
- Tiempo futuro ("se implementará", "se incorporará"); nunca pasado ("se implementó").
- Aludir a funcionalidades y casos de uso, no a archivos, hashes ni detalles internos.
- No inventar trabajo sin respaldo en git, código o specs.
- Entregar cada tarea en un bloque separado listo para copiar; no escribir en
  Azure DevOps salvo pedido explícito.

## Formato

```markdown
# <Título breve orientado a la funcionalidad>

## Objetivo
- <Qué se implementará y para qué>

## Alcance
- <Funcionalidad o caso de uso que se agregará/modificará>

## Actividades
- Se implementará ...
- Se ajustará ...

## Resultado esperado
- <Beneficio funcional y técnico>
```
