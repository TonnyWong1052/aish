package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "strconv"
    "strings"
    "syscall"
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
Use 'aish ask "your question"' or 'aish -p "your question"' to generate a command.
Use 'aish -a "your question"' to get a plain-text AI answer (no command suggestion).`,
    Example: `  aish -p "create a folder named logs"
  aish -a "who are you?"
  aish ask "list files sorted by size"`,
    SilenceUsage:  true,  // avoid printing usage on errors we already handle
    SilenceErrors: true,  // let our UI/error handler own error messages
    Run: func(cmd *cobra.Command, args []string) {
        if flagAnswer != "" { // 新增：一般問答模式（純文字回答，不輸出建議指令）
            runAnswerLogic(flagAnswer)
            return
        }
        if flagPrompt != "" { // 既有：自然語言轉指令模式
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
			errorHandler := ui.NewErrorHandler(flagDebug)
		userErr := errorHandler.CreateConfigurationError(
			"AISH is active, but no LLM provider is configured.",
			[]string{
				"Run 'aish init' to configure an LLM provider",
				"Check your current configuration with 'aish config show'",
				"Verify your API keys are correctly set",
			},
		)
		errorHandler.HandleError(userErr)
			return
		}

        provider, err := getProvider(providerName, providerCfg)
        if err != nil {
            return
        }

        // 允許 Ctrl+C 取消生成,並確保不會殘留或重啟新的轉圈動畫
        ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
        defer stop()

        presenter := ui.NewPresenter()

        // 顯示錯誤觸發器清單,標記當前捕獲的錯誤類型
        presenter.ShowErrorTriggersList(string(errorType), cfg.UserPreferences.EnabledLLMTriggers)

        // 簡單的 loading 消息
        if err := presenter.ShowLoadingWithTimer("Analyzing with AI"); err != nil {
            // Spinner failed to start, but continue without it
            pterm.Warning.Printfln("Warning: Could not start loading animation: %v", err)
        }
        suggestion, err := provider.GetSuggestion(ctx, llm.CapturedContext{
            Command:  commandStr,
            Stdout:   stdoutStr,
            Stderr:   stderrStr,
            ExitCode: exitCode,
        }, effectiveLanguage(cfg))

        if ctx.Err() != nil { // 使用者中斷
            presenter.StopLoading(false)
            // 優雅結束：返回而非再次觸發 capture，避免重啟動畫
            return
        }
        if err != nil {
            presenter.StopLoading(false)
            errorHandler := ui.NewErrorHandler(flagDebug)
            userErr := errorHandler.CreateProviderError(
                "Failed to get AI suggestion for the error.",
                []string{
                    "Check your internet connection",
     "Verify your LLM provider configuration",
     "Try switching to a different provider with 'aish config set default_provider gemini-cli'",
     "Check if you've exceeded API rate limits",
    },
   )
   userErr.Cause = err
   errorHandler.HandleError(userErr)
   return
        }
  // Bug Fix: If provider returns (nil, nil), it would cause a panic.
  // This ensures we handle cases where no suggestion is generated without an explicit error.
  if suggestion == nil {
   presenter.StopLoading(false)
   errorHandler := ui.NewErrorHandler(flagDebug)
   userErr := errorHandler.CreateProviderError(
    "The AI provider returned an empty suggestion.",
    []string{
     "This can happen with certain errors or prompts.",
     "Try a different prompt or check the provider's status.",
     "You can also switch to another provider via 'aish config set default_provider <name>'.",
    },
   )
   errorHandler.HandleError(userErr)
   return
  }

        presenter.StopLoading(true)

        // Add visual separator before AI analysis
        pterm.Println()

  for {
   // UI Alignment: Use "Generated Command" as title to match the -p flow.
   uiSuggestion := ui.Suggestion{
    Title:       "Generated Command",
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
                // Generate new suggestion based on user input
                if err := presenter.ShowLoadingWithTimer("Command Generating"); err != nil {
                    pterm.Warning.Printfln("Warning: Could not start loading animation: %v", err)
                }
                suggestion, err = provider.GetSuggestion(ctx, llm.CapturedContext{
                    Command: userInput,
                }, cfg.UserPreferences.Language)
                if ctx.Err() != nil { // 使用者中斷
                    presenter.StopLoading(false)
                    return
                }
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
		errorHandler := ui.NewErrorHandler(flagDebug)
		userErr := errorHandler.CreateConfigurationError(
			"Unable to load AISH configuration.",
			[]string{
				"Run 'aish init' to create initial configuration",
				"Check if configuration file is corrupted",
				"Verify ~/.config/aish/ directory permissions",
			},
		)
		userErr.Cause = err
		errorHandler.HandleError(userErr)
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
		errorHandler := ui.NewErrorHandler(flagDebug)
		userErr := errorHandler.CreateConfigurationError(
			"No LLM provider configured or configuration incomplete.",
			[]string{
				"Run 'aish init' to configure an LLM provider",
				"Check your current configuration with 'aish config show'",
				"Verify your API keys are correctly set",
			},
		)
		errorHandler.HandleError(userErr)
		os.Exit(1)
	}

    // 支援 Ctrl+C 優雅取消
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    presenter := ui.NewPresenter()
    // Use consistent loading label across prompt and hook flows
    if err := presenter.ShowLoadingWithTimer("Command Generating"); err != nil {
        pterm.Warning.Printfln("Warning: Could not start loading animation: %v", err)
    }

    cmdText, err := provider.GenerateCommand(ctx, promptStr, effectiveLanguage(cfg))
    if ctx.Err() != nil { // 使用者中斷
        presenter.StopLoading(false)
        return
    }
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
    // Track the latest prompt that produced the current command
    currentPrompt := promptStr

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
            Explanation: generateFallbackExplanation(currentPrompt, generatedCommand, effectiveLanguage(cfg)),
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

        // Regenerate command using new input as prompt (same label for consistency)
        if err := presenter.ShowLoadingWithTimer("Command Generating"); err != nil {
            pterm.Warning.Printfln("Warning: Could not start loading animation: %v", err)
        }
        cmdText, err := provider.GenerateCommand(ctx, userInput, effectiveLanguage(cfg))
        if ctx.Err() != nil { // 使用者中斷
            presenter.StopLoading(false)
            return
        }
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
        currentPrompt = strings.TrimSpace(userInput)
    }
}

// runAnswerLogic 以一般問答模式處理使用者輸入，僅輸出純文字答案，不提供指令建議或執行。
func runAnswerLogic(question string) {
    cfg, err := config.Load()
    if err != nil {
        errorHandler := ui.NewErrorHandler(flagDebug)
        userErr := errorHandler.CreateConfigurationError(
            "Unable to load AISH configuration.",
            []string{
                "Run 'aish init' to create initial configuration",
                "Check if configuration file is corrupted",
                "Verify ~/.config/aish/ directory permissions",
            },
        )
        userErr.Cause = err
        errorHandler.HandleError(userErr)
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
        errorHandler := ui.NewErrorHandler(flagDebug)
        userErr := errorHandler.CreateConfigurationError(
            "No LLM provider configured or configuration incomplete.",
            []string{
                "Run 'aish init' to configure an LLM provider",
                "Check your current configuration with 'aish config show'",
                "Verify your API keys are correctly set",
            },
        )
        errorHandler.HandleError(userErr)
        os.Exit(1)
    }

    // 支援 Ctrl+C 優雅取消
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    presenter := ui.NewPresenter()
    if err := presenter.ShowLoadingWithTimer("Answer Generating"); err != nil {
        pterm.Warning.Printfln("Warning: Could not start loading animation: %v", err)
    }

    // 重用 GenerateCommand：若屬一般問答，提示模板會回傳 echo 指令，其內容即為答案。
    cmdText, err := provider.GenerateCommand(ctx, question, effectiveLanguage(cfg))
    if ctx.Err() != nil { // 使用者中斷
        presenter.StopLoading(false)
        return
    }
    if err != nil || strings.TrimSpace(cmdText) == "" {
        presenter.StopLoading(false)
        if err != nil {
            pterm.Error.Printfln("Failed to get answer: %v", err)
        } else {
            pterm.Error.Println("Provider returned empty answer. Please refine your question or check provider configuration.")
        }
        os.Exit(1)
    }
    presenter.StopLoading(true)

    // 嘗試從 echo 指令抽取文字內容
    if ans, ok := extractEchoText(cmdText); ok {
        pterm.DefaultHeader.Println("AI Answer")
        pterm.Println(ans)
        return
    }

    // 若非 echo 指令，為避免顯示或執行指令，僅以純文字回應該指令字串
    pterm.DefaultHeader.Println("AI Answer")
    pterm.Println(cmdText)
}

// extractEchoText 嘗試從 echo/printf 形式的指令中抽取被引號包裹的文字內容。
// 支援：echo '...'
//      echo "..."
//      echo -n/-e '...'
//      printf '%s' '...'
func extractEchoText(cmd string) (string, bool) {
    s := strings.TrimSpace(cmd)
    // 去除可能的包裝（例如結尾分號）
    s = strings.TrimSuffix(s, ";")
    lower := strings.ToLower(s)

    // 僅處理以 echo/printf 開頭者
    if strings.HasPrefix(lower, "echo ") {
        body := strings.TrimSpace(s[len("echo "):])
        // 跳過旗標（例如 -n, -e 等）
        for strings.HasPrefix(body, "-") {
            // 取下一個 token
            parts := strings.Fields(body)
            if len(parts) <= 1 {
                return "", false
            }
            body = strings.TrimSpace(strings.TrimPrefix(body, parts[0]))
        }
        txt, ok := stripQuotes(body)
        if ok {
            return txt, true
        }
        // 沒有引號時，直接回傳剩餘字串
        if body != "" {
            return body, true
        }
        return "", false
    }
    if strings.HasPrefix(lower, "printf ") {
        body := strings.TrimSpace(s[len("printf "):])
        // 嘗試擷取最後一個引號包住的段落作為主要內容
        if txt, ok := lastQuotedSegment(body); ok {
            return txt, true
        }
        return "", false
    }
    return "", false
}

// stripQuotes 若字串以成對單/雙引號包裹，移除引號
func stripQuotes(in string) (string, bool) {
    t := strings.TrimSpace(in)
    if len(t) >= 2 {
        if (t[0] == '\'' && t[len(t)-1] == '\'') || (t[0] == '"' && t[len(t)-1] == '"') {
            return t[1 : len(t)-1], true
        }
    }
    return in, false
}

// lastQuotedSegment 尋找最後一段被單/雙引號包裹的文字
func lastQuotedSegment(in string) (string, bool) {
    s := strings.TrimSpace(in)
    // 從尾端掃描，尋找匹配引號
    for i := len(s) - 1; i >= 0; i-- {
        if s[i] == '\'' || s[i] == '"' {
            // 找前一個相同引號
            quote := s[i]
            // 向前尋找配對
            for j := i - 1; j >= 0; j-- {
                if s[j] == quote {
                    return s[j+1 : i], true
                }
            }
        }
    }
    return "", false
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
        // 放寬檢查：允許在執行期自動解析（僅從 ~/.config/aish/gemini_oauth_creds.json）
        // 即使 Project 未在 config 中，仍視為可啟動，交由 provider 解析
        return false
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
    flagAnswer      string
    flagAutoExecute bool // New auto-execute flag
)

// versionString is injected by ldflags: -X 'main._version=vX.Y.Z'
var _version string

func init() {
	// Persistent flags
	rootCmd.PersistentFlags().StringVar(&flagProvider, "provider", "", "override default provider for this run")
	rootCmd.PersistentFlags().StringVar(&flagLang, "lang", "", "override language for this run (e.g. en, zh-TW)")
	rootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "enable debug mode for verbose diagnostics")
    // Switch primary flag name from --auto-execute to --auto
    rootCmd.PersistentFlags().BoolVar(&flagAutoExecute, "auto", false, "automatically execute generated commands without confirmation")
    // Backward compatibility: keep --auto-execute as a hidden deprecated alias
    rootCmd.PersistentFlags().BoolVar(&flagAutoExecute, "auto-execute", false, "(deprecated) use --auto instead")
    _ = rootCmd.PersistentFlags().MarkDeprecated("auto-execute", "use --auto instead")
    _ = rootCmd.PersistentFlags().MarkHidden("auto-execute")
    rootCmd.Flags().StringVarP(&flagPrompt, "prompt", "p", "", "generates a command from a natural language prompt")
    rootCmd.Flags().StringVarP(&flagAnswer, "answer", "a", "", "answer a general question with plain text")

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
        return "v0.0.2"
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

// generateFallbackExplanation creates a human-friendly explanation for a generated command
// when the provider did not supply one. It references the user's prompt and gives a brief
// rationale for common commands. Defaults to English; returns Traditional Chinese for zh/zh-TW.
func generateFallbackExplanation(promptStr, cmd, lang string) string {
    fields := strings.Fields(cmd)
    if len(fields) == 0 {
        return ""
    }
    base := fields[0]

    // detect zh language preference
    useZH := strings.HasPrefix(strings.ToLower(strings.TrimSpace(lang)), "zh")

    if useZH {
        var detailZH string
        switch base {
        case "mkdir":
            detailZH = "建立資料夾；例如 `mkdir hi` 會在目前目錄建立名為 hi 的資料夾。若包含不存在的父層，改用 `mkdir -p <路徑>`。"
        case "rm":
            detailZH = "刪除檔案或資料夾；操作具破壞性，請謹慎使用（尤其是 `-rf`）。建議先用 `ls` 確認目標。"
        case "mv":
            detailZH = "移動或重新命名檔案/資料夾。"
        case "cp":
            detailZH = "複製檔案或資料夾；遞迴複製資料夾需加 `-r`。"
        case "ls":
            detailZH = "列出目前目錄內容；可搭配 `-l` 顯示詳細資訊。"
        case "grep":
            detailZH = "在文字中搜尋關鍵字；可搭配 `-r` 遞迴搜尋。"
        case "find":
            detailZH = "依條件在檔案系統尋找檔案/資料夾。"
        case "curl":
            detailZH = "發送 HTTP 請求；常用 `-L` 追蹤重新導向、`-o` 輸出到檔案。"
        case "git":
            detailZH = "Git 版本控制操作；請在版本庫目錄下執行對應子命令。"
        }
        if detailZH != "" {
            return fmt.Sprintf("根據你的提示：「%s」。\n此命令 `%s` 用於：%s", promptStr, cmd, detailZH)
        }
        return fmt.Sprintf("根據你的提示：「%s」，建議的最小可行命令為：`%s`。", promptStr, cmd)
    }

    // English default
    var detailEN string
    switch base {
    case "mkdir":
        detailEN = "Create a directory; e.g., `mkdir hi` creates folder `hi` in the current path. Use `mkdir -p <path>` to include missing parents."
    case "rm":
        detailEN = "Remove files or directories; destructive action (especially with `-rf`). Consider verifying targets with `ls` first."
    case "mv":
        detailEN = "Move or rename files/directories."
    case "cp":
        detailEN = "Copy files or directories; use `-r` for recursive directory copy."
    case "ls":
        detailEN = "List directory contents; add `-l` for details."
    case "grep":
        detailEN = "Search text for a pattern; use `-r` to search recursively."
    case "find":
        detailEN = "Find files/directories by conditions in the filesystem."
    case "curl":
        detailEN = "Send HTTP requests; commonly `-L` to follow redirects, `-o` to write to file."
    case "git":
        detailEN = "Git version control operations; run appropriate subcommands in a repository."
    }
    if detailEN != "" {
        return fmt.Sprintf("Based on your prompt: \"%s\".\nThe command `%s` is for: %s", promptStr, cmd, detailEN)
    }
    return fmt.Sprintf("Based on your prompt: \"%s\", the minimal viable command is: `%s`.", promptStr, cmd)
}
