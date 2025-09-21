package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/TonnyWong1052/aish/internal/errors"
)

// CacheEntry cache entry
type CacheEntry struct {
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	AccessedAt time.Time `json:"accessed_at"`
	HitCount   int64     `json:"hit_count"`
	Tags       []string  `json:"tags,omitempty"`
}

// IsExpired checks if cache is expired
func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// Touch updates access time and hit count
func (e *CacheEntry) Touch() {
	e.AccessedAt = time.Now()
	e.HitCount++
}

// CacheConfig cache configuration
type CacheConfig struct {
	MaxEntries      int           `json:"max_entries"`      // Maximum cache entries
	DefaultTTL      time.Duration `json:"default_ttl"`      // Default expiration time
	MaxTTL          time.Duration `json:"max_ttl"`          // Maximum expiration time
	CleanupInterval time.Duration `json:"cleanup_interval"` // Cleanup interval
	CacheDir        string        `json:"cache_dir"`        // Cache directory
	MaxFileSize     int64         `json:"max_file_size"`    // Maximum size of single cache file
	Enabled         bool          `json:"enabled"`          // Whether cache is enabled
}

// DefaultCacheConfig 返回默認緩存配置
func DefaultCacheConfig() CacheConfig {
	home, _ := os.UserHomeDir()
	return CacheConfig{
		MaxEntries:      1000,
		DefaultTTL:      24 * time.Hour,
		MaxTTL:          7 * 24 * time.Hour,
		CleanupInterval: time.Hour,
		CacheDir:        filepath.Join(home, ".config", "aish", "cache"),
		MaxFileSize:     1024 * 1024, // 1MB
		Enabled:         true,
	}
}

// Cache 緩存實現
type Cache struct {
	config CacheConfig
	index  map[string]*CacheEntry
	stats  CacheStats
}

// CacheStats 緩存統計
type CacheStats struct {
	Hits        int64     `json:"hits"`
	Misses      int64     `json:"misses"`
	Entries     int       `json:"entries"`
	LastCleanup time.Time `json:"last_cleanup"`
}

// HitRate 緩存命中率
func (s *CacheStats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total)
}

// NewCache 創建新的緩存實例
func NewCache(config CacheConfig) (*Cache, error) {
	if !config.Enabled {
		return &Cache{
			config: config,
			index:  make(map[string]*CacheEntry),
		}, nil
	}

	// 創建緩存目錄
	if err := os.MkdirAll(config.CacheDir, 0755); err != nil {
		return nil, errors.ErrFileSystemError("create_cache_dir", config.CacheDir, err)
	}

	cache := &Cache{
		config: config,
		index:  make(map[string]*CacheEntry),
		stats: CacheStats{
			LastCleanup: time.Now(),
		},
	}

	// 加載現有緩存索引
	if err := cache.loadIndex(); err != nil {
		// 如果加載失敗，從空索引開始（不返回錯誤）
		cache.index = make(map[string]*CacheEntry)
	}

	// 啟動清理協程
	go cache.startCleanupRoutine()

	return cache, nil
}

// Get 獲取緩存值
func (c *Cache) Get(key string) (string, bool) {
	if !c.config.Enabled {
		c.stats.Misses++
		return "", false
	}

	hashedKey := c.hashKey(key)
	entry, exists := c.index[hashedKey]

	if !exists {
		c.stats.Misses++
		return "", false
	}

	// 檢查是否過期
	if entry.IsExpired() {
		c.delete(hashedKey)
		c.stats.Misses++
		return "", false
	}

	// 讀取文件內容
	content, err := c.readCacheFile(hashedKey)
	if err != nil {
		c.delete(hashedKey)
		c.stats.Misses++
		return "", false
	}

	// 更新訪問信息
	entry.Touch()
	c.stats.Hits++

	return content, true
}

// Set 設置緩存值
func (c *Cache) Set(key, value string, ttl time.Duration) error {
	if !c.config.Enabled {
		return nil
	}

	// 驗證 TTL
	if ttl > c.config.MaxTTL {
		ttl = c.config.MaxTTL
	}
	if ttl <= 0 {
		ttl = c.config.DefaultTTL
	}

	// 檢查值大小
	if int64(len(value)) > c.config.MaxFileSize {
		return errors.NewError(errors.ErrCacheError, "緩存值過大")
	}

	hashedKey := c.hashKey(key)
	now := time.Now()

	// 創建緩存條目
	entry := &CacheEntry{
		Key:        key,
		Value:      value,
		CreatedAt:  now,
		ExpiresAt:  now.Add(ttl),
		AccessedAt: now,
		HitCount:   0,
	}

	// 檢查是否需要清理空間
	if len(c.index) >= c.config.MaxEntries {
		c.evictLRU()
	}

	// 寫入文件
	if err := c.writeCacheFile(hashedKey, value); err != nil {
		return err
	}

	// 更新索引
	c.index[hashedKey] = entry
	c.stats.Entries = len(c.index)

	// 保存索引
	c.saveIndex()

	return nil
}

// Delete 刪除緩存條目
func (c *Cache) Delete(key string) {
	if !c.config.Enabled {
		return
	}

	hashedKey := c.hashKey(key)
	c.delete(hashedKey)
}

// delete 內部刪除方法
func (c *Cache) delete(hashedKey string) {
	// 從索引中刪除
	delete(c.index, hashedKey)

	// 刪除緩存文件
	cacheFile := filepath.Join(c.config.CacheDir, hashedKey)
	os.Remove(cacheFile)

	c.stats.Entries = len(c.index)
}

// Clear 清空所有緩存
func (c *Cache) Clear() error {
	if !c.config.Enabled {
		return nil
	}

	// 刪除所有緩存文件
	for hashedKey := range c.index {
		cacheFile := filepath.Join(c.config.CacheDir, hashedKey)
		os.Remove(cacheFile)
	}

	// 清空索引
	c.index = make(map[string]*CacheEntry)
	c.stats.Entries = 0

	// 保存空索引
	return c.saveIndex()
}

// GetStats 獲取緩存統計
func (c *Cache) GetStats() CacheStats {
	return c.stats
}

// Cleanup 手動清理過期緩存
func (c *Cache) Cleanup() {
	if !c.config.Enabled {
		return
	}

	now := time.Now()
	var expiredKeys []string

	// 找出過期的緩存條目
	for hashedKey, entry := range c.index {
		if entry.IsExpired() {
			expiredKeys = append(expiredKeys, hashedKey)
		}
	}

	// 刪除過期條目
	for _, key := range expiredKeys {
		c.delete(key)
	}

	c.stats.LastCleanup = now
	c.saveIndex()
}

// evictLRU 使用 LRU 策略驅逐緩存
func (c *Cache) evictLRU() {
	if len(c.index) == 0 {
		return
	}

	// 找出最久未訪問的條目
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.index {
		if oldestKey == "" || entry.AccessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.AccessedAt
		}
	}

	if oldestKey != "" {
		c.delete(oldestKey)
	}
}

// startCleanupRoutine 啟動清理協程
func (c *Cache) startCleanupRoutine() {
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		c.Cleanup()
	}
}

// hashKey 對鍵進行哈希
func (c *Cache) hashKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", hash)
}

// readCacheFile 讀取緩存文件
func (c *Cache) readCacheFile(hashedKey string) (string, error) {
	cacheFile := filepath.Join(c.config.CacheDir, hashedKey)
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return "", errors.ErrFileSystemError("read_cache", cacheFile, err)
	}
	return string(data), nil
}

// writeCacheFile 寫入緩存文件
func (c *Cache) writeCacheFile(hashedKey, content string) error {
	cacheFile := filepath.Join(c.config.CacheDir, hashedKey)
	if err := os.WriteFile(cacheFile, []byte(content), 0644); err != nil {
		return errors.ErrFileSystemError("write_cache", cacheFile, err)
	}
	return nil
}

// loadIndex 加載緩存索引
func (c *Cache) loadIndex() error {
	indexFile := filepath.Join(c.config.CacheDir, "index.json")

	data, err := os.ReadFile(indexFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 索引文件不存在是正常的
		}
		return errors.ErrFileSystemError("read_index", indexFile, err)
	}

	var index map[string]*CacheEntry
	if err := json.Unmarshal(data, &index); err != nil {
		return errors.ErrFileSystemError("parse_index", indexFile, err)
	}

	c.index = index
	c.stats.Entries = len(index)

	return nil
}

// saveIndex 保存緩存索引
func (c *Cache) saveIndex() error {
	if !c.config.Enabled {
		return nil
	}

	indexFile := filepath.Join(c.config.CacheDir, "index.json")

	data, err := json.MarshalIndent(c.index, "", "  ")
	if err != nil {
		return errors.ErrFileSystemError("marshal_index", indexFile, err)
	}

	if err := os.WriteFile(indexFile, data, 0644); err != nil {
		return errors.ErrFileSystemError("write_index", indexFile, err)
	}

	return nil
}

// Close 關閉緩存（保存索引）
func (c *Cache) Close() error {
	return c.saveIndex()
}
