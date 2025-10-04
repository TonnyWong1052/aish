package security

import (
	"strings"
	"testing"
)

func TestNewSensitiveDataSanitizer(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	if sanitizer == nil {
		t.Fatal("Sanitizer should not be nil")
	}

	if !sanitizer.enabled {
		t.Error("Sanitizer should be enabled by default")
	}

	if len(sanitizer.patterns) == 0 {
		t.Error("Sanitizer should have default patterns")
	}
}

func TestSanitize_APIKey(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "API key with equals",
			input:    "api_key=sk-1234567890abcdef1234567890",
			expected: "***REDACTED_API_KEY***",
		},
		{
			name:     "API key with colon",
			input:    "apikey: 'sk-1234567890abcdef1234567890'",
			expected: "***REDACTED_API_KEY***",
		},
		{
			name:     "Bearer token",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expected: "Authorization: Bearer ***REDACTED_TOKEN***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.Sanitize(tt.input)
			if !strings.Contains(result, tt.expected) {
				t.Errorf("Expected result to contain %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSanitize_Password(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "password with equals",
			input:    "password=mySecretPass123",
			expected: "password=***REDACTED_PASSWORD***",
		},
		{
			name:     "pwd parameter",
			input:    "pwd: mypassword",
			expected: "pwd=***REDACTED_PASSWORD***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.Sanitize(tt.input)
			if !strings.Contains(result, tt.expected) {
				t.Errorf("Expected result to contain %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSanitize_Email(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	input := "Contact user@example.com for support"
	result := sanitizer.Sanitize(input)

	if strings.Contains(result, "user@example.com") {
		t.Error("Email should be sanitized")
	}

	if !strings.Contains(result, "@example.com") {
		t.Error("Domain part should be preserved")
	}
}

func TestSanitize_JWTToken(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	input := "Token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	result := sanitizer.Sanitize(input)

	if strings.Contains(result, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9") {
		t.Error("JWT token should be sanitized")
	}

	if !strings.Contains(result, "***JWT_TOKEN***") {
		t.Error("JWT token should be replaced with placeholder")
	}
}

func TestSanitize_CommandArgs(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "password flag",
			input:    "command --password mySecretPass",
			expected: "command --password ***REDACTED***",
		},
		{
			name:     "key parameter",
			input:    "curl -H 'api-key=sk-123456'",
			expected: "***REDACTED_API_KEY***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.Sanitize(tt.input)
			if !strings.Contains(result, "***REDACTED") {
				t.Errorf("Expected redacted content in %q, got %q", tt.input, result)
			}
		})
	}
}

func TestSanitize_NoSensitiveData(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	input := "This is a normal command without sensitive data"
	result := sanitizer.Sanitize(input)

	if result != input {
		t.Errorf("Non-sensitive data should not be modified. Got: %q", result)
	}
}

func TestSanitize_MultiplePatterns(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	input := "API_KEY=sk-123456 PASSWORD=secret123 user@example.com"
	result := sanitizer.Sanitize(input)

	// Should redact all sensitive patterns
	if strings.Contains(result, "sk-123456") {
		t.Error("API key should be redacted")
	}
	if strings.Contains(result, "secret123") {
		t.Error("Password should be redacted")
	}
	if strings.Contains(result, "user@") {
		t.Error("Email should be redacted")
	}
}
