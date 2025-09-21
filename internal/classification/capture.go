package classification

import "strings"

// ErrorType defines the category of a command execution error.
type ErrorType string

// Constants for various error types.
const (
	CommandNotFound         ErrorType = "CommandNotFound"
	FileNotFoundOrDirectory ErrorType = "FileNotFoundOrDirectory"
	PermissionDenied        ErrorType = "PermissionDenied"
	CannotExecute           ErrorType = "CannotExecute"
	InvalidArgumentOrOption ErrorType = "InvalidArgumentOrOption"
	ResourceExists          ErrorType = "ResourceExists"
	NotADirectory           ErrorType = "NotADirectory"
	TerminatedBySignal      ErrorType = "TerminatedBySignal"
	GenericError            ErrorType = "GenericError"
	// Extended error types for better classification
	NetworkError            ErrorType = "NetworkError"
	DatabaseError           ErrorType = "DatabaseError"
	ConfigError             ErrorType = "ConfigError"
	DependencyError        ErrorType = "DependencyError"
	TimeoutError           ErrorType = "TimeoutError"
	MemoryError            ErrorType = "MemoryError"
	DiskSpaceError         ErrorType = "DiskSpaceError"
	PermissionError         ErrorType = "PermissionError"
	AuthenticationError    ErrorType = "AuthenticationError"
	InteractiveToolUsage   ErrorType = "InteractiveToolUsage"
)

var allErrorTypes = []ErrorType{
	CommandNotFound,
	FileNotFoundOrDirectory,
	PermissionDenied,
	CannotExecute,
	InvalidArgumentOrOption,
	ResourceExists,
	NotADirectory,
	TerminatedBySignal,
	GenericError,
	// Extended error types
	NetworkError,
	DatabaseError,
	ConfigError,
	DependencyError,
	TimeoutError,
	MemoryError,
	DiskSpaceError,
	PermissionError,
	AuthenticationError,
	InteractiveToolUsage,
}

// AllErrorTypes returns a copy of supported error categories for hook triggers.
func AllErrorTypes() []ErrorType {
	result := make([]ErrorType, len(allErrorTypes))
	copy(result, allErrorTypes)
	return result
}

// AllErrorTypeStrings exposes supported error categories in string form.
func AllErrorTypeStrings() []string {
	result := make([]string, len(allErrorTypes))
	for i, errType := range allErrorTypes {
		result[i] = string(errType)
	}
	return result
}

// Classifier analyzes command output to determine the error type.
type Classifier struct{}

// NewClassifier creates a new Classifier.
func NewClassifier() *Classifier {
	return &Classifier{}
}

// isInteractiveToolUsageError checks if the error is from a known interactive tool's usage message
func isInteractiveToolUsageError(combined string) bool {
	// Claude Code specific error message
	if strings.Contains(combined, "Input must be provided either through stdin or as a prompt argument when using --print") {
		return true
	}
	
	// Other interactive tools' common usage errors
	usagePatterns := []string{
		"Usage:",
		"usage:",
		"Try '--help' for more information",
		"Use -h for help",
		"Run with --help for more information",
		"For help, run:",
	}
	
	for _, pattern := range usagePatterns {
		if strings.Contains(combined, pattern) {
			return true
		}
	}
	
	return false
}

// isNetworkError checks if the error is network-related
func isNetworkError(combined string) bool {
	networkPatterns := []string{
		"connection refused",
		"connection timed out",
		"network is unreachable",
		"host is down",
		"no route to host",
		"name resolution failed",
		"dns lookup failed",
		"connection reset by peer",
		"broken pipe",
		"network error",
		"connection lost",
		"unable to connect",
		"timeout connecting",
		"connection failed",
	}
	
	combinedLower := strings.ToLower(combined)
	for _, pattern := range networkPatterns {
		if strings.Contains(combinedLower, pattern) {
			return true
		}
	}
	return false
}

// isDatabaseError checks if the error is database-related
func isDatabaseError(combined string) bool {
	dbPatterns := []string{
		"database connection failed",
		"sql error",
		"database error",
		"connection to database",
		"database is locked",
		"table doesn't exist",
		"column doesn't exist",
		"constraint violation",
		"duplicate key",
		"foreign key constraint",
		"database timeout",
		"deadlock detected",
	}
	
	combinedLower := strings.ToLower(combined)
	for _, pattern := range dbPatterns {
		if strings.Contains(combinedLower, pattern) {
			return true
		}
	}
	return false
}

// isConfigError checks if the error is configuration-related
func isConfigError(combined string) bool {
	configPatterns := []string{
		"config file not found",
		"configuration error",
		"invalid configuration",
		"missing required config",
		"config parse error",
		"malformed config",
		"config validation failed",
		"configuration is invalid",
		"bad configuration",
		"config file corrupt",
	}
	
	combinedLower := strings.ToLower(combined)
	for _, pattern := range configPatterns {
		if strings.Contains(combinedLower, pattern) {
			return true
		}
	}
	return false
}

// isDependencyError checks if the error is dependency-related
func isDependencyError(combined string) bool {
	depPatterns := []string{
		"dependency not found",
		"missing dependency",
		"dependency conflict",
		"package not found",
		"module not found",
		"library not found",
		"shared library",
		"cannot load library",
		"undefined symbol",
		"version mismatch",
		"incompatible version",
		"dependency resolution failed",
	}
	
	combinedLower := strings.ToLower(combined)
	for _, pattern := range depPatterns {
		if strings.Contains(combinedLower, pattern) {
			return true
		}
	}
	return false
}

// isTimeoutError checks if the error is timeout-related
func isTimeoutError(combined string) bool {
	timeoutPatterns := []string{
		"timeout",
		"timed out",
		"operation timeout",
		"request timeout",
		"response timeout",
		"deadline exceeded",
		"context deadline exceeded",
		"execution timeout",
		"command timeout",
	}
	
	combinedLower := strings.ToLower(combined)
	for _, pattern := range timeoutPatterns {
		if strings.Contains(combinedLower, pattern) {
			return true
		}
	}
	return false
}

// isMemoryError checks if the error is memory-related
func isMemoryError(combined string) bool {
	memoryPatterns := []string{
		"out of memory",
		"memory allocation failed",
		"cannot allocate memory",
		"insufficient memory",
		"memory exhausted",
		"oom killed",
		"killed by signal 9",
		"memory limit exceeded",
		"heap space",
		"stack overflow",
	}
	
	combinedLower := strings.ToLower(combined)
	for _, pattern := range memoryPatterns {
		if strings.Contains(combinedLower, pattern) {
			return true
		}
	}
	return false
}

// isDiskSpaceError checks if the error is disk space related
func isDiskSpaceError(combined string) bool {
	diskPatterns := []string{
		"no space left on device",
		"disk full",
		"insufficient disk space",
		"device or resource busy",
		"no space left",
		"quota exceeded",
		"disk quota exceeded",
		"file system full",
		"not enough space",
		"storage space",
	}
	
	combinedLower := strings.ToLower(combined)
	for _, pattern := range diskPatterns {
		if strings.Contains(combinedLower, pattern) {
			return true
		}
	}
	return false
}

// isAuthenticationError checks if the error is authentication-related  
func isAuthenticationError(combined string) bool {
	authPatterns := []string{
		"authentication failed",
		"invalid credentials",
		"unauthorized",
		"access denied",
		"login failed",
		"invalid username or password",
		"authentication required",
		"invalid token",
		"token expired",
		"certificate verification failed",
		"ssl certificate",
		"permission denied",
		"forbidden",
		"401 unauthorized",
		"403 forbidden",
	}
	
	combinedLower := strings.ToLower(combined)
	for _, pattern := range authPatterns {
		if strings.Contains(combinedLower, pattern) {
			return true
		}
	}
	return false
}

// Classify determines the ErrorType from an exit code and collected output.
func (c *Classifier) Classify(exitCode int, stdout, stderr string) ErrorType {
	combined := stderr + "\n" + stdout

	// Skip classification for interactive tool usage errors
	if isInteractiveToolUsageError(combined) {
		return InteractiveToolUsage // Use dedicated type that's not auto-enabled
	}

	switch {
	case strings.Contains(combined, "command not found"):
		return CommandNotFound
	case strings.Contains(combined, "No such file or directory"):
		return FileNotFoundOrDirectory
	case strings.Contains(combined, "Permission denied"):
		return PermissionDenied
	case strings.Contains(combined, "cannot execute binary file"):
		return CannotExecute
	case strings.Contains(combined, "invalid argument") || strings.Contains(combined, "invalid option"):
		return InvalidArgumentOrOption
	case strings.Contains(combined, "File exists"):
		return ResourceExists
	case strings.Contains(combined, "is not a directory"):
		return NotADirectory
	// Extended error classification (check more specific errors first)
	case isDatabaseError(combined):
		return DatabaseError
	case isConfigError(combined):
		return ConfigError
	case isDependencyError(combined):
		return DependencyError
	case isMemoryError(combined):
		return MemoryError
	case isDiskSpaceError(combined):
		return DiskSpaceError
	case isAuthenticationError(combined):
		return AuthenticationError
	case isNetworkError(combined):
		return NetworkError
	case isTimeoutError(combined):
		return TimeoutError
	case exitCode > 128:
		return TerminatedBySignal
	default:
		return GenericError
	}
}
