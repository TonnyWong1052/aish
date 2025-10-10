package concurrent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/TonnyWong1052/aish/internal/llm"
)

// ProcessingCoordinator processing coordinator, manages concurrent AI requests and local processing
type ProcessingCoordinator struct {
	aiWorkerPool    *WorkerPool
	localWorkerPool *WorkerPool
	cacheWorkerPool *WorkerPool

	// Task counter
	taskCounter int64
	mu          sync.Mutex

	// 統計信息
	stats CoordinatorStats
}

// CoordinatorStats 協調器統計
type CoordinatorStats struct {
	AITasks         int64
	LocalTasks      int64
	CacheTasks      int64
	ConcurrentTasks int64
	TotalLatency    time.Duration
	AvgLatency      time.Duration
}

// AITaskPayload AI 任務負載
type AITaskPayload struct {
	Provider    llm.Provider
	Context     llm.CapturedContext
	Language    string
	RequestType string // "suggestion" 或 "command"
	Prompt      string
}

// LocalTaskPayload 本地任務負載
type LocalTaskPayload struct {
	TaskType string // "config_load", "history_save", "template_compile"
	Data     interface{}
}

// CacheTaskPayload 緩存任務負載
type CacheTaskPayload struct {
	Operation string // "get", "set", "delete"
	Key       string
	Value     interface{}
	TTL       time.Duration
}

// TaskGroup 任務組，用於協調多個相關任務
type TaskGroup struct {
	ID        string
	Tasks     []Task
	Results   map[string]TaskResult
	mu        sync.RWMutex
	done      chan struct{}
	timeout   time.Duration
	startTime time.Time
}

// NewProcessingCoordinator 創建新的處理協調器
func NewProcessingCoordinator() *ProcessingCoordinator {
	return &ProcessingCoordinator{
		aiWorkerPool: NewWorkerPool(WorkerPoolConfig{
			WorkerCount: 2, // AI 請求通常較慢，使用較少的工作者
			QueueSize:   20,
			EnableStats: true,
			Timeout:     60 * time.Second,
		}),
		localWorkerPool: NewWorkerPool(WorkerPoolConfig{
			WorkerCount: 4, // 本地處理較快，可以使用更多工作者
			QueueSize:   50,
			EnableStats: true,
			Timeout:     10 * time.Second,
		}),
		cacheWorkerPool: NewWorkerPool(WorkerPoolConfig{
			WorkerCount: 3, // 緩存操作中等速度
			QueueSize:   30,
			EnableStats: true,
			Timeout:     5 * time.Second,
		}),
	}
}

// ProcessAIRequest 處理 AI 請求（異步）
func (pc *ProcessingCoordinator) ProcessAIRequest(
	ctx context.Context,
	provider llm.Provider,
	capturedContext llm.CapturedContext,
	language string,
	callback func(*llm.Suggestion, error),
) string {
	taskID := pc.generateTaskID()

	task := Task{
		ID:       taskID,
		Priority: 5, // 中等優先級
		Payload: AITaskPayload{
			Provider:    provider,
			Context:     capturedContext,
			Language:    language,
			RequestType: "suggestion",
		},
		Execute: func(ctx context.Context, payload interface{}) (interface{}, error) {
			aiPayload := payload.(AITaskPayload)
			return aiPayload.Provider.GetSuggestion(ctx, aiPayload.Context, aiPayload.Language)
		},
		Callback: func(result interface{}, err error) {
			if suggestion, ok := result.(*llm.Suggestion); ok {
				callback(suggestion, err)
			} else {
				callback(nil, err)
			}
		},
	}

	pc.aiWorkerPool.Submit(task)
	pc.incrementTaskCount("ai")

	return taskID
}

// ProcessCommandGeneration 處理命令生成（異步）
func (pc *ProcessingCoordinator) ProcessCommandGeneration(
	ctx context.Context,
	provider llm.Provider,
	prompt string,
	language string,
	callback func(string, error),
) string {
	taskID := pc.generateTaskID()

	task := Task{
		ID:       taskID,
		Priority: 7, // 較高優先級，用戶直接請求
		Payload: AITaskPayload{
			Provider:    provider,
			RequestType: "command",
			Prompt:      prompt,
			Language:    language,
		},
		Execute: func(ctx context.Context, payload interface{}) (interface{}, error) {
			aiPayload := payload.(AITaskPayload)
			return aiPayload.Provider.GenerateCommand(ctx, aiPayload.Prompt, aiPayload.Language)
		},
		Callback: func(result interface{}, err error) {
			if command, ok := result.(string); ok {
				callback(command, err)
			} else {
				callback("", err)
			}
		},
	}

	pc.aiWorkerPool.Submit(task)
	pc.incrementTaskCount("ai")

	return taskID
}

// ProcessLocalTask 處理本地任務
func (pc *ProcessingCoordinator) ProcessLocalTask(
	taskType string,
	data interface{},
	callback func(interface{}, error),
) string {
	taskID := pc.generateTaskID()

	task := Task{
		ID:       taskID,
		Priority: 3, // 較低優先級
		Payload: LocalTaskPayload{
			TaskType: taskType,
			Data:     data,
		},
		Execute:  pc.executeLocalTask,
		Callback: callback,
	}

	pc.localWorkerPool.Submit(task)
	pc.incrementTaskCount("local")

	return taskID
}

// ProcessCacheTask 處理緩存任務
func (pc *ProcessingCoordinator) ProcessCacheTask(
	operation, key string,
	value interface{},
	ttl time.Duration,
	callback func(interface{}, error),
) string {
	taskID := pc.generateTaskID()

	task := Task{
		ID:       taskID,
		Priority: 1, // 最低優先級
		Payload: CacheTaskPayload{
			Operation: operation,
			Key:       key,
			Value:     value,
			TTL:       ttl,
		},
		Execute:  pc.executeCacheTask,
		Callback: callback,
	}

	pc.cacheWorkerPool.Submit(task)
	pc.incrementTaskCount("cache")

	return taskID
}

// CreateTaskGroup 創建任務組
func (pc *ProcessingCoordinator) CreateTaskGroup(id string, timeout time.Duration) *TaskGroup {
	return &TaskGroup{
		ID:        id,
		Tasks:     make([]Task, 0),
		Results:   make(map[string]TaskResult),
		done:      make(chan struct{}),
		timeout:   timeout,
		startTime: time.Now(),
	}
}

// ExecuteTaskGroup 執行任務組（並行執行所有任務）
func (pc *ProcessingCoordinator) ExecuteTaskGroup(group *TaskGroup) error {
	if len(group.Tasks) == 0 {
		close(group.done)
		return nil
	}

	var wg sync.WaitGroup

	for _, task := range group.Tasks {
		wg.Add(1)

		// 修改任務的回調函數以記錄結果
		originalCallback := task.Callback
		task.Callback = func(result interface{}, err error) {
			group.mu.Lock()
			group.Results[task.ID] = TaskResult{
				TaskID: task.ID,
				Result: result,
				Error:  err,
			}
			group.mu.Unlock()

			if originalCallback != nil {
				originalCallback(result, err)
			}

			wg.Done()
		}

		// 根據任務類型提交到對應的工作池
		switch task.Payload.(type) {
		case AITaskPayload:
			pc.aiWorkerPool.Submit(task)
		case LocalTaskPayload:
			pc.localWorkerPool.Submit(task)
		case CacheTaskPayload:
			pc.cacheWorkerPool.Submit(task)
		}
	}

	// 等待所有任務完成或超時
	go func() {
		wg.Wait()
		close(group.done)
	}()

	select {
	case <-group.done:
		return nil
	case <-time.After(group.timeout):
		return context.DeadlineExceeded
	}
}

// GetStats 獲取協調器統計
func (pc *ProcessingCoordinator) GetStats() CoordinatorStats {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	stats := pc.stats

	// 添加工作池統計
	aiStats := pc.aiWorkerPool.GetStats()
	localStats := pc.localWorkerPool.GetStats()
	cacheStats := pc.cacheWorkerPool.GetStats()

	stats.ConcurrentTasks = int64(aiStats.QueueSize + localStats.QueueSize + cacheStats.QueueSize)

	// 計算平均延遲
	totalTasks := stats.AITasks + stats.LocalTasks + stats.CacheTasks
	if totalTasks > 0 {
		avgAI := aiStats.AverageTime
		avgLocal := localStats.AverageTime
		avgCache := cacheStats.AverageTime

		totalTime := time.Duration(stats.AITasks)*avgAI +
			time.Duration(stats.LocalTasks)*avgLocal +
			time.Duration(stats.CacheTasks)*avgCache

		stats.AvgLatency = totalTime / time.Duration(totalTasks)
	}

	return stats
}

// Close 關閉協調器
func (pc *ProcessingCoordinator) Close() {
	pc.aiWorkerPool.Close()
	pc.localWorkerPool.Close()
	pc.cacheWorkerPool.Close()
}

// 內部方法

func (pc *ProcessingCoordinator) generateTaskID() string {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.taskCounter++
	return fmt.Sprintf("task_%d_%d", time.Now().UnixNano(), pc.taskCounter)
}

func (pc *ProcessingCoordinator) incrementTaskCount(taskType string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	switch taskType {
	case "ai":
		pc.stats.AITasks++
	case "local":
		pc.stats.LocalTasks++
	case "cache":
		pc.stats.CacheTasks++
	}
}

func (pc *ProcessingCoordinator) executeLocalTask(ctx context.Context, payload interface{}) (interface{}, error) {
	localPayload := payload.(LocalTaskPayload)

	switch localPayload.TaskType {
	case "config_load":
		// 配置加載邏輯
		return "config loaded", nil
	case "history_save":
		// 歷史保存邏輯
		return "history saved", nil
	case "template_compile":
		// 模板編譯邏輯
		return "template compiled", nil
	default:
		return nil, fmt.Errorf("unknown local task type: %s", localPayload.TaskType)
	}
}

func (pc *ProcessingCoordinator) executeCacheTask(ctx context.Context, payload interface{}) (interface{}, error) {
	cachePayload := payload.(CacheTaskPayload)

	switch cachePayload.Operation {
	case "get":
		// 緩存獲取邏輯
		return cachePayload.Value, nil
	case "set":
		// 緩存設置邏輯
		return "cache set", nil
	case "delete":
		// 緩存刪除邏輯
		return "cache deleted", nil
	default:
		return nil, fmt.Errorf("unknown cache operation: %s", cachePayload.Operation)
	}
}
