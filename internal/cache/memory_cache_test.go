package cache

import (
	"testing"
	"time"
)

func TestNewMemoryCache(t *testing.T) {
	capacity := 100
	cache := NewMemoryCache(capacity)

	if cache == nil {
		t.Fatal("NewMemoryCache should return non-nil cache")
	}

	if cache.capacity != capacity {
		t.Errorf("Expected capacity %d, got %d", capacity, cache.capacity)
	}

	if len(cache.items) != 0 {
		t.Errorf("Expected empty items map, got length %d", len(cache.items))
	}

	if cache.order.Len() != 0 {
		t.Errorf("Expected empty order list, got length %d", cache.order.Len())
	}

	stats := cache.GetStats()
	if stats.Capacity != capacity {
		t.Errorf("Expected stats capacity %d, got %d", capacity, stats.Capacity)
	}

	if stats.Size != 0 {
		t.Errorf("Expected stats size 0, got %d", stats.Size)
	}
}

func TestMemoryCacheSetGet(t *testing.T) {
	cache := NewMemoryCache(10)

	// Test setting and getting a value
	key := "test-key"
	value := "test-value"
	ttl := time.Hour

	cache.Set(key, value, ttl)

	retrieved, found := cache.Get(key)
	if !found {
		t.Error("Expected to find the key")
	}

	if retrieved != value {
		t.Errorf("Expected value '%s', got '%s'", value, retrieved)
	}

	stats := cache.GetStats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
	}

	if stats.Misses != 0 {
		t.Errorf("Expected 0 misses, got %d", stats.Misses)
	}

	if stats.Size != 1 {
		t.Errorf("Expected size 1, got %d", stats.Size)
	}
}

func TestMemoryCacheGetMiss(t *testing.T) {
	cache := NewMemoryCache(10)

	// Test getting non-existent key
	_, found := cache.Get("non-existent")
	if found {
		t.Error("Expected not to find non-existent key")
	}

	stats := cache.GetStats()
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}

	if stats.Hits != 0 {
		t.Errorf("Expected 0 hits, got %d", stats.Hits)
	}
}

func TestMemoryCacheExpiration(t *testing.T) {
	cache := NewMemoryCache(10)

	// Set with very short TTL
	key := "expire-key"
	value := "expire-value"
	ttl := 50 * time.Millisecond

	cache.Set(key, value, ttl)

	// Should exist immediately
	retrieved, found := cache.Get(key)
	if !found {
		t.Error("Expected to find key before expiration")
	}
	if retrieved != value {
		t.Errorf("Expected value '%s', got '%s'", value, retrieved)
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should not exist after expiration
	_, found = cache.Get(key)
	if found {
		t.Error("Expected key to be expired")
	}

	stats := cache.GetStats()
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss (expired), got %d", stats.Misses)
	}
}

func TestMemoryCacheUpdate(t *testing.T) {
	cache := NewMemoryCache(10)

	key := "update-key"
	value1 := "value1"
	value2 := "value2"
	ttl := time.Hour

	// Set initial value
	cache.Set(key, value1, ttl)

	retrieved, found := cache.Get(key)
	if !found || retrieved != value1 {
		t.Errorf("Expected value '%s', got '%s'", value1, retrieved)
	}

	// Update value
	cache.Set(key, value2, ttl)

	retrieved, found = cache.Get(key)
	if !found || retrieved != value2 {
		t.Errorf("Expected updated value '%s', got '%s'", value2, retrieved)
	}

	// Size should still be 1
	stats := cache.GetStats()
	if stats.Size != 1 {
		t.Errorf("Expected size 1 after update, got %d", stats.Size)
	}
}

func TestMemoryCacheLRUEviction(t *testing.T) {
	capacity := 3
	cache := NewMemoryCache(capacity)

	// Fill cache to capacity
	for i := 0; i < capacity; i++ {
		cache.Set(string(rune('a'+i)), string(rune('A'+i)), time.Hour)
	}

	stats := cache.GetStats()
	if stats.Size != capacity {
		t.Errorf("Expected size %d, got %d", capacity, stats.Size)
	}

	// Add one more item - should evict the LRU item
	cache.Set("d", "D", time.Hour)

	stats = cache.GetStats()
	if stats.Size != capacity {
		t.Errorf("Expected size %d after eviction, got %d", capacity, stats.Size)
	}

	if stats.Evictions != 1 {
		t.Errorf("Expected 1 eviction, got %d", stats.Evictions)
	}

	// The first item should be evicted
	_, found := cache.Get("a")
	if found {
		t.Error("Expected first item to be evicted")
	}

	// Other items should still exist
	_, found = cache.Get("b")
	if !found {
		t.Error("Expected second item to still exist")
	}
}

func TestMemoryCacheDelete(t *testing.T) {
	cache := NewMemoryCache(10)

	key := "delete-key"
	value := "delete-value"

	cache.Set(key, value, time.Hour)

	// Verify it exists
	_, found := cache.Get(key)
	if !found {
		t.Error("Expected key to exist before deletion")
	}

	// Delete it
	cache.Delete(key)

	// Verify it's gone
	_, found = cache.Get(key)
	if found {
		t.Error("Expected key to be deleted")
	}

	stats := cache.GetStats()
	if stats.Size != 0 {
		t.Errorf("Expected size 0 after deletion, got %d", stats.Size)
	}
}

func TestMemoryCacheClear(t *testing.T) {
	cache := NewMemoryCache(10)

	// Add multiple items
	for i := 0; i < 5; i++ {
		cache.Set(string(rune('a'+i)), string(rune('A'+i)), time.Hour)
	}

	stats := cache.GetStats()
	if stats.Size != 5 {
		t.Errorf("Expected size 5 before clear, got %d", stats.Size)
	}

	// Clear cache
	cache.Clear()

	stats = cache.GetStats()
	if stats.Size != 0 {
		t.Errorf("Expected size 0 after clear, got %d", stats.Size)
	}

	// Verify items are gone
	for i := 0; i < 5; i++ {
		_, found := cache.Get(string(rune('a' + i)))
		if found {
			t.Errorf("Expected item %c to be cleared", rune('a'+i))
		}
	}
}

func TestMemoryCacheCleanup(t *testing.T) {
	cache := NewMemoryCache(10)

	// Add items with different expiration times
	cache.Set("short", "value1", 50*time.Millisecond)
	cache.Set("long", "value2", time.Hour)
	cache.Set("medium", "value3", 100*time.Millisecond)

	stats := cache.GetStats()
	if stats.Size != 3 {
		t.Errorf("Expected size 3 before cleanup, got %d", stats.Size)
	}

	// Wait for some items to expire
	time.Sleep(75 * time.Millisecond)

	// Run cleanup
	cache.Cleanup()

	stats = cache.GetStats()
	if stats.Size != 2 {
		t.Errorf("Expected size 2 after cleanup (short expired), got %d", stats.Size)
	}

	// Verify which items remain
	_, found := cache.Get("short")
	if found {
		t.Error("Expected short-lived item to be cleaned up")
	}

	_, found = cache.Get("long")
	if !found {
		t.Error("Expected long-lived item to remain")
	}

	_, found = cache.Get("medium")
	if !found {
		t.Error("Expected medium-lived item to still exist")
	}
}

func TestMemoryCacheEntry(t *testing.T) {
	entry := &MemoryCacheEntry{
		key:        "test",
		value:      "value",
		createdAt:  time.Now(),
		expiresAt:  time.Now().Add(time.Hour),
		lastAccess: time.Now(),
		hitCount:   0,
	}

	// Test not expired
	if entry.IsExpired() {
		t.Error("Entry should not be expired")
	}

	// Test touch
	initialHitCount := entry.hitCount
	initialLastAccess := entry.lastAccess

	time.Sleep(1 * time.Millisecond) // Ensure time difference
	entry.Touch()

	if entry.hitCount != initialHitCount+1 {
		t.Errorf("Expected hit count %d, got %d", initialHitCount+1, entry.hitCount)
	}

	if !entry.lastAccess.After(initialLastAccess) {
		t.Error("Expected last access time to be updated")
	}

	// Test expired entry
	expiredEntry := &MemoryCacheEntry{
		key:        "expired",
		value:      "value",
		createdAt:  time.Now().Add(-2 * time.Hour),
		expiresAt:  time.Now().Add(-time.Hour),
		lastAccess: time.Now().Add(-time.Hour),
		hitCount:   0,
	}

	if !expiredEntry.IsExpired() {
		t.Error("Entry should be expired")
	}
}

func TestMemoryCacheStatsHitRate(t *testing.T) {
	stats := &MemoryCacheStats{
		Hits:   7,
		Misses: 3,
	}

	expectedHitRate := 0.7
	hitRate := stats.HitRate()

	if hitRate != expectedHitRate {
		t.Errorf("Expected hit rate %.2f, got %.2f", expectedHitRate, hitRate)
	}

	// Test zero hits and misses
	emptyStats := &MemoryCacheStats{
		Hits:   0,
		Misses: 0,
	}

	hitRate = emptyStats.HitRate()
	if hitRate != 0 {
		t.Errorf("Expected hit rate 0 for empty stats, got %.2f", hitRate)
	}
}

func TestMemoryCacheConcurrency(t *testing.T) {
	cache := NewMemoryCache(100)

	// Test concurrent access
	done := make(chan bool, 10)

	// Start multiple goroutines
	for i := 0; i < 10; i++ {
		go func(id int) {
			key := string(rune('a' + id))
			value := string(rune('A' + id))

			// Set value
			cache.Set(key, value, time.Hour)

			// Get value multiple times
			for j := 0; j < 10; j++ {
				retrieved, found := cache.Get(key)
				if !found || retrieved != value {
					t.Errorf("Concurrent access failed for key %s", key)
				}
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	stats := cache.GetStats()
	if stats.Size != 10 {
		t.Errorf("Expected size 10 after concurrent operations, got %d", stats.Size)
	}
}
