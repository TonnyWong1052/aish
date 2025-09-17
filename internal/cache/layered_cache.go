package cache

import (
	"sync"
	"time"
)

// LayeredCache 實現兩層緩存：L1 內存緩存 + L2 磁盤緩存
type LayeredCache struct {
	l1Cache *MemoryCache // L1: 內存緩存
	l2Cache *Cache       // L2: 磁盤緩存
	config  LayeredCacheConfig
	stats   LayeredCacheStats
	mu      sync.RWMutex
}

// LayeredCacheConfig 分層緩存配置
type LayeredCacheConfig struct {
	L1Capacity   int           // L1 緩存容量
	L1DefaultTTL time.Duration // L1 默認 TTL
	L2Config     CacheConfig   // L2 緩存配置
	WriteThrough bool          // 是否寫穿透（同時寫入 L1 和 L2）
	WriteBack    bool          // 是否寫回（延遲寫入 L2）
}

// LayeredCacheStats 分層緩存統計
type LayeredCacheStats struct {
	L1Stats     MemoryCacheStats
	L2Stats     CacheStats
	L1Hits      int64
	L2Hits      int64
	TotalMisses int64
	Promotions  int64 // L2 到 L1 的提升次數
	Demotions   int64 // L1 到 L2 的降級次數
}

// DefaultLayeredCacheConfig 默認分層緩存配置
func DefaultLayeredCacheConfig() LayeredCacheConfig {
	return LayeredCacheConfig{
		L1Capacity:   500, // 內存緩存 500 項
		L1DefaultTTL: time.Hour,
		L2Config:     DefaultCacheConfig(),
		WriteThrough: true,
		WriteBack:    false,
	}
}

// NewLayeredCache 創建新的分層緩存
func NewLayeredCache(config LayeredCacheConfig) (*LayeredCache, error) {
	l1 := NewMemoryCache(config.L1Capacity)

	l2, err := NewCache(config.L2Config)
	if err != nil {
		return nil, err
	}

	return &LayeredCache{
		l1Cache: l1,
		l2Cache: l2,
		config:  config,
	}, nil
}

// Get 從分層緩存獲取值
func (lc *LayeredCache) Get(key string) (string, bool) {
	// 首先嘗試 L1 緩存
	if value, found := lc.l1Cache.Get(key); found {
		lc.recordL1Hit()
		return value, true
	}

	// L1 未命中，嘗試 L2 緩存
	if value, found := lc.l2Cache.Get(key); found {
		lc.recordL2Hit()

		// 提升到 L1 緩存（熱數據上移）
		lc.l1Cache.Set(key, value, lc.config.L1DefaultTTL)
		lc.recordPromotion()

		return value, true
	}

	// 都未命中
	lc.recordMiss()
	return "", false
}

// Set 設置分層緩存值
func (lc *LayeredCache) Set(key, value string, ttl time.Duration) error {
	// 始終寫入 L1 緩存
	l1TTL := ttl
	if l1TTL > lc.config.L1DefaultTTL {
		l1TTL = lc.config.L1DefaultTTL
	}
	lc.l1Cache.Set(key, value, l1TTL)

	// 根據策略寫入 L2 緩存
	if lc.config.WriteThrough {
		// 寫穿透：同時寫入 L2
		return lc.l2Cache.Set(key, value, ttl)
	} else if lc.config.WriteBack {
		// 寫回：延遲寫入 L2（這裡簡化為立即寫入）
		// 在實際實現中，可以使用後台 goroutine 批量寫入
		return lc.l2Cache.Set(key, value, ttl)
	}

	return nil
}

// Delete 刪除緩存項
func (lc *LayeredCache) Delete(key string) {
	lc.l1Cache.Delete(key)
	lc.l2Cache.Delete(key)
}

// Clear 清空所有緩存
func (lc *LayeredCache) Clear() error {
	lc.l1Cache.Clear()
	return lc.l2Cache.Clear()
}

// GetStats 獲取統計信息
func (lc *LayeredCache) GetStats() LayeredCacheStats {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	stats := lc.stats
	stats.L1Stats = lc.l1Cache.GetStats()
	stats.L2Stats = lc.l2Cache.GetStats()
	return stats
}

// HitRate 計算總命中率
func (stats *LayeredCacheStats) HitRate() float64 {
	totalHits := stats.L1Hits + stats.L2Hits
	total := totalHits + stats.TotalMisses
	if total == 0 {
		return 0
	}
	return float64(totalHits) / float64(total)
}

// L1HitRate 計算 L1 命中率
func (stats *LayeredCacheStats) L1HitRate() float64 {
	total := stats.L1Hits + stats.L2Hits + stats.TotalMisses
	if total == 0 {
		return 0
	}
	return float64(stats.L1Hits) / float64(total)
}

// Close 關閉分層緩存
func (lc *LayeredCache) Close() error {
	return lc.l2Cache.Close()
}

// 統計方法
func (lc *LayeredCache) recordL1Hit() {
	lc.mu.Lock()
	lc.stats.L1Hits++
	lc.mu.Unlock()
}

func (lc *LayeredCache) recordL2Hit() {
	lc.mu.Lock()
	lc.stats.L2Hits++
	lc.mu.Unlock()
}

func (lc *LayeredCache) recordMiss() {
	lc.mu.Lock()
	lc.stats.TotalMisses++
	lc.mu.Unlock()
}

func (lc *LayeredCache) recordPromotion() {
	lc.mu.Lock()
	lc.stats.Promotions++
	lc.mu.Unlock()
}

func (lc *LayeredCache) recordDemotion() {
	lc.mu.Lock()
	lc.stats.Demotions++
	lc.mu.Unlock()
}

// WarmUp 預熱緩存，將 L2 中的熱點數據載入 L1
func (lc *LayeredCache) WarmUp(keys []string) {
	for _, key := range keys {
		if value, found := lc.l2Cache.Get(key); found {
			lc.l1Cache.Set(key, value, lc.config.L1DefaultTTL)
		}
	}
}

// Compact 壓縮 L2 緩存，清理過期項目
func (lc *LayeredCache) Compact() {
	lc.l2Cache.Cleanup()
	lc.l1Cache.Cleanup()
}
