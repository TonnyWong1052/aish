package cache

import (
	"context"
	"sync"
	"time"
)

// CachePrewarmer 快取預熱器
type CachePrewarmer struct {
	cache    *IntelligentCache
	config   *IntelligentCacheConfig
	patterns []PrewarmPattern
	ticker   *time.Ticker
	ctx      context.Context
	cancel   context.CancelFunc
	running  bool
	mu       sync.RWMutex
}

// PrewarmPattern 預熱模式
type PrewarmPattern struct {
	KeyPattern    string        `json:"key_pattern"`
	ValueLoader   ValueLoader   `json:"-"`
	Frequency     time.Duration `json:"frequency"`
	Priority      int           `json:"priority"`
	LastExecution time.Time     `json:"last_execution"`
}

// ValueLoader 值加載器函數類型
type ValueLoader func(ctx context.Context, key string) (interface{}, error)

// PrewarmerStats 預熱器統計
type PrewarmerStats struct {
	TotalPrewarmed  int64         `json:"total_prewarmed"`
	SuccessfulLoads int64         `json:"successful_loads"`
	FailedLoads     int64         `json:"failed_loads"`
	LastRun         time.Time     `json:"last_run"`
	AverageLoadTime time.Duration `json:"average_load_time"`
	IsRunning       bool          `json:"is_running"`
}

// NewCachePrewarmer 創建新的快取預熱器
func NewCachePrewarmer(cache *IntelligentCache, config *IntelligentCacheConfig) *CachePrewarmer {
	return &CachePrewarmer{
		cache:    cache,
		config:   config,
		patterns: make([]PrewarmPattern, 0),
	}
}

// AddPattern 添加預熱模式
func (cp *CachePrewarmer) AddPattern(pattern PrewarmPattern) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	cp.patterns = append(cp.patterns, pattern)
}

// RemovePattern 移除預熱模式
func (cp *CachePrewarmer) RemovePattern(keyPattern string) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	for i, pattern := range cp.patterns {
		if pattern.KeyPattern == keyPattern {
			cp.patterns = append(cp.patterns[:i], cp.patterns[i+1:]...)
			break
		}
	}
}

// Start 啟動預熱器
func (cp *CachePrewarmer) Start(ctx context.Context) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.running {
		return nil
	}

	cp.ctx, cp.cancel = context.WithCancel(ctx)
	cp.ticker = time.NewTicker(cp.config.PrewarmingInterval)
	cp.running = true

	go cp.run()

	return nil
}

// Stop 停止預熱器
func (cp *CachePrewarmer) Stop() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if !cp.running {
		return
	}

	cp.running = false
	if cp.cancel != nil {
		cp.cancel()
	}
	if cp.ticker != nil {
		cp.ticker.Stop()
	}
}

// run 運行預熱器主循環
func (cp *CachePrewarmer) run() {
	defer func() {
		cp.mu.Lock()
		cp.running = false
		cp.mu.Unlock()
	}()

	for {
		select {
		case <-cp.ctx.Done():
			return
		case <-cp.ticker.C:
			cp.executePrewarming()
		}
	}
}

// executePrewarming 執行預熱
func (cp *CachePrewarmer) executePrewarming() {
	cp.mu.RLock()
	patterns := make([]PrewarmPattern, len(cp.patterns))
	copy(patterns, cp.patterns)
	cp.mu.RUnlock()

	// 按優先級排序
	for i := 0; i < len(patterns)-1; i++ {
		for j := i + 1; j < len(patterns); j++ {
			if patterns[i].Priority < patterns[j].Priority {
				patterns[i], patterns[j] = patterns[j], patterns[i]
			}
		}
	}

	// 執行預熱
	processedCount := 0
	for _, pattern := range patterns {
		if processedCount >= cp.config.PrewarmingBatchSize {
			break
		}

		if cp.shouldExecutePattern(pattern) {
			cp.executePattern(pattern)
			processedCount++
		}
	}
}

// shouldExecutePattern 判斷是否應該執行模式
func (cp *CachePrewarmer) shouldExecutePattern(pattern PrewarmPattern) bool {
	now := time.Now()
	return now.Sub(pattern.LastExecution) >= pattern.Frequency
}

// executePattern 執行預熱模式
func (cp *CachePrewarmer) executePattern(pattern PrewarmPattern) {
	if pattern.ValueLoader == nil {
		return
	}

	// 簡化實現：根據模式生成一些鍵
	keys := cp.generateKeysFromPattern(pattern.KeyPattern)

	for _, key := range keys {
		select {
		case <-cp.ctx.Done():
			return
		default:
			cp.loadAndCache(key, pattern.ValueLoader)
		}
	}

	// 更新執行時間
	cp.mu.Lock()
	for i := range cp.patterns {
		if cp.patterns[i].KeyPattern == pattern.KeyPattern {
			cp.patterns[i].LastExecution = time.Now()
			break
		}
	}
	cp.mu.Unlock()
}

// generateKeysFromPattern 根據模式生成鍵
func (cp *CachePrewarmer) generateKeysFromPattern(pattern string) []string {
	// 簡化實現，實際應該根據模式智能生成
	keys := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		key := pattern + "_" + string(rune('0'+i))
		keys = append(keys, key)
	}
	return keys
}

// loadAndCache 加載並快取值
func (cp *CachePrewarmer) loadAndCache(key string, loader ValueLoader) {
	startTime := time.Now()

	// 檢查是否已經存在
	if _, exists := cp.cache.Get(key); exists {
		return
	}

	// 加載值
	value, err := loader(cp.ctx, key)
	if err != nil {
		return
	}

	// 快取值
	_ = cp.cache.Set(key, value, cp.config.DefaultTTL)

	// 可以在這裡記錄統計信息
	_ = time.Since(startTime)
}

// GetStats 獲取預熱器統計
func (cp *CachePrewarmer) GetStats() *PrewarmerStats {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	return &PrewarmerStats{
		IsRunning: cp.running,
		LastRun:   time.Now(), // 簡化實現
	}
}

// GetPatterns 獲取所有預熱模式
func (cp *CachePrewarmer) GetPatterns() []PrewarmPattern {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	patterns := make([]PrewarmPattern, len(cp.patterns))
	copy(patterns, cp.patterns)
	return patterns
}

// IsRunning 檢查預熱器是否正在運行
func (cp *CachePrewarmer) IsRunning() bool {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	return cp.running
}
