package errors

import "fmt"

// Configuration related error factory functions

// ErrConfigLoadFailed configuration loading failed
func ErrConfigLoadFailed(path string, cause error) *AishError {
	return WrapError(cause, ErrConfigLoad, "Configuration file loading failed").
		WithContext("config_path", path)
}

// ErrConfigSaveFailed configuration saving failed
func ErrConfigSaveFailed(path string, cause error) *AishError {
	return WrapError(cause, ErrConfigSave, "Configuration file saving failed").
		WithContext("config_path", path)
}

// ErrConfigValidationFailed configuration validation failed
func ErrConfigValidationFailed(field string, reason string) *AishError {
	return NewError(ErrConfigValidation, fmt.Sprintf("Configuration validation failed: %s", reason)).
		WithContext("field", field).
		WithContext("reason", reason)
}

// ErrProviderConfigMissing provider configuration missing
func ErrProviderConfigMissing(provider string) *AishError {
	return NewError(ErrConfigMissing, fmt.Sprintf("Provider '%s' configuration missing or incomplete", provider)).
		WithContext("provider", provider)
}

// LLM provider related error factory functions

// ErrProviderInitFailed provider initialization failed
func ErrProviderInitFailed(provider string, cause error) *AishError {
	return WrapError(cause, ErrProviderInit, fmt.Sprintf("provider '%s' initialization failed", provider)).
		WithContext("provider", provider)
}

// ErrProviderNotFoundError provider not found
func ErrProviderNotFoundError(provider string) *AishError {
	return NewError(ErrProviderNotFound, fmt.Sprintf("未知的提供商: %s", provider)).
		WithContext("provider", provider)
}

// ErrProviderRequestFailed provider request failed
func ErrProviderRequestFailed(provider string, cause error) *AishError {
	return NewRetryableError(ErrProviderRequest, fmt.Sprintf("提供商 '%s' 請求失敗", provider)).
		WithContext("provider", provider).
		WithCause(cause)
}

// ErrProviderResponseInvalid provider response invalid
func ErrProviderResponseInvalid(provider string, details string) *AishError {
	return NewError(ErrProviderResponse, fmt.Sprintf("提供商 '%s' 響應無效", provider)).
		WithContext("provider", provider).
		WithContext("details", details)
}

// ErrProviderAuthFailed provider authentication failed
func ErrProviderAuthFailed(provider string, cause error) *AishError {
	return WrapError(cause, ErrProviderAuth, fmt.Sprintf("提供商 '%s' 認證失敗", provider)).
		WithContext("provider", provider)
}

// ErrProviderQuotaExceeded provider quota exceeded
func ErrProviderQuotaExceeded(provider string) *AishError {
	return NewError(ErrProviderQuota, fmt.Sprintf("提供商 '%s' API 配額已用盡", provider)).
		WithContext("provider", provider)
}

// Shell Hook related error factory functions

// ErrHookInstallFailed Hook installation failed
func ErrHookInstallFailed(cause error) *AishError {
	return WrapError(cause, ErrHookInstall, "Shell hook 安裝失敗")
}

// ErrHookUninstallFailed Hook uninstallation failed
func ErrHookUninstallFailed(cause error) *AishError {
	return WrapError(cause, ErrHookUninstall, "Shell hook 卸載失敗")
}

// ErrHookExecutionFailed Hook execution failed
func ErrHookExecutionFailed(cause error) *AishError {
	return NewInternalError(ErrHookExecution, "Shell hook 執行失敗").
		WithCause(cause)
}

// History record related error factory functions

// ErrHistoryLoadFailed history record loading failed
func ErrHistoryLoadFailed(cause error) *AishError {
	return WrapError(cause, ErrHistoryLoad, "歷史記錄加載失敗")
}

// ErrHistorySaveFailed history record saving failed
func ErrHistorySaveFailed(cause error) *AishError {
	return WrapError(cause, ErrHistorySave, "歷史記錄保存失敗")
}

// ErrHistoryClearFailed history record clearing failed
func ErrHistoryClearFailed(cause error) *AishError {
	return WrapError(cause, ErrHistoryClear, "歷史記錄清理失敗")
}

// Context enhancement related error factory functions

// ErrContextEnhanceFailed context enhancement failed
func ErrContextEnhanceFailed(cause error) *AishError {
	return NewInternalError(ErrContextEnhance, "上下文增強失敗").
		WithCause(cause)
}

// ErrContextReadFailed context reading failed
func ErrContextReadFailed(source string, cause error) *AishError {
	return NewInternalError(ErrContextRead, "上下文讀取失敗").
		WithContext("source", source).
		WithCause(cause)
}

// User interface related error factory functions

// ErrUserInputInvalid user input invalid
func ErrUserInputInvalid(details string) *AishError {
	return NewError(ErrUserInput, "用戶輸入無效").
		WithContext("details", details)
}

// ErrUserCancelled user cancelled operation
func ErrUserCancelled() *AishError {
	return NewError(ErrUserCancel, "用戶取消操作")
}

// System related error factory functions

// ErrFileSystemError file system error
func ErrFileSystemError(operation string, path string, cause error) *AishError {
	return WrapError(cause, ErrFileSystem, fmt.Sprintf("文件系統操作失敗: %s", operation)).
		WithContext("operation", operation).
		WithContext("path", path)
}

// ErrNetworkError network error
func ErrNetworkError(operation string, cause error) *AishError {
	return NewRetryableError(ErrNetwork, fmt.Sprintf("網絡操作失敗: %s", operation)).
		WithContext("operation", operation).
		WithCause(cause)
}

// ErrPermissionDeniedError permission denied error
func ErrPermissionDeniedError(resource string) *AishError {
	return NewError(ErrPermission, fmt.Sprintf("權限被拒絕: %s", resource)).
		WithContext("resource", resource)
}

// ErrTimeoutError timeout error
func ErrTimeoutError(operation string, timeout string) *AishError {
	return NewRetryableError(ErrTimeout, fmt.Sprintf("操作超時: %s", operation)).
		WithContext("operation", operation).
		WithContext("timeout", timeout)
}
