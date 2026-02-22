#!/usr/bin/env bash
set -euo pipefail

REPO="${PROFLEX_REPO:-derekurban/proflex-cli}"
OFFICIAL_REPO="derekurban/proflex-cli"
LEGACY_REPO="derekurban2001/proflex-cli"
BINARY_NAME="proflex"
INSTALL_DIR="${PROFLEX_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${PROFLEX_VERSION:-latest}" # latest | vX.Y.Z
AUTO_PATH="${PROFLEX_AUTO_PATH:-1}" # 1/true/yes/on -> persist PATH update
VERIFY_SIGNATURES="${PROFLEX_VERIFY_SIGNATURES:-1}" # 1/true/yes/on -> enforce cosign verification
ALLOW_SOURCE_FALLBACK="${PROFLEX_ALLOW_SOURCE_FALLBACK:-0}" # 1/true/yes/on -> allow go install fallback
COSIGN_VERSION="${PROFLEX_COSIGN_VERSION:-v2.5.3}"
if [[ "$REPO" == "$OFFICIAL_REPO" || "$REPO" == "$LEGACY_REPO" ]]; then
  DEFAULT_COSIGN_IDENTITY_RE="^https://github.com/(derekurban/proflex-cli|derekurban2001/proflex-cli)/.github/workflows/release.yml@refs/tags/.*$"
else
  DEFAULT_COSIGN_IDENTITY_RE="^https://github.com/${REPO}/.github/workflows/release.yml@refs/tags/.*$"
fi
COSIGN_IDENTITY_RE="${PROFLEX_COSIGN_IDENTITY_RE:-$DEFAULT_COSIGN_IDENTITY_RE}"
COSIGN_OIDC_ISSUER="${PROFLEX_COSIGN_OIDC_ISSUER:-https://token.actions.githubusercontent.com}"

log() { printf "[proflex-install] %s\n" "$*"; }
warn() { printf "[proflex-install] WARN: %s\n" "$*" >&2; }
err() { printf "[proflex-install] ERROR: %s\n" "$*" >&2; exit 1; }

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
  marker="# Added by proflex installer"
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

sha256_file() {
  local file="$1"
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  elif command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$file" | awk '{print $NF}'
  else
    err "No SHA256 tool found (need shasum, sha256sum, or openssl)"
  fi
}

expected_hash_for_asset() {
  local checksums_file="$1" asset="$2"
  awk -v a="$asset" '$2 == a { print $1; exit }' "$checksums_file"
}

ensure_cosign() {
  local os="$1" arch="$2" tmpdir="$3"
  if command -v cosign >/dev/null 2>&1; then
    command -v cosign
    return 0
  fi

  local asset suffix url out
  suffix=""
  if [[ "$os" == "windows" ]]; then
    suffix=".exe"
  fi
  asset="cosign-${os}-${arch}${suffix}"
  url="https://github.com/sigstore/cosign/releases/download/${COSIGN_VERSION}/${asset}"
  out="${tmpdir}/${asset}"

  log "cosign not found; downloading ${COSIGN_VERSION} (${os}/${arch})"
  fetch "$url" "$out" || {
    warn "Unable to download cosign from ${url}"
    return 1
  }
  chmod +x "$out" || true
  printf "%s" "$out"
}

verify_checksums_signature() {
  local os="$1" arch="$2" tmpdir="$3" checksums_file="$4" sig_file="$5" cert_file="$6"
  if ! is_truthy "$VERIFY_SIGNATURES"; then
    warn "Signature verification disabled via PROFLEX_VERIFY_SIGNATURES=0"
    return 0
  fi

  local cosign_bin
  cosign_bin="$(ensure_cosign "$os" "$arch" "$tmpdir")" || return 1
  "$cosign_bin" verify-blob \
    --certificate "$cert_file" \
    --signature "$sig_file" \
    --certificate-identity-regexp "$COSIGN_IDENTITY_RE" \
    --certificate-oidc-issuer "$COSIGN_OIDC_ISSUER" \
    "$checksums_file" >/dev/null
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
  local checksums_sig="checksums.txt.sig"
  local checksums_cert="checksums.txt.pem"
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
  trap 'rm -rf "'"${tmpdir}"'"' RETURN

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

  fetch "${base}/${checksums}" "${tmpdir}/${checksums}" || {
    warn "Missing release checksum file (${checksums})"
    return 1
  }
  if is_truthy "$VERIFY_SIGNATURES"; then
    fetch "${base}/${checksums_sig}" "${tmpdir}/${checksums_sig}" || {
      warn "Missing release signature file (${checksums_sig})"
      return 1
    }
    fetch "${base}/${checksums_cert}" "${tmpdir}/${checksums_cert}" || {
      warn "Missing release certificate file (${checksums_cert})"
      return 1
    }
    if ! verify_checksums_signature "$os" "$arch" "$tmpdir" "${tmpdir}/${checksums}" "${tmpdir}/${checksums_sig}" "${tmpdir}/${checksums_cert}"; then
      warn "Signature verification failed for ${version}"
      return 1
    fi
  else
    warn "Signature verification disabled via PROFLEX_VERIFY_SIGNATURES=0"
  fi

  local expected_hash actual_hash
  expected_hash="$(expected_hash_for_asset "${tmpdir}/${checksums}" "$fetched_asset")"
  if [[ -z "$expected_hash" ]]; then
    warn "No checksum entry found for ${fetched_asset}"
    return 1
  fi
  actual_hash="$(sha256_file "${tmpdir}/${fetched_asset}")"
  if [[ "$expected_hash" != "$actual_hash" ]]; then
    warn "Checksum mismatch for ${fetched_asset}"
    return 1
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
    if [[ -z "$found" ]]; then
      warn "Could not find ${bin_name} in archive"
      return 1
    fi
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
    GO111MODULE=on go install "github.com/derekurban/proflex-cli@latest"
  else
    GO111MODULE=on go install "github.com/derekurban/proflex-cli@${version}"
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
      if is_truthy "$ALLOW_SOURCE_FALLBACK"; then
        warn "Could not resolve latest release tag; will use go install fallback"
      else
        err "Could not resolve latest release tag and source fallback is disabled (set PROFLEX_ALLOW_SOURCE_FALLBACK=1 to enable)"
      fi
    fi
  fi

  if [[ -n "${resolved_version:-}" && "$resolved_version" != "latest" ]]; then
    if install_from_release "$os" "$arch" "$resolved_version"; then
      :
    else
      if is_truthy "$ALLOW_SOURCE_FALLBACK"; then
        warn "Release install failed; using go install fallback"
        install_with_go "$os" "$VERSION"
      else
        err "Release install failed and source fallback is disabled (set PROFLEX_ALLOW_SOURCE_FALLBACK=1 to enable)"
      fi
    fi
  else
    if is_truthy "$ALLOW_SOURCE_FALLBACK"; then
      install_with_go "$os" "$VERSION"
    else
      err "No release version resolved and source fallback is disabled (set PROFLEX_ALLOW_SOURCE_FALLBACK=1 to enable)"
    fi
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
