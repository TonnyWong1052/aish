# AISH API Documentation

This document provides comprehensive API documentation for developers who want to integrate with AISH or extend its functionality.

## Table of Contents

- [Core Interfaces](#core-interfaces)
- [LLM Provider API](#llm-provider-api)
- [Configuration API](#configuration-api)
- [Cache API](#cache-api)
- [Security API](#security-api)
- [Integration Examples](#integration-examples)

## Core Interfaces

### Provider Interface

The `Provider` interface is the core abstraction for all LLM providers in AISH.

```go
package llm

import "context"

type Provider interface {
    // GetSuggestion analyzes captured command context and returns AI suggestions
    GetSuggestion(ctx context.Context, capturedContext CapturedContext, lang string) (*Suggestion, error)

    // GetEnhancedSuggestion provides advanced analysis with additional context
    GetEnhancedSuggestion(ctx context.Context, enhancedCtx EnhancedCapturedContext, lang string) (*Suggestion, error)

    // GenerateCommand creates shell commands from natural language prompts
    GenerateCommand(ctx context.Context, promptText string, lang string) (string, error)

    // VerifyConnection tests provider connectivity and returns available models
    VerifyConnection(ctx context.Context) ([]string, error)
}
```

### Data Structures

#### CapturedContext
```go
type CapturedContext struct {
    Command  string `json:"command"`
    Stdout   string `json:"stdout"`
    Stderr   string `json:"stderr"`
    ExitCode int    `json:"exit_code"`
}
```

#### EnhancedCapturedContext
```go
type EnhancedCapturedContext struct {
    CapturedContext
    WorkingDirectory string            `json:"working_directory"`
    Environment      map[string]string `json:"environment"`
    SystemInfo       SystemInfo        `json:"system_info"`
    History          []HistoryEntry    `json:"history"`
}
```

#### Suggestion
```go
type Suggestion struct {
    Explanation      string `json:"explanation"`
    CorrectedCommand string `json:"corrected_command"`
    Confidence       int    `json:"confidence,omitempty"`
    Category         string `json:"category,omitempty"`
}
```

## LLM Provider API

### Creating a New Provider

To create a new LLM provider, implement the `Provider` interface and register it with the system:

```go
package myprovider

import (
    "context"
    "github.com/TonnyWong1052/aish/internal/llm"
    "github.com/TonnyWong1052/aish/internal/config"
    "github.com/TonnyWong1052/aish/internal/prompt"
)

type MyProvider struct {
    cfg config.ProviderConfig
    pm  *prompt.Manager
}

func NewProvider(cfg config.ProviderConfig, pm *prompt.Manager) (llm.Provider, error) {
    return &MyProvider{
        cfg: cfg,
        pm:  pm,
    }, nil
}

func (p *MyProvider) GetSuggestion(ctx context.Context, capturedContext llm.CapturedContext, lang string) (*llm.Suggestion, error) {
    // Implementation details
    return &llm.Suggestion{
        Explanation:      "Error analysis...",
        CorrectedCommand: "corrected command",
    }, nil
}

// Register the provider
func init() {
    llm.RegisterProvider("myprovider", NewProvider)
}
```

### Provider Registration

```go
// Register your provider
llm.RegisterProvider("provider-name", NewProviderFunc)

// Get a registered provider
provider, err := llm.GetProvider("provider-name", config, promptManager)
```

### Error Handling

Providers should return structured errors that can be handled gracefully:

```go
import "github.com/TonnyWong1052/aish/internal/errors"

// Use structured error types
if rateLimited {
    return nil, errors.NewRateLimitError("Too many requests", 60) // 60 second retry
}

if authFailed {
    return nil, errors.NewAuthenticationError("Invalid API key")
}

if networkError {
    return nil, errors.NewNetworkError("Connection failed", err)
}
```

## Configuration API

### Configuration Structure

```go
type Config struct {
    DefaultProvider   string                       `json:"default_provider"`
    Providers         map[string]ProviderConfig    `json:"providers"`
    UserPreferences   UserPreferences              `json:"user_preferences"`
    Security          SecurityConfig               `json:"security"`
    Cache             CacheConfig                  `json:"cache"`
}

type ProviderConfig struct {
    APIKey      string `json:"api_key,omitempty"`
    APIEndpoint string `json:"api_endpoint,omitempty"`
    Model       string `json:"model,omitempty"`
    Project     string `json:"project,omitempty"`
}
```

### Configuration Management

```go
import "github.com/TonnyWong1052/aish/internal/config"

// Load configuration
cfg, err := config.Load()
if err != nil {
    // Handle error
}

// Save configuration
err = config.Save(cfg)
if err != nil {
    // Handle error
}

// Get provider configuration
providerCfg, exists := cfg.Providers["openai"]
if !exists {
    // Provider not configured
}
```

### Secure Configuration

For handling sensitive data like API keys:

```go
import "github.com/TonnyWong1052/aish/internal/config"

// Create secure config manager
secureConfig, err := config.NewSecureConfig()
if err != nil {
    // Handle error
}

// Set encrypted API key
err = secureConfig.SetAPIKey("openai", "your-api-key")
if err != nil {
    // Handle error
}

// Get decrypted API key
apiKey, err := secureConfig.GetDecryptedAPIKey("openai")
if err != nil {
    // Handle error
}
```

## Cache API

### Cache Interface

```go
type Cache interface {
    Get(key string) (interface{}, bool)
    Set(key string, value interface{}, ttl time.Duration) error
    Delete(key string) error
    Clear() error
    Stats() CacheStats
}
```

### Using the Cache

```go
import "github.com/TonnyWong1052/aish/internal/cache"

// Create cache instance
cache := cache.NewCache(cache.Config{
    MaxSize: 100,
    TTL:     time.Hour,
})

// Store data
err := cache.Set("key", "value", time.Minute*30)

// Retrieve data
value, exists := cache.Get("key")
if exists {
    // Use cached value
}
```

### Intelligent Cache

For semantic caching:

```go
import "github.com/TonnyWong1052/aish/internal/cache"

// Create intelligent cache
intelligentCache := cache.NewIntelligentCache(cache.IntelligentCacheConfig{
    MaxSize:               1000,
    SemanticThreshold:     0.8,
    EnablePrewarming:      true,
    PrewarmingConcurrency: 5,
})

// Cache with semantic key
err := intelligentCache.SetSemantic("error context", suggestion, time.Hour)

// Retrieve with similarity matching
suggestion, similarity, found := intelligentCache.GetSemantic("similar error context", 0.7)
```

## Security API

### Data Sanitization

```go
import "github.com/TonnyWong1052/aish/internal/security"

// Create sanitizer
sanitizer := security.NewSensitiveDataSanitizer()

// Sanitize text
cleanText := sanitizer.Sanitize("curl -H 'Authorization: Bearer sk-123' https://api.example.com")
// Result: "curl -H 'Authorization: Bearer ***REDACTED_TOKEN***' https://api.example.com"

// Check for sensitive data
hasSensitive := sanitizer.ContainsSensitiveData(input)

// Get matched patterns
patterns := sanitizer.GetMatchedPatterns(input)
```

### Custom Sanitization Patterns

```go
// Add custom pattern
err := sanitizer.AddPattern(
    "custom_token",
    `(?i)(mytoken[\s=])([a-zA-Z0-9]{20,})`,
    "$1***REDACTED***",
    8, // priority
)

// Enable/disable patterns
sanitizer.EnablePattern("custom_token")
sanitizer.DisablePattern("email")
```

### Encryption

```go
import "github.com/TonnyWong1052/aish/internal/crypto"

// Create secret manager
secretManager, err := crypto.NewSecretManager("/path/to/config")
if err != nil {
    // Handle error
}

// Encrypt sensitive data
encrypted, err := secretManager.EncryptString("sensitive-data")
if err != nil {
    // Handle error
}

// Decrypt data
decrypted, err := secretManager.DecryptString(encrypted)
if err != nil {
    // Handle error
}
```

## Integration Examples

### Custom Provider Integration

Here's a complete example of integrating a custom AI provider:

```go
package customprovider

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "text/template"

    "github.com/TonnyWong1052/aish/internal/llm"
    "github.com/TonnyWong1052/aish/internal/config"
    "github.com/TonnyWong1052/aish/internal/prompt"
)

type CustomProvider struct {
    cfg    config.ProviderConfig
    pm     *prompt.Manager
    client *http.Client
}

func NewProvider(cfg config.ProviderConfig, pm *prompt.Manager) (llm.Provider, error) {
    return &CustomProvider{
        cfg:    cfg,
        pm:     pm,
        client: &http.Client{Timeout: 30 * time.Second},
    }, nil
}

func (p *CustomProvider) GetSuggestion(ctx context.Context, capturedContext llm.CapturedContext, lang string) (*llm.Suggestion, error) {
    // Get prompt template
    promptTemplate, err := p.pm.GetPrompt("get_suggestion", lang)
    if err != nil {
        return nil, fmt.Errorf("failed to get prompt template: %w", err)
    }

    // Execute template
    var promptBuffer bytes.Buffer
    tmpl := template.Must(template.New("prompt").Parse(promptTemplate))
    if err := tmpl.Execute(&promptBuffer, capturedContext); err != nil {
        return nil, fmt.Errorf("failed to execute template: %w", err)
    }

    // Make API request
    requestBody := map[string]interface{}{
        "prompt": promptBuffer.String(),
        "model":  p.cfg.Model,
    }

    jsonBody, err := json.Marshal(requestBody)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", p.cfg.APIEndpoint, bytes.NewReader(jsonBody))
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
    }

    // Parse response
    var response struct {
        Explanation      string `json:"explanation"`
        CorrectedCommand string `json:"corrected_command"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &llm.Suggestion{
        Explanation:      response.Explanation,
        CorrectedCommand: response.CorrectedCommand,
    }, nil
}

func (p *CustomProvider) GenerateCommand(ctx context.Context, promptText string, lang string) (string, error) {
    // Similar implementation for command generation
    return "generated command", nil
}

func (p *CustomProvider) VerifyConnection(ctx context.Context) ([]string, error) {
    // Test API connectivity
    req, err := http.NewRequestWithContext(ctx, "GET", p.cfg.APIEndpoint+"/models", nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("verification failed with status %d", resp.StatusCode)
    }

    return []string{"custom-model-1", "custom-model-2"}, nil
}

// Register the provider
func init() {
    llm.RegisterProvider("custom", NewProvider)
}
```

### Error Classification Extension

```go
package classification

import (
    "strings"
    "github.com/TonnyWong1052/aish/internal/classification"
)

// Custom error classifier
type CustomClassifier struct {
    patterns map[string][]string
}

func NewCustomClassifier() *CustomClassifier {
    return &CustomClassifier{
        patterns: map[string][]string{
            "DatabaseError": {
                "connection refused",
                "database connection failed",
                "table doesn't exist",
            },
            "NetworkError": {
                "network unreachable",
                "timeout",
                "connection reset",
            },
        },
    }
}

func (c *CustomClassifier) Classify(exitCode int, stdout, stderr string) classification.ErrorType {
    output := strings.ToLower(stdout + " " + stderr)

    for errorType, patterns := range c.patterns {
        for _, pattern := range patterns {
            if strings.Contains(output, pattern) {
                return classification.ErrorType(errorType)
            }
        }
    }

    return classification.GenericError
}

// Register custom classifier
func init() {
    classification.RegisterClassifier("custom", NewCustomClassifier)
}
```

### Plugin System Integration

```go
package plugin

import (
    "context"
    "plugin"

    "github.com/TonnyWong1052/aish/internal/llm"
)

// Plugin interface
type PluginProvider interface {
    Name() string
    Version() string
    Initialize(config map[string]interface{}) error
    GetProvider() llm.Provider
}

// Plugin loader
type PluginLoader struct {
    plugins map[string]PluginProvider
}

func NewPluginLoader() *PluginLoader {
    return &PluginLoader{
        plugins: make(map[string]PluginProvider),
    }
}

func (l *PluginLoader) LoadPlugin(path string) error {
    p, err := plugin.Open(path)
    if err != nil {
        return err
    }

    symbol, err := p.Lookup("NewPlugin")
    if err != nil {
        return err
    }

    newPlugin, ok := symbol.(func() PluginProvider)
    if !ok {
        return fmt.Errorf("invalid plugin interface")
    }

    pluginProvider := newPlugin()
    l.plugins[pluginProvider.Name()] = pluginProvider

    return nil
}

func (l *PluginLoader) GetPlugin(name string) (PluginProvider, bool) {
    plugin, exists := l.plugins[name]
    return plugin, exists
}
```

## Testing APIs

### Provider Testing

```go
package tests

import (
    "context"
    "testing"

    "github.com/TonnyWong1052/aish/internal/llm"
    "github.com/TonnyWong1052/aish/internal/config"
    "github.com/TonnyWong1052/aish/internal/prompt"
)

func TestProviderSuggestion(t *testing.T) {
    // Create test provider
    cfg := config.ProviderConfig{
        APIKey: "test-key",
        Model:  "test-model",
    }

    pm := prompt.NewDefaultManager()
    provider, err := NewTestProvider(cfg, pm)
    if err != nil {
        t.Fatalf("Failed to create provider: %v", err)
    }

    // Test suggestion
    ctx := context.Background()
    capturedContext := llm.CapturedContext{
        Command:  "ls /nonexistent",
        Stderr:   "ls: /nonexistent: No such file or directory",
        ExitCode: 1,
    }

    suggestion, err := provider.GetSuggestion(ctx, capturedContext, "en")
    if err != nil {
        t.Fatalf("GetSuggestion failed: %v", err)
    }

    if suggestion.Explanation == "" {
        t.Error("Expected non-empty explanation")
    }

    if suggestion.CorrectedCommand == "" {
        t.Error("Expected non-empty corrected command")
    }
}
```

### Mock Provider

```go
package mocks

import (
    "context"
    "github.com/TonnyWong1052/aish/internal/llm"
)

type MockProvider struct {
    suggestions map[string]*llm.Suggestion
    commands    map[string]string
}

func NewMockProvider() *MockProvider {
    return &MockProvider{
        suggestions: make(map[string]*llm.Suggestion),
        commands:    make(map[string]string),
    }
}

func (m *MockProvider) AddSuggestion(key string, suggestion *llm.Suggestion) {
    m.suggestions[key] = suggestion
}

func (m *MockProvider) AddCommand(prompt, command string) {
    m.commands[prompt] = command
}

func (m *MockProvider) GetSuggestion(ctx context.Context, capturedContext llm.CapturedContext, lang string) (*llm.Suggestion, error) {
    if suggestion, exists := m.suggestions[capturedContext.Command]; exists {
        return suggestion, nil
    }

    return &llm.Suggestion{
        Explanation:      "Mock explanation",
        CorrectedCommand: "mock command",
    }, nil
}

func (m *MockProvider) GenerateCommand(ctx context.Context, promptText string, lang string) (string, error) {
    if command, exists := m.commands[promptText]; exists {
        return command, nil
    }

    return "mock generated command", nil
}

func (m *MockProvider) VerifyConnection(ctx context.Context) ([]string, error) {
    return []string{"mock-model"}, nil
}
```

## Best Practices

### 1. Error Handling
- Always return structured errors
- Use context for timeouts and cancellation
- Implement proper retry mechanisms
- Log errors with sufficient context

### 2. Performance
- Use connection pooling for HTTP clients
- Implement caching where appropriate
- Use streaming for large responses
- Monitor resource usage

### 3. Security
- Never log sensitive data
- Use secure defaults
- Validate all inputs
- Implement rate limiting

### 4. Testing
- Write comprehensive unit tests
- Use mocks for external dependencies
- Test error conditions
- Perform integration testing

### 5. Documentation
- Document all public APIs
- Provide usage examples
- Keep documentation up to date
- Include performance characteristics

This API documentation provides the foundation for extending AISH and integrating it with other systems. For more specific examples or integration questions, please refer to the source code or open an issue on the project repository.