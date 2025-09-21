# Copilot Instructions for AI Agents

This project is **AISH** (AI Shell), a Go-based CLI tool for intelligent terminal assistance, error analysis, and LLM integration. Follow these guidelines to be productive and maintain project conventions.

## Architecture & Data Flow
- **Event-Driven Core**: The main logic is triggered by a shell hook (`internal/shell/hook.go`) that runs after command failures, invoking `aish capture [exit_code] [command]`.
- **Error Capture & Analysis**: Command output is saved to `~/.config/aish/last_stdout` and related files, then analyzed by the `capture` command.
- **Pluggable LLM Providers**: Add new providers by creating a package under `internal/llm/` and registering via blank import in `cmd/aish/main.go`. See `internal/llm/types.go` for the provider interface.
- **Prompt Management**: All LLM prompts are loaded from `prompts.json` at runtime. Do not hardcode prompts; update the JSON file to change AI behavior.
- **Sensitive Data Handling**: The shell hook uses `sed` to redact secrets (API keys, etc.) before logging or analysis.

## Developer Workflows
- **Build**: `go build -o bin/aish ./cmd/aish`
- **Test**: `go test ./...` or for a package, e.g., `go test ./internal/shell/ -v`
- **Install/Setup**: Use `./scripts/install.sh` (optionally with `--with-init`), then run `aish init` to install hooks and configure providers.
- **Debugging**: Check `~/.config/aish/` for captured output and config. Use `AISH_DEBUG_GEMINI=1` for verbose Gemini logs.

## Coding & Documentation Conventions
- **Comments**: In-code comments must be in English. PRs/docs may use Chinese.
- **Import Order**: Use `gci` order: standard, default, then internal (`prefix(powerful-cli)`).
- **Linters**: Only formatting linters (`gofmt`, `gofumpt`, `goimports`, `gci`) are enforced. No deep static analysis.
- **State**: All persistent and temp state is in `~/.config/aish/`.

## Integration & Extensibility
- **LLM Providers**: Register via `init()` and blank import. See `internal/llm/types.go` and `cmd/aish/main.go`.
- **Gemini CLI Auth**: Credentials are read from `~/.gemini/access_token` or `~/.gemini/oauth_creds.json`.
- **Token Refresher**: See `tools/gemini/token-refresher/README.md` for OAuth token management.

## Key Files & Directories
- `cmd/aish/` — CLI entry point and command routing
- `internal/shell/hook.go` — Shell hook logic
- `internal/llm/` — LLM provider implementations
- `internal/context/`, `internal/history/` — State and history management
- `prompts.json` — LLM prompt definitions
- `scripts/install.sh` — Install and setup script

## Examples
- To add a new LLM provider: create `internal/llm/myprovider/`, implement the interface, and add a blank import in `cmd/aish/main.go`.
- To change prompt behavior: edit `prompts.json` (do not hardcode in Go files).

---

For more details, see `AGENTS.md`, `README.md`, and `CLAUDE.md`.
