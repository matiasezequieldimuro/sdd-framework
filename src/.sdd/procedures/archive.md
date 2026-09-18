# Archivar work item

## Objetivo

Cerrar un work item ya revisado, consolidando el conocimiento durable y preservando su expediente.

## Precondiciones

- La revisión humana de código está aprobada.
- Las acciones externas del archive fueron autorizadas explícitamente.

## Consolidación de documentación

- Si el work item es una feature, crear o completar la especificación vigente en `.sdd/specs/` a partir de `artifacts/specification.md`.
- Si el work item es un cambio, actualizar la especificación vigente en `.sdd/specs/` con los deltas implementados. No reemplazar el baseline completo por el documento del work item.
- Verificar que la documentación spec refleje el comportamiento implementado y que el changelog/baseline quede actualizado.
- Crear `artifacts/archive.md` con el resultado, las referencias al baseline y al changelog, y las acciones externas realizadas.

## Flujo de cierre

El cierre ejecuta las siguientes acciones, en este orden:

1. Commit: aplicar la skill auxiliar `git-commit`.
2. Push: publicar la rama remota correspondiente.
3. Pull request: aplicar la skill auxiliar `pull-request` y abrir la PR.

Ninguna acción se omite salvo indicación explícita del usuario. Si el usuario indica omitir una acción, registrar la decisión y el motivo en `artifacts/archive.md`; no inferir autorizaciones para las acciones restantes.

