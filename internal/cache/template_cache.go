package cache

import (
	"sync"
	"text/template"
	"time"
)

// CompiledTemplate represents a compiled template
type CompiledTemplate struct {
	Template     *template.Template
	CompiledAt   time.Time
	LastAccessed time.Time
	AccessCount  int64
}

// TemplateCache template cache
type TemplateCache struct {
	mu        sync.RWMutex
	templates map[string]CompiledTemplate
	maxSize   int
	stats     TemplateCacheStats
}

// TemplateCacheStats template cache statistics
type TemplateCacheStats struct {
	Hits      int64
	Misses    int64
	Size      int
	Capacity  int
	Evictions int64
	HitRate   float64
}

// NewTemplateCache creates a new template cache
func NewTemplateCache(maxSize int) *TemplateCache {
	return &TemplateCache{
		templates: make(map[string]CompiledTemplate),
		maxSize:   maxSize,
		stats: TemplateCacheStats{
			Capacity: maxSize,
		},
	}
}

// Get retrieves compiled template
func (tc *TemplateCache) Get(name string) (CompiledTemplate, bool) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tmpl, exists := tc.templates[name]
	if !exists {
		tc.stats.Misses++
		return CompiledTemplate{}, false
	}

	// Update access information
	tmpl.LastAccessed = time.Now()
	tmpl.AccessCount++
	tc.templates[name] = tmpl

	tc.stats.Hits++
	tc.updateHitRate()

	return tmpl, true
}

// Set sets compiled template
func (tc *TemplateCache) Set(name string, tmpl CompiledTemplate) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Check if eviction is needed
	if len(tc.templates) >= tc.maxSize {
		tc.evictLRU()
	}

	tmpl.CompiledAt = time.Now()
	tmpl.LastAccessed = time.Now()
	tc.templates[name] = tmpl
	tc.stats.Size = len(tc.templates)
}

// Delete removes template
func (tc *TemplateCache) Delete(name string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	delete(tc.templates, name)
	tc.stats.Size = len(tc.templates)
}

// Clear removes all templates
func (tc *TemplateCache) Clear() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.templates = make(map[string]CompiledTemplate)
	tc.stats.Size = 0
}

// GetStats retrieves statistics
func (tc *TemplateCache) GetStats() TemplateCacheStats {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	stats := tc.stats
	stats.Size = len(tc.templates)
	return stats
}

// evictLRU evicts least recently used template
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

// updateHitRate updates hit rate
func (tc *TemplateCache) updateHitRate() {
	total := tc.stats.Hits + tc.stats.Misses
	if total > 0 {
		tc.stats.HitRate = float64(tc.stats.Hits) / float64(total)
	}
}
