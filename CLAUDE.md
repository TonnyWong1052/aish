# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Architecture

**AISH** (AI Shell) is a Go CLI tool that captures terminal errors and provides AI-powered assistance through multiple LLM providers (Gemini, OpenAI, Claude, Ollama). The application uses a shell hook mechanism to intercept command execution and automatically analyze errors.

### Core Components

- **cmd/aish/**: Main CLI entry point with Cobra command framework
  - `main.go`: Application bootstrap and command routing
  - `init.go`: Setup wizard for configuration and shell hook installation
  - `config.go`: Configuration management commands
- **internal/llm/**: LLM provider integrations
  - `gemini-cli/`: Google Gemini integration with streaming support
  - `openai/`: OpenAI API integration
  - `anthropic/`: Anthropic Claude API integration
  - `ollama/`: Local Ollama integration for Llama models
- **internal/capture/**: Terminal output capture using pseudo-terminal (pty)
- **internal/commands/**: Command execution and processing
- **internal/classification/**: Error classification system for targeted AI responses
- **internal/config/**: Configuration system with validation
- **internal/ui/**: Interactive UI components (wizard, settings TUI)
- **internal/shell/**: Shell hook management for bash/zsh/PowerShell
- **internal/context/**: Context management for AI analysis
- **internal/errors/**: Error handling and types

### Key Architecture Patterns

- **Shell Hook Integration**: Automatically captures command errors via shell-specific hooks (DEBUG/ERR traps for bash/zsh)
- **Error Classification**: Categorizes errors (CommandNotFound, PermissionDenied, etc.) for targeted AI responses
- **Streaming AI Responses**: Real-time feedback from LLM providers
- **Security**: Automatic redaction of sensitive data (API keys, tokens) before AI analysis
- **Multi-Provider Support**: Pluggable LLM providers with consistent interface

## Development Commands

### Building
```bash
# Primary build method
make build

# Alternative build commands
go build -o aish ./cmd/aish
go build -o aish main.go
```

### Testing
```bash
# Run all tests with coverage
make test

# Run specific package tests
go test ./internal/capture/ -v
go test ./internal/llm/gemini-cli/... -v
go test ./internal/classification/ -v
go test ./internal/shell/ -v
go test ./internal/ui/ -v
```

### Linting and Formatting
```bash
# Apply formatting (gofumpt, gci, goimports)
make fmt

# Run linters (configured in .golangci.yml)
make lint

# Run go vet
make vet

# Full CI check (format, lint, vet, test)
make ci
```

### Running in Development
```bash
# Install and configure
./aish init

# Install hook and configure provider (interactive)
aish init

# Test error capture
./aish capture 127 "unknowncmd"

# Run with debug mode
AISH_DEBUG_GEMINI=1 ./aish

# Test configuration commands
./aish config show
./aish config set auto_execute true
./aish config get auto_execute
```

## Configuration

The application uses a layered configuration system:
- Config file: `~/.config/aish/config.json`
- Environment variables: `AISH_DEBUG_GEMINI`, `AISH_GEMINI_PROJECT`
- Shell hooks: Modified `.bashrc`/`.zshrc` for automatic error capture

### Supported LLM Providers

1. **OpenAI** (`openai`)
   - Endpoint: `https://api.openai.com/v1`
   - Models: `gpt-4`, `gpt-3.5-turbo`, etc.
   - Configuration: API key required

2. **Gemini** (`gemini`)
   - Endpoint: `https://generativelanguage.googleapis.com/v1`
   - Models: `gemini-pro`, etc.
   - Configuration: API key required

3. **Gemini CLI** (`gemini-cli`)
   - Endpoint: `https://cloudcode-pa.googleapis.com/v1internal:generateContent`
   - Models: `gemini-2.5-flash`, `gemini-2.5-pro`
   - Configuration: Google Cloud project ID + OAuth authentication

4. **Claude** (`claude`) - *New*
   - Endpoint: `https://api.anthropic.com/v1`
   - Models: `claude-3-5-sonnet-20241022`, `claude-3-5-haiku-20241022`, `claude-3-opus-20240229`
   - Configuration: Anthropic API key required
   - Setup: `aish config set providers.claude.api_key YOUR_API_KEY`

5. **Ollama** (`ollama`) - *New*
   - Endpoint: `http://localhost:11434` (local)
   - Models: `llama3.3`, `llama3.1`, `codellama`, etc.
   - Configuration: No API key needed (local models)
   - Setup: Install Ollama and pull models (`ollama pull llama3.3`)

### Switching Providers

```bash
# Set default provider
aish config set default_provider claude

# Or use environment variable
export AISH_DEFAULT_PROVIDER=ollama
```

## Testing Strategy

- **Unit Tests**: Located alongside source files (`*_test.go`)
- **Coverage Target**: Minimum 60% (enforced in CI)
- **Race Detection**: All tests run with `-race` flag
- **CI Pipeline**: Automated testing on Linux, macOS, Windows

## Key Implementation Details

### Shell Hook Mechanism
The shell hook (`internal/shell/hook.go`) uses DEBUG and ERR traps to capture command output. It:
1. Captures both stdout and stderr to temporary files
2. Detects non-zero exit codes
3. Filters out user interruptions (Ctrl+C) and AISH's own commands
4. Triggers AI analysis automatically

### Error Classification
The classifier (`internal/classification/classifier.go`) matches error patterns to categories, enabling:
- Targeted AI prompts for specific error types
- Better context for AI models
- Consistent handling of common errors

### Security Features
- Automatic redaction of sensitive parameters (`--api-key`, `--token`, `--password`)
- Environment variable protection (variables containing `SECRET`, `TOKEN`, etc.)
- Secure storage in `~/.config/aish/` with proper permissions

### Genkit Integration

**Claude** and **Ollama** providers use **[Genkit Go](https://firebase.google.com/docs/genkit/go/get-started-go)** (v1.0.5) for unified LLM interaction:

#### Architecture
- **Genkit Adapter Layer** (`internal/llm/genkit_adapter.go`): Bridges Genkit with the existing `llm.Provider` interface
- **Plugin-based Design**: Each provider uses Genkit's plugin system for LLM access
- **Backward Compatibility**: All existing Provider interface methods are preserved

#### Provider Implementation

**Claude Provider** (`internal/llm/anthropic/client.go`):
```go
import anthropicPlugin "github.com/firebase/genkit/go/plugins/compat_oai/anthropic"

g := genkit.Init(ctx,
    genkit.WithPlugins(&anthropicPlugin.Anthropic{
        Opts: []option.RequestOption{
            option.WithAPIKey(cfg.APIKey),
        },
    }),
)
modelName := "anthropic/" + cfg.Model
adapter := llm.NewGenkitAdapter(g, modelName)
```

**Ollama Provider** (`internal/llm/ollama/client.go`):
```go
import ollamaPlugin "github.com/firebase/genkit/go/plugins/ollama"

g := genkit.Init(ctx,
    genkit.WithPlugins(&ollamaPlugin.Ollama{
        ServerAddress: cfg.APIEndpoint, // http://localhost:11434
    }),
)
modelName := "ollama/" + cfg.Model
adapter := llm.NewGenkitAdapter(g, modelName)
```

#### Key Features
- **Unified API**: Both providers use the same `GenkitAdapter` methods
- **Model Name Prefix**: Genkit requires provider prefix (e.g., `"anthropic/claude-3-5-sonnet-20241022"`)
- **Structured Output**: Support for `GenerateData[T]()` for type-safe structured responses
- **Error Handling**: Consistent error wrapping across providers

#### Why Genkit?
- **Simplified Integration**: Reduces boilerplate for LLM API calls
- **Plugin Ecosystem**: Easy to add new providers
- **Telemetry & Tracing**: Built-in observability support (optional)
- **Type Safety**: Strong typing for prompts and responses

#### Testing Genkit Providers
```bash
# Compile with Genkit integration
go build -o aish ./cmd/aish

# Test Claude (requires API key)
aish config set default_provider claude
aish config set providers.claude.api_key YOUR_API_KEY
aish -p "list files"

# Test Ollama (requires Ollama running locally)
ollama pull llama3.3
aish config set default_provider ollama
aish -p "list files"
```

For more details, see [GENKIT.md](./GENKIT.md).

## Release and Distribution

The project uses GoReleaser for automated releases:
- **Trigger**: Git tags matching `v*` pattern (e.g., `v0.0.1`)
- **Artifacts**: Cross-platform binaries (Linux, macOS, Windows)
- **Packages**: `.deb` packages for Debian/Ubuntu distributions
- **APT Repository**: Automatically updated at https://tonnywong1052.github.io/aish-apt-repo
- **Homebrew Tap**: Available via `brew tap TonnyWong1052/aish`

### Creating a Release
```bash
# Tag the release
git tag v0.0.2
git push origin v0.0.2

# GitHub Actions will automatically:
# 1. Build binaries for all platforms
# 2. Create GitHub release with artifacts
# 3. Update APT repository
# 4. Generate checksums and sign packages
```

## Prompt Engineering

The application uses sophisticated prompt templates located in:
- `internal/prompts`: Template file containing AI prompts for different error categories
- `internal/prompt/manager.go`: Prompt template manager with context injection

Key prompt categories:
- **Error Analysis**: Context-aware prompts based on error classification
- **Command Generation**: Natural language to shell command translation
- **General Q&A**: Plain-text responses without command suggestions

## History Management

Error analysis history is stored in `~/.config/aish/history.json`:
```bash
# View history
aish history

# Re-analyze a specific error
aish history --replay <id>

# Clear all history
aish history --clear
```

## Important Implementation Notes

### Go Version
- **Required**: Go 1.24.0 or higher
- Uses latest Go features for better performance and type safety

### Dependencies
- **Cobra**: CLI framework for command routing and flags
- **pterm**: Terminal UI components with color support
- **Bubble Tea**: Interactive TUI for settings and wizards
- **logrus**: Structured logging

### Environment Variables
- `AISH_DEBUG_GEMINI`: Enable debug logging for Gemini provider
- `AISH_GEMINI_PROJECT`: Override default Gemini project ID
- `AISH_STDOUT_FILE`, `AISH_STDERR_FILE`: Custom paths for captured output
- `AISH_SKIP_COMMAND_PATTERNS`: Skip hook for specific command patterns
- `AISH_SKIP_ALL_USER_COMMANDS`: Skip all user-installed commands
- `AISH_CAPTURE_OFF`: Temporarily disable hook for one command
