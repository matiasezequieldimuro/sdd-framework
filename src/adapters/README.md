# Adapters de agentes

Cada subdirectorio contiene la fuente versionada de un adapter soportado por
`sdd-cli`.

```text
src/adapters/<adapter-id>/
├── adapter.yaml
└── assets propios de la plataforma
```

Los adapters traducen la experiencia de una herramienta hacia el contrato
portable de `.sdd/`. No deben duplicar workflows, estado ni el contenido
completo de los procedures.

Los assets se sincronizan al embed de la CLI mediante:

```bash
cd src/cli
go generate ./...
```

El proyecto destino puede consultar e instalar adapters con:

```bash
sdd-cli adapters list
sdd-cli adapters install <adapter-id>
```
