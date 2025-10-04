package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsInteractiveTTY(t *testing.T) {
	// Save original stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	// Create a pipe to simulate non-TTY stdin
	r, w, _ := os.Pipe()
	defer r.Close()
	defer w.Close()
	os.Stdin = r

	result := isInteractiveTTY()
	// In test environment, this should be false
	assert.False(t, result, "isInteractiveTTY should return false for pipe stdin")
}

func TestInitCmdFlags(t *testing.T) {
	// Test that init command has the expected flags
	assert.NotNil(t, initCmd, "initCmd should be initialized")

	flags := initCmd.Flags()
	assert.NotNil(t, flags, "initCmd should have flags")

	// Check for --reset flag
	resetFlag := flags.Lookup("reset")
	assert.NotNil(t, resetFlag, "initCmd should have --reset flag")
	assert.Equal(t, "bool", resetFlag.Value.Type(), "--reset should be a boolean flag")
}

func TestInitCmdBasicProperties(t *testing.T) {
	assert.Equal(t, "init", initCmd.Use)
	assert.NotEmpty(t, initCmd.Short)
	assert.NotEmpty(t, initCmd.Long)
	assert.NotNil(t, initCmd.Run)
}

func TestInitCmdBasicExecution(t *testing.T) {
	// Test command properties
	assert.Equal(t, "init", initCmd.Use)
	assert.Contains(t, initCmd.Short, "Initializes aish")

	// Note: Full init command execution requires mocking:
	// - shell.InstallHook()
	// - ui.RunWizard() or ui.RunTextWizard()
	// These are integration tests better suited for E2E testing
	t.Log("Full init command execution requires shell and UI mocking")
}
