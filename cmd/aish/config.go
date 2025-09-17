package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"powerful-cli/internal/config"
	"powerful-cli/internal/llm"
	"powerful-cli/internal/llm/openai"
	"powerful-cli/internal/prompt"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration settings",
	Run: func(cmd *cobra.Command, args []string) {
		runConfigureLogic(cmd, args)
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			pterm.Error.Printfln("Failed to load config: %v", err)
			os.Exit(1)
		}

		pterm.DefaultSection.Println("Current Configuration")

		providerCfg, ok := cfg.Providers[cfg.DefaultProvider]
		if !ok {
			pterm.Warning.Println("Default provider not set. Please run 'aish config'.")
			return
		}

		items := []pterm.BulletListItem{
			{Level: 0, Text: fmt.Sprintf("Default Provider: %s", cfg.DefaultProvider)},
			{Level: 1, Text: fmt.Sprintf("API Host: %s", providerCfg.APIEndpoint)},
			{Level: 1, Text: fmt.Sprintf("Model: %s", providerCfg.Model)},
		}
		if cfg.DefaultProvider == "gemini-cli" && strings.TrimSpace(providerCfg.Project) != "" {
			items = append(items, pterm.BulletListItem{Level: 1, Text: fmt.Sprintf("Project: %s", hideIfSet(providerCfg.Project))})
		}
		pterm.DefaultBulletList.WithItems(items).Render()
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := strings.TrimSpace(args[0])
		cfg, err := config.Load()
		if err != nil {
			pterm.Error.Printfln("Failed to load config: %v", err)
			os.Exit(1)
		}
		lower := strings.ToLower(key)
		switch lower {
		case "default_provider":
			fmt.Println(cfg.DefaultProvider)
			return
		case "user_preferences.language", "language":
			fmt.Println(cfg.UserPreferences.Language)
			return
		case "auto_execute", "auto-execute", "user_preferences.auto_execute":
			if cfg.UserPreferences.AutoExecute {
				fmt.Println("true")
			} else {
				fmt.Println("false")
			}
			return
		}
		if strings.HasPrefix(lower, "providers.") {
			parts := strings.Split(lower, ".")
			if len(parts) != 3 {
				pterm.Error.Println("Use providers.<name>.<field>, fields: api_endpoint|model|api_key|project")
				os.Exit(1)
			}
			name := parts[1]
			field := parts[2]
			pc, ok := cfg.Providers[name]
			if !ok {
				pterm.Error.Printfln("Provider not found: %s", name)
				os.Exit(1)
			}
			switch field {
			case "api_endpoint":
				fmt.Println(pc.APIEndpoint)
			case "model":
				fmt.Println(pc.Model)
			case "api_key":
				fmt.Println(maskIfSet(pc.APIKey))
			case "project":
				fmt.Println(hideIfSet(pc.Project))
			default:
				pterm.Error.Println("Unknown field. Use one of: api_endpoint|model|api_key|project")
				os.Exit(1)
			}
			return
		}
		pterm.Error.Printfln("Unsupported key: %s", key)
		os.Exit(1)
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key := strings.TrimSpace(args[0])
		value := strings.TrimSpace(args[1])
		cfg, err := config.Load()
		if err != nil {
			pterm.Error.Printfln("Failed to load config: %v", err)
			os.Exit(1)
		}
		lower := strings.ToLower(key)
		switch lower {
		case "default_provider":
			if _, ok := cfg.Providers[value]; !ok {
				pterm.Error.Printfln("Unknown provider: %s", value)
				os.Exit(1)
			}
			cfg.DefaultProvider = value
		case "user_preferences.language", "language":
			cfg.UserPreferences.Language = value
		case "auto_execute", "auto-execute", "user_preferences.auto_execute":
			switch strings.ToLower(value) {
			case "true", "1", "yes", "on", "enable", "enabled":
				cfg.UserPreferences.AutoExecute = true
			case "false", "0", "no", "off", "disable", "disabled":
				cfg.UserPreferences.AutoExecute = false
			default:
				pterm.Error.Printfln("Invalid value for auto_execute: %s. Use: true/false, 1/0, yes/no, on/off", value)
				os.Exit(1)
			}
		default:
			if strings.HasPrefix(lower, "providers.") {
				parts := strings.Split(lower, ".")
				if len(parts) != 3 {
					pterm.Error.Println("Use providers.<name>.<field>, fields: api_endpoint|model|api_key|project")
					os.Exit(1)
				}
				name := parts[1]
				field := parts[2]
				pc := cfg.Providers[name]
				switch field {
				case "api_endpoint":
					pc.APIEndpoint = value
				case "model":
					pc.Model = value
				case "api_key":
					pc.APIKey = value
				case "project":
					pc.Project = value
				default:
					pterm.Error.Println("Unknown field. Use one of: api_endpoint|model|api_key|project")
					os.Exit(1)
				}
				cfg.Providers[name] = pc
			} else {
				pterm.Error.Printfln("Unsupported key: %s", key)
				os.Exit(1)
			}
		}
		if err := cfg.Save(); err != nil {
			pterm.Error.Printfln("Failed to save config: %v", err)
			os.Exit(1)
		}
		pterm.Success.Println("Updated.")
	},
}

// runConfigureLogic contains the logic from the original configureCmd
func runConfigureLogic(cmd *cobra.Command, args []string) {
	// 若未明確指定 --interactive，則在 TTY 中預設啟用互動式精靈
	interactiveFlag, _ := cmd.Flags().GetBool("interactive")
	interactive := interactiveFlag
	if !cmd.Flags().Changed("interactive") {
		interactive = isInteractiveTTY()
	}

	cfg, err := config.Load()
	if err != nil {
		pterm.Error.Printfln("Failed to load config: %v", err)
		os.Exit(1)
	}
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]config.ProviderConfig)
	}

	// 僅在 interactive 旗標為 true 且 TTY 可用時執行 TUI 精靈
	if interactive && isInteractiveTTY() {
		pterm.Info.Println("Running interactive TUI configuration...")
	} else {
		if err := plainConfigureWizard(cfg); err != nil {
			pterm.Error.Printfln("Configuration failed: %v", err)
			os.Exit(1)
		}
		pterm.Success.Println("Configuration saved.")
		return
	}

	// 1) 選擇提供商（箭頭上下）
	type provItem struct{ label, key string }
	items := []provItem{{"OpenAI", "openai"}, {"Gemini", "gemini"}, {"Gemini CLI", "gemini-cli"}}
	// 預設值（以 label 呈現）
	defaultKey := cfg.DefaultProvider
	if defaultKey == "" {
		defaultKey = "openai"
	}
	defaultLabel := "OpenAI"
	for _, it := range items {
		if it.key == defaultKey {
			defaultLabel = it.label
			break
		}
	}

	pterm.Println("Default provider:")
	// 顯示互動式選單
	var labels []string
	for _, it := range items {
		labels = append(labels, it.label)
	}
	selLabel, err := pterm.DefaultInteractiveSelect.
		WithOptions(labels).
		WithDefaultOption(defaultLabel).
		Show("")
	if err != nil {
		_ = plainConfigureWizard(cfg)
		pterm.Success.Println("Configuration saved.")
		return
	}
	selProvider := defaultKey
	for _, it := range items {
		if it.label == selLabel {
			selProvider = it.key
			break
		}
	}
	cfg.DefaultProvider = selProvider

	// 準備現有 provider 設定或預設
	pc := cfg.Providers[selProvider]
	switch selProvider {
	case "openai":
		if pc.APIEndpoint == "" {
			pc.APIEndpoint = "https://api.openai.com/v1"
		}
		if pc.Model == "" {
			pc.Model = "gpt-4"
		}
	case "gemini":
		if pc.APIEndpoint == "" {
			pc.APIEndpoint = "https://generativelanguage.googleapis.com/v1"
		}
		if pc.Model == "" {
			pc.Model = "gemini-pro"
		}
	case "gemini-cli":
		// Gemini CLI 的 API endpoint 固定為完整路徑，不提供使用者修改
		pc.APIEndpoint = "https://cloudcode-pa.googleapis.com/v1internal:generateContent"
		if pc.Model == "" {
			pc.Model = "gemini-2.5-flash"
		}
	}

	// 2) API Endpoint（Gemini CLI 固定，其他可輸入）
	reader := bufio.NewReader(os.Stdin)
	if selProvider != "gemini-cli" {
		pterm.Println(fmt.Sprintf("API endpoint [%s]:", pc.APIEndpoint))
		fmt.Print(">: ")
		ep, _ := reader.ReadString('\n')
		if ep := strings.TrimSpace(ep); ep != "" {
			pc.APIEndpoint = ep
		}
	} else {
		// API endpoint is fixed for Gemini CLI, no need to display
	}

	// 3) 憑證/專案輸入
	switch selProvider {
	case "openai":
		// Ask about legacy endpoint *after* setting the endpoint URL
		omitV1, err := pterm.DefaultInteractiveConfirm.
			WithDefaultValue(pc.OmitV1Prefix).
			Show("Is this a legacy endpoint that omits the /v1 prefix (e.g., GitHub Copilot)?")
		if err == nil {
			pc.OmitV1Prefix = omitV1
			if !omitV1 {
				pterm.Info.Println("The /v1 path will be automatically appended for standard API calls.")
			}
		}

		// API Key（掩碼）
		pterm.Println("OpenAI API key (leave empty to skip):")
		fmt.Print(">: ")
		bytePassword, _ := term.ReadPassword(int(os.Stdin.Fd()))
		k := string(bytePassword)
		fmt.Println() // Add a newline after password input
		if strings.TrimSpace(k) != "" {
			pc.APIKey = strings.TrimSpace(k)
		}
	case "gemini":
		pterm.Println("Gemini API key (leave empty to skip):")
		fmt.Print(">: ")
		bytePassword, _ := term.ReadPassword(int(os.Stdin.Fd()))
		k := string(bytePassword)
		fmt.Println() // Add a newline after password input
		if strings.TrimSpace(k) != "" {
			pc.APIKey = strings.TrimSpace(k)
		}
	case "gemini-cli":
		// 顯示官方文件連結以協助設定 Workspace GCA
		pterm.Info.Printfln("Docs: Gemini CLI Workspace GCA: %s", "https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/authentication.md#workspace-gca")
		// 隱藏已設定的 Project ID，避免洩漏敏感資訊
		disp := hideIfSet(pc.Project)
		pterm.Println(fmt.Sprintf("Google Cloud project ID [%s]:", disp))
		fmt.Print(">: ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line != "" {
			pc.Project = line
		}
	}

	// 4) Model
	if selProvider == "gemini-cli" {
		// 僅支援 2.5 系列
		allowed := []string{"gemini-2.5-pro", "gemini-2.5-flash"}
		// 預設選項：若當前不在允許清單中，回退為 gemini-2.5-flash
		defaultModel := pc.Model
		inAllowed := false
		for _, m := range allowed {
			if m == defaultModel {
				inAllowed = true
				break
			}
		}
		if !inAllowed || defaultModel == "" {
			defaultModel = "gemini-2.5-flash"
		}

		pterm.Println("Model:")
		selModel, err := pterm.DefaultInteractiveSelect.
			WithOptions(allowed).
			WithDefaultOption(defaultModel).
			Show("")
		if err == nil && selModel != "" {
			pc.Model = selModel
		} else {
			// 後備：保持原值或設為預設
			pc.Model = defaultModel
		}
	} else { // For openai and gemini
		// Ask user if they want to fetch models from API
		fetchModels, _ := pterm.DefaultInteractiveConfirm.
			WithDefaultValue(true).
			Show("Automatically fetch available models from the API?")

		var newModel string
		var availableModels []string
		var err error

		if fetchModels {
			if strings.TrimSpace(pc.APIKey) == "" {
				pterm.Warning.Println("API key is not set. Skipping model list fetch.")
				err = fmt.Errorf("API key is required to fetch models")
			} else {
				// Create a temporary provider to fetch models
				// A nil prompt manager is safe because GetAvailableModels doesn't use it.
				var tempProvider llm.Provider
				tempProvider, err = openai.NewProvider(pc, (*prompt.Manager)(nil))

				if err == nil {
					// Type assert to the concrete OpenAIProvider to access GetAvailableModels
					if oaiProvider, ok := tempProvider.(*openai.OpenAIProvider); ok {
						spinner, _ := pterm.DefaultSpinner.Start("Fetching available models...")
						availableModels, err = oaiProvider.GetAvailableModels(context.Background())
						if err != nil {
							spinner.Fail(fmt.Sprintf("Failed to fetch models: %v", err))
						} else if len(availableModels) == 0 {
							spinner.Warning("No available models found.")
						} else {
							spinner.Success(fmt.Sprintf("Found %d models.", len(availableModels)))
						}
					} else {
						err = fmt.Errorf("failed to assert provider to *openai.OpenAIProvider")
					}
				}
			}
		}

		if err == nil && len(availableModels) > 0 {
			// Let user select from the list
			// Set a reasonable default
			defaultModel := pc.Model
			isDefaultInList := false
			for _, m := range availableModels {
				if m == defaultModel {
					isDefaultInList = true
					break
				}
			}
			if !isDefaultInList {
				// Try to find a good default
				for _, m := range []string{"gpt-4o", "gpt-4", "gpt-3.5-turbo"} {
					for _, am := range availableModels {
						if m == am {
							defaultModel = m
							goto foundDefault
						}
					}
				}
			foundDefault:
			}

			newModel, _ = pterm.DefaultInteractiveSelect.
				WithOptions(availableModels).
				WithDefaultOption(defaultModel).
				Show("Select a model")
		} else {
			// Fallback to manual input
			pterm.Println(fmt.Sprintf("Model name [%s]:", pc.Model))
			fmt.Print(">: ")
			m, _ := reader.ReadString('\n')
			newModel = m
		}

		if m := strings.TrimSpace(newModel); m != "" {
			pc.Model = m
		}
	}

	// 寫回 provider 設定
	cfg.Providers[selProvider] = pc

	// 5) 語言
	langs := []string{"english", "chinese", "japanese"}
	curLang := cfg.UserPreferences.Language
	if curLang == "" {
		curLang = "english"
	}
	pterm.Println("Language:")
	selLang, err := pterm.DefaultInteractiveSelect.
		WithOptions(langs).
		WithDefaultOption(curLang).
		Show("")
	if err != nil {
		_ = plainConfigureWizard(cfg)
		pterm.Success.Println("Configuration saved.")
		return
	}
	cfg.UserPreferences.Language = selLang

	// 6) 觸發器（多選）
	// 說明：將長提示移出互動組件，避免在窄終端換行導致每次箭頭重繪時
	//      重複輸出標題（觀察到的重複 "Enable auto-trigger..." 問題）。
	pterm.DefaultHeader.Println("Auto-trigger Error Types")
	pterm.Info.Println("Use arrows to move, space to toggle, enter to confirm.")
	triggerOptions := []string{
		"CommandNotFound",
		"FileNotFoundOrDirectory",
		"PermissionDenied",
		"CannotExecute",
		"InvalidArgumentOrOption",
		"ResourceExists",
		"NotADirectory",
		"TerminatedBySignal",
		"GenericError",
	}
	defaultTriggers := cfg.UserPreferences.EnabledLLMTriggers
	if len(defaultTriggers) == 0 {
		// 推薦預設：大部分錯誤都啟用
		defaultTriggers = []string{
			"CommandNotFound", "FileNotFoundOrDirectory", "PermissionDenied", "CannotExecute",
			"InvalidArgumentOrOption", "ResourceExists", "NotADirectory", "TerminatedBySignal", "GenericError",
		}
	}
	selTriggers, err := pterm.DefaultInteractiveMultiselect.
		WithOptions(triggerOptions).
		WithDefaultOptions(defaultTriggers).
		// 將顯示字串設為空，避免長行換行造成的重疊/殘留
		Show("")
	if err != nil {
		_ = plainConfigureWizard(cfg)
		pterm.Success.Println("Configuration saved.")
		return
	}
	cfg.UserPreferences.EnabledLLMTriggers = selTriggers

	// 7) 啟用 aish
	cfg.Enabled = true

	if err := cfg.Save(); err != nil {
		pterm.Error.Printfln("Failed to save config: %v", err)
		os.Exit(1)
	}

	pterm.Success.Println("Configuration saved.")
}

// maskIfSet 將非空鍵遮罩顯示
func maskIfSet(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	if len(v) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(v)-4) + v[len(v)-4:]
}

// hideIfSet hides sensitive values completely for display
// to avoid leaking real identifiers in prompts.
func hideIfSet(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	return "hidden"
}

// isInteractiveTTY 檢查是否在交互式 TTY 環境
func isInteractiveTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// plainConfigureWizard 提供不依賴 TUI 的純文字配置流程
func plainConfigureWizard(cfg *config.Config) error {
	reader := bufio.NewReader(os.Stdin)

	// --- Provider Selection ---
	defProv := cfg.DefaultProvider
	if defProv == "" {
		defProv = "openai"
	}
	fmt.Println("Default provider:")
	fmt.Println("  - openai")
	fmt.Println("  - gemini")
	fmt.Println("  - gemini-cli")
	fmt.Printf("[current: %s]\n>: ", defProv)
	prov, _ := reader.ReadString('\n')
	prov = strings.TrimSpace(prov)
	if prov == "" {
		prov = defProv
	}
	cfg.DefaultProvider = prov
	fmt.Println() // Add spacing

	// --- Provider-specific Configuration ---
	pc := cfg.Providers[prov]
	switch prov {
	case "openai":
		if pc.APIEndpoint == "" {
			pc.APIEndpoint = "https://api.openai.com/v1"
		}
		if pc.Model == "" {
			pc.Model = "gpt-4"
		}
	case "gemini":
		if pc.APIEndpoint == "" {
			pc.APIEndpoint = "https://generativelanguage.googleapis.com/v1"
		}
		if pc.Model == "" {
			pc.Model = "gemini-pro"
		}
	case "gemini-cli":
		// Gemini CLI 的 API endpoint 固定為完整路徑，不提供使用者修改
		pc.APIEndpoint = "https://cloudcode-pa.googleapis.com/v1internal:generateContent"
		if pc.Model == "" {
			pc.Model = "gemini-2.5-flash"
		}
	}

	// --- API Endpoint ---
	if prov != "gemini-cli" {
		fmt.Printf("API endpoint [%s]:\n>: ", pc.APIEndpoint)
		ep, _ := reader.ReadString('\n')
		ep = strings.TrimSpace(ep)
		if ep != "" {
			pc.APIEndpoint = ep
		}
		fmt.Println()
	} else {
		// API endpoint is fixed for Gemini CLI, no need to display
		fmt.Println()
	}

	// --- Credentials / Project ---
	switch prov {
	case "openai":
		// Ask about legacy endpoint in plain text mode as well
		fmt.Print("Is this a legacy endpoint that omits the /v1 prefix? (y/N)\n> ")
		yn, _ := reader.ReadString('\n')
		yn = strings.TrimSpace(strings.ToLower(yn))
		if yn == "y" || yn == "yes" {
			pc.OmitV1Prefix = true
		} else {
			pc.OmitV1Prefix = false
			fmt.Println("Info: The /v1 path will be automatically appended for standard API calls.")
		}
		fmt.Println()

		fmt.Println("OpenAI API key (leave empty to skip)")
		fmt.Print(">: ")
		k, _ := reader.ReadString('\n')
		k = strings.TrimSpace(k)
		if k != "" {
			pc.APIKey = k
		}
	case "gemini":
		fmt.Println("Gemini API key (leave empty to keep current)")
		fmt.Print(">: ")
		k, _ := reader.ReadString('\n')
		k = strings.TrimSpace(k)
		if k != "" {
			pc.APIKey = k
		}
	case "gemini-cli":
		// 顯示官方文件連結以協助設定 Workspace GCA
		fmt.Printf("Docs: Gemini CLI Workspace GCA: %s\n", "https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/authentication.md#workspace-gca")
		// 隱藏已設定的 Project ID，避免洩漏敏感資訊
		disp := hideIfSet(pc.Project)
		fmt.Printf("Google Cloud project ID [%s]:\n>: ", disp)
		prj, _ := reader.ReadString('\n')
		prj = strings.TrimSpace(prj)
		if prj != "" {
			pc.Project = prj
		}
	}
	fmt.Println()

	// --- Model ---
	if prov == "gemini-cli" {
		// 僅保留 2.5 系列
		allowed := []string{"gemini-2.5-pro", "gemini-2.5-flash"}
		// 顯示選項並允許輸入序號或完整名稱；空輸入採預設
		// 預設值：若當前不在允許清單，回退為 gemini-2.5-flash
		defModel := pc.Model
		valid := false
		for _, a := range allowed {
			if a == defModel {
				valid = true
				break
			}
		}
		if !valid || defModel == "" {
			defModel = "gemini-2.5-flash"
		}

		fmt.Println("Model (choose one):")
		for i, a := range allowed {
			fmt.Printf("  %d) %s\n", i+1, a)
		}
		fmt.Printf("[current: %s]\n>: ", defModel)
		m, _ := reader.ReadString('\n')
		m = strings.TrimSpace(m)
		switch m {
		case "1", allowed[0]:
			pc.Model = allowed[0]
		case "2", allowed[1]:
			pc.Model = allowed[1]
		case "":
			pc.Model = defModel
		default:
			// 無效輸入則沿用預設
			pc.Model = defModel
		}
	} else {
		fmt.Printf("Model name [%s]:\n>: ", pc.Model)
		m, _ := reader.ReadString('\n')
		m = strings.TrimSpace(m)
		if m != "" {
			pc.Model = m
		}
	}
	cfg.Providers[prov] = pc
	fmt.Println()

	// --- Optional: Test Connection (OpenAI only) ---
	if prov == "openai" {
		fmt.Print("Test connection now? (y/N)\n> ")
		yn, _ := reader.ReadString('\n')
		yn = strings.TrimSpace(strings.ToLower(yn))
		if yn == "y" || yn == "yes" {
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()
			// 這裡直接使用 OpenAI provider 對 /models 發出請求（採用 POST 優先，405 回退 GET）
			tempProv, err := openai.NewProvider(pc, (*prompt.Manager)(nil))
			if err != nil {
				fmt.Printf("Connection test unavailable: %v\n\n", err)
			} else {
				if oai, ok := tempProv.(*openai.OpenAIProvider); ok {
					models, verr := oai.GetAvailableModels(ctx)
					if verr != nil {
						fmt.Printf("Connection test failed: %v\n\n", verr)
					} else {
						// 顯示少量結果即可
						preview := models
						if len(preview) > 5 {
							preview = preview[:5]
						}
						if len(models) > 0 {
							fmt.Printf("Connection OK. Models available (sample): %v\n\n", preview)
						} else {
							fmt.Printf("Connection OK but no models returned.\n\n")
						}
					}
				} else {
					fmt.Printf("Connection test unavailable: provider type mismatch\n\n")
				}
			}
		}
	}

	// --- Language ---
	defLang := cfg.UserPreferences.Language
	if defLang == "" {
		defLang = "english"
	}
	fmt.Println("Language:")
	fmt.Println("  - english")
	fmt.Println("  - chinese")
	fmt.Println("  - japanese")
	fmt.Printf("[current: %s]\n>: ", defLang)
	lang, _ := reader.ReadString('\n')
	lang = strings.TrimSpace(lang)
	if lang == "" {
		lang = defLang
	}
	cfg.UserPreferences.Language = lang
	fmt.Println()

	// --- Triggers ---
	fmt.Print("Enable default error triggers? (Y/n)\n> ")
	yn, _ := reader.ReadString('\n')
	yn = strings.TrimSpace(strings.ToLower(yn))
	if yn == "" || yn == "y" || yn == "yes" {
		cfg.UserPreferences.EnabledLLMTriggers = []string{
			"CommandNotFound", "FileNotFoundOrDirectory", "PermissionDenied", "CannotExecute",
			"InvalidArgumentOrOption", "ResourceExists", "NotADirectory", "TerminatedBySignal", "GenericError",
		}
	}
	fmt.Println()

	cfg.Enabled = true
	return cfg.Save()
}

func init() {
	configCmd.Flags().BoolP("interactive", "i", false, "Run interactive configuration wizard")
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
}
