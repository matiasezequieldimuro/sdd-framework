# SDD Workflow — Motor de Spec-Driven Development

**SDD (Spec-Driven Development)** es un framework para trabajar con agentes de IA de
forma gobernada: en lugar de confiar en que un LLM "recuerde" en qué punto del proceso
está, el proceso se modela como fases explícitas con aprobaciones humanas, artefactos y
trazabilidad, y una CLI determinista se encarga de hacerlo cumplir.

> Estado: **BETA** (`v0.1.0-beta`).

## Filosofía

SDD separa dos responsabilidades que normalmente se mezclan:

```text
Agente o persona: realiza el trabajo cognitivo (redacta, investiga, programa).
CLI (sdd-cli):    gobierna el proceso (valida reglas, cambia estado, registra evidencia).
```

El agente hace el trabajo; la CLI valida transiciones, exige los gates humanos, prepara
los artefactos de cada fase y deja un historial auditable. Así el estado del proceso no
depende del chat ni de la memoria del modelo.

## El contrato

El corazón del framework es un **contrato declarativo** (`.sdd/`): define los workflows,
sus fases, dependencias, gates, artefactos y schemas como **datos**, no como código. Los
workflows se describen en YAML y se validan como un DAG (grafo sin ciclos). Esto hace el
proceso portable entre agentes y personalizable por proyecto.

## La CLI determinista

`sdd-cli` es el **motor determinista** que interpreta ese contrato. Es un binario nativo
único escrito en Go —sin Node.js, Python ni ningún runtime— con el contrato embebido.
No redacta documentos ni programa: crea work items, conoce el estado de cada fase, valida
transiciones, impide saltos no permitidos, exige aprobaciones humanas, prepara artefactos
y persiste cada cambio de forma atómica y trazable.

## Instalación rápida

**macOS / Linux (Ubuntu/WSL):**

```bash
curl -fsSL https://raw.githubusercontent.com/matiasezequieldimuro/sdd-framework/main/scripts/install.sh | sh
```

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/matiasezequieldimuro/sdd-framework/main/scripts/install.ps1 | iex
```

Los binarios se publican en [GitHub Releases](https://github.com/matiasezequieldimuro/sdd-framework/releases).
Guía completa (manual, build desde fuente, troubleshooting) en
[**docs/GUIA_INSTALACION.md**](./docs/GUIA_INSTALACION.md).

## Primeros pasos

```bash
sdd-cli version                       # verificar instalación
sdd-cli init                          # crear la estructura .sdd/ en tu proyecto
sdd-cli adapters install claude-code  # (opcional) adapter de Claude Code
sdd-cli --help                        # ver todos los comandos
```

## Documentación

- [**Guía de instalación**](./docs/GUIA_INSTALACION.md) — instalar, actualizar y desinstalar en cada OS.
- [**El workflow de SDD**](./docs/SDD_WORKFLOW.md) — escenarios, fases, gates, artefactos y fuentes de verdad.
- [**Referencia de la CLI**](./docs/CLI.md) — comandos, flags, ejemplos y contrato de salida.
- [**Changelog**](./CHANGELOG.md) — scope funcional de la BETA.

## Desarrollo

Requiere [Go 1.22+](https://go.dev/dl/).

```bash
cd src/cli
go generate ./...   # sincroniza recursos embebidos (.sdd y adapters)
go build -o sdd-cli .
go test ./...
```

## CI/CD

- **CI** (`.github/workflows/ci.yml`): valida formato, `go vet`, tests y build en cada
  push/PR hacia `main` y `development`.
- **Release** (`.github/workflows/release.yml`): al publicar un tag `v*` compila los
  binarios multiplataforma, genera `SHA256SUMS` y crea el GitHub Release.

```bash
git tag v0.1.0-beta
git push origin v0.1.0-beta
```
