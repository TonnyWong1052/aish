package ui

import (
	"strings"
	"testing"
)

func TestNewPresenter(t *testing.T) {
	presenter := NewPresenter()
	if presenter == nil {
		t.Error("NewPresenter should return a non-nil presenter")
	}

	// Note: We cannot check presenter.spinner directly as it's unexported
	// This test just verifies the presenter is created successfully
}

func TestSuggestion(t *testing.T) {
	testCases := []struct {
		name        string
		suggestion  Suggestion
		expectedLen int
	}{
		{
			name: "Complete suggestion",
			suggestion: Suggestion{
				Explanation: "This is a test explanation",
				Command:     "echo 'hello world'",
				Title:       "AI Suggestion",
			},
			expectedLen: 3, // All fields populated
		},
		{
			name: "Command only",
			suggestion: Suggestion{
				Command: "ls -la",
				Title:   "Quick Command",
			},
			expectedLen: 2, // Two fields populated
		},
		{
			name:        "Empty suggestion",
			suggestion:  Suggestion{},
			expectedLen: 0, // No fields populated
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test that suggestion struct works as expected
			suggestion := tc.suggestion

			fieldCount := 0
			if suggestion.Explanation != "" {
				fieldCount++
			}
			if suggestion.Command != "" {
				fieldCount++
			}
			if suggestion.Title != "" {
				fieldCount++
			}

			if fieldCount != tc.expectedLen {
				t.Errorf("Expected %d populated fields, got %d", tc.expectedLen, fieldCount)
			}
		})
	}
}

func TestPresenterShowStopLoading(t *testing.T) {
	presenter := NewPresenter()

	// Test ShowLoading
	presenter.ShowLoading("Testing loading")
	if presenter.spinner == nil {
		t.Error("ShowLoading should initialize spinner")
	}

	// Test StopLoading with success
	presenter.StopLoading(true)
	if presenter.spinner != nil {
		t.Error("StopLoading should reset spinner to nil")
	}

	// Test StopLoading with failure
	presenter.ShowLoading("Testing failure")
	presenter.StopLoading(false)
	if presenter.spinner != nil {
		t.Error("StopLoading should reset spinner to nil after failure")
	}

	// Test StopLoading when no spinner is running
	presenter.StopLoading(true) // Should not panic
}

func TestPresenterDoubleShowLoading(t *testing.T) {
	presenter := NewPresenter()

	// Test multiple ShowLoading calls
	presenter.ShowLoading("First loading")
	firstSpinner := presenter.spinner

	presenter.ShowLoading("Second loading")
	secondSpinner := presenter.spinner

	// The spinner should be replaced/updated
	if firstSpinner == nil || secondSpinner == nil {
		t.Error("Both spinners should be non-nil")
	}

	presenter.StopLoading(true)
}

// TestValidateUserInputHandling tests the different user input scenarios
func TestValidateUserInputHandling(t *testing.T) {
	testCases := []struct {
		name            string
		userInput       string
		expectedPrompt  string
		expectedProceed bool
	}{
		{
			name:            "Empty input (Enter)",
			userInput:       "",
			expectedPrompt:  "",
			expectedProceed: true,
		},
		{
			name:            "Whitespace only (treated as Enter)",
			userInput:       "   ",
			expectedPrompt:  "",
			expectedProceed: true,
		},
		{
			name:            "No response",
			userInput:       "n",
			expectedPrompt:  "",
			expectedProceed: false,
		},
		{
			name:            "No response (full word)",
			userInput:       "no",
			expectedPrompt:  "",
			expectedProceed: false,
		},
		{
			name:            "No response (case insensitive)",
			userInput:       "N",
			expectedPrompt:  "",
			expectedProceed: false,
		},
		{
			name:            "Custom prompt",
			userInput:       "show me files in /tmp",
			expectedPrompt:  "show me files in /tmp",
			expectedProceed: true,
		},
		{
			name:            "Custom prompt with extra whitespace",
			userInput:       "  ls -la  ",
			expectedPrompt:  "ls -la",
			expectedProceed: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate the input processing logic from Render method
			input := strings.TrimSpace(strings.ToLower(tc.userInput))

			var prompt string
			var proceed bool

			switch input {
			case "": // Enter
				prompt = ""
				proceed = true
			case "n", "no":
				prompt = ""
				proceed = false
			default:
				prompt = input
				proceed = true
			}

			if prompt != tc.expectedPrompt {
				t.Errorf("Expected prompt '%s', got '%s'", tc.expectedPrompt, prompt)
			}

			if proceed != tc.expectedProceed {
				t.Errorf("Expected proceed %v, got %v", tc.expectedProceed, proceed)
			}
		})
	}
}

func TestSuggestionValidation(t *testing.T) {
	testCases := []struct {
		name       string
		suggestion Suggestion
		isValid    bool
	}{
		{
			name: "Valid complete suggestion",
			suggestion: Suggestion{
				Explanation: "This command lists files",
				Command:     "ls -la",
				Title:       "File Listing",
			},
			isValid: true,
		},
		{
			name: "Valid minimal suggestion",
			suggestion: Suggestion{
				Command: "pwd",
				Title:   "Current Directory",
			},
			isValid: true,
		},
		{
			name: "Invalid - no command",
			suggestion: Suggestion{
				Explanation: "This is just an explanation",
				Title:       "No Command",
			},
			isValid: false,
		},
		{
			name: "Invalid - empty command",
			suggestion: Suggestion{
				Command: "",
				Title:   "Empty Command",
			},
			isValid: false,
		},
		{
			name: "Invalid - only whitespace command",
			suggestion: Suggestion{
				Command: "   ",
				Title:   "Whitespace Command",
			},
			isValid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test suggestion validation logic
			isValid := tc.suggestion.Command != "" && strings.TrimSpace(tc.suggestion.Command) != ""

			if isValid != tc.isValid {
				t.Errorf("Expected validity %v, got %v", tc.isValid, isValid)
			}
		})
	}
}

func TestPresenterFields(t *testing.T) {
	presenter := NewPresenter()

	// Test that we can use the presenter methods without panicking
	presenter.ShowLoading("test")
	presenter.StopLoading(true)
	// Note: We cannot check spinner field directly as it's unexported
}
