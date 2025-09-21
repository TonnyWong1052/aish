package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEnhancedHTTPClient_Do_Success(t *testing.T) {
	// 創建測試服務器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()
	
	// 創建增強 HTTP 客戶端
	config := DefaultEnhancedHTTPConfig()
	config.RetryConfig.MaxAttempts = 1 // 僅一次嘗試
	client := NewEnhancedHTTPClient(config)
	defer client.Close()
	
	// 創建請求
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	
	// 執行請求
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	
	// 驗證響應
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestEnhancedHTTPClient_Do_Retry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
		}
	}))
	defer server.Close()
	
	config := DefaultEnhancedHTTPConfig()
	config.RetryConfig.MaxAttempts = 3
	config.RetryConfig.InitialDelay = time.Millisecond
	client := NewEnhancedHTTPClient(config)
	defer client.Close()
	
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	
	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestEnhancedHTTPClient_CircuitBreaker(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	
	config := DefaultEnhancedHTTPConfig()
	config.RetryConfig.MaxAttempts = 1
	config.CircuitBreakerConfig.FailureThreshold = 2
	config.CircuitBreakerConfig.MinRequests = 1
	client := NewEnhancedHTTPClient(config)
	defer client.Close()
	
	// 觸發斷路器
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest("GET", server.URL, nil)
		client.Do(req)
	}
	
	// 檢查斷路器統計
	stats := client.GetCircuitBreakerStats()
	if stats.Failures < 2 {
		t.Errorf("Expected at least 2 failures, got %d", stats.Failures)
	}
}

func TestEnhancedHTTPClient_HealthCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	config := DefaultEnhancedHTTPConfig()
	client := NewEnhancedHTTPClient(config)
	defer client.Close()
	
	ctx := context.Background()
	err := client.HealthCheck(ctx, server.URL, nil)
	if err != nil {
		t.Errorf("Health check failed: %v", err)
	}
}

func TestEnhancedHTTPClient_HealthCheck_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	
	config := DefaultEnhancedHTTPConfig()
	client := NewEnhancedHTTPClient(config)
	defer client.Close()
	
	ctx := context.Background()
	err := client.HealthCheck(ctx, server.URL, nil)
	if err == nil {
		t.Error("Expected health check to fail")
	}
}

func TestEnhancedHTTPClient_DoWithTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	config := DefaultEnhancedHTTPConfig()
	client := NewEnhancedHTTPClient(config)
	defer client.Close()
	
	req, _ := http.NewRequest("GET", server.URL, nil)
	
	// 短超時應該失敗
	_, err := client.DoWithTimeout(req, 10*time.Millisecond)
	if err == nil {
		t.Error("Expected timeout error")
	}
	
	// 長超時應該成功
	resp, err := client.DoWithTimeout(req, 200*time.Millisecond)
	if err != nil {
		t.Errorf("Request with long timeout failed: %v", err)
	}
	if resp != nil {
		resp.Body.Close()
	}
}

func TestHTTPMetricsCollector(t *testing.T) {
	collector := NewHTTPMetricsCollector()
	
	// 記錄一些指標
	collector.RecordRequest("GET", "example.com", 200, 100*time.Millisecond, true, 1)
	collector.RecordRequest("POST", "example.com", 500, 200*time.Millisecond, false, 2)
	collector.RecordRetry("GET", "example.com", 1)
	
	metrics := collector.GetMetrics()
	
	if metrics.TotalRequests != 2 {
		t.Errorf("Expected 2 total requests, got %d", metrics.TotalRequests)
	}
	
	if metrics.SuccessfulRequests != 1 {
		t.Errorf("Expected 1 successful request, got %d", metrics.SuccessfulRequests)
	}
	
	if metrics.FailedRequests != 1 {
		t.Errorf("Expected 1 failed request, got %d", metrics.FailedRequests)
	}
	
	if metrics.TotalRetries != 2 { // 1 從記錄中 + 1 從 RecordRetry
		t.Errorf("Expected 2 total retries, got %d", metrics.TotalRetries)
	}
	
	expectedHitRate := 0.5
	if metrics.GetSuccessRate() != expectedHitRate {
		t.Errorf("Expected success rate %.2f, got %.2f", expectedHitRate, metrics.GetSuccessRate())
	}
}

func TestDefaultRetryableStatusCheck(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   bool
	}{
		{200, false},
		{400, false},
		{404, false},
		{429, true},
		{500, true},
		{502, true},
		{503, true},
	}
	
	for _, tt := range tests {
		result := DefaultRetryableStatusCheck(tt.statusCode)
		if result != tt.expected {
			t.Errorf("For status code %d, expected %v, got %v", 
				tt.statusCode, tt.expected, result)
		}
	}
}

func TestEnhancedHTTPConfig_Defaults(t *testing.T) {
	config := DefaultEnhancedHTTPConfig()
	
	if config.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", config.Timeout)
	}
	
	if config.MaxIdleConns != 100 {
		t.Errorf("Expected default MaxIdleConns 100, got %d", config.MaxIdleConns)
	}
	
	if config.RetryConfig == nil {
		t.Error("Expected RetryConfig to be set")
	}
	
	if config.CircuitBreakerConfig == nil {
		t.Error("Expected CircuitBreakerConfig to be set")
	}
	
	if !config.EnableMetrics {
		t.Error("Expected metrics to be enabled by default")
	}
}

// 基準測試
func BenchmarkEnhancedHTTPClient_Do(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	config := DefaultEnhancedHTTPConfig()
	config.EnableMetrics = false // 關閉指標以提高性能
	client := NewEnhancedHTTPClient(config)
	defer client.Close()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, _ := http.NewRequest("GET", server.URL, nil)
			resp, err := client.Do(req)
			if err != nil {
				b.Fatalf("Request failed: %v", err)
			}
			resp.Body.Close()
		}
	})
}

func BenchmarkHTTPMetricsCollector_RecordRequest(b *testing.B) {
	collector := NewHTTPMetricsCollector()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.RecordRequest("GET", "example.com", 200, 100*time.Millisecond, true, 1)
	}
}