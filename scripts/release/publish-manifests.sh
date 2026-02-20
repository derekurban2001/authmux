#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <tag-version> <manifest-root-dir>" >&2
  exit 1
fi

VERSION_TAG="$1" # vX.Y.Z
VERSION="${VERSION_TAG#v}"
MANIFEST_ROOT="$2"

need_dir() {
  [[ -d "$1" ]] || {
    echo "Required directory not found: $1" >&2
    exit 1
  }
}

publish_homebrew() {
  local tap_repo="${HOMEBREW_TAP_REPO:-}"
  local tap_token="${HOMEBREW_TAP_TOKEN:-}"
  local formula="${MANIFEST_ROOT}/homebrew/authmux.rb"

  if [[ -z "$tap_repo" || -z "$tap_token" ]]; then
    echo "[publish] skipping homebrew: HOMEBREW_TAP_REPO/HOMEBREW_TAP_TOKEN not set"
    return 0
  fi
  need_dir "$(dirname "$formula")"

  local workdir
  workdir="$(mktemp -d)"
  trap 'rm -rf "$workdir"' RETURN

  git clone "https://x-access-token:${tap_token}@github.com/${tap_repo}.git" "$workdir/tap"
  mkdir -p "$workdir/tap/Formula"
  cp "$formula" "$workdir/tap/Formula/authmux.rb"

  (
    cd "$workdir/tap"
    git config user.name "authmux-release-bot"
    git config user.email "actions@users.noreply.github.com"
    git add Formula/authmux.rb
    if git diff --cached --quiet; then
      echo "[publish] homebrew: no changes"
      exit 0
    fi
    git commit -m "authmux ${VERSION}"
    git push origin HEAD
  )
}

publish_scoop() {
  local bucket_repo="${SCOOP_BUCKET_REPO:-}"
  local bucket_token="${SCOOP_BUCKET_TOKEN:-}"
  local manifest="${MANIFEST_ROOT}/scoop/authmux.json"

  if [[ -z "$bucket_repo" || -z "$bucket_token" ]]; then
    echo "[publish] skipping scoop: SCOOP_BUCKET_REPO/SCOOP_BUCKET_TOKEN not set"
    return 0
  fi
  [[ -f "$manifest" ]] || {
    echo "Missing scoop manifest: $manifest" >&2
    exit 1
  }

  local workdir
  workdir="$(mktemp -d)"
  trap 'rm -rf "$workdir"' RETURN

  git clone "https://x-access-token:${bucket_token}@github.com/${bucket_repo}.git" "$workdir/bucket"
  cp "$manifest" "$workdir/bucket/authmux.json"

  (
    cd "$workdir/bucket"
    git config user.name "authmux-release-bot"
    git config user.email "actions@users.noreply.github.com"
    git add authmux.json
    if git diff --cached --quiet; then
      echo "[publish] scoop: no changes"
      exit 0
    fi
    git commit -m "authmux ${VERSION}"
    git push origin HEAD
  )
}

publish_winget() {
  local winget_repo="${WINGET_REPO:-microsoft/winget-pkgs}"
  local winget_token="${WINGET_TOKEN:-}"
  local package_id="${AUTHMUX_WINGET_PACKAGE_ID:-DerekUrban.AuthMux}"
  local publisher_path="${package_id%%.*}"
  local name_path="${package_id#*.}"
  local src_dir="${MANIFEST_ROOT}/winget/manifests/d/${publisher_path}/${name_path}/${VERSION}"

  if [[ -z "$winget_token" ]]; then
    echo "[publish] skipping winget: WINGET_TOKEN not set"
    return 0
  fi
  [[ -d "$src_dir" ]] || {
    echo "Missing winget manifest dir: $src_dir" >&2
    exit 1
  }

  local workdir branch
  workdir="$(mktemp -d)"
  branch="authmux-${VERSION}"
  trap 'rm -rf "$workdir"' RETURN

  git clone "https://x-access-token:${winget_token}@github.com/${winget_repo}.git" "$workdir/winget"
  mkdir -p "$workdir/winget/manifests/d/${publisher_path}/${name_path}/${VERSION}"
  cp "$src_dir/"* "$workdir/winget/manifests/d/${publisher_path}/${name_path}/${VERSION}/"

  (
    cd "$workdir/winget"
    git config user.name "authmux-release-bot"
    git config user.email "actions@users.noreply.github.com"
    git checkout -b "$branch"
    git add "manifests/d/${publisher_path}/${name_path}/${VERSION}"
    if git diff --cached --quiet; then
      echo "[publish] winget: no changes"
      exit 0
    fi
    git commit -m "Add ${package_id} ${VERSION}"
    git push origin "$branch"
    if command -v gh >/dev/null 2>&1; then
      GH_TOKEN="$winget_token" gh pr create \
        --repo "$winget_repo" \
        --title "Add ${package_id} ${VERSION}" \
        --body "Automated manifest update for AuthMux ${VERSION}" \
        --head "$branch" \
        --base master || true
    else
      echo "[publish] winget branch pushed: ${branch}. Create PR manually."
    fi
  )
}

publish_chocolatey() {
  local api_key="${CHOCOLATEY_API_KEY:-}"
  local src_dir="${MANIFEST_ROOT}/chocolatey"

  if [[ -z "$api_key" ]]; then
    echo "[publish] skipping chocolatey: CHOCOLATEY_API_KEY not set"
    return 0
  fi
  if ! command -v choco >/dev/null 2>&1; then
    echo "[publish] skipping chocolatey: choco CLI not found"
    return 0
  fi
  [[ -d "$src_dir" ]] || {
    echo "Missing chocolatey package dir: $src_dir" >&2
    exit 1
  }

  local workdir nupkg
  workdir="$(mktemp -d)"
  trap 'rm -rf "$workdir"' RETURN
  cp -R "$src_dir/"* "$workdir/"

  (
    cd "$workdir"
    choco pack authmux.nuspec
    nupkg="$(ls -1 *.nupkg | head -n1)"
    [[ -n "$nupkg" ]] || {
      echo "Failed to build chocolatey package" >&2
      exit 1
    }
    choco push "$nupkg" --source https://push.chocolatey.org/ --api-key "$api_key"
  )
}

need_dir "$MANIFEST_ROOT"
publish_homebrew
publish_scoop
publish_winget
publish_chocolatey
echo "[publish] done"
