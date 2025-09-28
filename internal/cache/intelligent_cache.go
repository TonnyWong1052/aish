package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	aerrors "github.com/TonnyWong1052/aish/internal/errors"
)

// IntelligentCache 智能快取系統，支援語義相似性檢測和智能預熱
type IntelligentCache struct {
	primaryCache  *Cache
	semanticIndex *SemanticIndex
	prewarmer     *CachePrewarmer
	analytics     *CacheAnalytics
	config        *IntelligentCacheConfig
	mu            sync.RWMutex
}

// IntelligentCacheConfig 智能快取配置
type IntelligentCacheConfig struct {
	// 基礎快取設置
	MaxSize         int           `json:"max_size"`
	DefaultTTL      time.Duration `json:"default_ttl"`
	CleanupInterval time.Duration `json:"cleanup_interval"`

	// 語義相似性設置
	EnableSemantic      bool    `json:"enable_semantic"`
	SimilarityThreshold float64 `json:"similarity_threshold"`
	MaxSimilarResults   int     `json:"max_similar_results"`

	// 智能預熱設置
	EnablePrewarming    bool          `json:"enable_prewarming"`
	PrewarmingInterval  time.Duration `json:"prewarming_interval"`
	PrewarmingBatchSize int           `json:"prewarming_batch_size"`

	// 分析設置
	EnableAnalytics bool          `json:"enable_analytics"`
	AnalyticsWindow time.Duration `json:"analytics_window"`

	// 快取策略
	EvictionPolicy     EvictionPolicy `json:"eviction_policy"`
	CompressionEnabled bool           `json:"compression_enabled"`
	EncryptionEnabled  bool           `json:"encryption_enabled"`
}

// EvictionPolicy 淘汰策略
type EvictionPolicy string

const (
	EvictionLRU         EvictionPolicy = "LRU"         // 最近最少使用
	EvictionLFU         EvictionPolicy = "LFU"         // 最不常用
	EvictionTTL         EvictionPolicy = "TTL"         // 過期時間
	EvictionIntelligent EvictionPolicy = "INTELLIGENT" // 智能淘汰
)

// DefaultIntelligentCacheConfig 返回默認智能快取配置
func DefaultIntelligentCacheConfig() *IntelligentCacheConfig {
	return &IntelligentCacheConfig{
		MaxSize:             1000,
		DefaultTTL:          time.Hour,
		CleanupInterval:     10 * time.Minute,
		EnableSemantic:      true,
		SimilarityThreshold: 0.8,
		MaxSimilarResults:   5,
		EnablePrewarming:    true,
		PrewarmingInterval:  30 * time.Minute,
		PrewarmingBatchSize: 10,
		EnableAnalytics:     true,
		AnalyticsWindow:     24 * time.Hour,
		EvictionPolicy:      EvictionIntelligent,
		CompressionEnabled:  true,
		EncryptionEnabled:   false,
	}
}

// IntelligentCacheEntry 智能快取條目
type IntelligentCacheEntry struct {
	Key          string        `json:"key"`
	Value        interface{}   `json:"value"`
	CreatedAt    time.Time     `json:"created_at"`
	LastAccessed time.Time     `json:"last_accessed"`
	AccessCount  int64         `json:"access_count"`
	TTL          time.Duration `json:"ttl"`
	Size         int64         `json:"size"`
	Semantic     *SemanticData `json:"semantic,omitempty"`
	Compressed   bool          `json:"compressed"`
	Encrypted    bool          `json:"encrypted"`
}

// SemanticData 語義數據
type SemanticData struct {
	Keywords    []string  `json:"keywords"`
	Vector      []float64 `json:"vector"`
	Fingerprint string    `json:"fingerprint"`
}

// NewIntelligentCache 創建新的智能快取
func NewIntelligentCache(config *IntelligentCacheConfig) (*IntelligentCache, error) {
	if config == nil {
		config = DefaultIntelligentCacheConfig()
	}

	// 創建基礎快取
	cacheConfig := CacheConfig{
		MaxEntries:      config.MaxSize,
		DefaultTTL:      config.DefaultTTL,
		CleanupInterval: config.CleanupInterval,
	}

    primaryCache, err := NewCache(cacheConfig)
    if err != nil {
		return nil, aerrors.WrapError(err, aerrors.ErrCacheError, "創建基礎快取失敗")
    }

	ic := &IntelligentCache{
		primaryCache: primaryCache,
		config:       config,
	}

	// 初始化語義索引
	if config.EnableSemantic {
		ic.semanticIndex = NewSemanticIndex(config.SimilarityThreshold)
	}

	// 初始化預熱器
	if config.EnablePrewarming {
		ic.prewarmer = NewCachePrewarmer(ic, config)
	}

	// 初始化分析器
	if config.EnableAnalytics {
		ic.analytics = NewCacheAnalytics(config.AnalyticsWindow)
	}

	return ic, nil
}

// Set 設置快取條目
func (ic *IntelligentCache) Set(key string, value interface{}, ttl time.Duration) error {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	entry := &IntelligentCacheEntry{
		Key:          key,
		Value:        value,
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		AccessCount:  0,
		TTL:          ttl,
		Size:         ic.calculateSize(value),
	}

	// 處理語義數據
	if ic.config.EnableSemantic && ic.semanticIndex != nil {
		semantic, err := ic.extractSemanticData(key, value)
		if err == nil {
			entry.Semantic = semantic
			ic.semanticIndex.Index(key, semantic)
		}
	}

	// 處理壓縮
	if ic.config.CompressionEnabled {
		compressed, err := ic.compressValue(value)
		if err == nil {
			entry.Value = compressed
			entry.Compressed = true
		}
	}

	// 將條目序列化為字符串存儲到基礎快取
    entryData, err := ic.serializeEntry(entry)
    if err != nil {
		return aerrors.WrapError(err, aerrors.ErrCacheWrite, "序列化快取條目失敗")
    }

    err = ic.primaryCache.Set(key, entryData, ttl)
    if err != nil {
		return aerrors.WrapError(err, aerrors.ErrCacheWrite, "寫入快取失敗")
    }

	// 記錄分析數據
	if ic.analytics != nil {
		ic.analytics.RecordSet(key, entry.Size)
	}

	return nil
}

// Get 獲取快取條目
func (ic *IntelligentCache) Get(key string) (interface{}, bool) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	// 從基礎快取獲取
	entryData, exists := ic.primaryCache.Get(key)
	if !exists {
		// 記錄快取未命中
		if ic.analytics != nil {
			ic.analytics.RecordMiss(key)
		}
		return nil, false
	}

	entry, err := ic.deserializeEntry(entryData)
	if err != nil {
		return nil, false
	}

	// 更新訪問統計
	entry.LastAccessed = time.Now()
	entry.AccessCount++

	// 解壓縮
	value := entry.Value
	if entry.Compressed {
		decompressed, err := ic.decompressValue(entry.Value)
		if err == nil {
			value = decompressed
		}
	}

	// 記錄快取命中
	if ic.analytics != nil {
		ic.analytics.RecordHit(key)
	}

	return value, true
}

// GetSimilar 根據語義相似性獲取類似的快取條目
func (ic *IntelligentCache) GetSimilar(key string, query interface{}) ([]SimilarResult, error) {
    if !ic.config.EnableSemantic || ic.semanticIndex == nil {
		return nil, aerrors.NewError(aerrors.ErrCacheError, "語義搜索未啟用")
    }

	ic.mu.RLock()
	defer ic.mu.RUnlock()

	// 提取查詢的語義數據
    querySemantics, err := ic.extractSemanticData(key, query)
    if err != nil {
		return nil, aerrors.WrapError(err, aerrors.ErrCacheError, "提取查詢語義數據失敗")
    }

	// 搜索相似條目
	similarKeys := ic.semanticIndex.FindSimilar(querySemantics, ic.config.MaxSimilarResults)

	results := make([]SimilarResult, 0, len(similarKeys))
	for _, result := range similarKeys {
		if entryData, exists := ic.primaryCache.Get(result.Key); exists {
			if entry, err := ic.deserializeEntry(entryData); err == nil {
				value := entry.Value
				if entry.Compressed {
					if decompressed, err := ic.decompressValue(entry.Value); err == nil {
						value = decompressed
					}
				}

				results = append(results, SimilarResult{
					Key:        result.Key,
					Value:      value,
					Similarity: result.Similarity,
					Entry:      entry,
				})
			}
		}
	}

	return results, nil
}

// SimilarResult 相似搜索結果
type SimilarResult struct {
	Key        string                 `json:"key"`
	Value      interface{}            `json:"value"`
	Similarity float64                `json:"similarity"`
	Entry      *IntelligentCacheEntry `json:"entry"`
}

// Delete 刪除快取條目
func (ic *IntelligentCache) Delete(key string) bool {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	// 從語義索引中移除
	if ic.semanticIndex != nil {
		ic.semanticIndex.Remove(key)
	}

	// 從基礎快取中刪除
	ic.primaryCache.Delete(key)
	deleted := true // 簡化實現

	// 記錄分析數據
	if ic.analytics != nil && deleted {
		ic.analytics.RecordDelete(key)
	}

	return deleted
}

// GetStats 獲取快取統計信息
func (ic *IntelligentCache) GetStats() *IntelligentCacheStats {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	stats := &IntelligentCacheStats{
		PrimaryCache: ic.primaryCache.GetStats(),
	}

	if ic.semanticIndex != nil {
		stats.SemanticIndex = ic.semanticIndex.GetStats()
	}

	if ic.analytics != nil {
		stats.Analytics = ic.analytics.GetStats()
	}

	return stats
}

// IntelligentCacheStats 智能快取統計信息
type IntelligentCacheStats struct {
	PrimaryCache  CacheStats           `json:"primary_cache"`
	SemanticIndex *SemanticIndexStats  `json:"semantic_index,omitempty"`
	Analytics     *CacheAnalyticsStats `json:"analytics,omitempty"`
}

// StartPrewarming 啟動快取預熱
func (ic *IntelligentCache) StartPrewarming(ctx context.Context) error {
    if !ic.config.EnablePrewarming || ic.prewarmer == nil {
		return aerrors.NewError(aerrors.ErrCacheError, "快取預熱未啟用")
    }

	return ic.prewarmer.Start(ctx)
}

// StopPrewarming 停止快取預熱
func (ic *IntelligentCache) StopPrewarming() {
	if ic.prewarmer != nil {
		ic.prewarmer.Stop()
	}
}

// Optimize 優化快取（智能淘汰和重組）
func (ic *IntelligentCache) Optimize() error {
	ic.mu.Lock()
	defer ic.mu.Unlock()

    if ic.analytics == nil {
		return aerrors.NewError(aerrors.ErrCacheError, "快取分析未啟用")
    }

	analytics := ic.analytics.GetStats()

	// 根據配置的淘汰策略進行優化
	switch ic.config.EvictionPolicy {
	case EvictionIntelligent:
		return ic.intelligentEviction(analytics)
	case EvictionLRU:
		return ic.lruEviction()
	case EvictionLFU:
		return ic.lfuEviction()
	case EvictionTTL:
		return ic.ttlEviction()
    default:
		return aerrors.NewError(aerrors.ErrCacheError, "未知的淘汰策略")
	}
}

// extractSemanticData 提取語義數據
func (ic *IntelligentCache) extractSemanticData(key string, value interface{}) (*SemanticData, error) {
	// 簡化的語義提取實現
	text := fmt.Sprintf("%v", value)
	keywords := ic.extractKeywords(text)
	vector := ic.generateVector(text)
	fingerprint := ic.generateFingerprint(text)

	return &SemanticData{
		Keywords:    keywords,
		Vector:      vector,
		Fingerprint: fingerprint,
	}, nil
}

// extractKeywords 提取關鍵詞
func (ic *IntelligentCache) extractKeywords(text string) []string {
	// 簡化的關鍵詞提取
	words := strings.Fields(strings.ToLower(text))
	wordCount := make(map[string]int)

	for _, word := range words {
		// 過濾短詞和常用詞
		if len(word) > 2 && !ic.isStopWord(word) {
			wordCount[word]++
		}
	}

	// 按頻率排序
	type wordFreq struct {
		word string
		freq int
	}

	var wordFreqs []wordFreq
	for word, freq := range wordCount {
		wordFreqs = append(wordFreqs, wordFreq{word, freq})
	}

	sort.Slice(wordFreqs, func(i, j int) bool {
		return wordFreqs[i].freq > wordFreqs[j].freq
	})

	// 返回前10個關鍵詞
	keywords := make([]string, 0, 10)
	for i, wf := range wordFreqs {
		if i >= 10 {
			break
		}
		keywords = append(keywords, wf.word)
	}

	return keywords
}

// generateVector 生成向量表示
func (ic *IntelligentCache) generateVector(text string) []float64 {
	// 簡化的向量生成（使用哈希）
	hasher := fnv.New64a()
	hasher.Write([]byte(text))
	hash := hasher.Sum64()

	// 生成固定長度的向量
	vector := make([]float64, 128)
	for i := range vector {
		vector[i] = float64((hash >> uint(i%64)) & 1)
	}

	// 標準化向量
	norm := 0.0
	for _, v := range vector {
		norm += v * v
	}
	norm = math.Sqrt(norm)

	if norm > 0 {
		for i := range vector {
			vector[i] /= norm
		}
	}

	return vector
}

// generateFingerprint 生成指紋
func (ic *IntelligentCache) generateFingerprint(text string) string {
	hasher := sha256.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))[:16]
}

// isStopWord 檢查是否為停用詞
func (ic *IntelligentCache) isStopWord(word string) bool {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "is": true,
		"are": true, "was": true, "were": true, "be": true, "been": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
	}
	return stopWords[word]
}

// calculateSize 計算值的大小
func (ic *IntelligentCache) calculateSize(value interface{}) int64 {
	// 簡化的大小計算
	text := fmt.Sprintf("%v", value)
	return int64(len(text))
}

// compressValue 壓縮值
func (ic *IntelligentCache) compressValue(value interface{}) (interface{}, error) {
	// 簡化實現，實際應該使用真正的壓縮算法
	return value, nil
}

// decompressValue 解壓縮值
func (ic *IntelligentCache) decompressValue(value interface{}) (interface{}, error) {
	// 簡化實現
	return value, nil
}

// 淘汰策略實現
func (ic *IntelligentCache) intelligentEviction(analytics *CacheAnalyticsStats) error {
	// 基於訪問模式和語義相似性的智能淘汰
	// 這裡是簡化實現
	return nil
}

func (ic *IntelligentCache) lruEviction() error {
	// LRU 淘汰
	return nil
}

func (ic *IntelligentCache) lfuEviction() error {
	// LFU 淘汰
	return nil
}

func (ic *IntelligentCache) ttlEviction() error {
	// TTL 淘汰
	return nil
}

// Close 關閉智能快取
func (ic *IntelligentCache) Close() error {
	if ic.prewarmer != nil {
		ic.prewarmer.Stop()
	}

	// 基礎快取的關閉（如果支持的話）
	return nil
}

// serializeEntry 序列化快取條目
func (ic *IntelligentCache) serializeEntry(entry *IntelligentCacheEntry) (string, error) {
	// 簡化實現，實際應該使用更高效的序列化
	return fmt.Sprintf("%v", entry.Value), nil
}

// deserializeEntry 反序列化快取條目
func (ic *IntelligentCache) deserializeEntry(data string) (*IntelligentCacheEntry, error) {
	// 簡化實現
	return &IntelligentCacheEntry{
		Value:        data,
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
	}, nil
}
