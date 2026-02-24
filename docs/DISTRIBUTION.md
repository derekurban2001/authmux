# Distribution and Release

ProfileX releases are built from a Git tag (`vX.Y.Z`) and published through GitHub Actions.

## What the release workflow does

The release workflow (`.github/workflows/release.yml`) performs:

1. Build multi-platform binaries with GoReleaser.
2. Publish release archives and `checksums.txt`.
3. Sign `checksums.txt` with Sigstore/cosign (keyless OIDC).
4. Upload `checksums.txt.sig` and `checksums.txt.pem` to the release.
5. Publish npm package `profilex-cli` when `NPM_TOKEN` is configured.

## Required secrets

- `GITHUB_TOKEN` (provided by GitHub Actions)
- `NPM_TOKEN` (optional, required to publish `profilex-cli`)

## Bump, tag, and push helper

You can create and push the next release tag with helper scripts:

- PowerShell: `scripts/release/bump-tag.ps1`
- Bash: `scripts/release/bump-tag.sh`

Examples:

```powershell
./scripts/release/bump-tag.ps1 --patch
./scripts/release/bump-tag.ps1 --minor --dry-run
./scripts/release/bump-tag.ps1 --major --remote origin
```

```bash
./scripts/release/bump-tag.sh --patch
./scripts/release/bump-tag.sh --minor --dry-run
./scripts/release/bump-tag.sh --major --remote origin
```

Behavior:

1. Optionally fetches tags from remote.
2. Calculates the next semver tag from latest `vX.Y.Z`.
3. Creates an annotated tag (`release vX.Y.Z`).
4. Pushes current branch and the new tag.

By default it requires a clean working tree. Use `--allow-dirty` to override.

## Installer trust model

Installers verify:

1. `checksums.txt` signature via cosign (`checksums.txt.sig` + `checksums.txt.pem`).
2. Downloaded archive hash against `checksums.txt`.

Defaults:

- Signature verification enabled (`PROFILEX_VERIFY_SIGNATURES=1`)
- Source fallback disabled (`PROFILEX_ALLOW_SOURCE_FALLBACK=0`)
