# Installation

## One-command install

```bash
curl -fsSL https://raw.githubusercontent.com/derekurban/proflex-cli/main/install.sh | bash
```

For Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/derekurban/proflex-cli/main/install.ps1 | iex
```

This installer will:

1. Try to download a matching binary from GitHub Releases
2. Verify SHA256 checksums
3. Verify signed release metadata (`checksums.txt.sig` + `checksums.txt.pem`) with Sigstore/cosign
4. Install to user-local bin and update PATH (unless disabled)

## Installer options

Environment variables:

- `PROFLEX_INSTALL_DIR`: install destination (default `~/.local/bin`)
- `PROFLEX_VERSION`: `latest` (default) or specific tag (e.g. `v0.1.0`)
- `PROFLEX_AUTO_PATH`: `1` (default) to auto-add install dir to PATH, `0` to disable
- `PROFLEX_VERIFY_SIGNATURES`: `1` (default) enforce cosign verification, `0` to disable
- `PROFLEX_ALLOW_SOURCE_FALLBACK`: `0` (default) disable fallback to `go install`; set to `1` to enable
- `PROFLEX_AUTO_INSTALL_GO`: `1` (default) auto-install Go if source fallback is enabled and Go is missing
- `PROFLEX_COSIGN_VERSION`: cosign version used if cosign is not on PATH (default `v2.5.3`)
- `PROFLEX_COSIGN_IDENTITY_RE`: certificate identity regex for `cosign verify-blob`
- `PROFLEX_COSIGN_OIDC_ISSUER`: OIDC issuer for `cosign verify-blob`

## npm / pnpm global install

```bash
npm i -g proflex-cli
# or
pnpm add -g proflex-cli
```

## From source

```bash
git clone https://github.com/derekurban/proflex-cli.git
cd proflex-cli
go build -o proflex .
mv proflex ~/.local/bin/
```

## Verify install

```bash
proflex --help
```

## Uninstall

```bash
proflex uninstall
```

Optional cleanup:

- `proflex uninstall --purge` also removes local Proflex state (`~/.proflex`).
