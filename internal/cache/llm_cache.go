package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/TonnyWong1052/aish/internal/llm"
)

// LLMCacheKey LLM cache key structure
type LLMCacheKey struct {
	Provider    string              `json:"provider"`
	Model       string              `json:"model"`
	Context     llm.CapturedContext `json:"context"`
	Language    string              `json:"language"`
	RequestType string              `json:"request_type"` // "suggestion" or "command_generation"
	Prompt      string              `json:"prompt,omitempty"`
}

// Hash generates hash value for cache key
func (k *LLMCacheKey) Hash() string {
	data, _ := json.Marshal(k)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("llm_%x", hash)
}

// LLMCachedResponse LLM cached response structure
type LLMCachedResponse struct {
	Suggestion  *llm.Suggestion `json:"suggestion,omitempty"`
	Command     string          `json:"command,omitempty"`
	CachedAt    time.Time       `json:"cached_at"`
	Provider    string          `json:"provider"`
	Model       string          `json:"model"`
	RequestType string          `json:"request_type"`
}

// LLMCache LLM-specific cache
type LLMCache struct {
	cache           *Cache
	config          LLMCacheConfig
	similarityCache *SimilarityCache
}

// LLMCacheConfig LLM cache configuration
type LLMCacheConfig struct {
	Enabled             bool          `json:"enabled"`
	DefaultTTL          time.Duration `json:"default_ttl"`
	SuggestionTTL       time.Duration `json:"suggestion_ttl"`
	CommandTTL          time.Duration `json:"command_ttl"`
	EnableSimilarity    bool          `json:"enable_similarity"`
	SimilarityThreshold float64       `json:"similarity_threshold"`
	MaxSimilarityCache  int           `json:"max_similarity_cache"`
}

// DefaultLLMCacheConfig default LLM cache configuration
func DefaultLLMCacheConfig() LLMCacheConfig {
	return LLMCacheConfig{
		Enabled:             true,
		DefaultTTL:          24 * time.Hour,
		SuggestionTTL:       6 * time.Hour,  // Error suggestion cache time is shorter
		CommandTTL:          24 * time.Hour, // Command generation cache time is longer
		EnableSimilarity:    true,
		SimilarityThreshold: 0.85, // 85% similarity threshold
		MaxSimilarityCache:  500,
	}
}

// NewLLMCache 創建新的 LLM 緩存
func NewLLMCache(baseCache *Cache, config LLMCacheConfig) *LLMCache {
	var similarityCache *SimilarityCache
	if config.EnableSimilarity {
		similarityCache = NewSimilarityCache(config.MaxSimilarityCache, config.SimilarityThreshold)
	}

	return &LLMCache{
		cache:           baseCache,
		config:          config,
		similarityCache: similarityCache,
	}
}

// GetSuggestion 獲取建議緩存
func (lc *LLMCache) GetSuggestion(key LLMCacheKey) (*llm.Suggestion, bool) {
	if !lc.config.Enabled {
		return nil, false
	}

	key.RequestType = "suggestion"

	// 首先嘗試精確匹配
	if suggestion := lc.getExactMatch(key); suggestion != nil {
		return suggestion, true
	}

	// 然後嘗試相似匹配
	if lc.config.EnableSimilarity {
		return lc.getSimilarSuggestion(key)
	}

	return nil, false
}

// SetSuggestion 設置建議緩存
func (lc *LLMCache) SetSuggestion(key LLMCacheKey, suggestion *llm.Suggestion) error {
	if !lc.config.Enabled {
		return nil
	}

	key.RequestType = "suggestion"

	response := LLMCachedResponse{
		Suggestion:  suggestion,
		CachedAt:    time.Now(),
		Provider:    key.Provider,
		Model:       key.Model,
		RequestType: "suggestion",
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}

	// 設置到基礎緩存
	if err := lc.cache.Set(key.Hash(), string(responseJSON), lc.config.SuggestionTTL); err != nil {
		return err
	}

	// 添加到相似度緩存
	if lc.config.EnableSimilarity {
		lc.similarityCache.Add(key, string(responseJSON))
	}

	return nil
}

// GetCommand 獲取命令緩存
func (lc *LLMCache) GetCommand(key LLMCacheKey) (string, bool) {
	if !lc.config.Enabled {
		return "", false
	}

	key.RequestType = "command_generation"

	// 首先嘗試精確匹配
	if command := lc.getExactCommandMatch(key); command != "" {
		return command, true
	}

	// 然後嘗試相似匹配
	if lc.config.EnableSimilarity {
		return lc.getSimilarCommand(key)
	}

	return "", false
}

// SetCommand 設置命令緩存
func (lc *LLMCache) SetCommand(key LLMCacheKey, command string) error {
	if !lc.config.Enabled {
		return nil
	}

	key.RequestType = "command_generation"

	response := LLMCachedResponse{
		Command:     command,
		CachedAt:    time.Now(),
		Provider:    key.Provider,
		Model:       key.Model,
		RequestType: "command_generation",
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}

	// 設置到基礎緩存
	if err := lc.cache.Set(key.Hash(), string(responseJSON), lc.config.CommandTTL); err != nil {
		return err
	}

	// 添加到相似度緩存
	if lc.config.EnableSimilarity {
		lc.similarityCache.Add(key, string(responseJSON))
	}

	return nil
}

// getExactMatch 獲取精確匹配的建議
func (lc *LLMCache) getExactMatch(key LLMCacheKey) *llm.Suggestion {
	responseStr, found := lc.cache.Get(key.Hash())
	if !found {
		return nil
	}

	var response LLMCachedResponse
	if err := json.Unmarshal([]byte(responseStr), &response); err != nil {
		return nil
	}

	return response.Suggestion
}

// getExactCommandMatch 獲取精確匹配的命令
func (lc *LLMCache) getExactCommandMatch(key LLMCacheKey) string {
	responseStr, found := lc.cache.Get(key.Hash())
	if !found {
		return ""
	}

	var response LLMCachedResponse
	if err := json.Unmarshal([]byte(responseStr), &response); err != nil {
		return ""
	}

	return response.Command
}

// getSimilarSuggestion 獲取相似的建議
func (lc *LLMCache) getSimilarSuggestion(key LLMCacheKey) (*llm.Suggestion, bool) {
	similarResponseStr := lc.similarityCache.GetSimilar(key)
	if similarResponseStr == "" {
		return nil, false
	}

	var response LLMCachedResponse
	if err := json.Unmarshal([]byte(similarResponseStr), &response); err != nil {
		return nil, false
	}

	return response.Suggestion, true
}

// getSimilarCommand 獲取相似的命令
func (lc *LLMCache) getSimilarCommand(key LLMCacheKey) (string, bool) {
	similarResponseStr := lc.similarityCache.GetSimilar(key)
	if similarResponseStr == "" {
		return "", false
	}

	var response LLMCachedResponse
	if err := json.Unmarshal([]byte(similarResponseStr), &response); err != nil {
		return "", false
	}

	return response.Command, true
}

// Clear 清空 LLM 緩存
func (lc *LLMCache) Clear() error {
	if lc.similarityCache != nil {
		lc.similarityCache.Clear()
	}
	return lc.cache.Clear()
}

// GetStats 獲取緩存統計
func (lc *LLMCache) GetStats() LLMCacheStats {
	baseStats := lc.cache.GetStats()

	stats := LLMCacheStats{
		Hits:        baseStats.Hits,
		Misses:      baseStats.Misses,
		Entries:     baseStats.Entries,
		LastCleanup: baseStats.LastCleanup,
		HitRate:     baseStats.HitRate(),
	}

	if lc.similarityCache != nil {
		stats.SimilarityHits = lc.similarityCache.GetHits()
		stats.SimilarityEntries = lc.similarityCache.GetSize()
	}

	return stats
}

// LLMCacheStats LLM 緩存統計
type LLMCacheStats struct {
	Hits              int64     `json:"hits"`
	Misses            int64     `json:"misses"`
	Entries           int       `json:"entries"`
	LastCleanup       time.Time `json:"last_cleanup"`
	HitRate           float64   `json:"hit_rate"`
	SimilarityHits    int64     `json:"similarity_hits"`
	SimilarityEntries int       `json:"similarity_entries"`
}

// SimilarityCache 相似度緩存
type SimilarityCache struct {
	entries   []SimilarityCacheEntry
	maxSize   int
	threshold float64
	hits      int64
}

// SimilarityCacheEntry 相似度緩存條目
type SimilarityCacheEntry struct {
	Key      LLMCacheKey `json:"key"`
	Response string      `json:"response"`
	AddedAt  time.Time   `json:"added_at"`
}

// NewSimilarityCache 創建新的相似度緩存
func NewSimilarityCache(maxSize int, threshold float64) *SimilarityCache {
	return &SimilarityCache{
		entries:   make([]SimilarityCacheEntry, 0, maxSize),
		maxSize:   maxSize,
		threshold: threshold,
	}
}

// Add 添加到相似度緩存
func (sc *SimilarityCache) Add(key LLMCacheKey, response string) {
	entry := SimilarityCacheEntry{
		Key:      key,
		Response: response,
		AddedAt:  time.Now(),
	}

	// 如果已滿，移除最舊的條目
	if len(sc.entries) >= sc.maxSize {
		sc.entries = sc.entries[1:]
	}

	sc.entries = append(sc.entries, entry)
}

// GetSimilar 獲取相似的響應
func (sc *SimilarityCache) GetSimilar(key LLMCacheKey) string {
	bestMatch := ""
	bestSimilarity := 0.0

	for _, entry := range sc.entries {
		if entry.Key.RequestType != key.RequestType {
			continue
		}

		similarity := sc.calculateSimilarity(key, entry.Key)
		if similarity >= sc.threshold && similarity > bestSimilarity {
			bestSimilarity = similarity
			bestMatch = entry.Response
		}
	}

	if bestMatch != "" {
		sc.hits++
	}

	return bestMatch
}

// calculateSimilarity 計算兩個緩存鍵的相似度
func (sc *SimilarityCache) calculateSimilarity(key1, key2 LLMCacheKey) float64 {
	// 簡單的文本相似度計算
	// 可以使用更複雜的算法如編輯距離或詞向量相似度

	// 命令相似度
	var commandSim float64
	if key1.Context.Command != "" || key2.Context.Command != "" {
		commandSim = sc.textSimilarity(key1.Context.Command, key2.Context.Command)
	} else {
		commandSim = 1.0 // 都為空，視為相同
	}

	// 錯誤輸出相似度
	var stderrSim float64
	if key1.Context.Stderr != "" || key2.Context.Stderr != "" {
		stderrSim = sc.textSimilarity(key1.Context.Stderr, key2.Context.Stderr)
	} else {
		stderrSim = 1.0
	}

	// 退出代碼相似度
	var exitCodeSim float64
	if key1.Context.ExitCode == key2.Context.ExitCode {
		exitCodeSim = 1.0
	} else {
		exitCodeSim = 0.0
	}

	// 提示相似度（如果有）
	var promptSim float64 = 1.0
	if key1.Prompt != "" || key2.Prompt != "" {
		promptSim = sc.textSimilarity(key1.Prompt, key2.Prompt)
	}

	// 加權平均
	totalSim := (commandSim*0.3 + stderrSim*0.4 + exitCodeSim*0.2 + promptSim*0.1)

	return totalSim
}

// textSimilarity 計算文本相似度（簡單版本）
func (sc *SimilarityCache) textSimilarity(text1, text2 string) float64 {
	if text1 == text2 {
		return 1.0
	}
	if text1 == "" || text2 == "" {
		return 0.0
	}

	// 轉換為小寫並分詞
	words1 := strings.Fields(strings.ToLower(text1))
	words2 := strings.Fields(strings.ToLower(text2))

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	// 計算詞集合交集
	wordSet1 := make(map[string]bool)
	for _, word := range words1 {
		wordSet1[word] = true
	}

	intersection := 0
	for _, word := range words2 {
		if wordSet1[word] {
			intersection++
		}
	}

	// Jaccard 相似度
	union := len(words1) + len(words2) - intersection
	if union == 0 {
		return 1.0
	}

	return float64(intersection) / float64(union)
}

// Clear 清空相似度緩存
func (sc *SimilarityCache) Clear() {
	sc.entries = sc.entries[:0]
	sc.hits = 0
}

// GetHits 獲取相似度緩存命中次數
func (sc *SimilarityCache) GetHits() int64 {
	return sc.hits
}

// GetSize 獲取相似度緩存大小
func (sc *SimilarityCache) GetSize() int {
	return len(sc.entries)
}
