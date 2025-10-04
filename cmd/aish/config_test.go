package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TonnyWong1052/aish/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a test config directory
func setupTestConfig(t *testing.T, cfg *config.Config) (cleanup func()) {
	// Create temp home directory
	tmpHome := t.TempDir()
	originalHome := os.Getenv("HOME")

	// Set HOME to temp directory
	os.Setenv("HOME", tmpHome)

	// Create config directory
	configDir := filepath.Join(tmpHome, config.DefaultConfigDir)
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	// Save config
	if cfg != nil {
		require.NoError(t, cfg.Save())
	}

	// Return cleanup function
	return func() {
		os.Setenv("HOME", originalHome)
	}
}

func TestConfigShowCmd(t *testing.T) {
	// Set up test config
	cfg := &config.Config{
		DefaultProvider: "openai",
		Providers: map[string]config.ProviderConfig{
			"openai": {
				APIEndpoint: "https://api.openai.com/v1",
				Model:       "gpt-4",
			},
		},
	}

	cleanup := setupTestConfig(t, cfg)
	defer cleanup()

	// Test that configShowCmd exists and has proper structure
	assert.NotNil(t, configShowCmd)
	assert.Equal(t, "show", configShowCmd.Use)
	assert.NotNil(t, configShowCmd.Run)
}

func TestConfigGetCmd(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
	}{
		{
			name: "get default_provider",
			cfg: &config.Config{
				DefaultProvider: "gemini",
			},
		},
		{
			name: "get auto_execute in UserPreferences",
			cfg: &config.Config{
				DefaultProvider: "openai",
				UserPreferences: config.UserPreferences{
					AutoExecute: true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupTestConfig(t, tt.cfg)
			defer cleanup()

			// Test that configGetCmd exists
			assert.NotNil(t, configGetCmd)
			assert.Contains(t, configGetCmd.Use, "get")
			assert.NotNil(t, configGetCmd.Run)
		})
	}
}

func TestConfigSetCmd(t *testing.T) {
	// Test that configSetCmd exists and has proper structure
	assert.NotNil(t, configSetCmd)
	assert.Contains(t, configSetCmd.Use, "set")
	assert.NotNil(t, configSetCmd.Run)
}

func TestConfigCmd(t *testing.T) {
	// Test that configCmd exists and has proper structure
	assert.NotNil(t, configCmd)
	assert.Equal(t, "config", configCmd.Use)
	assert.NotEmpty(t, configCmd.Short)
	assert.NotNil(t, configCmd.Run)
}

func TestConfigSubcommands(t *testing.T) {
	// Test that config command has all expected subcommands
	subcommands := configCmd.Commands()

	// Map subcommand names (extract first word from Use field)
	cmdNames := make(map[string]bool)
	for _, cmd := range subcommands {
		// Extract first word from Use (e.g., "get [key]" -> "get")
		parts := strings.Fields(cmd.Use)
		if len(parts) > 0 {
			cmdNames[parts[0]] = true
		}
	}

	// Verify expected subcommands exist
	expectedCmds := []string{"show", "get", "set"}
	for _, expected := range expectedCmds {
		assert.True(t, cmdNames[expected], "Expected subcommand %s not found", expected)
	}
}
