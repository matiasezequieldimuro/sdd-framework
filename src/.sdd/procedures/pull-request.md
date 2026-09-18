# Crear pull request

## Objetivo

Presentar el cambio para revisión humana con contexto suficiente para decidir y verificarlo.

La PR debe ser concisa, simple de leer y fácil de entender. El Team Lead debe
captar rápidamente qué se hizo, por qué y cómo se validó, sin tener que leer una
narrativa extensa o detalles que no ayuden a revisar el cambio.

## Estructura

La PR debe incluir, en este orden:

1. **Resumen:** qué cambia y por qué.
2. **Alcance:** qué incluye y qué queda fuera.
3. **Implementación:** decisiones relevantes y trade-offs.
4. **Validación:** comandos ejecutados y resultado.
5. **Documentación SDD:** enlaces a la spec, artifacts, baseline y archive relacionados.
6. **Riesgos o pendientes:** limitaciones conocidas, migraciones o trabajo posterior.

## Tono

- Claro, directo y técnico, sin lenguaje promocional.
- Redactar el título y el cuerpo de la PR en español.
- Conciso y escaneable: priorizar información de alto valor y evitar una PR innecesariamente larga.
- Describir hechos verificables y decisiones, no narrar el proceso del agente.
- Ser explícito sobre incertidumbres, riesgos y validaciones que no pudieron ejecutarse.
- Mantener el título breve y orientado al resultado, usando la misma convención de commits cuando sea útil.