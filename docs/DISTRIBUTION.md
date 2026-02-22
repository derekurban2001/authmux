# Distribution and Release

Proflex releases are built from a Git tag (`vX.Y.Z`) and published through GitHub Actions.

## What the release workflow does

The release workflow (`.github/workflows/release.yml`) performs:

1. Build multi-platform binaries with GoReleaser.
2. Publish release archives and `checksums.txt`.
3. Sign `checksums.txt` with Sigstore/cosign (keyless OIDC).
4. Upload `checksums.txt.sig` and `checksums.txt.pem` to the release.
5. Publish npm package `@derekurban2001/proflex-cli` when `NPM_TOKEN` is configured.

## Required secrets

- `GITHUB_TOKEN` (provided by GitHub Actions)
- `NPM_TOKEN` (optional, required to publish `@derekurban2001/proflex-cli`)

## Installer trust model

Installers verify:

1. `checksums.txt` signature via cosign (`checksums.txt.sig` + `checksums.txt.pem`).
2. Downloaded archive hash against `checksums.txt`.

Defaults:

- Signature verification enabled (`PROFLEX_VERIFY_SIGNATURES=1`)
- Source fallback disabled (`PROFLEX_ALLOW_SOURCE_FALLBACK=0`)
