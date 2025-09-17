package concurrent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Pipeline processing pipeline for chaining multiple processing stages
type Pipeline struct {
	stages []PipelineStage
	mu     sync.RWMutex
	stats  PipelineStats
}

// PipelineStage pipeline stage
type PipelineStage struct {
	Name       string
	Processor  func(ctx context.Context, data interface{}) (interface{}, error)
	Parallel   bool // Whether parallel processing is possible
	Timeout    time.Duration
	MaxWorkers int
	workerPool *WorkerPool
}

// PipelineStats 管道統計
type PipelineStats struct {
	TotalProcessed int64
	TotalErrors    int64
	AvgLatency     time.Duration
	StageStats     map[string]StageStats
}

// StageStats 階段統計
type StageStats struct {
	Processed int64
	Errors    int64
	AvgTime   time.Duration
	MaxTime   time.Duration
	MinTime   time.Duration
}

// PipelineConfig 管道配置
type PipelineConfig struct {
	BufferSize     int
	EnableMetrics  bool
	DefaultTimeout time.Duration
}

// NewPipeline 創建新的處理管道
func NewPipeline(config PipelineConfig) *Pipeline {
	return &Pipeline{
		stages: make([]PipelineStage, 0),
		stats: PipelineStats{
			StageStats: make(map[string]StageStats),
		},
	}
}

// AddStage 添加處理階段
func (p *Pipeline) AddStage(stage PipelineStage) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 如果階段支持並行處理，創建工作池
	if stage.Parallel && stage.MaxWorkers > 0 {
		stage.workerPool = NewWorkerPool(WorkerPoolConfig{
			WorkerCount: stage.MaxWorkers,
			QueueSize:   stage.MaxWorkers * 2,
			EnableStats: true,
			Timeout:     stage.Timeout,
		})
	}

	p.stages = append(p.stages, stage)

	// 初始化階段統計
	p.stats.StageStats[stage.Name] = StageStats{
		MinTime: time.Duration(1<<63 - 1), // 最大可能值
	}
}

// Process 處理數據通過整個管道
func (p *Pipeline) Process(ctx context.Context, data interface{}) (interface{}, error) {
	startTime := time.Now()
	currentData := data

	p.mu.RLock()
	stages := make([]PipelineStage, len(p.stages))
	copy(stages, p.stages)
	p.mu.RUnlock()

	for _, stage := range stages {
		stageStartTime := time.Now()

		var result interface{}
		var err error

		if stage.Parallel && stage.workerPool != nil {
			// 並行處理
			result, err = p.processStageParallel(ctx, stage, currentData)
		} else {
			// 串行處理
			stageCtx := ctx
			if stage.Timeout > 0 {
				var cancel context.CancelFunc
				stageCtx, cancel = context.WithTimeout(ctx, stage.Timeout)
				defer cancel()
			}

			result, err = stage.Processor(stageCtx, currentData)
		}

		stageTime := time.Since(stageStartTime)
		p.updateStageStats(stage.Name, stageTime, err)

		if err != nil {
			p.incrementErrors()
			return nil, err
		}

		currentData = result
	}

	totalTime := time.Since(startTime)
	p.updatePipelineStats(totalTime)

	return currentData, nil
}

// ProcessBatch 批量處理數據
func (p *Pipeline) ProcessBatch(ctx context.Context, items []interface{}) ([]interface{}, []error) {
	results := make([]interface{}, len(items))
	errors := make([]error, len(items))

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10) // 限制並發數

	for i, item := range items {
		wg.Add(1)
		go func(index int, data interface{}) {
			defer wg.Done()

			semaphore <- struct{}{}        // 獲取信號量
			defer func() { <-semaphore }() // 釋放信號量

			result, err := p.Process(ctx, data)
			results[index] = result
			errors[index] = err
		}(i, item)
	}

	wg.Wait()
	return results, errors
}

// GetStats 獲取管道統計
func (p *Pipeline) GetStats() PipelineStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 深度複製統計信息
	stats := p.stats
	stats.StageStats = make(map[string]StageStats)
	for name, stageStats := range p.stats.StageStats {
		stats.StageStats[name] = stageStats
	}

	return stats
}

// Close 關閉管道
func (p *Pipeline) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, stage := range p.stages {
		if stage.workerPool != nil {
			stage.workerPool.Close()
		}
	}
}

// 內部方法

func (p *Pipeline) processStageParallel(ctx context.Context, stage PipelineStage, data interface{}) (interface{}, error) {
	resultChan := make(chan interface{}, 1)
	errorChan := make(chan error, 1)

	task := Task{
		ID:      "pipeline_" + stage.Name,
		Payload: data,
		Execute: stage.Processor,
		Callback: func(result interface{}, err error) {
			if err != nil {
				errorChan <- err
			} else {
				resultChan <- result
			}
		},
	}

	if !stage.workerPool.Submit(task) {
		return nil, fmt.Errorf("failed to submit task to stage %s", stage.Name)
	}

	timeout := stage.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errorChan:
		return nil, err
	case <-time.After(timeout):
		return nil, context.DeadlineExceeded
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *Pipeline) updateStageStats(stageName string, duration time.Duration, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	stats := p.stats.StageStats[stageName]
	stats.Processed++

	if err != nil {
		stats.Errors++
	}

	// 更新時間統計
	if stats.MaxTime < duration {
		stats.MaxTime = duration
	}
	if stats.MinTime > duration {
		stats.MinTime = duration
	}

	// 計算平均時間
	if stats.AvgTime == 0 {
		stats.AvgTime = duration
	} else {
		stats.AvgTime = (stats.AvgTime + duration) / 2
	}

	p.stats.StageStats[stageName] = stats
}

func (p *Pipeline) updatePipelineStats(duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats.TotalProcessed++

	// 更新平均延遲
	if p.stats.AvgLatency == 0 {
		p.stats.AvgLatency = duration
	} else {
		p.stats.AvgLatency = (p.stats.AvgLatency + duration) / 2
	}
}

func (p *Pipeline) incrementErrors() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats.TotalErrors++
}

// PipelineBuilder 管道構建器
type PipelineBuilder struct {
	pipeline *Pipeline
}

// NewPipelineBuilder 創建管道構建器
func NewPipelineBuilder(config PipelineConfig) *PipelineBuilder {
	return &PipelineBuilder{
		pipeline: NewPipeline(config),
	}
}

// AddSerialStage 添加串行階段
func (pb *PipelineBuilder) AddSerialStage(name string, processor func(context.Context, interface{}) (interface{}, error)) *PipelineBuilder {
	stage := PipelineStage{
		Name:      name,
		Processor: processor,
		Parallel:  false,
		Timeout:   30 * time.Second,
	}
	pb.pipeline.AddStage(stage)
	return pb
}

// AddParallelStage 添加並行階段
func (pb *PipelineBuilder) AddParallelStage(name string, processor func(context.Context, interface{}) (interface{}, error), maxWorkers int) *PipelineBuilder {
	stage := PipelineStage{
		Name:       name,
		Processor:  processor,
		Parallel:   true,
		Timeout:    30 * time.Second,
		MaxWorkers: maxWorkers,
	}
	pb.pipeline.AddStage(stage)
	return pb
}

// WithTimeout 設置階段超時
func (pb *PipelineBuilder) WithTimeout(timeout time.Duration) *PipelineBuilder {
	if len(pb.pipeline.stages) > 0 {
		lastIndex := len(pb.pipeline.stages) - 1
		pb.pipeline.stages[lastIndex].Timeout = timeout
	}
	return pb
}

// Build 構建管道
func (pb *PipelineBuilder) Build() *Pipeline {
	return pb.pipeline
}
