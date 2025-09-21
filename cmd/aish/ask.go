package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var askCmd = &cobra.Command{
	Use:   "ask [prompt]",
	Short: "Generates a command from a natural language prompt",
	Long: `Takes a natural language prompt and uses the configured LLM
to generate a shell command that accomplishes the described task.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		prompt := args[0]
		if prompt == "" {
			fmt.Println("Error: prompt cannot be empty")
			os.Exit(1)
		}
		// This function is defined in main.go
		runPromptLogic(prompt)
	},
}
