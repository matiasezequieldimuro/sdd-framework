<#
.SYNOPSIS
    sdd-cli installer for Windows (PowerShell).

.DESCRIPTION
    Downloads the latest sdd-cli release for windows/amd64, verifies its checksum,
    installs it and adds the install directory to the user PATH.

.EXAMPLE
    irm https://raw.githubusercontent.com/matiasezequieldimuro/sdd-framework/main/scripts/install.ps1 | iex

.NOTES
    Environment overrides:
      $env:SDD_VERSION      Install a specific tag (e.g. v0.1.0-beta). Default: latest release.
      $env:SDD_INSTALL_DIR  Target directory. Default: $env:LOCALAPPDATA\Programs\sdd-cli
#>

$ErrorActionPreference = 'Stop'

$Repo = 'matiasezequieldimuro/sdd-framework'
$Binary = 'sdd-cli'

function Write-Info($msg) { Write-Host "==> $msg" -ForegroundColor Cyan }

# --- Detect architecture ---------------------------------------------------
$arch = 'amd64'
if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') {
    throw "windows/arm64 is not published. Build from source: see docs/10-guia-instalacion.md"
}

# --- Resolve version -------------------------------------------------------
$version = $env:SDD_VERSION
if ([string]::IsNullOrWhiteSpace($version)) {
    Write-Info "Resolving latest release..."
    # First entry of the releases list = most recent (includes pre-releases like beta).
    $releases = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases" -Headers @{ 'User-Agent' = 'sdd-cli-installer' }
    $version = $releases[0].tag_name
    if ([string]::IsNullOrWhiteSpace($version)) {
        throw "Could not determine the latest release tag. Set `$env:SDD_VERSION manually."
    }
}
Write-Info "Installing $Binary $version for windows/$arch"

# --- Download & verify -----------------------------------------------------
$archive = "${Binary}_${version}_windows_${arch}.zip"
$baseUrl = "https://github.com/$Repo/releases/download/$version"
$tmp = Join-Path $env:TEMP ("sdd-cli-" + [System.Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tmp -Force | Out-Null

try {
    $zipPath = Join-Path $tmp $archive
    Write-Info "Downloading $archive..."
    Invoke-WebRequest -Uri "$baseUrl/$archive" -OutFile $zipPath -UseBasicParsing

    Write-Info "Verifying checksum..."
    $sumsPath = Join-Path $tmp 'SHA256SUMS'
    try {
        Invoke-WebRequest -Uri "$baseUrl/SHA256SUMS" -OutFile $sumsPath -UseBasicParsing
        $line = Select-String -Path $sumsPath -Pattern ([regex]::Escape($archive)) | Select-Object -First 1
        if ($line) {
            $expected = ($line.Line -split '\s+')[0]
            $actual = (Get-FileHash -Path $zipPath -Algorithm SHA256).Hash.ToLower()
            if ($expected.ToLower() -ne $actual) {
                throw "Checksum mismatch for $archive"
            }
            Write-Info "Checksum OK."
        }
        else {
            Write-Warning "$archive not listed in SHA256SUMS, skipping verification."
        }
    }
    catch {
        Write-Warning "SHA256SUMS not available, skipping verification."
    }

    # --- Install -----------------------------------------------------------
    # If the user did not force a directory and sdd-cli is already resolvable,
    # install over that active copy so the upgrade actually takes effect.
    $installDir = $env:SDD_INSTALL_DIR
    $active = Get-Command $Binary -ErrorAction SilentlyContinue
    if ([string]::IsNullOrWhiteSpace($installDir)) {
        if ($active) {
            $installDir = Split-Path -Parent $active.Source
            Write-Info "Existing $Binary found at $($active.Source); it will be overwritten."
        }
        else {
            $installDir = Join-Path $env:LOCALAPPDATA 'Programs\sdd-cli'
        }
    }
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null

    Expand-Archive -Path $zipPath -DestinationPath $tmp -Force
    Copy-Item -Path (Join-Path $tmp "$Binary.exe") -Destination (Join-Path $installDir "$Binary.exe") -Force
    Write-Info "Installed to $installDir\$Binary.exe"

    # Warn about other copies elsewhere on PATH (stale copies are the usual cause
    # of old-behaviour surprises after an upgrade).
    $others = @(Get-Command $Binary -All -ErrorAction SilentlyContinue |
        Where-Object { (Split-Path -Parent $_.Source) -ne $installDir })
    if ($others.Count -gt 0) {
        Write-Warning "Other $Binary copies exist on your PATH:"
        $others | ForEach-Object { Write-Warning "  $($_.Source)" }
        Write-Warning "Remove them to avoid confusion."
    }

    # --- PATH --------------------------------------------------------------
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($userPath -notlike "*$installDir*") {
        $newPath = if ([string]::IsNullOrEmpty($userPath)) { $installDir } else { "$userPath;$installDir" }
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
        Write-Info "Added $installDir to your user PATH. Restart the terminal to pick it up."
    }

    Write-Host ""
    Write-Info "Done. Verify with: $Binary version"
}
finally {
    Remove-Item -Path $tmp -Recurse -Force -ErrorAction SilentlyContinue
}
