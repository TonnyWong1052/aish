# AISH Architecture Documentation

![AISH System Architecture](./demo/system_architecture.png)

## Overview

AISH (AI Shell) is a sophisticated command-line tool that provides intelligent terminal assistance by capturing command errors, analyzing them with AI, and offering smart suggestions. This document provides a comprehensive overview of the system architecture, design patterns, and implementation details.

## System Architecture

### Core Components

```
┌─────────────────────────────────────────────────────────────────┐
│                        AISH CLI Application                     │
├─────────────────────────────────────────────────────────────────┤
│  cmd/aish/                                                      │
│  ├── main.go           (Entry point & command routing)         │
│  ├── init.go           (Setup & configuration wizard)          │
│  ├── config.go         (Configuration management)              │
│  ├── history.go        (Command history operations)            │
│  └── hook.go           (Shell hook management)                 │
└─────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Internal Packages                          │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │   LLM Layer     │  │ Classification  │  │   Shell Hook    │ │
│  │                 │  │                 │  │                 │ │
│  │ • OpenAI        │  │ • Error Types   │  │ • Bash/Zsh     │ │
│  │ • Gemini        │  │ • Pattern Match │  │ • Command Cap   │ │
│  │ • Gemini CLI    │  │ • Recovery      │  │ • Output Redir  │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
│                                                                 │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │   Security      │  │     Caching     │  │   Monitoring    │ │
│  │                 │  │                 │  │                 │ │
│  │ • Data Sanit    │  │ • Intelligent   │  │ • Performance   │ │
│  │ • Encryption    │  │ • Semantic      │  │ • Analytics     │ │
│  │ • Secure Config │  │ • Prewarming    │  │ • Error Track   │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Layer-by-Layer Architecture

### 1. CLI Interface Layer (`cmd/aish/`)

**Purpose**: Command-line interface and user interaction management

**Key Components**:
- **main.go**: Application entry point with Cobra command framework
- **init.go**: Interactive setup wizard for first-time configuration
- **config.go**: Configuration file management and provider settings
- **history.go**: Command history storage and retrieval operations
- **hook.go**: Shell hook installation and management

**Responsibilities**:
- Command parsing and routing
- User input validation
- Configuration wizard flow
- Shell integration setup

### 2. LLM Provider Layer (`internal/llm/`)

**Purpose**: Abstraction layer for different AI/LLM providers

**Architecture Pattern**: Strategy Pattern with Provider Interface

```go
type Provider interface {
    GetSuggestion(ctx context.Context, capturedContext CapturedContext, lang string) (*Suggestion, error)
    GetEnhancedSuggestion(ctx context.Context, enhancedCtx EnhancedCapturedContext, lang string) (*Suggestion, error)
    GenerateCommand(ctx context.Context, promptText string, lang string) (string, error)
    VerifyConnection(ctx context.Context) ([]string, error)
}
```

**Supported Providers**:
- **OpenAI**: GPT models via official API
- **Gemini**: Google's Gemini via official API
- **Gemini CLI**: Google's Gemini via CLI tool (recommended)

**Key Features**:
- Provider registration system
- Automatic failover between HTTP/cURL/CLI methods
- Token management and refresh
- Request caching and retry logic

### 3. Shell Hook Layer (`internal/shell/`)

**Purpose**: Seamless shell integration for automatic error capture

**Supported Shells**:
- **Bash**: Uses `trap DEBUG` and `PROMPT_COMMAND`
- **Zsh**: Uses `preexec` and `precmd` hooks
- **PowerShell**: Profile modification (Windows)

**Security Features**:
- Automatic sensitive data redaction
- Command filtering and whitelisting
- TTY stream handling for interactive tools

**Hook Workflow**:
```
User Command → Pre-execution Hook → Command Execution → Post-execution Hook
      ↓                ↓                     ↓                    ↓
  Redaction      Output Capture       Exit Code Check     AI Analysis Trigger
```

### 4. Error Classification Layer (`internal/classification/`)

**Purpose**: Intelligent categorization of command failures

**Classification Types**:
- `CommandNotFound` - Missing commands or typos
- `FileNotFoundOrDirectory` - Path/file access issues
- `PermissionDenied` - Access rights problems
- `CannotExecute` - Binary execution issues
- `InvalidArgumentOrOption` - Syntax errors
- `ResourceExists` - File/directory conflicts
- `NotADirectory` - Path type mismatches
- `TerminatedBySignal` - User interruption
- `GenericError` - Other error types

**Pattern Matching Engine**:
- Regex-based error detection
- Exit code analysis
- Output pattern recognition
- Context-aware classification

### 5. Security Layer (`internal/security/`)

**Purpose**: Comprehensive data protection and sanitization

**Components**:

**Data Sanitizer** (`sanitizer.go`):
- API key and token redaction
- Password and secret masking
- Environment variable protection
- Credit card and SSN scrubbing
- JWT token detection

**Encryption Manager** (`crypto/`):
- AES-256-GCM encryption for stored secrets
- PBKDF2 key derivation
- Secure key storage and rotation
- API key encryption for configuration

**Secure Configuration** (`secure_config.go`):
- Encrypted configuration storage
- Access control and validation
- Secure defaults and hardening

### 6. Caching Layer (`internal/cache/`)

**Purpose**: Intelligent caching for performance optimization

**Cache Types**:

**Intelligent Cache**:
- Semantic similarity detection
- Smart cache key generation
- TTL-based expiration
- LRU eviction policies

**Semantic Index**:
- Content fingerprinting
- Similarity scoring
- Context-aware matching

**Cache Prewarmer**:
- Predictive content loading
- Background refresh jobs
- Usage pattern analysis

### 7. Concurrent Processing (`internal/concurrent/`)

**Purpose**: Efficient resource utilization and parallel processing

**Components**:
- **Worker Pools**: Separate pools for AI, local, and cache operations
- **Coordinator**: Central orchestration of concurrent tasks
- **Pipeline**: Stream processing for large data sets

**Concurrency Patterns**:
- Producer-Consumer queues
- Circuit breaker for fault tolerance
- Rate limiting and backpressure

### 8. Monitoring & Analytics (`internal/monitoring/`)

**Purpose**: Performance tracking and system observability

**Metrics Collection**:
- Response time percentiles
- Error rates and classifications
- Cache hit/miss ratios
- Resource utilization

**Performance Analysis**:
- Latency tracking
- Trend analysis
- Anomaly detection
- Capacity planning metrics

## Design Patterns & Principles

### 1. Dependency Injection
- Provider registration system
- Configurable components
- Testable architecture

### 2. Strategy Pattern
- Multiple LLM providers
- Pluggable cache backends
- Configurable sanitization rules

### 3. Observer Pattern
- Event-driven hook system
- Metrics collection
- Configuration changes

### 4. Circuit Breaker
- Provider fault tolerance
- Graceful degradation
- Automatic recovery

### 5. Command Pattern
- CLI command structure
- Undo/redo operations
- Batch processing

## Data Flow

### 1. Error Capture Flow
```
Shell Command → Hook Intercept → Output Capture → Error Classification → AI Analysis → User Presentation
```

### 2. Command Generation Flow
```
User Prompt → Template Processing → Provider Selection → AI Request → Response Parse → Command Validation
```

### 3. Configuration Flow
```
User Input → Validation → Encryption → Storage → Provider Registration → Connection Test
```

## Security Architecture

### 1. Data Protection Layers
- **Input Sanitization**: Command-line argument cleaning
- **Transport Security**: HTTPS/TLS for API calls
- **Storage Encryption**: AES-256 for local secrets
- **Output Filtering**: Sensitive data redaction

### 2. Access Control
- **File Permissions**: Secure configuration directories
- **Process Isolation**: Sandboxed command execution
- **Network Security**: Validated API endpoints
- **Audit Logging**: Security event tracking

### 3. Threat Model
- **Man-in-the-Middle**: Certificate pinning
- **Data Exfiltration**: Automatic redaction
- **Local Access**: File encryption
- **API Abuse**: Rate limiting and monitoring

## Configuration Management

### 1. Configuration Hierarchy
```
Environment Variables → Command Line Flags → Config File → Defaults
```

### 2. Provider Configuration
```yaml
providers:
  openai:
    api_key: "encrypted_key"
    api_endpoint: "https://api.openai.com/v1"
    model: "gpt-4"
  gemini:
    api_key: "encrypted_key"
    project: "project-id"
    model: "gemini-pro"
```

### 3. Security Settings
```yaml
security:
  enable_sanitization: true
  encryption_key_rotation: "30d"
  audit_logging: true
```

## Testing Strategy

### 1. Unit Tests
- Individual component testing
- Mock provider implementations
- Isolated functionality verification

### 2. Integration Tests
- End-to-end provider testing
- Shell hook validation
- Configuration management

### 3. Performance Tests
- Load testing for concurrent requests
- Memory usage profiling
- Cache performance benchmarks

### 4. Security Tests
- Sanitization effectiveness
- Encryption/decryption validation
- Access control verification

## Deployment Architecture

### 1. Installation Methods
- **Homebrew**: macOS/Linux package manager
- **Binary Release**: Cross-platform executables
- **Source Build**: Manual compilation

### 2. System Integration
- **Shell Profile**: Automatic hook installation
- **Configuration**: Secure key storage
- **Updates**: Automated version checking

### 3. Platform Support
- **macOS**: Native support with Homebrew
- **Linux**: Multiple distribution support
- **Windows**: PowerShell integration

## Extension Points

### 1. New LLM Providers
- Implement `Provider` interface
- Register with provider registry
- Add configuration schema

### 2. Custom Error Classifiers
- Extend classification patterns
- Add new error types
- Implement recovery strategies

### 3. Additional Shell Support
- Implement shell-specific hooks
- Add platform integration
- Create installation scripts

### 4. Monitoring Integrations
- Custom metrics exporters
- Third-party APM tools
- Log aggregation systems

## Performance Considerations

### 1. Response Time Optimization
- Request caching and memoization
- Parallel provider requests
- Connection pooling

### 2. Memory Management
- Efficient string processing
- Cache size limits
- Garbage collection tuning

### 3. Network Optimization
- Request batching
- Compression and streaming
- Connection reuse

### 4. Scalability Features
- Horizontal scaling support
- Load balancing strategies
- Resource usage monitoring

## Development Guidelines

### 1. Code Organization
- Package-based modularity
- Clear interface definitions
- Minimal dependencies

### 2. Error Handling
- Structured error types
- Context propagation
- Graceful degradation

### 3. Logging & Debugging
- Structured logging with levels
- Debug mode support
- Performance profiling

### 4. Documentation
- Inline code comments
- API documentation
- Architecture decisions

This architecture enables AISH to provide intelligent, secure, and performant terminal assistance while maintaining extensibility for future enhancements.