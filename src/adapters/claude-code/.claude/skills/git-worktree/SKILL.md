---
name: git-worktree
description: Usar para aprovisionar, configurar entorno y desaprovisionar worktrees aislados para desarrollo paralelo.
---

# Git Worktree

Utilizar esta skill para gestionar el ciclo de vida de **worktrees aislados** para features, change requests y bugs. Permite el trabajo paralelo de múltiples orquestadores en terminales independientes sin colisionar en ramas estables (`develop`, `main`).

> **Regla de Autoridad:** Solo el `sdd-orchestrator` tiene autorización para crear o remover worktrees. Los subagentes especialistas (`sdd-developer`, `sdd-verifier`, etc.) nunca administran worktrees.

---

## 1. Reglas de Aislamiento

1. **Nunca trabajar en ramas estables:** Todo cambio de código, prueba o experimentación debe realizarse dentro de un worktree aislado en su propia rama.
2. **No anidar worktrees:** Si la sesión actual ya se encuentra dentro de una rama de feature/worktree (`git branch --show-current` no es una rama estable), operar directamente sobre el directorio actual.
3. **Ubicación externa:** Los worktrees se crean en un directorio hermano fuera del repositorio raíz:
   ```text
   ../<repo-name>-worktrees/<work-item-id>
   ```
   Esto previene conflictos de indexación, linters recursivos y observadores de build tools.
4. **Convención de ramas:**
   - Features: `feature/<work-item-id>`
   - Bugs: `bug/<work-item-id>`
   - Change Requests: `cr/<work-item-id>`

---

## 2. Operaciones

### A. Crear Worktree (`create`)

Antes de crear el worktree:
1. Verificar la rama actual:
   ```bash
   git branch --show-current
   ```
   Confirmar si coincide con la rama estable (`Development Branch` o `Main Branch` definida en `CLAUDE.md`).
2. Inspeccionar los worktrees existentes antes de crear uno:
   ```bash
   git worktree list --porcelain
   ```
   Si alguno coincide claramente con el work item por ID, rama o estado SDD,
   reutilizarlo. Si hay más de un candidato, detenerse y pedir aclaración. Sólo
   continuar con la creación cuando no exista una coincidencia.
3. Obtener el último estado de la rama base:
   ```bash
   git fetch origin <base-branch>
   ```
4. Determinar el nombre de la nueva rama y la ruta destino como **ruta absoluta**
   (nunca relativa; una ruta relativa se resuelve contra el cwd del proceso que
   invoca la CLI, que puede no ser el repositorio raíz, y `sdd-cli` termina sin
   encontrar `.claude`/`.sdd`):
   ```bash
   REPO_NAME=$(basename $(git rev-parse --show-toplevel))
   WORKTREE_PATH="$(cd "$(git rev-parse --show-toplevel)/.." && pwd)/${REPO_NAME}-worktrees/<work-item-id>"
   BRANCH_NAME="feature/<work-item-id>" # o bug/... o cr/...
   ```
5. Crear el worktree y la nueva rama apuntando a la rama base:
   ```bash
   git worktree add -b "$BRANCH_NAME" "$WORKTREE_PATH" origin/<base-branch>
   ```

### B. Bootstrap del Entorno (`bootstrap`)

Un worktree virgen solo contiene archivos rastreados por Git. Es mandatorio inicializar dependencias y configuraciones locales antes de comenzar a trabajar:

1. **Copiar variables de entorno locales (si existen en la raíz y son necesarias para desarrollo):**
   ```bash
   # Solo si existen en el repositorio raíz
   [ -f .env ] && cp .env "$WORKTREE_PATH/.env"
   [ -f .env.local ] && cp .env.local "$WORKTREE_PATH/.env.local"
   ```
2. **Instalar dependencias según el stack del proyecto:**
   - **Node.js:** Si existe `package.json`:
     - Si hay `pnpm-lock.yaml`: `(cd "$WORKTREE_PATH" && pnpm install)`
     - Si hay `package-lock.json`: `(cd "$WORKTREE_PATH" && npm install)`
     - Si hay `yarn.lock`: `(cd "$WORKTREE_PATH" && yarn install)`
     - Si hay `bun.lockb` o `bun.lock`: `(cd "$WORKTREE_PATH" && bun install)`
   - **Go:** Si existe `go.mod`:
     - `(cd "$WORKTREE_PATH" && go mod download)`
   - **Python:** Si existe `pyproject.toml` o `requirements.txt`:
     - Preparar o enlazar el entorno virtual según las convenciones del proyecto.

### C. Instruir al Usuario (Sesión Paralela)

Una vez aprovisionado el worktree y configurado el entorno:
1. Inicializar el work item en el worktree, pasando siempre `--dir` como ruta
   absoluta (`$WORKTREE_PATH`, nunca `../...`):
   ```bash
   sdd-cli start <work-item-id> --dir "$WORKTREE_PATH" --title "<title>" ...
   ```

2. Informar al usuario de forma clara:
   - Indicar la ruta absoluta y relativa del nuevo worktree.
   - Solicitar al usuario que abra una nueva terminal en dicha carpeta y ejecute `claude`:
     ```bash
     cd ../<repo-name>-worktrees/<work-item-id>
     claude
     ```
   - Esto garantiza que el orquestador y los subagentes operen con permisos completos y contexto nativo de working directory.

### D. Eliminar Worktree (`remove`)

Esta operación se realiza **únicamente** tras concluir el flujo de `archive` (cuando el `sdd-archivist` ha realizado commit, push a origin y la PR ha sido creada exitosamente):

1. Confirmar que no queden cambios sin commitear en el worktree:
   ```bash
   git -C "$WORKTREE_PATH" status --porcelain
   ```
   Si hay cambios no guardados, advertir al usuario y abortar la eliminación a menos que confirme descartarlos.
2. Remover el worktree:
   ```bash
   git worktree remove "$WORKTREE_PATH"
   git worktree prune
   ```
3. Informar al usuario que el espacio de trabajo ha sido limpiado y que el trabajo continúa en la Pull Request remota.
