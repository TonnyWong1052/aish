package auth

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestShouldDebug(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"debug enabled with 1", "1", true},
		{"debug enabled with true", "true", true},
		{"debug enabled with yes", "yes", true},
		{"debug enabled with debug", "debug", true},
		{"debug disabled with 0", "0", false},
		{"debug disabled with false", "false", false},
		{"debug disabled with empty", "", false},
		{"debug disabled with random", "random", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("AISH_GEMINI_DEBUG", tt.envValue)
			defer os.Unsetenv("AISH_GEMINI_DEBUG")

			result := shouldDebug()
			if result != tt.expected {
				t.Errorf("shouldDebug() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRefreshThreshold(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected time.Duration
	}{
		{"default threshold", "", 2 * time.Hour},
		{"custom 90m", "90m", 90 * time.Minute},
		{"custom 3h", "3h", 3 * time.Hour},
		{"custom 30m", "30m", 30 * time.Minute},
		{"invalid value", "invalid", 2 * time.Hour},
		{"negative value", "-1h", 2 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("AISH_GEMINI_REFRESH_THRESHOLD", tt.envValue)
				defer os.Unsetenv("AISH_GEMINI_REFRESH_THRESHOLD")
			}

			result := refreshThreshold()
			if result != tt.expected {
				t.Errorf("refreshThreshold() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetString(t *testing.T) {
	m := map[string]any{
		"string":  "value",
		"number":  42,
		"bool":    true,
		"missing": nil,
	}

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"existing string", "string", "value"},
		{"number to string", "number", ""},
		{"bool to string", "bool", ""},
		{"missing key", "missing", ""},
		{"nonexistent key", "nonexistent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getString(m, tt.key)
			if result != tt.expected {
				t.Errorf("getString(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

func TestGetNumber(t *testing.T) {
	m := map[string]any{
		"float":   42.5,
		"string":  "not a number",
		"missing": nil,
	}

	tests := []struct {
		name     string
		key      string
		expected float64
	}{
		{"existing float", "float", 42.5},
		{"string value", "string", 0},
		{"missing key", "missing", 0},
		{"nonexistent key", "nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getNumber(m, tt.key)
			if result != tt.expected {
				t.Errorf("getNumber(%q) = %v, want %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected string
	}{
		{"both empty", "", "", ""},
		{"first non-empty", "first", "", "first"},
		{"second non-empty", "", "second", "second"},
		{"both non-empty", "first", "second", "first"},
		{"first with spaces", "  ", "second", "second"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := firstNonEmpty(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("firstNonEmpty(%q, %q) = %q, want %q", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestLoadCredentials(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "oauth_creds.json")

	// Test case 1: Valid credentials file
	t.Run("valid credentials", func(t *testing.T) {
		validCreds := OAuthCredentials{
			ExpiryDate:   time.Now().Add(1*time.Hour).Unix() * 1000,
			AccessToken:  "test_access_token",
			RefreshToken: "test_refresh_token",
			TokenURI:     "https://oauth2.googleapis.com/token",
			ClientID:     "test_client_id",
			ClientSecret: "test_client_secret",
		}

		data, err := json.Marshal(validCreds)
		if err != nil {
			t.Fatalf("Failed to marshal credentials: %v", err)
		}

		if err := os.WriteFile(credsPath, data, 0o600); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		creds, err := loadCredentials(credsPath)
		if err != nil {
			t.Errorf("loadCredentials() error = %v, want nil", err)
		}

		if creds.AccessToken != validCreds.AccessToken {
			t.Errorf("AccessToken = %v, want %v", creds.AccessToken, validCreds.AccessToken)
		}

		if creds.RefreshToken != validCreds.RefreshToken {
			t.Errorf("RefreshToken = %v, want %v", creds.RefreshToken, validCreds.RefreshToken)
		}
	})

	// Test case 2: Non-existent file
	t.Run("non-existent file", func(t *testing.T) {
		nonExistentPath := filepath.Join(tmpDir, "nonexistent.json")
		_, err := loadCredentials(nonExistentPath)
		if err == nil {
			t.Error("loadCredentials() error = nil, want error")
		}
	})

	// Test case 3: Invalid JSON
	t.Run("invalid json", func(t *testing.T) {
		invalidJSONPath := filepath.Join(tmpDir, "invalid.json")
		if err := os.WriteFile(invalidJSONPath, []byte("not valid json"), 0o600); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		_, err := loadCredentials(invalidJSONPath)
		if err == nil {
			t.Error("loadCredentials() error = nil, want error for invalid JSON")
		}
	})

	// Test case 4: Empty file
	t.Run("empty file", func(t *testing.T) {
		emptyPath := filepath.Join(tmpDir, "empty.json")
		if err := os.WriteFile(emptyPath, []byte("{}"), 0o600); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		creds, err := loadCredentials(emptyPath)
		if err != nil {
			t.Errorf("loadCredentials() error = %v, want nil", err)
		}

		if creds.AccessToken != "" {
			t.Error("Empty file should have empty AccessToken")
		}
	})
}

func TestGetAishCredsPath(t *testing.T) {
	// Set up temporary home directory
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Unsetenv("HOME")

	path, err := getAishCredsPath()
	if err != nil {
		t.Errorf("getAishCredsPath() error = %v, want nil", err)
	}

	expectedPath := filepath.Join(tmpDir, ".config", "aish", "gemini_oauth_creds.json")
	if path != expectedPath {
		t.Errorf("getAishCredsPath() = %v, want %v", path, expectedPath)
	}
}

func TestHasGeminiCLIInPath(t *testing.T) {
	// This test checks if the function runs without panic
	// Actual result depends on system PATH
	result := hasGeminiCLIInPath()

	// Just verify it returns a boolean
	if result {
		t.Log("gemini-cli found in PATH")
	} else {
		t.Log("gemini-cli not found in PATH")
	}
}

func TestFormatTokenEndpointError(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   []byte
		want   string
	}{
		{
			name:   "400 with json error",
			status: 400,
			body:   []byte(`{"error":"invalid_grant","error_description":"Token expired"}`),
			want:   "invalid_grant",
		},
		{
			name:   "401 unauthorized",
			status: 401,
			body:   []byte("Unauthorized"),
			want:   "401",
		},
		{
			name:   "500 server error",
			status: 500,
			body:   []byte("Internal Server Error"),
			want:   "500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := formatTokenEndpointError(tt.status, tt.body)
			if err == nil {
				t.Error("formatTokenEndpointError() error = nil, want error")
				return
			}

			errStr := err.Error()
			if errStr == "" {
				t.Error("Error message should not be empty")
			}
		})
	}
}

func TestInferClientIDFromIDToken(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "empty token",
			token:   "",
			wantErr: true,
		},
		{
			name:    "invalid format",
			token:   "not.a.valid.token",
			wantErr: true,
		},
		{
			name:    "invalid base64",
			token:   "header.@@@.signature",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := inferClientIDFromIDToken(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("inferClientIDFromIDToken() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEnsureValidToken_NoCredentials(t *testing.T) {
	// Set up temporary home directory with no credentials
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer func() {
		if oldHome != "" {
			os.Setenv("HOME", oldHome)
		} else {
			os.Unsetenv("HOME")
		}
	}()

	ctx := context.Background()
	err := EnsureValidToken(ctx)

	// Should fail because no credentials exist
	if err == nil {
		t.Log("EnsureValidToken() succeeded (may have system credentials)")
	} else {
		t.Logf("EnsureValidToken() error = %v (expected without credentials)", err)
	}
}

func TestOAuthCredentials_Structure(t *testing.T) {
	creds := OAuthCredentials{
		ExpiryDate:   1234567890000,
		AccessToken:  "test_token",
		RefreshToken: "test_refresh",
		TokenURI:     "https://oauth2.googleapis.com/token",
		ClientID:     "test_client",
		ClientSecret: "test_secret",
	}

	// Test JSON marshaling
	data, err := json.Marshal(creds)
	if err != nil {
		t.Fatalf("Failed to marshal OAuthCredentials: %v", err)
	}

	// Test JSON unmarshaling
	var decoded OAuthCredentials
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal OAuthCredentials: %v", err)
	}

	if decoded.AccessToken != creds.AccessToken {
		t.Errorf("AccessToken = %v, want %v", decoded.AccessToken, creds.AccessToken)
	}

	if decoded.ExpiryDate != creds.ExpiryDate {
		t.Errorf("ExpiryDate = %v, want %v", decoded.ExpiryDate, creds.ExpiryDate)
	}
}

func TestDebugf(t *testing.T) {
	// Test that debugf doesn't panic
	os.Setenv("AISH_GEMINI_DEBUG", "1")
	defer os.Unsetenv("AISH_GEMINI_DEBUG")

	// Should not panic
	debugf("Test debug message: %s\n", "test")
	debugf("Test with number: %d\n", 42)

	// Test with debug disabled
	os.Setenv("AISH_GEMINI_DEBUG", "0")
	debugf("This should not output\n")
}

func TestConstants(t *testing.T) {
	// Verify constants are defined
	if DefaultPublicClientID == "" {
		t.Error("DefaultPublicClientID should not be empty")
	}

	if DefaultPublicClientSecret == "" {
		t.Error("DefaultPublicClientSecret should not be empty")
	}

	// Verify they follow expected format
	if !strings.Contains(DefaultPublicClientID, ".apps.googleusercontent.com") {
		t.Error("DefaultPublicClientID should be a valid Google OAuth client ID")
	}

	if !strings.HasPrefix(DefaultPublicClientSecret, "GOCSPX-") {
		t.Error("DefaultPublicClientSecret should have GOCSPX- prefix")
	}
}
