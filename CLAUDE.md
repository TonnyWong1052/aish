# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**AISH** (AI Shell) is a Go CLI tool that captures terminal errors and provides AI-powered assistance through multiple LLM providers (Gemini, OpenAI, Claude, Ollama). The application uses a shell hook mechanism to intercept command execution and automatically analyze errors.

**Tech Stack**: Go 1.24.1, Cobra CLI framework, Firebase Genkit, OpenAI SDK, Charm TUI libraries

## Architecture

### Core Components

- **cmd/aish/**: Main CLI entry point with Cobra command framework
  - `main.go`: Application bootstrap and command routing
  - `init.go`: Setup wizard for configuration and shell hook installation
  - `config.go`: Configuration management commands
  - `doctor.go`: Diagnostic tool for troubleshooting setup issues
  - `history.go`: Error history management and replay functionality
  - `uninstall.go`: Clean removal of AISH components
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
make build          # Builds to bin/aish

# Alternative build commands
go build -o aish ./cmd/aish
```

### Testing
```bash
# Run all tests with coverage (minimum 60% required)
make test

# Check coverage threshold
make coverage-min

# Run specific package tests
go test ./internal/capture/ -v
go test ./internal/llm/gemini-cli/... -v
go test ./internal/classification/ -v
go test ./internal/shell/ -v
go test ./internal/ui/ -v

# All tests run with race detection enabled
go test ./... -race
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

# Dependency management
make tidy
```

### Running in Development
```bash
# Build and install
make build && ./bin/aish init

# Test error capture
./bin/aish capture 127 "unknowncmd"

# Run with debug mode
AISH_DEBUG_GEMINI=1 ./bin/aish

# Test configuration commands
./bin/aish config show
./bin/aish config set auto_execute true
./bin/aish config get auto_execute

# Diagnostic checks (useful for development)
./bin/aish doctor                    # Run all checks
./bin/aish doctor --llm              # Check LLM connectivity
./bin/aish doctor --hooks            # Check shell hook installation
./bin/aish doctor --json             # Output in JSON format
./bin/aish doctor --fix              # Attempt to fix issues

# Test natural language commands
./bin/aish -p "list all files"       # Generate command
./bin/aish -a "explain shell hooks"  # Plain text answer
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

The project uses **GoReleaser** for automated cross-platform releases. Configuration is in `.goreleaser.yaml`.

### Release Workflow

When a version tag is pushed (e.g., `v0.0.2`), GitHub Actions automatically:
1. Builds binaries for multiple platforms (Linux, macOS, Windows) with both amd64 and arm64 architectures
2. Creates `.deb` and `.rpm` packages for Linux distributions
3. Updates the APT repository at https://tonnywong1052.github.io/aish-apt-repo
4. Updates Homebrew tap at https://github.com/TonnyWong1052/homebrew-aish
5. Prepares Scoop manifest for Windows (manual upload required)
6. Prepares WinGet manifest for Windows Package Manager (manual upload required)
7. Prepares AUR PKGBUILD for Arch Linux (manual upload required)
8. Generates SHA256 checksums for all artifacts
9. Creates GitHub Release with all artifacts and changelog

### Creating a Release
```bash
# 1. Ensure all changes are committed and tests pass
make ci

# 2. Tag the release (follow semantic versioning)
git tag v0.0.2
git push origin v0.0.2

# 3. GitHub Actions will automatically trigger the release workflow
# Monitor at: https://github.com/TonnyWong1052/aish/actions

# 4. After release, manual steps (if needed):
# - Upload Scoop manifest to scoop-aish repository
# - Submit WinGet manifest PR to microsoft/winget-pkgs
# - Upload AUR PKGBUILD to aur.archlinux.org
```

### GoReleaser Configuration Details

**Build Configuration** (`.goreleaser.yaml` lines 8-21):
- CGO disabled for static binaries
- Version info injected via ldflags: `-X main._version={{.Version}}`
- Supports: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64, windows/arm64

**Package Managers**:
- **APT (Debian/Ubuntu)**: Fully automated via GitHub Actions + GitHub Pages
- **Homebrew (macOS/Linux)**: Fully automated, formula published to TonnyWong1052/homebrew-aish
- **Scoop (Windows)**: Manifest prepared, requires manual upload to scoop-aish repo
- **WinGet (Windows)**: Manifest prepared, requires PR to microsoft/winget-pkgs
- **AUR (Arch Linux)**: PKGBUILD prepared, requires manual upload to AUR

**Scripts**:
- `packaging/deb/postinstall.sh`: Post-installation script for .deb packages
- `packaging/deb/preremove.sh`: Pre-removal script for .deb packages

### Testing GoReleaser Locally
```bash
# Install GoReleaser (if not already installed)
brew install goreleaser  # macOS
# or: go install github.com/goreleaser/goreleaser/v2@latest

# Test build without publishing (dry-run)
goreleaser release --snapshot --clean

# Validate configuration
goreleaser check

# Build artifacts will be in dist/ directory
```

## Prompt Engineering

The application uses sophisticated prompt templates located in:
- `internal/prompts`: Template file containing AI prompts for different error categories
- `internal/prompt/manager.go`: Prompt template manager with context injection

Key prompt categories:
- **Error Analysis**: Context-aware prompts based on error classification
- **Command Generation**: Natural language to shell command translation
- **General Q&A**: Plain-text responses without command suggestions

## Diagnostic Tool (`aish doctor`)

The `doctor` command provides comprehensive diagnostics for troubleshooting AISH setup issues. It validates:

### What It Checks
1. **Shell Hook Installation**: Verifies hook is installed in `.bashrc`/`.zshrc`/etc.
2. **File Permissions**: Checks accessibility of config and history files
3. **LLM Provider Connectivity**: Tests connection to configured AI providers
4. **Proxy Configuration**: Validates proxy environment variables

### Usage Examples
```bash
# Run all diagnostic checks
aish doctor

# Run specific checks
aish doctor --hooks                          # Only check shell hooks
aish doctor --llm --provider=openai          # Test specific provider
aish doctor --perms                          # Check file permissions
aish doctor --proxy                          # Check proxy settings

# Output formats
aish doctor --json                           # Machine-readable JSON output
aish doctor --verbose                        # Include detailed diagnostics

# Automatic fixes
aish doctor --fix                            # Attempt to fix detected issues

# Advanced options
aish doctor --timeout=10s                    # Custom network timeout
aish doctor --no-parallel                    # Run checks sequentially
```

### Exit Codes
- `0`: All checks passed
- `1`: One or more checks failed

### Implementation Details
The doctor command (`cmd/aish/doctor.go`) runs checks in parallel by default (up to 4 concurrent checks) for faster diagnostics. Each check returns:
- Status: OK, WARN, or FAIL
- Message: Human-readable description
- Suggestion: Recommended fix (if applicable)
- Duration: Time taken for the check

Common issues detected:
- Missing or incorrect shell hook installation
- Invalid API keys or authentication failures
- Network connectivity problems (timeouts, TLS errors)
- Rate limiting from LLM providers
- Incorrect file permissions on config/history files
- Misconfigured proxy settings

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
- **Required**: Go 1.24.1 or higher (as specified in `go.mod`)
- Uses latest Go features including enhanced generics support for type-safe LLM responses
- All tests run with race detection enabled (`-race` flag)

### Key Dependencies
- **Cobra** (`spf13/cobra`): CLI framework for command routing and flags
- **Genkit Go** (`firebase/genkit/go` v1.0.5): Unified LLM interaction framework for Claude and Ollama
- **OpenAI Go SDK** (`openai/openai-go`): Official OpenAI API client
- **pterm**: Terminal UI components with color support
- **Bubble Tea** (`charmbracelet/bubbletea`): Interactive TUI for settings and wizards
- **logrus**: Structured logging

See `go.mod` for complete dependency list. Run `make tidy` after modifying dependencies.

### Environment Variables
- `AISH_DEBUG_GEMINI`: Enable debug logging for Gemini provider
- `AISH_GEMINI_PROJECT`: Override default Gemini project ID
- `AISH_STDOUT_FILE`, `AISH_STDERR_FILE`: Custom paths for captured output
- `AISH_SKIP_COMMAND_PATTERNS`: Skip hook for specific command patterns
- `AISH_SKIP_ALL_USER_COMMANDS`: Skip all user-installed commands
- `AISH_CAPTURE_OFF`: Temporarily disable hook for one command
- `XDG_CONFIG_HOME`: Override config directory location (default: `~/.config`)

## Development Best Practices

### Before Submitting Changes
```bash
# Always run the full CI suite before committing
make ci

# Ensure coverage meets minimum threshold
make coverage-min

# Clean build artifacts
make clean
```

### Adding a New LLM Provider
1. Create provider package in `internal/llm/<provider-name>/`
2. Implement the `llm.Provider` interface
3. For Genkit-based providers: Use `llm.NewGenkitAdapter()` (see Claude/Ollama examples)
4. Add provider configuration to `internal/config/config.go`
5. Register provider constant in `internal/config/providers.go`
6. Update `cmd/aish/init.go` wizard with new provider option
7. Add provider-specific tests
8. Update documentation in CLAUDE.md and README.md

### Debugging Shell Hook Issues
```bash
# Enable hook debugging
export AISH_DEBUG_GEMINI=1

# Check hook installation
cat ~/.zshrc | grep -A 20 "# AISH Hook"

# Test hook manually
./bin/aish capture 1 "false"

# View captured output
ls -la ~/.config/aish/
cat ~/.config/aish/last_stdout
cat ~/.config/aish/last_stderr
```

### Working with Genkit Providers
The Claude and Ollama providers use Firebase Genkit for LLM interaction:
- Provider initialization: Create Genkit instance with plugin in `NewClient()`
- Model naming: Use format `"<provider>/<model>"` (e.g., `"anthropic/claude-3-5-sonnet-20241022"`)
- Error handling: Genkit errors are wrapped with context-specific messages
- Testing: Mock Genkit responses in unit tests using test doubles

### Common Development Scenarios

**Testing a single provider:**
```bash
# Build
make build

# Configure provider
./bin/aish config set default_provider claude
./bin/aish config set providers.claude.api_key YOUR_KEY

# Test connectivity
./bin/aish doctor --llm --provider=claude

# Test command generation
./bin/aish -p "list files"
```

**Debugging error classification:**
```bash
# Enable verbose logging
export AISH_DEBUG_GEMINI=1

# Trigger specific error types
./bin/aish capture 127 "unknowncmd"        # CommandNotFound
./bin/aish capture 126 "cat /etc/shadow"  # PermissionDenied
```

**Testing across platforms:**
- Use GitHub Actions for automated cross-platform testing
- For local testing: Use Docker containers or VMs for Linux/Windows
- GoReleaser snapshot builds: `goreleaser release --snapshot --clean`
