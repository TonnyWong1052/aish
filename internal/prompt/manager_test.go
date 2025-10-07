package prompt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewManager(t *testing.T) {
	// Create temp file with test prompts
	tmpDir, err := os.MkdirTemp("", "prompt-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testPrompts := map[string]map[string]string{
		"test_prompt": {
			"en":    "English prompt",
			"zh-TW": "Traditional Chinese prompt",
		},
	}

	promptFile := filepath.Join(tmpDir, "prompts.json")
	data, err := json.Marshal(testPrompts)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(promptFile, data, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Test NewManager
	manager, err := NewManager(promptFile)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.prompts == nil {
		t.Error("Manager prompts map is nil")
	}

	if len(manager.prompts) != 1 {
		t.Errorf("Expected 1 prompt category, got %d", len(manager.prompts))
	}
}

func TestNewManagerNonExistentFile(t *testing.T) {
	_, err := NewManager("/nonexistent/prompts.json")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestNewManagerInvalidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt-invalid-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	promptFile := filepath.Join(tmpDir, "invalid.json")
	err = os.WriteFile(promptFile, []byte("{invalid json"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = NewManager(promptFile)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestNewDefaultManager(t *testing.T) {
	manager := NewDefaultManager()

	if manager == nil {
		t.Fatal("NewDefaultManager returned nil")
	}

	if manager.prompts == nil {
		t.Error("Default manager prompts map is nil")
	}

	// Check that default prompts exist
	expectedKeys := []string{"generate_command", "get_suggestion", "get_enhanced_suggestion"}
	for _, key := range expectedKeys {
		if _, ok := manager.prompts[key]; !ok {
			t.Errorf("Expected default prompt key '%s' not found", key)
		}
	}

	// Check that English prompts exist
	for _, key := range expectedKeys {
		prompt, err := manager.GetPrompt(key, "en")
		if err != nil {
			t.Errorf("Failed to get English prompt for '%s': %v", key, err)
		}
		if prompt == "" {
			t.Errorf("English prompt for '%s' is empty", key)
		}
	}
}

func TestGetPrompt(t *testing.T) {
	manager := NewDefaultManager()

	tests := []struct {
		name     string
		key      string
		lang     string
		wantErr  bool
		contains string
	}{
		{
			name:     "Valid English generate_command",
			key:      "generate_command",
			lang:     "en",
			wantErr:  false,
			contains: "shell command generator",
		},
		{
			name:     "Valid Chinese get_suggestion",
			key:      "get_suggestion",
			lang:     "zh-TW",
			wantErr:  false,
			contains: "除錯",
		},
		{
			name:     "Valid Japanese generate_command",
			key:      "generate_command",
			lang:     "japanese",
			wantErr:  false,
			contains: "シェルコマンド",
		},
		{
			name:    "Invalid prompt key",
			key:     "nonexistent_prompt",
			lang:    "en",
			wantErr: true,
		},
		{
			name:     "Unsupported language fallback to English",
			key:      "generate_command",
			lang:     "unsupported_lang",
			wantErr:  false,
			contains: "shell command generator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, err := manager.GetPrompt(tt.key, tt.lang)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("GetPrompt failed: %v", err)
			}

			if prompt == "" {
				t.Error("Prompt is empty")
			}

			if tt.contains != "" && !strings.Contains(prompt, tt.contains) {
				t.Errorf("Expected prompt to contain '%s', got: %s", tt.contains, prompt)
			}
		})
	}
}

func TestGetPromptAllLanguages(t *testing.T) {
	manager := NewDefaultManager()

	languages := []string{
		"en", "zh-TW", "zh-CN", "japanese", "korean",
		"spanish", "french", "german", "italian",
		"portuguese", "russian", "arabic",
	}

	for _, key := range []string{"generate_command", "get_suggestion", "get_enhanced_suggestion"} {
		for _, lang := range languages {
			t.Run(key+"_"+lang, func(t *testing.T) {
				prompt, err := manager.GetPrompt(key, lang)
				if err != nil {
					t.Errorf("Failed to get prompt for %s/%s: %v", key, lang, err)
				}
				if prompt == "" {
					t.Errorf("Empty prompt for %s/%s", key, lang)
				}
			})
		}
	}
}

func TestGetPromptTemplateVariables(t *testing.T) {
	manager := NewDefaultManager()

	tests := []struct {
		key       string
		variables []string
	}{
		{
			key:       "generate_command",
			variables: []string{"{{.Prompt}}"},
		},
		{
			key: "get_suggestion",
			variables: []string{
				"{{.Command}}",
				"{{.ExitCode}}",
				"{{.Stdout}}",
				"{{.Stderr}}",
			},
		},
		{
			key: "get_enhanced_suggestion",
			variables: []string{
				"{{.Command}}",
				"{{.WorkingDirectory}}",
				"{{.ShellType}}",
				// Note: Template uses {{range .DirectoryListing}} not {{.DirectoryListing}}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			prompt, err := manager.GetPrompt(tt.key, "en")
			if err != nil {
				t.Fatalf("GetPrompt failed: %v", err)
			}

			for _, variable := range tt.variables {
				if !strings.Contains(prompt, variable) {
					t.Errorf("Expected prompt to contain template variable '%s'", variable)
				}
			}
		})
	}
}

func TestPromptJSONSchema(t *testing.T) {
	manager := NewDefaultManager()

	// All prompts should mention JSON output
	for _, key := range []string{"generate_command", "get_suggestion", "get_enhanced_suggestion"} {
		prompt, err := manager.GetPrompt(key, "en")
		if err != nil {
			t.Fatalf("GetPrompt failed: %v", err)
		}

		// Check that prompts mention JSON
		if !strings.Contains(strings.ToLower(prompt), "json") {
			t.Errorf("Prompt '%s' should mention JSON output", key)
		}
	}
}

func TestGenerateCommandPromptSchema(t *testing.T) {
	manager := NewDefaultManager()
	prompt, err := manager.GetPrompt("generate_command", "en")
	if err != nil {
		t.Fatal(err)
	}

	// Should specify the command field in JSON schema
	if !strings.Contains(prompt, `"command"`) {
		t.Error("generate_command prompt should specify 'command' field")
	}
}

func TestGetSuggestionPromptSchema(t *testing.T) {
	manager := NewDefaultManager()
	prompt, err := manager.GetPrompt("get_suggestion", "en")
	if err != nil {
		t.Fatal(err)
	}

	// Should specify explanation and command fields
	if !strings.Contains(prompt, `"explanation"`) {
		t.Error("get_suggestion prompt should specify 'explanation' field")
	}

	if !strings.Contains(prompt, `"command"`) {
		t.Error("get_suggestion prompt should specify 'command' field")
	}
}

func TestManagerWithCustomPrompts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt-custom-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	customPrompts := map[string]map[string]string{
		"custom_prompt": {
			"en":    "Custom English prompt with {{.Variable}}",
			"zh-TW": "自定義繁體中文提示 {{.Variable}}",
		},
		"another_prompt": {
			"en": "Another prompt",
		},
	}

	promptFile := filepath.Join(tmpDir, "custom.json")
	data, err := json.Marshal(customPrompts)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(promptFile, data, 0644)
	if err != nil {
		t.Fatal(err)
	}

	manager, err := NewManager(promptFile)
	if err != nil {
		t.Fatal(err)
	}

	// Test custom prompt retrieval
	prompt, err := manager.GetPrompt("custom_prompt", "en")
	if err != nil {
		t.Fatalf("Failed to get custom prompt: %v", err)
	}

	if !strings.Contains(prompt, "Custom English prompt") {
		t.Error("Custom prompt content mismatch")
	}

	// Test Chinese custom prompt
	prompt, err = manager.GetPrompt("custom_prompt", "zh-TW")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(prompt, "自定義") {
		t.Error("Custom Chinese prompt content mismatch")
	}
}

func TestGetPromptEmptyKey(t *testing.T) {
	manager := NewDefaultManager()

	_, err := manager.GetPrompt("", "en")
	if err == nil {
		t.Error("Expected error for empty prompt key")
	}
}

func TestManagerPromptsImmutability(t *testing.T) {
	manager := NewDefaultManager()

	// Get a prompt
	prompt1, err := manager.GetPrompt("generate_command", "en")
	if err != nil {
		t.Fatal(err)
	}

	// Get the same prompt again
	prompt2, err := manager.GetPrompt("generate_command", "en")
	if err != nil {
		t.Fatal(err)
	}

	// They should be identical
	if prompt1 != prompt2 {
		t.Error("Same prompt returned different values")
	}
}

func TestDefaultPromptsCompleteness(t *testing.T) {
	manager := NewDefaultManager()

	// Check that all three main prompts exist
	requiredPrompts := []string{
		"generate_command",
		"get_suggestion",
		"get_enhanced_suggestion",
	}

	for _, key := range requiredPrompts {
		langMap, ok := manager.prompts[key]
		if !ok {
			t.Errorf("Required prompt '%s' is missing", key)
			continue
		}

		// Each should have at least English
		if _, ok := langMap["en"]; !ok {
			t.Errorf("Prompt '%s' missing English translation", key)
		}

		// Check for multiple language support
		if len(langMap) < 5 {
			t.Errorf("Prompt '%s' should support multiple languages, got %d", key, len(langMap))
		}
	}
}

func TestEnhancedSuggestionPromptContext(t *testing.T) {
	manager := NewDefaultManager()
	prompt, err := manager.GetPrompt("get_enhanced_suggestion", "en")
	if err != nil {
		t.Fatal(err)
	}

	// Should have context fields
	contextFields := []string{
		"Working Directory",
		"Shell",
		"Recent Command History",
		"Directory Contents",
	}

	for _, field := range contextFields {
		if !strings.Contains(prompt, field) {
			t.Errorf("Enhanced suggestion prompt should mention '%s'", field)
		}
	}
}
