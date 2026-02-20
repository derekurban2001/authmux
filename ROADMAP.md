# AuthMux Roadmap

## ✅ Phase 0 — Repository bootstrap

- Public GitHub repo
- MIT license
- README + roadmap
- CI workflow (build + test)

## ✅ Phase 1 — Core engine

- Profile registry (`state.json`)
- Tool adapters for:
  - Claude (`CLAUDE_CONFIG_DIR`)
  - Codex (`CODEX_HOME`)
- Commands:
  - add, list, use, run, status, logout, rename, remove

## ✅ Phase 2 — TUI experience

- Full-screen terminal UI
- Profile list + status panel
- Setup/management actions (add/login/launch/logout/default/remove)

## ✅ Phase 3 — Shim commands

- Generate `claude-<profile>` and `codex-<profile>` launcher commands
- Uninstall by profile or all generated shims

## ✅ Phase 4 — Safety + diagnostics

- `authmux doctor` checks:
  - tool binaries in PATH
  - profile directory integrity
  - default profile integrity

---

## Next ideas (post-v1)

- Profile export/import
- Optional encrypted metadata backups
- Better codex status parsing for future codex versions
- Remote profile sync (opt-in)
- Optional shell completion packages
