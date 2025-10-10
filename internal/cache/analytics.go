package cache

import (
	"sync"
	"time"
)

// CacheAnalytics 快取分析器
type CacheAnalytics struct {
	window time.Duration
	stats  *CacheAnalyticsStats
	events []CacheEvent
	mu     sync.RWMutex
}

// CacheAnalyticsStats 快取分析統計
type CacheAnalyticsStats struct {
	TotalHits    int64     `json:"total_hits"`
	TotalMisses  int64     `json:"total_misses"`
	TotalSets    int64     `json:"total_sets"`
	TotalDeletes int64     `json:"total_deletes"`
	HitRate      float64   `json:"hit_rate"`
	MissRate     float64   `json:"miss_rate"`
	AverageSize  int64     `json:"average_size"`
	TotalSize    int64     `json:"total_size"`
	WindowStart  time.Time `json:"window_start"`
	WindowEnd    time.Time `json:"window_end"`
	TopHitKeys   []string  `json:"top_hit_keys"`
	TopMissKeys  []string  `json:"top_miss_keys"`
}

// CacheEvent 快取事件
type CacheEvent struct {
	Type      EventType `json:"type"`
	Key       string    `json:"key"`
	Size      int64     `json:"size"`
	Timestamp time.Time `json:"timestamp"`
}

// EventType 事件類型
type EventType string

const (
	EventHit    EventType = "HIT"
	EventMiss   EventType = "MISS"
	EventSet    EventType = "SET"
	EventDelete EventType = "DELETE"
)

// NewCacheAnalytics 創建新的快取分析器
func NewCacheAnalytics(window time.Duration) *CacheAnalytics {
	return &CacheAnalytics{
		window: window,
		stats: &CacheAnalyticsStats{
			WindowStart: time.Now(),
			TopHitKeys:  make([]string, 0),
			TopMissKeys: make([]string, 0),
		},
		events: make([]CacheEvent, 0),
	}
}

// RecordHit 記錄快取命中
func (ca *CacheAnalytics) RecordHit(key string) {
	ca.recordEvent(CacheEvent{
		Type:      EventHit,
		Key:       key,
		Timestamp: time.Now(),
	})
}

// RecordMiss 記錄快取未命中
func (ca *CacheAnalytics) RecordMiss(key string) {
	ca.recordEvent(CacheEvent{
		Type:      EventMiss,
		Key:       key,
		Timestamp: time.Now(),
	})
}

// RecordSet 記錄快取設置
func (ca *CacheAnalytics) RecordSet(key string, size int64) {
	ca.recordEvent(CacheEvent{
		Type:      EventSet,
		Key:       key,
		Size:      size,
		Timestamp: time.Now(),
	})
}

// RecordDelete 記錄快取刪除
func (ca *CacheAnalytics) RecordDelete(key string) {
	ca.recordEvent(CacheEvent{
		Type:      EventDelete,
		Key:       key,
		Timestamp: time.Now(),
	})
}

// recordEvent 記錄事件
func (ca *CacheAnalytics) recordEvent(event CacheEvent) {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	ca.events = append(ca.events, event)
	ca.cleanupOldEvents()
	ca.updateStats()
}

// cleanupOldEvents 清理過期事件
func (ca *CacheAnalytics) cleanupOldEvents() {
	now := time.Now()
	cutoff := now.Add(-ca.window)

	// 找到第一個未過期的事件
	firstValid := 0
	for i, event := range ca.events {
		if event.Timestamp.After(cutoff) {
			firstValid = i
			break
		}
	}

	// 移除過期事件
	if firstValid > 0 {
		ca.events = ca.events[firstValid:]
	}
}

// updateStats 更新統計信息
func (ca *CacheAnalytics) updateStats() {
	now := time.Now()

	// 重置統計
	ca.stats = &CacheAnalyticsStats{
		WindowStart: now.Add(-ca.window),
		WindowEnd:   now,
		TopHitKeys:  make([]string, 0),
		TopMissKeys: make([]string, 0),
	}

	hitKeys := make(map[string]int)
	missKeys := make(map[string]int)
	var totalSize int64
	var sizeCount int

	// 統計事件
	for _, event := range ca.events {
		switch event.Type {
		case EventHit:
			ca.stats.TotalHits++
			hitKeys[event.Key]++
		case EventMiss:
			ca.stats.TotalMisses++
			missKeys[event.Key]++
		case EventSet:
			ca.stats.TotalSets++
			if event.Size > 0 {
				totalSize += event.Size
				sizeCount++
			}
		case EventDelete:
			ca.stats.TotalDeletes++
		}
	}

	// 計算比率
	totalRequests := ca.stats.TotalHits + ca.stats.TotalMisses
	if totalRequests > 0 {
		ca.stats.HitRate = float64(ca.stats.TotalHits) / float64(totalRequests)
		ca.stats.MissRate = float64(ca.stats.TotalMisses) / float64(totalRequests)
	}

	// 計算平均大小
	if sizeCount > 0 {
		ca.stats.AverageSize = totalSize / int64(sizeCount)
	}
	ca.stats.TotalSize = totalSize

	// 找出熱點鍵
	ca.stats.TopHitKeys = ca.getTopKeys(hitKeys, 10)
	ca.stats.TopMissKeys = ca.getTopKeys(missKeys, 10)
}

// getTopKeys 獲取訪問次數最多的鍵
func (ca *CacheAnalytics) getTopKeys(keyCount map[string]int, limit int) []string {
	type keyFreq struct {
		key   string
		count int
	}

	var keyFreqs []keyFreq
	for key, count := range keyCount {
		keyFreqs = append(keyFreqs, keyFreq{key, count})
	}

	// 按頻率排序
	for i := 0; i < len(keyFreqs)-1; i++ {
		for j := i + 1; j < len(keyFreqs); j++ {
			if keyFreqs[i].count < keyFreqs[j].count {
				keyFreqs[i], keyFreqs[j] = keyFreqs[j], keyFreqs[i]
			}
		}
	}

	// 返回前 limit 個
	result := make([]string, 0, limit)
	for i, kf := range keyFreqs {
		if i >= limit {
			break
		}
		result = append(result, kf.key)
	}

	return result
}

// GetStats 獲取統計信息
func (ca *CacheAnalytics) GetStats() *CacheAnalyticsStats {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	// 返回統計信息的副本
	statsCopy := *ca.stats
	statsCopy.TopHitKeys = make([]string, len(ca.stats.TopHitKeys))
	copy(statsCopy.TopHitKeys, ca.stats.TopHitKeys)
	statsCopy.TopMissKeys = make([]string, len(ca.stats.TopMissKeys))
	copy(statsCopy.TopMissKeys, ca.stats.TopMissKeys)

	return &statsCopy
}

// Reset 重置分析數據
func (ca *CacheAnalytics) Reset() {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	ca.events = make([]CacheEvent, 0)
	ca.stats = &CacheAnalyticsStats{
		WindowStart: time.Now(),
		TopHitKeys:  make([]string, 0),
		TopMissKeys: make([]string, 0),
	}
}
