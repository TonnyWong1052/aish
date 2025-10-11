package llm_test

import (
	"context"
	"fmt"

	"github.com/TonnyWong1052/aish/internal/config"
	"github.com/TonnyWong1052/aish/internal/llm"
)

// Example demonstrates how to use an LLM provider
func Example() {
	// Create a provider configuration
	cfg := config.ProviderConfig{
		APIKey:      "your-api-key",
		Model:       "gemini-2.5-flash",
		APIEndpoint: "https://generativelanguage.googleapis.com/v1",
	}

	// Get a registered provider (note: provider must be registered first)
	// This is just an example - in real usage, providers are registered via init()
	provider, err := llm.GetProvider("gemini", cfg, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Create a captured context
	ctx := llm.CapturedContext{
		Command:  "unknowncmd",
		Stderr:   "bash: unknowncmd: command not found",
		ExitCode: 127,
	}

	// Get AI suggestion
	suggestion, err := provider.GetSuggestion(context.Background(), ctx, "en")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Suggestion: %s\n", suggestion.CorrectedCommand)
	// Output will vary based on AI response
}

// Example_capturedContext demonstrates creating a CapturedContext
func Example_capturedContext() {
	// Create a context for a command that failed
	ctx := llm.CapturedContext{
		Command:  "git pus origin main",
		Stderr:   "git: 'pus' is not a git command. See 'git --help'.",
		ExitCode: 1,
		Stdout:   "",
	}

	fmt.Printf("Command: %s\n", ctx.Command)
	fmt.Printf("Exit Code: %d\n", ctx.ExitCode)
	fmt.Printf("Has Error: %t\n", ctx.ExitCode != 0)
	// Output:
	// Command: git pus origin main
	// Exit Code: 1
	// Has Error: true
}

// Example_suggestion demonstrates creating a Suggestion
func Example_suggestion() {
	suggestion := llm.Suggestion{
		CorrectedCommand: "git push origin main",
		Explanation:      "Fixed typo: 'pus' should be 'push'",
	}

	fmt.Printf("Corrected: %s\n", suggestion.CorrectedCommand)
	fmt.Printf("Explanation: %s\n", suggestion.Explanation)
	// Output:
	// Corrected: git push origin main
	// Explanation: Fixed typo: 'pus' should be 'push'
}
