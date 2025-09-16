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

// Classify determines the ErrorType from an exit code and stderr.
func (c *Classifier) Classify(exitCode int, stderr string) ErrorType {
	switch {
	case strings.Contains(stderr, "command not found"):
		return CommandNotFound
	case strings.Contains(stderr, "No such file or directory"):
		return FileNotFoundOrDirectory
	case strings.Contains(stderr, "Permission denied"):
		return PermissionDenied
	case strings.Contains(stderr, "cannot execute binary file"):
		return CannotExecute
	case strings.Contains(stderr, "invalid argument") || strings.Contains(stderr, "invalid option"):
		return InvalidArgumentOrOption
	case strings.Contains(stderr, "File exists"):
		return ResourceExists
	case strings.Contains(stderr, "is not a directory"):
		return NotADirectory
	case exitCode > 128:
		return TerminatedBySignal
	default:
		return GenericError
	}
}
