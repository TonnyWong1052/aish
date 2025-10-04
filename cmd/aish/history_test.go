package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHistoryCmd(t *testing.T) {
	// Test that historyCmd exists and has proper structure
	assert.NotNil(t, historyCmd)
	assert.Equal(t, "history", historyCmd.Use)
	assert.NotEmpty(t, historyCmd.Short)
	assert.NotEmpty(t, historyCmd.Long)
	assert.NotNil(t, historyCmd.Run)
}

func TestHistoryClearCmd(t *testing.T) {
	// Test that historyClearCmd exists and has proper structure
	assert.NotNil(t, historyClearCmd)
	assert.Equal(t, "clear", historyClearCmd.Use)
	assert.NotEmpty(t, historyClearCmd.Short)
	assert.NotNil(t, historyClearCmd.Run)
}

func TestHistorySubcommands(t *testing.T) {
	// Test that history command has all expected subcommands
	subcommands := historyCmd.Commands()

	// Map subcommand names
	cmdNames := make(map[string]bool)
	for _, cmd := range subcommands {
		cmdNames[cmd.Use] = true
	}

	// Verify expected subcommands exist
	expectedCmds := []string{"clear"}
	for _, expected := range expectedCmds {
		assert.True(t, cmdNames[expected], "Expected subcommand %s not found", expected)
	}
}

// Note: Full testing of listHistoryAndAnalyze requires mocking:
// - history.Load()
// - pterm interactive select
// - LLM provider interactions
// These are integration tests better suited for E2E testing
