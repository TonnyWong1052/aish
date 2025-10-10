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

func TestSanitizeLines(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	lines := []string{
		"Normal line",
		"API_KEY=secret123",
		"password=mypass",
	}

	result := sanitizer.SanitizeLines(lines)

	if len(result) != len(lines) {
		t.Errorf("Expected %d lines, got %d", len(lines), len(result))
	}

	if result[0] != "Normal line" {
		t.Error("Non-sensitive line should not change")
	}

	if strings.Contains(result[1], "secret123") {
		t.Error("API key in line 1 should be redacted")
	}

	if strings.Contains(result[2], "mypass") {
		t.Error("Password in line 2 should be redacted")
	}
}

func TestSanitizeMap(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	data := map[string]interface{}{
		"username":      "john",
		"api_key_value": "apikey=sk-1234567890abcdef",
		"pwd_setting":   "password=secretpass",
	}

	result := sanitizer.SanitizeMap(data)

	if result["username"] != "john" {
		t.Error("Non-sensitive data should not change")
	}

	apiKeyStr, ok := result["api_key_value"].(string)
	if !ok || strings.Contains(apiKeyStr, "sk-1234567890abcdef") {
		t.Error("API key should be redacted")
	}

	passwordStr, ok := result["pwd_setting"].(string)
	if !ok || strings.Contains(passwordStr, "secretpass") {
		t.Error("Password should be redacted")
	}
}

func TestEnable_Disable(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	input := "API_KEY=secret123"

	// Enabled by default
	result := sanitizer.Sanitize(input)
	if strings.Contains(result, "secret123") {
		t.Error("Should redact when enabled")
	}

	// Disable
	sanitizer.SetEnabled(false)
	result = sanitizer.Sanitize(input)
	if result != input {
		t.Error("Should not redact when disabled")
	}

	// Re-enable
	sanitizer.SetEnabled(true)
	result = sanitizer.Sanitize(input)
	if strings.Contains(result, "secret123") {
		t.Error("Should redact when re-enabled")
	}
}

func TestIsEnabled(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	if !sanitizer.IsEnabled() {
		t.Error("Should be enabled by default")
	}

	sanitizer.SetEnabled(false)
	if sanitizer.IsEnabled() {
		t.Error("Should be disabled after SetEnabled(false)")
	}

	sanitizer.SetEnabled(true)
	if !sanitizer.IsEnabled() {
		t.Error("Should be enabled after SetEnabled(true)")
	}
}

func TestAddCustomPattern(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	// Add custom pattern
	err := sanitizer.AddPattern("custom_id", `ID-\d{6}`, "***ID***", 5)
	if err != nil {
		t.Fatalf("Failed to add custom pattern: %v", err)
	}

	input := "User ID-123456 accessed the system"
	result := sanitizer.Sanitize(input)

	if strings.Contains(result, "ID-123456") {
		t.Error("Custom pattern should be redacted")
	}

	if !strings.Contains(result, "***ID***") {
		t.Error("Custom pattern replacement should be present")
	}
}

func TestAddCustomPattern_InvalidRegex(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	err := sanitizer.AddPattern("invalid", "[invalid(", "***", 5)
	if err == nil {
		t.Error("Should return error for invalid regex")
	}
}

func TestRemovePattern(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	// Remove email pattern
	sanitizer.RemovePattern("email")

	input := "Contact user@example.com"
	result := sanitizer.Sanitize(input)

	// Email should not be redacted after removal
	if !strings.Contains(result, "user@example.com") {
		t.Error("Email should not be redacted after pattern removal")
	}
}

func TestGetPatterns(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	patterns := sanitizer.GetPatterns()

	if len(patterns) == 0 {
		t.Error("Should have default patterns")
	}

	// Check that some expected patterns exist
	hasAPIKey := false
	hasPassword := false

	for _, p := range patterns {
		if p.Name == "api_key" {
			hasAPIKey = true
		}
		if p.Name == "password" {
			hasPassword = true
		}
	}

	if !hasAPIKey {
		t.Error("Should have api_key pattern")
	}
	if !hasPassword {
		t.Error("Should have password pattern")
	}
}

func TestSanitize_EmptyString(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	result := sanitizer.Sanitize("")
	if result != "" {
		t.Error("Empty string should remain empty")
	}
}

func TestSanitizeLines_EmptySlice(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	result := sanitizer.SanitizeLines([]string{})
	if len(result) != 0 {
		t.Error("Empty slice should remain empty")
	}
}

func TestSanitizeMap_EmptyMap(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	result := sanitizer.SanitizeMap(map[string]interface{}{})
	if len(result) != 0 {
		t.Error("Empty map should remain empty")
	}
}

func TestSanitize_PrivateKey(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	input := `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA...
-----END RSA PRIVATE KEY-----`

	result := sanitizer.Sanitize(input)

	if strings.Contains(result, "BEGIN RSA PRIVATE KEY") {
		t.Error("Private key should be redacted")
	}

	if !strings.Contains(result, "PRIVATE_KEY_REDACTED") {
		t.Error("Should contain redaction marker")
	}
}

func TestSanitize_DatabaseConnection(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	input := "mysql://user:password123@localhost:3306/db"
	result := sanitizer.Sanitize(input)

	if strings.Contains(result, "password123") {
		t.Error("Database password should be redacted")
	}

	if !strings.Contains(result, "REDACTED") {
		t.Error("Should contain redaction marker")
	}
}

func TestSanitize_GitHubToken(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	tests := []struct {
		name  string
		input string
	}{
		{"personal token", "ghp_1234567890abcdef1234567890abcdef1234567890"},
		{"oauth token", "gho_1234567890abcdef1234567890abcdef1234567890"},
		{"user token", "ghu_1234567890abcdef1234567890abcdef1234567890"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.Sanitize(tt.input)
			if !strings.Contains(result, "GITHUB_TOKEN") {
				t.Errorf("GitHub token should be redacted, got: %s", result)
			}
		})
	}
}

func TestSanitize_AWSKeys(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	input := "AWS_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE AWS_SECRET=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
	result := sanitizer.Sanitize(input)

	if strings.Contains(result, "AKIAIOSFODNN7EXAMPLE") {
		t.Error("AWS access key should be redacted")
	}

	if strings.Contains(result, "wJalrXUtnFEMI") {
		t.Error("AWS secret key should be redacted")
	}
}

func TestSanitizeMap_NestedData(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	data := map[string]interface{}{
		"config": map[string]interface{}{
			"credentials": "api_key=sk-secret123456789012345",
			"username":    "john",
		},
		"safe_value": 42,
	}

	result := sanitizer.SanitizeMap(data)

	// Check nested map
	configMap, ok := result["config"].(map[string]interface{})
	if !ok {
		t.Fatal("Config should still be a map")
	}

	creds, _ := configMap["credentials"].(string)
	if strings.Contains(creds, "sk-secret123") {
		t.Error("Nested API key should be redacted")
	}

	if configMap["username"] != "john" {
		t.Error("Non-sensitive nested value should not change")
	}

	if result["safe_value"] != 42 {
		t.Error("Top-level non-sensitive value should not change")
	}
}

func TestPatternPriority(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	patterns := sanitizer.GetPatterns()

	// Verify patterns are sorted by priority
	for i := 0; i < len(patterns)-1; i++ {
		if patterns[i].Priority < patterns[i+1].Priority {
			t.Errorf("Patterns not sorted by priority: %s (priority %d) before %s (priority %d)",
				patterns[i].Name, patterns[i].Priority,
				patterns[i+1].Name, patterns[i+1].Priority)
		}
	}
}

func TestSanitize_CreditCard(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	input := "Card number: 4532-1234-5678-9010"
	result := sanitizer.Sanitize(input)

	if strings.Contains(result, "4532-1234") {
		t.Error("Credit card should be redacted")
	}
}

func TestSanitize_SSN(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()

	input := "SSN: 123-45-6789"
	result := sanitizer.Sanitize(input)

	if strings.Contains(result, "123-45-6789") {
		t.Error("SSN should be redacted")
	}
}

func TestSanitizeLines_WithDisabled(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()
	sanitizer.SetEnabled(false)

	lines := []string{
		"API_KEY=secret123",
		"password=mypass",
	}

	result := sanitizer.SanitizeLines(lines)

	// Should not redact when disabled
	if result[0] != lines[0] || result[1] != lines[1] {
		t.Error("Should not modify lines when disabled")
	}
}

func TestSanitizeMap_WithDisabled(t *testing.T) {
	sanitizer := NewSensitiveDataSanitizer()
	sanitizer.SetEnabled(false)

	data := map[string]interface{}{
		"api_key":  "secret123",
		"password": "mypass",
	}

	result := sanitizer.SanitizeMap(data)

	// Should not redact when disabled
	if result["api_key"] != "secret123" || result["password"] != "mypass" {
		t.Error("Should not modify map when disabled")
	}
}
