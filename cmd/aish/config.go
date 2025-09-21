package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"github.com/TonnyWong1052/aish/internal/config"
	"github.com/TonnyWong1052/aish/internal/llm"
	"github.com/TonnyWong1052/aish/internal/llm/openai"
	"github.com/TonnyWong1052/aish/internal/prompt"
	"github.com/TonnyWong1052/aish/internal/ui"
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
	// If --interactive is not explicitly specified, enable interactive wizard by default in TTY
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

    // Only execute TUI wizard when interactive flag is true and TTY is available
    if interactive && isInteractiveTTY() {
        pterm.Info.Println("Running interactive TUI configuration...")
        // Delegate to centralized UI wizard to avoid duplicated logic
        wiz := ui.NewConfigWizard(cfg)
        if err := wiz.Run(); err != nil {
            pterm.Error.Printfln("Configuration failed: %v", err)
            os.Exit(1)
        }
        return
    } else {
        if err := plainConfigureWizard(cfg); err != nil {
            pterm.Error.Printfln("Configuration failed: %v", err)
            os.Exit(1)
        }
        pterm.Success.Println("Configuration saved.")
        return
    }

	// 1) Select provider (arrow keys up/down)
	type provItem struct{ label, key string }
	items := []provItem{{"OpenAI", "openai"}, {"Gemini", "gemini"}, {"Gemini CLI", "gemini-cli"}}
	// Default value (displayed as label)
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
	// Display interactive menu
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

	// Prepare existing provider settings or defaults
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
		// Gemini CLI's API endpoint is fixed and should not be customized
		pc.APIEndpoint = "https://cloudcode-pa.googleapis.com/v1internal"
		if pc.Model == "" {
			pc.Model = "gemini-2.5-flash"
		}
	}

	// 2) API Endpoint (Gemini CLI fixed, others can be input)
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

	// 3) Credentials/Project input
	switch selProvider {
	case "openai":
		// 自動判斷是否需要省略 /v1 前綴（若端點路徑已包含 /v* 則不再追加）
		pc.OmitV1Prefix = shouldOmitV1(pc.APIEndpoint)

		// API Key (masked)
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
		// Display official documentation link to help configure Workspace GCA
		pterm.Info.Printfln("Docs: Gemini CLI Workspace GCA: %s", "https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/authentication.md#workspace-gca")
		// Hide configured Project ID to avoid leaking sensitive information
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
		// Only support 2.5 series
		allowed := []string{"gemini-2.5-pro", "gemini-2.5-flash"}
		// Default option: if current is not in allowed list, fallback to gemini-2.5-flash
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
			// Fallback: keep original value or set to default
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

	// Write back provider settings
	cfg.Providers[selProvider] = pc

	// 5) Language
	langs := []string{"english", "zh-TW", "zh-CN", "japanese", "korean", "spanish", "french", "german", "italian", "portuguese", "russian", "arabic"}
	langNames := map[string]string{
		"english":    "English",
		"zh-TW":      "繁體中文 (Traditional Chinese)",
		"zh-CN":      "简体中文 (Simplified Chinese)",
		"japanese":   "日本語 (Japanese)",
		"korean":     "한국어 (Korean)",
		"spanish":    "Español (Spanish)",
		"french":     "Français (French)",
		"german":     "Deutsch (German)",
		"italian":    "Italiano (Italian)",
		"portuguese": "Português (Portuguese)",
		"russian":    "Русский (Russian)",
		"arabic":     "العربية (Arabic)",
	}
	curLang := cfg.UserPreferences.Language
	if curLang == "" {
		curLang = "english"
	}
	pterm.Println("Language:")
	for _, lang := range langs {
		pterm.Printf("• %s\n", langNames[lang])
	}
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

	// 6) Triggers (multi-select)
	// Note: Move long prompts out of interactive components to avoid line wrapping in narrow terminals
	//       causing repeated title output on each arrow redraw (observed "Enable auto-trigger..." repetition issue).
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
		// Recommended defaults: enable most errors
		defaultTriggers = []string{
			"CommandNotFound", "FileNotFoundOrDirectory", "PermissionDenied", "CannotExecute",
			"InvalidArgumentOrOption", "ResourceExists", "NotADirectory", "TerminatedBySignal", "GenericError",
		}
	}
    // Use custom multiselect to avoid pterm index bug when no selection yet
    selTriggers, err := ui.MultiSelectNoHelp(
        "Select error types to auto-trigger AI analysis (space to toggle, enter to confirm):",
        triggerOptions,
        defaultTriggers,
    )
	if err != nil {
		_ = plainConfigureWizard(cfg)
		pterm.Success.Println("Configuration saved.")
		return
	}
	cfg.UserPreferences.EnabledLLMTriggers = selTriggers

	// 7) Enable aish
	cfg.Enabled = true

	if err := cfg.Save(); err != nil {
		pterm.Error.Printfln("Failed to save config: %v", err)
		os.Exit(1)
	}

	pterm.Success.Println("Configuration saved.")
}

// maskIfSet masks non-empty keys for display
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

// isInteractiveTTY checks if in interactive TTY environment
func isInteractiveTTY() bool {
    return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// shouldOmitV1 returns true when the given endpoint already embeds a version-like
// path segment (e.g., /v, /v1, /v1beta, /v1internal), in which case we should not
// auto-append /v1 again.
func shouldOmitV1(endpoint string) bool {
    e := strings.TrimSpace(strings.ToLower(endpoint))
    // Normalize trailing slash for checks
    e = strings.TrimSuffix(e, "/")
    // Heuristic: if the path already contains a version segment starting with '/v'
    // we avoid appending '/v1'. This covers '/v', '/v1', '/v1beta', '/v1internal', etc.
    // It also works with many proxy endpoints like '.../v'.
    // Safe default when path has no '/v' segment: return false (we will manage '/v1').
    // Note: We intentionally keep this heuristic simple to avoid overfitting.
    // If later runtime detection finds otherwise, provider-level fallback will handle it.
    return strings.Contains(e, "/v")
}

// plainConfigureWizard provides a plain text configuration flow that doesn't depend on TUI
func plainConfigureWizard(cfg *config.Config) error {
	reader := bufio.NewReader(os.Stdin)

	// --- Provider Selection ---
	defProv := cfg.DefaultProvider
	if defProv == "" {
		defProv = "openai"
	}
    fmt.Println("Available providers:")
    fmt.Println("  - openai")
    fmt.Println("  - gemini")
    fmt.Println("  - gemini-cli")
    fmt.Printf("Q: Select default provider [current: %s]\n\nA: ", defProv)
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
		// Gemini CLI's API endpoint is fixed to full path, not allowing user modification
		pc.APIEndpoint = "https://cloudcode-pa.googleapis.com/v1internal:generateContent"
		if pc.Model == "" {
			pc.Model = "gemini-2.5-flash"
		}
	}

	// --- API Endpoint ---
	if prov != "gemini-cli" {
    fmt.Printf("Q: API endpoint [current: %s]\n\nA: ", pc.APIEndpoint)
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
		// 自動判斷是否需要省略 /v1 前綴（若端點路徑已包含 /v* 則不再追加）
		pc.OmitV1Prefix = shouldOmitV1(pc.APIEndpoint)
		fmt.Println()

        fmt.Println("Q: OpenAI API key (leave empty to skip)")
        fmt.Print("\nA: ")
		k, _ := reader.ReadString('\n')
		k = strings.TrimSpace(k)
		if k != "" {
			pc.APIKey = k
		}
	case "gemini":
        fmt.Println("Q: Gemini API key (leave empty to keep current)")
        fmt.Print("\nA: ")
		k, _ := reader.ReadString('\n')
		k = strings.TrimSpace(k)
		if k != "" {
			pc.APIKey = k
		}
	case "gemini-cli":
		// Display official documentation link to help configure Workspace GCA
		fmt.Printf("Docs: Gemini CLI Workspace GCA: %s\n", "https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/authentication.md#workspace-gca")
		// Hide configured Project ID to avoid leaking sensitive information
		disp := hideIfSet(pc.Project)
        fmt.Printf("Q: Google Cloud project ID [current: %s]\n\nA: ", disp)
		prj, _ := reader.ReadString('\n')
		prj = strings.TrimSpace(prj)
		if prj != "" {
			pc.Project = prj
		}
	}
	fmt.Println()

	// --- Model ---
	if prov == "gemini-cli" {
		// Only keep 2.5 series
		allowed := []string{"gemini-2.5-pro", "gemini-2.5-flash"}
		// Display options and allow input of sequence number or full name; empty input uses default
		// Default value: if current is not in allowed list, fallback to gemini-2.5-flash
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

        fmt.Println("Available models:")
		for i, a := range allowed {
			fmt.Printf("  %d) %s\n", i+1, a)
		}
        fmt.Printf("Q: Select model [current: %s]\n\nA: ", defModel)
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
			// Invalid input uses default
			pc.Model = defModel
		}
	} else {
        fmt.Printf("Q: Model name [current: %s]\n\nA: ", pc.Model)
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
    fmt.Print("Q: Test connection now? (y/N)\n\nA: ")
		yn, _ := reader.ReadString('\n')
		yn = strings.TrimSpace(strings.ToLower(yn))
		if yn == "y" || yn == "yes" {
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()
			// Here directly use OpenAI provider to make request to /models (POST preferred, 405 fallback to GET)
			tempProv, err := openai.NewProvider(pc, (*prompt.Manager)(nil))
			if err != nil {
				fmt.Printf("Connection test unavailable: %v\n\n", err)
			} else {
				if oai, ok := tempProv.(*openai.OpenAIProvider); ok {
					models, verr := oai.GetAvailableModels(ctx)
					if verr != nil {
						fmt.Printf("Connection test failed: %v\n\n", verr)
					} else {
						// Display a few results is sufficient
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
    fmt.Println("Q: Select language (only 'english' is supported for now)")
    fmt.Print("\nA: ")
    _ , _ = reader.ReadString('\n') // ignore user input, enforce english
    cfg.UserPreferences.Language = "english"
    fmt.Println()

    // --- Triggers ---
    fmt.Print("Q: Enable default error triggers? (Y/n)\n\nA: ")
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
