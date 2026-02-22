# @derekurban2001/proflex-cli

Install Proflex with npm or pnpm:

```bash
npm i -g @derekurban2001/proflex-cli
# or
pnpm add -g @derekurban2001/proflex-cli
```

The package downloads the platform-specific Proflex release binary during `postinstall`, verifies SHA256 checksums, verifies signed release metadata with Sigstore/cosign, and exposes the `proflex` command.

Configuration:

- `PROFLEX_REPO`: override GitHub repo for releases (default: `derekurban2001/proflex-cli`)
- `PROFLEX_VERIFY_SIGNATURES`: set `0` to disable signature verification (default: `1`)
