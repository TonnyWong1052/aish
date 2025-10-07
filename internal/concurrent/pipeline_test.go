package concurrent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewPipeline(t *testing.T) {
	config := PipelineConfig{
		BufferSize:     10,
		EnableMetrics:  true,
		DefaultTimeout: 5 * time.Second,
	}

	pipeline := NewPipeline(config)
	if pipeline == nil {
		t.Fatal("Pipeline should not be nil")
	}

	if len(pipeline.stages) != 0 {
		t.Error("New pipeline should have no stages")
	}
}

func TestPipeline_AddStage(t *testing.T) {
	pipeline := NewPipeline(PipelineConfig{})

	stage := PipelineStage{
		Name: "test-stage",
		Processor: func(ctx context.Context, data interface{}) (interface{}, error) {
			return data, nil
		},
		Parallel: false,
	}

	pipeline.AddStage(stage)

	if len(pipeline.stages) != 1 {
		t.Errorf("Expected 1 stage, got %d", len(pipeline.stages))
	}
}

func TestPipeline_Process(t *testing.T) {
	pipeline := NewPipeline(PipelineConfig{})

	// Add stage that transforms string to uppercase
	pipeline.AddStage(PipelineStage{
		Name: "uppercase",
		Processor: func(ctx context.Context, data interface{}) (interface{}, error) {
			str := data.(string)
			return strings.ToUpper(str), nil
		},
	})

	// Add stage that appends suffix
	pipeline.AddStage(PipelineStage{
		Name: "suffix",
		Processor: func(ctx context.Context, data interface{}) (interface{}, error) {
			str := data.(string)
			return str + "_PROCESSED", nil
		},
	})

	result, err := pipeline.Process(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	expected := "HELLO_PROCESSED"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}

	pipeline.Close()
}

func TestPipeline_ProcessError(t *testing.T) {
	pipeline := NewPipeline(PipelineConfig{})

	pipeline.AddStage(PipelineStage{
		Name: "success",
		Processor: func(ctx context.Context, data interface{}) (interface{}, error) {
			return "stage1", nil
		},
	})

	expectedError := errors.New("stage error")
	pipeline.AddStage(PipelineStage{
		Name: "error",
		Processor: func(ctx context.Context, data interface{}) (interface{}, error) {
			return nil, expectedError
		},
	})

	pipeline.AddStage(PipelineStage{
		Name: "unreachable",
		Processor: func(ctx context.Context, data interface{}) (interface{}, error) {
			t.Error("This stage should not be reached")
			return nil, nil
		},
	})

	_, err := pipeline.Process(context.Background(), "input")
	if err != expectedError {
		t.Errorf("Expected error %v, got %v", expectedError, err)
	}

	pipeline.Close()
}

func TestPipeline_ProcessBatch(t *testing.T) {
	pipeline := NewPipeline(PipelineConfig{})

	pipeline.AddStage(PipelineStage{
		Name: "multiply",
		Processor: func(ctx context.Context, data interface{}) (interface{}, error) {
			num := data.(int)
			return num * 2, nil
		},
	})

	items := []interface{}{1, 2, 3, 4, 5}
	results, errors := pipeline.ProcessBatch(context.Background(), items)

	if len(results) != 5 {
		t.Errorf("Expected 5 results, got %d", len(results))
	}

	for i, result := range results {
		if result != items[i].(int)*2 {
			t.Errorf("Item %d: expected %d, got %v", i, items[i].(int)*2, result)
		}
		if errors[i] != nil {
			t.Errorf("Item %d: unexpected error %v", i, errors[i])
		}
	}

	pipeline.Close()
}

func TestPipeline_GetStats(t *testing.T) {
	pipeline := NewPipeline(PipelineConfig{EnableMetrics: true})

	pipeline.AddStage(PipelineStage{
		Name: "test",
		Processor: func(ctx context.Context, data interface{}) (interface{}, error) {
			time.Sleep(10 * time.Millisecond)
			return data, nil
		},
	})

	// Process a few items
	for i := 0; i < 3; i++ {
		pipeline.Process(context.Background(), i)
	}

	stats := pipeline.GetStats()

	if stats.TotalProcessed != 3 {
		t.Errorf("Expected 3 processed, got %d", stats.TotalProcessed)
	}

	if stats.TotalErrors != 0 {
		t.Errorf("Expected 0 errors, got %d", stats.TotalErrors)
	}

	if stats.AvgLatency == 0 {
		t.Error("Average latency should not be zero")
	}

	pipeline.Close()
}

func TestPipeline_StageWithTimeout(t *testing.T) {
	pipeline := NewPipeline(PipelineConfig{})

	pipeline.AddStage(PipelineStage{
		Name:    "slow",
		Timeout: 50 * time.Millisecond,
		Processor: func(ctx context.Context, data interface{}) (interface{}, error) {
			select {
			case <-time.After(200 * time.Millisecond):
				return "done", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	})

	_, err := pipeline.Process(context.Background(), "input")
	if err == nil {
		t.Error("Expected timeout error")
	}

	pipeline.Close()
}

func TestPipelineBuilder(t *testing.T) {
	builder := NewPipelineBuilder(PipelineConfig{})

	pipeline := builder.
		AddSerialStage("stage1", func(ctx context.Context, data interface{}) (interface{}, error) {
			return data.(int) + 1, nil
		}).
		AddSerialStage("stage2", func(ctx context.Context, data interface{}) (interface{}, error) {
			return data.(int) * 2, nil
		}).
		WithTimeout(5 * time.Second).
		Build()

	result, err := pipeline.Process(context.Background(), 5)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// (5 + 1) * 2 = 12
	if result != 12 {
		t.Errorf("Expected 12, got %v", result)
	}

	pipeline.Close()
}

func TestPipelineBuilder_ParallelStage(t *testing.T) {
	builder := NewPipelineBuilder(PipelineConfig{})

	pipeline := builder.
		AddParallelStage("parallel", func(ctx context.Context, data interface{}) (interface{}, error) {
			return data, nil
		}, 2).
		Build()

	result, err := pipeline.Process(context.Background(), "test")
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if result != "test" {
		t.Errorf("Expected 'test', got %v", result)
	}

	pipeline.Close()
}

func TestPipeline_StageStats(t *testing.T) {
	pipeline := NewPipeline(PipelineConfig{EnableMetrics: true})

	pipeline.AddStage(PipelineStage{
		Name: "tracked",
		Processor: func(ctx context.Context, data interface{}) (interface{}, error) {
			time.Sleep(5 * time.Millisecond)
			return data, nil
		},
	})

	// Process multiple times
	for i := 0; i < 5; i++ {
		pipeline.Process(context.Background(), i)
	}

	stats := pipeline.GetStats()
	stageStats, exists := stats.StageStats["tracked"]

	if !exists {
		t.Fatal("Stage stats should exist")
	}

	if stageStats.Processed != 5 {
		t.Errorf("Expected 5 processed in stage, got %d", stageStats.Processed)
	}

	if stageStats.AvgTime == 0 {
		t.Error("Average time should not be zero")
	}

	pipeline.Close()
}

func TestPipeline_ContextCancellation(t *testing.T) {
	pipeline := NewPipeline(PipelineConfig{})

	pipeline.AddStage(PipelineStage{
		Name: "cancellable",
		Processor: func(ctx context.Context, data interface{}) (interface{}, error) {
			select {
			case <-time.After(1 * time.Second):
				return "done", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := pipeline.Process(ctx, "input")
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	pipeline.Close()
}
