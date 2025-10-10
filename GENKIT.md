# Genkit Go Integration Architecture

This document describes the integration of **Genkit Go** (v1.0.5) framework in the AISH project for unified LLM provider management.

## Table of Contents
- [Overview](#overview)
- [Why Genkit?](#why-genkit)
- [Architecture](#architecture)
- [Implementation Details](#implementation-details)
- [Provider Setup](#provider-setup)
- [Usage Examples](#usage-examples)
- [Migration Guide](#migration-guide)
- [Future Extensions](#future-extensions)

## Overview

**Genkit Go** is Google's open-source AI framework that provides a unified interface for interacting with various LLM providers. AISH uses Genkit for **Claude (Anthropic)** and **Ollama** providers to simplify integration and maintain consistency.

### Key Benefits
- **Unified API**: Single interface for multiple LLM providers
- **Plugin Ecosystem**: Easy to add new providers via Genkit plugins
- **Type Safety**: Strong typing for prompts and structured responses
- **Built-in Features**: Telemetry, tracing, and error handling
- **Reduced Boilerplate**: Less code for LLM interactions

## Why Genkit?

### Before Genkit
Each provider required custom HTTP client implementation:
- Manual request/response handling
- Provider-specific error parsing
- Duplicate code for similar operations
- Complex JSON marshaling/unmarshaling

### After Genkit
- Single adapter layer bridges Genkit with existing `llm.Provider` interface
- Plugin-based provider initialization
- Consistent error handling across providers
- Simplified code maintenance

## Architecture

### High-Level Design

```
┌─────────────────────────────────────────────────┐
│              AISH Application                   │
│  (commands, capture, classification, etc.)      │
└────────────────────┬────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────┐
│          llm.Provider Interface                 │
│  - GetSuggestion()                              │
│  - GetEnhancedSuggestion()                      │
│  - GenerateCommand()                            │
│  - VerifyConnection()                           │
└────────────────────┬────────────────────────────┘
                     │
        ┌────────────┴────────────┐
        │                         │
        ▼                         ▼
┌──────────────┐          ┌──────────────┐
│    Genkit    │          │   Non-Genkit │
│   Providers  │          │   Providers  │
├──────────────┤          ├──────────────┤
│ • Claude     │          │ • Gemini     │
│ • Ollama     │          │ • Gemini-CLI │
└──────┬───────┘          │ • OpenAI     │
       │                  └──────────────┘
       ▼
┌─────────────────────────┐
│  GenkitAdapter Layer    │
│  (internal/llm/)        │
├─────────────────────────┤
│ • Generate()            │
│ • GenerateStructured[T] │
│ • TestGeneration()      │
└──────────┬──────────────┘
           │
           ▼
┌─────────────────────────┐
│   Genkit Go Core        │
│   (firebase/genkit)     │
├─────────────────────────┤
│ • genkit.Init()         │
│ • genkit.Generate()     │
│ • genkit.GenerateData() │
└──────────┬──────────────┘
           │
    ┌──────┴────────┐
    │               │
    ▼               ▼
┌─────────┐   ┌──────────┐
│Anthropic│   │  Ollama  │
│ Plugin  │   │  Plugin  │
└─────────┘   └──────────┘
```

### Component Breakdown

1. **llm.Provider Interface**: Core abstraction that all providers implement
2. **GenkitAdapter**: Bridge between Genkit and the Provider interface
3. **Genkit Plugins**: Provider-specific plugins (Anthropic, Ollama)
4. **Genkit Core**: Framework that manages model interactions

## Implementation Details

### GenkitAdapter Layer

**File**: `internal/llm/genkit_adapter.go`

```go
package llm

import (
    "context"
    "fmt"
    "github.com/firebase/genkit/go/ai"
    "github.com/firebase/genkit/go/genkit"
)

// GenkitAdapter bridges Genkit with the llm.Provider interface
type GenkitAdapter struct {
    g         *genkit.Genkit
    modelName string
}

func NewGenkitAdapter(g *genkit.Genkit, modelName string) *GenkitAdapter {
    return &GenkitAdapter{
        g:         g,
        modelName: modelName,
    }
}

// Generate wraps Genkit's generate method
func (a *GenkitAdapter) Generate(ctx context.Context, prompt string) (string, error) {
    resp, err := genkit.Generate(ctx, a.g,
        ai.WithPrompt(prompt),
        ai.WithModelName(a.modelName),
    )
    if err != nil {
        return "", fmt.Errorf("genkit generate failed: %w", err)
    }
    if resp == nil {
        return "", fmt.Errorf("genkit returned nil response")
    }
    return resp.Text(), nil
}

// GenerateStructured provides type-safe structured output
func GenerateStructured[T any](ctx context.Context, a *GenkitAdapter, prompt string) (*T, error) {
    result, _, err := genkit.GenerateData[T](ctx, a.g,
        ai.WithPrompt(prompt),
        ai.WithModelName(a.modelName),
    )
    if err != nil {
        return nil, fmt.Errorf("genkit structured generate failed: %w", err)
    }
    if result == nil {
        return nil, fmt.Errorf("genkit returned nil structured result")
    }
    return result, nil
}

// TestGeneration tests basic connectivity
func (a *GenkitAdapter) TestGeneration(ctx context.Context) error {
    _, err := a.Generate(ctx, "Hello")
    return err
}
```

### Claude Provider Implementation

**File**: `internal/llm/anthropic/client.go`

```go
package anthropic

import (
    "context"
    "fmt"
    "strings"
    "text/template"

    "github.com/TonnyWong1052/aish/internal/config"
    "github.com/TonnyWong1052/aish/internal/llm"
    "github.com/TonnyWong1052/aish/internal/prompt"
    "github.com/firebase/genkit/go/genkit"
    anthropicPlugin "github.com/firebase/genkit/go/plugins/compat_oai/anthropic"
    "github.com/openai/openai-go/option"
)

type ClaudeProvider struct {
    cfg     config.ProviderConfig
    pm      *prompt.Manager
    genkit  *genkit.Genkit
    adapter *llm.GenkitAdapter
}

func NewProvider(cfg config.ProviderConfig, pm *prompt.Manager) (llm.Provider, error) {
    ctx := context.Background()

    // Initialize Genkit with Anthropic plugin
    g := genkit.Init(ctx,
        genkit.WithPlugins(&anthropicPlugin.Anthropic{
            Opts: []option.RequestOption{
                option.WithAPIKey(cfg.APIKey),
            },
        }),
    )

    // Model name requires "anthropic/" prefix
    modelName := "anthropic/" + cfg.Model
    adapter := llm.NewGenkitAdapter(g, modelName)

    return &ClaudeProvider{
        cfg:     cfg,
        pm:      pm,
        genkit:  g,
        adapter: adapter,
    }, nil
}

// GetSuggestion, GetEnhancedSuggestion, GenerateCommand, VerifyConnection
// all use adapter.Generate() or adapter.TestGeneration()
```

### Ollama Provider Implementation

**File**: `internal/llm/ollama/client.go`

```go
package ollama

import (
    "context"
    "fmt"
    "strings"
    "text/template"

    "github.com/TonnyWong1052/aish/internal/config"
    "github.com/TonnyWong1052/aish/internal/llm"
    "github.com/TonnyWong1052/aish/internal/prompt"
    "github.com/firebase/genkit/go/genkit"
    ollamaPlugin "github.com/firebase/genkit/go/plugins/ollama"
)

type OllamaProvider struct {
    cfg     config.ProviderConfig
    pm      *prompt.Manager
    genkit  *genkit.Genkit
    adapter *llm.GenkitAdapter
}

func NewProvider(cfg config.ProviderConfig, pm *prompt.Manager) (llm.Provider, error) {
    ctx := context.Background()

    // Initialize Genkit with Ollama plugin
    g := genkit.Init(ctx,
        genkit.WithPlugins(&ollamaPlugin.Ollama{
            ServerAddress: cfg.APIEndpoint, // http://localhost:11434
        }),
    )

    // Model name requires "ollama/" prefix
    modelName := "ollama/" + cfg.Model
    adapter := llm.NewGenkitAdapter(g, modelName)

    return &OllamaProvider{
        cfg:     cfg,
        pm:      pm,
        genkit:  g,
        adapter: adapter,
    }, nil
}

// Same interface implementation as Claude provider
```

## Provider Setup

### Claude (Anthropic)

1. **Install**: No additional installation needed (included in Genkit Go)
2. **Configure**:
   ```bash
   aish config set default_provider claude
   aish config set providers.claude.api_key YOUR_ANTHROPIC_API_KEY
   aish config set providers.claude.model claude-3-5-sonnet-20241022
   ```
3. **Available Models**:
   - `claude-3-5-sonnet-20241022` (recommended)
   - `claude-3-5-haiku-20241022`
   - `claude-3-opus-20240229`

### Ollama (Local LLMs)

1. **Install Ollama**:
   ```bash
   # macOS
   brew install ollama

   # Linux
   curl -fsSL https://ollama.com/install.sh | sh

   # Windows
   # Download from https://ollama.com/download
   ```

2. **Pull Models**:
   ```bash
   ollama pull llama3.3
   ollama pull codellama
   ollama pull mistral
   ```

3. **Configure AISH**:
   ```bash
   aish config set default_provider ollama
   aish config set providers.ollama.model llama3.3
   # No API key needed for Ollama
   ```

4. **Start Ollama Server** (if not running):
   ```bash
   ollama serve
   ```

## Usage Examples

### Basic Command Generation

```bash
# Using Claude
aish -p "list all files sorted by size"

# Using Ollama
aish --provider ollama -p "create a backup of my home directory"
```

### Error Analysis

```bash
# Let AISH capture the error automatically
unknowncmd --invalid-flag

# AISH will:
# 1. Capture the error via shell hook
# 2. Send context to configured LLM (Claude/Ollama via Genkit)
# 3. Display explanation and corrected command
```

### Configuration Verification

```bash
# Check current provider
aish config show

# Test provider connection
aish --debug -p "hello"
```

## Migration Guide

### From Direct HTTP Client to Genkit

**Before** (Direct HTTP implementation):
```go
func (p *ClaudeProvider) GetSuggestion(ctx context.Context, ...) (*llm.Suggestion, error) {
    // Manual HTTP request construction
    reqBody, _ := json.Marshal(map[string]interface{}{
        "model": p.cfg.Model,
        "messages": []map[string]string{
            {"role": "user", "content": prompt},
        },
    })

    req, _ := http.NewRequestWithContext(ctx, "POST", p.cfg.APIEndpoint+"/messages", bytes.NewReader(reqBody))
    req.Header.Set("x-api-key", p.cfg.APIKey)
    req.Header.Set("anthropic-version", "2023-06-01")

    resp, err := p.client.Do(req)
    // ... manual response parsing
}
```

**After** (Genkit integration):
```go
func (p *ClaudeProvider) GetSuggestion(ctx context.Context, ...) (*llm.Suggestion, error) {
    // Simple adapter call
    response, err := p.adapter.Generate(ctx, prompt)
    if err != nil {
        return nil, fmt.Errorf("Claude generation failed: %w", err)
    }
    return parseSuggestionResponse(response)
}
```

### Backward Compatibility

All existing `llm.Provider` interface methods remain unchanged:
- ✅ `GetSuggestion()`
- ✅ `GetEnhancedSuggestion()`
- ✅ `GenerateCommand()`
- ✅ `VerifyConnection()`

No changes required in calling code!

## Future Extensions

### Adding New Genkit Provider

1. **Check Genkit Plugin Availability**: Visit [Genkit Plugins](https://firebase.google.com/docs/genkit/plugins)

2. **Install Plugin** (if available):
   ```bash
   go get github.com/firebase/genkit/go/plugins/[provider]
   ```

3. **Implement Provider**:
   ```go
   package newprovider

   import (
       "github.com/firebase/genkit/go/genkit"
       newPlugin "github.com/firebase/genkit/go/plugins/newprovider"
   )

   func NewProvider(cfg config.ProviderConfig, pm *prompt.Manager) (llm.Provider, error) {
       ctx := context.Background()

       g := genkit.Init(ctx,
           genkit.WithPlugins(&newPlugin.NewProvider{
               // Plugin configuration
           }),
       )

       modelName := "newprovider/" + cfg.Model
       adapter := llm.NewGenkitAdapter(g, modelName)

       return &NewProviderStruct{
           cfg:     cfg,
           pm:      pm,
           genkit:  g,
           adapter: adapter,
       }, nil
   }
   ```

4. **Register Provider**:
   ```go
   func init() {
       llm.RegisterProvider("newprovider", NewProvider)
   }
   ```

### Potential Providers to Add

- **Google AI Studio**: via `googleai` plugin
- **Vertex AI**: via `vertexai` plugin
- **Cohere**: via `cohere` plugin (if available)
- **Mistral AI**: via custom plugin

### Advanced Genkit Features

#### Structured Output Example

```go
type CommandSuggestion struct {
    Command     string `json:"command"`
    Explanation string `json:"explanation"`
    SafetyLevel string `json:"safety_level"`
}

func (p *ClaudeProvider) GetStructuredSuggestion(ctx context.Context, prompt string) (*CommandSuggestion, error) {
    result, err := llm.GenerateStructured[CommandSuggestion](ctx, p.adapter, prompt)
    if err != nil {
        return nil, err
    }
    return result, nil
}
```

#### Telemetry Integration

```go
import "github.com/firebase/genkit/go/plugins/opentelemetry"

g := genkit.Init(ctx,
    genkit.WithPlugins(
        &anthropicPlugin.Anthropic{...},
        &opentelemetry.Plugin{
            MetricExporter: /* your exporter */,
            TraceExporter:  /* your exporter */,
        },
    ),
)
```

## Dependencies

### Go Modules

```go
require (
    github.com/firebase/genkit/go v1.0.5
    github.com/openai/openai-go v1.12.0  // Required by Anthropic plugin
)
```

### Plugin Dependencies (Included in Genkit Core)

- `github.com/firebase/genkit/go/plugins/compat_oai/anthropic`
- `github.com/firebase/genkit/go/plugins/ollama`

## Troubleshooting

### Common Issues

1. **Import Errors**:
   ```bash
   go get github.com/firebase/genkit/go@v1.0.5
   go mod tidy
   ```

2. **Model Name Issues**:
   - Ensure model names include provider prefix: `anthropic/` or `ollama/`
   - Example: `"anthropic/claude-3-5-sonnet-20241022"`

3. **Ollama Connection Failed**:
   ```bash
   # Check if Ollama is running
   curl http://localhost:11434/api/tags

   # Start Ollama if needed
   ollama serve
   ```

4. **Claude Authentication Failed**:
   ```bash
   # Verify API key is set
   aish config show

   # Set API key if missing
   aish config set providers.claude.api_key YOUR_KEY
   ```

## References

- **Genkit Go Documentation**: https://firebase.google.com/docs/genkit/go/get-started-go
- **Genkit GitHub**: https://github.com/firebase/genkit/tree/main/go
- **AISH Project**: https://github.com/TonnyWong1052/aish
- **Ollama**: https://ollama.com
- **Anthropic API**: https://docs.anthropic.com/claude/reference/getting-started-with-the-api

---

**Last Updated**: 2025-10-04
**Genkit Version**: v1.0.5
**Author**: AISH Development Team
