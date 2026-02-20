# Security Policy

## Supported versions

AuthMux is pre-1.0. Please use the latest `main` release/commit.

## Reporting a vulnerability

Please do **not** open public issues for security vulnerabilities.

Instead:

1. Open a private security advisory on GitHub (preferred), or
2. Contact the maintainer directly through GitHub.

Include:

- Affected version/commit
- Reproduction steps
- Potential impact
- Suggested mitigation (if available)

## Security notes

- AuthMux stores profile metadata under `~/.authmux` (or `AUTHMUX_HOME`).
- Auth tokens remain managed by Claude/Codex in their own storage contexts.
- Treat per-profile auth directories as sensitive system state.
