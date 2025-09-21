package errors

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryManager_Execute_Success(t *testing.T) {
	manager := NewRetryManager(nil)
	
	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		return nil // 成功
	}
	
	result := manager.Execute(context.Background(), fn)
	
	if !result.Success {
		t.Error("Expected success")
	}
	if result.Attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", result.Attempts)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}
}

func TestRetryManager_Execute_RetryableError(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  time.Millisecond,
		MaxDelay:      time.Second,
		BackoffFactor: 2.0,
		Jitter:        false,
	}
	manager := NewRetryManager(config)
	
	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		if callCount < 3 {
			return NewRetryableError(ErrNetwork, "Network error")
		}
		return nil // 第三次成功
	}
	
	result := manager.Execute(context.Background(), fn)
	
	if !result.Success {
		t.Error("Expected success after retries")
	}
	if result.Attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", result.Attempts)
	}
	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}
}

func TestRetryManager_Execute_NonRetryableError(t *testing.T) {
	manager := NewRetryManager(nil)
	
	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		return NewError(ErrUserCancel, "User cancelled")
	}
	
	result := manager.Execute(context.Background(), fn)
	
	if result.Success {
		t.Error("Expected failure")
	}
	if result.Attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", result.Attempts)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}
}

func TestRetryManager_Execute_MaxAttemptsReached(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:   2,
		InitialDelay:  time.Millisecond,
		MaxDelay:      time.Second,
		BackoffFactor: 2.0,
		Jitter:        false,
	}
	manager := NewRetryManager(config)
	
	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		return NewRetryableError(ErrNetwork, "Network error")
	}
	
	result := manager.Execute(context.Background(), fn)
	
	if result.Success {
		t.Error("Expected failure")
	}
	if result.Attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", result.Attempts)
	}
	if callCount != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
}

func TestRetryManager_Execute_ContextCancellation(t *testing.T) {
	manager := NewRetryManager(nil)
	
	ctx, cancel := context.WithCancel(context.Background())
	
	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		if callCount == 1 {
			cancel() // 取消上下文
			return NewRetryableError(ErrNetwork, "Network error")
		}
		return nil
	}
	
	result := manager.Execute(ctx, fn)
	
	if result.Success {
		t.Error("Expected failure due to context cancellation")
	}
	if result.LastError != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", result.LastError)
	}
}

func TestRetryManager_ExecuteWithCallback(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  time.Millisecond,
		MaxDelay:      time.Second,
		BackoffFactor: 2.0,
		Jitter:        false,
	}
	manager := NewRetryManager(config)
	
	callbackCalls := 0
	var callbackErrors []error
	var callbackAttempts []int
	
	onRetry := func(attempt int, err error) {
		callbackCalls++
		callbackErrors = append(callbackErrors, err)
		callbackAttempts = append(callbackAttempts, attempt)
	}
	
	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		if callCount < 3 {
			return NewRetryableError(ErrNetwork, "Network error")
		}
		return nil
	}
	
	result := manager.ExecuteWithCallback(context.Background(), fn, onRetry)
	
	if !result.Success {
		t.Error("Expected success")
	}
	if callbackCalls != 2 {
		t.Errorf("Expected 2 callback calls, got %d", callbackCalls)
	}
}

func TestShouldRetry(t *testing.T) {
	manager := NewRetryManager(nil)
	
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "retryable AISH error",
			err:      NewRetryableError(ErrNetwork, "Network error"),
			expected: true,
		},
		{
			name:     "non-retryable AISH error",
			err:      NewError(ErrUserCancel, "User cancelled"),
			expected: false,
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: false,
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("generic error"),
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := manager.shouldRetry(tt.err)
			if got != tt.expected {
				t.Errorf("shouldRetry() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCalculateDelay(t *testing.T) {
	tests := []struct {
		name      string
		config    *RetryConfig
		baseDelay time.Duration
		attempt   int
		minDelay  time.Duration
		maxDelay  time.Duration
	}{
		{
			name: "no jitter",
			config: &RetryConfig{
				Jitter: false,
			},
			baseDelay: time.Second,
			attempt:   1,
			minDelay:  time.Second,
			maxDelay:  time.Second,
		},
		{
			name: "with jitter",
			config: &RetryConfig{
				Jitter: true,
			},
			baseDelay: time.Second,
			attempt:   1,
			minDelay:  750 * time.Millisecond, // ~25% less
			maxDelay:  1250 * time.Millisecond, // ~25% more
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewRetryManager(tt.config)
			delay := manager.calculateDelay(tt.baseDelay, tt.attempt)
			
			if delay < tt.minDelay || delay > tt.maxDelay {
				t.Errorf("calculateDelay() = %v, want between %v and %v", 
					delay, tt.minDelay, tt.maxDelay)
			}
		})
	}
}

func TestRetry_ConvenienceFunction(t *testing.T) {
	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		if callCount < 2 {
			return NewRetryableError(ErrNetwork, "Network error")
		}
		return nil
	}
	
	result := Retry(context.Background(), fn)
	
	if !result.Success {
		t.Error("Expected success")
	}
	if result.Attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", result.Attempts)
	}
}

func TestRetryWithConfig_ConvenienceFunction(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:   5,
		InitialDelay:  time.Millisecond,
		MaxDelay:      time.Second,
		BackoffFactor: 1.5,
		Jitter:        false,
	}
	
	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		if callCount < 4 {
			return NewRetryableError(ErrNetwork, "Network error")
		}
		return nil
	}
	
	result := RetryWithConfig(context.Background(), config, fn)
	
	if !result.Success {
		t.Error("Expected success")
	}
	if result.Attempts != 4 {
		t.Errorf("Expected 4 attempts, got %d", result.Attempts)
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "retryable error",
			err:      NewRetryableError(ErrNetwork, "Network error"),
			expected: true,
		},
		{
			name:     "non-retryable error",
			err:      NewError(ErrUserCancel, "User cancelled"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("generic error"),
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRetryableError(tt.err)
			if got != tt.expected {
				t.Errorf("IsRetryableError() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestWrapRetryableError(t *testing.T) {
	originalErr := errors.New("original error")
	wrappedErr := WrapRetryableError(originalErr, ErrNetwork, "Network operation failed")
	
	if wrappedErr == nil {
		t.Fatal("Expected non-nil error")
	}
	
	if !wrappedErr.IsRetryable() {
		t.Error("Expected error to be retryable")
	}
	
	if wrappedErr.Code != ErrNetwork {
		t.Errorf("Expected code %v, got %v", ErrNetwork, wrappedErr.Code)
	}
	
	if wrappedErr.Unwrap() != originalErr {
		t.Error("Expected wrapped error to unwrap to original error")
	}
}

func TestFormatRetryResult(t *testing.T) {
	tests := []struct {
		name     string
		result   *RetryResult
		contains string
	}{
		{
			name: "single attempt success",
			result: &RetryResult{
				Success:   true,
				Attempts:  1,
				TotalTime: time.Millisecond * 100,
			},
			contains: "操作成功完成",
		},
		{
			name: "multiple attempts success",
			result: &RetryResult{
				Success:   true,
				Attempts:  3,
				TotalTime: time.Millisecond * 500,
			},
			contains: "3 次嘗試後成功完成",
		},
		{
			name: "failure",
			result: &RetryResult{
				Success:   false,
				Attempts:  3,
				TotalTime: time.Millisecond * 500,
				LastError: errors.New("final error"),
			},
			contains: "操作失敗，已嘗試 3 次",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatted := FormatRetryResult(tt.result)
			if !contains(formatted, tt.contains) {
				t.Errorf("Expected formatted result to contain %q, got %q", 
					tt.contains, formatted)
			}
		})
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		(len(substr) == 0 || findSubstring(s, substr) >= 0)
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}