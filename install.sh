#!/usr/bin/env bash
set -euo pipefail

REPO="derekurban2001/authmux"
BINARY_NAME="authmux"
INSTALL_DIR="${AUTHMUX_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${AUTHMUX_VERSION:-latest}" # latest | vX.Y.Z
AUTO_PATH="${AUTHMUX_AUTO_PATH:-1}" # 1/true/yes/on -> persist PATH update

log() { printf "[authmux-install] %s\n" "$*"; }
warn() { printf "[authmux-install] WARN: %s\n" "$*" >&2; }
err() { printf "[authmux-install] ERROR: %s\n" "$*" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || err "Required command not found: $1"
}

is_truthy() {
  case "${1,,}" in
    1|true|yes|on) return 0 ;;
    *) return 1 ;;
  esac
}

path_contains_dir() {
  local dir="$1"
  case ":$PATH:" in
    *":${dir}:"*) return 0 ;;
    *) return 1 ;;
  esac
}

default_profile_file() {
  if [[ -n "${ZSH_VERSION:-}" || "${SHELL:-}" == *zsh ]]; then
    printf "%s" "${HOME}/.zshrc"
  elif [[ -n "${BASH_VERSION:-}" || "${SHELL:-}" == *bash ]]; then
    printf "%s" "${HOME}/.bashrc"
  else
    printf "%s" "${HOME}/.profile"
  fi
}

persist_path_update() {
  local profile line marker
  profile="$(default_profile_file)"
  marker="# Added by authmux installer"
  line="export PATH=\"${INSTALL_DIR}:\$PATH\""

  mkdir -p "$(dirname "$profile")"
  touch "$profile"

  if grep -Fqs "$line" "$profile"; then
    log "PATH already configured in ${profile}"
    return 0
  fi

  printf "\n%s\n%s\n" "$marker" "$line" >> "$profile"
  log "Added ${INSTALL_DIR} to PATH in ${profile}"
}

binary_filename_for_os() {
  local os="$1"
  if [[ "$os" == "windows" ]]; then
    printf "%s.exe" "$BINARY_NAME"
  else
    printf "%s" "$BINARY_NAME"
  fi
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
  local checksums="checksums.txt"
  local base="https://github.com/${REPO}/releases/download/${version}"
  local bin_name
  bin_name="$(binary_filename_for_os "$os")"

  local -a assets
  if [[ "$os" == "windows" ]]; then
    assets=(
      "${BINARY_NAME}_${ver_no_v}_${os}_${arch}.zip"
      "${BINARY_NAME}_${ver_no_v}_${os}_${arch}.tar.gz"
    )
  else
    assets=("${BINARY_NAME}_${ver_no_v}_${os}_${arch}.tar.gz")
  fi

  local tmpdir
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT

  log "Trying GitHub release install: ${version} (${os}/${arch})"

  local fetched_asset=""
  local asset
  for asset in "${assets[@]}"; do
    if fetch "${base}/${asset}" "${tmpdir}/${asset}"; then
      fetched_asset="$asset"
      break
    fi
  done
  if [[ -z "$fetched_asset" ]]; then
    warn "Release asset not found for ${os}/${arch}"
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

  if [[ "$fetched_asset" == *.zip ]]; then
    if command -v unzip >/dev/null 2>&1; then
      unzip -q "${tmpdir}/${fetched_asset}" -d "$tmpdir"
    elif command -v bsdtar >/dev/null 2>&1; then
      bsdtar -xf "${tmpdir}/${fetched_asset}" -C "$tmpdir"
    else
      warn "No unzip tool found; cannot extract ${fetched_asset}"
      return 1
    fi
  else
    tar -xzf "${tmpdir}/${fetched_asset}" -C "$tmpdir"
  fi

  if [[ ! -f "${tmpdir}/${bin_name}" ]]; then
    local found
    found="$(find "$tmpdir" -type f \( -name "$bin_name" -o -name "$BINARY_NAME" \) | head -n1 || true)"
    [[ -n "$found" ]] || err "Could not find ${bin_name} in archive"
    cp "$found" "${tmpdir}/${bin_name}"
  fi

  mkdir -p "$INSTALL_DIR"
  install -m 0755 "${tmpdir}/${bin_name}" "${INSTALL_DIR}/${bin_name}"
  log "Installed ${bin_name} to ${INSTALL_DIR}/${bin_name}"
  return 0
}

install_with_go() {
  local os="$1" version="$2"
  need_cmd go
  log "Falling back to go install"
  if [[ "$version" == "latest" ]]; then
    GO111MODULE=on go install "github.com/derekurban2001/authmux@latest"
  else
    GO111MODULE=on go install "github.com/derekurban2001/authmux@${version}"
  fi
  local gobin
  gobin="$(go env GOBIN)"
  if [[ -z "$gobin" ]]; then
    gobin="$(go env GOPATH)/bin"
  fi

  local bin_name src_bin
  bin_name="$(binary_filename_for_os "$os")"
  if [[ -f "${gobin}/${bin_name}" ]]; then
    src_bin="${gobin}/${bin_name}"
  elif [[ -f "${gobin}/${BINARY_NAME}" ]]; then
    src_bin="${gobin}/${BINARY_NAME}"
  else
    err "go install completed but binary not found at ${gobin}/${bin_name}"
  fi

  mkdir -p "$INSTALL_DIR"
  cp "$src_bin" "${INSTALL_DIR}/${bin_name}"
  chmod +x "${INSTALL_DIR}/${bin_name}"
  log "Installed ${bin_name} to ${INSTALL_DIR}/${bin_name}"
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
      install_with_go "$os" "$VERSION"
    fi
  else
    install_with_go "$os" "$VERSION"
  fi

  if is_truthy "$AUTO_PATH"; then
    if ! path_contains_dir "$INSTALL_DIR"; then
      export PATH="${INSTALL_DIR}:$PATH"
      log "Added ${INSTALL_DIR} to PATH for current shell session"
    fi
    if ! persist_path_update; then
      warn "Could not persist PATH update; add this to your shell config: export PATH=\"${INSTALL_DIR}:\$PATH\""
    fi
  fi

  if ! command -v "$BINARY_NAME" >/dev/null 2>&1 && ! command -v "${BINARY_NAME}.exe" >/dev/null 2>&1; then
    warn "${BINARY_NAME} is installed but not currently on PATH"
    warn "Add this to your shell config: export PATH=\"${INSTALL_DIR}:\$PATH\""
    if is_truthy "$AUTO_PATH"; then
      warn "Restart your shell so PATH changes take effect"
    fi
  fi

  log "Done"
  log "Run: ${BINARY_NAME} --help"
}

main "$@"
