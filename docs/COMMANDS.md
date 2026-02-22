# Command Reference

## `proflex add <tool> <profile>`

Create profile, install shim. On first use via the shim, the tool's native auth flow runs.

## `proflex list [--tool claude|codex] [--json]`

List profiles with status and default marker.

## `proflex use <tool> <profile>`

Set default profile for a tool.

## `proflex run <tool> [profile] -- [tool args...]`

Run a tool in selected/default profile context.

Examples:

```bash
proflex run claude personal -- --model sonnet
proflex run codex -- --profile deep-review
```

## `proflex rename <tool> <old-profile> <new-profile>`

Rename profile and move profile directory.

## `proflex remove <tool> <profile> [--purge]`

Remove profile from registry.

`--purge` also deletes profile directory.

## `proflex shim install [--dir <path>]`

Generate launcher shims for all profiles.

## `proflex shim uninstall --all [--dir <path>]`

Remove all Proflex-generated shims in a directory.

## `proflex shim uninstall <tool> <profile> [--dir <path>]`

Remove one specific generated shim.
