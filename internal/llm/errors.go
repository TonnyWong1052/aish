package llm

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// LLMError represents different types of LLM-related errors
type LLMError struct {
	Type    ErrorType
	Message string
	Cause   error
}

// ErrorType defines the category of LLM errors
type ErrorType string

const (
	// Network-related errors
	NetworkError ErrorType = "network_error"
	TimeoutError ErrorType = "timeout_error"

	// Authentication and authorization errors
	AuthError          ErrorType = "auth_error"
	QuotaExceededError ErrorType = "quota_exceeded_error"

	// Request-related errors
	InvalidRequestError ErrorType = "invalid_request_error"
	ModelNotFoundError  ErrorType = "model_not_found_error"

	// Response-related errors
	InvalidResponseError ErrorType = "invalid_response_error"
	EmptyResponseError   ErrorType = "empty_response_error"

	// Configuration errors
	ConfigError   ErrorType = "config_error"
	ProviderError ErrorType = "provider_error"

	// Generic errors
	UnknownError ErrorType = "unknown_error"
)

// Error implements the error interface
func (e *LLMError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %s)", e.Type, e.Message, e.Cause.Error())
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap allows errors.Is and errors.As to work correctly
func (e *LLMError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns true if the error is potentially recoverable with retry
func (e *LLMError) IsRetryable() bool {
	switch e.Type {
	case NetworkError, TimeoutError, QuotaExceededError:
		return true
	default:
		return false
	}
}

// NewLLMError creates a new LLM error
func NewLLMError(errorType ErrorType, message string, cause error) *LLMError {
	return &LLMError{
		Type:    errorType,
		Message: message,
		Cause:   cause,
	}
}

// ClassifyHTTPError classifies HTTP response errors
func ClassifyHTTPError(resp *http.Response, err error) *LLMError {
	if err != nil {
		if strings.Contains(err.Error(), "timeout") {
			return NewLLMError(TimeoutError, "Request timed out", err)
		}
		return NewLLMError(NetworkError, "Network request failed", err)
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return NewLLMError(AuthError, "Authentication failed - check API key", nil)
	case http.StatusForbidden:
		return NewLLMError(AuthError, "Access forbidden - insufficient permissions", nil)
	case http.StatusNotFound:
		return NewLLMError(ModelNotFoundError, "Model or endpoint not found", nil)
	case http.StatusTooManyRequests:
		return NewLLMError(QuotaExceededError, "Rate limit or quota exceeded", nil)
	case http.StatusBadRequest:
		return NewLLMError(InvalidRequestError, "Bad request - check request parameters", nil)
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return NewLLMError(NetworkError, fmt.Sprintf("Server error (status: %d)", resp.StatusCode), nil)
	default:
		if resp.StatusCode >= 400 {
			return NewLLMError(UnknownError, fmt.Sprintf("HTTP error (status: %d)", resp.StatusCode), nil)
		}
	}

	return nil
}

// ClassifyProviderError classifies provider-specific errors
func ClassifyProviderError(providerName string, err error) *LLMError {
	errMsg := strings.ToLower(err.Error())

	// Common error patterns across providers
	switch {
	case strings.Contains(errMsg, "api key"):
		return NewLLMError(AuthError, "Invalid or missing API key", err)
	case strings.Contains(errMsg, "quota") || strings.Contains(errMsg, "rate limit"):
		return NewLLMError(QuotaExceededError, "API quota or rate limit exceeded", err)
	case strings.Contains(errMsg, "timeout"):
		return NewLLMError(TimeoutError, "Request timeout", err)
	case strings.Contains(errMsg, "model"):
		return NewLLMError(ModelNotFoundError, "Model not found or unavailable", err)
	case strings.Contains(errMsg, "network") || strings.Contains(errMsg, "connection"):
		return NewLLMError(NetworkError, "Network connectivity issue", err)
	case strings.Contains(errMsg, "parse") || strings.Contains(errMsg, "decode"):
		return NewLLMError(InvalidResponseError, "Failed to parse API response", err)
	case strings.Contains(errMsg, "empty") || strings.Contains(errMsg, "no response"):
		return NewLLMError(EmptyResponseError, "Received empty response from API", err)
	}

	// Provider-specific error patterns
	switch providerName {
	case "openai":
		return classifyOpenAIError(err)
	case "gemini":
		return classifyGeminiError(err)
	case "gemini-cli":
		return classifyGeminiCLIError(err)
	}

	return NewLLMError(UnknownError, "Unknown error occurred", err)
}

// classifyOpenAIError handles OpenAI-specific error classification
func classifyOpenAIError(err error) *LLMError {
	errMsg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errMsg, "insufficient_quota"):
		return NewLLMError(QuotaExceededError, "OpenAI quota exceeded", err)
	case strings.Contains(errMsg, "invalid_api_key"):
		return NewLLMError(AuthError, "Invalid OpenAI API key", err)
	case strings.Contains(errMsg, "model_not_found"):
		return NewLLMError(ModelNotFoundError, "OpenAI model not found", err)
	default:
		return NewLLMError(ProviderError, "OpenAI provider error", err)
	}
}

// classifyGeminiError handles Gemini-specific error classification
func classifyGeminiError(err error) *LLMError {
	errMsg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errMsg, "api_key_invalid"):
		return NewLLMError(AuthError, "Invalid Gemini API key", err)
	case strings.Contains(errMsg, "quota_exceeded"):
		return NewLLMError(QuotaExceededError, "Gemini quota exceeded", err)
	case strings.Contains(errMsg, "model_not_found"):
		return NewLLMError(ModelNotFoundError, "Gemini model not found", err)
	default:
		return NewLLMError(ProviderError, "Gemini provider error", err)
	}
}

// classifyGeminiCLIError handles Gemini CLI-specific error classification
func classifyGeminiCLIError(err error) *LLMError {
	errMsg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errMsg, "oauth") || strings.Contains(errMsg, "authentication"):
		return NewLLMError(AuthError, "Gemini CLI authentication failed - please run 'gemini auth'", err)
	case strings.Contains(errMsg, "project"):
		return NewLLMError(ConfigError, "Invalid or missing Google Cloud project", err)
	case strings.Contains(errMsg, "not found"):
		return NewLLMError(ConfigError, "Gemini CLI not found - please install it", err)
	default:
		return NewLLMError(ProviderError, "Gemini CLI provider error", err)
	}
}

// WrapError wraps an existing error with LLM error context
func WrapError(errorType ErrorType, message string, cause error) *LLMError {
	// If the cause is already an LLMError, don't double-wrap
	if llmErr, ok := cause.(*LLMError); ok {
		return llmErr
	}
	return NewLLMError(errorType, message, cause)
}

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error) bool {
	var llmErr *LLMError
	if errors.As(err, &llmErr) {
		return llmErr.IsRetryable()
	}

	// Check for common retryable error patterns
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "network") ||
		strings.Contains(errMsg, "connection") ||
		strings.Contains(errMsg, "temporary")
}
