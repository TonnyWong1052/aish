package monitoring

import (
	"math"
	"sort"
	"sync"
	"time"
)

// PerformanceAnalyzer 性能分析器
type PerformanceAnalyzer struct {
	operations map[string]*OperationStats
	latencies  map[string]*LatencyTracker
	mu         sync.RWMutex
	config     *AnalyzerConfig
}

// AnalyzerConfig 分析器配置
type AnalyzerConfig struct {
	MaxSamples        int           `json:"max_samples"`
	SampleWindow      time.Duration `json:"sample_window"`
	EnablePercentiles bool          `json:"enable_percentiles"`
	EnableTrends      bool          `json:"enable_trends"`
	RetentionPeriod   time.Duration `json:"retention_period"`
}

// OperationStats 操作統計
type OperationStats struct {
	Name             string        `json:"name"`
	TotalCount       int64         `json:"total_count"`
	SuccessCount     int64         `json:"success_count"`
	ErrorCount       int64         `json:"error_count"`
	TotalDuration    time.Duration `json:"total_duration"`
	MinDuration      time.Duration `json:"min_duration"`
	MaxDuration      time.Duration `json:"max_duration"`
	AverageDuration  time.Duration `json:"average_duration"`
	LastExecuted     time.Time     `json:"last_executed"`
	FirstExecuted    time.Time     `json:"first_executed"`
	ErrorRate        float64       `json:"error_rate"`
	OperationsPerSec float64       `json:"operations_per_sec"`
}

// LatencyTracker 延遲追蹤器
type LatencyTracker struct {
	samples    []LatencySample
	maxSamples int
	mu         sync.RWMutex
}

// LatencySample 延遲樣本
type LatencySample struct {
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
	Success   bool          `json:"success"`
}

// PercentileStats 百分位統計
type PercentileStats struct {
	P50 time.Duration `json:"p50"`
	P90 time.Duration `json:"p90"`
	P95 time.Duration `json:"p95"`
	P99 time.Duration `json:"p99"`
}

// TrendStats ���勢統計
type TrendStats struct {
	Slope           float64 `json:"slope"`            // 趨勢斜率
	RSquared        float64 `json:"r_squared"`        // R²值
	Direction       string  `json:"direction"`        // 趨勢方向
	ConfidenceLevel float64 `json:"confidence_level"` // 置信水平
}

// DefaultAnalyzerConfig 返回默認分析器配置
func DefaultAnalyzerConfig() *AnalyzerConfig {
	return &AnalyzerConfig{
		MaxSamples:        1000,
		SampleWindow:      time.Minute,
		EnablePercentiles: true,
		EnableTrends:      true,
		RetentionPeriod:   time.Hour,
	}
}

// NewPerformanceAnalyzer 創建性能分析器
func NewPerformanceAnalyzer(config *AnalyzerConfig) *PerformanceAnalyzer {
	if config == nil {
		config = DefaultAnalyzerConfig()
	}

	return &PerformanceAnalyzer{
		operations: make(map[string]*OperationStats),
		latencies:  make(map[string]*LatencyTracker),
		config:     config,
	}
}

// RecordOperation 記錄操作執行
func (pa *PerformanceAnalyzer) RecordOperation(name string, duration time.Duration, success bool) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	now := time.Now()

	// 更新操作統計
	stats, exists := pa.operations[name]
	if !exists {
		stats = &OperationStats{
			Name:          name,
			MinDuration:   duration,
			MaxDuration:   duration,
			FirstExecuted: now,
		}
		pa.operations[name] = stats
	}

	stats.TotalCount++
	stats.TotalDuration += duration
	stats.LastExecuted = now

	if success {
		stats.SuccessCount++
	} else {
		stats.ErrorCount++
	}

	if duration < stats.MinDuration {
		stats.MinDuration = duration
	}
	if duration > stats.MaxDuration {
		stats.MaxDuration = duration
	}

	// 計算派生統計
	stats.AverageDuration = stats.TotalDuration / time.Duration(stats.TotalCount)
	stats.ErrorRate = float64(stats.ErrorCount) / float64(stats.TotalCount) * 100

	// 計算每秒操作數
	if elapsed := now.Sub(stats.FirstExecuted); elapsed > 0 {
		stats.OperationsPerSec = float64(stats.TotalCount) / elapsed.Seconds()
	}

	// 記錄延遲樣本
	pa.recordLatencySample(name, duration, success, now)
}

// recordLatencySample 記錄延遲樣本
func (pa *PerformanceAnalyzer) recordLatencySample(name string, duration time.Duration, success bool, timestamp time.Time) {
	tracker, exists := pa.latencies[name]
	if !exists {
		tracker = &LatencyTracker{
			maxSamples: pa.config.MaxSamples,
		}
		pa.latencies[name] = tracker
	}

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	sample := LatencySample{
		Timestamp: timestamp,
		Duration:  duration,
		Success:   success,
	}

	tracker.samples = append(tracker.samples, sample)

	// 限制樣本數量
	if len(tracker.samples) > tracker.maxSamples {
		tracker.samples = tracker.samples[1:]
	}

	// 清理過期樣本
	cutoff := timestamp.Add(-pa.config.RetentionPeriod)
	for i, sample := range tracker.samples {
		if sample.Timestamp.After(cutoff) {
			tracker.samples = tracker.samples[i:]
			break
		}
	}
}

// GetOperationStats 獲取操作統計
func (pa *PerformanceAnalyzer) GetOperationStats(name string) (*OperationStats, bool) {
	pa.mu.RLock()
	defer pa.mu.RUnlock()

	stats, exists := pa.operations[name]
	if !exists {
		return nil, false
	}

	// 返回副本
	statsCopy := *stats
	return &statsCopy, true
}

// GetAllOperationStats 獲取所有操作統計
func (pa *PerformanceAnalyzer) GetAllOperationStats() map[string]*OperationStats {
	pa.mu.RLock()
	defer pa.mu.RUnlock()

	result := make(map[string]*OperationStats)
	for name, stats := range pa.operations {
		statsCopy := *stats
		result[name] = &statsCopy
	}

	return result
}

// GetPercentiles 獲取百分位統計
func (pa *PerformanceAnalyzer) GetPercentiles(name string) (*PercentileStats, bool) {
	if !pa.config.EnablePercentiles {
		return nil, false
	}

	pa.mu.RLock()
	tracker, exists := pa.latencies[name]
	pa.mu.RUnlock()

	if !exists {
		return nil, false
	}

	tracker.mu.RLock()
	defer tracker.mu.RUnlock()

	if len(tracker.samples) == 0 {
		return nil, false
	}

	// 提取成功的樣本並排序
	var durations []time.Duration
	for _, sample := range tracker.samples {
		if sample.Success {
			durations = append(durations, sample.Duration)
		}
	}

	if len(durations) == 0 {
		return nil, false
	}

	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	return &PercentileStats{
		P50: pa.calculatePercentile(durations, 0.5),
		P90: pa.calculatePercentile(durations, 0.9),
		P95: pa.calculatePercentile(durations, 0.95),
		P99: pa.calculatePercentile(durations, 0.99),
	}, true
}

// calculatePercentile 計算百分位數
func (pa *PerformanceAnalyzer) calculatePercentile(sorted []time.Duration, percentile float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}

	index := percentile * float64(len(sorted)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if lower == upper {
		return sorted[lower]
	}

	// 線性插值
	weight := index - float64(lower)
	return time.Duration(float64(sorted[lower])*(1-weight) + float64(sorted[upper])*weight)
}

// GetTrends 獲取趨勢分析
func (pa *PerformanceAnalyzer) GetTrends(name string) (*TrendStats, bool) {
	if !pa.config.EnableTrends {
		return nil, false
	}

	pa.mu.RLock()
	tracker, exists := pa.latencies[name]
	pa.mu.RUnlock()

	if !exists {
		return nil, false
	}

	tracker.mu.RLock()
	defer tracker.mu.RUnlock()

	if len(tracker.samples) < 3 {
		return nil, false // 需要至少3個數據點
	}

	// 準備數據
	var x, y []float64
	for i, sample := range tracker.samples {
		x = append(x, float64(i))
		y = append(y, float64(sample.Duration.Nanoseconds()))
	}

	// 計算線性回歸
	slope, intercept, rSquared := pa.linearRegression(x, y)

	// 判斷趨勢方向
	direction := "stable"
	confidenceLevel := rSquared * 100

	if math.Abs(slope) > 1e6 && rSquared > 0.5 { // 1ms 變化閾值
		if slope > 0 {
			direction = "increasing"
		} else {
			direction = "decreasing"
		}
	}

	_ = intercept // 暫時不使用

	return &TrendStats{
		Slope:           slope,
		RSquared:        rSquared,
		Direction:       direction,
		ConfidenceLevel: confidenceLevel,
	}, true
}

// linearRegression 線性回歸計算
func (pa *PerformanceAnalyzer) linearRegression(x, y []float64) (slope, intercept, rSquared float64) {
	if len(x) != len(y) || len(x) < 2 {
		return 0, 0, 0
	}

	n := float64(len(x))

	// 計算均值
	var sumX, sumY float64
	for i := range x {
		sumX += x[i]
		sumY += y[i]
	}
	meanX := sumX / n
	meanY := sumY / n

	// 計算斜率和截距
	var numerator, denominator, totalSumSquares, residualSumSquares float64
	for i := range x {
		dx := x[i] - meanX
		dy := y[i] - meanY
		numerator += dx * dy
		denominator += dx * dx
		totalSumSquares += dy * dy
	}

	if denominator == 0 {
		return 0, meanY, 0
	}

	slope = numerator / denominator
	intercept = meanY - slope*meanX

	// 計算 R²
	for i := range x {
		predicted := slope*x[i] + intercept
		residualSumSquares += math.Pow(y[i]-predicted, 2)
	}

	if totalSumSquares == 0 {
		rSquared = 1
	} else {
		rSquared = 1 - residualSumSquares/totalSumSquares
	}

	return slope, intercept, rSquared
}

// GetLatencyHistory 獲取延遲歷史
func (pa *PerformanceAnalyzer) GetLatencyHistory(name string, window time.Duration) ([]LatencySample, bool) {
	pa.mu.RLock()
	tracker, exists := pa.latencies[name]
	pa.mu.RUnlock()

	if !exists {
		return nil, false
	}

	tracker.mu.RLock()
	defer tracker.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	var result []LatencySample

	for _, sample := range tracker.samples {
		if sample.Timestamp.After(cutoff) {
			result = append(result, sample)
		}
	}

	return result, len(result) > 0
}

// Reset 重置特定操作的統計
func (pa *PerformanceAnalyzer) Reset(name string) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	delete(pa.operations, name)
	delete(pa.latencies, name)
}

// ResetAll 重置所有統計
func (pa *PerformanceAnalyzer) ResetAll() {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	pa.operations = make(map[string]*OperationStats)
	pa.latencies = make(map[string]*LatencyTracker)
}

// StartTimer 開始計時器
func (pa *PerformanceAnalyzer) StartTimer(name string) *Timer {
	return &Timer{
		analyzer:  pa,
		name:      name,
		startTime: time.Now(),
	}
}

// Timer 計時器
type Timer struct {
	analyzer  *PerformanceAnalyzer
	name      string
	startTime time.Time
}

// Stop 停止計時並記錄結果
func (t *Timer) Stop(success bool) {
	duration := time.Since(t.startTime)
	t.analyzer.RecordOperation(t.name, duration, success)
}

// StopWithError 停止計時並根據錯誤記錄結果
func (t *Timer) StopWithError(err error) {
	t.Stop(err == nil)
}

// GetSummary 獲取性能摘要
func (pa *PerformanceAnalyzer) GetSummary() *PerformanceSummary {
	pa.mu.RLock()
	defer pa.mu.RUnlock()

	summary := &PerformanceSummary{
		TotalOperations:   len(pa.operations),
		AverageLatency:    0,
		TotalErrorRate:    0,
		OperationsSummary: make(map[string]*OperationStats),
	}

	var totalDuration time.Duration
	var totalCount, totalErrors int64

	for name, stats := range pa.operations {
		summary.OperationsSummary[name] = stats
		totalDuration += stats.TotalDuration
		totalCount += stats.TotalCount
		totalErrors += stats.ErrorCount
	}

	if totalCount > 0 {
		summary.AverageLatency = totalDuration / time.Duration(totalCount)
		summary.TotalErrorRate = float64(totalErrors) / float64(totalCount) * 100
	}

	return summary
}

// PerformanceSummary 性能摘要
type PerformanceSummary struct {
	TotalOperations   int                        `json:"total_operations"`
	AverageLatency    time.Duration              `json:"average_latency"`
	TotalErrorRate    float64                    `json:"total_error_rate"`
	OperationsSummary map[string]*OperationStats `json:"operations_summary"`
}

// Cleanup 清理過期數據
func (pa *PerformanceAnalyzer) Cleanup() {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	cutoff := time.Now().Add(-pa.config.RetentionPeriod)

	for name, tracker := range pa.latencies {
		tracker.mu.Lock()
		var validSamples []LatencySample
		for _, sample := range tracker.samples {
			if sample.Timestamp.After(cutoff) {
				validSamples = append(validSamples, sample)
			}
		}
		tracker.samples = validSamples
		tracker.mu.Unlock()

		// 如果沒有有效樣本，移除追蹤器
		if len(validSamples) == 0 {
			delete(pa.latencies, name)
			delete(pa.operations, name)
		}
	}
}
