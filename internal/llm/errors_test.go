package llm

import (
	"errors"
	"net/http"
	"testing"
)

func TestLLMError_Error(t *testing.T) {
	testCases := []struct {
		name     string
		llmError *LLMError
		expected string
	}{
		{
			name: "Error with cause",
			llmError: &LLMError{
				Type:    NetworkError,
				Message: "Request failed",
				Cause:   errors.New("connection timeout"),
			},
			expected: "network_error: Request failed (caused by: connection timeout)",
		},
		{
			name: "Error without cause",
			llmError: &LLMError{
				Type:    AuthError,
				Message: "Invalid API key",
				Cause:   nil,
			},
			expected: "auth_error: Invalid API key",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.llmError.Error()
			if result != tc.expected {
				t.Errorf("Expected: %s, Got: %s", tc.expected, result)
			}
		})
	}
}

func TestLLMError_IsRetryable(t *testing.T) {
	testCases := []struct {
		name      string
		errorType ErrorType
		expected  bool
	}{
		{"Network error is retryable", NetworkError, true},
		{"Timeout error is retryable", TimeoutError, true},
		{"Quota exceeded is retryable", QuotaExceededError, true},
		{"Auth error is not retryable", AuthError, false},
		{"Invalid request is not retryable", InvalidRequestError, false},
		{"Config error is not retryable", ConfigError, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := &LLMError{Type: tc.errorType}
			result := err.IsRetryable()
			if result != tc.expected {
				t.Errorf("Expected: %v, Got: %v", tc.expected, result)
			}
		})
	}
}

func TestClassifyHTTPError(t *testing.T) {
	testCases := []struct {
		name         string
		statusCode   int
		expectedType ErrorType
	}{
		{"401 Unauthorized", http.StatusUnauthorized, AuthError},
		{"403 Forbidden", http.StatusForbidden, AuthError},
		{"404 Not Found", http.StatusNotFound, ModelNotFoundError},
		{"429 Too Many Requests", http.StatusTooManyRequests, QuotaExceededError},
		{"400 Bad Request", http.StatusBadRequest, InvalidRequestError},
		{"500 Internal Server Error", http.StatusInternalServerError, NetworkError},
		{"502 Bad Gateway", http.StatusBadGateway, NetworkError},
		{"503 Service Unavailable", http.StatusServiceUnavailable, NetworkError},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock response
			resp := &http.Response{StatusCode: tc.statusCode}

			result := ClassifyHTTPError(resp, nil)
			if result == nil {
				t.Fatal("Expected error classification, got nil")
			}

			if result.Type != tc.expectedType {
				t.Errorf("Expected: %v, Got: %v", tc.expectedType, result.Type)
			}
		})
	}
}

func TestClassifyHTTPError_WithNetworkError(t *testing.T) {
	networkErr := errors.New("connection timeout")

	result := ClassifyHTTPError(nil, networkErr)
	if result == nil {
		t.Fatal("Expected error classification, got nil")
	}

	if result.Type != TimeoutError {
		t.Errorf("Expected timeout error, got: %v", result.Type)
	}

	if result.Cause != networkErr {
		t.Error("Expected cause to be preserved")
	}
}

func TestClassifyProviderError(t *testing.T) {
	testCases := []struct {
		name         string
		provider     string
		errorMsg     string
		expectedType ErrorType
	}{
		{"OpenAI API key error", "openai", "api key invalid", AuthError},
		{"Gemini quota error", "gemini", "quota exceeded", QuotaExceededError},
		{"Timeout error", "any", "request timeout", TimeoutError},
		{"Model not found", "any", "model not available", ModelNotFoundError},
		{"Network error", "any", "network connection failed", NetworkError},
		{"Parse error", "any", "failed to parse response", InvalidResponseError},
		{"Empty response", "any", "no response received", EmptyResponseError},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := errors.New(tc.errorMsg)
			result := ClassifyProviderError(tc.provider, err)

			if result.Type != tc.expectedType {
				t.Errorf("Expected: %v, Got: %v", tc.expectedType, result.Type)
			}
		})
	}
}

func TestClassifyOpenAIError(t *testing.T) {
	testCases := []struct {
		name         string
		errorMsg     string
		expectedType ErrorType
	}{
		{"Insufficient quota", "insufficient_quota", QuotaExceededError},
		{"Invalid API key", "invalid_api_key", AuthError},
		{"Model not found", "model_not_found", ModelNotFoundError},
		{"Generic OpenAI error", "some other error", ProviderError},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := errors.New(tc.errorMsg)
			result := classifyOpenAIError(err)

			if result.Type != tc.expectedType {
				t.Errorf("Expected: %v, Got: %v", tc.expectedType, result.Type)
			}
		})
	}
}

func TestClassifyGeminiError(t *testing.T) {
	testCases := []struct {
		name         string
		errorMsg     string
		expectedType ErrorType
	}{
		{"Invalid API key", "api_key_invalid", AuthError},
		{"Quota exceeded", "quota_exceeded", QuotaExceededError},
		{"Model not found", "model_not_found", ModelNotFoundError},
		{"Generic Gemini error", "some other error", ProviderError},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := errors.New(tc.errorMsg)
			result := classifyGeminiError(err)

			if result.Type != tc.expectedType {
				t.Errorf("Expected: %v, Got: %v", tc.expectedType, result.Type)
			}
		})
	}
}

func TestClassifyGeminiCLIError(t *testing.T) {
	testCases := []struct {
		name         string
		errorMsg     string
		expectedType ErrorType
	}{
		{"OAuth error", "oauth token invalid", AuthError},
		{"Authentication error", "authentication failed", AuthError},
		{"Project error", "invalid project", ConfigError},
		{"CLI not found", "gemini-cli: not found", ConfigError},
		{"Generic CLI error", "some other error", ProviderError},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := errors.New(tc.errorMsg)
			result := classifyGeminiCLIError(err)

			if result.Type != tc.expectedType {
				t.Errorf("Expected: %v, Got: %v", tc.expectedType, result.Type)
			}
		})
	}
}

func TestWrapError(t *testing.T) {
	originalErr := errors.New("original error")

	// Test wrapping a regular error
	wrappedErr := WrapError(NetworkError, "Network failed", originalErr)
	if wrappedErr.Type != NetworkError {
		t.Errorf("Expected NetworkError, got: %v", wrappedErr.Type)
	}
	if wrappedErr.Cause != originalErr {
		t.Error("Expected cause to be preserved")
	}

	// Test double-wrapping prevention
	llmErr := &LLMError{Type: AuthError, Message: "Auth failed"}
	reWrapped := WrapError(NetworkError, "Network failed", llmErr)
	if reWrapped != llmErr {
		t.Error("Expected LLMError not to be double-wrapped")
	}
}

func TestIsRetryableError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Retryable LLM error",
			err:      &LLMError{Type: NetworkError},
			expected: true,
		},
		{
			name:     "Non-retryable LLM error",
			err:      &LLMError{Type: AuthError},
			expected: false,
		},
		{
			name:     "Timeout error string",
			err:      errors.New("request timeout occurred"),
			expected: true,
		},
		{
			name:     "Network error string",
			err:      errors.New("network connection failed"),
			expected: true,
		},
		{
			name:     "Non-retryable error string",
			err:      errors.New("authentication failed"),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsRetryableError(tc.err)
			if result != tc.expected {
				t.Errorf("Expected: %v, Got: %v", tc.expected, result)
			}
		})
	}
}

func TestErrorTypeConstants(t *testing.T) {
	// Ensure all error type constants are properly defined
	expectedTypes := map[ErrorType]string{
		NetworkError:         "network_error",
		TimeoutError:         "timeout_error",
		AuthError:            "auth_error",
		QuotaExceededError:   "quota_exceeded_error",
		InvalidRequestError:  "invalid_request_error",
		ModelNotFoundError:   "model_not_found_error",
		InvalidResponseError: "invalid_response_error",
		EmptyResponseError:   "empty_response_error",
		ConfigError:          "config_error",
		ProviderError:        "provider_error",
		UnknownError:         "unknown_error",
	}

	for errorType, expected := range expectedTypes {
		if string(errorType) != expected {
			t.Errorf("Error type %v should have string value %s, got %s",
				errorType, expected, string(errorType))
		}
	}
}
