# Crear commit

## Objetivo

Registrar los cambios del work item en un commit claro, revisable y focalizado.

## Convención

Usar Conventional Commits:

```text
<tipo>(<alcance>): <descripcion breve>
```

Tipos habituales:

- `feat`: nueva funcionalidad.
- `fix`: corrección de un defecto.
- `docs`: documentación.
- `refactor`: cambio interno sin modificar el comportamiento.
- `test`: pruebas.
- `chore`: mantenimiento.

Ejemplos:

```text
feat(checkout): agregar validacion de cupones
fix(cli): rechazar archive antes de la revision de codigo
docs(sdd): documentar el flujo de archive
```

Cuando haga falta agregar contexto, usar la primera línea como resumen y continuar
con un cuerpo opcional separado por una línea en blanco:

```text
feat(checkout): agregar validacion de cupones

Se valida el cupon antes de calcular el total para evitar aplicar
descuentos sobre pedidos que no cumplen las condiciones requeridas.
```

## Reglas

- Mantener cada commit enfocado en un cambio coherente.
- No mezclar refactors, formatting o cambios no relacionados con el objetivo del work item.
- Evitar commits inmensos: separar por unidad funcional o por tipo de cambio cuando eso mejore la revisión.
- El cuerpo del commit es opcional; usarlo sólo para explicar contexto, decisiones o argumentos que no entren en el resumen.
- Revisar el diff y las pruebas relevantes antes de crear el commit.
- Generá los commits en Español.
- No incluyas NUNCA "Co-Authored By" ni cualquier referencia del agente de código utilizado.