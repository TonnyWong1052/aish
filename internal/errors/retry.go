package errors

import (
	"context"
	"fmt"
	"math"
	"time"
)

// RetryConfig 定義重試配置
type RetryConfig struct {
	MaxAttempts   int           `json:"max_attempts"`   // 最大重試次數
	InitialDelay  time.Duration `json:"initial_delay"`  // 初始延遲
	MaxDelay      time.Duration `json:"max_delay"`      // 最大延遲
	BackoffFactor float64       `json:"backoff_factor"` // 回退因數
	Jitter        bool          `json:"jitter"`         // 是否加入隨機抖動
}

// DefaultRetryConfig 返回默認重試配置
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
	}
}

// RetryableFunc 定義可重試的函數類型
type RetryableFunc func(ctx context.Context) error

// RetryResult 重試結果
type RetryResult struct {
	Success    bool          `json:"success"`
	Attempts   int           `json:"attempts"`
	LastError  error         `json:"last_error"`
	TotalTime  time.Duration `json:"total_time"`
	FirstError error         `json:"first_error"`
}

// RetryManager 重試管理器
type RetryManager struct {
	config *RetryConfig
}

// NewRetryManager 創建新的重試管理器
func NewRetryManager(config *RetryConfig) *RetryManager {
	if config == nil {
		config = DefaultRetryConfig()
	}
	return &RetryManager{
		config: config,
	}
}

// Execute 執行可重試的函數
func (r *RetryManager) Execute(ctx context.Context, fn RetryableFunc) *RetryResult {
	result := &RetryResult{
		Success: false,
	}

	startTime := time.Now()
	defer func() {
		result.TotalTime = time.Since(startTime)
	}()

	delay := r.config.InitialDelay

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		result.Attempts = attempt

		// 檢查上下文是否被取消
		select {
		case <-ctx.Done():
			result.LastError = ctx.Err()
			return result
		default:
		}

		// 執行函數
		err := fn(ctx)
		if err == nil {
			result.Success = true
			return result
		}

		if result.FirstError == nil {
			result.FirstError = err
		}
		result.LastError = err

		// 檢查錯誤是否可重試
		if !r.shouldRetry(err) {
			break
		}

		// 如果不是最後一次嘗試，則等待
		if attempt < r.config.MaxAttempts {
			sleepTime := r.calculateDelay(delay, attempt)

			select {
			case <-ctx.Done():
				result.LastError = ctx.Err()
				return result
			case <-time.After(sleepTime):
				// 更新下次延遲時間
				delay = time.Duration(float64(delay) * r.config.BackoffFactor)
				if delay > r.config.MaxDelay {
					delay = r.config.MaxDelay
				}
			}
		}
	}

	return result
}

// ExecuteWithCallback 執行可重試的函數並提供回調
func (r *RetryManager) ExecuteWithCallback(
	ctx context.Context,
	fn RetryableFunc,
	onRetry func(attempt int, err error),
) *RetryResult {
	originalFn := fn
	fn = func(ctx context.Context) error {
		err := originalFn(ctx)
		if err != nil && onRetry != nil {
			// 這裡的 attempt 會在主循環中設置
			onRetry(0, err) // 0 表示當前嘗試，實際值由調用方設置
		}
		return err
	}

	result := &RetryResult{
		Success: false,
	}

	startTime := time.Now()
	defer func() {
		result.TotalTime = time.Since(startTime)
	}()

	delay := r.config.InitialDelay

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		result.Attempts = attempt

		// 檢查上下文是否被取消
		select {
		case <-ctx.Done():
			result.LastError = ctx.Err()
			return result
		default:
		}

		// 執行函數
		err := originalFn(ctx)
		if err == nil {
			result.Success = true
			return result
		}

		if result.FirstError == nil {
			result.FirstError = err
		}
		result.LastError = err

		// 調用重試回調
		if onRetry != nil {
			onRetry(attempt, err)
		}

		// 檢查錯誤是否可重試
		if !r.shouldRetry(err) {
			break
		}

		// 如果不是最後一次嘗試，則等待
		if attempt < r.config.MaxAttempts {
			sleepTime := r.calculateDelay(delay, attempt)

			select {
			case <-ctx.Done():
				result.LastError = ctx.Err()
				return result
			case <-time.After(sleepTime):
				// 更新下次延遲時間
				delay = time.Duration(float64(delay) * r.config.BackoffFactor)
				if delay > r.config.MaxDelay {
					delay = r.config.MaxDelay
				}
			}
		}
	}

	return result
}

// shouldRetry 判斷錯誤是否應該重試
func (r *RetryManager) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	// 檢查是否為 AISH 錯誤
	if aishErr, ok := GetAishError(err); ok {
		return aishErr.IsRetryable()
	}

	// 對於非 AISH 錯誤，檢查常見的可重試錯誤類型
	switch err {
	case context.DeadlineExceeded, context.Canceled:
		return false // 上下文錯誤不重試
	default:
		// 可以根據需要添加更多的錯誤類型判斷
		return false
	}
}

// calculateDelay 計算延遲時間（包含可選的抖動）
func (r *RetryManager) calculateDelay(baseDelay time.Duration, attempt int) time.Duration {
	delay := baseDelay

	if r.config.Jitter {
		// 添加 ±25% 的隨機抖動
		jitterRange := float64(delay) * 0.25
		jitter := (math.Pow(-1, float64(attempt)) * jitterRange) / 2
		delay = time.Duration(float64(delay) + jitter)
	}

	if delay < 0 {
		delay = time.Millisecond * 100
	}

	return delay
}

// Retry 便利函數，使用默認配置重試
func Retry(ctx context.Context, fn RetryableFunc) *RetryResult {
	manager := NewRetryManager(nil)
	return manager.Execute(ctx, fn)
}

// RetryWithConfig 便利函數，使用自定義配置重試
func RetryWithConfig(ctx context.Context, config *RetryConfig, fn RetryableFunc) *RetryResult {
	manager := NewRetryManager(config)
	return manager.Execute(ctx, fn)
}

// IsRetryableError 檢查錯誤是否可重試
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	if aishErr, ok := GetAishError(err); ok {
		return aishErr.IsRetryable()
	}

	return false
}

// WrapRetryableError 包裝錯誤為可重試錯誤
func WrapRetryableError(err error, code ErrorCode, message string) *AishError {
	if err == nil {
		return nil
	}

	aishErr := WrapError(err, code, message)
	aishErr.Retryable = true
	return aishErr
}

// FormatRetryResult 格式化重試結果為用戶友好的消息
func FormatRetryResult(result *RetryResult) string {
	if result.Success {
		if result.Attempts == 1 {
			return "操作成功完成"
		}
		return fmt.Sprintf("操作在 %d 次嘗試後成功完成（總時間: %v）",
			result.Attempts, result.TotalTime.Round(time.Millisecond))
	}

	return fmt.Sprintf("操作失敗，已嘗試 %d 次（總時間: %v）：%v",
		result.Attempts, result.TotalTime.Round(time.Millisecond), result.LastError)
}
