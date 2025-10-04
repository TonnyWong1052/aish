package classification

import (
	"context"
	"time"

	"github.com/TonnyWong1052/aish/internal/config"
)

// RetryConfig defines configuration for error recovery retry mechanism
type RetryConfig struct {
	MaxRetries         int           `json:"max_retries"`         // Maximum number of retry attempts
	BackoffFactor      time.Duration `json:"backoff_factor"`      // Base backoff duration between retries
	RetryableErrors    []ErrorType   `json:"retryable_errors"`    // List of error types that can be retried
	MaxBackoff         time.Duration `json:"max_backoff"`         // Maximum backoff duration
	ExponentialBackoff bool          `json:"exponential_backoff"` // Whether to use exponential backoff
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:    config.MaxRetryAttempts,
		BackoffFactor: config.DefaultRetryDelay,
		RetryableErrors: []ErrorType{
			NetworkError,
			TimeoutError,
			DatabaseError,
			MemoryError,
			DiskSpaceError,
		},
		MaxBackoff:         30 * time.Second,
		ExponentialBackoff: true,
	}
}

// RecoveryStrategy defines how to handle specific error types
type RecoveryStrategy struct {
	ErrorType   ErrorType `json:"error_type"`
	Retryable   bool      `json:"retryable"`
	AutoRecover bool      `json:"auto_recover"`
	Suggestion  string    `json:"suggestion"`
}

// GetRecoveryStrategies returns recovery strategies for different error types
func GetRecoveryStrategies() map[ErrorType]*RecoveryStrategy {
	return map[ErrorType]*RecoveryStrategy{
		CommandNotFound: {
			ErrorType:   CommandNotFound,
			Retryable:   false,
			AutoRecover: false,
			Suggestion:  "Check if the command is installed and available in PATH",
		},
		FileNotFoundOrDirectory: {
			ErrorType:   FileNotFoundOrDirectory,
			Retryable:   false,
			AutoRecover: false,
			Suggestion:  "Verify the file path and ensure the file exists",
		},
		PermissionDenied: {
			ErrorType:   PermissionDenied,
			Retryable:   false,
			AutoRecover: false,
			Suggestion:  "Check file permissions or run with elevated privileges",
		},
		NetworkError: {
			ErrorType:   NetworkError,
			Retryable:   true,
			AutoRecover: true,
			Suggestion:  "Check network connectivity and try again",
		},
		TimeoutError: {
			ErrorType:   TimeoutError,
			Retryable:   true,
			AutoRecover: true,
			Suggestion:  "Increase timeout duration or check system performance",
		},
		DatabaseError: {
			ErrorType:   DatabaseError,
			Retryable:   true,
			AutoRecover: false,
			Suggestion:  "Check database connectivity and configuration",
		},
		ConfigError: {
			ErrorType:   ConfigError,
			Retryable:   false,
			AutoRecover: false,
			Suggestion:  "Review and fix configuration settings",
		},
		DependencyError: {
			ErrorType:   DependencyError,
			Retryable:   false,
			AutoRecover: false,
			Suggestion:  "Install missing dependencies or check versions",
		},
		MemoryError: {
			ErrorType:   MemoryError,
			Retryable:   true,
			AutoRecover: false,
			Suggestion:  "Free up memory or increase available memory",
		},
		DiskSpaceError: {
			ErrorType:   DiskSpaceError,
			Retryable:   false,
			AutoRecover: false,
			Suggestion:  "Free up disk space before retrying",
		},
		AuthenticationError: {
			ErrorType:   AuthenticationError,
			Retryable:   false,
			AutoRecover: false,
			Suggestion:  "Check credentials and authentication settings",
		},
		InteractiveToolUsage: {
			ErrorType:   InteractiveToolUsage,
			Retryable:   false,
			AutoRecover: false,
			Suggestion:  "Review command usage and provide required arguments",
		},
		GenericError: {
			ErrorType:   GenericError,
			Retryable:   false,
			AutoRecover: false,
			Suggestion:  "Review error details and try alternative approaches",
		},
	}
}

// IsRetryable checks if an error type can be retried
func IsRetryable(errorType ErrorType, retryConfig *RetryConfig) bool {
	for _, retryableType := range retryConfig.RetryableErrors {
		if errorType == retryableType {
			return true
		}
	}
	return false
}

// CalculateBackoff calculates the backoff duration for a retry attempt
func CalculateBackoff(attempt int, config *RetryConfig) time.Duration {
	if !config.ExponentialBackoff {
		return config.BackoffFactor
	}

	// Exponential backoff: base * 2^attempt
	backoff := config.BackoffFactor * time.Duration(1<<uint(attempt))

	// Cap at maximum backoff
	if backoff > config.MaxBackoff {
		backoff = config.MaxBackoff
	}

	return backoff
}

// ShouldRetry determines if a command should be retried based on error type and attempt count
func ShouldRetry(errorType ErrorType, attempt int, config *RetryConfig) bool {
	if attempt >= config.MaxRetries {
		return false
	}

	return IsRetryable(errorType, config)
}

// RecoveryManager manages error recovery and retry logic
type RecoveryManager struct {
	config     *RetryConfig
	strategies map[ErrorType]*RecoveryStrategy
}

// NewRecoveryManager creates a new recovery manager
func NewRecoveryManager(config *RetryConfig) *RecoveryManager {
	if config == nil {
		config = DefaultRetryConfig()
	}

	return &RecoveryManager{
		config:     config,
		strategies: GetRecoveryStrategies(),
	}
}

// GetStrategy returns the recovery strategy for a specific error type
func (rm *RecoveryManager) GetStrategy(errorType ErrorType) *RecoveryStrategy {
	strategy, exists := rm.strategies[errorType]
	if !exists {
		// Return generic strategy for unknown error types
		return rm.strategies[GenericError]
	}
	return strategy
}

// CanAutoRecover checks if an error can be automatically recovered
func (rm *RecoveryManager) CanAutoRecover(errorType ErrorType) bool {
	strategy := rm.GetStrategy(errorType)
	return strategy.AutoRecover
}

// GetSuggestion returns a recovery suggestion for an error type
func (rm *RecoveryManager) GetSuggestion(errorType ErrorType) string {
	strategy := rm.GetStrategy(errorType)
	return strategy.Suggestion
}

// RetryWithBackoff executes a function with retry logic and exponential backoff
func (rm *RecoveryManager) RetryWithBackoff(ctx context.Context, errorType ErrorType, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt < rm.config.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute the function
		if err := fn(); err == nil {
			return nil // Success
		} else {
			lastErr = err
		}

		// Check if we should retry
		if !ShouldRetry(errorType, attempt, rm.config) {
			break
		}

		// Calculate and wait for backoff
		if attempt < rm.config.MaxRetries-1 { // Don't wait after the last attempt
			backoff := CalculateBackoff(attempt, rm.config)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				// Continue to next attempt
			}
		}
	}

	return lastErr
}

// GetRetryConfig returns the current retry configuration
func (rm *RecoveryManager) GetRetryConfig() *RetryConfig {
	return rm.config
}

// UpdateRetryConfig updates the retry configuration
func (rm *RecoveryManager) UpdateRetryConfig(config *RetryConfig) {
	rm.config = config
}
