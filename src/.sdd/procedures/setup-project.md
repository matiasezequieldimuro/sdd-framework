# Preparar proyecto para SDD

## Objetivo

Dejar un proyecto listo para usar el contrato SDD y los adaptadores de agentes elegidos, sin crear work items ni asumir configuración privada.

## Precondiciones

- Se conoce la raíz del proyecto objetivo.
- `sdd-cli` está disponible.
- El usuario indicó qué adaptadores desea instalar.

## Resultado

Ejecutar `sdd-cli init` cuando `.sdd/` no exista y luego instalar únicamente los adaptadores solicitados. Validar el resultado con `sdd-cli validate`. No sobrescribir instalaciones existentes ni crear secretos.
