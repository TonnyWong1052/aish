package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ProviderConfig stores the configuration for a single LLM provider.
type ProviderConfig struct {
	APIEndpoint  string `json:"api_endpoint"`
	APIKey       string `json:"api_key"`
	Model        string `json:"model"`
	Project      string `json:"project,omitempty"`        // For Gemini-CLI
	OmitV1Prefix bool   `json:"omit_v1_prefix,omitempty"` // For OpenAI-compatible APIs that do not use the /v1 prefix
}

// ContextConfig defines configuration options for the context enhancer.
type ContextConfig struct {
	MaxHistoryEntries  int  `json:"max_history_entries"`  // Max number of history entries (default 10)
	IncludeDirectories bool `json:"include_directories"`  // Whether to include directory listings (default true)
	FilterSensitiveCmd bool `json:"filter_sensitive_cmd"` // Whether to filter sensitive commands (default true)
	EnableEnhanced     bool `json:"enable_enhanced"`      // Whether to enable enhanced context analysis (default true)
}

// LoggingConfig defines logging configuration options.
type LoggingConfig struct {
	Level      string `json:"level"`       // Log level: trace, debug, info, warn, error, fatal, panic
	Format     string `json:"format"`      // Format: json, text
	Output     string `json:"output"`      // Output: file, console, both
	LogFile    string `json:"log_file"`    // Log file path
	MaxSize    int64  `json:"max_size"`    // Max file size (MB)
	MaxBackups int    `json:"max_backups"` // Max number of backup files
}

// CacheConfig defines cache configuration options.
type CacheConfig struct {
	Enabled             bool    `json:"enabled"`              // Whether to enable caching
	MaxEntries          int     `json:"max_entries"`          // Max number of cache entries
	DefaultTTLHours     int     `json:"default_ttl_hours"`    // Default TTL in hours
	SuggestionTTLHours  int     `json:"suggestion_ttl_hours"` // Suggestion cache TTL in hours
	CommandTTLHours     int     `json:"command_ttl_hours"`    // Command cache TTL in hours
	EnableSimilarity    bool    `json:"enable_similarity"`    // Enable similarity matching
	SimilarityThreshold float64 `json:"similarity_threshold"` // Similarity threshold
	MaxSimilarityCache  int     `json:"max_similarity_cache"` // Max entries for similarity cache
}

// UserPreferences stores user-specific settings.
type UserPreferences struct {
	Language           string        `json:"language"`
	EnabledLLMTriggers []string      `json:"enabled_llm_triggers"`
	AutoExecute        bool          `json:"auto_execute"` // Automatically execute generated commands without user confirmation
	Context            ContextConfig `json:"context"`
	Logging            LoggingConfig `json:"logging"`
	Cache              CacheConfig   `json:"cache"`
	MaxHistorySize     int           `json:"max_history_size"`

	// Core AISH settings
	ShowTips      bool `json:"show_tips"`      // Display helpful tips during usage
	VerboseOutput bool `json:"verbose_output"` // Show detailed diagnostic information
}

// Config is the main configuration structure for the application.
type Config struct {
	Enabled         bool                      `json:"enabled"`
	DefaultProvider string                    `json:"default_provider"`
	Providers       map[string]ProviderConfig `json:"providers"`
	UserPreferences UserPreferences           `json:"user_preferences"`
}

// GetConfigPath returns the full path to the configuration file.
func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, DefaultConfigDir, DefaultConfigFileName), nil
}

// Load reads the configuration from the file, or returns a default config.
func newDefaultConfig() *Config {
	return &Config{
		Enabled:         true,
		DefaultProvider: ProviderOpenAI,
		Providers: map[string]ProviderConfig{
			ProviderOpenAI:    {APIEndpoint: OpenAIAPIEndpoint, APIKey: "", Model: DefaultOpenAIModel},
			ProviderGemini:    {APIEndpoint: GeminiAPIEndpoint, APIKey: "YOUR_GEMINI_API_KEY", Model: DefaultGeminiModel},
			ProviderGeminiCLI: {APIEndpoint: GeminiCLIAPIEndpoint, Project: "YOUR_GEMINI_PROJECT_ID", Model: DefaultGeminiCLIModel},
			ProviderClaude:    {APIEndpoint: ClaudeAPIEndpoint, APIKey: "", Model: DefaultClaudeModel},
			ProviderOllama:    {APIEndpoint: OllamaAPIEndpoint, APIKey: "", Model: DefaultOllamaModel},
		},
		UserPreferences: UserPreferences{
			Language: "en",
			EnabledLLMTriggers: []string{
				"CommandNotFound",
				"FileNotFoundOrDirectory",
				"PermissionDenied",
				"CannotExecute",
				"InvalidArgumentOrOption",
				"ResourceExists",
				"NotADirectory",
				"TerminatedBySignal",
				"GenericError",
				"NetworkError",
				"DatabaseError",
				"ConfigError",
				"DependencyError",
				"TimeoutError",
				"MemoryError",
				"DiskSpaceError",
				"PermissionError",
				"AuthenticationError",
				"InteractiveToolUsage",
			},
			AutoExecute: false, // Default to false, require user to enable manually
			Context: ContextConfig{
				MaxHistoryEntries:  DefaultMaxHistoryEntries,
				IncludeDirectories: true,
				FilterSensitiveCmd: true,
				EnableEnhanced:     true,
			},
			Logging: LoggingConfig{
				Level:      LogLevelInfo,
				Format:     LogFormatText,
				Output:     LogOutputFile,
				LogFile:    "", // Will be set at runtime
				MaxSize:    MaxLogFileSize,
				MaxBackups: DefaultMaxBackups,
			},
			Cache: CacheConfig{
				Enabled:             true,
				MaxEntries:          DefaultCacheEntries,
				DefaultTTLHours:     DefaultCacheTTLHours,
				SuggestionTTLHours:  DefaultSuggestionTTLHours,
				CommandTTLHours:     DefaultCommandTTLHours,
				EnableSimilarity:    true,
				SimilarityThreshold: DefaultSimilarityThreshold,
				MaxSimilarityCache:  DefaultMaxSimilarityCache,
			},
			MaxHistorySize: DefaultMaxHistorySize,

			// Core AISH settings defaults
			ShowTips:      true,
			VerboseOutput: false,
		},
	}
}

func Load() (*Config, error) {
	// Use new migration loading system, fallback to legacy format loading for compatibility
	cfg, migrationResult, err := LoadWithMigration()
	if err != nil {
		// Fallback to legacy config reading
		legacyCfg, legacyErr := LoadLegacy()
		if legacyErr != nil {
			return nil, err
		}
		// Try to upgrade legacy config and save as versioned new format (ignore failures)
		if path, perr := GetConfigPath(); perr == nil {
			migrator := NewMigrator(path)
			_ = migrator.saveVersionedConfig(legacyCfg, CurrentVersion)
		}
		cfg = legacyCfg
		migrationResult = nil
	}

	// If migration was performed, can log information (silently handled for now)
	if migrationResult != nil {
		// Can add logging or user notification here
		// Example: fmt.Printf("Config migrated from version %s to %s\n", migrationResult.FromVersion, migrationResult.ToVersion)
		_ = migrationResult // Suppress unused variable warning
	}

	// Execute config validation and auto-fix
	fixes, err := cfg.ValidateAndFix()
	if err != nil {
		return nil, err
	}

	// If there are auto-fixes, save the config
	if len(fixes) > 0 {
		if err := cfg.Save(); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

// LoadLegacy keeps the old loading method as backup
func LoadLegacy() (*Config, error) {
	path, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	var cfg Config

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Config file does not exist, so create a default one.
		cfg = *newDefaultConfig()
	} else {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	}

	// Set default log file path (if not set)
	if cfg.UserPreferences.Logging.LogFile == "" {
		home, _ := os.UserHomeDir()
		cfg.UserPreferences.Logging.LogFile = filepath.Join(home, DefaultConfigDir, DefaultLogDir, DefaultLogFileName)
	}

	// If it's a newly created config, save it
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := cfg.Save(); err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}

// Save writes the current configuration to the file.
func (c *Config) Save() error {
	// Placeholder implementation
	path, err := GetConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
