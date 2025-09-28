package monitoring

import (
	"context"
	"runtime"
	"sync"
	"time"
)

// SystemMonitor 系統監控器
type SystemMonitor struct {
	metrics    *SystemMetrics
	collectors []MetricsCollector
	config     *MonitorConfig
	running    bool
	ticker     *time.Ticker
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex
}

// MonitorConfig 監控配置
type MonitorConfig struct {
	CollectInterval  time.Duration   `json:"collect_interval"`
	MetricsRetention time.Duration   `json:"metrics_retention"`
	EnableCPUProfile bool            `json:"enable_cpu_profile"`
	EnableMemProfile bool            `json:"enable_mem_profile"`
	EnableGoroutines bool            `json:"enable_goroutines"`
	AlertThresholds  AlertThresholds `json:"alert_thresholds"`
}

// AlertThresholds 警報閾值
type AlertThresholds struct {
	CPUUsage       float64 `json:"cpu_usage"`
	MemoryUsage    float64 `json:"memory_usage"`
	GoroutineCount int     `json:"goroutine_count"`
	HeapSize       int64   `json:"heap_size"`
}

// SystemMetrics 系統指標
type SystemMetrics struct {
	Timestamp     time.Time              `json:"timestamp"`
	Runtime       RuntimeMetrics         `json:"runtime"`
	Memory        MemoryMetrics          `json:"memory"`
	Goroutines    GoroutineMetrics       `json:"goroutines"`
	GC            GCMetrics              `json:"gc"`
	Performance   PerformanceMetrics     `json:"performance"`
	CustomMetrics map[string]interface{} `json:"custom_metrics"`
	Alerts        []Alert                `json:"alerts"`
}

// RuntimeMetrics 運行時指標
type RuntimeMetrics struct {
	GoVersion    string        `json:"go_version"`
	GOOS         string        `json:"goos"`
	GOARCH       string        `json:"goarch"`
	NumCPU       int           `json:"num_cpu"`
	NumGoroutine int           `json:"num_goroutine"`
	Uptime       time.Duration `json:"uptime"`
}

// MemoryMetrics 內存指標
type MemoryMetrics struct {
	Alloc        uint64  `json:"alloc"`         // 已分配的堆內存
	TotalAlloc   uint64  `json:"total_alloc"`   // 累計分配的內存
	Sys          uint64  `json:"sys"`           // 系統內存
	HeapAlloc    uint64  `json:"heap_alloc"`    // 堆分配
	HeapSys      uint64  `json:"heap_sys"`      // 堆系統內存
	HeapIdle     uint64  `json:"heap_idle"`     // 空閒堆內存
	HeapInuse    uint64  `json:"heap_inuse"`    // 使用中的堆內存
	HeapReleased uint64  `json:"heap_released"` // 釋放給系統的內存
	StackInuse   uint64  `json:"stack_inuse"`   // 棧使用的內存
	StackSys     uint64  `json:"stack_sys"`     // 棧系統內存
	UsagePercent float64 `json:"usage_percent"` // 內存使用百分比
}

// GoroutineMetrics Goroutine 指標
type GoroutineMetrics struct {
	Total   int                        `json:"total"`
	Running int                        `json:"running"`
	Waiting int                        `json:"waiting"`
	Blocked int                        `json:"blocked"`
	Details map[string]GoroutineDetail `json:"details"`
}

// GoroutineDetail Goroutine 詳細信息
type GoroutineDetail struct {
	Count    int    `json:"count"`
	Function string `json:"function"`
	State    string `json:"state"`
}

// GCMetrics 垃圾回收指標
type GCMetrics struct {
	NumGC         uint32        `json:"num_gc"`
	PauseTotal    time.Duration `json:"pause_total"`
	PauseNs       []uint64      `json:"pause_ns"`
	LastGC        time.Time     `json:"last_gc"`
	NextGC        uint64        `json:"next_gc"`
	GCCPUFraction float64       `json:"gc_cpu_fraction"`
}

// PerformanceMetrics 性能指標
type PerformanceMetrics struct {
	RequestsPerSecond float64            `json:"requests_per_second"`
	AverageLatency    time.Duration      `json:"average_latency"`
	P95Latency        time.Duration      `json:"p95_latency"`
	P99Latency        time.Duration      `json:"p99_latency"`
	ErrorRate         float64            `json:"error_rate"`
	ThroughputMBps    float64            `json:"throughput_mbps"`
	CustomCounters    map[string]int64   `json:"custom_counters"`
	CustomGauges      map[string]float64 `json:"custom_gauges"`
}

// Alert 警報
type Alert struct {
	Type      string    `json:"type"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Timestamp time.Time `json:"timestamp"`
}

// MetricsCollector 指標收集器接口
type MetricsCollector interface {
	Collect() (map[string]interface{}, error)
	Name() string
}

// DefaultMonitorConfig 返回默認監控配置
func DefaultMonitorConfig() *MonitorConfig {
	return &MonitorConfig{
		CollectInterval:  10 * time.Second,
		MetricsRetention: time.Hour,
		EnableCPUProfile: true,
		EnableMemProfile: true,
		EnableGoroutines: true,
		AlertThresholds: AlertThresholds{
			CPUUsage:       80.0,
			MemoryUsage:    85.0,
			GoroutineCount: 1000,
			HeapSize:       1024 * 1024 * 1024, // 1GB
		},
	}
}

// NewSystemMonitor 創建系統監控器
func NewSystemMonitor(config *MonitorConfig) *SystemMonitor {
	if config == nil {
		config = DefaultMonitorConfig()
	}

	return &SystemMonitor{
		config:     config,
		collectors: make([]MetricsCollector, 0),
		metrics: &SystemMetrics{
			CustomMetrics: make(map[string]interface{}),
		},
	}
}

// Start 啟動監控
func (sm *SystemMonitor) Start(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.running {
		return nil
	}

	sm.ctx, sm.cancel = context.WithCancel(ctx)
	sm.ticker = time.NewTicker(sm.config.CollectInterval)
	sm.running = true

	go sm.collectLoop()

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
	if sm.cancel != nil {
		sm.cancel()
	}
	if sm.ticker != nil {
		sm.ticker.Stop()
	}
}

// collectLoop 收集循環
func (sm *SystemMonitor) collectLoop() {
	defer func() {
		sm.mu.Lock()
		sm.running = false
		sm.mu.Unlock()
	}()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-sm.ticker.C:
			sm.collectMetrics()
		}
	}
}

// collectMetrics 收集指標
func (sm *SystemMonitor) collectMetrics() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()

	// 收集運行時指標
	sm.metrics.Runtime = sm.collectRuntimeMetrics()

	// 收集內存指標
	sm.metrics.Memory = sm.collectMemoryMetrics()

	// 收集 Goroutine 指標
	if sm.config.EnableGoroutines {
		sm.metrics.Goroutines = sm.collectGoroutineMetrics()
	}

	// 收集 GC 指標
	sm.metrics.GC = sm.collectGCMetrics()

	// 收集自定義指標
	for _, collector := range sm.collectors {
		if metrics, err := collector.Collect(); err == nil {
			for k, v := range metrics {
				sm.metrics.CustomMetrics[collector.Name()+"."+k] = v
			}
		}
	}

	sm.metrics.Timestamp = now

	// 檢查警報
	sm.checkAlerts()
}

// collectRuntimeMetrics 收集運行時指標
func (sm *SystemMonitor) collectRuntimeMetrics() RuntimeMetrics {
	return RuntimeMetrics{
		GoVersion:    runtime.Version(),
		GOOS:         runtime.GOOS,
		GOARCH:       runtime.GOARCH,
		NumCPU:       runtime.NumCPU(),
		NumGoroutine: runtime.NumGoroutine(),
		Uptime:       time.Since(startTime),
	}
}

// collectMemoryMetrics 收集內存指標
func (sm *SystemMonitor) collectMemoryMetrics() MemoryMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 計算內存使用百分比
	usagePercent := float64(m.HeapInuse) / float64(m.HeapSys) * 100
	if m.HeapSys == 0 {
		usagePercent = 0
	}

	return MemoryMetrics{
		Alloc:        m.Alloc,
		TotalAlloc:   m.TotalAlloc,
		Sys:          m.Sys,
		HeapAlloc:    m.HeapAlloc,
		HeapSys:      m.HeapSys,
		HeapIdle:     m.HeapIdle,
		HeapInuse:    m.HeapInuse,
		HeapReleased: m.HeapReleased,
		StackInuse:   m.StackInuse,
		StackSys:     m.StackSys,
		UsagePercent: usagePercent,
	}
}

// collectGoroutineMetrics 收集 Goroutine 指標
func (sm *SystemMonitor) collectGoroutineMetrics() GoroutineMetrics {
	total := runtime.NumGoroutine()

	// 簡化實現，實際可以通過 runtime.Stack 獲取更詳細信息
	return GoroutineMetrics{
		Total:   total,
		Running: total / 4, // 估算
		Waiting: total / 2, // 估算
		Blocked: total / 4, // 估算
		Details: make(map[string]GoroutineDetail),
	}
}

// collectGCMetrics 收集垃圾回收指標
func (sm *SystemMonitor) collectGCMetrics() GCMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	lastGC := time.Time{}
	if m.LastGC != 0 {
		lastGC = time.Unix(0, int64(m.LastGC))
	}

	return GCMetrics{
		NumGC:         m.NumGC,
		PauseTotal:    time.Duration(m.PauseTotalNs),
		PauseNs:       m.PauseNs[:],
		LastGC:        lastGC,
		NextGC:        m.NextGC,
		GCCPUFraction: m.GCCPUFraction,
	}
}

// checkAlerts 檢查警報
func (sm *SystemMonitor) checkAlerts() {
	sm.metrics.Alerts = sm.metrics.Alerts[:0] // 清空警報

	// 檢查內存使用率
	if sm.metrics.Memory.UsagePercent > sm.config.AlertThresholds.MemoryUsage {
		sm.metrics.Alerts = append(sm.metrics.Alerts, Alert{
			Type:      "memory_usage",
			Level:     "warning",
			Message:   "High memory usage detected",
			Value:     sm.metrics.Memory.UsagePercent,
			Threshold: sm.config.AlertThresholds.MemoryUsage,
			Timestamp: time.Now(),
		})
	}

	// 檢查 Goroutine 數量
	if sm.metrics.Runtime.NumGoroutine > sm.config.AlertThresholds.GoroutineCount {
		sm.metrics.Alerts = append(sm.metrics.Alerts, Alert{
			Type:      "goroutine_count",
			Level:     "warning",
			Message:   "High goroutine count detected",
			Value:     float64(sm.metrics.Runtime.NumGoroutine),
			Threshold: float64(sm.config.AlertThresholds.GoroutineCount),
			Timestamp: time.Now(),
		})
	}

	// 檢查堆大小
	if sm.metrics.Memory.HeapAlloc > uint64(sm.config.AlertThresholds.HeapSize) {
		sm.metrics.Alerts = append(sm.metrics.Alerts, Alert{
			Type:      "heap_size",
			Level:     "critical",
			Message:   "Heap size exceeded threshold",
			Value:     float64(sm.metrics.Memory.HeapAlloc),
			Threshold: float64(sm.config.AlertThresholds.HeapSize),
			Timestamp: time.Now(),
		})
	}
}

// AddCollector 添加自定義指標收集器
func (sm *SystemMonitor) AddCollector(collector MetricsCollector) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.collectors = append(sm.collectors, collector)
}

// RemoveCollector 移除指標收集器
func (sm *SystemMonitor) RemoveCollector(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for i, collector := range sm.collectors {
		if collector.Name() == name {
			sm.collectors = append(sm.collectors[:i], sm.collectors[i+1:]...)
			break
		}
	}
}

// GetMetrics 獲取當前指標
func (sm *SystemMonitor) GetMetrics() *SystemMetrics {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 返回指標的副本
	metrics := *sm.metrics

	// 深拷貝 CustomMetrics
	metrics.CustomMetrics = make(map[string]interface{})
	for k, v := range sm.metrics.CustomMetrics {
		metrics.CustomMetrics[k] = v
	}

	// 深拷貝 Alerts
	metrics.Alerts = make([]Alert, len(sm.metrics.Alerts))
	copy(metrics.Alerts, sm.metrics.Alerts)

	return &metrics
}

// IsRunning 檢查監控是否正在運行
func (sm *SystemMonitor) IsRunning() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.running
}

// UpdateConfig 更新監控配置
func (sm *SystemMonitor) UpdateConfig(config *MonitorConfig) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.config = config

	// 如果正在運行，重新啟動 ticker
	if sm.running && sm.ticker != nil {
		sm.ticker.Stop()
		sm.ticker = time.NewTicker(config.CollectInterval)
	}
}

// ForceGC 強制垃圾回收
func (sm *SystemMonitor) ForceGC() {
	runtime.GC()
}

// GetGoroutineProfile 獲取 Goroutine 配置文件
func (sm *SystemMonitor) GetGoroutineProfile() []byte {
	if !sm.config.EnableGoroutines {
		return nil
	}

	// 簡化實現，實際應該使用 pprof
	buf := make([]byte, 1<<20) // 1MB
	n := runtime.Stack(buf, true)
	return buf[:n]
}

// 啟動時間
var startTime = time.Now()
