# Installation

## One-command install

```bash
curl -fsSL https://raw.githubusercontent.com/derekurban2001/authmux/main/install.sh | bash
```

For Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/derekurban2001/authmux/main/install.ps1 | iex
```

This installer will:

1. Try to download a matching binary from GitHub Releases
2. Verify SHA256 checksums
3. Verify signed release metadata (`checksums.txt.sig` + `checksums.txt.pem`) with Sigstore/cosign
4. Install to user-local bin and update PATH (unless disabled)

## Installer options

Environment variables:

- `AUTHMUX_INSTALL_DIR`: install destination (default `~/.local/bin`)
- `AUTHMUX_VERSION`: `latest` (default) or specific tag (e.g. `v0.1.0`)
- `AUTHMUX_AUTO_PATH`: `1` (default) to auto-add install dir to PATH, `0` to disable
- `AUTHMUX_VERIFY_SIGNATURES`: `1` (default) enforce cosign verification, `0` to disable
- `AUTHMUX_ALLOW_SOURCE_FALLBACK`: `0` (default) disable fallback to `go install`; set to `1` to enable
- `AUTHMUX_AUTO_INSTALL_GO`: `1` (default) auto-install Go if source fallback is enabled and Go is missing
- `AUTHMUX_COSIGN_VERSION`: cosign version used if cosign is not on PATH (default `v2.5.3`)
- `AUTHMUX_COSIGN_IDENTITY_RE`: certificate identity regex for `cosign verify-blob`
- `AUTHMUX_COSIGN_OIDC_ISSUER`: OIDC issuer for `cosign verify-blob`

By default, installers update PATH for the current session and persist the change for future shells.
If no tagged release exists yet, set `AUTHMUX_ALLOW_SOURCE_FALLBACK=1`.

Example:

```bash
AUTHMUX_INSTALL_DIR="$HOME/bin" AUTHMUX_VERSION="latest" \
  curl -fsSL https://raw.githubusercontent.com/derekurban2001/authmux/main/install.sh | bash
```

PowerShell example:

```powershell
$env:AUTHMUX_INSTALL_DIR = "$HOME\.local\bin"
$env:AUTHMUX_VERSION = "latest"
irm https://raw.githubusercontent.com/derekurban2001/authmux/main/install.ps1 | iex
```

Source fallback example (opt-in):

```bash
AUTHMUX_ALLOW_SOURCE_FALLBACK=1 curl -fsSL https://raw.githubusercontent.com/derekurban2001/authmux/main/install.sh | bash
```

## Package manager manifests

Release workflow generates publish-ready manifests and packages for:

- Homebrew
- Winget
- Scoop
- Chocolatey

Generated files are attached to each release under `package-manifests/`.

## npm / pnpm global install

The release workflow also publishes `@authmux/cli` (when `NPM_TOKEN` is configured):

```bash
npm i -g @authmux/cli
# or
pnpm add -g @authmux/cli
```

## From source

```bash
git clone https://github.com/derekurban2001/authmux.git
cd authmux
go build -o authmux .
mv authmux ~/.local/bin/
```

## Verify install

```bash
authmux --help
authmux doctor
```
