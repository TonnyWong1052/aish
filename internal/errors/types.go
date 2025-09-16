package errors

import (
	"fmt"
	"runtime"
)

// ErrorCode defines error code type
type ErrorCode string

// System-level error codes
const (
	// Configuration related errors
	ErrConfigLoad       ErrorCode = "CONFIG_LOAD"
	ErrConfigSave       ErrorCode = "CONFIG_SAVE"
	ErrConfigValidation ErrorCode = "CONFIG_VALIDATION"
	ErrConfigMissing    ErrorCode = "CONFIG_MISSING"

	// LLM provider errors
	ErrProviderInit     ErrorCode = "PROVIDER_INIT"
	ErrProviderNotFound ErrorCode = "PROVIDER_NOT_FOUND"
	ErrProviderRequest  ErrorCode = "PROVIDER_REQUEST"
	ErrProviderResponse ErrorCode = "PROVIDER_RESPONSE"
	ErrProviderAuth     ErrorCode = "PROVIDER_AUTH"
	ErrProviderQuota    ErrorCode = "PROVIDER_QUOTA"

	// Shell Hook errors
	ErrHookInstall   ErrorCode = "HOOK_INSTALL"
	ErrHookUninstall ErrorCode = "HOOK_UNINSTALL"
	ErrHookExecution ErrorCode = "HOOK_EXECUTION"

	// History record errors
	ErrHistoryLoad  ErrorCode = "HISTORY_LOAD"
	ErrHistorySave  ErrorCode = "HISTORY_SAVE"
	ErrHistoryClear ErrorCode = "HISTORY_CLEAR"

	// Classifier errors
	ErrClassification ErrorCode = "CLASSIFICATION"

	// Context enhancement errors
	ErrContextEnhance ErrorCode = "CONTEXT_ENHANCE"
	ErrContextRead    ErrorCode = "CONTEXT_READ"

	// User interface errors
	ErrUserInput  ErrorCode = "USER_INPUT"
	ErrUserCancel ErrorCode = "USER_CANCEL"

	// Cache errors
	ErrCacheError ErrorCode = "CACHE_ERROR"
	ErrCacheRead  ErrorCode = "CACHE_READ"
	ErrCacheWrite ErrorCode = "CACHE_WRITE"

	// System errors
	ErrFileSystem ErrorCode = "FILE_SYSTEM"
	ErrNetwork    ErrorCode = "NETWORK"
	ErrPermission ErrorCode = "PERMISSION"
	ErrTimeout    ErrorCode = "TIMEOUT"
)

// AishError represents a structured error for AISH application
type AishError struct {
	Code       ErrorCode              `json:"code"`
	Message    string                 `json:"message"`
	Details    string                 `json:"details,omitempty"`
	Cause      error                  `json:"-"`
	Context    map[string]interface{} `json:"context,omitempty"`
	Stack      string                 `json:"stack,omitempty"`
	Retryable  bool                   `json:"retryable"`
	UserFacing bool                   `json:"user_facing"`
}

// Error implements error interface
func (e *AishError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap supports Go 1.13+ error wrapping
func (e *AishError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns whether the error is retryable
func (e *AishError) IsRetryable() bool {
	return e.Retryable
}

// IsUserFacing returns whether the error should be shown to user
func (e *AishError) IsUserFacing() bool {
	return e.UserFacing
}

// WithContext adds context information
func (e *AishError) WithContext(key string, value interface{}) *AishError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithCause adds root cause
func (e *AishError) WithCause(cause error) *AishError {
	e.Cause = cause
	return e
}

// NewError creates a new AISH error
func NewError(code ErrorCode, message string) *AishError {
	return &AishError{
		Code:       code,
		Message:    message,
		Context:    make(map[string]interface{}),
		Stack:      captureStack(),
		Retryable:  false,
		UserFacing: true,
	}
}

// NewInternalError creates internal error (not shown to user)
func NewInternalError(code ErrorCode, message string) *AishError {
	return &AishError{
		Code:       code,
		Message:    message,
		Context:    make(map[string]interface{}),
		Stack:      captureStack(),
		Retryable:  false,
		UserFacing: false,
	}
}

// NewRetryableError creates retryable error
func NewRetryableError(code ErrorCode, message string) *AishError {
	return &AishError{
		Code:       code,
		Message:    message,
		Context:    make(map[string]interface{}),
		Stack:      captureStack(),
		Retryable:  true,
		UserFacing: true,
	}
}

// WrapError wraps existing error
func WrapError(err error, code ErrorCode, message string) *AishError {
	if err == nil {
		return nil
	}

	return &AishError{
		Code:       code,
		Message:    message,
		Cause:      err,
		Context:    make(map[string]interface{}),
		Stack:      captureStack(),
		Retryable:  false,
		UserFacing: true,
	}
}

// captureStack captures current stack information
func captureStack() string {
	// Skip current function and the function that called it
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return "unknown"
	}
	return fmt.Sprintf("%s:%d", file, line)
}

// IsAishError checks if it's an AISH error
func IsAishError(err error) bool {
	_, ok := err.(*AishError)
	return ok
}

// GetAishError tries to convert error to AISH error
func GetAishError(err error) (*AishError, bool) {
	aishErr, ok := err.(*AishError)
	return aishErr, ok
}

// HasCode checks if error has specific code
func HasCode(err error, code ErrorCode) bool {
	if aishErr, ok := GetAishError(err); ok {
		return aishErr.Code == code
	}
	return false
}
