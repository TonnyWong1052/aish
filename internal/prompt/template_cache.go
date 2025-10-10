package prompt

import (
	"sync"
	"text/template"
	"time"
)

// CachedTemplate 緩存的模板
type CachedTemplate struct {
	Template     *template.Template
	Source       string
	CompiledAt   time.Time
	LastAccessed time.Time
	AccessCount  int64
	Size         int64 // 模板源碼大小
}

// TemplateCache 模板緩存管理器
type TemplateCache struct {
	mu        sync.RWMutex
	templates map[string]*CachedTemplate
	maxSize   int
	stats     TemplateCacheStats
}

// TemplateCacheStats 模板緩存統計
type TemplateCacheStats struct {
	Hits         int64
	Misses       int64
	Compilations int64
	Evictions    int64
	TotalSize    int64
	CacheSize    int
	MaxSize      int
	HitRate      float64
}

// NewTemplateCache 創建新的模板緩存
func NewTemplateCache(maxSize int) *TemplateCache {
	return &TemplateCache{
		templates: make(map[string]*CachedTemplate),
		maxSize:   maxSize,
		stats: TemplateCacheStats{
			MaxSize: maxSize,
		},
	}
}

// GetOrCompile 獲取緩存的模板或編譯新模板
func (tc *TemplateCache) GetOrCompile(name, source string) (*template.Template, error) {
	// 首先嘗試從緩存獲取
	if tmpl := tc.get(name, source); tmpl != nil {
		return tmpl, nil
	}

	// 緩存未命中，編譯新模板
	return tc.compileAndCache(name, source)
}

// Get 從緩存獲取模板（僅當源碼匹配時）
func (tc *TemplateCache) get(name, source string) *template.Template {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	cached, exists := tc.templates[name]
	if !exists {
		tc.stats.Misses++
		return nil
	}

	// 檢查源碼是否匹配
	if cached.Source != source {
		tc.stats.Misses++
		// 源碼不匹配，刪除舊緩存
		delete(tc.templates, name)
		tc.updateCacheSize()
		return nil
	}

	// 更新訪問信息
	cached.LastAccessed = time.Now()
	cached.AccessCount++
	tc.stats.Hits++
	tc.updateHitRate()

	return cached.Template
}

// compileAndCache 編譯模板並緩存
func (tc *TemplateCache) compileAndCache(name, source string) (*template.Template, error) {
	// 編譯模板
	tmpl, err := template.New(name).Parse(source)
	if err != nil {
		return nil, err
	}

	tc.mu.Lock()
	defer tc.mu.Unlock()

	// 檢查是否需要驅逐
	if len(tc.templates) >= tc.maxSize {
		tc.evictLRU()
	}

	// 緩存新模板
	now := time.Now()
	cached := &CachedTemplate{
		Template:     tmpl,
		Source:       source,
		CompiledAt:   now,
		LastAccessed: now,
		AccessCount:  1,
		Size:         int64(len(source)),
	}

	tc.templates[name] = cached
	tc.stats.Compilations++
	tc.updateCacheSize()

	return tmpl, nil
}

// Clear 清空所有緩存
func (tc *TemplateCache) Clear() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.templates = make(map[string]*CachedTemplate)
	tc.updateCacheSize()
}

// Remove 移除特定模板
func (tc *TemplateCache) Remove(name string) bool {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if _, exists := tc.templates[name]; exists {
		delete(tc.templates, name)
		tc.updateCacheSize()
		return true
	}
	return false
}

// GetStats 獲取緩存統計
func (tc *TemplateCache) GetStats() TemplateCacheStats {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	stats := tc.stats
	stats.CacheSize = len(tc.templates)

	// 計算總大小
	var totalSize int64
	for _, cached := range tc.templates {
		totalSize += cached.Size
	}
	stats.TotalSize = totalSize

	return stats
}

// Warmup 預熱緩存，預編譯常用模板
func (tc *TemplateCache) Warmup(templates map[string]string) error {
	for name, source := range templates {
		if _, err := tc.GetOrCompile(name, source); err != nil {
			return err
		}
	}
	return nil
}

// 內部方法

func (tc *TemplateCache) evictLRU() {
	if len(tc.templates) == 0 {
		return
	}

	var oldestName string
	var oldestTime time.Time

	for name, cached := range tc.templates {
		if oldestName == "" || cached.LastAccessed.Before(oldestTime) {
			oldestName = name
			oldestTime = cached.LastAccessed
		}
	}

	if oldestName != "" {
		delete(tc.templates, oldestName)
		tc.stats.Evictions++
	}
}

func (tc *TemplateCache) updateCacheSize() {
	tc.stats.CacheSize = len(tc.templates)
}

func (tc *TemplateCache) updateHitRate() {
	total := tc.stats.Hits + tc.stats.Misses
	if total > 0 {
		tc.stats.HitRate = float64(tc.stats.Hits) / float64(total)
	}
}

// TemplateManager 模板管理器（增強版本）
type EnhancedTemplateManager struct {
	cache   *TemplateCache
	funcMap template.FuncMap
	mu      sync.RWMutex
}

// NewEnhancedTemplateManager 創建增強的模板管理器
func NewEnhancedTemplateManager(cacheSize int) *EnhancedTemplateManager {
	return &EnhancedTemplateManager{
		cache: NewTemplateCache(cacheSize),
		funcMap: template.FuncMap{
			"add": func(a, b int) int { return a + b },
			"sub": func(a, b int) int { return a - b },
			"mul": func(a, b int) int { return a * b },
			"div": func(a, b int) int {
				if b != 0 {
					return a / b
				}
				return 0
			},
			"mod": func(a, b int) int {
				if b != 0 {
					return a % b
				}
				return 0
			},
			"min": func(a, b int) int {
				if a < b {
					return a
				}
				return b
			},
			"max": func(a, b int) int {
				if a > b {
					return a
				}
				return b
			},
		},
	}
}

// GetTemplate 獲取模板（帶函數映射）
func (etm *EnhancedTemplateManager) GetTemplate(name, source string) (*template.Template, error) {
	// 為源碼添加函數映射的哈希，確保緩存一致性
	cacheKey := name + "_enhanced"

	tmpl, err := etm.cache.GetOrCompile(cacheKey, source)
	if err != nil {
		return nil, err
	}

	// 如果是新編譯的模板，添加函數映射
	if tmpl.Lookup(name) == nil {
		tmpl = tmpl.Funcs(etm.funcMap)

		// 重新解析以應用函數映射
		tmpl, err = tmpl.Parse(source)
		if err != nil {
			return nil, err
		}
	}

	return tmpl, nil
}

// AddFunc 添加自定義函數
func (etm *EnhancedTemplateManager) AddFunc(name string, fn interface{}) {
	etm.mu.Lock()
	defer etm.mu.Unlock()

	etm.funcMap[name] = fn

	// 清空緩存以確保新函數生效
	etm.cache.Clear()
}

// GetStats 獲取統計信息
func (etm *EnhancedTemplateManager) GetStats() TemplateCacheStats {
	return etm.cache.GetStats()
}
