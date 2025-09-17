package cache

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"powerful-cli/internal/llm"
)

// LLMCacheManager 統一管理所有 LLM 相關緩存
type LLMCacheManager struct {
	layeredCache  *LayeredCache  // 分層緩存用於一般響應
	llmCache      *LLMCache      // 專用 LLM 緩存用於相似度匹配
	templateCache *TemplateCache // 模板緩存
	enabled       bool
	mu            sync.RWMutex
	stats         CacheManagerStats
}

// CacheManagerStats 緩存管理器統計
type CacheManagerStats struct {
	LayeredStats  LayeredCacheStats
	LLMStats      LLMCacheStats
	TemplateStats TemplateCacheStats
	TotalRequests int64
	CacheHitRate  float64
	AvgLatency    time.Duration
}

// NewLLMCacheManager 創建新的 LLM 緩存管理器
func NewLLMCacheManager(enabled bool) (*LLMCacheManager, error) {
	if !enabled {
		return &LLMCacheManager{
			enabled: false,
		}, nil
	}

	// 創建分層緩存
	layeredConfig := DefaultLayeredCacheConfig()
	layeredConfig.L1Capacity = 300 // 內存緩存 300 項
	layeredConfig.L1DefaultTTL = 45 * time.Minute

	layeredCache, err := NewLayeredCache(layeredConfig)
	if err != nil {
		return nil, err
	}

	// 創建專用 LLM 緩存
	llmCache := NewLLMCache(layeredCache.l2Cache, DefaultLLMCacheConfig())

	// 創建模板緩存
	templateCache := NewTemplateCache(200) // 200 個模板緩存

	return &LLMCacheManager{
		layeredCache:  layeredCache,
		llmCache:      llmCache,
		templateCache: templateCache,
		enabled:       true,
	}, nil
}

// GetSuggestion 獲取建議，使用分層緩存策略
func (cm *LLMCacheManager) GetSuggestion(ctx context.Context, key LLMCacheKey) (*llm.Suggestion, bool) {
	if !cm.enabled {
		return nil, false
	}

	start := time.Now()
	defer func() {
		cm.updateStats(time.Since(start))
	}()

	cm.incrementRequests()

	// 首先嘗試分層緩存（更快）
	hashKey := key.Hash()
	if content, found := cm.layeredCache.Get(hashKey); found {
		if suggestion := cm.parseSuggestionFromContent(content); suggestion != nil {
			cm.recordHit()
			return suggestion, true
		}
	}

	// 然後嘗試 LLM 緩存（相似度匹配）
	if suggestion, found := cm.llmCache.GetSuggestion(key); found {
		// 提升到分層緩存
		if content := cm.serializeSuggestion(suggestion); content != "" {
			cm.layeredCache.Set(hashKey, content, 30*time.Minute)
		}
		cm.recordHit()
		return suggestion, true
	}

	cm.recordMiss()
	return nil, false
}

// SetSuggestion 設置建議到所有適當的緩存層
func (cm *LLMCacheManager) SetSuggestion(key LLMCacheKey, suggestion *llm.Suggestion) error {
	if !cm.enabled {
		return nil
	}

	// 設置到分層緩存
	content := cm.serializeSuggestion(suggestion)
	if content != "" {
		if err := cm.layeredCache.Set(key.Hash(), content, 30*time.Minute); err != nil {
			return err
		}
	}

	// 設置到 LLM 緩存（用於相似度匹配）
	return cm.llmCache.SetSuggestion(key, suggestion)
}

// GetCommand 獲取命令
func (cm *LLMCacheManager) GetCommand(ctx context.Context, key LLMCacheKey) (string, bool) {
	if !cm.enabled {
		return "", false
	}

	start := time.Now()
	defer func() {
		cm.updateStats(time.Since(start))
	}()

	cm.incrementRequests()

	// 首先嘗試分層緩存
	hashKey := "cmd:" + key.Hash()
	if content, found := cm.layeredCache.Get(hashKey); found {
		cm.recordHit()
		return content, true
	}

	// 然後嘗試 LLM 緩存
	if command, found := cm.llmCache.GetCommand(key); found {
		// 提升到分層緩存
		cm.layeredCache.Set(hashKey, command, time.Hour)
		cm.recordHit()
		return command, true
	}

	cm.recordMiss()
	return "", false
}

// SetCommand 設置命令到緩存
func (cm *LLMCacheManager) SetCommand(key LLMCacheKey, command string) error {
	if !cm.enabled {
		return nil
	}

	// 設置到分層緩存
	hashKey := "cmd:" + key.Hash()
	if err := cm.layeredCache.Set(hashKey, command, time.Hour); err != nil {
		return err
	}

	// 設置到 LLM 緩存
	return cm.llmCache.SetCommand(key, command)
}

// GetTemplate 獲取編譯後的模板
func (cm *LLMCacheManager) GetTemplate(name string) (CompiledTemplate, bool) {
	if !cm.enabled || cm.templateCache == nil {
		return CompiledTemplate{}, false
	}

	return cm.templateCache.Get(name)
}

// SetTemplate 緩存編譯後的模板
func (cm *LLMCacheManager) SetTemplate(name string, template CompiledTemplate) {
	if !cm.enabled || cm.templateCache == nil {
		return
	}

	cm.templateCache.Set(name, template)
}

// Invalidate 使特定類型的緩存失效
func (cm *LLMCacheManager) Invalidate(cacheType string, pattern string) error {
	if !cm.enabled {
		return nil
	}

	switch cacheType {
	case "suggestion":
		// 清理建議緩存
		return cm.llmCache.Clear()
	case "command":
		// 清理命令緩存（需要實現模式匹配）
		return cm.llmCache.Clear()
	case "template":
		// 清理模板緩存
		if cm.templateCache != nil {
			cm.templateCache.Clear()
		}
	case "all":
		// 清理所有緩存
		if err := cm.layeredCache.Clear(); err != nil {
			return err
		}
		if err := cm.llmCache.Clear(); err != nil {
			return err
		}
		if cm.templateCache != nil {
			cm.templateCache.Clear()
		}
	}

	return nil
}

// GetStats 獲取綜合統計信息
func (cm *LLMCacheManager) GetStats() CacheManagerStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	stats := cm.stats
	if cm.enabled {
		stats.LayeredStats = cm.layeredCache.GetStats()
		stats.LLMStats = cm.llmCache.GetStats()
		if cm.templateCache != nil {
			stats.TemplateStats = cm.templateCache.GetStats()
		}
	}

	return stats
}

// WarmUp 預熱緩存
func (cm *LLMCacheManager) WarmUp(commonPrompts []string, provider, model string) {
	if !cm.enabled {
		return
	}

	// 預熱 LLM 緩存
	cm.llmCache.WarmUp(commonPrompts, provider, model)

	// 預熱分層緩存
	var keys []string
	for _, prompt := range commonPrompts {
		key := LLMCacheKey{
			Provider: provider,
			Model:    model,
			Prompt:   prompt,
		}
		keys = append(keys, key.Hash())
	}
	cm.layeredCache.WarmUp(keys)
}

// Close 關閉緩存管理器
func (cm *LLMCacheManager) Close() error {
	if !cm.enabled {
		return nil
	}

	var lastErr error

	if cm.layeredCache != nil {
		if err := cm.layeredCache.Close(); err != nil {
			lastErr = err
		}
	}

	if cm.llmCache != nil {
		if err := cm.llmCache.Clear(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// 統計輔助方法
func (cm *LLMCacheManager) incrementRequests() {
	cm.mu.Lock()
	cm.stats.TotalRequests++
	cm.mu.Unlock()
}

func (cm *LLMCacheManager) recordHit() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 更新命中率
	hits := cm.stats.LayeredStats.L1Hits + cm.stats.LayeredStats.L2Hits
	total := hits + cm.stats.LayeredStats.TotalMisses
	if total > 0 {
		cm.stats.CacheHitRate = float64(hits) / float64(total)
	}
}

func (cm *LLMCacheManager) recordMiss() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 統計會在 GetStats 時重新計算
}

func (cm *LLMCacheManager) updateStats(latency time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 更新平均延遲
	if cm.stats.AvgLatency == 0 {
		cm.stats.AvgLatency = latency
	} else {
		cm.stats.AvgLatency = (cm.stats.AvgLatency + latency) / 2
	}
}

// 序列化輔助方法
func (cm *LLMCacheManager) serializeSuggestion(suggestion *llm.Suggestion) string {
	// 簡化版本：實際應該使用 JSON
	return suggestion.Explanation + "|" + suggestion.CorrectedCommand
}

func (cm *LLMCacheManager) parseSuggestionFromContent(content string) *llm.Suggestion {
	// 簡化版本：實際應該使用 JSON 解析
	parts := strings.SplitN(content, "|", 2)
	if len(parts) != 2 {
		return nil
	}

	return &llm.Suggestion{
		Explanation:      parts[0],
		CorrectedCommand: parts[1],
	}
}
