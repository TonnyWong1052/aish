package llm

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/TonnyWong1052/aish/internal/errors"
)

// EnhancedHTTPClient 增強版 HTTP 客戶端，整合了重試機制和斷路器
type EnhancedHTTPClient struct {
	client          *http.Client
	retryManager    *errors.RetryManager
	circuitBreaker  *errors.CircuitBreaker
	metricsCollector *HTTPMetricsCollector
	config          *EnhancedHTTPConfig
}

// EnhancedHTTPConfig 增強版 HTTP 客戶端配置
type EnhancedHTTPConfig struct {
	// HTTP 設置
	Timeout                time.Duration `json:"timeout"`
	MaxIdleConns           int           `json:"max_idle_conns"`
	MaxIdleConnsPerHost    int           `json:"max_idle_conns_per_host"`
	IdleConnTimeout        time.Duration `json:"idle_conn_timeout"`
	TLSHandshakeTimeout    time.Duration `json:"tls_handshake_timeout"`
	ResponseHeaderTimeout  time.Duration `json:"response_header_timeout"`
	
	// 重試設置
	RetryConfig *errors.RetryConfig `json:"retry_config"`
	
	// 斷路器設置
	CircuitBreakerConfig *errors.CircuitBreakerConfig `json:"circuit_breaker_config"`
	
	// 指標收集
	EnableMetrics bool `json:"enable_metrics"`
	
	// TLS 設置
	TLSConfig *tls.Config `json:"-"`
	
	// 自定義請求檢查函數
	IsRetryableStatusCode func(statusCode int) bool `json:"-"`
}

// DefaultEnhancedHTTPConfig 返回默認的增強版 HTTP 客戶端配置
func DefaultEnhancedHTTPConfig() *EnhancedHTTPConfig {
	return &EnhancedHTTPConfig{
		Timeout:               30 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		RetryConfig:           errors.DefaultRetryConfig(),
		CircuitBreakerConfig:  errors.DefaultCircuitBreakerConfig(),
		EnableMetrics:         true,
		IsRetryableStatusCode: DefaultRetryableStatusCheck,
	}
}

// DefaultRetryableStatusCheck 默認的重試狀態碼檢查函數
func DefaultRetryableStatusCheck(statusCode int) bool {
	// 重試 5xx 伺服器錯誤和 429 限流錯誤
	return statusCode >= 500 || statusCode == 429
}

// NewEnhancedHTTPClient 創建新的增強版 HTTP 客戶端
func NewEnhancedHTTPClient(config *EnhancedHTTPConfig) *EnhancedHTTPClient {
	if config == nil {
		config = DefaultEnhancedHTTPConfig()
	}
	
	// 創建 HTTP 傳輸層
	transport := &http.Transport{
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ResponseHeaderTimeout: config.ResponseHeaderTimeout,
		ForceAttemptHTTP2:     true,
		Proxy:                 http.ProxyFromEnvironment,
		TLSClientConfig:       config.TLSConfig,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ExpectContinueTimeout: 1 * time.Second,
	}
	
	client := &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}
	
	enhancedClient := &EnhancedHTTPClient{
		client:         client,
		retryManager:   errors.NewRetryManager(config.RetryConfig),
		circuitBreaker: errors.NewCircuitBreaker(config.CircuitBreakerConfig),
		config:         config,
	}
	
	// 啟用指標收集
	if config.EnableMetrics {
		enhancedClient.metricsCollector = NewHTTPMetricsCollector()
	}
	
	return enhancedClient
}

// Do 執行 HTTP 請求（包含重試和斷路器保護）
func (c *EnhancedHTTPClient) Do(req *http.Request) (*http.Response, error) {
	startTime := time.Now()
	
	// 使用斷路器保護的可重試函數
	var result *http.Response
	var httpErr error
	
	retryResult := c.retryManager.ExecuteWithCallback(req.Context(), func(ctx context.Context) error {
		return c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
			// 複製請求以避免重試時的問題
			reqClone := req.Clone(ctx)
			
			resp, err := c.client.Do(reqClone)
			if err != nil {
				return errors.WrapRetryableError(err, errors.ErrNetwork, "HTTP 請求失敗")
			}
			
			// 檢查狀態碼是否可重試
			if c.config.IsRetryableStatusCode != nil && c.config.IsRetryableStatusCode(resp.StatusCode) {
				resp.Body.Close()
				return errors.NewRetryableError(errors.ErrProviderRequest, 
					fmt.Sprintf("HTTP 狀態碼 %d 表示暫時性錯誤", resp.StatusCode))
			}
			
			result = resp
			return nil
		})
	}, func(attempt int, err error) {
		if c.metricsCollector != nil {
			c.metricsCollector.RecordRetry(req.Method, req.URL.Host, attempt)
		}
	})
	
	// 記錄指標
	if c.metricsCollector != nil {
		duration := time.Since(startTime)
		statusCode := 0
		if result != nil {
			statusCode = result.StatusCode
		}
		
		c.metricsCollector.RecordRequest(
			req.Method,
			req.URL.Host,
			statusCode,
			duration,
			retryResult.Success,
			retryResult.Attempts,
		)
	}
	
	if !retryResult.Success {
		return nil, retryResult.LastError
	}
	
	return result, httpErr
}

// DoWithTimeout 使用自定義超時執行請求
func (c *EnhancedHTTPClient) DoWithTimeout(req *http.Request, timeout time.Duration) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(req.Context(), timeout)
	defer cancel()
	
	return c.Do(req.WithContext(ctx))
}

// HealthCheck 執行健康檢查
func (c *EnhancedHTTPClient) HealthCheck(ctx context.Context, url string, headers map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return errors.WrapError(err, errors.ErrNetwork, "創建健康檢查請求失敗")
	}
	
	// 添加標頭
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	
	resp, err := c.Do(req)
	if err != nil {
		return errors.WrapError(err, errors.ErrNetwork, "健康檢查請求失敗")
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		return errors.NewError(errors.ErrProviderResponse, 
			fmt.Sprintf("健康檢查失敗，狀態碼: %d", resp.StatusCode))
	}
	
	return nil
}

// GetMetrics 獲取 HTTP 客戶端指標
func (c *EnhancedHTTPClient) GetMetrics() *HTTPMetrics {
	if c.metricsCollector != nil {
		return c.metricsCollector.GetMetrics()
	}
	return nil
}

// GetCircuitBreakerStats 獲取斷路器統計信息
func (c *EnhancedHTTPClient) GetCircuitBreakerStats() errors.CircuitBreakerStats {
	return c.circuitBreaker.GetStats()
}

// Close 關閉客戶端連接
func (c *EnhancedHTTPClient) Close() {
	if transport, ok := c.client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}

// HTTPMetricsCollector HTTP 指標收集器
type HTTPMetricsCollector struct {
	mu      sync.RWMutex
	metrics *HTTPMetrics
}

// HTTPMetrics HTTP 指標數據
type HTTPMetrics struct {
	TotalRequests    int64                    `json:"total_requests"`
	SuccessfulRequests int64                  `json:"successful_requests"`
	FailedRequests   int64                    `json:"failed_requests"`
	TotalRetries     int64                    `json:"total_retries"`
	AverageLatency   time.Duration            `json:"average_latency"`
	StatusCodeCounts map[int]int64            `json:"status_code_counts"`
	HostMetrics      map[string]*HostMetrics  `json:"host_metrics"`
	LastUpdated      time.Time                `json:"last_updated"`
}

// HostMetrics 主機級別的指標
type HostMetrics struct {
	Requests       int64         `json:"requests"`
	Failures       int64         `json:"failures"`
	AverageLatency time.Duration `json:"average_latency"`
	LastRequest    time.Time     `json:"last_request"`
}

// NewHTTPMetricsCollector 創建新的 HTTP 指標收集器
func NewHTTPMetricsCollector() *HTTPMetricsCollector {
	return &HTTPMetricsCollector{
		metrics: &HTTPMetrics{
			StatusCodeCounts: make(map[int]int64),
			HostMetrics:      make(map[string]*HostMetrics),
			LastUpdated:      time.Now(),
		},
	}
}

// RecordRequest 記錄 HTTP 請求指標
func (c *HTTPMetricsCollector) RecordRequest(
	method, host string,
	statusCode int,
	duration time.Duration,
	success bool,
	attempts int,
) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.metrics.TotalRequests++
	c.metrics.LastUpdated = time.Now()
	
	if success {
		c.metrics.SuccessfulRequests++
	} else {
		c.metrics.FailedRequests++
	}
	
	// 更新平均延遲（使用簡單移動平均）
	if c.metrics.TotalRequests == 1 {
		c.metrics.AverageLatency = duration
	} else {
		// 指數移動平均
		alpha := 0.1
		c.metrics.AverageLatency = time.Duration(
			alpha*float64(duration) + (1-alpha)*float64(c.metrics.AverageLatency),
		)
	}
	
	// 記錄狀態碼
	if statusCode > 0 {
		c.metrics.StatusCodeCounts[statusCode]++
	}
	
	// 記錄重試次數
	if attempts > 1 {
		c.metrics.TotalRetries += int64(attempts - 1)
	}
	
	// 更新主機指標
	if host != "" {
		hostMetric, exists := c.metrics.HostMetrics[host]
		if !exists {
			hostMetric = &HostMetrics{}
			c.metrics.HostMetrics[host] = hostMetric
		}
		
		hostMetric.Requests++
		hostMetric.LastRequest = time.Now()
		
		if !success {
			hostMetric.Failures++
		}
		
		// 更新主機平均延遲
		if hostMetric.Requests == 1 {
			hostMetric.AverageLatency = duration
		} else {
			alpha := 0.1
			hostMetric.AverageLatency = time.Duration(
				alpha*float64(duration) + (1-alpha)*float64(hostMetric.AverageLatency),
			)
		}
	}
}

// RecordRetry 記錄重試事件
func (c *HTTPMetricsCollector) RecordRetry(method, host string, attempt int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.metrics.TotalRetries++
}

// GetMetrics 獲取指標數據的副本
func (c *HTTPMetricsCollector) GetMetrics() *HTTPMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// 創建深拷貝
	metricsCopy := &HTTPMetrics{
		TotalRequests:      c.metrics.TotalRequests,
		SuccessfulRequests: c.metrics.SuccessfulRequests,
		FailedRequests:     c.metrics.FailedRequests,
		TotalRetries:       c.metrics.TotalRetries,
		AverageLatency:     c.metrics.AverageLatency,
		StatusCodeCounts:   make(map[int]int64),
		HostMetrics:        make(map[string]*HostMetrics),
		LastUpdated:        c.metrics.LastUpdated,
	}
	
	for code, count := range c.metrics.StatusCodeCounts {
		metricsCopy.StatusCodeCounts[code] = count
	}
	
	for host, metric := range c.metrics.HostMetrics {
		metricsCopy.HostMetrics[host] = &HostMetrics{
			Requests:       metric.Requests,
			Failures:       metric.Failures,
			AverageLatency: metric.AverageLatency,
			LastRequest:    metric.LastRequest,
		}
	}
	
	return metricsCopy
}

// Reset 重置指標數據
func (c *HTTPMetricsCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.metrics = &HTTPMetrics{
		StatusCodeCounts: make(map[int]int64),
		HostMetrics:      make(map[string]*HostMetrics),
		LastUpdated:      time.Now(),
	}
}

// GetSuccessRate 獲取成功率
func (m *HTTPMetrics) GetSuccessRate() float64 {
	if m.TotalRequests == 0 {
		return 0
	}
	return float64(m.SuccessfulRequests) / float64(m.TotalRequests)
}

// GetFailureRate 獲取失敗率
func (m *HTTPMetrics) GetFailureRate() float64 {
	if m.TotalRequests == 0 {
		return 0
	}
	return float64(m.FailedRequests) / float64(m.TotalRequests)
}

// GetRetryRate 獲取重試率
func (m *HTTPMetrics) GetRetryRate() float64 {
	if m.TotalRequests == 0 {
		return 0
	}
	return float64(m.TotalRetries) / float64(m.TotalRequests)
}