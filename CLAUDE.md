# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Architecture

This is **AISH** (AI Shell), a powerful CLI tool written in Go that integrates with Google's Gemini AI to provide intelligent terminal assistance. The application captures terminal output, processes it with AI, and provides smart insights and suggestions.

### Core Components

- **main.go**: Entry point with command routing and configuration
- **cmd/aish/**: Alternative entry point for the CLI tool  
- **internal/llm/gemini-cli/**: Google Gemini AI integration client with streaming support
- **internal/capture/**: Terminal output capture system using pseudo-terminal (pty)
- **internal/commands/**: Command execution and processing logic

### Key Architecture Patterns

- Uses Go modules with clean internal package structure
- Implements pseudo-terminal (pty) for seamless command capture
- Streaming AI responses for real-time feedback
- Environment variable configuration for API keys and project settings

## Development Commands

### Building
```bash
# Build the main binary
go build -o aish ./cmd/aish

# Build with specific output location
go build -o aish main.go
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test ./... -v

# Test specific packages
go test ./internal/capture/ -v
go test ./internal/llm/gemini-cli/... -v
```

### Running
```bash
# Run directly with Go
go run main.go

# Run the cmd version
go run ./cmd/aish

# Run with debug mode for Gemini
AISH_DEBUG_GEMINI=1 go run main.go

# Test with non-existent project (for error handling)
AISH_DEBUG_GEMINI=1 AISH_GEMINI_PROJECT=Not-Exist-Project-Name go run main.go
```

## Configuration

The application uses environment variables:
- `AISH_DEBUG_GEMINI`: Enable debug logging for Gemini API calls
- `AISH_GEMINI_PROJECT`: Google Cloud project ID for Gemini API

## Key Files to Understand

- `internal/llm/gemini-cli/client.go`: Core AI integration logic
- `internal/capture/capture.go`: Terminal output capture mechanism  
- `internal/commands/commands.go`: Command processing and execution
- `main.go`: Application entry point and configuration

## Testing Strategy

Tests are located alongside source files using Go's standard `*_test.go` convention. The codebase includes unit tests for core packages like capture and gemini-cli integration.
