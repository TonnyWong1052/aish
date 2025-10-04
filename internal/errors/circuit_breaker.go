package errors

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CircuitState 定義斷路器狀態
type CircuitState int

const (
	// StateClosed 關閉狀態（正常工作）
	StateClosed CircuitState = iota
	// StateOpen 開啟狀態（故障保護）
	StateOpen
	// StateHalfOpen 半開狀態（測試恢復）
	StateHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreakerConfig 斷路器配置
type CircuitBreakerConfig struct {
	// 失敗次數閾值
	FailureThreshold int `json:"failure_threshold"`
	// 成功次數閾值（半開狀態下）
	SuccessThreshold int `json:"success_threshold"`
	// 開啟狀態持續時間
	Timeout time.Duration `json:"timeout"`
	// 統計窗口大小
	WindowSize int `json:"window_size"`
	// 最小請求數量
	MinRequests int `json:"min_requests"`
}

// DefaultCircuitBreakerConfig 返回默認斷路器配置
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
		WindowSize:       100,
		MinRequests:      10,
	}
}

// CircuitBreaker 斷路器實現
type CircuitBreaker struct {
	config          *CircuitBreakerConfig
	state           CircuitState
	failures        int
	successes       int
	requests        int
	lastFailureTime time.Time
	mu              sync.RWMutex

	// 統計窗口
	window      []bool // true = success, false = failure
	windowIndex int
}

// NewCircuitBreaker 創建新的斷路器
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}

	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
		window: make([]bool, config.WindowSize),
	}
}

// Execute 執行操作，如果斷路器開啟則快速失敗
func (cb *CircuitBreaker) Execute(ctx context.Context, fn RetryableFunc) error {
	// 檢查斷路器狀態
	if !cb.canExecute() {
		return NewError(ErrTimeout, fmt.Sprintf("斷路器處於 %s 狀態，拒絕執行", cb.state))
	}

	// 執行操作
	err := fn(ctx)

	// 記錄結果
	cb.recordResult(err == nil)

	return err
}

// canExecute 檢查是否可以執行操作
func (cb *CircuitBreaker) canExecute() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// 檢查是否可以轉換到半開狀態
		if time.Since(cb.lastFailureTime) >= cb.config.Timeout {
			cb.state = StateHalfOpen
			cb.successes = 0
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// recordResult 記錄操作結果
func (cb *CircuitBreaker) recordResult(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// 更新統計窗口
	cb.window[cb.windowIndex] = success
	cb.windowIndex = (cb.windowIndex + 1) % cb.config.WindowSize
	cb.requests++

	if success {
		cb.successes++
		cb.onSuccess()
	} else {
		cb.failures++
		cb.lastFailureTime = time.Now()
		cb.onFailure()
	}
}

// onSuccess 處理成功事件
func (cb *CircuitBreaker) onSuccess() {
	switch cb.state {
	case StateHalfOpen:
		if cb.successes >= cb.config.SuccessThreshold {
			cb.state = StateClosed
			cb.reset()
		}
	}
}

// onFailure 處理失敗事件
func (cb *CircuitBreaker) onFailure() {
	switch cb.state {
	case StateClosed:
		if cb.shouldOpen() {
			cb.state = StateOpen
		}
	case StateHalfOpen:
		cb.state = StateOpen
	}
}

// shouldOpen 判斷是否應該開啟斷路器
func (cb *CircuitBreaker) shouldOpen() bool {
	if cb.requests < cb.config.MinRequests {
		return false
	}

	// 計算窗口內的失敗率
	failureCount := 0
	windowRequests := min(cb.requests, cb.config.WindowSize)

	for i := 0; i < windowRequests; i++ {
		if !cb.window[i] {
			failureCount++
		}
	}

	return failureCount >= cb.config.FailureThreshold
}

// reset 重置統計信息
func (cb *CircuitBreaker) reset() {
	cb.failures = 0
	cb.successes = 0
	cb.requests = 0
	cb.windowIndex = 0
	for i := range cb.window {
		cb.window[i] = false
	}
}

// GetState 獲取當前狀態
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats 獲取統計信息
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	failureCount := 0
	successCount := 0
	windowRequests := min(cb.requests, cb.config.WindowSize)

	for i := 0; i < windowRequests; i++ {
		if cb.window[i] {
			successCount++
		} else {
			failureCount++
		}
	}

	return CircuitBreakerStats{
		State:           cb.state,
		Failures:        failureCount,
		Successes:       successCount,
		Requests:        cb.requests,
		LastFailureTime: cb.lastFailureTime,
	}
}

// CircuitBreakerStats 斷路器統計信息
type CircuitBreakerStats struct {
	State           CircuitState `json:"state"`
	Failures        int          `json:"failures"`
	Successes       int          `json:"successes"`
	Requests        int          `json:"requests"`
	LastFailureTime time.Time    `json:"last_failure_time"`
}

// FailureRate 計算失敗率
func (s CircuitBreakerStats) FailureRate() float64 {
	if s.Requests == 0 {
		return 0
	}
	return float64(s.Failures) / float64(s.Requests)
}

// SuccessRate 計算成功率
func (s CircuitBreakerStats) SuccessRate() float64 {
	if s.Requests == 0 {
		return 0
	}
	return float64(s.Successes) / float64(s.Requests)
}

// FallbackFunc 定義回退函數類型
type FallbackFunc func(ctx context.Context, err error) error

// ExecuteWithFallback 執行操作，失敗時使用回退函數
func (cb *CircuitBreaker) ExecuteWithFallback(
	ctx context.Context,
	primaryFn RetryableFunc,
	fallbackFn FallbackFunc,
) error {
	err := cb.Execute(ctx, primaryFn)
	if err != nil && fallbackFn != nil {
		return fallbackFn(ctx, err)
	}
	return err
}

// ExecuteWithRetry 結合重試機制的斷路器執行
func (cb *CircuitBreaker) ExecuteWithRetry(
	ctx context.Context,
	fn RetryableFunc,
	retryConfig *RetryConfig,
) *RetryResult {
	retryManager := NewRetryManager(retryConfig)

	wrappedFn := func(ctx context.Context) error {
		return cb.Execute(ctx, fn)
	}

	return retryManager.Execute(ctx, wrappedFn)
}

// min 返回兩個整數中的較小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// IsCircuitBreakerError 檢查是否為斷路器錯誤
func IsCircuitBreakerError(err error) bool {
	if aishErr, ok := GetAishError(err); ok {
		return aishErr.Code == ErrTimeout &&
			len(aishErr.Message) > 0 &&
			aishErr.Message[0:2] == "斷路器"
	}
	return false
}

// CircuitBreakerManager 管理多個斷路器
type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
}

// NewCircuitBreakerManager 創建斷路器管理器
func NewCircuitBreakerManager() *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// GetOrCreate 獲取或創建斷路器
func (m *CircuitBreakerManager) GetOrCreate(name string, config *CircuitBreakerConfig) *CircuitBreaker {
	m.mu.Lock()
	defer m.mu.Unlock()

	if breaker, exists := m.breakers[name]; exists {
		return breaker
	}

	breaker := NewCircuitBreaker(config)
	m.breakers[name] = breaker
	return breaker
}

// Get 獲取斷路器
func (m *CircuitBreakerManager) Get(name string) (*CircuitBreaker, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	breaker, exists := m.breakers[name]
	return breaker, exists
}

// GetAllStats 獲取所有斷路器的統計信息
func (m *CircuitBreakerManager) GetAllStats() map[string]CircuitBreakerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]CircuitBreakerStats)
	for name, breaker := range m.breakers {
		stats[name] = breaker.GetStats()
	}
	return stats
}

// Remove 移除斷路器
func (m *CircuitBreakerManager) Remove(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.breakers, name)
}

// Clear 清除所有斷路器
func (m *CircuitBreakerManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.breakers = make(map[string]*CircuitBreaker)
}
