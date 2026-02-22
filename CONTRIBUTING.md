# Contributing to Proflex

Thanks for your interest in contributing.

## Development setup

```bash
git clone https://github.com/derekurban/proflex-cli.git
cd proflex-cli
go mod tidy
go test ./...
```

## Build locally

```bash
go build -o proflex .
./proflex --help
```

## Run checks before opening a PR

```bash
gofmt -w $(find . -name '*.go')
go vet ./...
go test ./...
```

## Project structure

- `internal/cli`: CLI command dispatch and output
- `internal/store`: state/profile persistence
- `internal/adapters`: tool-specific integrations (Claude/Codex)
- `internal/app`: core profile management logic
- `internal/shim`: generated launcher script logic

## Contribution guidelines

- Keep commands explicit and readable (no shorthand command aliases).
- Avoid introducing dependencies that require external services for tests.
- Add tests for behavior changes.
- Keep docs updated when command behavior changes.

## Pull request checklist

- [ ] Tests added/updated
- [ ] `go test ./...` passes
- [ ] `go vet ./...` passes
- [ ] README/docs updated (if user-facing behavior changed)
