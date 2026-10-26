# Guía de instalación de sdd-cli

> Estado: **BETA**.
> Audiencia: usuarios que quieren instalar y usar el motor SDD.
> Alcance: instalación del binario `sdd-cli` en Windows, macOS y Linux (Ubuntu/WSL).

---

## 1. Qué voy a instalar

`sdd-cli` es **un único binario nativo** escrito en Go. No necesita Node.js, Python
ni ningún runtime: se descarga un solo ejecutable, se pone en el `PATH` y queda listo.

- No hay instalador con dependencias ni servicios en segundo plano.
- "Instalar" = copiar el ejecutable a una carpeta que esté en tu `PATH`.
- "Actualizar" = reemplazar ese ejecutable por uno más nuevo.
- "Desinstalar" = borrar ese ejecutable.

Los binarios se publican en **GitHub Releases**:
<https://github.com/matiasezequieldimuro/sdd-framework/releases>

---

## 2. Qué archivo me corresponde

Elegí el archivo según tu sistema operativo y arquitectura:

| Sistema operativo | Arquitectura | Archivo a descargar |
| --- | --- | --- |
| Linux (Ubuntu, WSL) | x86_64 / amd64 | `sdd-cli_<version>_linux_amd64.tar.gz` |
| macOS Intel | x86_64 / amd64 | `sdd-cli_<version>_darwin_amd64.tar.gz` |
| macOS Apple Silicon (M1/M2/M3) | arm64 | `sdd-cli_<version>_darwin_arm64.tar.gz` |
| Windows | x86_64 / amd64 | `sdd-cli_<version>_windows_amd64.zip` |

`<version>` es la etiqueta del release, por ejemplo `v0.1.0-beta`.
Cada release incluye `SHA256SUMS` para verificar la integridad.

Cómo saber tu arquitectura:

```bash
# macOS / Linux
uname -m        # x86_64 -> amd64 ; arm64 o aarch64 -> arm64
```

```powershell
# Windows
$env:PROCESSOR_ARCHITECTURE   # AMD64 -> amd64
```

---

## 3. Métodos de instalación

Hay tres caminos. Elegí **uno**:

- **3.1 Rápida (recomendada):** un comando, resuelve todo automáticamente.
- **3.2 Manual desde Releases:** descargás el archivo vos mismo.
- **3.3 Compilar desde fuente:** requiere Go, útil para desarrollo.

### 3.1. Instalación rápida (recomendada)

Detecta tu plataforma, descarga el binario correcto del último release, verifica el
checksum y lo instala en el `PATH`.

**macOS y Linux (Ubuntu/WSL):**

```bash
curl -fsSL https://raw.githubusercontent.com/matiasezequieldimuro/sdd-framework/main/scripts/install.sh | sh
```

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/matiasezequieldimuro/sdd-framework/main/scripts/install.ps1 | iex
```

Opciones (variables de entorno):

| Variable | Efecto |
| --- | --- |
| `SDD_VERSION` | Instala una versión específica (ej. `v0.1.0-beta`). Default: último release. |
| `SDD_INSTALL_DIR` | Carpeta destino. Default: si ya hay un `sdd-cli` en el `PATH`, lo sobrescribe en su lugar; si no, `/usr/local/bin` (o `~/.local/bin`) en Unix y `%LOCALAPPDATA%\Programs\sdd-cli` en Windows. |

```bash
# Ejemplo: versión fija y carpeta propia
SDD_VERSION=v0.1.0-beta SDD_INSTALL_DIR="$HOME/.local/bin" \
  curl -fsSL https://raw.githubusercontent.com/matiasezequieldimuro/sdd-framework/main/scripts/install.sh | sh
```

### 3.2. Instalación manual desde Releases

Usá este método si preferís no ejecutar el script. Es el mismo para instalar por
primera vez o para actualizar (sobrescribe el binario).

> **Tip:** definimos `VERSION` una sola vez al principio y el resto de los comandos
> se copian y pegan tal cual. Cambiá solo esa línea por la versión que quieras
> (mirá la última en la [página de Releases](https://github.com/matiasezequieldimuro/sdd-framework/releases)).

**Linux (Ubuntu / WSL) — amd64:**

```bash
VERSION="v0.1.0-beta"     # <-- única línea a ajustar

cd /tmp
curl -fsSLO "https://github.com/matiasezequieldimuro/sdd-framework/releases/download/${VERSION}/sdd-cli_${VERSION}_linux_amd64.tar.gz"
curl -fsSLO "https://github.com/matiasezequieldimuro/sdd-framework/releases/download/${VERSION}/SHA256SUMS"

# Verificar integridad (debe imprimir: ...linux_amd64.tar.gz: OK)
sha256sum -c SHA256SUMS --ignore-missing

# Instalar (sobrescribe cualquier versión previa en esa ruta)
tar -xzf "sdd-cli_${VERSION}_linux_amd64.tar.gz"
sudo install -m 0755 sdd-cli /usr/local/bin/sdd-cli

hash -r                   # limpia el cache de rutas del shell
sdd-cli version
```

**macOS — Intel (amd64) o Apple Silicon (arm64):**

```bash
VERSION="v0.1.0-beta"
ARCH="arm64"              # Intel: usá "amd64"

cd /tmp
curl -fsSLO "https://github.com/matiasezequieldimuro/sdd-framework/releases/download/${VERSION}/sdd-cli_${VERSION}_darwin_${ARCH}.tar.gz"
curl -fsSLO "https://github.com/matiasezequieldimuro/sdd-framework/releases/download/${VERSION}/SHA256SUMS"

shasum -a 256 -c SHA256SUMS --ignore-missing

tar -xzf "sdd-cli_${VERSION}_darwin_${ARCH}.tar.gz"
sudo install -m 0755 sdd-cli /usr/local/bin/sdd-cli

# macOS: el binario no está firmado por Apple; autorizalo si Gatekeeper lo bloquea
xattr -d com.apple.quarantine /usr/local/bin/sdd-cli 2>/dev/null || true

hash -r
sdd-cli version
```

**Windows (PowerShell) — amd64:**

```powershell
$Version = "v0.1.0-beta"
$Dir = "$env:LOCALAPPDATA\Programs\sdd-cli"

$zip = "$env:TEMP\sdd-cli.zip"
Invoke-WebRequest -Uri "https://github.com/matiasezequieldimuro/sdd-framework/releases/download/$Version/sdd-cli_${Version}_windows_amd64.zip" -OutFile $zip -UseBasicParsing

New-Item -ItemType Directory -Path $Dir -Force | Out-Null
Expand-Archive -Path $zip -DestinationPath $Dir -Force

# Agregar al PATH de usuario (una sola vez)
$userPath = [Environment]::GetEnvironmentVariable('Path','User')
if ($userPath -notlike "*$Dir*") {
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$Dir", 'User')
}
# Abrí una NUEVA terminal y luego:
sdd-cli version
```

### 3.3. Compilar desde fuente

Requiere [Go 1.22 o superior](https://go.dev/dl/).

```bash
git clone https://github.com/matiasezequieldimuro/sdd-framework.git
cd sdd-framework/src/cli

go generate ./...    # sincroniza recursos embebidos (.sdd y adapters)
go build -ldflags "-X sdd-cli/cmd.version=v0.1.0-dev" -o sdd-cli .

sudo install -m 0755 sdd-cli /usr/local/bin/sdd-cli   # opcional: dejarlo en el PATH
sdd-cli version
```

> **Cuidado al mezclar `go install` con la instalación por curl/release.** `go install`
> deja el binario en `~/go/bin/sdd-cli` (tu `GOBIN`/`GOPATH/bin`), mientras que el
> instalador rápido o la descarga manual lo dejan en `/usr/local/bin` o `~/.local/bin`.
> Si tenés ambos, **el que gana es el que aparece primero en tu `PATH`**, no el que
> instalaste último: por eso podés "instalar la versión nueva" y seguir ejecutando una
> vieja. Además, un binario hecho con `go install`/`go build` sin `-ldflags` reporta
> `version = dev`, así que no siempre distinguís cuál estás corriendo. Recomendaciones:
> para desarrollo usá un solo camino (por ejemplo `go run .` o `go build -o /tmp/sdd .`
> e invocalo por ruta), o mantené dev y release en carpetas separadas y controlá el
> orden del `PATH`. Ante la duda, corré `type -a sdd-cli` (macOS/Linux) o
> `Get-Command sdd-cli -All` (Windows) para ver todas las copias, y `sdd-cli version`
> para confirmar cuál se ejecuta. El instalador rápido, cuando ya existe un `sdd-cli`
> en el `PATH`, sobrescribe esa copia activa y avisa si detecta otras.

---

## 4. Verificar la instalación

```bash
sdd-cli version      # muestra versión, commit y fecha de compilación
sdd-cli --help       # lista los comandos disponibles
```

Salida esperada de `sdd-cli version`:

```text
sdd-cli v0.1.0-beta (commit 91e5b13, built 2026-09-16T15:03:14Z)
```

Para confirmar qué ejecutable se está usando y detectar copias viejas:

```bash
type -a sdd-cli      # macOS/Linux: lista TODAS las copias en el PATH, en orden
```

```powershell
Get-Command sdd-cli -All   # Windows
```

---

## 5. Reinstalar / actualizar

Actualizar es simplemente **reemplazar el binario** por uno más nuevo.

> **Excepción:** pasar de un binario 0.x a 1.x requiere migrar el contrato del proyecto
> (ver §5.1).

- **Si instalaste con el método rápido (3.1):** volvé a ejecutar el mismo comando;
  sobrescribe el binario con el último release.

  ```bash
  # macOS / Linux
  curl -fsSL https://raw.githubusercontent.com/matiasezequieldimuro/sdd-framework/main/scripts/install.sh | sh
  ```

  ```powershell
  # Windows
  irm https://raw.githubusercontent.com/matiasezequieldimuro/sdd-framework/main/scripts/install.ps1 | iex
  ```

- **Si instalaste manual (3.2):** repetí los pasos con la nueva `VERSION`. El
  comando `install -m 0755 ...` sobrescribe el binario existente en su lugar.

> **Importante:** después de actualizar corré `hash -r` (bash/zsh) y verificá con
> `type -a sdd-cli` que no haya quedado **otra copia vieja antes en el PATH**. Si la
> hay, borrala (ver sección 6). Es la causa más común de "ejecuté la versión nueva
> pero sigue comportándose como la vieja".

### 5.1 Migrar un proyecto existente al contrato 1.0

`sdd-cli` 1.x solo acepta el contrato `schema_version: "1.0"`. No hay comando de migración
todavía, así que la migración es manual.

1. **Antes de actualizar el binario** (con el binario 0.x):
   - Verificar que `.sdd/work-items/.transactions/` no tenga transacciones en curso:
     - Un marker `<id>.archive.json` o `<id>.cancel.json` indica un archivado o una
       cancelación interrumpida. Ejecutar `sdd-cli status <id>` con el binario 0.x: la
       transacción se completa o se revierte y el marker desaparece.
     - Un `<id>.backup` cuyo work item **no** está en `active/` indica un guardado
       interrumpido. `sdd-cli status <id>` con el binario 0.x restaura el work item.
     - Un `<id>.backup` cuyo work item **sí** está en `active/` es un resto de un guardado
       ya publicado. Es inofensivo: el motor lo elimina en la próxima escritura del item.
   - Verificar que `sdd-cli validate` pase.
   - Hacer commit o backup de `.sdd/`.
2. Instalar el binario 1.x.
3. Obtener el contrato nuevo: `sdd-cli init --dir <tmp-vacío>`.
4. Reemplazar `.sdd/schemas/*.json` por los de `<tmp>/.sdd/schemas/`. El motor lee los
   schemas del proyecto.
5. Cambiar `schema_version: "0.1"` → `"1.0"` en:
   - `.sdd/config.yaml`
   - `.sdd/registry/capabilities.yaml`
   - `.sdd/workflows/*.workflow.yaml`
   - `.sdd/templates/*.md` (front-matter)

   Si no los personalizaste, podés reemplazarlos por los de `<tmp>`. Si los personalizaste,
   editá solo esa línea.
6. En **todos** los work items (`.sdd/work-items/active/`, `archive/`, `cancelled/`):
   - `manifest.yaml`: `schema_version: "1.0"` y `workflow.version: "1.0"`.
   - `artifacts/*.md`: front-matter `schema_version: "1.0"`.
   - **No editar `events.jsonl`**: las líneas históricas `"0.1"` siguen siendo válidas.
7. Opcional: el front-matter de documentos propios (por ejemplo `.sdd/context/specs/*.md`). El
   motor no lo valida. No editar archivos usados como `--from-artifact`, porque su hash
   SHA-256 está registrado en el manifest y `validate` fallaría.
8. Los archivos del adapter (`.claude/`, `CLAUDE.md`, `.mcp.json`) no llevan la versión y no
   requieren cambios.
9. Ejecutar `sdd-cli validate`. Las checks `*.schema_version_supported`, `*.schema_valid` y
   `artifact.identity_valid` señalan cada archivo pendiente.

---

## 6. Desinstalar

1. Encontrá dónde está(n) instalado(s):

   ```bash
   type -a sdd-cli          # macOS/Linux
   ```
   ```powershell
   Get-Command sdd-cli -All  # Windows
   ```

2. Borrá el/los ejecutable(s):

   ```bash
   # macOS / Linux (una línea por cada ruta que haya reportado type -a)
   sudo rm /usr/local/bin/sdd-cli
   rm ~/.local/bin/sdd-cli 2>/dev/null || true
   ```

   ```powershell
   # Windows
   Remove-Item "$env:LOCALAPPDATA\Programs\sdd-cli\sdd-cli.exe"
   ```

3. (Opcional) Quitá la carpeta del `PATH` si la habías agregado a mano
   (Windows: "Editar las variables de entorno del sistema"; Unix: la línea
   `export PATH=...` en tu `~/.bashrc` / `~/.zshrc`).

> Desinstalar el binario **no** borra las carpetas `.sdd/` de tus proyectos; esas
> son datos tuyos y se eliminan aparte si así lo querés.

---

## 7. Primeros pasos

Dentro de tu proyecto:

```bash
sdd-cli init                          # crea la estructura .sdd/
sdd-cli adapters install claude-code  # (opcional) adapter de Claude Code
sdd-cli --help                        # ver todos los comandos
```

Seguí con:

- [**CLI.md**](./CLI.md) — referencia completa de comandos, flags y ejemplos.
- [**SDD_WORKFLOW.md**](./SDD_WORKFLOW.md) — escenarios, fases, artefactos y fuentes de verdad.

---

## 8. Solución de problemas

| Síntoma | Causa probable | Solución |
| --- | --- | --- |
| `sdd-cli: command not found` | La carpeta de instalación no está en el `PATH` | Agregala al `PATH` (ver 3.1/3.2) y reabrí la terminal |
| `unknown command "version"` o comportamiento viejo tras actualizar | Hay un binario **viejo antes en el PATH** | `type -a sdd-cli` para ubicarlo, borralo (sección 6) y corré `hash -r` |
| macOS bloquea el binario | Gatekeeper / quarantine (no está firmado) | `xattr -d com.apple.quarantine <ruta>/sdd-cli` |
| PowerShell no ejecuta el script | Política de ejecución | `Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass` |
| `404 Not Found` al descargar | `VERSION` incorrecta o release inexistente | Copiá la etiqueta exacta desde la página de Releases |
| `could not determine the latest release tag` | Aún no hay releases o problema de red | Especificá `SDD_VERSION` manualmente |
| Checksum mismatch | Descarga corrupta | Reintentá la descarga |
