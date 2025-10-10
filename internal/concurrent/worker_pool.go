package concurrent

import (
	"context"
	"runtime"
	"sync"
	"time"
)

// Task represents an executable task
type Task struct {
	ID       string
	Payload  interface{}
	Priority int
	Execute  func(ctx context.Context, payload interface{}) (interface{}, error)
	Callback func(result interface{}, err error)
}

// WorkerPool worker pool manager
type WorkerPool struct {
	workers     int
	taskQueue   chan Task
	resultQueue chan TaskResult
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	stats       PoolStats
	mu          sync.RWMutex
}

// TaskResult 任務執行結果
type TaskResult struct {
	TaskID    string
	Result    interface{}
	Error     error
	Duration  time.Duration
	StartTime time.Time
	EndTime   time.Time
}

// PoolStats 工作池統計
type PoolStats struct {
	TotalTasks     int64
	CompletedTasks int64
	FailedTasks    int64
	AverageTime    time.Duration
	ActiveWorkers  int
	QueueSize      int
	MaxQueueSize   int
}

// WorkerPoolConfig 工作池配置
type WorkerPoolConfig struct {
	WorkerCount int
	QueueSize   int
	EnableStats bool
	Timeout     time.Duration
}

// DefaultWorkerPoolConfig 默認工作池配置
func DefaultWorkerPoolConfig() WorkerPoolConfig {
	return WorkerPoolConfig{
		WorkerCount: runtime.NumCPU(),
		QueueSize:   100,
		EnableStats: true,
		Timeout:     30 * time.Second,
	}
}

// NewWorkerPool 創建新的工作池
func NewWorkerPool(config WorkerPoolConfig) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &WorkerPool{
		workers:     config.WorkerCount,
		taskQueue:   make(chan Task, config.QueueSize),
		resultQueue: make(chan TaskResult, config.QueueSize),
		ctx:         ctx,
		cancel:      cancel,
		stats: PoolStats{
			MaxQueueSize: config.QueueSize,
		},
	}

	// 啟動工作協程
	for i := 0; i < config.WorkerCount; i++ {
		pool.wg.Add(1)
		go pool.worker(i)
	}

	// 啟動結果處理協程
	go pool.resultProcessor()

	return pool
}

// Submit 提交任務到工作池
func (wp *WorkerPool) Submit(task Task) bool {
	select {
	case wp.taskQueue <- task:
		wp.mu.Lock()
		wp.stats.TotalTasks++
		wp.stats.QueueSize = len(wp.taskQueue)
		wp.mu.Unlock()
		return true
	default:
		return false // 隊列滿了
	}
}

// SubmitWithTimeout 帶超時的任務提交
func (wp *WorkerPool) SubmitWithTimeout(task Task, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case wp.taskQueue <- task:
		wp.mu.Lock()
		wp.stats.TotalTasks++
		wp.stats.QueueSize = len(wp.taskQueue)
		wp.mu.Unlock()
		return true
	case <-ctx.Done():
		return false
	}
}

// SubmitBatch 批量提交任務
func (wp *WorkerPool) SubmitBatch(tasks []Task) int {
	submitted := 0
	for _, task := range tasks {
		if wp.Submit(task) {
			submitted++
		} else {
			break // 隊列滿了，停止提交
		}
	}
	return submitted
}

// GetStats 獲取工作池統計
func (wp *WorkerPool) GetStats() PoolStats {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	stats := wp.stats
	stats.QueueSize = len(wp.taskQueue)
	stats.ActiveWorkers = wp.workers
	return stats
}

// Close 關閉工作池
func (wp *WorkerPool) Close() {
	close(wp.taskQueue)
	wp.wg.Wait()
	wp.cancel()
	close(wp.resultQueue)
}

// Shutdown 優雅關閉工作池
func (wp *WorkerPool) Shutdown(timeout time.Duration) error {
	done := make(chan struct{})

	go func() {
		wp.Close()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		wp.cancel()
		return context.DeadlineExceeded
	}
}

// worker 工作協程
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	for {
		select {
		case task, ok := <-wp.taskQueue:
			if !ok {
				return // 通道關閉
			}
			wp.executeTask(task)
		case <-wp.ctx.Done():
			return // 上下文取消
		}
	}
}

// executeTask 執行任務
func (wp *WorkerPool) executeTask(task Task) {
	startTime := time.Now()

	result, err := task.Execute(wp.ctx, task.Payload)

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	taskResult := TaskResult{
		TaskID:    task.ID,
		Result:    result,
		Error:     err,
		Duration:  duration,
		StartTime: startTime,
		EndTime:   endTime,
	}

	// 更新統計
	wp.mu.Lock()
	if err != nil {
		wp.stats.FailedTasks++
	} else {
		wp.stats.CompletedTasks++
	}

	// 更新平均時間
	if wp.stats.AverageTime == 0 {
		wp.stats.AverageTime = duration
	} else {
		wp.stats.AverageTime = (wp.stats.AverageTime + duration) / 2
	}
	wp.mu.Unlock()

	// 發送結果到結果隊列
	select {
	case wp.resultQueue <- taskResult:
	case <-wp.ctx.Done():
	}

	// 調用回調函數
	if task.Callback != nil {
		task.Callback(result, err)
	}
}

// resultProcessor 結果處理協程
func (wp *WorkerPool) resultProcessor() {
	for {
		select {
		case result, ok := <-wp.resultQueue:
			if !ok {
				return
			}
			// 這裡可以添加結果處理邏輯，如日誌記錄
			_ = result
		case <-wp.ctx.Done():
			return
		}
	}
}

// PriorityTaskQueue 優先級任務隊列
type PriorityTaskQueue struct {
	tasks []Task
	mu    sync.Mutex
}

// NewPriorityTaskQueue 創建優先級任務隊列
func NewPriorityTaskQueue() *PriorityTaskQueue {
	return &PriorityTaskQueue{
		tasks: make([]Task, 0),
	}
}

// Push 添加任務
func (pq *PriorityTaskQueue) Push(task Task) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	// 根據優先級插入
	inserted := false
	for i, t := range pq.tasks {
		if task.Priority > t.Priority {
			pq.tasks = append(pq.tasks[:i], append([]Task{task}, pq.tasks[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted {
		pq.tasks = append(pq.tasks, task)
	}
}

// Pop 取出最高優先級任務
func (pq *PriorityTaskQueue) Pop() (Task, bool) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if len(pq.tasks) == 0 {
		return Task{}, false
	}

	task := pq.tasks[0]
	pq.tasks = pq.tasks[1:]
	return task, true
}

// Size 獲取隊列大小
func (pq *PriorityTaskQueue) Size() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return len(pq.tasks)
}

// Clear 清空隊列
func (pq *PriorityTaskQueue) Clear() {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.tasks = pq.tasks[:0]
}
