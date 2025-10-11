package classification_test

import (
	"fmt"

	"github.com/TonnyWong1052/aish/internal/classification"
)

// Example demonstrates basic error classification
func Example() {
	classifier := classification.NewClassifier()

	// Classify a command not found error
	errorType := classifier.Classify(127, "", "bash: unknowncmd: command not found")
	fmt.Println(errorType)
	// Output: CommandNotFound
}

// Example_commandNotFound demonstrates classifying a command not found error
func Example_commandNotFound() {
	classifier := classification.NewClassifier()

	exitCode := 127
	stderr := "bash: unknowncmd: command not found"

	errorType := classifier.Classify(exitCode, "", stderr)
	fmt.Printf("Error Type: %s\n", errorType)
	fmt.Printf("Is Command Not Found: %t\n", errorType == classification.CommandNotFound)
	// Output:
	// Error Type: CommandNotFound
	// Is Command Not Found: true
}

// Example_permissionDenied demonstrates classifying a permission denied error
func Example_permissionDenied() {
	classifier := classification.NewClassifier()

	exitCode := 1
	stderr := "cat: /root/secret: Permission denied"

	errorType := classifier.Classify(exitCode, "", stderr)
	fmt.Println(errorType)
	// Output: PermissionDenied
}

// Example_invalidArgument demonstrates classifying an invalid argument error
func Example_invalidArgument() {
	classifier := classification.NewClassifier()

	exitCode := 1
	stderr := "ls: invalid option -- 'Z'"

	errorType := classifier.Classify(exitCode, "", stderr)
	fmt.Println(errorType)
	// Output: InvalidArgumentOrOption
}

// Example_fileNotFound demonstrates classifying a file not found error
func Example_fileNotFound() {
	classifier := classification.NewClassifier()

	exitCode := 1
	stderr := "cat: /nonexistent/file: No such file or directory"

	errorType := classifier.Classify(exitCode, "", stderr)
	fmt.Println(errorType)
	// Output: FileNotFoundOrDirectory
}

// Example_multipleErrors demonstrates how the classifier handles multiple error patterns
func Example_multipleErrors() {
	classifier := classification.NewClassifier()

	// When multiple patterns match, the first one takes precedence
	stderr := "bash: command not found\npermission denied"
	errorType := classifier.Classify(127, "", stderr)

	// Exit code 127 strongly indicates CommandNotFound
	fmt.Println(errorType)
	// Output: CommandNotFound
}

// Example_genericError demonstrates classifying a generic error
func Example_genericError() {
	classifier := classification.NewClassifier()

	exitCode := 1
	stderr := "Some random error occurred"

	errorType := classifier.Classify(exitCode, "", stderr)
	fmt.Println(errorType)
	// Output: GenericError
}
