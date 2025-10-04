package classification

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries to be 3, got %d", config.MaxRetries)
	}

	if config.BackoffFactor != 100*time.Millisecond {
		t.Errorf("Expected BackoffFactor to be 100ms, got %v", config.BackoffFactor)
	}

	if !config.ExponentialBackoff {
		t.Error("Expected ExponentialBackoff to be true")
	}

	expectedRetryableErrors := []ErrorType{
		NetworkError,
		TimeoutError,
		DatabaseError,
		MemoryError,
		DiskSpaceError,
	}

	if len(config.RetryableErrors) != len(expectedRetryableErrors) {
		t.Errorf("Expected %d retryable errors, got %d", len(expectedRetryableErrors), len(config.RetryableErrors))
	}
}

func TestIsRetryable(t *testing.T) {
	config := DefaultRetryConfig()

	testCases := []struct {
		errorType ErrorType
		expected  bool
	}{
		{NetworkError, true},
		{TimeoutError, true},
		{DatabaseError, true},
		{MemoryError, true},
		{DiskSpaceError, true},
		{CommandNotFound, false},
		{PermissionDenied, false},
		{ConfigError, false},
		{AuthenticationError, false},
	}

	for _, tc := range testCases {
		t.Run(string(tc.errorType), func(t *testing.T) {
			result := IsRetryable(tc.errorType, config)
			if result != tc.expected {
				t.Errorf("Expected %v for %v, got %v", tc.expected, tc.errorType, result)
			}
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	config := DefaultRetryConfig()

	testCases := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},  // base
		{1, 200 * time.Millisecond},  // base * 2^1
		{2, 400 * time.Millisecond},  // base * 2^2
		{3, 800 * time.Millisecond},  // base * 2^3
		{4, 1600 * time.Millisecond}, // base * 2^4
		{5, 3200 * time.Millisecond}, // base * 2^5
	}

	for _, tc := range testCases {
		t.Run(string(rune(tc.attempt)), func(t *testing.T) {
			result := CalculateBackoff(tc.attempt, config)
			if result != tc.expected {
				t.Errorf("Expected backoff %v for attempt %d, got %v", tc.expected, tc.attempt, result)
			}
		})
	}

	// Test max backoff cap
	config.MaxBackoff = 1 * time.Second
	result := CalculateBackoff(10, config) // Should be capped
	if result != config.MaxBackoff {
		t.Errorf("Expected backoff to be capped at %v, got %v", config.MaxBackoff, result)
	}
}

func TestLinearBackoff(t *testing.T) {
	config := DefaultRetryConfig()
	config.ExponentialBackoff = false

	for attempt := 0; attempt < 5; attempt++ {
		result := CalculateBackoff(attempt, config)
		if result != config.BackoffFactor {
			t.Errorf("Expected linear backoff %v for attempt %d, got %v", config.BackoffFactor, attempt, result)
		}
	}
}

func TestShouldRetry(t *testing.T) {
	config := DefaultRetryConfig()

	testCases := []struct {
		name      string
		errorType ErrorType
		attempt   int
		expected  bool
	}{
		{"retryable error within limit", NetworkError, 1, true},
		{"retryable error at limit", NetworkError, 3, false},
		{"retryable error over limit", NetworkError, 4, false},
		{"non-retryable error", CommandNotFound, 1, false},
		{"retryable error first attempt", TimeoutError, 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ShouldRetry(tc.errorType, tc.attempt, config)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestRecoveryManager(t *testing.T) {
	manager := NewRecoveryManager(nil) // Should use default config

	// Test strategy retrieval
	strategy := manager.GetStrategy(NetworkError)
	if !strategy.Retryable {
		t.Error("NetworkError should be retryable")
	}
	if !strategy.AutoRecover {
		t.Error("NetworkError should support auto-recovery")
	}

	strategy = manager.GetStrategy(CommandNotFound)
	if strategy.Retryable {
		t.Error("CommandNotFound should not be retryable")
	}
	if strategy.AutoRecover {
		t.Error("CommandNotFound should not support auto-recovery")
	}

	// Test unknown error type
	strategy = manager.GetStrategy(ErrorType("UnknownError"))
	if strategy != manager.GetStrategy(GenericError) {
		t.Error("Unknown error type should return GenericError strategy")
	}
}

func TestCanAutoRecover(t *testing.T) {
	manager := NewRecoveryManager(nil)

	testCases := []struct {
		errorType ErrorType
		expected  bool
	}{
		{NetworkError, true},
		{TimeoutError, true},
		{DatabaseError, false},
		{CommandNotFound, false},
		{ConfigError, false},
	}

	for _, tc := range testCases {
		t.Run(string(tc.errorType), func(t *testing.T) {
			result := manager.CanAutoRecover(tc.errorType)
			if result != tc.expected {
				t.Errorf("Expected %v for %v, got %v", tc.expected, tc.errorType, result)
			}
		})
	}
}

func TestGetSuggestion(t *testing.T) {
	manager := NewRecoveryManager(nil)

	suggestion := manager.GetSuggestion(NetworkError)
	if suggestion == "" {
		t.Error("NetworkError should have a suggestion")
	}

	suggestion = manager.GetSuggestion(CommandNotFound)
	if suggestion == "" {
		t.Error("CommandNotFound should have a suggestion")
	}
}

func TestRetryWithBackoff(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:         3,
		BackoffFactor:      10 * time.Millisecond,
		RetryableErrors:    []ErrorType{NetworkError},
		MaxBackoff:         100 * time.Millisecond,
		ExponentialBackoff: false, // Use linear for faster tests
	}

	manager := NewRecoveryManager(config)
	ctx := context.Background()

	t.Run("success on first attempt", func(t *testing.T) {
		attempts := 0
		err := manager.RetryWithBackoff(ctx, NetworkError, func() error {
			attempts++
			return nil // Success
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("success on second attempt", func(t *testing.T) {
		attempts := 0
		err := manager.RetryWithBackoff(ctx, NetworkError, func() error {
			attempts++
			if attempts == 1 {
				return errors.New("temporary failure")
			}
			return nil // Success on second attempt
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if attempts != 2 {
			t.Errorf("Expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("failure after max retries", func(t *testing.T) {
		attempts := 0
		testErr := errors.New("persistent failure")
		err := manager.RetryWithBackoff(ctx, NetworkError, func() error {
			attempts++
			return testErr
		})

		if err != testErr {
			t.Errorf("Expected %v, got %v", testErr, err)
		}
		if attempts != config.MaxRetries {
			t.Errorf("Expected %d attempts, got %d", config.MaxRetries, attempts)
		}
	})

	t.Run("non-retryable error", func(t *testing.T) {
		attempts := 0
		testErr := errors.New("non-retryable")
		err := manager.RetryWithBackoff(ctx, CommandNotFound, func() error {
			attempts++
			return testErr
		})

		if err != testErr {
			t.Errorf("Expected %v, got %v", testErr, err)
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts)
		}
	})
}

func TestRetryWithContext(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:         5,
		BackoffFactor:      50 * time.Millisecond,
		RetryableErrors:    []ErrorType{NetworkError},
		MaxBackoff:         100 * time.Millisecond,
		ExponentialBackoff: false,
	}

	manager := NewRecoveryManager(config)

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		attempts := 0
		go func() {
			time.Sleep(25 * time.Millisecond) // Cancel after a short delay
			cancel()
		}()

		err := manager.RetryWithBackoff(ctx, NetworkError, func() error {
			attempts++
			return errors.New("will be cancelled")
		})

		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}

		// Should have made at least one attempt but not all retries
		if attempts == 0 {
			t.Error("Expected at least one attempt")
		}
		if attempts >= config.MaxRetries {
			t.Errorf("Expected fewer than %d attempts due to cancellation, got %d", config.MaxRetries, attempts)
		}
	})
}
