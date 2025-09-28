package cache

import (
	"math"
	"sort"
	"sync"
)

// SemanticIndex 語義索引，用於快速相似性搜索
type SemanticIndex struct {
	entries   map[string]*IndexEntry
	threshold float64
	mu        sync.RWMutex
}

// IndexEntry 索引條目
type IndexEntry struct {
	Key       string        `json:"key"`
	Semantic  *SemanticData `json:"semantic"`
	CreatedAt int64         `json:"created_at"`
}

// SemanticIndexStats 語義索引統計
type SemanticIndexStats struct {
	TotalEntries        int   `json:"total_entries"`
	AverageVectorLength int   `json:"average_vector_length"`
	IndexSize           int64 `json:"index_size"`
}

// SimilarityResult 相似性搜索結果
type SimilarityResult struct {
	Key        string  `json:"key"`
	Similarity float64 `json:"similarity"`
}

// NewSemanticIndex 創建新的語義索引
func NewSemanticIndex(threshold float64) *SemanticIndex {
	return &SemanticIndex{
		entries:   make(map[string]*IndexEntry),
		threshold: threshold,
	}
}

// Index 將條目添加到語義索引
func (si *SemanticIndex) Index(key string, semantic *SemanticData) {
	si.mu.Lock()
	defer si.mu.Unlock()

	si.entries[key] = &IndexEntry{
		Key:       key,
		Semantic:  semantic,
		CreatedAt: getCurrentTimestamp(),
	}
}

// Remove 從語義索引中移除條目
func (si *SemanticIndex) Remove(key string) {
	si.mu.Lock()
	defer si.mu.Unlock()

	delete(si.entries, key)
}

// FindSimilar 查找相似的條目
func (si *SemanticIndex) FindSimilar(query *SemanticData, maxResults int) []SimilarityResult {
	si.mu.RLock()
	defer si.mu.RUnlock()

	var results []SimilarityResult

	for key, entry := range si.entries {
		similarity := si.calculateSimilarity(query, entry.Semantic)
		if similarity >= si.threshold {
			results = append(results, SimilarityResult{
				Key:        key,
				Similarity: similarity,
			})
		}
	}

	// 按相似度排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	// 限制結果數量
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	return results
}

// calculateSimilarity 計算兩個語義數據之間的相似度
func (si *SemanticIndex) calculateSimilarity(a, b *SemanticData) float64 {
	// 結合多種相似度計算方法
	weights := map[string]float64{
		"keywords":    0.3,
		"vector":      0.5,
		"fingerprint": 0.2,
	}

	var totalSimilarity float64

	// 關鍵詞相似度
	keywordSim := si.calculateKeywordSimilarity(a.Keywords, b.Keywords)
	totalSimilarity += weights["keywords"] * keywordSim

	// 向量相似度
	vectorSim := si.calculateVectorSimilarity(a.Vector, b.Vector)
	totalSimilarity += weights["vector"] * vectorSim

	// 指紋相似度
	fingerprintSim := si.calculateFingerprintSimilarity(a.Fingerprint, b.Fingerprint)
	totalSimilarity += weights["fingerprint"] * fingerprintSim

	return totalSimilarity
}

// calculateKeywordSimilarity 計算關鍵詞相似度（Jaccard相似度）
func (si *SemanticIndex) calculateKeywordSimilarity(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	setA := make(map[string]bool)
	for _, keyword := range a {
		setA[keyword] = true
	}

	setB := make(map[string]bool)
	for _, keyword := range b {
		setB[keyword] = true
	}

	// 計算交集
	intersection := 0
	for keyword := range setA {
		if setB[keyword] {
			intersection++
		}
	}

	// 計算並集
	union := len(setA) + len(setB) - intersection

	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// calculateVectorSimilarity 計算向量相似度（餘弦相似度）
func (si *SemanticIndex) calculateVectorSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	if len(a) == 0 {
		return 1.0
	}

	var dotProduct, normA, normB float64

	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	normA = math.Sqrt(normA)
	normB = math.Sqrt(normB)

	if normA == 0.0 || normB == 0.0 {
		return 0.0
	}

	return dotProduct / (normA * normB)
}

// calculateFingerprintSimilarity 計算指紋相似度（漢明距離）
func (si *SemanticIndex) calculateFingerprintSimilarity(a, b string) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	if len(a) == 0 {
		return 1.0
	}

	matches := 0
	for i := range a {
		if a[i] == b[i] {
			matches++
		}
	}

	return float64(matches) / float64(len(a))
}

// GetStats 獲取語義索引統計信息
func (si *SemanticIndex) GetStats() *SemanticIndexStats {
	si.mu.RLock()
	defer si.mu.RUnlock()

	totalVectorLength := 0
	for _, entry := range si.entries {
		if entry.Semantic != nil {
			totalVectorLength += len(entry.Semantic.Vector)
		}
	}

	avgVectorLength := 0
	if len(si.entries) > 0 {
		avgVectorLength = totalVectorLength / len(si.entries)
	}

	return &SemanticIndexStats{
		TotalEntries:        len(si.entries),
		AverageVectorLength: avgVectorLength,
		IndexSize:           int64(len(si.entries) * 1024), // 粗略估計
	}
}

// Clear 清空語義索引
func (si *SemanticIndex) Clear() {
	si.mu.Lock()
	defer si.mu.Unlock()

	si.entries = make(map[string]*IndexEntry)
}

// UpdateThreshold 更新相似度閾值
func (si *SemanticIndex) UpdateThreshold(threshold float64) {
	si.mu.Lock()
	defer si.mu.Unlock()

	si.threshold = threshold
}

// GetThreshold 獲取當前相似度閾值
func (si *SemanticIndex) GetThreshold() float64 {
	si.mu.RLock()
	defer si.mu.RUnlock()

	return si.threshold
}

// getCurrentTimestamp 獲取當前時間戳
func getCurrentTimestamp() int64 {
	return int64(1000) // 簡化實現
}
