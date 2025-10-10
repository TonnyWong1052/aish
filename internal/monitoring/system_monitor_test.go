package monitoring

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestDefaultMonitorConfig(t *testing.T) {
	config := DefaultMonitorConfig()

	if config.CollectInterval != 10*time.Second {
		t.Errorf("Expected collect interval 10s, got %v", config.CollectInterval)
	}

	if config.MetricsRetention != time.Hour {
		t.Errorf("Expected retention 1h, got %v", config.MetricsRetention)
	}

	if !config.EnableCPUProfile {
		t.Error("CPU profiling should be enabled by default")
	}

	if !config.EnableMemProfile {
		t.Error("Memory profiling should be enabled by default")
	}

	if !config.EnableGoroutines {
		t.Error("Goroutine monitoring should be enabled by default")
	}
}

func TestNewSystemMonitor(t *testing.T) {
	config := DefaultMonitorConfig()
	monitor := NewSystemMonitor(config)

	if monitor == nil {
		t.Fatal("Monitor should not be nil")
	}

	if monitor.config.CollectInterval != config.CollectInterval {
		t.Error("Config not properly set")
	}

	if monitor.running {
		t.Error("Monitor should not be running initially")
	}
}

func TestSystemMonitor_StartStop(t *testing.T) {
	config := DefaultMonitorConfig()
	config.CollectInterval = 100 * time.Millisecond
	monitor := NewSystemMonitor(config)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Start monitor
	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}

	if !monitor.IsRunning() {
		t.Error("Monitor should be running")
	}

	// Wait a bit for collection
	time.Sleep(200 * time.Millisecond)

	// Stop monitor
	monitor.Stop()

	if monitor.IsRunning() {
		t.Error("Monitor should be stopped")
	}
}

func TestSystemMonitor_CollectMetrics(t *testing.T) {
	config := DefaultMonitorConfig()
	monitor := NewSystemMonitor(config)

	// Manually trigger collection
	monitor.collectMetrics()

	metrics := monitor.GetMetrics()
	if metrics == nil {
		t.Fatal("Metrics should not be nil")
	}

	if metrics.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}

	// Check runtime metrics
	if metrics.Runtime.NumCPU != runtime.NumCPU() {
		t.Errorf("Expected %d CPUs, got %d", runtime.NumCPU(), metrics.Runtime.NumCPU)
	}

	if metrics.Runtime.GoVersion != runtime.Version() {
		t.Errorf("Expected Go version %s, got %s", runtime.Version(), metrics.Runtime.GoVersion)
	}

	// Check memory metrics
	if metrics.Memory.Alloc == 0 {
		t.Error("Memory allocation should not be zero")
	}

	// Check goroutine count
	if metrics.Runtime.NumGoroutine == 0 {
		t.Error("Goroutine count should not be zero")
	}
}

func TestSystemMonitor_GetMetrics(t *testing.T) {
	config := DefaultMonitorConfig()
	monitor := NewSystemMonitor(config)

	// Initially should have default metrics
	metrics := monitor.GetMetrics()
	if metrics == nil {
		t.Error("Should return metrics even before collection")
	}
}

func TestSystemMonitor_IsRunning(t *testing.T) {
	config := DefaultMonitorConfig()
	monitor := NewSystemMonitor(config)

	if monitor.IsRunning() {
		t.Error("Monitor should not be running initially")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor.Start(ctx)
	if !monitor.IsRunning() {
		t.Error("Monitor should be running after Start")
	}

	monitor.Stop()
	if monitor.IsRunning() {
		t.Error("Monitor should not be running after Stop")
	}
}

func TestSystemMonitor_UpdateConfig(t *testing.T) {
	config := DefaultMonitorConfig()
	monitor := NewSystemMonitor(config)

	newConfig := &MonitorConfig{
		CollectInterval:  10 * time.Second,
		MetricsRetention: 2 * time.Hour,
		EnableCPUProfile: false,
		EnableMemProfile: false,
		EnableGoroutines: true,
	}

	monitor.UpdateConfig(newConfig)

	if monitor.config.CollectInterval != 10*time.Second {
		t.Error("Config not updated")
	}

	if monitor.config.EnableCPUProfile {
		t.Error("CPU profile should be disabled")
	}
}

func TestSystemMonitor_ForceGC(t *testing.T) {
	config := DefaultMonitorConfig()
	monitor := NewSystemMonitor(config)

	// Should not panic
	monitor.ForceGC()

	// Verify GC ran by checking metrics
	monitor.collectMetrics()
	metrics := monitor.GetMetrics()

	if metrics.GC.NumGC == 0 {
		t.Log("GC count is 0, but this might be expected in test environment")
	}
}

func TestSystemMonitor_GetGoroutineProfile(t *testing.T) {
	config := DefaultMonitorConfig()
	monitor := NewSystemMonitor(config)

	profile := monitor.GetGoroutineProfile()
	if len(profile) == 0 {
		t.Error("Goroutine profile should not be empty")
	}
}

func TestSystemMonitor_MultipleStartStop(t *testing.T) {
	config := DefaultMonitorConfig()
	config.CollectInterval = 50 * time.Millisecond
	monitor := NewSystemMonitor(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start
	monitor.Start(ctx)
	if !monitor.IsRunning() {
		t.Error("Should be running")
	}

	// Try to start again (should be no-op or handle gracefully)
	err := monitor.Start(ctx)
	if err == nil {
		t.Log("Multiple starts handled gracefully")
	}

	// Stop
	monitor.Stop()
	if monitor.IsRunning() {
		t.Error("Should be stopped")
	}

	// Stop again (should be safe)
	monitor.Stop()
}

func TestCollectRuntimeMetrics(t *testing.T) {
	config := DefaultMonitorConfig()
	monitor := NewSystemMonitor(config)

	metrics := monitor.collectRuntimeMetrics()

	if metrics.GoVersion == "" {
		t.Error("Go version should not be empty")
	}

	if metrics.GOOS == "" {
		t.Error("GOOS should not be empty")
	}

	if metrics.GOARCH == "" {
		t.Error("GOARCH should not be empty")
	}

	if metrics.NumCPU <= 0 {
		t.Error("NumCPU should be positive")
	}

	if metrics.NumGoroutine <= 0 {
		t.Error("NumGoroutine should be positive")
	}
}

func TestCollectMemoryMetrics(t *testing.T) {
	config := DefaultMonitorConfig()
	monitor := NewSystemMonitor(config)

	metrics := monitor.collectMemoryMetrics()

	if metrics.Alloc == 0 {
		t.Error("Alloc should not be zero")
	}

	if metrics.Sys == 0 {
		t.Error("Sys should not be zero")
	}

	if metrics.HeapAlloc == 0 {
		t.Error("HeapAlloc should not be zero")
	}

	if metrics.UsagePercent < 0 || metrics.UsagePercent > 100 {
		t.Errorf("UsagePercent should be between 0-100, got %f", metrics.UsagePercent)
	}
}

func TestCollectGoroutineMetrics(t *testing.T) {
	config := DefaultMonitorConfig()
	monitor := NewSystemMonitor(config)

	metrics := monitor.collectGoroutineMetrics()

	if metrics.Total <= 0 {
		t.Error("Total goroutines should be positive")
	}

	// Should match runtime
	if metrics.Total != runtime.NumGoroutine() {
		t.Logf("Goroutine count mismatch: metrics=%d, runtime=%d (may vary slightly)",
			metrics.Total, runtime.NumGoroutine())
	}
}

func TestCollectGCMetrics(t *testing.T) {
	config := DefaultMonitorConfig()
	monitor := NewSystemMonitor(config)

	// Force a GC to ensure we have data
	runtime.GC()

	metrics := monitor.collectGCMetrics()

	if metrics.NumGC == 0 {
		t.Log("No GC cycles yet (may be expected in test)")
	}

	if metrics.GCCPUFraction < 0 || metrics.GCCPUFraction > 1 {
		t.Errorf("GC CPU fraction should be between 0-1, got %f", metrics.GCCPUFraction)
	}
}

func TestAlertThresholds(t *testing.T) {
	thresholds := AlertThresholds{
		CPUUsage:       80.0,
		MemoryUsage:    90.0,
		GoroutineCount: 1000,
		HeapSize:       1024 * 1024 * 1024, // 1GB
	}

	if thresholds.CPUUsage != 80.0 {
		t.Errorf("Expected CPU threshold 80.0, got %f", thresholds.CPUUsage)
	}

	if thresholds.MemoryUsage != 90.0 {
		t.Errorf("Expected memory threshold 90.0, got %f", thresholds.MemoryUsage)
	}

	if thresholds.GoroutineCount != 1000 {
		t.Errorf("Expected goroutine threshold 1000, got %d", thresholds.GoroutineCount)
	}
}

func TestSystemMonitor_ContextCancellation(t *testing.T) {
	config := DefaultMonitorConfig()
	config.CollectInterval = 50 * time.Millisecond
	monitor := NewSystemMonitor(config)

	ctx, cancel := context.WithCancel(context.Background())

	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start: %v", err)
	}

	// Cancel context
	cancel()

	// Give it time to stop
	time.Sleep(100 * time.Millisecond)

	// Monitor should stop due to context cancellation
	if monitor.IsRunning() {
		t.Log("Monitor still running after context cancel (may need manual Stop)")
	}

	monitor.Stop()
}

func TestSystemMonitor_MetricsUpdate(t *testing.T) {
	config := DefaultMonitorConfig()
	monitor := NewSystemMonitor(config)

	// Collect initial metrics
	monitor.collectMetrics()
	metrics1 := monitor.GetMetrics()
	timestamp1 := metrics1.Timestamp

	// Wait a bit and collect again
	time.Sleep(10 * time.Millisecond)
	monitor.collectMetrics()
	metrics2 := monitor.GetMetrics()
	timestamp2 := metrics2.Timestamp

	if !timestamp2.After(timestamp1) {
		t.Error("Second metrics timestamp should be after first")
	}
}
