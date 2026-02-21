# AuthMux Architecture

AuthMux has four core layers:

1. **CLI layer (`cmd/authmux`)**
   - Parses commands/flags and dispatches to app logic.
2. **App layer (`internal/app`)**
   - Implements profile lifecycle and orchestration.
3. **Adapter layer (`internal/adapters`)**
   - Encapsulates tool-specific behavior.
   - Claude: `CLAUDE_CONFIG_DIR`
   - Codex: `CODEX_HOME`
4. **Persistence layer (`internal/store`)**
   - Reads/writes `state.json`.

## Data model

`state.json`:

- version
- defaults map (`tool -> profile`)
- profile list (`tool`, `name`, `dir`, `created_at`)

## Runtime model

- `authmux run ...` resolves tool + profile context.
- Adapter injects environment variable for that profile directory.
- Tool is launched normally (`claude` or `codex`) with isolated auth context.

## TUI model

The TUI (`internal/tui`) wraps app actions:

- profile selection
- login/logout
- launch
- default selection
- profile add/remove
- shim installation

## Shims

`internal/shim` generates launcher scripts:

- Unix/macOS/Linux: `claude-<profile>`, `codex-<profile>`
- Windows: `claude-<profile>.cmd`, `codex-<profile>.cmd`

Each shim executes:

```bash
authmux run <tool> <profile> -- "$@"
```
