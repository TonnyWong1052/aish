package config

import "time"

// Application constants
const (
	// Application metadata
	AppName        = "aish"
	AppDescription = "AI Shell - An intelligent shell debugger powered by LLMs"
	
	// Directory and file paths
	DefaultConfigDir      = ".config/aish"
	DefaultStateDir       = ".config/aish"
	DefaultLogDir         = "logs"
	DefaultCacheDir       = "cache"
	DefaultConfigFileName = "config.json"
	DefaultLogFileName    = "aish.log"
	
	// File size limits
	MaxCaptureBytes     = 200_000 // Maximum bytes to capture from stdout/stderr
	MaxLogFileSize      = 10      // Maximum log file size in MB
	MaxConfigFileSize   = 1       // Maximum config file size in MB
	DefaultMaxBackups   = 5       // Default number of log backup files
	
	// Cache configuration
	DefaultCacheEntries        = 1000
	DefaultCacheTTLHours       = 24
	DefaultSuggestionTTLHours  = 6
	DefaultCommandTTLHours     = 24
	DefaultSimilarityThreshold = 0.85
	DefaultMaxSimilarityCache  = 500
	
	// History management
	DefaultMaxHistorySize     = 100
	DefaultMaxHistoryEntries  = 10
	
	// Timeouts and intervals
	DefaultHTTPTimeout     = 30 * time.Second
	DefaultCleanupInterval = time.Minute
	DefaultRetryDelay      = 100 * time.Millisecond
	MaxRetryAttempts       = 3
	
	// API endpoints
	OpenAIAPIEndpoint    = "https://api.openai.com/v1"
	GeminiAPIEndpoint    = "https://generativelanguage.googleapis.com/v1"
	GeminiCLIAPIEndpoint = "https://cloudcode-pa.googleapis.com/v1internal:generateContent"
	
	// Default models
	DefaultOpenAIModel    = "gpt-4"
	DefaultGeminiModel    = "gemini-pro"
	DefaultGeminiCLIModel = "gemini-2.5-flash"
	
	// Log levels
	LogLevelTrace = "trace"
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
	LogLevelFatal = "fatal"
	LogLevelPanic = "panic"
	
	// Log formats
	LogFormatJSON = "json"
	LogFormatText = "text"
	
	// Log outputs
	LogOutputFile    = "file"
	LogOutputConsole = "console"
	LogOutputBoth    = "both"
	
	// Shell hook markers
	HookStartMarker = "# AISH (AI Shell) Hook - Start"
	HookEndMarker   = "# AISH (AI Shell) Hook - End"
	
	// Environment variables
	EnvAISHDebug              = "AISH_DEBUG"
	EnvAISHStateDir           = "AISH_STATE_DIR"
	EnvAISHStdoutFile         = "AISH_STDOUT_FILE"
	EnvAISHStderrFile         = "AISH_STDERR_FILE"
	EnvAISHCaptureOff         = "AISH_CAPTURE_OFF"
	EnvAISHHookDisabled       = "AISH_HOOK_DISABLED"
	EnvAISHSkipCommandPatterns = "AISH_SKIP_COMMAND_PATTERNS"
	EnvAISHSkipAllUserCommands = "AISH_SKIP_ALL_USER_COMMANDS"
	EnvAISHSystemDirWhitelist = "AISH_SYSTEM_DIR_WHITELIST"
	
	// Gemini-specific environment variables
	EnvAISHGeminiDebug       = "AISH_GEMINI_DEBUG"
	EnvAISHGeminiProject     = "AISH_GEMINI_PROJECT"
	EnvAISHGeminiBearer      = "AISH_GEMINI_BEARER"
	EnvAISHGeminiUseCURL     = "AISH_GEMINI_USE_CURL"
	EnvAISHGeminiTimeout     = "AISH_GEMINI_TIMEOUT"
	EnvAISHGeminiCAFile      = "AISH_GEMINI_CA_FILE"
	EnvAISHGeminiSkipTLSVerify = "AISH_GEMINI_SKIP_TLS_VERIFY"
	
	// Exit codes
	ExitSuccess         = 0
	ExitGenericError    = 1
	ExitPermissionError = 77
	ExitConfigError     = 78
	ExitAuthError       = 79
	ExitUserCancel      = 130
	
	// Provider names
	ProviderOpenAI    = "openai"
	ProviderGemini    = "gemini"
	ProviderGeminiCLI = "gemini-cli"
	
	// Default system directory whitelist (colon-separated)
	DefaultSystemDirWhitelist = "/bin:/usr/bin:/sbin:/usr/sbin:/usr/libexec:/System/Library:/lib:/usr/lib"
	DefaultWindowsSystemDirWhitelist = "C:\\Windows\\System32;C:\\Windows;C:\\Windows\\SysWOW64;C:\\Program Files\\PowerShell\\7;C:\\Windows\\System32\\WindowsPowerShell\\v1.0"
	
	// File permissions
	DefaultDirPermissions  = 0755
	DefaultFilePermissions = 0644
	DefaultExecPermissions = 0755
	
	// Validation limits
	MaxProviderNameLength = 50
	MaxModelNameLength    = 100
	MaxAPIKeyLength       = 500
	MaxProjectIDLength    = 100
	MaxPromptLength       = 10000
	MaxResponseLength     = 50000
)

// GetValidLogLevels returns all valid log levels
func GetValidLogLevels() []string {
	return []string{
		LogLevelTrace,
		LogLevelDebug,
		LogLevelInfo,
		LogLevelWarn,
		LogLevelError,
		LogLevelFatal,
		LogLevelPanic,
	}
}

// GetValidLogFormats returns all valid log formats
func GetValidLogFormats() []string {
	return []string{
		LogFormatJSON,
		LogFormatText,
	}
}

// GetValidLogOutputs returns all valid log outputs
func GetValidLogOutputs() []string {
	return []string{
		LogOutputFile,
		LogOutputConsole,
		LogOutputBoth,
	}
}

// GetSupportedProviders returns all supported LLM providers
func GetSupportedProviders() []string {
	return []string{
		ProviderOpenAI,
		ProviderGemini,
		ProviderGeminiCLI,
	}
}

// IsValidLogLevel checks if a log level is valid
func IsValidLogLevel(level string) bool {
	for _, validLevel := range GetValidLogLevels() {
		if level == validLevel {
			return true
		}
	}
	return false
}

// IsValidProvider checks if a provider is supported
func IsValidProvider(provider string) bool {
	for _, validProvider := range GetSupportedProviders() {
		if provider == validProvider {
			return true
		}
	}
	return false
}