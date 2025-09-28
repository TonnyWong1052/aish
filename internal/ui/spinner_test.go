package ui

import (
	"testing"
	"time"
)

func TestNewAnimatedSpinner(t *testing.T) {
	testMessage := "Loading test"
	testStyle := StyleSpinner

	spinner := NewAnimatedSpinner(testMessage, testStyle)

	if spinner == nil {
		t.Fatal("NewAnimatedSpinner should return a non-nil spinner")
	}

	if spinner.message != testMessage {
		t.Errorf("Expected message '%s', got '%s'", testMessage, spinner.message)
	}

	if spinner.style != testStyle {
		t.Errorf("Expected style %v, got %v", testStyle, spinner.style)
	}

	if spinner.isRunning {
		t.Error("New spinner should not be running initially")
	}

	if spinner.ctx != nil {
		t.Error("New spinner should have nil context initially")
	}

	if spinner.cancel != nil {
		t.Error("New spinner should have nil cancel function initially")
	}
}

func TestAnimatedSpinnerStartStop(t *testing.T) {
	spinner := NewAnimatedSpinner("Test loading", StyleSpinner)

	// Test Start
	spinner.Start()

	if !spinner.IsRunning() {
		t.Error("Spinner should be running after Start()")
	}

	if spinner.ctx == nil {
		t.Error("Context should be set after Start()")
	}

	if spinner.cancel == nil {
		t.Error("Cancel function should be set after Start()")
	}

	if spinner.startTime.IsZero() {
		t.Error("Start time should be set after Start()")
	}

	// Give the spinner a moment to run
	time.Sleep(100 * time.Millisecond)

	// Test Stop with success
	spinner.Stop(true)

	if spinner.IsRunning() {
		t.Error("Spinner should not be running after Stop()")
	}

	// Test that stopping again doesn't cause issues
	spinner.Stop(false) // Should not panic
}

func TestAnimatedSpinnerDoubleStart(t *testing.T) {
	spinner := NewAnimatedSpinner("Test double start", StyleDots)

	// Start the spinner
	spinner.Start()
	if !spinner.IsRunning() {
		t.Error("Spinner should be running after first Start()")
	}

	// Try to start again
	spinner.Start() // Should not create a new goroutine
	if !spinner.IsRunning() {
		t.Error("Spinner should still be running after second Start()")
	}

	spinner.Stop(true)
	if spinner.IsRunning() {
		t.Error("Spinner should be stopped")
	}
}

func TestAnimatedSpinnerStopBeforeStart(t *testing.T) {
	spinner := NewAnimatedSpinner("Test stop before start", StyleWave)

	// Stop before starting - should not panic
	spinner.Stop(true)

	if spinner.IsRunning() {
		t.Error("Spinner should not be running after stop without start")
	}
}

func TestAnimationStyles(t *testing.T) {
	styles := []AnimationStyle{
		StyleSpinner,
		StyleDots,
		StyleWave,
		StyleProgress,
	}

	for _, style := range styles {
		t.Run(string(rune(int(style)+'0')), func(t *testing.T) {
			spinner := NewAnimatedSpinner("Testing style", style)

			if spinner.style != style {
				t.Errorf("Expected style %v, got %v", style, spinner.style)
			}

			// Test that each style can start and stop without issues
			spinner.Start()
			time.Sleep(50 * time.Millisecond) // Brief run
			spinner.Stop(true)

			if spinner.IsRunning() {
				t.Error("Spinner should be stopped after test")
			}
		})
	}
}

func TestAnimatedSpinnerUpdateMessage(t *testing.T) {
	originalMessage := "Original message"
	newMessage := "Updated message"

	spinner := NewAnimatedSpinner(originalMessage, StyleSpinner)

	if spinner.message != originalMessage {
		t.Errorf("Expected original message '%s', got '%s'", originalMessage, spinner.message)
	}

	// Update message while not running
	spinner.UpdateMessage(newMessage)
	if spinner.message != newMessage {
		t.Errorf("Expected updated message '%s', got '%s'", newMessage, spinner.message)
	}

	// Update message while running
	spinner.Start()
	spinner.UpdateMessage("Running message")
	if spinner.message != "Running message" {
		t.Error("Message should be updated while running")
	}

	spinner.Stop(true)
}

func TestAnimatedSpinnerConcurrency(t *testing.T) {
	spinner := NewAnimatedSpinner("Concurrency test", StyleProgress)

	// Start the spinner
	spinner.Start()

	// Test concurrent operations
	done := make(chan bool, 3)

	// Goroutine 1: Check IsRunning
	go func() {
		for i := 0; i < 10; i++ {
			spinner.IsRunning()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Goroutine 2: Update message
	go func() {
		for i := 0; i < 10; i++ {
			spinner.UpdateMessage("Concurrent update")
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Goroutine 3: Try multiple starts
	go func() {
		for i := 0; i < 5; i++ {
			spinner.Start() // Should be safe to call multiple times
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	spinner.Stop(true)

	if spinner.IsRunning() {
		t.Error("Spinner should be stopped after concurrency test")
	}
}

func TestAnimatedSpinnerStartTime(t *testing.T) {
	spinner := NewAnimatedSpinner("Time test", StyleSpinner)

	beforeStart := time.Now()
	spinner.Start()
	afterStart := time.Now()

	if spinner.startTime.Before(beforeStart) || spinner.startTime.After(afterStart) {
		t.Error("Start time should be set between beforeStart and afterStart")
	}

	// Let it run for a bit
	time.Sleep(50 * time.Millisecond)

	spinner.Stop(true)
}

func TestAnimatedSpinnerContextCancellation(t *testing.T) {
	spinner := NewAnimatedSpinner("Context test", StyleDots)

	spinner.Start()

	// Get the context
	ctx := spinner.ctx
	if ctx == nil {
		t.Fatal("Context should be set after Start()")
	}

	// Check that context is not cancelled initially
	select {
	case <-ctx.Done():
		t.Error("Context should not be cancelled initially")
	default:
		// Good, context is not cancelled
	}

	// Stop the spinner, which should cancel the context
	spinner.Stop(true)

	// Verify context is cancelled
	select {
	case <-ctx.Done():
		// Good, context is cancelled
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be cancelled after Stop()")
	}
}

func TestAnimationStyleConstants(t *testing.T) {
	// Test that all style constants are defined and unique
	styles := []AnimationStyle{
		StyleSpinner,
		StyleDots,
		StyleWave,
		StyleProgress,
	}

	// Check that values are consecutive starting from 0
	for i, style := range styles {
		if int(style) != i {
			t.Errorf("Expected style %d to have value %d, got %d", i, i, int(style))
		}
	}

	// Test that each style produces different spinners
	for i, style := range styles {
		spinner := NewAnimatedSpinner("Test", style)
		if spinner.style != style {
			t.Errorf("Style %d not properly set", i)
		}
	}
}
