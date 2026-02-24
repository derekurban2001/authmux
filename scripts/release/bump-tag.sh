#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  ./scripts/release/bump-tag.sh --patch|--minor|--major [options]

Options:
  --dry-run       Print actions without creating/pushing tag
  --remote <name> Remote to push to (default: origin)
  --no-fetch      Skip fetching tags before calculating next version
  --allow-dirty   Allow running with uncommitted changes
  -h, --help      Show this help
EOF
}

die() {
  echo "error: $*" >&2
  exit 1
}

run_git() {
  git "$@"
}

bump=""
dry_run=0
remote="origin"
no_fetch=0
allow_dirty=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --patch|--minor|--major)
      [[ -z "$bump" ]] || die "choose exactly one of --patch, --minor, --major"
      bump="${1#--}"
      shift
      ;;
    --dry-run)
      dry_run=1
      shift
      ;;
    --remote)
      [[ $# -ge 2 ]] || die "--remote requires a value"
      remote="$2"
      shift 2
      ;;
    --no-fetch)
      no_fetch=1
      shift
      ;;
    --allow-dirty)
      allow_dirty=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

[[ -n "$bump" ]] || die "choose one bump type: --patch, --minor, or --major"

run_git rev-parse --is-inside-work-tree >/dev/null
run_git remote get-url "$remote" >/dev/null

if [[ "$allow_dirty" -eq 0 ]]; then
  if [[ -n "$(git status --porcelain)" ]]; then
    die "working tree is dirty. commit/stash changes or rerun with --allow-dirty"
  fi
fi

if [[ "$no_fetch" -eq 0 ]]; then
  run_git fetch --tags --prune "$remote"
fi

branch="$(git rev-parse --abbrev-ref HEAD)"
[[ "$branch" != "HEAD" ]] || die "detached HEAD is not supported"

latest="$(
  git tag --list 'v*' --sort=-v:refname \
    | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' \
    | head -n1 || true
)"
if [[ -z "$latest" ]]; then
  latest="v0.0.0"
fi

if [[ ! "$latest" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
  die "latest version tag is invalid: $latest"
fi

major="${BASH_REMATCH[1]}"
minor="${BASH_REMATCH[2]}"
patch="${BASH_REMATCH[3]}"

case "$bump" in
  major)
    major=$((major + 1))
    minor=0
    patch=0
    ;;
  minor)
    minor=$((minor + 1))
    patch=0
    ;;
  patch)
    patch=$((patch + 1))
    ;;
esac

next="v${major}.${minor}.${patch}"

if git rev-parse -q --verify "refs/tags/${next}" >/dev/null; then
  die "tag already exists: ${next}"
fi

echo "Current version tag: ${latest}"
echo "Next version tag:    ${next}"
echo "Branch:              ${branch}"
echo "Remote:              ${remote}"

if [[ "$dry_run" -eq 1 ]]; then
  echo
  echo "Dry run - no changes made."
  echo "Would run:"
  echo "  git tag -a ${next} -m \"release ${next}\""
  echo "  git push ${remote} ${branch}"
  echo "  git push ${remote} ${next}"
  exit 0
fi

run_git tag -a "${next}" -m "release ${next}"
run_git push "${remote}" "${branch}"
run_git push "${remote}" "${next}"

echo
echo "Release tag pushed: ${next}"
