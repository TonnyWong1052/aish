package errors

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_Execute_Success(t *testing.T) {
	cb := NewCircuitBreaker(nil)

	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		return nil // 成功
	}

	err := cb.Execute(context.Background(), fn)
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}

	if cb.GetState() != StateClosed {
		t.Errorf("Expected CLOSED state, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_Execute_Failure_OpensCircuit(t *testing.T) {
	config := &CircuitBreakerConfig{
		FailureThreshold: 3,
		MinRequests:      1,
		WindowSize:       10,
	}
	cb := NewCircuitBreaker(config)

	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		return errors.New("test error")
	}

	// 執行足夠的失敗請求以開啟斷路器
	for i := 0; i < 5; i++ {
		_ = cb.Execute(context.Background(), fn)
	}

	stats := cb.GetStats()
	if stats.State != StateOpen {
		t.Errorf("Expected OPEN state, got %s", stats.State)
	}

	if stats.Failures < 3 {
		t.Errorf("Expected at least 3 failures, got %d", stats.Failures)
	}
}

func TestCircuitBreaker_Execute_HalfOpen_Recovery(t *testing.T) {
	config := &CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		MinRequests:      1,
		WindowSize:       10,
		Timeout:          time.Millisecond, // 很短的超時
	}
	cb := NewCircuitBreaker(config)

	// 先讓斷路器開啟
	failingFn := func(ctx context.Context) error {
		return errors.New("test error")
	}

	for i := 0; i < 3; i++ {
		_ = cb.Execute(context.Background(), failingFn)
	}

	if cb.GetState() != StateOpen {
		t.Error("Circuit breaker should be OPEN")
	}

	// 等待超時，讓斷路器進入半開狀態
	time.Sleep(2 * time.Millisecond)

	// 執行成功的函數
	successFn := func(ctx context.Context) error {
		return nil
	}

	// 第一次執行應該讓斷路器進入半開狀態
	err := cb.Execute(context.Background(), successFn)
	if err != nil {
		t.Errorf("First execution after timeout failed: %v", err)
	}

	// 繼續執行成功的函數直到斷路器關閉
	for i := 0; i < 3; i++ {
		_ = cb.Execute(context.Background(), successFn)
	}

	if cb.GetState() != StateClosed {
		t.Errorf("Expected CLOSED state after recovery, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_ExecuteWithFallback(t *testing.T) {
	config := &CircuitBreakerConfig{
		FailureThreshold: 1,
		MinRequests:      1,
		WindowSize:       10,
	}
	cb := NewCircuitBreaker(config)

	// 先讓斷路器開啟
	failingFn := func(ctx context.Context) error {
		return errors.New("primary error")
	}

	for i := 0; i < 2; i++ {
		cb.Execute(context.Background(), failingFn)
	}

	fallbackCalled := false
	fallbackFn := func(ctx context.Context, err error) error {
		fallbackCalled = true
		return nil
	}

	err := cb.ExecuteWithFallback(context.Background(), failingFn, fallbackFn)
	if err != nil {
		t.Errorf("Expected fallback to handle error, got: %v", err)
	}

	if !fallbackCalled {
		t.Error("Fallback function was not called")
	}
}

func TestCircuitBreaker_ExecuteWithRetry(t *testing.T) {
	cb := NewCircuitBreaker(nil)

	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		if callCount < 3 {
			return NewRetryableError(ErrNetwork, "Network error")
		}
		return nil
	}

	retryConfig := &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  time.Millisecond,
		BackoffFactor: 1.0,
		Jitter:        false,
	}

	result := cb.ExecuteWithRetry(context.Background(), fn, retryConfig)

	if !result.Success {
		t.Error("Expected retry to eventually succeed")
	}

	if result.Attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", result.Attempts)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 function calls, got %d", callCount)
	}
}

func TestCircuitBreakerManager(t *testing.T) {
	manager := NewCircuitBreakerManager()

	// 創建斷路器
	config := &CircuitBreakerConfig{
		FailureThreshold: 5,
	}
	cb1 := manager.GetOrCreate("service1", config)
	cb2 := manager.GetOrCreate("service2", config)

	if cb1 == cb2 {
		t.Error("Expected different circuit breakers for different services")
	}

	// 獲取相同服務應該返回相同的斷路器
	cb1Again := manager.GetOrCreate("service1", config)
	if cb1 != cb1Again {
		t.Error("Expected same circuit breaker for same service")
	}

	// 測試獲取
	retrievedCb, exists := manager.Get("service1")
	if !exists {
		t.Error("Expected to find service1 circuit breaker")
	}
	if retrievedCb != cb1 {
		t.Error("Retrieved circuit breaker should be the same instance")
	}

	// 測試統計
	stats := manager.GetAllStats()
	if len(stats) != 2 {
		t.Errorf("Expected 2 circuit breakers, got %d", len(stats))
	}

	// 測試移除
	manager.Remove("service1")
	_, exists = manager.Get("service1")
	if exists {
		t.Error("Expected service1 to be removed")
	}

	// 測試清除
	manager.Clear()
	stats = manager.GetAllStats()
	if len(stats) != 0 {
		t.Errorf("Expected 0 circuit breakers after clear, got %d", len(stats))
	}
}

func TestCircuitBreakerStats(t *testing.T) {
	cb := NewCircuitBreaker(nil)

	// 執行一些操作
	successFn := func(ctx context.Context) error { return nil }
	failureFn := func(ctx context.Context) error { return errors.New("error") }

	cb.Execute(context.Background(), successFn)
	cb.Execute(context.Background(), successFn)
	cb.Execute(context.Background(), failureFn)

	stats := cb.GetStats()

	if stats.Successes != 2 {
		t.Errorf("Expected 2 successes, got %d", stats.Successes)
	}

	if stats.Failures != 1 {
		t.Errorf("Expected 1 failure, got %d", stats.Failures)
	}

	expectedFailureRate := 1.0 / 3.0
	if stats.FailureRate() != expectedFailureRate {
		t.Errorf("Expected failure rate %.3f, got %.3f",
			expectedFailureRate, stats.FailureRate())
	}

	expectedSuccessRate := 2.0 / 3.0
	if stats.SuccessRate() != expectedSuccessRate {
		t.Errorf("Expected success rate %.3f, got %.3f",
			expectedSuccessRate, stats.SuccessRate())
	}
}

func TestIsCircuitBreakerError(t *testing.T) {
	// 斷路器錯誤
	cbError := NewError(ErrTimeout, "斷路器處於 OPEN 狀態，拒絕執行")
	if !IsCircuitBreakerError(cbError) {
		t.Error("Expected circuit breaker error to be identified")
	}

	// 普通錯誤
	normalError := NewError(ErrNetwork, "Network error")
	if IsCircuitBreakerError(normalError) {
		t.Error("Expected normal error not to be identified as circuit breaker error")
	}

	// 非 AISH 錯誤
	genericError := errors.New("generic error")
	if IsCircuitBreakerError(genericError) {
		t.Error("Expected generic error not to be identified as circuit breaker error")
	}
}

func TestCircuitBreakerConfig_Defaults(t *testing.T) {
	config := DefaultCircuitBreakerConfig()

	if config.FailureThreshold != 5 {
		t.Errorf("Expected default failure threshold 5, got %d", config.FailureThreshold)
	}

	if config.SuccessThreshold != 3 {
		t.Errorf("Expected default success threshold 3, got %d", config.SuccessThreshold)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", config.Timeout)
	}

	if config.WindowSize != 100 {
		t.Errorf("Expected default window size 100, got %d", config.WindowSize)
	}

	if config.MinRequests != 10 {
		t.Errorf("Expected default min requests 10, got %d", config.MinRequests)
	}
}

// 基準測試
func BenchmarkCircuitBreaker_Execute_Success(b *testing.B) {
	cb := NewCircuitBreaker(nil)
	fn := func(ctx context.Context) error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Execute(context.Background(), fn)
	}
}

func BenchmarkCircuitBreaker_Execute_Failure(b *testing.B) {
	cb := NewCircuitBreaker(nil)
	fn := func(ctx context.Context) error {
		return errors.New("test error")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Execute(context.Background(), fn)
	}
}

func BenchmarkCircuitBreakerManager_GetOrCreate(b *testing.B) {
	manager := NewCircuitBreakerManager()
	config := DefaultCircuitBreakerConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		serviceName := "service" + string(rune(i%10))
		manager.GetOrCreate(serviceName, config)
	}
}
