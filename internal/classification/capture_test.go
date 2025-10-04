package classification

import (
	"testing"
)

func TestClassifier_Classify(t *testing.T) {
	classifier := NewClassifier()

	testCases := []struct {
		name     string
		exitCode int
		stdout   string
		stderr   string
		expected ErrorType
	}{
		{
			name:     "Command not found",
			exitCode: 127,
			stdout:   "",
			stderr:   "bash: unknowncmd: command not found",
			expected: CommandNotFound,
		},
		{
			name:     "File not found",
			exitCode: 1,
			stdout:   "",
			stderr:   "cat: /nonexistent/file: No such file or directory",
			expected: FileNotFoundOrDirectory,
		},
		{
			name:     "Permission denied",
			exitCode: 1,
			stdout:   "",
			stderr:   "cat: /root/secret: Permission denied",
			expected: PermissionDenied,
		},
		{
			name:     "Cannot execute binary",
			exitCode: 126,
			stdout:   "",
			stderr:   "bash: ./script: cannot execute binary file",
			expected: CannotExecute,
		},
		{
			name:     "Invalid argument",
			exitCode: 1,
			stdout:   "",
			stderr:   "ls: invalid option -- 'Z'",
			expected: InvalidArgumentOrOption,
		},
		{
			name:     "Invalid argument variant",
			exitCode: 1,
			stdout:   "",
			stderr:   "cp: invalid argument '--badarg'",
			expected: InvalidArgumentOrOption,
		},
		{
			name:     "File exists",
			exitCode: 1,
			stdout:   "",
			stderr:   "mkdir: /tmp/test: File exists",
			expected: ResourceExists,
		},
		{
			name:     "Not a directory",
			exitCode: 1,
			stdout:   "",
			stderr:   "cd: /etc/passwd: is not a directory",
			expected: NotADirectory,
		},
		{
			name:     "Terminated by signal",
			exitCode: 130, // SIGINT (Ctrl+C)
			stdout:   "",
			stderr:   "Process interrupted",
			expected: TerminatedBySignal,
		},
		{
			name:     "Usage error emitted on stdout",
			exitCode: 1,
			stdout:   "Error: Input must be provided either through stdin or as a prompt argument when using --print\n",
			stderr:   "",
			expected: InteractiveToolUsage,
		},
		{
			name:     "Generic error",
			exitCode: 1,
			stdout:   "",
			stderr:   "Some random error occurred",
			expected: GenericError,
		},
		{
			name:     "Success case (exit code 0)",
			exitCode: 0,
			stdout:   "",
			stderr:   "",
			expected: GenericError, // Even success gets classified as GenericError by default
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := classifier.Classify(tc.exitCode, tc.stdout, tc.stderr)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestClassifier_EdgeCases(t *testing.T) {
	classifier := NewClassifier()

	testCases := []struct {
		name     string
		exitCode int
		stdout   string
		stderr   string
		expected ErrorType
	}{
		{
			name:     "Empty stderr",
			exitCode: 1,
			stdout:   "",
			stderr:   "",
			expected: GenericError,
		},
		{
			name:     "Mixed error messages",
			exitCode: 1,
			stdout:   "",
			stderr:   "command not found, but also Permission denied",
			expected: CommandNotFound, // First match wins
		},
		{
			name:     "Case sensitivity",
			exitCode: 1,
			stdout:   "",
			stderr:   "COMMAND NOT FOUND",
			expected: GenericError, // Case sensitive matching
		},
		{
			name:     "Multiline stderr",
			exitCode: 1,
			stdout:   "",
			stderr:   "Error occurred:\nPermission denied\nOperation failed",
			expected: PermissionDenied,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := classifier.Classify(tc.exitCode, tc.stdout, tc.stderr)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestInteractiveToolUsageErrors(t *testing.T) {
	classifier := NewClassifier()

	testCases := []struct {
		name     string
		exitCode int
		stdout   string
		stderr   string
		expected ErrorType
	}{
		{
			name:     "Claude Code usage error",
			exitCode: 1,
			stdout:   "Error: Input must be provided either through stdin or as a prompt argument when using --print\n",
			stderr:   "",
			expected: InteractiveToolUsage,
		},
		{
			name:     "npm usage error",
			exitCode: 1,
			stdout:   "Usage: npm <command>\n\nwhere <command> is one of:",
			stderr:   "",
			expected: InteractiveToolUsage,
		},
		{
			name:     "yarn usage error",
			exitCode: 1,
			stdout:   "usage: yarn [--version]",
			stderr:   "",
			expected: InteractiveToolUsage,
		},
		{
			name:     "Tool with help suggestion",
			exitCode: 1,
			stdout:   "",
			stderr:   "Try '--help' for more information",
			expected: InteractiveToolUsage,
		},
		{
			name:     "Tool with help flag suggestion",
			exitCode: 1,
			stdout:   "",
			stderr:   "Use -h for help",
			expected: InteractiveToolUsage,
		},
		{
			name:     "Still classify real invalid arguments",
			exitCode: 1,
			stdout:   "",
			stderr:   "ls: invalid option -- 'Z'",
			expected: InvalidArgumentOrOption,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := classifier.Classify(tc.exitCode, tc.stdout, tc.stderr)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestErrorTypeString(t *testing.T) {
	// Test that ErrorType values are properly defined
	testCases := []struct {
		errorType ErrorType
		expected  string
	}{
		{CommandNotFound, "CommandNotFound"},
		{FileNotFoundOrDirectory, "FileNotFoundOrDirectory"},
		{PermissionDenied, "PermissionDenied"},
		{CannotExecute, "CannotExecute"},
		{InvalidArgumentOrOption, "InvalidArgumentOrOption"},
		{ResourceExists, "ResourceExists"},
		{NotADirectory, "NotADirectory"},
		{TerminatedBySignal, "TerminatedBySignal"},
		{GenericError, "GenericError"},
		{InteractiveToolUsage, "InteractiveToolUsage"},
		{NetworkError, "NetworkError"},
		{DatabaseError, "DatabaseError"},
		{ConfigError, "ConfigError"},
		{DependencyError, "DependencyError"},
		{TimeoutError, "TimeoutError"},
		{MemoryError, "MemoryError"},
		{DiskSpaceError, "DiskSpaceError"},
		{AuthenticationError, "AuthenticationError"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			if string(tc.errorType) != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, string(tc.errorType))
			}
		})
	}
}

func TestExtendedErrorClassification(t *testing.T) {
	classifier := NewClassifier()

	testCases := []struct {
		name     string
		exitCode int
		stdout   string
		stderr   string
		expected ErrorType
	}{
		{
			name:     "Network error - connection refused",
			exitCode: 1,
			stdout:   "",
			stderr:   "Error: connection refused to server:8080",
			expected: NetworkError,
		},
		{
			name:     "Network error - timeout",
			exitCode: 1,
			stdout:   "",
			stderr:   "curl: (7) Failed to connect to example.com port 443: Connection timed out",
			expected: NetworkError,
		},
		{
			name:     "Database error - connection failed",
			exitCode: 1,
			stdout:   "",
			stderr:   "Error: database connection failed: could not connect to server",
			expected: DatabaseError,
		},
		{
			name:     "Database error - SQL syntax",
			exitCode: 1,
			stdout:   "",
			stderr:   "SQL Error: syntax error at or near 'SELCT'",
			expected: DatabaseError,
		},
		{
			name:     "Config error - file not found",
			exitCode: 1,
			stdout:   "",
			stderr:   "Error: config file not found: /etc/myapp/config.yaml",
			expected: ConfigError,
		},
		{
			name:     "Config error - invalid configuration",
			exitCode: 1,
			stdout:   "",
			stderr:   "Configuration error: invalid configuration in section 'database'",
			expected: ConfigError,
		},
		{
			name:     "Dependency error - package not found",
			exitCode: 1,
			stdout:   "",
			stderr:   "Error: package not found: some-missing-package@1.0.0",
			expected: DependencyError,
		},
		{
			name:     "Dependency error - version mismatch",
			exitCode: 1,
			stdout:   "",
			stderr:   "Error: version mismatch for dependency xyz: expected 2.0.0, got 1.5.0",
			expected: DependencyError,
		},
		{
			name:     "Timeout error - operation timeout",
			exitCode: 1,
			stdout:   "",
			stderr:   "Error: operation timeout after 30 seconds",
			expected: TimeoutError,
		},
		{
			name:     "Timeout error - context deadline",
			exitCode: 1,
			stdout:   "",
			stderr:   "context deadline exceeded",
			expected: TimeoutError,
		},
		{
			name:     "Memory error - out of memory",
			exitCode: 137,
			stdout:   "",
			stderr:   "Error: out of memory: cannot allocate 1GB buffer",
			expected: MemoryError,
		},
		{
			name:     "Memory error - heap space",
			exitCode: 1,
			stdout:   "",
			stderr:   "java.lang.OutOfMemoryError: Java heap space",
			expected: MemoryError,
		},
		{
			name:     "Disk space error - no space left",
			exitCode: 1,
			stdout:   "",
			stderr:   "Error: no space left on device",
			expected: DiskSpaceError,
		},
		{
			name:     "Disk space error - quota exceeded",
			exitCode: 1,
			stdout:   "",
			stderr:   "Write failed: disk quota exceeded",
			expected: DiskSpaceError,
		},
		{
			name:     "Authentication error - invalid credentials",
			exitCode: 1,
			stdout:   "",
			stderr:   "Error: authentication failed: invalid credentials",
			expected: AuthenticationError,
		},
		{
			name:     "Authentication error - 401 unauthorized",
			exitCode: 1,
			stdout:   "",
			stderr:   "HTTP 401 Unauthorized: access denied",
			expected: AuthenticationError,
		},
		{
			name:     "Authentication error - token expired",
			exitCode: 1,
			stdout:   "",
			stderr:   "Error: token expired, please re-authenticate",
			expected: AuthenticationError,
		},
		{
			name:     "Multiple error patterns - network takes priority",
			exitCode: 1,
			stdout:   "",
			stderr:   "connection refused during authentication",
			expected: NetworkError, // Network error should be classified first
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := classifier.Classify(tc.exitCode, tc.stdout, tc.stderr)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v for case: %s", tc.expected, result, tc.name)
			}
		})
	}
}
