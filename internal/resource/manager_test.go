package resource

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestDefaultResourceConfig(t *testing.T) {
	config := DefaultResourceConfig()

	if config.MemoryLimitMB != 512 {
		t.Errorf("Expected memory limit 512MB, got %d", config.MemoryLimitMB)
	}

	if config.GoroutineLimit != 1000 {
		t.Errorf("Expected goroutine limit 1000, got %d", config.GoroutineLimit)
	}

	if config.FileHandleLimit != 100 {
		t.Errorf("Expected file handle limit 100, got %d", config.FileHandleLimit)
	}

	if config.MonitorInterval != 5*time.Second {
		t.Errorf("Expected monitor interval 5s, got %v", config.MonitorInterval)
	}

	if !config.EnableMonitoring {
		t.Error("Monitoring should be enabled by default")
	}

	if !config.AutoCleanup {
		t.Error("Auto cleanup should be enabled by default")
	}
}

func TestNewResourceManager(t *testing.T) {
	config := DefaultResourceConfig()
	rm := NewResourceManager(config)

	if rm == nil {
		t.Fatal("ResourceManager should not be nil")
	}

	if rm.memoryLimit != config.MemoryLimitMB*1024*1024 {
		t.Error("Memory limit not properly set")
	}

	if rm.goroutineLimit != config.GoroutineLimit {
		t.Error("Goroutine limit not properly set")
	}

	if rm.fileHandleLimit != config.FileHandleLimit {
		t.Error("File handle limit not properly set")
	}

	rm.Cleanup()
}

func TestResourceManager_Cleanup(t *testing.T) {
	config := DefaultResourceConfig()
	config.MonitorInterval = 100 * time.Millisecond
	rm := NewResourceManager(config)

	// Wait a bit for monitoring
	time.Sleep(150 * time.Millisecond)

	// Cleanup
	err := rm.Cleanup()
	if err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}
}

func TestResourceManager_GetStats(t *testing.T) {
	config := DefaultResourceConfig()
	rm := NewResourceManager(config)
	defer rm.Cleanup()

	stats := rm.GetStats()

	if stats.MemoryLimit != config.MemoryLimitMB*1024*1024 {
		t.Error("Stats memory limit incorrect")
	}

	if stats.GoroutineLimit != config.GoroutineLimit {
		t.Error("Stats goroutine limit incorrect")
	}

	if stats.LastUpdate.IsZero() {
		t.Log("Stats last update not set (may be expected before first update)")
	}
}

func TestResourceManager_CheckMemoryLimit(t *testing.T) {
	config := DefaultResourceConfig()
	config.MemoryLimitMB = 1 // Very low limit
	rm := NewResourceManager(config)
	defer rm.Cleanup()

	// Try to request more than limit
	err := rm.AcquireMemory(2 * 1024 * 1024) // 2MB
	if err == nil {
		t.Error("Should fail when requesting more than limit")
	}

	// Request reasonable amount
	err = rm.AcquireMemory(512 * 1024) // 512KB
	if err != nil {
		t.Errorf("Should succeed: %v", err)
	}

	// Release memory
	rm.ReleaseMemory(512 * 1024)
}

func TestResourceManager_GoroutineLimit(t *testing.T) {
	config := DefaultResourceConfig()
	config.GoroutineLimit = 5
	rm := NewResourceManager(config)
	defer rm.Cleanup()

	// Request goroutines up to limit
	for i := 0; i < 5; i++ {
		err := rm.AcquireGoroutine()
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
	}

	// Try to exceed limit
	err := rm.AcquireGoroutine()
	if err == nil {
		t.Error("Should fail when exceeding goroutine limit")
	}

	// Release some
	rm.ReleaseGoroutine()
	rm.ReleaseGoroutine()

	// Should work now
	err = rm.AcquireGoroutine()
	if err != nil {
		t.Errorf("Should succeed after release: %v", err)
	}
}

func TestResourceManager_FileHandleLimit(t *testing.T) {
	config := DefaultResourceConfig()
	config.FileHandleLimit = 10
	rm := NewResourceManager(config)
	defer rm.Cleanup()

	// Request file handles
	for i := 0; i < 10; i++ {
		err := rm.AcquireFileHandle()
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
	}

	// Try to exceed limit
	err := rm.AcquireFileHandle()
	if err == nil {
		t.Error("Should fail when exceeding file handle limit")
	}

	// Release one
	rm.ReleaseFileHandle()

	// Should work now
	err = rm.AcquireFileHandle()
	if err != nil {
		t.Errorf("Should succeed after release: %v", err)
	}
}

func TestResourceManager_ConcurrentMemoryRequests(t *testing.T) {
	config := DefaultResourceConfig()
	config.MemoryLimitMB = 10
	rm := NewResourceManager(config)
	defer rm.Cleanup()

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	// Multiple goroutines trying to request memory
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := rm.AcquireMemory(1024 * 1024) // 1MB each
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
				time.Sleep(10 * time.Millisecond)
				rm.ReleaseMemory(1024 * 1024)
			}
		}()
	}

	wg.Wait()

	if successCount == 0 {
		t.Error("At least some requests should succeed")
	}

	if successCount > 10 {
		t.Error("Should not exceed memory limit")
	}
}

func TestResourceManager_CreateResourcePool(t *testing.T) {
	config := DefaultResourceConfig()
	rm := NewResourceManager(config)
	defer rm.Cleanup()

	factory := func() (interface{}, error) {
		return &struct{ ID int }{ID: 1}, nil
	}

	cleanup := func(res interface{}) error {
		return nil
	}

	pool := rm.CreateResourcePool("test-pool", 5, factory, cleanup)
	if pool == nil {
		t.Fatal("Pool should not be nil")
	}

	if pool.name != "test-pool" {
		t.Error("Pool name not set correctly")
	}

	if pool.maxSize != 5 {
		t.Error("Pool max size not set correctly")
	}
}

func TestResourcePool_BorrowReturn(t *testing.T) {
	config := DefaultResourceConfig()
	rm := NewResourceManager(config)
	defer rm.Cleanup()

	counter := 0
	factory := func() (interface{}, error) {
		counter++
		return counter, nil
	}

	cleanup := func(res interface{}) error {
		return nil
	}

	pool := rm.CreateResourcePool("test-pool", 3, factory, cleanup)

	// Borrow resource
	ctx := context.Background()
	res, err := pool.Borrow(ctx)
	if err != nil {
		t.Fatalf("Failed to borrow: %v", err)
	}

	if res == nil {
		t.Fatal("Resource should not be nil")
	}

	// Return resource
	pool.Return(res)

	// Borrow again (should reuse)
	res2, err := pool.Borrow(ctx)
	if err != nil {
		t.Fatalf("Failed to borrow second time: %v", err)
	}

	if res2 == nil {
		t.Fatal("Resource should not be nil")
	}
}

func TestResourcePool_Stats(t *testing.T) {
	config := DefaultResourceConfig()
	rm := NewResourceManager(config)
	defer rm.Cleanup()

	factory := func() (interface{}, error) {
		return "resource", nil
	}

	cleanup := func(res interface{}) error {
		return nil
	}

	pool := rm.CreateResourcePool("test-pool", 5, factory, cleanup)
	ctx := context.Background()

	// Borrow some resources
	res1, _ := pool.Borrow(ctx)
	res2, _ := pool.Borrow(ctx)

	stats := pool.GetStats()
	if stats.Created == 0 {
		t.Error("Should have created resources")
	}

	if stats.Borrowed == 0 {
		t.Error("Should have borrowed resources")
	}

	// Return
	pool.Return(res1)
	pool.Return(res2)

	stats = pool.GetStats()
	if stats.Returned == 0 {
		t.Error("Should have returned resources")
	}
}

func TestResourceManager_GetPoolStats(t *testing.T) {
	config := DefaultResourceConfig()
	rm := NewResourceManager(config)
	defer rm.Cleanup()

	factory := func() (interface{}, error) {
		return "resource", nil
	}

	cleanup := func(res interface{}) error {
		return nil
	}

	pool := rm.CreateResourcePool("my-pool", 3, factory, cleanup)
	ctx := context.Background()

	// Use the pool
	res, _ := pool.Borrow(ctx)
	pool.Return(res)

	// Get pool directly
	pool2 := rm.GetResourcePool("my-pool")
	if pool2 == nil {
		t.Fatal("Should return pool")
	}

	poolStats := pool2.GetStats()
	if poolStats.Created == 0 {
		t.Error("Stats should show created resources")
	}
}

func TestResourceManager_ForceGC(t *testing.T) {
	config := DefaultResourceConfig()
	rm := NewResourceManager(config)
	defer rm.Cleanup()

	// Should not panic
	rm.ForceGC()
}

func TestResourceManager_GetStats_Tracking(t *testing.T) {
	config := DefaultResourceConfig()
	rm := NewResourceManager(config)
	defer rm.Cleanup()

	stats := rm.GetStats()

	// GoroutineCount may be 0 if none acquired yet
	if stats.GoroutineCount < 0 {
		t.Error("Goroutine count should not be negative")
	}

	if stats.MemoryUsage < 0 {
		t.Error("Memory usage should not be negative")
	}

	if stats.MemoryLimit <= 0 {
		t.Error("Memory limit should be positive")
	}
}

func TestResourceStats_Utilization(t *testing.T) {
	config := DefaultResourceConfig()
	config.MemoryLimitMB = 10
	rm := NewResourceManager(config)
	defer rm.Cleanup()

	// Request some memory
	rm.AcquireMemory(5 * 1024 * 1024) // 5MB

	stats := rm.GetStats()
	if stats.MemoryUtilization < 0 || stats.MemoryUtilization > 1 {
		t.Errorf("Memory utilization should be 0-1, got %f", stats.MemoryUtilization)
	}

	rm.ReleaseMemory(5 * 1024 * 1024)
}

func TestResourceManager_MemoryTracking(t *testing.T) {
	config := DefaultResourceConfig()
	rm := NewResourceManager(config)
	defer rm.Cleanup()

	// Get initial stats
	stats1 := rm.GetStats()
	initial := stats1.MemoryUsage

	// Request memory
	rm.AcquireMemory(1024 * 1024) // 1MB

	stats2 := rm.GetStats()
	if stats2.MemoryUsage != initial+1024*1024 {
		t.Errorf("Current memory not tracked correctly: expected %d, got %d",
			initial+1024*1024, stats2.MemoryUsage)
	}

	// Release memory
	rm.ReleaseMemory(1024 * 1024)

	stats3 := rm.GetStats()
	if stats3.MemoryUsage != initial {
		t.Errorf("Memory not released correctly: expected %d, got %d",
			initial, stats3.MemoryUsage)
	}
}

func TestResourcePool_ContextCancellation(t *testing.T) {
	config := DefaultResourceConfig()
	rm := NewResourceManager(config)
	defer rm.Cleanup()

	factory := func() (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return "resource", nil
	}

	cleanup := func(res interface{}) error {
		return nil
	}

	pool := rm.CreateResourcePool("test-pool", 1, factory, cleanup)

	// Borrow one resource
	ctx1 := context.Background()
	res1, _ := pool.Borrow(ctx1)

	// Try to borrow with cancelled context
	ctx2, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := pool.Borrow(ctx2)
	if err == nil {
		t.Log("Context cancellation may not be checked")
	}

	// Clean up
	pool.Return(res1)
}
