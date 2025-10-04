# Repository Guidelines

## Project Structure & Module Organization
- `cmd/aish`: Cobra entrypoint and subcommands; `main.go` orchestrates init, history, and hooks.
- `internal/`: modules by responsibility — `shell/` (hook install/run), `llm/` (providers), `config/` (load/validate), `history/`, `ui/`. Tests live alongside sources.
- `scripts/`: install and packaging helpers; `web-bundles/` front‑end asset snapshots; `bin/` local build outputs.
- User config lives in `~/.config/aish/` (`config.json`, `history.json`, `logs/aish.log`).

## Build, Test, and Development Commands
- `go build -o bin/aish ./cmd/aish` — build the CLI binary (release‑equivalent output).
- `go test ./... -cover` — run all unit tests with coverage.
- `go test ./internal/shell -v` — focus tests for shell hook behavior (incl. cross‑shell scenarios).
- `./scripts/install.sh --with-init` — simulate user installation and verify defaults + hooks.

## Coding Style & Naming Conventions
- Language: Go. Tabs for indentation; format with `gofmt`, `gofumpt`, `goimports`, and `gci`.
- Packages: short, lowercase; imports ordered per project convention.
- Errors: exported errors prefixed with `Err` and kept centrally.
- Comments: English full sentences; flags/fields use `camelCase`.

## Testing Guidelines
- Use standard `testing` with table‑driven cases; name tests `TestFeature_Scenario`; use `t.Parallel()` when safe.
- When changing hooks or config, add tests for `~/.config/aish/last_stdout` trimming/cleanup and hook install/uninstall transitions.
- Recommended before significant changes: `go test ./... -race`.

## Commit & Pull Request Guidelines
- Commits: English imperative mood, one topic per commit (e.g., `Refine prompt caching`); branches `feature/<topic>` and `fix/<ticket>`.
- PRs: include purpose, linked issues, and test results; add screenshots/videos when helpful; call out changes to hooks, config schema, or external deps.

## Security & Configuration Tips (Optional)
- Provide API keys/tokens via env vars or `~/.config/aish/config.json`; never commit secrets.
- Logs default to `~/.config/aish/logs/aish.log`; scrub sensitive data before sharing.
- Extending LLM providers: follow `internal/llm` factory registration and provide a fallback path.

