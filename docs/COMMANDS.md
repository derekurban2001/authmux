# Command Reference

## `authmux`

Open the terminal UI.

## `authmux add <tool> <profile>`

Create profile registry entry and launch tool-native login flow.

## `authmux list [--tool claude|codex] [--json]`

List profiles with status and default marker.

## `authmux use <tool> <profile>`

Set default profile for a tool.

## `authmux run <tool> [profile] -- [tool args...]`

Run a tool in selected/default profile context.

Examples:

```bash
authmux run claude personal -- --model sonnet
authmux run codex -- --profile deep-review
```

## `authmux status [tool] [profile] [--json]`

Show statuses globally, for one tool, or one profile.

## `authmux logout <tool> <profile>`

Run tool-native logout in one profile context.

## `authmux rename <tool> <old-profile> <new-profile>`

Rename profile and move profile directory.

## `authmux remove <tool> <profile> [--purge]`

Remove profile from registry.

`--purge` also deletes profile directory.

## `authmux shim install [--dir <path>]`

Generate launcher shims for all profiles.

## `authmux shim uninstall --all [--dir <path>]`

Remove all AuthMux-generated shims in a directory.

## `authmux shim uninstall <tool> <profile> [--dir <path>]`

Remove one specific generated shim.

## `authmux doctor [--json]`

Run health checks:

- binary detection (`claude`, `codex`)
- profile directory existence
- default profile integrity
