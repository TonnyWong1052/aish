package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"powerful-cli/internal/classification"
	"powerful-cli/internal/config"
	"powerful-cli/internal/history"
	"powerful-cli/internal/llm"
	_ "powerful-cli/internal/llm/gemini"
	_ "powerful-cli/internal/llm/gemini-cli"
	_ "powerful-cli/internal/llm/openai"
	"powerful-cli/internal/prompt"
	"powerful-cli/internal/ui"

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
		// 安全性調整：不再重跑上一條命令以獲取輸出，避免副作用與高耗時。
		// 若 hook 已將輸出寫入暫存檔，透過環境變數讀取並截取尾部內容（避免超大字串）。
		const maxCaptureBytes = 200_000
		stdoutStr := readTail(os.Getenv("AISH_STDOUT_FILE"), maxCaptureBytes)
		stderrStr := readTail(os.Getenv("AISH_STDERR_FILE"), maxCaptureBytes)

		classifier := classification.NewClassifier()
		errorType := classifier.Classify(exitCode, stderrStr)
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

		presenter := ui.NewPresenter()
		presenter.ShowLoading("Generate command animate[as aish -p also has it]")

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
	presenter.ShowLoading("Generate command animate[as aish -p also has it]")

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

	// 檢查是否啟用自動執行（命令列參數優先於配置檔案）
	shouldAutoExecute := flagAutoExecute || cfg.UserPreferences.AutoExecute
	if shouldAutoExecute {
		pterm.Info.Println("Auto-executing command...")
		executeCommand(generatedCommand)
		return
	}

	// 與 hook 流程統一的互動樣式
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

		// 以新輸入作為提示重新產生指令
		presenter.ShowLoading("Generate command animate[as aish -p also has it]")
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
	case "openai":
		return cfg.APIKey == "" || cfg.APIKey == "YOUR_OPENAI_API_KEY"
	case "gemini":
		return cfg.APIKey == "" || cfg.APIKey == "YOUR_GEMINI_API_KEY"
	case "gemini-cli":
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
	// 讓可用命令的顯示順序遵循加入順序，確保 init 排在第一
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

// 全域旗標
var (
	flagProvider    string
	flagLang        string
	flagDebug       bool
	flagPrompt      string
	flagAutoExecute bool // 新增自動執行旗標
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
			os.Setenv("AISH_DEBUG", "1")
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

// readTail 讀取檔案尾端最多 maxBytes 的內容（若路徑為空或讀取失敗，回傳空字串）
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
