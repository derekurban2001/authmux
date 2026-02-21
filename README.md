# AuthMux

A profile-based authentication manager for **Claude Code** and **OpenAI Codex CLI**.

AuthMux makes multi-account usage simple:

- Keep multiple auth contexts isolated per tool/profile
- Launch each tool with the right auth context instantly
- Set defaults per tool
- Generate shortcut commands like `claude-work` and `codex-personal`
- Use a terminal UI (`authmux`) for setup and day-to-day control

---

## Why

Neither Claude Code nor Codex currently provides a first-class UX for managing many named auth contexts across multiple accounts/subscriptions.

AuthMux solves this by wrapping each tool's native auth storage directory:

- Claude Code → `CLAUDE_CONFIG_DIR`
- Codex CLI → `CODEX_HOME`

This enables truly isolated, parallel sessions with different accounts.

---

## Install

### One-command install (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/derekurban2001/authmux/main/install.sh | bash
```

For Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/derekurban2001/authmux/main/install.ps1 | iex
```

Optional env vars:

- `AUTHMUX_INSTALL_DIR` (default: `~/.local/bin`)
- `AUTHMUX_VERSION` (`latest` by default, or tag like `v0.1.0`)
- `AUTHMUX_AUTO_PATH` (`1` by default; set `0` to disable PATH updates)
- `AUTHMUX_VERIFY_SIGNATURES` (`1` by default; set `0` to disable cosign verification)
- `AUTHMUX_ALLOW_SOURCE_FALLBACK` (`0` by default; set `1` to allow `go install` fallback)
- `AUTHMUX_AUTO_INSTALL_GO` (`1` by default; only used when source fallback is enabled)
- `AUTHMUX_COSIGN_VERSION` (default: `v2.5.3`)
- `AUTHMUX_COSIGN_IDENTITY_RE` (advanced: certificate identity regex for cosign verification)
- `AUTHMUX_COSIGN_OIDC_ISSUER` (advanced: OIDC issuer for cosign verification)

Both installers automatically add the install directory to your PATH (current session + persistent user config) by default.
If no tagged release exists yet, use `AUTHMUX_ALLOW_SOURCE_FALLBACK=1`.

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

### Package manager channels

Release workflow generates publish-ready manifests/packages for:

- Homebrew
- Winget
- Scoop
- Chocolatey

Release workflow can also publish npm package `@authmux/cli`:

```bash
npm i -g @authmux/cli
# or
pnpm add -g @authmux/cli
```

### From source

```bash
git clone https://github.com/derekurban2001/authmux.git
cd authmux
go build -o authmux .
# optional
mv authmux ~/.local/bin/
```

---

## Quick start

```bash
# add profile + login flow
authmux add claude personal
authmux add codex work

# set defaults
authmux use claude personal
authmux use codex work

# list and status
authmux list
authmux status

# run using default profile
authmux run claude -- --model sonnet
authmux run codex -- --profile deep-review

# run with explicit profile
authmux run claude personal -- --print "hello"
```

Run `authmux` with no args to open the TUI.

---

## Commands

### Core

- `authmux` – open TUI
- `authmux add <tool> <profile>` (also installs/updates that profile shim)
- `authmux list [--tool claude|codex] [--json]`
- `authmux use <tool> <profile>`
- `authmux run <tool> [profile] -- [tool args...]`
- `authmux status [tool] [profile] [--json]`
- `authmux logout <tool> <profile>`
- `authmux rename <tool> <old-profile> <new-profile>`
- `authmux remove <tool> <profile> [--purge]`

### Shims

- `authmux shim install [--dir <path>]`
- `authmux shim uninstall --all [--dir <path>]`
- `authmux shim uninstall <tool> <profile> [--dir <path>]`

### Health

- `authmux doctor [--json]`

Detailed command docs: [`docs/COMMANDS.md`](docs/COMMANDS.md)

---

## TUI controls

- `↑/↓` or `j/k` – move selection
- `Enter` – launch selected profile
- `a` – add profile
- `l` – login selected profile
- `o` – logout selected profile
- `u` – set selected as default
- `d` – remove selected profile (registry only)
- `s` – install shims
- `r` – refresh statuses
- `q` – quit

---

## Storage

Default root:

- `~/.authmux` (or `AUTHMUX_HOME` override)

Important files/dirs:

- `state.json` – profile/default registry
- `profiles/claude/<profile>/...`
- `profiles/codex/<profile>/...`

---

## Testing

```bash
go test ./...
go vet ./...
```

The test suite covers:

- store persistence and validation
- profile lifecycle management (create/default/rename/remove)
- adapter command/env wiring
- shim generation/removal behavior
- command contract expectations (including no shorthand aliases)

---

## Documentation

- [`docs/INSTALL.md`](docs/INSTALL.md)
- [`docs/DISTRIBUTION.md`](docs/DISTRIBUTION.md)
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)
- [`docs/COMMANDS.md`](docs/COMMANDS.md)
- [`CONTRIBUTING.md`](CONTRIBUTING.md)
- [`SECURITY.md`](SECURITY.md)
- [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md)

---

## Notes

- AuthMux stores profile metadata, not your secret tokens directly.
- Tokens remain managed by each tool inside its own profile directory / keychain path.
- You can run concurrent sessions with different profiles by launching each with different profile contexts.

---

## License

MIT
