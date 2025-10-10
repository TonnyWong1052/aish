package cache

import (
	"container/list"
	"sync"
	"time"
)

// MemoryCache 實現內存 LRU 緩存作為 L1 緩存
type MemoryCache struct {
	mu       sync.RWMutex
	capacity int
	items    map[string]*list.Element
	order    *list.List
	stats    MemoryCacheStats
}

// MemoryCacheEntry 內存緩存項
type MemoryCacheEntry struct {
	key        string
	value      string
	createdAt  time.Time
	expiresAt  time.Time
	lastAccess time.Time
	hitCount   int64
}

// MemoryCacheStats 內存緩存統計
type MemoryCacheStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
	Size      int
	Capacity  int
}

// IsExpired 檢查是否過期
func (e *MemoryCacheEntry) IsExpired() bool {
	return time.Now().After(e.expiresAt)
}

// Touch 更新訪問時間和次數
func (e *MemoryCacheEntry) Touch() {
	e.lastAccess = time.Now()
	e.hitCount++
}

// NewMemoryCache 創建新的內存緩存
func NewMemoryCache(capacity int) *MemoryCache {
	return &MemoryCache{
		capacity: capacity,
		items:    make(map[string]*list.Element),
		order:    list.New(),
		stats: MemoryCacheStats{
			Capacity: capacity,
		},
	}
}

// Get 從內存緩存獲取值
func (mc *MemoryCache) Get(key string) (string, bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	elem, exists := mc.items[key]
	if !exists {
		mc.stats.Misses++
		return "", false
	}

	entry := elem.Value.(*MemoryCacheEntry)

	// 檢查是否過期
	if entry.IsExpired() {
		mc.removeElement(elem)
		mc.stats.Misses++
		return "", false
	}

	// 移動到列表前面（LRU）
	mc.order.MoveToFront(elem)
	entry.Touch()
	mc.stats.Hits++

	return entry.value, true
}

// Set 設置內存緩存值
func (mc *MemoryCache) Set(key, value string, ttl time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()

	// 如果鍵已存在，更新值
	if elem, exists := mc.items[key]; exists {
		entry := elem.Value.(*MemoryCacheEntry)
		entry.value = value
		entry.expiresAt = now.Add(ttl)
		entry.lastAccess = now
		mc.order.MoveToFront(elem)
		return
	}

	// 檢查是否需要驅逐
	if mc.order.Len() >= mc.capacity {
		mc.evictLRU()
	}

	// 添加新條目
	entry := &MemoryCacheEntry{
		key:        key,
		value:      value,
		createdAt:  now,
		expiresAt:  now.Add(ttl),
		lastAccess: now,
		hitCount:   0,
	}

	elem := mc.order.PushFront(entry)
	mc.items[key] = elem
	mc.stats.Size = len(mc.items)
}

// Delete 刪除緩存項
func (mc *MemoryCache) Delete(key string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if elem, exists := mc.items[key]; exists {
		mc.removeElement(elem)
	}
}

// Clear 清空所有緩存
func (mc *MemoryCache) Clear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.items = make(map[string]*list.Element)
	mc.order = list.New()
	mc.stats.Size = 0
}

// GetStats 獲取統計信息
func (mc *MemoryCache) GetStats() MemoryCacheStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	stats := mc.stats
	stats.Size = len(mc.items)
	return stats
}

// HitRate 計算命中率
func (stats *MemoryCacheStats) HitRate() float64 {
	total := stats.Hits + stats.Misses
	if total == 0 {
		return 0
	}
	return float64(stats.Hits) / float64(total)
}

// removeElement 移除列表元素
func (mc *MemoryCache) removeElement(elem *list.Element) {
	entry := elem.Value.(*MemoryCacheEntry)
	delete(mc.items, entry.key)
	mc.order.Remove(elem)
	mc.stats.Size = len(mc.items)
}

// evictLRU 驅逐最久未使用的項目
func (mc *MemoryCache) evictLRU() {
	elem := mc.order.Back()
	if elem != nil {
		mc.removeElement(elem)
		mc.stats.Evictions++
	}
}

// Cleanup 清理過期項目
func (mc *MemoryCache) Cleanup() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	var toRemove []*list.Element

	for elem := mc.order.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*MemoryCacheEntry)
		if entry.IsExpired() {
			toRemove = append(toRemove, elem)
		}
	}

	for _, elem := range toRemove {
		mc.removeElement(elem)
	}
}
