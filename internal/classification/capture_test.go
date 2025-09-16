package classification

import (
	"testing"
)

func TestClassifier_Classify(t *testing.T) {
	classifier := NewClassifier()

	testCases := []struct {
		name     string
		exitCode int
		stderr   string
		expected ErrorType
	}{
		{
			name:     "Command not found",
			exitCode: 127,
			stderr:   "bash: unknowncmd: command not found",
			expected: CommandNotFound,
		},
		{
			name:     "File not found",
			exitCode: 1,
			stderr:   "cat: /nonexistent/file: No such file or directory",
			expected: FileNotFoundOrDirectory,
		},
		{
			name:     "Permission denied",
			exitCode: 1,
			stderr:   "cat: /root/secret: Permission denied",
			expected: PermissionDenied,
		},
		{
			name:     "Cannot execute binary",
			exitCode: 126,
			stderr:   "bash: ./script: cannot execute binary file",
			expected: CannotExecute,
		},
		{
			name:     "Invalid argument",
			exitCode: 1,
			stderr:   "ls: invalid option -- 'Z'",
			expected: InvalidArgumentOrOption,
		},
		{
			name:     "Invalid argument variant",
			exitCode: 1,
			stderr:   "cp: invalid argument '--badarg'",
			expected: InvalidArgumentOrOption,
		},
		{
			name:     "File exists",
			exitCode: 1,
			stderr:   "mkdir: /tmp/test: File exists",
			expected: ResourceExists,
		},
		{
			name:     "Not a directory",
			exitCode: 1,
			stderr:   "cd: /etc/passwd: is not a directory",
			expected: NotADirectory,
		},
		{
			name:     "Terminated by signal",
			exitCode: 130, // SIGINT (Ctrl+C)
			stderr:   "Process interrupted",
			expected: TerminatedBySignal,
		},
		{
			name:     "Generic error",
			exitCode: 1,
			stderr:   "Some random error occurred",
			expected: GenericError,
		},
		{
			name:     "Success case (exit code 0)",
			exitCode: 0,
			stderr:   "",
			expected: GenericError, // Even success gets classified as GenericError by default
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := classifier.Classify(tc.exitCode, tc.stderr)
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
		stderr   string
		expected ErrorType
	}{
		{
			name:     "Empty stderr",
			exitCode: 1,
			stderr:   "",
			expected: GenericError,
		},
		{
			name:     "Mixed error messages",
			exitCode: 1,
			stderr:   "command not found, but also Permission denied",
			expected: CommandNotFound, // First match wins
		},
		{
			name:     "Case sensitivity",
			exitCode: 1,
			stderr:   "COMMAND NOT FOUND",
			expected: GenericError, // Case sensitive matching
		},
		{
			name:     "Multiline stderr",
			exitCode: 1,
			stderr:   "Error occurred:\nPermission denied\nOperation failed",
			expected: PermissionDenied,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := classifier.Classify(tc.exitCode, tc.stderr)
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
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			if string(tc.errorType) != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, string(tc.errorType))
			}
		})
	}
}
