# profilex-cli

Install ProfileX with npm or pnpm:

```bash
npm i -g profilex-cli
# or
pnpm add -g profilex-cli
```

The package downloads the platform-specific ProfileX release binary during `postinstall`, verifies SHA256 checksums, verifies signed release metadata with Sigstore/cosign, and exposes the `profilex` command.

Configuration:

- `PROFILEX_REPO`: override GitHub repo for releases (default: `derekurban/profilex-cli`)
- `PROFILEX_VERIFY_SIGNATURES`: set `0` to disable signature verification (default: `1`)
