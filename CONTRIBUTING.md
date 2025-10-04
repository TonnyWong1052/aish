# Contributing to AISH

Thank you for your interest in contributing to AISH (AI Shell)! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Contributing Guidelines](#contributing-guidelines)
- [Code Standards](#code-standards)
- [Testing](#testing)
- [Documentation](#documentation)
- [Pull Request Process](#pull-request-process)
- [Release Process](#release-process)

## Getting Started

### Prerequisites

- **Go 1.24.1 or later**: Required for building AISH
- **Git**: For version control
- **Make**: For running build commands (optional but recommended)
- **Docker**: For containerized testing (optional)

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork locally:

```bash
git clone https://github.com/YOUR_USERNAME/aish.git
cd aish
```

3. Add the upstream remote:

```bash
git remote add upstream https://github.com/TonnyWong1052/aish.git
```

4. Create a feature branch:

```bash
git checkout -b feature/your-feature-name
```

## Development Setup

### Local Development Environment

1. **Install Go dependencies**:

```bash
go mod download
```

2. **Build AISH**:

```bash
# Build main CLI（任選其一）
go build -o aish ./cmd/aish
# 或使用 Makefile 統一流程
make build
```

3. **Run tests**:

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test ./... -race -covermode=atomic -coverprofile=coverage.out

# Run specific package tests
go test ./internal/llm/... -v
```

4. **Install development tools** (optional):

```bash
# Install golangci-lint for linting
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Install staticcheck for additional static analysis
go install honnef.co/go/tools/cmd/staticcheck@latest
```

### Configuration for Development

1. **Set up test configuration**:

```bash
# Create test config directory
mkdir -p ~/.config/aish-dev

# Set environment variable for development
export AISH_CONFIG_DIR=~/.config/aish-dev
```

2. **Configure test LLM provider**:

```bash
# Use a test API key (or mock provider for testing)
./aish config set default_provider gemini-cli
./aish config set providers.gemini-cli.project "test-project"
```

### Development Workflow

1. **Check code formatting（推薦使用 Makefile）**：

```bash
# 一鍵格式化（gofumpt + gci + goimports）
make fmt

# 僅檢查 gofmt 差異（CI 也會執行，需為零差異）
gofmt -s -l .
```

2. **Run linting**：

```bash
# Using golangci-lint
golangci-lint run

# Using go vet
go vet ./...
```

3. **Run tests**：

```bash
# Quick test run
go test ./...

# Full test with race detection
go test ./... -race

# Test specific components
go test ./internal/llm/gemini-cli/... -v
```

## Contributing Guidelines

### Types of Contributions

We welcome the following types of contributions:

- **🐛 Bug fixes**: Fix existing issues or unexpected behavior
- **✨ Features**: Add new functionality to AISH
- **📚 Documentation**: Improve docs, add examples, or write tutorials
- **🧪 Tests**: Add or improve test coverage
- **🔧 Infrastructure**: Improve build, CI/CD, or development experience
- **🎨 UI/UX**: Improve command-line interface and user experience

### Before Contributing

1. **Check existing issues**: Look for related issues or feature requests
2. **Discuss major changes**: Open an issue to discuss significant changes before implementing
3. **Follow conventions**: Ensure your contribution follows project conventions
4. **Write tests**: Include tests for new functionality
5. **Update documentation**: Update relevant documentation

### Contribution Areas

#### 1. LLM Provider Integration

Add support for new LLM providers:

```go
// Example: Adding a new provider
package newprovider

import (
    "context"
    "github.com/TonnyWong1052/aish/internal/llm"
    "github.com/TonnyWong1052/aish/internal/config"
    "github.com/TonnyWong1052/aish/internal/prompt"
)

type NewProvider struct {
    cfg config.ProviderConfig
    pm  *prompt.Manager
}

func NewProvider(cfg config.ProviderConfig, pm *prompt.Manager) (llm.Provider, error) {
    return &NewProvider{cfg: cfg, pm: pm}, nil
}

func (p *NewProvider) GetSuggestion(ctx context.Context, capturedContext llm.CapturedContext, lang string) (*llm.Suggestion, error) {
    // Implementation
    return &llm.Suggestion{}, nil
}

// Register provider
func init() {
    llm.RegisterProvider("newprovider", NewProvider)
}
```

#### 2. Error Classification

Extend error classification patterns:

```go
// Add new error patterns in internal/classification/capture.go
var errorPatterns = []ErrorPattern{
    {
        Type: CustomError,
        Patterns: []string{
            "custom error pattern",
            "specific failure message",
        },
    },
}
```

#### 3. Shell Integration

Add support for new shells:

```bash
# Add shell hooks in internal/shell/hook.go
# Example: Fish shell support
function aish_preexec --on-event fish_preexec
    # Implementation for Fish shell
end
```

#### 4. Security Enhancements

Improve data sanitization patterns:

```go
// Add new sanitization patterns in internal/security/sanitizer.go
{"new_pattern", `(?i)(pattern)`, "***REDACTED***", 8},
```

## Code Standards

### Go Style Guidelines

1. **Follow Go conventions**:
   - Use `gofmt` for formatting
   - Follow Go naming conventions
   - Use Go idioms and patterns

2. **Package organization**:
   - Keep packages focused and cohesive
   - Use clear, descriptive package names
   - Avoid circular dependencies

3. **Error handling**:
   - Always handle errors appropriately
   - Use structured error types when helpful
   - Provide meaningful error messages

4. **Documentation**:
   - Document all exported functions and types
   - Use meaningful variable and function names
   - Include examples for complex functionality

### Code Quality Standards

```go
// ✅ Good: Clear function with documentation
// GetSuggestion analyzes the captured command context and returns
// an AI-generated suggestion for fixing the error.
func (p *Provider) GetSuggestion(ctx context.Context, capturedContext CapturedContext, lang string) (*Suggestion, error) {
    if capturedContext.Command == "" {
        return nil, errors.New("command cannot be empty")
    }

    // Implementation...
}

// ❌ Bad: Unclear function without documentation
func (p *Provider) getSug(ctx context.Context, cc CapturedContext, l string) (*Suggestion, error) {
    // Implementation...
}
```

### Security Guidelines

1. **Never log sensitive data**:

```go
// ✅ Good: Log sanitized data
logger.Info("Processing command", "sanitized_command", sanitizer.Sanitize(command))

// ❌ Bad: Log raw data
logger.Info("Processing command", "command", command)
```

2. **Validate all inputs**:

```go
// ✅ Good: Input validation
func validateAPIKey(key string) error {
    if len(key) < 16 {
        return errors.New("API key too short")
    }
    if strings.Contains(key, " ") {
        return errors.New("API key contains invalid characters")
    }
    return nil
}
```

3. **Use secure defaults**:

```go
// ✅ Good: Secure by default
config := &Config{
    EnableSanitization: true,
    LogLevel:          "info", // Not debug by default
    MaxRetries:        3,
}
```

## Testing

### Test Structure

Organize tests alongside the code they test:

```
internal/
├── llm/
│   ├── provider.go
│   ├── provider_test.go
│   └── gemini/
│       ├── client.go
│       └── client_test.go
```

### Test Categories

1. **Unit Tests**: Test individual functions and methods

```go
func TestSanitizeAPIKey(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {
            name:     "basic API key",
            input:    "api-key-12345",
            expected: "***REDACTED***",
        },
    }

    sanitizer := NewSensitiveDataSanitizer()
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := sanitizer.Sanitize(tt.input)
            if result != tt.expected {
                t.Errorf("expected %q, got %q", tt.expected, result)
            }
        })
    }
}
```

2. **Integration Tests**: Test component interactions

```go
func TestProviderIntegration(t *testing.T) {
    // Skip in short mode
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    config := config.ProviderConfig{
        APIKey: "test-key",
        Model:  "test-model",
    }

    provider, err := NewProvider(config, nil)
    if err != nil {
        t.Fatalf("failed to create provider: %v", err)
    }

    // Test provider functionality
}
```

3. **Mock Testing**: Use mocks for external dependencies

```go
type mockLLMProvider struct {
    suggestions map[string]*llm.Suggestion
}

func (m *mockLLMProvider) GetSuggestion(ctx context.Context, capturedContext llm.CapturedContext, lang string) (*llm.Suggestion, error) {
    if suggestion, exists := m.suggestions[capturedContext.Command]; exists {
        return suggestion, nil
    }
    return nil, errors.New("no suggestion available")
}
```

### Test Best Practices

1. **Use table-driven tests** for multiple test cases
2. **Test error conditions** as well as success cases
3. **Use meaningful test names** that describe what is being tested
4. **Clean up resources** in tests (use `t.Cleanup()` or `defer`)
5. **Use build tags** for integration tests: `//go:build integration`

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run only unit tests
go test ./... -short

# Run integration tests
go test ./... -tags=integration

# Run specific test
go test ./internal/llm/... -run TestProviderSuggestion

# Run tests with race detection
go test ./... -race

# Benchmark tests
go test ./... -bench=.
```

## Documentation

### Types of Documentation

1. **Code Comments**: Document complex logic and public APIs

```go
// ClassifyError determines the error type based on exit code and output.
// It returns the most specific error classification that matches the given
// criteria, falling back to GenericError if no specific pattern matches.
func ClassifyError(exitCode int, stdout, stderr string) ErrorType {
    // Implementation...
}
```

2. **README Updates**: Update user-facing documentation

3. **API Documentation**: Document APIs in `docs/API.md`

4. **Architecture Documentation**: Update `ARCHITECTURE.md` for design changes

5. **Examples**: Provide usage examples

```go
// Example usage in documentation:
//
//  sanitizer := security.NewSensitiveDataSanitizer()
//  cleanText := sanitizer.Sanitize("curl -H 'Authorization: Bearer sk-123'")
//  // Result: "curl -H 'Authorization: Bearer ***REDACTED***'"
```

### Documentation Standards

1. **Use clear, concise language**
2. **Provide examples** for complex features
3. **Keep documentation up to date** with code changes
4. **Include troubleshooting information** when appropriate
5. **Use proper markdown formatting**

## Pull Request Process

### Before Submitting

1. **Ensure tests pass**:

```bash
go test ./...
golangci-lint run
gofmt -s -l .
```

2. **Update documentation** if needed

3. **Add tests** for new functionality

4. **Check for breaking changes**

### Pull Request Template

Use this template for your pull requests:

```markdown
## Description
Brief description of changes and motivation.

## Type of Change
- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature (non-breaking change that adds functionality)
- [ ] Breaking change (fix or feature that causes existing functionality to change)
- [ ] Documentation update

## Changes Made
- List specific changes made
- Include any new dependencies
- Mention any configuration changes

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Manual testing completed

## Screenshots (if applicable)
Include screenshots of CLI output or interface changes.

## Checklist
- [ ] Code follows project style guidelines
- [ ] Self-review completed
- [ ] Tests added for new functionality
- [ ] Documentation updated
- [ ] No breaking changes (or clearly documented)
```

### Review Process

1. **Automated checks**: CI/CD pipeline runs automatically
2. **Code review**: Maintainers review code for quality and style
3. **Testing**: Verify functionality works as expected
4. **Documentation**: Check that documentation is updated
5. **Approval**: At least one maintainer approval required

### Addressing Review Comments

1. **Make requested changes** in new commits
2. **Respond to comments** to clarify any questions
3. **Test changes** before requesting re-review
4. **Squash commits** before merging (if requested)

## Release Process

### Version Management

AISH uses semantic versioning (semver):

- **Major** (x.0.0): Breaking changes
- **Minor** (0.x.0): New features, backward compatible
- **Patch** (0.0.x): Bug fixes, backward compatible

### Release Workflow

1. **Version bump**: Update version in relevant files
2. **Changelog**: Update CHANGELOG.md with new features and fixes
3. **Tag release**: Create and push git tag
4. **GitHub release**: Automated via GitHub Actions
5. **Homebrew**: Automated formula update

### Contributing to Releases

- Test release candidates
- Report issues with pre-releases
- Help update documentation for new features
- Assist with Homebrew formula updates

## Community Guidelines

### Code of Conduct

- Be respectful and inclusive
- Welcome newcomers and help them contribute
- Focus on constructive feedback
- Assume good intentions

### Getting Help

- **Documentation**: Check existing docs first
- **Issues**: Search for existing issues before creating new ones
- **Discussions**: Use GitHub Discussions for questions
- **Discord/Slack**: Join community channels (if available)

### Recognition

Contributors are recognized in:
- Release notes
- Contributors section in README
- Special recognition for significant contributions

## Development Resources

### Useful Commands

```bash
# Development workflow
make dev-setup          # Set up development environment
make test               # Run all tests
make lint               # Run linting
make build              # Build binaries
make clean              # Clean build artifacts

# Release workflow
make release-dry-run    # Test release process
make release            # Create release
```

### Project Structure

```
aish/
├── cmd/aish/           # CLI application entry point
├── internal/           # Private application code
│   ├── llm/           # LLM provider implementations
│   ├── security/      # Security and sanitization
│   ├── config/        # Configuration management
│   └── ...
├── docs/              # Documentation
├── scripts/           # Build and utility scripts
├── demo/              # Demo assets
└── tools/             # Development tools
```

### Key Files to Know

- `cmd/aish/main.go`: CLI entry point
- `internal/llm/provider.go`: LLM provider interface
- `internal/config/config.go`: Configuration structure
- `internal/security/sanitizer.go`: Data sanitization
- `ARCHITECTURE.md`: System architecture documentation

Thank you for contributing to AISH! Your contributions help make intelligent terminal assistance better for everyone.
4. **One‑shot 本地 CI**：

```bash
# 本地模擬 CI 的主要步驟：gofmt 檢查 + golangci-lint + vet + test
make ci
```

5. **整理依賴（go.mod/go.sum）**：

```bash
make tidy
```

6. **啟用 Git hooks（可選）**：

```bash
# 使用本庫的 hooks 目錄
git config core.hooksPath .githooks

# 確保 pre-commit 可執行（部分平台需要）
chmod +x .githooks/pre-commit

# 或使用腳本直接執行預檢查
bash scripts/pre-commit.sh
```

7. **覆蓋率門檻（本地檢查）**：

```bash
# 預設 60%，可覆寫 COV_MIN
make test && make coverage-min
make test && make coverage-min COV_MIN=65
```

### Formatting & Imports（基於 Uber Go Style Guide）

- Import 分組與排序：標準庫一組、其他套件一組、內部模組一組（`prefix(github.com/TonnyWong1052/aish)`）。
- 盡量避免不必要別名（例如 `runtimetrace "runtime/trace"`），僅在 package 名與路徑不相符時（或衝突）使用別名。
- 錯誤處理：對外的哨兵錯誤以 `ErrXxx` 命名；客製錯誤型別以 `XxxError` 命名；使用 `errors.Is/As` 比對，`fmt.Errorf("…: %w", err)` 包裝。
- 測試：優先 table‑driven；錯誤用 `t.Fatal/Fatalf` 結束，不使用 `panic`。
