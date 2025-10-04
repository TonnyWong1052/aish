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
