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

## Adapter `claude-code`

`sdd-cli adapters install claude-code` publica `CLAUDE.md`, `.mcp.json` y `.claude/`
en la raíz del proyecto (falla sin escribir nada si alguna de esas raíces ya existe):

| Tipo | Contenido |
| --- | --- |
| Agents (`.claude/agents/`) | `sdd-orchestrator`, `sdd-functional-analyst`, `sdd-planner`, `sdd-developer`, `sdd-verifier`, `sdd-debugger`, `sdd-archivist` |
| Skills (`.claude/skills/`) | `sdd-cli`, `status`, `setup-project`, `onboard-project`, `answer-project-question`, `research-topic`, `generate-prd`, `generate-change-request`, `generate-specification`, `document-issue`, `investigate-bug`, `create-plan`, `implement`, `verify`, `verify-ui`, `archive`, `git-commit`, `git-worktree`, `pull-request`, `certificate-weekly-hs` |
| Commands (`.claude/commands/`) | `/sdd:feature`, `/sdd:change-request`, `/sdd:fast-change`, `/sdd:bug-known-cause`, `/sdd:bug-investigation`, `/sdd:status`, `/sdd:next`, `/sdd:approve`, `/sdd:reject`, `/sdd:rework`, `/sdd:cancel`, `/sdd:archive`, `/certificate-weekly-hs` |
| Hooks (`.claude/hooks/`) | `protect-sdd-paths.py`, `protect-secrets.py` |
| Settings | `.claude/settings.json` (permisos, incluidos `allow`/`ask`/`deny` de `playwright-cli`, y hooks) y `.claude/settings.local.json.example` |
| MCP (`.mcp.json`) | `github`, `context7`, `azure-devops` (reemplazar `TU_ORGANIZACION`) |

`certificate-weekly-hs` (skill y comando) redacta tareas de certificación de horas en
Azure DevOps a partir del trabajo reciente del repositorio; se instala en todo proyecto.
