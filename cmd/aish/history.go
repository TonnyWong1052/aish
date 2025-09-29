package main

import (
	"context"
	"fmt"
	"os"

	"github.com/TonnyWong1052/aish/internal/config"
	"github.com/TonnyWong1052/aish/internal/history"
	"github.com/TonnyWong1052/aish/internal/llm"
	"github.com/TonnyWong1052/aish/internal/ui"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// This is the new parent command for history
var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Manage the error history",
	Long:  `View, re-run analysis on, or clear the history of captured errors.`,
	Run:   listHistoryAndAnalyze, // The default action is to list and allow selection
}

var historyClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clears the saved analysis history",
	Run: func(cmd *cobra.Command, args []string) {
		if err := history.Clear(); err != nil {
			pterm.Error.Printfln("Failed to clear history: %v", err)
			os.Exit(1)
		}
		pterm.Success.Println("History cleared.")
	},
}

// listHistoryAndAnalyze contains the logic from the original historyCmd
func listHistoryAndAnalyze(cmd *cobra.Command, args []string) {
	hist, err := history.Load()
	if err != nil {
		pterm.Error.Printfln("Failed to load history: %v", err)
		os.Exit(1)
	}

	if len(hist.Entries) == 0 {
		pterm.Info.Println("No history found.")
		return
	}

	var options []string
	for _, entry := range hist.Entries {
		command := entry.Command
		if len(command) > 50 {
			command = command[:47] + "..."
		}
		options = append(options, fmt.Sprintf("%s [%s] - %s", entry.Timestamp.Format("2006-01-02 15:04:05"), entry.ErrorType, command))
	}

	fmt.Println()
	selected, _ := pterm.DefaultInteractiveSelect.
		WithOptions(options).
		Show("Select an error to analyze >")

	var selectedEntry history.Entry
	for i, option := range options {
		if option == selected {
			selectedEntry = hist.Entries[i]
			break
		}
	}

	cfg, err := config.Load()
	if err != nil {
		pterm.Error.Printfln("Failed to load config: %v", err)
		os.Exit(1)
	}

	providerName := effectiveProviderName(cfg)
	providerCfg, ok := cfg.Providers[providerName]
	if !ok || isProviderConfigIncomplete(providerName, providerCfg) {
		pterm.Error.Printfln("Default provider not configured. Please run 'aish config'.")
		os.Exit(1)
	}
	provider, err := getProvider(providerName, providerCfg)
	if err != nil {
		pterm.Error.Printfln("Failed to create provider: %v", err)
		os.Exit(1)
	}

	presenter := ui.NewPresenter()
	if err := presenter.ShowLoadingWithTimer("Analyzing selected error"); err != nil {
		pterm.Warning.Printfln("Warning: Could not start loading animation: %v", err)
	}

	suggestion, err := provider.GetSuggestion(context.Background(), llm.CapturedContext{
		Command:  selectedEntry.Command,
		Stdout:   selectedEntry.Stdout,
		Stderr:   selectedEntry.Stderr,
		ExitCode: selectedEntry.ExitCode,
	}, effectiveLanguage(cfg))

	if err != nil {
		presenter.StopLoading(false)
		pterm.Error.Printfln("Failed to get suggestion: %v", err)
		os.Exit(1)
	}
	presenter.StopLoading(true)

	for {
		uiSuggestion := ui.Suggestion{
			Title:       "Analysis of Historical Error",
			Explanation: suggestion.Explanation,
			Command:     suggestion.CorrectedCommand,
		}
		userInput, shouldContinue, err := presenter.Render(uiSuggestion)
		if err != nil {
			pterm.Warning.Printfln("Operation cancelled: %v", err)
			return
		}

		if !shouldContinue {
			break
		}

		if userInput == "" {
			executeCommand(suggestion.CorrectedCommand)
			break
		} else {
			if err := presenter.ShowLoadingWithTimer("Getting new suggestion"); err != nil {
				pterm.Warning.Printfln("Warning: Could not start loading animation: %v", err)
			}
			suggestion, err = provider.GetSuggestion(context.Background(), llm.CapturedContext{
				Command: userInput,
			}, cfg.UserPreferences.Language)
			if err != nil {
				presenter.StopLoading(false)
				pterm.Error.Printfln("Failed to get new suggestion: %v", err)
				break
			}
			presenter.StopLoading(true)
		}
	}
}

func init() {
	historyCmd.AddCommand(historyClearCmd)
}
