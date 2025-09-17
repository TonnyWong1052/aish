package cache

import (
	"sync"
	"text/template"
	"time"
)

// CompiledTemplate 表示編譯後的模板
type CompiledTemplate struct {
	Template     *template.Template
	CompiledAt   time.Time
	LastAccessed time.Time
	AccessCount  int64
}

// TemplateCache 模板緩存
type TemplateCache struct {
	mu        sync.RWMutex
	templates map[string]CompiledTemplate
	maxSize   int
	stats     TemplateCacheStats
}

// TemplateCacheStats 模板緩存統計
type TemplateCacheStats struct {
	Hits      int64
	Misses    int64
	Size      int
	Capacity  int
	Evictions int64
	HitRate   float64
}

// NewTemplateCache 創建新的模板緩存
func NewTemplateCache(maxSize int) *TemplateCache {
	return &TemplateCache{
		templates: make(map[string]CompiledTemplate),
		maxSize:   maxSize,
		stats: TemplateCacheStats{
			Capacity: maxSize,
		},
	}
}

// Get 獲取編譯後的模板
func (tc *TemplateCache) Get(name string) (CompiledTemplate, bool) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tmpl, exists := tc.templates[name]
	if !exists {
		tc.stats.Misses++
		return CompiledTemplate{}, false
	}

	// 更新訪問信息
	tmpl.LastAccessed = time.Now()
	tmpl.AccessCount++
	tc.templates[name] = tmpl

	tc.stats.Hits++
	tc.updateHitRate()

	return tmpl, true
}

// Set 設置編譯後的模板
func (tc *TemplateCache) Set(name string, tmpl CompiledTemplate) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// 檢查是否需要驅逐
	if len(tc.templates) >= tc.maxSize {
		tc.evictLRU()
	}

	tmpl.CompiledAt = time.Now()
	tmpl.LastAccessed = time.Now()
	tc.templates[name] = tmpl
	tc.stats.Size = len(tc.templates)
}

// Delete 刪除模板
func (tc *TemplateCache) Delete(name string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	delete(tc.templates, name)
	tc.stats.Size = len(tc.templates)
}

// Clear 清空所有模板
func (tc *TemplateCache) Clear() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.templates = make(map[string]CompiledTemplate)
	tc.stats.Size = 0
}

// GetStats 獲取統計信息
func (tc *TemplateCache) GetStats() TemplateCacheStats {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	stats := tc.stats
	stats.Size = len(tc.templates)
	return stats
}

// evictLRU 驅逐最久未使用的模板
func (tc *TemplateCache) evictLRU() {
	if len(tc.templates) == 0 {
		return
	}

	var oldestName string
	var oldestTime time.Time

	for name, tmpl := range tc.templates {
		if oldestName == "" || tmpl.LastAccessed.Before(oldestTime) {
			oldestName = name
			oldestTime = tmpl.LastAccessed
		}
	}

	if oldestName != "" {
		delete(tc.templates, oldestName)
		tc.stats.Evictions++
	}
}

// updateHitRate 更新命中率
func (tc *TemplateCache) updateHitRate() {
	total := tc.stats.Hits + tc.stats.Misses
	if total > 0 {
		tc.stats.HitRate = float64(tc.stats.Hits) / float64(total)
	}
}
