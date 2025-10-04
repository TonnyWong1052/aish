package resource

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// ResourceManager resource manager
type ResourceManager struct {
	mu              sync.RWMutex
	memoryLimit     int64 // ��存限制（字節）
	currentMemory   int64 // 當前內存使用
	goroutineLimit  int   // 協程限制
	currentRoutines int64 // 當前協程數
	fileHandleLimit int   // 文件句柄限制
	currentHandles  int64 // 當前文件句柄數

	// 資源池
	resourcePools map[string]*ResourcePool

	// 監控
	stats      ResourceStats
	monitoring bool
	stopChan   chan struct{}
}

// ResourceStats 資源統計
type ResourceStats struct {
	MemoryUsage       int64     `json:"memory_usage"`
	MemoryLimit       int64     `json:"memory_limit"`
	MemoryUtilization float64   `json:"memory_utilization"`
	GoroutineCount    int64     `json:"goroutine_count"`
	GoroutineLimit    int       `json:"goroutine_limit"`
	FileHandleCount   int64     `json:"file_handle_count"`
	FileHandleLimit   int       `json:"file_handle_limit"`
	LastUpdate        time.Time `json:"last_update"`

	// 歷史統計
	PeakMemory       int64   `json:"peak_memory"`
	PeakGoroutines   int64   `json:"peak_goroutines"`
	PeakFileHandles  int64   `json:"peak_file_handles"`
	AvgMemoryUsage   float64 `json:"avg_memory_usage"`
	ResourceWarnings int64   `json:"resource_warnings"`
}

// ResourceConfig 資源配置
type ResourceConfig struct {
	MemoryLimitMB    int64 // 內存限制（MB）
	GoroutineLimit   int   // 協程限制
	FileHandleLimit  int   // 文件句柄限制
	MonitorInterval  time.Duration
	EnableMonitoring bool
	AutoCleanup      bool
}

// ResourcePool 資源池
type ResourcePool struct {
	name      string
	resources chan interface{}
	factory   func() (interface{}, error)
	cleanup   func(interface{}) error
	maxSize   int
	current   int64
	stats     PoolStats
	mu        sync.RWMutex
}

// PoolStats 資源池統計
type PoolStats struct {
	Created   int64
	Borrowed  int64
	Returned  int64
	Destroyed int64
	InUse     int64
}

// DefaultResourceConfig 默認資源配置
func DefaultResourceConfig() ResourceConfig {
	return ResourceConfig{
		MemoryLimitMB:    512,  // 512MB
		GoroutineLimit:   1000, // 1000 個協程
		FileHandleLimit:  100,  // 100 個文件句柄
		MonitorInterval:  5 * time.Second,
		EnableMonitoring: true,
		AutoCleanup:      true,
	}
}

// NewResourceManager 創建新的資源管理器
func NewResourceManager(config ResourceConfig) *ResourceManager {
	rm := &ResourceManager{
		memoryLimit:     config.MemoryLimitMB * 1024 * 1024,
		goroutineLimit:  config.GoroutineLimit,
		fileHandleLimit: config.FileHandleLimit,
		resourcePools:   make(map[string]*ResourcePool),
		monitoring:      config.EnableMonitoring,
		stopChan:        make(chan struct{}),
	}

	if config.EnableMonitoring {
		go rm.startMonitoring(config.MonitorInterval)
	}

	return rm
}

// AcquireMemory 申請內存資源
func (rm *ResourceManager) AcquireMemory(size int64) error {
	if atomic.LoadInt64(&rm.currentMemory)+size > rm.memoryLimit {
		return fmt.Errorf("memory limit exceeded: requested %d, available %d",
			size, rm.memoryLimit-atomic.LoadInt64(&rm.currentMemory))
	}

	atomic.AddInt64(&rm.currentMemory, size)
	return nil
}

// ReleaseMemory 釋放內存資源
func (rm *ResourceManager) ReleaseMemory(size int64) {
	atomic.AddInt64(&rm.currentMemory, -size)
}

// AcquireGoroutine 申請協程資源
func (rm *ResourceManager) AcquireGoroutine() error {
	current := atomic.LoadInt64(&rm.currentRoutines)
	if int(current) >= rm.goroutineLimit {
		return fmt.Errorf("goroutine limit exceeded: limit %d", rm.goroutineLimit)
	}

	atomic.AddInt64(&rm.currentRoutines, 1)
	return nil
}

// ReleaseGoroutine 釋放協程資源
func (rm *ResourceManager) ReleaseGoroutine() {
	atomic.AddInt64(&rm.currentRoutines, -1)
}

// AcquireFileHandle 申請文件句柄資源
func (rm *ResourceManager) AcquireFileHandle() error {
	current := atomic.LoadInt64(&rm.currentHandles)
	if int(current) >= rm.fileHandleLimit {
		return fmt.Errorf("file handle limit exceeded: limit %d", rm.fileHandleLimit)
	}

	atomic.AddInt64(&rm.currentHandles, 1)
	return nil
}

// ReleaseFileHandle 釋放文件句柄資源
func (rm *ResourceManager) ReleaseFileHandle() {
	atomic.AddInt64(&rm.currentHandles, -1)
}

// CreateResourcePool 創建資源池
func (rm *ResourceManager) CreateResourcePool(name string, maxSize int,
	factory func() (interface{}, error), cleanup func(interface{}) error) *ResourcePool {

	rm.mu.Lock()
	defer rm.mu.Unlock()

	pool := &ResourcePool{
		name:      name,
		resources: make(chan interface{}, maxSize),
		factory:   factory,
		cleanup:   cleanup,
		maxSize:   maxSize,
	}

	rm.resourcePools[name] = pool
	return pool
}

// GetResourcePool 獲取資源池
func (rm *ResourceManager) GetResourcePool(name string) *ResourcePool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return rm.resourcePools[name]
}

// GetStats 獲取資源統計
func (rm *ResourceManager) GetStats() ResourceStats {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// 更新當前統計
	currentMem := atomic.LoadInt64(&rm.currentMemory)
	currentGoroutines := atomic.LoadInt64(&rm.currentRoutines)
	currentHandles := atomic.LoadInt64(&rm.currentHandles)

	stats := rm.stats
	stats.MemoryUsage = currentMem
	stats.MemoryLimit = rm.memoryLimit
	stats.MemoryUtilization = float64(currentMem) / float64(rm.memoryLimit)
	stats.GoroutineCount = currentGoroutines
	stats.GoroutineLimit = rm.goroutineLimit
	stats.FileHandleCount = currentHandles
	stats.FileHandleLimit = rm.fileHandleLimit
	stats.LastUpdate = time.Now()

	// 更新峰值統計
	if currentMem > stats.PeakMemory {
		stats.PeakMemory = currentMem
	}
	if currentGoroutines > stats.PeakGoroutines {
		stats.PeakGoroutines = currentGoroutines
	}
	if currentHandles > stats.PeakFileHandles {
		stats.PeakFileHandles = currentHandles
	}

	return stats
}

// Cleanup 清理資源
func (rm *ResourceManager) Cleanup() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	var lastErr error

	// 清理所有資源池
	for name, pool := range rm.resourcePools {
		if err := pool.Cleanup(); err != nil {
			lastErr = fmt.Errorf("failed to cleanup pool %s: %w", name, err)
		}
	}

	// 停止監控
	if rm.monitoring {
		close(rm.stopChan)
		rm.monitoring = false
	}

	return lastErr
}

// ForceGC 強制垃圾回收
func (rm *ResourceManager) ForceGC() {
	runtime.GC()
	runtime.GC() // 運行兩次以確保完全清理
}

// 內部方法

func (rm *ResourceManager) startMonitoring(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rm.updateStats()
		case <-rm.stopChan:
			return
		}
	}
}

func (rm *ResourceManager) updateStats() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	currentMem := atomic.LoadInt64(&rm.currentMemory)

	// 更新平均內存使用
	if rm.stats.AvgMemoryUsage == 0 {
		rm.stats.AvgMemoryUsage = float64(currentMem)
	} else {
		rm.stats.AvgMemoryUsage = (rm.stats.AvgMemoryUsage + float64(currentMem)) / 2
	}

	// 檢查資源警告
	memUtilization := float64(currentMem) / float64(rm.memoryLimit)
	goroutineUtilization := float64(atomic.LoadInt64(&rm.currentRoutines)) / float64(rm.goroutineLimit)
	handleUtilization := float64(atomic.LoadInt64(&rm.currentHandles)) / float64(rm.fileHandleLimit)

	if memUtilization > 0.8 || goroutineUtilization > 0.8 || handleUtilization > 0.8 {
		atomic.AddInt64(&rm.stats.ResourceWarnings, 1)
	}
}

// ResourcePool 方法

// Borrow 從資源池借用資源
func (rp *ResourcePool) Borrow(ctx context.Context) (interface{}, error) {
	select {
	case resource := <-rp.resources:
		atomic.AddInt64(&rp.stats.Borrowed, 1)
		atomic.AddInt64(&rp.stats.InUse, 1)
		return resource, nil
	default:
		// 池中沒有可用資源，創建新的
		if atomic.LoadInt64(&rp.current) < int64(rp.maxSize) {
			resource, err := rp.factory()
			if err != nil {
				return nil, err
			}

			atomic.AddInt64(&rp.current, 1)
			atomic.AddInt64(&rp.stats.Created, 1)
			atomic.AddInt64(&rp.stats.Borrowed, 1)
			atomic.AddInt64(&rp.stats.InUse, 1)

			return resource, nil
		}

		// 池已滿，等待可用資源
		select {
		case resource := <-rp.resources:
			atomic.AddInt64(&rp.stats.Borrowed, 1)
			atomic.AddInt64(&rp.stats.InUse, 1)
			return resource, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// Return 歸還資源到池
func (rp *ResourcePool) Return(resource interface{}) {
	select {
	case rp.resources <- resource:
		atomic.AddInt64(&rp.stats.Returned, 1)
		atomic.AddInt64(&rp.stats.InUse, -1)
	default:
		// 池已滿，銷毀資源
		if rp.cleanup != nil {
			_ = rp.cleanup(resource)
		}
		atomic.AddInt64(&rp.current, -1)
		atomic.AddInt64(&rp.stats.Destroyed, 1)
		atomic.AddInt64(&rp.stats.InUse, -1)
	}
}

// GetStats 獲取池統計
func (rp *ResourcePool) GetStats() PoolStats {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	return rp.stats
}

// Cleanup 清理資源池
func (rp *ResourcePool) Cleanup() error {
	close(rp.resources)

	var lastErr error

	// 清理所有剩餘資源
	for resource := range rp.resources {
		if rp.cleanup != nil {
			if err := rp.cleanup(resource); err != nil {
				lastErr = err
			}
		}
		atomic.AddInt64(&rp.stats.Destroyed, 1)
	}

	atomic.StoreInt64(&rp.current, 0)
	return lastErr
}

// ResourceGuard 資源守衛，自動管理資源生命週期
type ResourceGuard struct {
	rm           *ResourceManager
	memSize      int64
	hasGoroutine bool
	hasHandle    bool
}

// NewResourceGuard 創建資源守衛
func (rm *ResourceManager) NewResourceGuard() *ResourceGuard {
	return &ResourceGuard{rm: rm}
}

// AcquireMemory 申請內存資源
func (rg *ResourceGuard) AcquireMemory(size int64) error {
	if err := rg.rm.AcquireMemory(size); err != nil {
		return err
	}
	rg.memSize += size
	return nil
}

// AcquireGoroutine 申請協程資源
func (rg *ResourceGuard) AcquireGoroutine() error {
	if err := rg.rm.AcquireGoroutine(); err != nil {
		return err
	}
	rg.hasGoroutine = true
	return nil
}

// AcquireFileHandle 申請文件句柄資源
func (rg *ResourceGuard) AcquireFileHandle() error {
	if err := rg.rm.AcquireFileHandle(); err != nil {
		return err
	}
	rg.hasHandle = true
	return nil
}

// Release 釋放所有資源
func (rg *ResourceGuard) Release() {
	if rg.memSize > 0 {
		rg.rm.ReleaseMemory(rg.memSize)
		rg.memSize = 0
	}

	if rg.hasGoroutine {
		rg.rm.ReleaseGoroutine()
		rg.hasGoroutine = false
	}

	if rg.hasHandle {
		rg.rm.ReleaseFileHandle()
		rg.hasHandle = false
	}
}
