package llm

import (
	"context"
	"fmt"
	"github.com/TonnyWong1052/aish/internal/config"
	"github.com/TonnyWong1052/aish/internal/prompt"
)

// Suggestion represents a suggestion provided by LLM
type Suggestion struct {
	Explanation      string `json:"explanation"`      // Error explanation
	CorrectedCommand string `json:"correctedCommand"` // Corrected command
}

// CapturedContext represents captured command context
type CapturedContext struct {
	Command  string `json:"command"`  // Executed command
	Stdout   string `json:"stdout"`   // Standard output
	Stderr   string `json:"stderr"`   // Standard error
	ExitCode int    `json:"exitCode"` // Exit code
}

// EnhancedCapturedContext represents enhanced command context with more background information
type EnhancedCapturedContext struct {
	CapturedContext           // Embed original structure
	RecentCommands   []string `json:"recentCommands"`   // Recent command history
	DirectoryListing []string `json:"directoryListing"` // Current directory file listing
	WorkingDirectory string   `json:"workingDirectory"` // Current working directory
	ShellType        string   `json:"shellType"`        // Shell type (bash/zsh)
}

// Provider represents LLM provider interface
type Provider interface {
	// GetSuggestion gets suggestion based on captured context
	GetSuggestion(ctx context.Context, capturedCtx CapturedContext, language string) (*Suggestion, error)

	// GetEnhancedSuggestion gets suggestion based on enhanced context
	GetEnhancedSuggestion(ctx context.Context, enhancedCtx EnhancedCapturedContext, language string) (*Suggestion, error)

	// GenerateCommand generates command from natural language prompt
	GenerateCommand(ctx context.Context, prompt string, language string) (string, error)

	// VerifyConnection verifies connection and gets available models
	VerifyConnection(ctx context.Context) ([]string, error)
}

// ProviderFactory is a function that creates a new Provider
type ProviderFactory func(config.ProviderConfig, *prompt.Manager) (Provider, error)

var providerFactories = make(map[string]ProviderFactory)

// RegisterProvider makes provider available by name
func RegisterProvider(name string, factory ProviderFactory) {
	providerFactories[name] = factory
}

// GetProvider creates a new provider by name
func GetProvider(name string, cfg config.ProviderConfig, pm *prompt.Manager) (Provider, error) {
	factory, ok := providerFactories[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
	return factory(cfg, pm)
}
