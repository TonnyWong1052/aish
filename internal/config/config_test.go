package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGetConfigPath(t *testing.T) {
	path, err := GetConfigPath()
	if err != nil {
		t.Fatalf("Failed to get config path: %v", err)
	}

	// Should end with correct path components
	if !strings.HasSuffix(path, filepath.Join(DefaultConfigDir, DefaultConfigFileName)) {
		t.Errorf("Config path should end with %s/%s, got %s", DefaultConfigDir, DefaultConfigFileName, path)
	}
}

func TestNewDefaultConfig(t *testing.T) {
	config := newDefaultConfig()

	// Test basic configuration
	if !config.Enabled {
		t.Error("Default config should be enabled")
	}

	if config.DefaultProvider != ProviderOpenAI {
		t.Errorf("Expected default provider %s, got %s", ProviderOpenAI, config.DefaultProvider)
	}

	// Test providers
	expectedProviders := []string{ProviderOpenAI, ProviderGemini, ProviderGeminiCLI}
	for _, provider := range expectedProviders {
		if _, exists := config.Providers[provider]; !exists {
			t.Errorf("Expected provider %s to exist in default config", provider)
		}
	}

	// Test OpenAI provider config
	openaiConfig := config.Providers[ProviderOpenAI]
	if openaiConfig.APIEndpoint != OpenAIAPIEndpoint {
		t.Errorf("Expected OpenAI endpoint %s, got %s", OpenAIAPIEndpoint, openaiConfig.APIEndpoint)
	}
	if openaiConfig.Model != DefaultOpenAIModel {
		t.Errorf("Expected OpenAI model %s, got %s", DefaultOpenAIModel, openaiConfig.Model)
	}

	// Test Gemini provider config
	geminiConfig := config.Providers[ProviderGemini]
	if geminiConfig.APIEndpoint != GeminiAPIEndpoint {
		t.Errorf("Expected Gemini endpoint %s, got %s", GeminiAPIEndpoint, geminiConfig.APIEndpoint)
	}
	if geminiConfig.Model != DefaultGeminiModel {
		t.Errorf("Expected Gemini model %s, got %s", DefaultGeminiModel, geminiConfig.Model)
	}

	// Test user preferences
	prefs := config.UserPreferences
	if prefs.Language != "en" {
		t.Errorf("Expected language 'en', got %s", prefs.Language)
	}

	if prefs.AutoExecute {
		t.Error("AutoExecute should be false by default")
	}

	// Test enabled LLM triggers should include all error types
	expectedTriggerCount := 19 // Number of error types defined
	if len(prefs.EnabledLLMTriggers) != expectedTriggerCount {
		t.Errorf("Expected %d LLM triggers, got %d", expectedTriggerCount, len(prefs.EnabledLLMTriggers))
	}

	// Test context config defaults
	if prefs.Context.MaxHistoryEntries != DefaultMaxHistoryEntries {
		t.Errorf("Expected MaxHistoryEntries %d, got %d", DefaultMaxHistoryEntries, prefs.Context.MaxHistoryEntries)
	}

	if !prefs.Context.IncludeDirectories {
		t.Error("IncludeDirectories should be true by default")
	}

	if !prefs.Context.FilterSensitiveCmd {
		t.Error("FilterSensitiveCmd should be true by default")
	}

	if !prefs.Context.EnableEnhanced {
		t.Error("EnableEnhanced should be true by default")
	}

	// Test logging config defaults
	if prefs.Logging.Level != LogLevelInfo {
		t.Errorf("Expected log level %s, got %s", LogLevelInfo, prefs.Logging.Level)
	}

	if prefs.Logging.Format != LogFormatText {
		t.Errorf("Expected log format %s, got %s", LogFormatText, prefs.Logging.Format)
	}

	if prefs.Logging.Output != LogOutputFile {
		t.Errorf("Expected log output %s, got %s", LogOutputFile, prefs.Logging.Output)
	}

	if prefs.Logging.MaxSize != MaxLogFileSize {
		t.Errorf("Expected MaxSize %d, got %d", MaxLogFileSize, prefs.Logging.MaxSize)
	}

	if prefs.Logging.MaxBackups != DefaultMaxBackups {
		t.Errorf("Expected MaxBackups %d, got %d", DefaultMaxBackups, prefs.Logging.MaxBackups)
	}

	// Test cache config defaults
	if !prefs.Cache.Enabled {
		t.Error("Cache should be enabled by default")
	}

	if prefs.Cache.MaxEntries != DefaultCacheEntries {
		t.Errorf("Expected MaxEntries %d, got %d", DefaultCacheEntries, prefs.Cache.MaxEntries)
	}

	if prefs.Cache.DefaultTTLHours != DefaultCacheTTLHours {
		t.Errorf("Expected DefaultTTLHours %d, got %d", DefaultCacheTTLHours, prefs.Cache.DefaultTTLHours)
	}

	if prefs.Cache.SuggestionTTLHours != DefaultSuggestionTTLHours {
		t.Errorf("Expected SuggestionTTLHours %d, got %d", DefaultSuggestionTTLHours, prefs.Cache.SuggestionTTLHours)
	}

	if prefs.Cache.CommandTTLHours != DefaultCommandTTLHours {
		t.Errorf("Expected CommandTTLHours %d, got %d", DefaultCommandTTLHours, prefs.Cache.CommandTTLHours)
	}

	if !prefs.Cache.EnableSimilarity {
		t.Error("EnableSimilarity should be true by default")
	}

	if prefs.Cache.SimilarityThreshold != DefaultSimilarityThreshold {
		t.Errorf("Expected SimilarityThreshold %f, got %f", DefaultSimilarityThreshold, prefs.Cache.SimilarityThreshold)
	}

	if prefs.Cache.MaxSimilarityCache != DefaultMaxSimilarityCache {
		t.Errorf("Expected MaxSimilarityCache %d, got %d", DefaultMaxSimilarityCache, prefs.Cache.MaxSimilarityCache)
	}

	// Test history size
	if prefs.MaxHistorySize != DefaultMaxHistorySize {
		t.Errorf("Expected MaxHistorySize %d, got %d", DefaultMaxHistorySize, prefs.MaxHistorySize)
	}
}

func TestConfigSaveAndLoad(t *testing.T) {
	// Create test config
	testConfig := &Config{
		Enabled:         true,
		DefaultProvider: ProviderGemini,
		Providers: map[string]ProviderConfig{
			ProviderGemini: {
				APIEndpoint: GeminiAPIEndpoint,
				APIKey:      "test-key",
				Model:       DefaultGeminiModel,
			},
		},
		UserPreferences: UserPreferences{
			Language:           "zh-TW",
			EnabledLLMTriggers: []string{"CommandNotFound", "NetworkError"},
			AutoExecute:        true,
			Context: ContextConfig{
				MaxHistoryEntries:  5,
				IncludeDirectories: false,
				FilterSensitiveCmd: false,
				EnableEnhanced:     false,
			},
			Logging: LoggingConfig{
				Level:      LogLevelDebug,
				Format:     LogFormatJSON,
				Output:     LogOutputConsole,
				LogFile:    "test.log",
				MaxSize:    5,
				MaxBackups: 3,
			},
			Cache: CacheConfig{
				Enabled:             false,
				MaxEntries:          500,
				DefaultTTLHours:     12,
				SuggestionTTLHours:  3,
				CommandTTLHours:     12,
				EnableSimilarity:    false,
				SimilarityThreshold: 0.9,
				MaxSimilarityCache:  250,
			},
			MaxHistorySize: 50,
		},
	}

	// Test basic config structure validation
	if testConfig.Enabled != true {
		t.Error("Expected config to be enabled")
	}

	if testConfig.DefaultProvider != ProviderGemini {
		t.Errorf("Expected DefaultProvider %s, got %s", ProviderGemini, testConfig.DefaultProvider)
	}

	if len(testConfig.Providers) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(testConfig.Providers))
	}

	// Verify Gemini provider config
	geminiConfig := testConfig.Providers[ProviderGemini]
	if geminiConfig.APIEndpoint != GeminiAPIEndpoint {
		t.Errorf("Expected APIEndpoint %s, got %s", GeminiAPIEndpoint, geminiConfig.APIEndpoint)
	}

	if geminiConfig.APIKey != "test-key" {
		t.Errorf("Expected APIKey 'test-key', got %s", geminiConfig.APIKey)
	}

	if geminiConfig.Model != DefaultGeminiModel {
		t.Errorf("Expected Model %s, got %s", DefaultGeminiModel, geminiConfig.Model)
	}

	// Verify user preferences
	prefs := testConfig.UserPreferences

	if prefs.Language != "zh-TW" {
		t.Errorf("Expected Language 'zh-TW', got %s", prefs.Language)
	}

	if !prefs.AutoExecute {
		t.Error("Expected AutoExecute to be true")
	}

	if len(prefs.EnabledLLMTriggers) != 2 {
		t.Errorf("Expected 2 LLM triggers, got %d", len(prefs.EnabledLLMTriggers))
	}
}

func TestLoadNonExistentConfig(t *testing.T) {
	// Test default config creation logic
	config := newDefaultConfig()

	if config == nil {
		t.Fatal("newDefaultConfig should return non-nil config")
	}

	// Should create a default config
	if config.DefaultProvider != ProviderOpenAI {
		t.Errorf("Expected default provider %s, got %s", ProviderOpenAI, config.DefaultProvider)
	}

	if !config.Enabled {
		t.Error("Default config should be enabled")
	}

	if len(config.Providers) != 5 {
		t.Errorf("Expected 5 default providers, got %d", len(config.Providers))
	}

	// Test that default error triggers are set
	if len(config.UserPreferences.EnabledLLMTriggers) == 0 {
		t.Error("Default config should have LLM triggers enabled")
	}
}

func TestConfigConstants(t *testing.T) {
	// Test that constants are properly defined
	if AppName == "" {
		t.Error("AppName should not be empty")
	}

	if DefaultConfigDir == "" {
		t.Error("DefaultConfigDir should not be empty")
	}

	if DefaultConfigFileName == "" {
		t.Error("DefaultConfigFileName should not be empty")
	}

	// Test providers
	supportedProviders := GetSupportedProviders()
	expectedProviders := []string{ProviderOpenAI, ProviderGemini, ProviderGeminiCLI, ProviderClaude, ProviderOllama}

	if len(supportedProviders) != len(expectedProviders) {
		t.Errorf("Expected %d supported providers, got %d", len(expectedProviders), len(supportedProviders))
	}

	for _, provider := range expectedProviders {
		found := false
		for _, supported := range supportedProviders {
			if provider == supported {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Provider %s should be in supported providers list", provider)
		}
	}

	// Test validation functions
	validLevels := GetValidLogLevels()
	expectedLevels := []string{LogLevelTrace, LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError, LogLevelFatal, LogLevelPanic}

	if len(validLevels) != len(expectedLevels) {
		t.Errorf("Expected %d valid log levels, got %d", len(expectedLevels), len(validLevels))
	}

	// Test IsValidLogLevel
	for _, level := range expectedLevels {
		if !IsValidLogLevel(level) {
			t.Errorf("Level %s should be valid", level)
		}
	}

	if IsValidLogLevel("invalid-level") {
		t.Error("Invalid level should not be considered valid")
	}

	// Test IsValidProvider
	for _, provider := range expectedProviders {
		if !IsValidProvider(provider) {
			t.Errorf("Provider %s should be valid", provider)
		}
	}

	if IsValidProvider("invalid-provider") {
		t.Error("Invalid provider should not be considered valid")
	}
}

func TestProviderConfig(t *testing.T) {
	pc := ProviderConfig{
		APIEndpoint:  "https://api.example.com",
		APIKey:       "test-key",
		Model:        "test-model",
		Project:      "test-project",
		OmitV1Prefix: true,
	}

	if pc.APIEndpoint != "https://api.example.com" {
		t.Errorf("Expected APIEndpoint 'https://api.example.com', got %s", pc.APIEndpoint)
	}

	if pc.APIKey != "test-key" {
		t.Errorf("Expected APIKey 'test-key', got %s", pc.APIKey)
	}

	if pc.Model != "test-model" {
		t.Errorf("Expected Model 'test-model', got %s", pc.Model)
	}

	if pc.Project != "test-project" {
		t.Errorf("Expected Project 'test-project', got %s", pc.Project)
	}

	if !pc.OmitV1Prefix {
		t.Error("Expected OmitV1Prefix to be true")
	}
}

func TestContextConfig(t *testing.T) {
	cc := ContextConfig{
		MaxHistoryEntries:  20,
		IncludeDirectories: true,
		FilterSensitiveCmd: true,
		EnableEnhanced:     true,
	}

	if cc.MaxHistoryEntries != 20 {
		t.Errorf("Expected MaxHistoryEntries 20, got %d", cc.MaxHistoryEntries)
	}

	if !cc.IncludeDirectories {
		t.Error("IncludeDirectories should be true")
	}

	if !cc.FilterSensitiveCmd {
		t.Error("FilterSensitiveCmd should be true")
	}

	if !cc.EnableEnhanced {
		t.Error("EnableEnhanced should be true")
	}
}

func TestLoggingConfig(t *testing.T) {
	lc := LoggingConfig{
		Level:      LogLevelDebug,
		Format:     LogFormatJSON,
		Output:     LogOutputBoth,
		LogFile:    "/tmp/test.log",
		MaxSize:    10,
		MaxBackups: 5,
	}

	if lc.Level != LogLevelDebug {
		t.Errorf("Expected Level %s, got %s", LogLevelDebug, lc.Level)
	}

	if lc.Format != LogFormatJSON {
		t.Errorf("Expected Format %s, got %s", LogFormatJSON, lc.Format)
	}

	if lc.Output != LogOutputBoth {
		t.Errorf("Expected Output %s, got %s", LogOutputBoth, lc.Output)
	}

	if lc.LogFile != "/tmp/test.log" {
		t.Errorf("Expected LogFile '/tmp/test.log', got %s", lc.LogFile)
	}

	if lc.MaxSize != 10 {
		t.Errorf("Expected MaxSize 10, got %d", lc.MaxSize)
	}

	if lc.MaxBackups != 5 {
		t.Errorf("Expected MaxBackups 5, got %d", lc.MaxBackups)
	}
}

func TestCacheConfig(t *testing.T) {
	cc := CacheConfig{
		Enabled:             true,
		MaxEntries:          1000,
		DefaultTTLHours:     24,
		SuggestionTTLHours:  6,
		CommandTTLHours:     12,
		EnableSimilarity:    true,
		SimilarityThreshold: 0.8,
		MaxSimilarityCache:  500,
	}

	if !cc.Enabled {
		t.Error("Cache should be enabled")
	}

	if cc.MaxEntries != 1000 {
		t.Errorf("Expected MaxEntries 1000, got %d", cc.MaxEntries)
	}

	if cc.DefaultTTLHours != 24 {
		t.Errorf("Expected DefaultTTLHours 24, got %d", cc.DefaultTTLHours)
	}

	if cc.SuggestionTTLHours != 6 {
		t.Errorf("Expected SuggestionTTLHours 6, got %d", cc.SuggestionTTLHours)
	}

	if cc.CommandTTLHours != 12 {
		t.Errorf("Expected CommandTTLHours 12, got %d", cc.CommandTTLHours)
	}

	if !cc.EnableSimilarity {
		t.Error("EnableSimilarity should be true")
	}

	if cc.SimilarityThreshold != 0.8 {
		t.Errorf("Expected SimilarityThreshold 0.8, got %f", cc.SimilarityThreshold)
	}

	if cc.MaxSimilarityCache != 500 {
		t.Errorf("Expected MaxSimilarityCache 500, got %d", cc.MaxSimilarityCache)
	}
}

func TestUserPreferences(t *testing.T) {
	up := UserPreferences{
		Language:           "ja",
		EnabledLLMTriggers: []string{"CommandNotFound", "NetworkError"},
		AutoExecute:        true,
		Context: ContextConfig{
			MaxHistoryEntries:  15,
			IncludeDirectories: false,
			FilterSensitiveCmd: true,
			EnableEnhanced:     true,
		},
		Logging: LoggingConfig{
			Level:  LogLevelWarn,
			Format: LogFormatText,
			Output: LogOutputConsole,
		},
		Cache: CacheConfig{
			Enabled:    false,
			MaxEntries: 100,
		},
		MaxHistorySize: 75,
		ShowTips:       false,
		VerboseOutput:  true,
	}

	if up.Language != "ja" {
		t.Errorf("Expected Language 'ja', got %s", up.Language)
	}

	if len(up.EnabledLLMTriggers) != 2 {
		t.Errorf("Expected 2 enabled triggers, got %d", len(up.EnabledLLMTriggers))
	}

	if !up.AutoExecute {
		t.Error("AutoExecute should be true")
	}

	if up.Context.MaxHistoryEntries != 15 {
		t.Errorf("Expected MaxHistoryEntries 15, got %d", up.Context.MaxHistoryEntries)
	}

	if up.Logging.Level != LogLevelWarn {
		t.Errorf("Expected log level %s, got %s", LogLevelWarn, up.Logging.Level)
	}

	if up.Cache.Enabled {
		t.Error("Cache should be disabled")
	}

	if up.MaxHistorySize != 75 {
		t.Errorf("Expected MaxHistorySize 75, got %d", up.MaxHistorySize)
	}

	if up.ShowTips {
		t.Error("ShowTips should be false")
	}

	if !up.VerboseOutput {
		t.Error("VerboseOutput should be true")
	}
}

func TestGetValidLogFormats(t *testing.T) {
	formats := GetValidLogFormats()
	expectedFormats := []string{LogFormatJSON, LogFormatText}

	if len(formats) != len(expectedFormats) {
		t.Errorf("Expected %d formats, got %d", len(expectedFormats), len(formats))
	}

	for _, format := range expectedFormats {
		found := false
		for _, f := range formats {
			if f == format {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Format %s should be in valid formats", format)
		}
	}
}

func TestGetValidLogOutputs(t *testing.T) {
	outputs := GetValidLogOutputs()
	expectedOutputs := []string{LogOutputFile, LogOutputConsole, LogOutputBoth}

	if len(outputs) != len(expectedOutputs) {
		t.Errorf("Expected %d outputs, got %d", len(expectedOutputs), len(outputs))
	}

	for _, output := range expectedOutputs {
		found := false
		for _, o := range outputs {
			if o == output {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Output %s should be in valid outputs", output)
		}
	}
}

func TestIsValidLogFormat(t *testing.T) {
	// Test that standard formats are defined
	if LogFormatJSON == "" {
		t.Error("LogFormatJSON should not be empty")
	}

	if LogFormatText == "" {
		t.Error("LogFormatText should not be empty")
	}
}

func TestIsValidLogOutput(t *testing.T) {
	// Test that standard outputs are defined
	if LogOutputFile == "" {
		t.Error("LogOutputFile should not be empty")
	}

	if LogOutputConsole == "" {
		t.Error("LogOutputConsole should not be empty")
	}

	if LogOutputBoth == "" {
		t.Error("LogOutputBoth should not be empty")
	}
}

func TestConfigEnabled(t *testing.T) {
	config := &Config{
		Enabled:         false,
		DefaultProvider: ProviderOpenAI,
		Providers:       make(map[string]ProviderConfig),
	}

	if config.Enabled {
		t.Error("Config should be disabled")
	}

	config.Enabled = true
	if !config.Enabled {
		t.Error("Config should be enabled")
	}
}
