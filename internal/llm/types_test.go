package llm

import (
	"context"
	"fmt"
	"testing"

	"github.com/TonnyWong1052/aish/internal/config"
	"github.com/TonnyWong1052/aish/internal/prompt"
)

func TestSuggestion(t *testing.T) {
	testCases := []struct {
		name       string
		suggestion Suggestion
		expected   Suggestion
	}{
		{
			name: "Complete suggestion",
			suggestion: Suggestion{
				Explanation:      "This command lists files in long format",
				CorrectedCommand: "ls -la",
			},
			expected: Suggestion{
				Explanation:      "This command lists files in long format",
				CorrectedCommand: "ls -la",
			},
		},
		{
			name: "Empty suggestion",
			suggestion: Suggestion{
				Explanation:      "",
				CorrectedCommand: "",
			},
			expected: Suggestion{
				Explanation:      "",
				CorrectedCommand: "",
			},
		},
		{
			name: "Command only",
			suggestion: Suggestion{
				Explanation:      "",
				CorrectedCommand: "pwd",
			},
			expected: Suggestion{
				Explanation:      "",
				CorrectedCommand: "pwd",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.suggestion.Explanation != tc.expected.Explanation {
				t.Errorf("Expected explanation '%s', got '%s'", tc.expected.Explanation, tc.suggestion.Explanation)
			}
			if tc.suggestion.CorrectedCommand != tc.expected.CorrectedCommand {
				t.Errorf("Expected corrected command '%s', got '%s'", tc.expected.CorrectedCommand, tc.suggestion.CorrectedCommand)
			}
		})
	}
}

func TestCapturedContext(t *testing.T) {
	testCases := []struct {
		name    string
		context CapturedContext
	}{
		{
			name: "Successful command",
			context: CapturedContext{
				Command:  "ls -la",
				Stdout:   "total 16\ndrwxr-xr-x  4 user user 128 Jan 1 12:00 .\n",
				Stderr:   "",
				ExitCode: 0,
			},
		},
		{
			name: "Failed command",
			context: CapturedContext{
				Command:  "cat /nonexistent",
				Stdout:   "",
				Stderr:   "cat: /nonexistent: No such file or directory",
				ExitCode: 1,
			},
		},
		{
			name: "Command not found",
			context: CapturedContext{
				Command:  "unknowncmd",
				Stdout:   "",
				Stderr:   "unknowncmd: command not found",
				ExitCode: 127,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.context

			if ctx.Command != tc.context.Command {
				t.Errorf("Expected command '%s', got '%s'", tc.context.Command, ctx.Command)
			}
			if ctx.Stdout != tc.context.Stdout {
				t.Errorf("Expected stdout '%s', got '%s'", tc.context.Stdout, ctx.Stdout)
			}
			if ctx.Stderr != tc.context.Stderr {
				t.Errorf("Expected stderr '%s', got '%s'", tc.context.Stderr, ctx.Stderr)
			}
			if ctx.ExitCode != tc.context.ExitCode {
				t.Errorf("Expected exit code %d, got %d", tc.context.ExitCode, ctx.ExitCode)
			}
		})
	}
}

func TestEnhancedCapturedContext(t *testing.T) {
	baseContext := CapturedContext{
		Command:  "ls missing",
		Stdout:   "",
		Stderr:   "ls: missing: No such file or directory",
		ExitCode: 2,
	}

	enhancedContext := EnhancedCapturedContext{
		CapturedContext:  baseContext,
		RecentCommands:   []string{"pwd", "cd /tmp", "ls"},
		DirectoryListing: []string{"file1.txt", "file2.log", "subdir/"},
		WorkingDirectory: "/tmp",
		ShellType:        "bash",
	}

	// Test embedded structure
	if enhancedContext.Command != baseContext.Command {
		t.Errorf("Expected command '%s', got '%s'", baseContext.Command, enhancedContext.Command)
	}
	if enhancedContext.ExitCode != baseContext.ExitCode {
		t.Errorf("Expected exit code %d, got %d", baseContext.ExitCode, enhancedContext.ExitCode)
	}

	// Test enhanced fields
	if len(enhancedContext.RecentCommands) != 3 {
		t.Errorf("Expected 3 recent commands, got %d", len(enhancedContext.RecentCommands))
	}
	if enhancedContext.WorkingDirectory != "/tmp" {
		t.Errorf("Expected working directory '/tmp', got '%s'", enhancedContext.WorkingDirectory)
	}
	if enhancedContext.ShellType != "bash" {
		t.Errorf("Expected shell type 'bash', got '%s'", enhancedContext.ShellType)
	}
}

// MockProvider implements Provider interface for testing
type MockProvider struct {
	suggestion    *Suggestion
	command       string
	models        []string
	suggestionErr error
	commandErr    error
	connectionErr error
}

func (m *MockProvider) GetSuggestion(ctx context.Context, capturedCtx CapturedContext, language string) (*Suggestion, error) {
	return m.suggestion, m.suggestionErr
}

func (m *MockProvider) GetEnhancedSuggestion(ctx context.Context, enhancedCtx EnhancedCapturedContext, language string) (*Suggestion, error) {
	return m.suggestion, m.suggestionErr
}

func (m *MockProvider) GenerateCommand(ctx context.Context, prompt string, language string) (string, error) {
	return m.command, m.commandErr
}

func (m *MockProvider) VerifyConnection(ctx context.Context) ([]string, error) {
	return m.models, m.connectionErr
}

func TestProvider(t *testing.T) {
	// Test MockProvider implementation
	mockProvider := &MockProvider{
		suggestion: &Suggestion{
			Explanation:      "Test explanation",
			CorrectedCommand: "test command",
		},
		command:       "generated command",
		models:        []string{"model1", "model2"},
		suggestionErr: nil,
		commandErr:    nil,
		connectionErr: nil,
	}

	ctx := context.Background()

	// Test GetSuggestion
	capturedCtx := CapturedContext{
		Command:  "test",
		ExitCode: 1,
	}
	suggestion, err := mockProvider.GetSuggestion(ctx, capturedCtx, "en")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if suggestion.Explanation != "Test explanation" {
		t.Errorf("Expected 'Test explanation', got '%s'", suggestion.Explanation)
	}

	// Test GetEnhancedSuggestion
	enhancedCtx := EnhancedCapturedContext{
		CapturedContext: capturedCtx,
		ShellType:       "bash",
	}
	enhancedSuggestion, err := mockProvider.GetEnhancedSuggestion(ctx, enhancedCtx, "en")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if enhancedSuggestion.CorrectedCommand != "test command" {
		t.Errorf("Expected 'test command', got '%s'", enhancedSuggestion.CorrectedCommand)
	}

	// Test GenerateCommand
	command, err := mockProvider.GenerateCommand(ctx, "test prompt", "en")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if command != "generated command" {
		t.Errorf("Expected 'generated command', got '%s'", command)
	}

	// Test VerifyConnection
	models, err := mockProvider.VerifyConnection(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}
}

func TestProviderRegistry(t *testing.T) {
	// Test provider registration
	testFactoryName := "test-provider"
	testFactory := func(cfg config.ProviderConfig, pm *prompt.Manager) (Provider, error) {
		return &MockProvider{}, nil
	}

	// Register provider
	RegisterProvider(testFactoryName, testFactory)

	// Test getting registered provider
	provider, err := GetProvider(testFactoryName, config.ProviderConfig{}, nil)
	if err != nil {
		t.Errorf("Expected no error getting registered provider, got %v", err)
	}
	if provider == nil {
		t.Error("Expected non-nil provider")
	}

	// Test getting unknown provider
	_, err = GetProvider("unknown-provider", config.ProviderConfig{}, nil)
	if err == nil {
		t.Error("Expected error for unknown provider")
	}
	expectedError := "unknown provider: unknown-provider"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestProviderRegistryMultiple(t *testing.T) {
	// Test registering multiple providers
	providers := []string{"provider1", "provider2", "provider3"}

	for _, name := range providers {
		RegisterProvider(name, func(cfg config.ProviderConfig, pm *prompt.Manager) (Provider, error) {
			return &MockProvider{}, nil
		})
	}

	// Test all providers can be retrieved
	for _, name := range providers {
		provider, err := GetProvider(name, config.ProviderConfig{}, nil)
		if err != nil {
			t.Errorf("Expected no error getting provider '%s', got %v", name, err)
		}
		if provider == nil {
			t.Errorf("Expected non-nil provider for '%s'", name)
		}
	}
}

func TestProviderWithError(t *testing.T) {
	// Test provider factory that returns error
	errorProviderName := "error-provider"
	expectedError := "factory error"

	RegisterProvider(errorProviderName, func(cfg config.ProviderConfig, pm *prompt.Manager) (Provider, error) {
		return nil, fmt.Errorf("%s", expectedError)
	})

	provider, err := GetProvider(errorProviderName, config.ProviderConfig{}, nil)
	if err == nil {
		t.Error("Expected error from provider factory")
	}
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
	if provider != nil {
		t.Error("Expected nil provider when factory returns error")
	}
}
