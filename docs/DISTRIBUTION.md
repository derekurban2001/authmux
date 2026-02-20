# Distribution and Release Channels

AuthMux releases are built from a Git tag (`vX.Y.Z`) and published through GitHub Actions.

## What release workflow does

The release workflow (`.github/workflows/release.yml`) performs:

1. Build multi-platform binaries with GoReleaser.
2. Publish release archives and `checksums.txt`.
3. Sign `checksums.txt` with Sigstore/cosign (keyless OIDC).
4. Upload `checksums.txt.sig` and `checksums.txt.pem` to the release.
5. Generate package-manager manifests:
   - Homebrew formula
   - Winget manifests
   - Scoop manifest
   - Chocolatey package files
6. Publish npm package `@authmux/cli` when `NPM_TOKEN` is configured.

Generated manifests are attached to each release under `package-manifests/`.

## Required secrets

- `GITHUB_TOKEN` (provided by GitHub Actions)
- `NPM_TOKEN` (optional, required to publish `@authmux/cli`)
- `HOMEBREW_TAP_TOKEN` (optional, push formula updates to tap repo)
- `SCOOP_BUCKET_TOKEN` (optional, push scoop manifest updates)
- `WINGET_TOKEN` (optional, open winget PRs from automation)
- `CHOCOLATEY_API_KEY` (optional, publish package to Chocolatey)

## Optional repository variables

- `HOMEBREW_TAP_REPO` (for example `derekurban2001/homebrew-tap`)
- `SCOOP_BUCKET_REPO` (for example `derekurban2001/scoop-bucket`)
- `WINGET_REPO` (default: `microsoft/winget-pkgs`)
- `AUTHMUX_WINGET_PACKAGE_ID` (default: `DerekUrban.AuthMux`)

Chocolatey publish runs in a dedicated Windows job when `CHOCOLATEY_API_KEY` is set.

## Installer trust model

Installers verify:

1. `checksums.txt` signature via cosign (`checksums.txt.sig` + `checksums.txt.pem`).
2. Downloaded archive hash against `checksums.txt`.

Defaults:

- Signature verification enabled (`AUTHMUX_VERIFY_SIGNATURES=1`)
- Source fallback disabled (`AUTHMUX_ALLOW_SOURCE_FALLBACK=0`)
