# Installation

## One-command install

```bash
curl -fsSL https://raw.githubusercontent.com/derekurban2001/authmux/main/install.sh | bash
```

This installer will:

1. Try to download a matching binary from GitHub Releases
2. Verify checksums when available
3. Fall back to `go install` if a binary release isn't found

## Installer options

Environment variables:

- `AUTHMUX_INSTALL_DIR`: install destination (default `~/.local/bin`)
- `AUTHMUX_VERSION`: `latest` (default) or specific tag (e.g. `v0.1.0`)

Example:

```bash
AUTHMUX_INSTALL_DIR="$HOME/bin" AUTHMUX_VERSION="latest" \
  curl -fsSL https://raw.githubusercontent.com/derekurban2001/authmux/main/install.sh | bash
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
