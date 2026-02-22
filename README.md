# Proflex

A profile manager for **Claude Code** and **OpenAI Codex CLI**.

Proflex gives each tool its own isolated config directory per profile, and generates shims like `claude-work` and `codex-personal` so you can switch between accounts instantly.

---

## Why

Neither Claude Code nor Codex provides built-in multi-account support. Proflex solves this by redirecting each tool's native config directory:

- Claude Code → `CLAUDE_CONFIG_DIR`
- Codex CLI → `CODEX_HOME`

Each profile gets its own isolated directory. Auth happens naturally through the tool's normal flow on first run.

---

## Install

### One-command install (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/derekurban2001/proflex/main/install.sh | bash
```

For Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/derekurban2001/proflex/main/install.ps1 | iex
```

### npm

```bash
npm i -g @proflex/cli
```

### From source

```bash
go install github.com/derekurban2001/proflex@latest
```

### Installer options

Environment variables:

- `PROFLEX_INSTALL_DIR` (default: `~/.local/bin`)
- `PROFLEX_VERSION` (`latest` by default, or tag like `v0.1.0`)
- `PROFLEX_AUTO_PATH` (`1` by default; set `0` to disable PATH updates)
- `PROFLEX_VERIFY_SIGNATURES` (`1` by default; set `0` to disable cosign verification)
- `PROFLEX_ALLOW_SOURCE_FALLBACK` (`0` by default; set `1` to allow `go install` fallback)

---

## Quick start

```bash
# Create profiles
proflex add claude personal
proflex add claude work
proflex add codex main

# Set defaults
proflex use claude work

# List profiles with auth status
proflex list

# Use the shims directly
claude-personal
claude-work
codex-main
```

After creating a profile, just run the shim (e.g. `claude-work`). You'll be prompted to authenticate on first use.

---

## Commands

- `proflex add <tool> <profile>` — Create profile + install shim
- `proflex remove <tool> <profile> [--purge]` — Remove profile + shim
- `proflex list [--tool claude|codex] [--json]` — List profiles with status
- `proflex use <tool> <profile>` — Set default profile
- `proflex rename <tool> <old> <new>` — Rename a profile
- `proflex run <tool> [profile] -- [args...]` — Run tool with profile context
- `proflex shim install [--dir <path>]` — Reinstall all shims
- `proflex shim uninstall [--all] [<tool> <profile>]` — Remove shims

---

## Storage

Default root: `~/.proflex` (or `PROFLEX_HOME` override)

```
~/.proflex/
├── state.json
└── profiles/
    ├── claude/
    │   ├── personal/
    │   └── work/
    └── codex/
        └── main/
```

---

## Testing

```bash
go test ./...
go vet ./...
```

---

## License

MIT
