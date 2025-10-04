package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/TonnyWong1052/aish/internal/config"
	"github.com/TonnyWong1052/aish/internal/llm"
	"github.com/TonnyWong1052/aish/internal/prompt"
)

func createTestProvider(t *testing.T, serverURL string) (*OpenAIProvider, error) {
	t.Helper()

	cfg := config.ProviderConfig{
		APIEndpoint: serverURL,
		APIKey:      "test-api-key",
		Model:       "gpt-4",
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	promptsPath := filepath.Join(wd, "..", "..", "prompts")

	pm, err := prompt.NewManager(promptsPath)
	if err != nil {
		return nil, err
	}

	provider, err := NewProvider(cfg, pm)
	if err != nil {
		return nil, err
	}

	return provider.(*OpenAIProvider), nil
}

func TestNewProvider(t *testing.T) {
	provider, err := createTestProvider(t, "https://api.openai.com/v1")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if provider == nil {
		t.Fatal("Provider should not be nil")
	}

	if provider.cfg.APIKey != "test-api-key" {
		t.Errorf("Expected API key 'test-api-key', got '%s'", provider.cfg.APIKey)
	}

	if provider.cfg.Model != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got '%s'", provider.cfg.Model)
	}
}

func TestMapLanguage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"english", "en"},
		{"en", "en"},
		{"chinese", "zh-TW"},
		{"zh", "zh-TW"},
		{"中文", "zh-TW"},
		{"fr", "en"}, // default
		{"", "en"},   // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapLanguage(tt.input)
			if result != tt.expected {
				t.Errorf("mapLanguage(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestVerifyConnection_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			response := ModelsResponse{
				Object: "list",
				Data: []struct {
					ID      string `json:"id"`
					Object  string `json:"object"`
					Created int64  `json:"created"`
					OwnedBy string `json:"owned_by"`
				}{
					{ID: "gpt-4", Object: "model", Created: 1234567890, OwnedBy: "openai"},
					{ID: "gpt-3.5-turbo", Object: "model", Created: 1234567891, OwnedBy: "openai"},
				},
			}
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer mockServer.Close()

	provider, err := createTestProvider(t, mockServer.URL)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	models, err := provider.VerifyConnection(context.Background())
	if err != nil {
		t.Fatalf("VerifyConnection failed: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}

	if models[0] != "gpt-4" {
		t.Errorf("Expected first model 'gpt-4', got '%s'", models[0])
	}
}

func TestVerifyConnection_MissingAPIKey(t *testing.T) {
	cfg := config.ProviderConfig{
		APIEndpoint: "https://api.openai.com/v1",
		APIKey:      "",
		Model:       "gpt-4",
	}

	wd, _ := os.Getwd()
	promptsPath := filepath.Join(wd, "..", "..", "prompts")
	pm, _ := prompt.NewManager(promptsPath)

	provider, _ := NewProvider(cfg, pm)
	openaiProvider := provider.(*OpenAIProvider)

	_, err := openaiProvider.VerifyConnection(context.Background())
	if err == nil {
		t.Error("Expected error with missing API key")
	}
	if err != nil && err.Error() != "API key is missing for OpenAI" {
		t.Errorf("Expected 'API key is missing for OpenAI' error, got: %v", err)
	}
}

func TestParseSuggestionResponse(t *testing.T) {
	provider, err := createTestProvider(t, "https://api.openai.com/v1")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	tests := []struct {
		name             string
		response         string
		wantExplanation  string
		wantCommand      string
	}{
		{
			name: "text response with markers",
			response: `Explanation: Use grep to search
Command: grep pattern file.txt`,
			wantExplanation:  "Use grep to search",
			wantCommand:      "grep pattern file.txt",
		},
		{
			name:             "empty response",
			response:         "",
			wantExplanation:  "請檢查命令語法和參數是否正確。", // Default message is in Chinese
			wantCommand:      "echo '無法自動修正命令，請手動檢查'", // Default message is in Chinese
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestion, err := provider.parseSuggestionResponse(tt.response)
			if err != nil {
				t.Fatalf("parseSuggestionResponse() error = %v", err)
			}

			if suggestion.Explanation != tt.wantExplanation {
				t.Errorf("Explanation = %q, want %q", suggestion.Explanation, tt.wantExplanation)
			}

			if suggestion.CorrectedCommand != tt.wantCommand {
				t.Errorf("CorrectedCommand = %q, want %q", suggestion.CorrectedCommand, tt.wantCommand)
			}
		})
	}
}

func TestResolveURL(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    string
		subpath     string
		expected    string
	}{
		{
			name:        "with /v1",
			endpoint:    "https://api.openai.com/v1",
			subpath:     "/chat/completions",
			expected:    "https://api.openai.com/v1/chat/completions",
		},
		{
			name:        "without /v1",
			endpoint:    "https://custom.api.com",
			subpath:     "/chat/completions",
			expected:    "https://custom.api.com/v1/chat/completions",
		},
		{
			name:        "trailing slash",
			endpoint:    "https://api.openai.com/v1/",
			subpath:     "/models",
			expected:    "https://api.openai.com/v1/models",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, _ := createTestProvider(t, tt.endpoint)
			result := provider.resolveURL(tt.subpath)
			if result != tt.expected {
				t.Errorf("resolveURL() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetSuggestion_TemplateError(t *testing.T) {
	provider, err := createTestProvider(t, "https://api.openai.com/v1")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	capturedContext := llm.CapturedContext{
		Command:  "ls",
		Stdout:   "",
		Stderr:   "command not found",
		ExitCode: 127,
	}

	// This will fail because it needs a real API connection
	_, err = provider.GetSuggestion(context.Background(), capturedContext, "en")
	// We expect an error since we don't have a real API connection
	if err == nil {
		t.Error("Expected error without real API connection")
	}
}
