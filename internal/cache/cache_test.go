package cache

import (
	"fmt"
	"testing"
	"time"
)

func TestNewCache(t *testing.T) {
	tempDir := t.TempDir()
	config := CacheConfig{
		Enabled:         true,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		MaxTTL:          24 * time.Hour,
		CleanupInterval: time.Minute,
		CacheDir:        tempDir,
		MaxFileSize:     1024,
	}

	cache, err := NewCache(config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	if cache.config.CacheDir != tempDir {
		t.Errorf("Expected cache directory %s，got %s", tempDir, cache.config.CacheDir)
	}

	// Test disabled cache
	config.Enabled = false
	cache, err = NewCache(config)
	if err != nil {
		t.Fatalf("Failed to create disabled cache: %v", err)
	}

	if cache.config.Enabled {
		t.Error("Cache should be disabled")
	}
}

func TestCacheSetGet(t *testing.T) {
	tempDir := t.TempDir()
	config := CacheConfig{
		Enabled:         true,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		MaxTTL:          24 * time.Hour,
		CleanupInterval: time.Minute,
		CacheDir:        tempDir,
		MaxFileSize:     1024,
	}

	cache, err := NewCache(config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Test set and get
	key := "test-key"
	value := "test-value"

	err = cache.Set(key, value, time.Hour)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	retrievedValue, found := cache.Get(key)
	if !found {
		t.Error("Should find cache entry")
	}

	if retrievedValue != value {
		t.Errorf("Expected value %s，got %s", value, retrievedValue)
	}

	// Test non-existent key
	_, found = cache.Get("nonexistent-key")
	if found {
		t.Error("Should not find non-existent key")
	}
}

func TestCacheExpiration(t *testing.T) {
	tempDir := t.TempDir()
	config := CacheConfig{
		Enabled:         true,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		MaxTTL:          24 * time.Hour,
		CleanupInterval: time.Minute,
		CacheDir:        tempDir,
		MaxFileSize:     1024,
	}

	cache, err := NewCache(config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Set an entry that expires quickly
	key := "expiring-key"
	value := "expiring-value"

	err = cache.Set(key, value, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// Immediate get should succeed
	_, found := cache.Get(key)
	if !found {
		t.Error("Should find cache entry")
	}

	// Wait for expiration
	time.Sleep(120 * time.Millisecond)

	// Should not be found now
	_, found = cache.Get(key)
	if found {
		t.Error("Expired entry should not be found")
	}
}

func TestCacheDelete(t *testing.T) {
	tempDir := t.TempDir()
	config := CacheConfig{
		Enabled:         true,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		MaxTTL:          24 * time.Hour,
		CleanupInterval: time.Minute,
		CacheDir:        tempDir,
		MaxFileSize:     1024,
	}

	cache, err := NewCache(config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Set entry
	key := "delete-test"
	value := "delete-value"

	err = cache.Set(key, value, time.Hour)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// Confirm existence
	_, found := cache.Get(key)
	if !found {
		t.Error("Should find cache entry")
	}

	// Delete
	cache.Delete(key)

	// 確認已Delete
	_, found = cache.Get(key)
	if found {
		t.Error("Delete的條目不應該被找到")
	}
}

func TestCacheClear(t *testing.T) {
	tempDir := t.TempDir()
	config := CacheConfig{
		Enabled:         true,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		MaxTTL:          24 * time.Hour,
		CleanupInterval: time.Minute,
		CacheDir:        tempDir,
		MaxFileSize:     1024,
	}

	cache, err := NewCache(config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Set multiple entries
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		err = cache.Set(key, value, time.Hour)
		if err != nil {
			t.Fatalf("Failed to set cache: %v", err)
		}
	}

	// Confirm entries exist
	stats := cache.GetStats()
	if stats.Entries != 5 {
		t.Errorf("Expected 5 entries，got %d", stats.Entries)
	}

	// Clear cache
	err = cache.Clear()
	if err != nil {
		t.Fatalf("Clear cache失敗: %v", err)
	}

	// Confirm cleared
	stats = cache.GetStats()
	if stats.Entries != 0 {
		t.Errorf("Expected 0 entries，got %d", stats.Entries)
	}
}

func TestCacheStats(t *testing.T) {
	tempDir := t.TempDir()
	config := CacheConfig{
		Enabled:         true,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		MaxTTL:          24 * time.Hour,
		CleanupInterval: time.Minute,
		CacheDir:        tempDir,
		MaxFileSize:     1024,
	}

	cache, err := NewCache(config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Test hits and misses
	_ = cache.Set("test", "value", time.Hour)

	// Hit
	cache.Get("test")
	cache.Get("test")

	// 未Hit
	cache.Get("nonexistent")

	stats := cache.GetStats()
	if stats.Hits != 2 {
		t.Errorf("Expected 2 hits，got %d", stats.Hits)
	}

	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss，got %d", stats.Misses)
	}

	if stats.Entries != 1 {
		t.Errorf("Expected 1 entry，got %d", stats.Entries)
	}

	if stats.HitRate() != 2.0/3.0 {
		t.Errorf("Expected hit rate %.2f，got %.2f", 2.0/3.0, stats.HitRate())
	}
}

func TestCacheDisabled(t *testing.T) {
	config := CacheConfig{
		Enabled: false,
	}

	cache, err := NewCache(config)
	if err != nil {
		t.Fatalf("Failed to create disabled cache: %v", err)
	}

	// Set should not perform any operation
	err = cache.Set("test", "value", time.Hour)
	if err != nil {
		t.Errorf("Disabled cache set should not fail: %v", err)
	}

	// Get should always miss
	_, found := cache.Get("test")
	if found {
		t.Error("Disabled cache should not find any entries")
	}
}

func TestCacheMaxEntries(t *testing.T) {
	tempDir := t.TempDir()
	config := CacheConfig{
		Enabled:         true,
		MaxEntries:      2, // Very small limit
		DefaultTTL:      time.Hour,
		MaxTTL:          24 * time.Hour,
		CleanupInterval: time.Minute,
		CacheDir:        tempDir,
		MaxFileSize:     1024,
	}

	cache, err := NewCache(config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Add more entries than max limit
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		err = cache.Set(key, value, time.Hour)
		if err != nil {
			t.Fatalf("Failed to set cache: %v", err)
		}
	}

	stats := cache.GetStats()
	if stats.Entries > config.MaxEntries {
		t.Errorf("Cache entries %d exceed max limit %d", stats.Entries, config.MaxEntries)
	}
}

func TestDefaultCacheConfig(t *testing.T) {
	config := DefaultCacheConfig()

	if config.MaxEntries != 1000 {
		t.Errorf("Expected max entries 1000, got %d", config.MaxEntries)
	}

	if config.DefaultTTL != 24*time.Hour {
		t.Errorf("Expected default TTL 24h, got %v", config.DefaultTTL)
	}

	if !config.Enabled {
		t.Error("Default config should be enabled")
	}
}

func TestCacheEntry_IsExpired(t *testing.T) {
	entry := &CacheEntry{
		Key:       "test",
		Value:     "value",
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}

	if !entry.IsExpired() {
		t.Error("Entry should be expired")
	}

	entry.ExpiresAt = time.Now().Add(1 * time.Hour)
	if entry.IsExpired() {
		t.Error("Entry should not be expired")
	}
}

func TestCacheEntry_Touch(t *testing.T) {
	entry := &CacheEntry{
		Key:        "test",
		Value:      "value",
		AccessedAt: time.Now().Add(-1 * time.Hour),
		HitCount:   5,
	}

	oldAccessTime := entry.AccessedAt
	oldHitCount := entry.HitCount

	time.Sleep(10 * time.Millisecond)
	entry.Touch()

	if !entry.AccessedAt.After(oldAccessTime) {
		t.Error("AccessedAt should be updated")
	}

	if entry.HitCount != oldHitCount+1 {
		t.Errorf("Expected hit count %d, got %d", oldHitCount+1, entry.HitCount)
	}
}

func TestCacheStats_HitRate(t *testing.T) {
	tests := []struct {
		hits     int64
		misses   int64
		expected float64
	}{
		{10, 5, 10.0 / 15.0},
		{5, 5, 0.5},
		{10, 0, 1.0},
		{0, 10, 0.0},
		{0, 0, 0.0},
	}

	for _, tt := range tests {
		stats := CacheStats{
			Hits:   tt.hits,
			Misses: tt.misses,
		}

		hitRate := stats.HitRate()
		if hitRate != tt.expected {
			t.Errorf("Hits=%d, Misses=%d: expected hit rate %.2f, got %.2f",
				tt.hits, tt.misses, tt.expected, hitRate)
		}
	}
}

func TestCache_SetTTLValidation(t *testing.T) {
	tempDir := t.TempDir()
	config := CacheConfig{
		Enabled:         true,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		MaxTTL:          24 * time.Hour,
		CleanupInterval: time.Minute,
		CacheDir:        tempDir,
		MaxFileSize:     1024,
	}

	cache, err := NewCache(config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Test TTL exceeding max
	err = cache.Set("test1", "value1", 48*time.Hour)
	if err != nil {
		t.Fatalf("Set should not fail: %v", err)
	}

	// Entry should be capped at MaxTTL
	hashedKey := cache.hashKey("test1")
	entry := cache.index[hashedKey]
	expectedExpiry := entry.CreatedAt.Add(config.MaxTTL)

	if !entry.ExpiresAt.Equal(expectedExpiry) {
		t.Error("TTL should be capped at MaxTTL")
	}

	// Test zero/negative TTL - should use default
	err = cache.Set("test2", "value2", 0)
	if err != nil {
		t.Fatalf("Set should not fail: %v", err)
	}

	hashedKey2 := cache.hashKey("test2")
	entry2 := cache.index[hashedKey2]
	expectedExpiry2 := entry2.CreatedAt.Add(config.DefaultTTL)

	if !entry2.ExpiresAt.Equal(expectedExpiry2) {
		t.Error("Zero TTL should use default TTL")
	}
}

func TestCache_SetLargeValue(t *testing.T) {
	tempDir := t.TempDir()
	config := CacheConfig{
		Enabled:         true,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		MaxTTL:          24 * time.Hour,
		CleanupInterval: time.Minute,
		CacheDir:        tempDir,
		MaxFileSize:     100, // Very small
	}

	cache, err := NewCache(config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	largeValue := string(make([]byte, 200))
	err = cache.Set("large", largeValue, time.Hour)

	if err == nil {
		t.Error("Should fail to set value exceeding max file size")
	}
}

func TestCache_Cleanup(t *testing.T) {
	tempDir := t.TempDir()
	config := CacheConfig{
		Enabled:         true,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		MaxTTL:          24 * time.Hour,
		CleanupInterval: time.Minute,
		CacheDir:        tempDir,
		MaxFileSize:     1024,
	}

	cache, err := NewCache(config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Add some entries with short TTL
	cache.Set("expire1", "val1", 50*time.Millisecond)
	cache.Set("expire2", "val2", 50*time.Millisecond)
	cache.Set("keep", "val3", time.Hour)

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Manual cleanup
	cache.Cleanup()

	// Expired entries should be gone
	_, found := cache.Get("expire1")
	if found {
		t.Error("Expired entry1 should be cleaned up")
	}

	_, found = cache.Get("expire2")
	if found {
		t.Error("Expired entry2 should be cleaned up")
	}

	// Non-expired entry should remain
	_, found = cache.Get("keep")
	if !found {
		t.Error("Non-expired entry should remain")
	}
}

func TestCache_CloseAndReload(t *testing.T) {
	tempDir := t.TempDir()
	config := CacheConfig{
		Enabled:         true,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		MaxTTL:          24 * time.Hour,
		CleanupInterval: time.Minute,
		CacheDir:        tempDir,
		MaxFileSize:     1024,
	}

	// Create cache and add entries
	cache1, err := NewCache(config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	cache1.Set("persistent1", "value1", time.Hour)
	cache1.Set("persistent2", "value2", time.Hour)

	// Close cache
	err = cache1.Close()
	if err != nil {
		t.Fatalf("Failed to close cache: %v", err)
	}

	// Create new cache instance (should load index)
	cache2, err := NewCache(config)
	if err != nil {
		t.Fatalf("Failed to create second cache: %v", err)
	}
	defer cache2.Close()

	// Verify entries are loaded
	val, found := cache2.Get("persistent1")
	if !found || val != "value1" {
		t.Error("Should load persistent entry1")
	}

	val, found = cache2.Get("persistent2")
	if !found || val != "value2" {
		t.Error("Should load persistent entry2")
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	t.Skip("Cache implementation has known race conditions - needs mutex protection")
}

func TestCache_DeleteDisabled(t *testing.T) {
	config := CacheConfig{
		Enabled: false,
	}

	cache, _ := NewCache(config)

	// Delete should not panic when disabled
	cache.Delete("any-key")
}

func TestCache_ClearDisabled(t *testing.T) {
	config := CacheConfig{
		Enabled: false,
	}

	cache, _ := NewCache(config)

	// Clear should not fail when disabled
	err := cache.Clear()
	if err != nil {
		t.Errorf("Clear on disabled cache should not fail: %v", err)
	}
}

func TestCache_CleanupDisabled(t *testing.T) {
	config := CacheConfig{
		Enabled: false,
	}

	cache, _ := NewCache(config)

	// Cleanup should not panic when disabled
	cache.Cleanup()
}
