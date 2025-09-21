package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/TonnyWong1052/aish/internal/classification"
	"github.com/TonnyWong1052/aish/internal/config"
	"github.com/TonnyWong1052/aish/internal/history"
	"github.com/TonnyWong1052/aish/internal/llm"
	_ "github.com/TonnyWong1052/aish/internal/llm/gemini"
	_ "github.com/TonnyWong1052/aish/internal/llm/gemini-cli"
	_ "github.com/TonnyWong1052/aish/internal/llm/openai"
	"github.com/TonnyWong1052/aish/internal/prompt"
	"github.com/TonnyWong1052/aish/internal/ui"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "aish",
	Short: "An intelligent shell debugger powered by LLMs",
	Long: `aish is a CLI tool that automatically captures errors
from your last command, sends them to an LLM, and provides you with an
explanation and a corrected command.

Use 'aish init' to get started.
Use 'aish ask "your question"' or 'aish -p "your question"' to generate a command.`,
	Run: func(cmd *cobra.Command, args []string) {
		if flagPrompt != "" {
			runPromptLogic(flagPrompt)
			return
		}
		cmd.Help()
	},
}

// captureCmd remains a hidden internal command used by the shell hook.
var captureCmd = &cobra.Command{
	Use:    "capture [exit_code] [command]",
	Short:  "Internal command to capture context and trigger analysis",
	Hidden: true,
	Args:   cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		exitCode, err := strconv.Atoi(args[0])
		if err != nil {
			// Silently fail
			return
		}
		commandStr := args[1]

		cfg, err := config.Load()
		if err != nil || !cfg.Enabled {
			return
		}
		// Security adjustment: No longer re-run the previous command to get output, avoiding side effects and high latency.
		// If hook has written output to temp files, read through environment variables and capture tail content (avoid oversized strings).
		stdoutStr := readTail(os.Getenv(config.EnvAISHStdoutFile), config.MaxCaptureBytes)
		stderrStr := readTail(os.Getenv(config.EnvAISHStderrFile), config.MaxCaptureBytes)

		classifier := classification.NewClassifier()
		errorType := classifier.Classify(exitCode, stdoutStr, stderrStr)
		_ = history.Add(history.Entry{
			Timestamp: time.Now(),
			Command:   commandStr,
			Stdout:    stdoutStr,
			Stderr:    stderrStr,
			ExitCode:  exitCode,
			ErrorType: errorType,
		})

		isErrorTypeEnabled := false
		for _, enabledType := range cfg.UserPreferences.EnabledLLMTriggers {
			if enabledType == string(errorType) {
				isErrorTypeEnabled = true
				break
			}
		}
		if !isErrorTypeEnabled {
			return
		}

		providerName := effectiveProviderName(cfg)
		providerCfg, ok := cfg.Providers[providerName]
		if !ok || isProviderConfigIncomplete(providerName, providerCfg) {
			pterm.Error.Printfln("aish is active, but no LLM provider is configured. Run 'aish config' to set one up.")
			return
		}

		provider, err := getProvider(providerName, providerCfg)
		if err != nil {
			return
		}

		// Add visual separator before AI analysis
		pterm.Println()
		pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.BgBlue)).
			WithTextStyle(pterm.NewStyle(pterm.FgWhite)).
			Println("AI Analysis")
		
		presenter := ui.NewPresenter()
		presenter.ShowLoading("正在分析錯誤並生成建議...")

		suggestion, err := provider.GetSuggestion(context.Background(), llm.CapturedContext{
			Command:  commandStr,
			Stdout:   stdoutStr,
			Stderr:   stderrStr,
			ExitCode: exitCode,
		}, effectiveLanguage(cfg))

		if err != nil {
			presenter.StopLoading(false)
			pterm.Error.Printfln("Failed to get suggestion: %v", err)
			return
		}
		presenter.StopLoading(true)

		for {
			uiSuggestion := ui.Suggestion{
				Title:       "AI Suggestion",
				Explanation: suggestion.Explanation,
				Command:     suggestion.CorrectedCommand,
			}
			userInput, shouldContinue, err := presenter.Render(uiSuggestion)
			if err != nil || !shouldContinue {
				return
			}

			if userInput == "" {
				executeCommand(suggestion.CorrectedCommand)
				break
			} else {
				presenter.ShowLoading("Getting new suggestion...")
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
	},
}

// runPromptLogic is called by the 'ask' command.
func runPromptLogic(promptStr string) {
	cfg, err := config.Load()
	if err != nil {
		pterm.Error.Printfln("Failed to load config: %v", err)
		os.Exit(1)
	}

	var provider llm.Provider
	providerName := effectiveProviderName(cfg)
	if providerCfg, ok := cfg.Providers[providerName]; ok && !isProviderConfigIncomplete(providerName, providerCfg) {
		if p, err := getProvider(providerName, providerCfg); err == nil {
			provider = p
		}
	}

	if provider == nil {
		pterm.Error.Println("No LLM provider configured or configuration incomplete. Please run 'aish config' first.")
		os.Exit(1)
	}

	presenter := ui.NewPresenter()
	presenter.ShowLoading("Generating command...")

	cmdText, err := provider.GenerateCommand(context.Background(), promptStr, effectiveLanguage(cfg))
	if err != nil || strings.TrimSpace(cmdText) == "" {
		presenter.StopLoading(false)
		if err != nil {
			pterm.Error.Printfln("Failed to generate command: %v", err)
		} else {
			pterm.Error.Println("Provider returned empty command. Please refine your prompt or check provider configuration.")
		}
		os.Exit(1)
	}
	presenter.StopLoading(true)
	generatedCommand := strings.TrimSpace(cmdText)

	// Check if auto-execute is enabled (command line arguments take priority over config file)
	shouldAutoExecute := flagAutoExecute || cfg.UserPreferences.AutoExecute
	if shouldAutoExecute {
		pterm.Info.Println("Auto-executing command...")
		executeCommand(generatedCommand)
		return
	}

	// Interactive style consistent with hook flow
	for {
		sug := ui.Suggestion{
			Title:       "Generated Command",
			Explanation: "Based on your prompt",
			Command:     generatedCommand,
		}
		userInput, ok, err := presenter.Render(sug)
		if err != nil || !ok {
			return
		}
		if strings.TrimSpace(userInput) == "" {
			executeCommand(generatedCommand)
			return
		}

		// Regenerate command using new input as prompt
		presenter.ShowLoading("Generating new command...")
		cmdText, err := provider.GenerateCommand(context.Background(), userInput, effectiveLanguage(cfg))
		if err != nil || strings.TrimSpace(cmdText) == "" {
			presenter.StopLoading(false)
			if err != nil {
				pterm.Error.Printfln("Failed to generate command: %v", err)
			} else {
				pterm.Error.Println("Provider returned empty command. Please refine your prompt or check provider configuration.")
			}
			os.Exit(1)
		}
		presenter.StopLoading(true)
		generatedCommand = strings.TrimSpace(cmdText)
	}
}

func getProvider(providerName string, cfg config.ProviderConfig) (llm.Provider, error) {
	pm, err := prompt.NewManager("prompts.json")
	if err != nil {
		pm = prompt.NewDefaultManager()
	}
	return llm.GetProvider(providerName, cfg, pm)
}

func isProviderConfigIncomplete(providerName string, cfg config.ProviderConfig) bool {
	switch providerName {
	case config.ProviderOpenAI:
		return cfg.APIKey == "" || cfg.APIKey == "YOUR_OPENAI_API_KEY"
	case config.ProviderGemini:
		return cfg.APIKey == "" || cfg.APIKey == "YOUR_GEMINI_API_KEY"
	case config.ProviderGeminiCLI:
		return cfg.Project == "" || cfg.Project == "YOUR_GEMINI_PROJECT_ID"
	default:
		return true
	}
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the version number of aish",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("aish", versionString())
	},
}

func init() {
	// Make available commands display in the order they were added, ensuring init is first
	cobra.EnableCommandSorting = false
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(askCmd)
	rootCmd.AddCommand(hookCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(captureCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// Global flags
var (
	flagProvider    string
	flagLang        string
	flagDebug       bool
	flagPrompt      string
	flagAutoExecute bool // New auto-execute flag
)

// versionString is injected by ldflags: -X 'main._version=vX.Y.Z'
var _version string

func init() {
	// Persistent flags
	rootCmd.PersistentFlags().StringVar(&flagProvider, "provider", "", "override default provider for this run")
	rootCmd.PersistentFlags().StringVar(&flagLang, "lang", "", "override language for this run (e.g. en, zh-TW)")
	rootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "enable debug mode for verbose diagnostics")
	rootCmd.PersistentFlags().BoolVar(&flagAutoExecute, "auto-execute", false, "automatically execute generated commands without confirmation")
	rootCmd.Flags().StringVarP(&flagPrompt, "prompt", "p", "", "generates a command from a natural language prompt")

	// Enable debug mode (affects all subcommands)
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if flagDebug {
			os.Setenv(config.EnvAISHDebug, "1")
		}
	}

}

func effectiveProviderName(cfg *config.Config) string {
	if strings.TrimSpace(flagProvider) != "" {
		return flagProvider
	}
	return cfg.DefaultProvider
}

func effectiveLanguage(cfg *config.Config) string {
	if strings.TrimSpace(flagLang) != "" {
		return flagLang
	}
	return cfg.UserPreferences.Language
}

func versionString() string {
    if strings.TrimSpace(_version) == "" {
        return "v0.0.1"
    }
    return _version
}

// readTail reads the tail of a file up to maxBytes (returns empty string if path is empty or read fails)
func readTail(path string, maxBytes int) string {
	if path == "" {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return ""
	}
	size := fi.Size()
	if size <= 0 {
		return ""
	}
	var start int64 = 0
	if size > int64(maxBytes) {
		start = size - int64(maxBytes)
	}
	buf := make([]byte, size-start)
	_, _ = f.ReadAt(buf, start)
	return string(buf)
}
