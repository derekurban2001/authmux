# @authmux/cli

Install AuthMux with npm or pnpm:

```bash
npm i -g @authmux/cli
# or
pnpm add -g @authmux/cli
```

The package downloads the platform-specific AuthMux release binary during `postinstall`, verifies SHA256 checksums, verifies signed release metadata with Sigstore/cosign, and exposes the `authmux` command.

Configuration:

- `AUTHMUX_REPO`: override GitHub repo for releases (default: `derekurban2001/authmux`)
- `AUTHMUX_VERIFY_SIGNATURES`: set `0` to disable signature verification (default: `1`)
