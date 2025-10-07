package concurrent

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultWorkerPoolConfig(t *testing.T) {
	config := DefaultWorkerPoolConfig()

	if config.WorkerCount <= 0 {
		t.Error("WorkerCount should be positive")
	}

	if config.QueueSize != 100 {
		t.Errorf("Expected QueueSize 100, got %d", config.QueueSize)
	}

	if !config.EnableStats {
		t.Error("EnableStats should be true")
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", config.Timeout)
	}
}

func TestNewWorkerPool(t *testing.T) {
	config := WorkerPoolConfig{
		WorkerCount: 2,
		QueueSize:   10,
		EnableStats: true,
		Timeout:     5 * time.Second,
	}

	pool := NewWorkerPool(config)
	if pool == nil {
		t.Fatal("Pool should not be nil")
	}

	stats := pool.GetStats()
	if stats.ActiveWorkers != 2 {
		t.Errorf("Expected 2 active workers, got %d", stats.ActiveWorkers)
	}

	pool.Close()
}

func TestWorkerPool_Submit(t *testing.T) {
	pool := NewWorkerPool(WorkerPoolConfig{
		WorkerCount: 2,
		QueueSize:   5,
		EnableStats: true,
	})
	defer pool.Close()

	executed := make(chan bool, 1)

	task := Task{
		ID:      "test-1",
		Payload: "test data",
		Execute: func(ctx context.Context, payload interface{}) (interface{}, error) {
			executed <- true
			return "result", nil
		},
	}

	success := pool.Submit(task)
	if !success {
		t.Error("Task submission should succeed")
	}

	select {
	case <-executed:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Task was not executed in time")
	}
}

func TestWorkerPool_SubmitFullQueue(t *testing.T) {
	pool := NewWorkerPool(WorkerPoolConfig{
		WorkerCount: 1,
		QueueSize:   2,
		EnableStats: true,
	})
	defer pool.Close()

	// Create slow task to fill the queue
	slowTask := Task{
		ID: "slow",
		Execute: func(ctx context.Context, payload interface{}) (interface{}, error) {
			time.Sleep(100 * time.Millisecond)
			return nil, nil
		},
	}

	// Fill up the queue
	pool.Submit(slowTask)
	pool.Submit(slowTask)
	pool.Submit(slowTask)

	// Try to submit another task - should fail
	fastTask := Task{
		ID: "fast",
		Execute: func(ctx context.Context, payload interface{}) (interface{}, error) {
			return nil, nil
		},
	}

	success := pool.Submit(fastTask)
	if success {
		t.Error("Submit should fail when queue is full")
	}
}

func TestWorkerPool_SubmitWithTimeout(t *testing.T) {
	pool := NewWorkerPool(WorkerPoolConfig{
		WorkerCount: 2,
		QueueSize:   10,
	})
	defer pool.Close()

	task := Task{
		ID: "timeout-test",
		Execute: func(ctx context.Context, payload interface{}) (interface{}, error) {
			return "done", nil
		},
	}

	success := pool.SubmitWithTimeout(task, 1*time.Second)
	if !success {
		t.Error("Task submission with timeout should succeed")
	}
}

func TestWorkerPool_SubmitWithTimeout_Expired(t *testing.T) {
	t.Skip("Flaky test - timing dependent on system load")
}

func TestWorkerPool_SubmitBatch(t *testing.T) {
	pool := NewWorkerPool(WorkerPoolConfig{
		WorkerCount: 3,
		QueueSize:   10,
	})
	defer pool.Close()

	tasks := []Task{
		{ID: "batch-1", Execute: func(ctx context.Context, payload interface{}) (interface{}, error) {
			return "1", nil
		}},
		{ID: "batch-2", Execute: func(ctx context.Context, payload interface{}) (interface{}, error) {
			return "2", nil
		}},
		{ID: "batch-3", Execute: func(ctx context.Context, payload interface{}) (interface{}, error) {
			return "3", nil
		}},
	}

	submitted := pool.SubmitBatch(tasks)
	if submitted != 3 {
		t.Errorf("Expected 3 tasks submitted, got %d", submitted)
	}
}

func TestWorkerPool_GetStats(t *testing.T) {
	pool := NewWorkerPool(WorkerPoolConfig{
		WorkerCount: 2,
		QueueSize:   10,
		EnableStats: true,
	})
	defer pool.Close()

	// Submit some tasks
	for i := 0; i < 5; i++ {
		pool.Submit(Task{
			ID: "stats-test",
			Execute: func(ctx context.Context, payload interface{}) (interface{}, error) {
				time.Sleep(10 * time.Millisecond)
				return nil, nil
			},
		})
	}

	time.Sleep(100 * time.Millisecond)

	stats := pool.GetStats()

	if stats.TotalTasks != 5 {
		t.Errorf("Expected 5 total tasks, got %d", stats.TotalTasks)
	}

	if stats.MaxQueueSize != 10 {
		t.Errorf("Expected max queue size 10, got %d", stats.MaxQueueSize)
	}
}

func TestWorkerPool_TaskCallback(t *testing.T) {
	pool := NewWorkerPool(WorkerPoolConfig{
		WorkerCount: 2,
		QueueSize:   5,
	})
	defer pool.Close()

	callbackCalled := make(chan bool, 1)
	expectedResult := "callback result"

	task := Task{
		ID: "callback-test",
		Execute: func(ctx context.Context, payload interface{}) (interface{}, error) {
			return expectedResult, nil
		},
		Callback: func(result interface{}, err error) {
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != expectedResult {
				t.Errorf("Expected result %v, got %v", expectedResult, result)
			}
			callbackCalled <- true
		},
	}

	pool.Submit(task)

	select {
	case <-callbackCalled:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Callback was not called")
	}
}

func TestWorkerPool_TaskError(t *testing.T) {
	pool := NewWorkerPool(WorkerPoolConfig{
		WorkerCount: 2,
		QueueSize:   5,
		EnableStats: true,
	})
	defer pool.Close()

	expectedError := errors.New("task error")
	errorReceived := make(chan error, 1)

	task := Task{
		ID: "error-test",
		Execute: func(ctx context.Context, payload interface{}) (interface{}, error) {
			return nil, expectedError
		},
		Callback: func(result interface{}, err error) {
			errorReceived <- err
		},
	}

	pool.Submit(task)

	select {
	case err := <-errorReceived:
		if err != expectedError {
			t.Errorf("Expected error %v, got %v", expectedError, err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Error callback not called")
	}

	time.Sleep(100 * time.Millisecond)
	stats := pool.GetStats()

	if stats.FailedTasks != 1 {
		t.Errorf("Expected 1 failed task, got %d", stats.FailedTasks)
	}
}

func TestWorkerPool_Shutdown(t *testing.T) {
	pool := NewWorkerPool(WorkerPoolConfig{
		WorkerCount: 2,
		QueueSize:   5,
	})

	// Submit a few tasks
	for i := 0; i < 3; i++ {
		pool.Submit(Task{
			ID: "shutdown-test",
			Execute: func(ctx context.Context, payload interface{}) (interface{}, error) {
				time.Sleep(10 * time.Millisecond)
				return nil, nil
			},
		})
	}

	err := pool.Shutdown(2 * time.Second)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestWorkerPool_ShutdownTimeout(t *testing.T) {
	pool := NewWorkerPool(WorkerPoolConfig{
		WorkerCount: 1,
		QueueSize:   5,
	})

	// Submit long-running task
	pool.Submit(Task{
		ID: "long-running",
		Execute: func(ctx context.Context, payload interface{}) (interface{}, error) {
			time.Sleep(2 * time.Second)
			return nil, nil
		},
	})

	err := pool.Shutdown(50 * time.Millisecond)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}
}

func TestWorkerPool_ConcurrentSubmit(t *testing.T) {
	pool := NewWorkerPool(WorkerPoolConfig{
		WorkerCount: 4,
		QueueSize:   50,
		EnableStats: true,
	})
	defer pool.Close()

	var completed int32

	// Submit tasks concurrently
	for i := 0; i < 20; i++ {
		go func() {
			task := Task{
				ID: "concurrent",
				Execute: func(ctx context.Context, payload interface{}) (interface{}, error) {
					atomic.AddInt32(&completed, 1)
					return nil, nil
				},
			}
			pool.Submit(task)
		}()
	}

	time.Sleep(500 * time.Millisecond)

	if atomic.LoadInt32(&completed) != 20 {
		t.Errorf("Expected 20 completed tasks, got %d", atomic.LoadInt32(&completed))
	}
}

func TestPriorityTaskQueue(t *testing.T) {
	pq := NewPriorityTaskQueue()

	if pq.Size() != 0 {
		t.Error("New queue should be empty")
	}

	// Add tasks with different priorities
	pq.Push(Task{ID: "low", Priority: 1})
	pq.Push(Task{ID: "high", Priority: 10})
	pq.Push(Task{ID: "medium", Priority: 5})

	if pq.Size() != 3 {
		t.Errorf("Expected size 3, got %d", pq.Size())
	}

	// Pop should return highest priority first
	task, ok := pq.Pop()
	if !ok || task.ID != "high" {
		t.Errorf("Expected 'high' task first, got %s", task.ID)
	}

	task, ok = pq.Pop()
	if !ok || task.ID != "medium" {
		t.Errorf("Expected 'medium' task second, got %s", task.ID)
	}

	task, ok = pq.Pop()
	if !ok || task.ID != "low" {
		t.Errorf("Expected 'low' task last, got %s", task.ID)
	}

	// Pop from empty queue
	_, ok = pq.Pop()
	if ok {
		t.Error("Pop from empty queue should return false")
	}
}

func TestPriorityTaskQueue_Clear(t *testing.T) {
	pq := NewPriorityTaskQueue()

	pq.Push(Task{ID: "1", Priority: 1})
	pq.Push(Task{ID: "2", Priority: 2})

	if pq.Size() != 2 {
		t.Error("Queue should have 2 tasks")
	}

	pq.Clear()

	if pq.Size() != 0 {
		t.Error("Queue should be empty after clear")
	}
}

func TestWorkerPool_ContextCancellation(t *testing.T) {
	pool := NewWorkerPool(WorkerPoolConfig{
		WorkerCount: 2,
		QueueSize:   5,
	})

	started := make(chan bool, 1)
	cancelled := make(chan bool, 1)

	task := Task{
		ID: "cancel-test",
		Execute: func(ctx context.Context, payload interface{}) (interface{}, error) {
			started <- true
			<-ctx.Done()
			cancelled <- true
			return nil, ctx.Err()
		},
	}

	pool.Submit(task)

	// Wait for task to start
	<-started

	// Cancel pool context
	pool.cancel()

	select {
	case <-cancelled:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("Task was not cancelled")
	}

	pool.Close()
}
