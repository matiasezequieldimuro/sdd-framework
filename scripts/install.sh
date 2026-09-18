#!/usr/bin/env sh
# sdd-cli installer for macOS and Linux.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/matiasezequieldimuro/sdd-framework/main/scripts/install.sh | sh
#
# Environment overrides:
#   SDD_VERSION      Install a specific tag (e.g. v0.1.0-beta). Default: latest release.
#   SDD_INSTALL_DIR  Target directory for the binary. Default: /usr/local/bin (or ~/.local/bin
#                    when /usr/local/bin is not writable).
set -eu

REPO="matiasezequieldimuro/sdd-framework"
BINARY="sdd-cli"

info() { printf '\033[0;34m==>\033[0m %s\n' "$1"; }
err() { printf '\033[0;31merror:\033[0m %s\n' "$1" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || err "'$1' is required but was not found in PATH."; }

need curl
need tar

# --- Detect platform -------------------------------------------------------
os="$(uname -s)"
case "$os" in
  Linux) os="linux" ;;
  Darwin) os="darwin" ;;
  *) err "unsupported operating system: $os" ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *) err "unsupported architecture: $arch" ;;
esac

# We publish linux/amd64, darwin/amd64 and darwin/arm64.
if [ "$os" = "linux" ] && [ "$arch" != "amd64" ]; then
  err "no published build for linux/$arch. Build from source: see docs/10-guia-instalacion.md"
fi

# --- Resolve version -------------------------------------------------------
version="${SDD_VERSION:-}"
if [ -z "$version" ]; then
  info "Resolving latest release..."
  # Capture the full response first (avoids curl SIGPIPE/exit-23 from an early
  # pipe close), then take the first tag_name = most recent release (includes
  # pre-releases like beta).
  releases_json="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases")" \
    || err "could not reach the GitHub releases API. Set SDD_VERSION manually."
  version="$(printf '%s' "$releases_json" | awk -F'"' '/"tag_name":/{print $4; exit}')"
  [ -n "$version" ] || err "could not determine the latest release tag. Set SDD_VERSION manually."
fi
info "Installing ${BINARY} ${version} for ${os}/${arch}"

# --- Download & verify -----------------------------------------------------
archive="${BINARY}_${version}_${os}_${arch}.tar.gz"
base_url="https://github.com/${REPO}/releases/download/${version}"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

info "Downloading ${archive}..."
curl -fsSL "${base_url}/${archive}" -o "${tmp}/${archive}" \
  || err "download failed: ${base_url}/${archive}"

info "Verifying checksum..."
if curl -fsSL "${base_url}/SHA256SUMS" -o "${tmp}/SHA256SUMS" 2>/dev/null; then
  expected="$(grep " ${archive}\$" "${tmp}/SHA256SUMS" | awk '{print $1}' | head -n1)"
  if [ -n "$expected" ]; then
    if command -v sha256sum >/dev/null 2>&1; then
      actual="$(sha256sum "${tmp}/${archive}" | awk '{print $1}')"
    else
      actual="$(shasum -a 256 "${tmp}/${archive}" | awk '{print $1}')"
    fi
    [ "$expected" = "$actual" ] || err "checksum mismatch for ${archive}"
    info "Checksum OK."
  else
    printf 'warning: %s not listed in SHA256SUMS, skipping verification\n' "$archive" >&2
  fi
else
  printf 'warning: SHA256SUMS not available, skipping verification\n' >&2
fi

# --- Install ---------------------------------------------------------------
tar -C "$tmp" -xzf "${tmp}/${archive}"

# Choose the install directory. When the user did not force one and an sdd-cli is
# already resolvable on PATH, install over that same active copy so the upgrade
# actually takes effect (instead of leaving the old one shadowing the new one).
install_dir="${SDD_INSTALL_DIR:-}"
active_binary="$(command -v "$BINARY" 2>/dev/null || true)"
if [ -z "$install_dir" ]; then
  if [ -n "$active_binary" ]; then
    install_dir="$(dirname "$active_binary")"
    info "Existing ${BINARY} found at ${active_binary}; it will be overwritten."
  elif [ -w /usr/local/bin ] 2>/dev/null; then
    install_dir="/usr/local/bin"
  else
    install_dir="${HOME}/.local/bin"
  fi
fi
mkdir -p "$install_dir"

if mv "${tmp}/${BINARY}" "${install_dir}/${BINARY}" 2>/dev/null; then
  :
else
  info "Elevated permissions needed to write to ${install_dir}"
  sudo mv "${tmp}/${BINARY}" "${install_dir}/${BINARY}"
fi
chmod +x "${install_dir}/${BINARY}" 2>/dev/null || sudo chmod +x "${install_dir}/${BINARY}"

info "Installed to ${install_dir}/${BINARY}"

case ":${PATH}:" in
  *":${install_dir}:"*) ;;
  *)
    printf '\n'
    printf 'note: %s is not on your PATH. Add this to your shell profile:\n' "$install_dir"
    printf '  export PATH="%s:$PATH"\n' "$install_dir"
    ;;
esac

# Warn about other sdd-cli copies elsewhere on PATH. They are now shadowed by the
# fresh install, but stale copies earlier in PATH are the usual cause of
# "unknown command"/old-behaviour surprises, so surface them explicitly.
others=""
saved_ifs="$IFS"
IFS=:
for dir in $PATH; do
  [ -z "$dir" ] && dir="."
  [ "$dir" = "$install_dir" ] && continue
  if [ -x "${dir}/${BINARY}" ]; then
    others="${others}  ${dir}/${BINARY}\n"
  fi
done
IFS="$saved_ifs"
if [ -n "$others" ]; then
  printf '\n'
  printf 'warning: other %s copies exist on your PATH:\n' "$BINARY" >&2
  printf "$others" >&2
  printf 'Remove them to avoid confusion, e.g.: rm <path> (use sudo if needed).\n' >&2
fi

printf '\n'
info "Done. Verify with: ${BINARY} version"
