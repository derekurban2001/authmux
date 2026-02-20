#!/usr/bin/env bash
set -euo pipefail

REPO="derekurban2001/authmux"
BINARY_NAME="authmux"
INSTALL_DIR="${AUTHMUX_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${AUTHMUX_VERSION:-latest}" # latest | vX.Y.Z

log() { printf "[authmux-install] %s\n" "$*"; }
warn() { printf "[authmux-install] WARN: %s\n" "$*" >&2; }
err() { printf "[authmux-install] ERROR: %s\n" "$*" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || err "Required command not found: $1"
}

detect_os() {
  local os
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    linux*) echo "linux" ;;
    darwin*) echo "darwin" ;;
    msys*|mingw*|cygwin*) echo "windows" ;;
    *) err "Unsupported OS: $os" ;;
  esac
}

detect_arch() {
  local arch
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) err "Unsupported architecture: $arch" ;;
  esac
}

fetch() {
  local url="$1" out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$out"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$out" "$url"
  else
    err "curl or wget is required"
  fi
}

latest_tag() {
  local api="https://api.github.com/repos/${REPO}/releases/latest"
  local tmp
  tmp="$(mktemp)"
  if ! fetch "$api" "$tmp" 2>/dev/null; then
    rm -f "$tmp"
    return 1
  fi
  local tag
  tag="$(grep -Eo '"tag_name"\s*:\s*"[^"]+"' "$tmp" | head -n1 | sed -E 's/.*"([^"]+)"/\1/')"
  rm -f "$tmp"
  if [[ -z "$tag" ]]; then
    return 1
  fi
  printf "%s" "$tag"
}

install_from_release() {
  local os="$1" arch="$2" version="$3"
  local ver_no_v="${version#v}"
  local asset="${BINARY_NAME}_${ver_no_v}_${os}_${arch}.tar.gz"
  local checksums="checksums.txt"
  local base="https://github.com/${REPO}/releases/download/${version}"

  local tmpdir
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT

  log "Trying GitHub release install: ${version} (${os}/${arch})"
  if ! fetch "${base}/${asset}" "${tmpdir}/${asset}"; then
    warn "Release asset not found: ${asset}"
    return 1
  fi

  if fetch "${base}/${checksums}" "${tmpdir}/${checksums}"; then
    if command -v shasum >/dev/null 2>&1; then
      (cd "$tmpdir" && shasum -a 256 -c checksums.txt --ignore-missing) || err "Checksum verification failed"
    elif command -v sha256sum >/dev/null 2>&1; then
      (cd "$tmpdir" && sha256sum -c checksums.txt --ignore-missing) || err "Checksum verification failed"
    else
      warn "No sha256 tool found; skipping checksum verification"
    fi
  else
    warn "No checksums.txt found for ${version}; skipping checksum verification"
  fi

  tar -xzf "${tmpdir}/${asset}" -C "$tmpdir"
  if [[ ! -f "${tmpdir}/${BINARY_NAME}" ]]; then
    local found
    found="$(find "$tmpdir" -type f -name "$BINARY_NAME" | head -n1 || true)"
    [[ -n "$found" ]] || err "Could not find ${BINARY_NAME} in archive"
    cp "$found" "${tmpdir}/${BINARY_NAME}"
  fi

  mkdir -p "$INSTALL_DIR"
  install -m 0755 "${tmpdir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
  log "Installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}"
  return 0
}

install_with_go() {
  need_cmd go
  log "Falling back to go install"
  if [[ "$VERSION" == "latest" ]]; then
    GO111MODULE=on go install "github.com/derekurban2001/authmux@latest"
  else
    GO111MODULE=on go install "github.com/derekurban2001/authmux@${VERSION}"
  fi
  local gobin
  gobin="$(go env GOBIN)"
  if [[ -z "$gobin" ]]; then
    gobin="$(go env GOPATH)/bin"
  fi
  if [[ ! -f "${gobin}/${BINARY_NAME}" ]]; then
    err "go install completed but binary not found at ${gobin}/${BINARY_NAME}"
  fi
  mkdir -p "$INSTALL_DIR"
  cp "${gobin}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
  chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
  log "Installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}"
}

main() {
  local os arch resolved_version
  os="$(detect_os)"
  arch="$(detect_arch)"

  resolved_version="$VERSION"
  if [[ "$VERSION" == "latest" ]]; then
    resolved_version="$(latest_tag || true)"
    if [[ -z "$resolved_version" ]]; then
      warn "Could not resolve latest release tag; will use go install"
    fi
  fi

  if [[ -n "${resolved_version:-}" && "$resolved_version" != "latest" ]]; then
    if install_from_release "$os" "$arch" "$resolved_version"; then
      :
    else
      install_with_go
    fi
  else
    install_with_go
  fi

  if ! command -v "$BINARY_NAME" >/dev/null 2>&1; then
    warn "${BINARY_NAME} is installed but not currently on PATH"
    warn "Add this to your shell config: export PATH=\"${INSTALL_DIR}:\$PATH\""
  fi

  log "Done"
  log "Run: ${BINARY_NAME} --help"
}

main "$@"
