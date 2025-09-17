package resource

import (
	"context"
	"runtime"
	"sync"
	"time"
)

// SystemMonitor system resource monitor
type SystemMonitor struct {
	mu         sync.RWMutex
	running    bool
	stopChan   chan struct{}
	interval   time.Duration
	stats      SystemStats
	callbacks  []MonitorCallback
	thresholds MonitorThresholds
}

// SystemStats system statistics information
type SystemStats struct {
	// Memory statistics
	MemoryAlloc      uint64 `json:"memory_alloc"`       // Currently allocated memory
	MemoryTotalAlloc uint64 `json:"memory_total_alloc"` // Total allocated memory
	MemoryHeapAlloc  uint64 `json:"memory_heap_alloc"`  // Heap memory allocation
	MemoryHeapSys    uint64 `json:"memory_heap_sys"`    // Heap system memory
	MemoryStack      uint64 `json:"memory_stack"`       // Stack memory

	// Garbage collection statistics
	GCRuns       uint32 `json:"gc_runs"`        // GC run count
	GCPauseTotal uint64 `json:"gc_pause_total"` // Total GC pause time
	GCPauseLast  uint64 `json:"gc_pause_last"`  // Last GC pause time

	// Goroutine statistics
	NumGoroutine int `json:"num_goroutine"` // Goroutine count
	NumCPU       int `json:"num_cpu"`       // CPU count

	// 時間戳
	Timestamp time.Time `json:"timestamp"`

	// Historical statistics
	PeakMemory     uint64  `json:"peak_memory"`
	PeakGoroutines int     `json:"peak_goroutines"`
	AvgMemory      float64 `json:"avg_memory"`
	AvgGoroutines  float64 `json:"avg_goroutines"`
}

// MonitorCallback monitoring callback function
type MonitorCallback func(stats SystemStats)

// MonitorThresholds monitoring thresholds
type MonitorThresholds struct {
	MemoryMB       uint64 // Memory threshold (MB)
	GoroutineCount int    // Goroutine count threshold
	GCPauseMS      uint64 // GC pause time threshold (milliseconds)
}

// AlertLevel 警告級別
type AlertLevel int

const (
	AlertLevelInfo AlertLevel = iota
	AlertLevelWarning
	AlertLevelCritical
)

// Alert 警告信息
type Alert struct {
	Level     AlertLevel  `json:"level"`
	Message   string      `json:"message"`
	Timestamp time.Time   `json:"timestamp"`
	Value     interface{} `json:"value"`
	Threshold interface{} `json:"threshold"`
}

// NewSystemMonitor 創建系統監控器
func NewSystemMonitor(interval time.Duration) *SystemMonitor {
	return &SystemMonitor{
		interval:  interval,
		stopChan:  make(chan struct{}),
		callbacks: make([]MonitorCallback, 0),
		thresholds: MonitorThresholds{
			MemoryMB:       100, // 100MB
			GoroutineCount: 100, // 100 個協程
			GCPauseMS:      10,  // 10ms
		},
	}
}

// Start 開始監控
func (sm *SystemMonitor) Start(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.running {
		return nil
	}

	sm.running = true
	go sm.monitorLoop(ctx)

	return nil
}

// Stop 停止監控
func (sm *SystemMonitor) Stop() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !sm.running {
		return
	}

	sm.running = false
	close(sm.stopChan)
}

// AddCallback 添加監控回調
func (sm *SystemMonitor) AddCallback(callback MonitorCallback) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.callbacks = append(sm.callbacks, callback)
}

// SetThresholds 設置監控閾值
func (sm *SystemMonitor) SetThresholds(thresholds MonitorThresholds) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.thresholds = thresholds
}

// GetStats 獲取當前統計信息
func (sm *SystemMonitor) GetStats() SystemStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.stats
}

// TriggerGC 觸發垃圾回收
func (sm *SystemMonitor) TriggerGC() {
	runtime.GC()
}

// GetMemoryUsageMB 獲取內存使用量（MB）
func (sm *SystemMonitor) GetMemoryUsageMB() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / 1024 / 1024
}

// CheckAlerts 檢查警告
func (sm *SystemMonitor) CheckAlerts() []Alert {
	sm.mu.RLock()
	stats := sm.stats
	thresholds := sm.thresholds
	sm.mu.RUnlock()

	var alerts []Alert

	// 檢查內存使用
	memoryMB := float64(stats.MemoryAlloc) / 1024 / 1024
	if memoryMB > float64(thresholds.MemoryMB) {
		level := AlertLevelWarning
		if memoryMB > float64(thresholds.MemoryMB)*1.5 {
			level = AlertLevelCritical
		}

		alerts = append(alerts, Alert{
			Level:     level,
			Message:   "Memory usage exceeded threshold",
			Timestamp: time.Now(),
			Value:     memoryMB,
			Threshold: thresholds.MemoryMB,
		})
	}

	// 檢查協程數量
	if stats.NumGoroutine > thresholds.GoroutineCount {
		level := AlertLevelWarning
		if stats.NumGoroutine > thresholds.GoroutineCount*2 {
			level = AlertLevelCritical
		}

		alerts = append(alerts, Alert{
			Level:     level,
			Message:   "Goroutine count exceeded threshold",
			Timestamp: time.Now(),
			Value:     stats.NumGoroutine,
			Threshold: thresholds.GoroutineCount,
		})
	}

	// 檢查 GC 暫停時間
	gcPauseMS := stats.GCPauseLast / 1000000 // 納秒轉毫秒
	if gcPauseMS > thresholds.GCPauseMS {
		level := AlertLevelWarning
		if gcPauseMS > thresholds.GCPauseMS*2 {
			level = AlertLevelCritical
		}

		alerts = append(alerts, Alert{
			Level:     level,
			Message:   "GC pause time exceeded threshold",
			Timestamp: time.Now(),
			Value:     gcPauseMS,
			Threshold: thresholds.GCPauseMS,
		})
	}

	return alerts
}

// 內部方法

func (sm *SystemMonitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(sm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.collectStats()
			sm.notifyCallbacks()
		case <-sm.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (sm *SystemMonitor) collectStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 收集統計信息
	newStats := SystemStats{
		MemoryAlloc:      m.Alloc,
		MemoryTotalAlloc: m.TotalAlloc,
		MemoryHeapAlloc:  m.HeapAlloc,
		MemoryHeapSys:    m.HeapSys,
		MemoryStack:      m.StackSys,
		GCRuns:           m.NumGC,
		GCPauseTotal:     m.PauseTotalNs,
		NumGoroutine:     runtime.NumGoroutine(),
		NumCPU:           runtime.NumCPU(),
		Timestamp:        time.Now(),
	}

	// 計算 GC 暫停時間
	if m.NumGC > 0 {
		newStats.GCPauseLast = m.PauseNs[(m.NumGC+255)%256]
	}

	// 更新峰值統計
	if newStats.MemoryAlloc > sm.stats.PeakMemory {
		sm.stats.PeakMemory = newStats.MemoryAlloc
	}
	if newStats.NumGoroutine > sm.stats.PeakGoroutines {
		sm.stats.PeakGoroutines = newStats.NumGoroutine
	}

	// 計算平均值
	if sm.stats.AvgMemory == 0 {
		sm.stats.AvgMemory = float64(newStats.MemoryAlloc)
	} else {
		sm.stats.AvgMemory = (sm.stats.AvgMemory + float64(newStats.MemoryAlloc)) / 2
	}

	if sm.stats.AvgGoroutines == 0 {
		sm.stats.AvgGoroutines = float64(newStats.NumGoroutine)
	} else {
		sm.stats.AvgGoroutines = (sm.stats.AvgGoroutines + float64(newStats.NumGoroutine)) / 2
	}

	// 保留峰值和平均值
	newStats.PeakMemory = sm.stats.PeakMemory
	newStats.PeakGoroutines = sm.stats.PeakGoroutines
	newStats.AvgMemory = sm.stats.AvgMemory
	newStats.AvgGoroutines = sm.stats.AvgGoroutines

	sm.stats = newStats
}

func (sm *SystemMonitor) notifyCallbacks() {
	sm.mu.RLock()
	stats := sm.stats
	callbacks := make([]MonitorCallback, len(sm.callbacks))
	copy(callbacks, sm.callbacks)
	sm.mu.RUnlock()

	// 異步調用回調函數
	go func() {
		for _, callback := range callbacks {
			if callback != nil {
				callback(stats)
			}
		}
	}()
}

// MemoryTracker 內存追蹤器
type MemoryTracker struct {
	mu          sync.RWMutex
	allocations map[string]int64
	totalAlloc  int64
}

// NewMemoryTracker 創建內存追蹤器
func NewMemoryTracker() *MemoryTracker {
	return &MemoryTracker{
		allocations: make(map[string]int64),
	}
}

// Track 追蹤內存分配
func (mt *MemoryTracker) Track(label string, size int64) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	mt.allocations[label] += size
	mt.totalAlloc += size
}

// Untrack 取消追蹤內存
func (mt *MemoryTracker) Untrack(label string, size int64) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	mt.allocations[label] -= size
	mt.totalAlloc -= size

	if mt.allocations[label] <= 0 {
		delete(mt.allocations, label)
	}
}

// GetAllocations 獲取分配情況
func (mt *MemoryTracker) GetAllocations() map[string]int64 {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	result := make(map[string]int64)
	for k, v := range mt.allocations {
		result[k] = v
	}
	return result
}

// GetTotalAllocation 獲取總分配量
func (mt *MemoryTracker) GetTotalAllocation() int64 {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	return mt.totalAlloc
}
