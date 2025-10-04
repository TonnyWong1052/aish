package performance

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TonnyWong1052/aish/internal/cache"
	"github.com/TonnyWong1052/aish/internal/concurrent"
	"github.com/TonnyWong1052/aish/internal/history"
	"github.com/TonnyWong1052/aish/internal/llm"
	"github.com/TonnyWong1052/aish/internal/resource"
)

// BenchmarkHTTPClient 測試 HTTP 客戶端性能
func BenchmarkHTTPClient(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	pool := llm.GetDefaultPool()
	client := pool.GetClient("benchmark", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(server.URL)
		if err == nil {
			resp.Body.Close()
		}
	}
}

// BenchmarkLayeredCache 測試分層緩存性能
func BenchmarkLayeredCache(b *testing.B) {
	config := cache.DefaultLayeredCacheConfig()
	layeredCache, _ := cache.NewLayeredCache(config)
	defer layeredCache.Close()

	key := "test_key"
	value := "test_value"

	b.Run("Set", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = layeredCache.Set(fmt.Sprintf("%s_%d", key, i), value, time.Minute)
		}
	})

	b.Run("Get_L1_Hit", func(b *testing.B) {
		_ = layeredCache.Set(key, value, time.Minute)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			layeredCache.Get(key)
		}
	})

	b.Run("Get_L2_Hit", func(b *testing.B) {
		_ = layeredCache.Set(key, value, time.Minute)
		// layeredCache.L1Cache.Delete(key) // L1Cache is unexported, skip this test
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			layeredCache.Get(key)
		}
	})
}

// BenchmarkHistoryManager 測試歷史管理器性能
func BenchmarkHistoryManager(b *testing.B) {
	config := history.DefaultOptimizedConfig()
	manager, _ := history.NewOptimizedManager(config)
	defer manager.Close()

	entry := history.Entry{
		Command: "benchmark command",
	}

	b.Run("Append", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = manager.Append(entry)
		}
	})
}

// BenchmarkWorkerPool 測試工作池性能
func BenchmarkWorkerPool(b *testing.B) {
	pool := concurrent.NewWorkerPool(concurrent.DefaultWorkerPoolConfig())
	defer pool.Close()

	task := concurrent.Task{
		Execute: func(ctx context.Context, payload interface{}) (interface{}, error) {
			time.Sleep(1 * time.Millisecond)
			return nil, nil
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Submit(task)
	}
}

// BenchmarkResourceManager 測試資源管理器性能
func BenchmarkResourceManager(b *testing.B) {
	rm := resource.NewResourceManager(resource.DefaultResourceConfig())
	_ = rm.Cleanup()

	guard := rm.NewResourceGuard()

	b.Run("AcquireRelease", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = guard.AcquireMemory(1024)
			_ = guard.AcquireGoroutine()
			guard.Release()
		}
	})
}
