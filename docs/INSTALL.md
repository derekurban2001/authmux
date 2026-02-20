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
2. Verify checksums when available
3. Fall back to `go install` if a binary release isn't found

## Installer options

Environment variables:

- `AUTHMUX_INSTALL_DIR`: install destination (default `~/.local/bin`)
- `AUTHMUX_VERSION`: `latest` (default) or specific tag (e.g. `v0.1.0`)
- `AUTHMUX_AUTO_PATH`: `1` (default) to auto-add install dir to PATH, `0` to disable

By default, installers update PATH for the current session and persist the change for future shells.

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
