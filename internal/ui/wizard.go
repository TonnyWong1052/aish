package ui

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "strconv"
    "strings"
    "time"

    "github.com/TonnyWong1052/aish/internal/config"
    aerrors "github.com/TonnyWong1052/aish/internal/errors"
    "github.com/TonnyWong1052/aish/internal/llm/gemini/auth"
    "github.com/TonnyWong1052/aish/internal/llm/openai"
    "github.com/TonnyWong1052/aish/internal/prompt"

    "github.com/pterm/pterm"
)

// ConfigWizard configuration wizard
type ConfigWizard struct {
	config              *config.Config
	AdvancedGateEnabled bool
	QuickStartMode      bool
}

// NewConfigWizard creates a new configuration wizard
func NewConfigWizard(cfg *config.Config, advancedGate bool) *ConfigWizard {
	return &ConfigWizard{
		config:              cfg,
		AdvancedGateEnabled: advancedGate,
	}
}

// Run runs the configuration wizard
func (w *ConfigWizard) Run() error {
	// Show welcome message
	w.showWelcome()

	// Check if user wants to use quick start mode
	if w.shouldUseQuickStart() {
		w.QuickStartMode = true
		return w.runQuickStart()
	}

	// Continue with normal wizard flow
	pterm.Info.Println("Starting detailed configuration wizard...")
	pterm.Println()

	advancedPrompted := false
	skipAdvanced := false

	// Configuration steps
	steps := []ConfigStep{
		{Name: "Select LLM Provider", Handler: w.configureProvider},
		{Name: "Configure Language Preference", Handler: w.configureLanguage},
		{Name: "Configure Error Triggers", Handler: w.configureErrorTriggers},
		{Name: "Configure Context Settings", Handler: w.configureContext},
		{Name: "Configure Logging Settings", Handler: w.configureLogging},
		{Name: "Configure Cache Settings", Handler: w.configureCache},
		{Name: "Finish Configuration", Handler: w.finishConfiguration},
	}

	// Execute configuration steps
	for i, step := range steps {
		if skipAdvanced && (step.Name == "Configure Context Settings" ||
			step.Name == "Configure Logging Settings" ||
			step.Name == "Configure Cache Settings") {
			pterm.Warning.Printf("Skipping Step %d/%d: %s\n", i+1, len(steps), step.Name)
			continue
		}

		pterm.DefaultSection.Printf("Step %d/%d: %s", i+1, len(steps), step.Name)

		if err := step.Handler(); err != nil {
        if aerrors.HasCode(err, aerrors.ErrUserCancel) {
            pterm.Info.Println("Configuration cancelled")
            return err
        }
			return err
		}

		pterm.Success.Printf("✓ %s completed\n", step.Name)
		pterm.Println() // Add empty line separator

		// Ask about advanced settings after Step 3 (Configure Error Triggers) is completed
		if w.AdvancedGateEnabled && !advancedPrompted && i == 2 { // Check by index instead of name
			advancedPrompted = true
			pterm.Println() // Add extra spacing before the prompt
			pterm.DefaultSection.Println("Advanced Configuration Options")
			configureAdvanced, _ := pterm.DefaultInteractiveConfirm.
				WithDefaultValue(false).
				Show("Would you like to configure advanced settings (Context, Logging, Cache)?")
			if !configureAdvanced {
				skipAdvanced = true
				pterm.Info.Println("Skipping advanced settings (Steps 4-6). Using default values.")
			}
			pterm.Println() // Add spacing after the prompt
		}
	}

	return nil
}

// ConfigStep configuration step
type ConfigStep struct {
	Name    string
	Handler func() error
}

// showWelcome shows welcome message and asks about quick start
func (w *ConfigWizard) showWelcome() {
	pterm.DefaultHeader.WithFullWidth().Println("AISH Configuration Wizard")
	pterm.Info.Println("Welcome to AISH (AI Shell)!")
	pterm.Info.Println("This wizard will help you set up AISH's various features.")
	pterm.Info.Println("You can cancel the configuration at any time by pressing Ctrl+C.")
	pterm.Println()
}

// shouldUseQuickStart asks user if they want to use quick start mode
func (w *ConfigWizard) shouldUseQuickStart() bool {
	pterm.DefaultSection.Println("🚀 Quick Start Setup (Recommended)")
	pterm.Info.Println("Automatically configures:")
	pterm.Printf("  • Provider: Gemini CLI (Google Cloud)\n")
	pterm.Printf("  • Authentication: Web browser (OAuth)\n")
	pterm.Printf("  • Model: gemini-2.5-flash\n")
	pterm.Printf("  • All other settings: optimized defaults\n")
	pterm.Println()

	useQuickStart, _ := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(true).
		Show("Would you like to use Quick Start setup?")

	pterm.Println()
	return useQuickStart
}

// configureProvider configures LLM provider
func (w *ConfigWizard) configureProvider() error {
	// Show provider options
	providers := []string{"openai", "gemini", "gemini-cli", "claude", "ollama"}
	descriptions := map[string]string{
		"openai":     "OpenAI GPT series models (requires API key)",
		"gemini":     "Google Gemini public API (requires API key)",
		"gemini-cli": "Google Cloud Code private API (requires OAuth)",
		"claude":     "Anthropic Claude models via Genkit (requires API key)",
		"ollama":     "Local Ollama models via Genkit (no API key, runs locally)",
	}

	pterm.Info.Println("Available LLM providers:")
	for _, provider := range providers {
		pterm.Printf("• %s: %s\n", provider, descriptions[provider])
	}

	// Tip: suggest gemini-cli to users as an easier, keyless option with generous free usage
	pterm.Info.Println("Tip: 'gemini-cli' is recommended (OAuth login, no API key, easy setup, often higher free usage)")

	selectedProvider, _ := pterm.DefaultInteractiveSelect.
		WithOptions(providers).
		WithDefaultOption(w.config.DefaultProvider).
		Show("Select the provider you want to configure")

	// Get existing config or create new one
	providerConfig, exists := w.config.Providers[selectedProvider]
	if !exists {
		providerConfig = config.ProviderConfig{}
	}

	// Configure based on provider type
	switch selectedProvider {
	case "openai":
		if err := w.configureOpenAI(&providerConfig); err != nil {
			return err
		}
	case "gemini":
		if err := w.configureGemini(&providerConfig); err != nil {
			return err
		}
	case "gemini-cli":
		if err := w.configureGeminiCLI(&providerConfig); err != nil {
			return err
		}
	case "claude":
		if err := w.configureClaude(&providerConfig); err != nil {
			return err
		}
	case "ollama":
		if err := w.configureOllama(&providerConfig); err != nil {
			return err
		}
	}

	// Update configuration
	w.config.DefaultProvider = selectedProvider
	w.config.Providers[selectedProvider] = providerConfig

	return nil
}

// configureOpenAI configures OpenAI provider
func (w *ConfigWizard) configureOpenAI(cfg *config.ProviderConfig) error {
	pterm.DefaultHeader.Println("OpenAI Configuration")

	// API endpoint
	defaultEndpoint := "https://api.openai.com/v1"
	if cfg.APIEndpoint == "" {
		cfg.APIEndpoint = defaultEndpoint
	}

	useCustomEndpoint, _ := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(cfg.APIEndpoint != defaultEndpoint).
		Show("Do you want to use a custom API endpoint?")

	if useCustomEndpoint {
		endpoint, _ := pterm.DefaultInteractiveTextInput.
			WithDefaultValue(cfg.APIEndpoint).
			Show("Enter OpenAI API endpoint")
		cfg.APIEndpoint = endpoint
	} else {
		cfg.APIEndpoint = defaultEndpoint
	}

	// 自動判斷是否需要省略 /v1 前綴（若端點路徑已包含 /v* 則不再追加）
	cfg.OmitV1Prefix = shouldOmitV1(cfg.APIEndpoint)

	// API key
	pterm.Info.Println("You can get your API key at https://platform.openai.com/api-keys")
	apiKey, _ := pterm.DefaultInteractiveTextInput.
		WithMask("*").
		WithDefaultValue(cfg.APIKey).
		Show("Enter your OpenAI API key")
	cfg.APIKey = apiKey

	// Model selection
	return w.configureOpenAIModel(cfg)
}

// configureOpenAIModel configures OpenAI model
func (w *ConfigWizard) configureOpenAIModel(cfg *config.ProviderConfig) error {
	pterm.DefaultHeader.Println("Model Selection")

	// Provide selection options
	searchOptions := []string{
		"Fetch available models from API",
		"Use predefined common models",
		"Manually enter model name",
	}

	searchMethod, _ := pterm.DefaultInteractiveSelect.
		WithOptions(searchOptions).
		WithDefaultOption(searchOptions[0]).
		Show("Select model configuration method")

	var selectedModel string
	var err error

	switch searchMethod {
	case "Fetch available models from API":
		selectedModel, err = w.selectModelFromAPI(cfg)
	case "Use predefined common models":
		selectedModel, err = w.selectFromCommonModels(cfg)
	case "Manually input model name":
		selectedModel, err = w.inputCustomModel(cfg)
	}

	if err != nil {
		pterm.Warning.Printf("Model selection failed: %v\n", err)
		pterm.Info.Println("Falling back to manual input mode...")
		selectedModel, err = w.inputCustomModel(cfg)
		if err != nil {
			return err
		}
	}

	cfg.Model = selectedModel
	pterm.Success.Printf("Selected model: %s\n", selectedModel)
	return nil
}

// selectModelFromAPI fetches and selects model from API
func (w *ConfigWizard) selectModelFromAPI(cfg *config.ProviderConfig) (string, error) {
	if cfg.APIKey == "" || cfg.APIKey == "YOUR_OPENAI_API_KEY" {
		return "", fmt.Errorf("valid API key required to fetch model list")
	}

	pterm.Info.Println("Fetching available models from OpenAI API...")

	// 交由 OpenAI provider 統一處理端點與回退
	ctx := context.Background()
	prov, err := openai.NewProvider(*cfg, (*prompt.Manager)(nil))
	if err != nil {
		return "", fmt.Errorf("failed to init provider: %w", err)
	}
	oai, ok := prov.(*openai.OpenAIProvider)
	if !ok {
		return "", fmt.Errorf("provider type mismatch")
	}
	models, err := oai.GetAvailableModels(ctx)
	if err != nil {
		return "", err
	}
	if len(models) == 0 {
		return "", fmt.Errorf("no available models found")
	}

	// Group models by category
	gptModels := []string{}
	otherModels := []string{}

	for _, id := range models {
		if strings.Contains(id, "gpt-") {
			gptModels = append(gptModels, id)
		} else {
			otherModels = append(otherModels, id)
		}
	}

	pterm.Success.Printf("Found %d available models\n", len(models))

	// Build options list
	allOptions := []string{}

	if len(gptModels) > 0 {
		pterm.Info.Printf("GPT models (%d):\n", len(gptModels))
		for _, model := range gptModels {
			pterm.Printf("  • %s\n", model)
			allOptions = append(allOptions, model)
		}
	}

	if len(otherModels) > 0 {
		pterm.Info.Printf("Other models (%d):\n", len(otherModels))
		for _, model := range otherModels {
			pterm.Printf("  • %s\n", model)
			allOptions = append(allOptions, model)
		}
	}

	// Add manual input option
	allOptions = append(allOptions, "Enter model name manually")

	// Set default option
	defaultOption := cfg.Model
	if defaultOption == "" && len(gptModels) > 0 {
		// Prefer common GPT models
		for _, commonModel := range []string{"gpt-4o", "gpt-4", "gpt-4-turbo", "gpt-3.5-turbo"} {
			for _, availableModel := range gptModels {
				if availableModel == commonModel {
					defaultOption = commonModel
					break
				}
			}
			if defaultOption != "" {
				break
			}
		}
		// If not found, use the first GPT model
		if defaultOption == "" {
			defaultOption = gptModels[0]
		}
	}

	selectedModel, _ := pterm.DefaultInteractiveSelect.
		WithOptions(allOptions).
		WithDefaultOption(defaultOption).
		Show("Select a model")

	if selectedModel == "Enter model name manually" {
		return w.inputCustomModel(cfg)
	}

	return selectedModel, nil
}

// selectFromCommonModels selects from predefined common models
func (w *ConfigWizard) selectFromCommonModels(cfg *config.ProviderConfig) (string, error) {
	commonModels := []string{
		"gpt-4o", "gpt-4o-mini", "gpt-4", "gpt-4-turbo",
		"gpt-3.5-turbo", "gpt-3.5-turbo-16k",
		"text-davinci-003", "text-curie-001",
	}

	if cfg.Model == "" {
		cfg.Model = "gpt-4o"
	}

	allOptions := append(commonModels, "Enter model name manually")

	selectedModel, _ := pterm.DefaultInteractiveSelect.
		WithOptions(allOptions).
		WithDefaultOption(cfg.Model).
		Show("Select a model")

	if selectedModel == "Enter model name manually" {
		return w.inputCustomModel(cfg)
	}

	return selectedModel, nil
}

// inputCustomModel manually input model name
func (w *ConfigWizard) inputCustomModel(cfg *config.ProviderConfig) (string, error) {
	pterm.Info.Println("You can enter any OpenAI-supported model name.")
	pterm.Info.Println("Examples: gpt-4o, gpt-4, gpt-3.5-turbo, text-davinci-003, etc.")

	customModel, _ := pterm.DefaultInteractiveTextInput.
		WithDefaultValue(cfg.Model).
		Show("Enter model name")

	if strings.TrimSpace(customModel) == "" {
		return "", fmt.Errorf("model name cannot be empty")
	}

	return strings.TrimSpace(customModel), nil
}

// shouldOmitV1 根據端點路徑是否已含 /v* 判定是否省略 /v1 自動附加
func shouldOmitV1(endpoint string) bool {
	e := strings.TrimSpace(strings.ToLower(endpoint))
	e = strings.TrimSuffix(e, "/")
	return strings.Contains(e, "/v")
}

// configureGemini configures Gemini provider
func (w *ConfigWizard) configureGemini(cfg *config.ProviderConfig) error {
	pterm.DefaultHeader.Println("Gemini Configuration")

	// API endpoint
	defaultEndpoint := "https://generativelanguage.googleapis.com/v1"
	if cfg.APIEndpoint == "" {
		cfg.APIEndpoint = defaultEndpoint
	}

	useCustomEndpoint, _ := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(cfg.APIEndpoint != defaultEndpoint).
		Show("Do you want to use a custom API endpoint?")

	if useCustomEndpoint {
		endpoint, _ := pterm.DefaultInteractiveTextInput.
			WithDefaultValue(cfg.APIEndpoint).
			Show("Enter Gemini API endpoint")
		cfg.APIEndpoint = endpoint
	} else {
		cfg.APIEndpoint = defaultEndpoint
	}

	// API key
	pterm.Info.Println("You can get an API key from https://makersuite.google.com/app/apikey")
	apiKey, _ := pterm.DefaultInteractiveTextInput.
		WithMask("*").
		WithDefaultValue(cfg.APIKey).
		Show("Enter your Gemini API key")
	cfg.APIKey = apiKey

	// Model selection
	commonModels := []string{
		"gemini-pro", "gemini-pro-vision",
	}

	if cfg.Model == "" {
		cfg.Model = "gemini-pro"
	}

	model, _ := pterm.DefaultInteractiveSelect.
		WithOptions(append(commonModels, "Enter model name manually")).
		WithDefaultOption(cfg.Model).
		Show("Select a model")

	if model == "Enter model name manually" {
		customModel, _ := pterm.DefaultInteractiveTextInput.
			WithDefaultValue(cfg.Model).
			Show("Enter model name")
		cfg.Model = customModel
	} else {
		cfg.Model = model
	}

	return nil
}

// configureGeminiCLI configures Gemini CLI provider
func (w *ConfigWizard) configureGeminiCLI(cfg *config.ProviderConfig) error {
	pterm.DefaultHeader.Println("Gemini CLI Configuration")

	// API endpoint
	defaultEndpoint := "https://cloudcode-pa.googleapis.com/v1internal"
	if cfg.APIEndpoint == "" {
		cfg.APIEndpoint = defaultEndpoint
	}
	// Gemini CLI uses a fixed endpoint; do not prompt for customization
	cfg.APIEndpoint = defaultEndpoint

	// Authentication method selection first
	authOptions := []string{
		"Authenticate now via web browser",
		"Use existing credentials from `gemini-cli`",
	}
	authMethod, _ := pterm.DefaultInteractiveSelect.
		WithOptions(authOptions).
		WithDefaultOption("Authenticate now via web browser").
		Show("How would you like to authenticate with Gemini CLI?")

	// Only ask for Project ID if using existing credentials
	if authMethod == "Use existing credentials from `gemini-cli`" {
		pterm.Info.Println("You need a Google Cloud Project ID for existing credentials.")
		pterm.Println("Enter your Google Cloud Project ID:")
		projectID, _ := pterm.DefaultInteractiveTextInput.
			Show(">")
		cfg.Project = projectID
	}

	// Model selection (only 2.5 series supported)
	commonModels := []string{
		"gemini-2.5-pro",
		"gemini-2.5-flash",
	}

	if cfg.Model == "" {
		cfg.Model = "gemini-2.5-flash"
	}

	model, _ := pterm.DefaultInteractiveSelect.
		WithOptions(append(commonModels, "Enter model name manually")).
		WithDefaultOption(cfg.Model).
		Show("Select a model")

	if model == "Enter model name manually" {
		customModel, _ := pterm.DefaultInteractiveTextInput.
			WithDefaultValue(cfg.Model).
			Show("Enter model name")
		cfg.Model = customModel
	} else {
		cfg.Model = model
	}

	// Handle authentication based on selection
	if authMethod == "Authenticate now via web browser" {
		pterm.Info.Println("Starting web-based authentication flow...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := auth.StartWebAuthFlow(ctx); err != nil {
			pterm.Error.Printf("Web authentication failed: %v\n", err)
			pterm.Warning.Println("You can try again or choose to use existing credentials.")
			return err
		}
		pterm.Success.Println("Authentication credentials saved successfully.")
		// Give a moment for the success message to be displayed
		time.Sleep(500 * time.Millisecond)

		// Show the Google account email bound to this OAuth session
		if email, err := auth.GetAuthenticatedEmail(context.Background()); err == nil {
			if s := strings.TrimSpace(email); s != "" {
				pterm.Success.Printf("Authenticated Google account: %s\n", s)
			}
		}


		// 先讀取 AISH 憑證檔的 project_id（若 OAuth callback 已提供該欄位）
		if pid := readProjectIDFromAishCreds(); (strings.TrimSpace(cfg.Project) == "" || cfg.Project == "YOUR_GEMINI_PROJECT_ID") && pid != "" {
			cfg.Project = pid
			pterm.Success.Printf("Detected project from credentials: %s\n", displayLabelForProject(cfg.Project))
		}

		// 一律先嘗試以 OAuth 列出可用專案，讓使用者確認或更換（避免沿用舊設定或 gcloud 預設）
		if list, err := auth.SearchProjectsV3(context.Background()); err == nil && len(list) > 0 {
                // 若目前設定已有專案，且可被當前 OAuth 帳號訪問，直接沿用；不可訪問則自動清空並改用 OAuth 清單
                if s := strings.TrimSpace(cfg.Project); s != "" && s != "YOUR_GEMINI_PROJECT_ID" {
                    if _, err := auth.GetProject(context.Background(), s); err == nil {
                        pterm.Success.Printf("Using configured project: %s\n", displayLabelForProject(cfg.Project))
                    } else {
                        pterm.Warning.Printf("Configured project not accessible with current OAuth account: %s\n", displayLabelForProject(s))
                        cfg.Project = ""
                    }
                }

			if strings.TrimSpace(cfg.Project) == "" || cfg.Project == "YOUR_GEMINI_PROJECT_ID" {
				// 預設候選（若有）
				def := auth.PickDefaultProject(list)
				if def == "" && len(list) > 0 {
					def = strings.TrimSpace(list[0].ProjectID)
				}

                    // 自動選擇預設專案（無需手動確認）
                    if strings.TrimSpace(def) != "" {
                        cfg.Project = def
                        pterm.Success.Printf("Selected project (auto): %s\n", displayLabelForProject(cfg.Project))
                    }
			}
            }

        // If a configured project exists but may not be accessible with current OAuth account, auto-switch (no prompt)
        if strings.TrimSpace(cfg.Project) != "" && cfg.Project != "YOUR_GEMINI_PROJECT_ID" {
            if _, err := auth.GetProject(context.Background(), strings.TrimSpace(cfg.Project)); err != nil {
                pterm.Warning.Printf("Configured project is not accessible with current OAuth account: %s\n", displayLabelForProject(cfg.Project))
                pterm.Info.Println("Auto-switching to an OAuth-accessible project...")
                cfg.Project = ""
            }
        }

        // If still not set, try metadata/gcloud auto-detection (no external APIs)
	        if strings.TrimSpace(cfg.Project) == "" || cfg.Project == "YOUR_GEMINI_PROJECT_ID" {
	            pterm.Info.Println("Attempting to auto-detect Google Cloud Project from metadata/gcloud...")
            // Also show the authenticated Google account for clarity and the gcloud account
            var oauthEmail, gcloudEmail string
            if email, err := auth.GetAuthenticatedEmail(context.Background()); err == nil {
                if s := strings.TrimSpace(email); s != "" {
                    oauthEmail = s
                    pterm.Info.Printf("Authenticated Google account (OAuth): %s\n", s)
                }
            }
            if s := strings.TrimSpace(getGcloudAccount()); s != "" {
                gcloudEmail = s
                pterm.Info.Printf("gcloud default account: %s\n", s)
            }

            useGcloud := true
            if oauthEmail != "" && gcloudEmail != "" && !strings.EqualFold(oauthEmail, gcloudEmail) {
                // Auto-decide: avoid gcloud fallback when accounts mismatch
                pterm.Warning.Println("OAuth account differs from gcloud account; skipping gcloud-based project detection.")
                useGcloud = false
            }

            pid := ""
            if useGcloud {
                if p, _ := auth.AutoDetectProjectID(context.Background()); strings.TrimSpace(p) != "" {
                    pid = strings.TrimSpace(p)
                    // Verify the detected project is accessible with current OAuth credentials
                    if _, err := auth.GetProject(context.Background(), pid); err != nil {
                        pterm.Warning.Printf("gcloud default project '%s' is not accessible with current OAuth account, skipping.\n", pid)
                        pid = ""
                    } else {
                        cfg.Project = pid
                    }
                }
            }
            if pid != "" {
                pterm.Success.Printf("Auto-detected default project: %s\n", displayLabelForProject(cfg.Project))
            } else {
                // gcloud not found? offer to install and set up
                if !hasCommand("gcloud") {
                    pterm.Warning.Println("gcloud CLI not found. It is required for local project auto-detection.")
                    install, _ := pterm.DefaultInteractiveConfirm.
                        WithDefaultValue(true).
                        Show("Install Google Cloud CLI (gcloud) now?")
                    if install {
                        if err := ensureGcloudInstalled(); err != nil {
                            pterm.Error.Printfln("Failed to install gcloud automatically: %v", err)
                            pterm.Info.Println("Please install gcloud manually and re-run 'aish config'. Docs: https://cloud.google.com/sdk/docs/install")
                        }
                    } else {
                        // 使用者拒絕安裝 gcloud：以錯誤終止初始化流程
                        pterm.Error.Println("gcloud is required to complete Gemini CLI project setup.")
                        switch runtime.GOOS {
                        case "darwin":
                            pterm.Info.Println("Install via Homebrew: brew install --cask google-cloud-sdk")
                        case "linux":
                            pterm.Info.Println("Install via your package manager (e.g., apt-get install google-cloud-cli) or see docs below")
                        default:
                            pterm.Info.Println("Install gcloud using your platform's package manager")
                        }
                        pterm.Info.Println("Docs: https://cloud.google.com/sdk/docs/install")
                        pterm.Info.Println("Alternatively, set project manually later: aish config set providers.gemini-cli.project <PROJECT_ID>")
                        return aerrors.NewError(aerrors.ErrConfigValidation, "gcloud not installed; user declined installation")
                    }
                }

                if hasCommand("gcloud") {
                    pterm.Info.Println("Launching 'gcloud auth login' (a browser will open)...")
                    _ = runCommandInteractive("gcloud", "auth", "login")

	                    if pid := getGcloudProjectID(); strings.TrimSpace(pid) != "" && pid != "(unset)" {
	                        cfg.Project = pid
	                        pterm.Success.Printf("Detected default project from gcloud: %s\n", displayLabelForProject(cfg.Project))
                    } else {
                        // 列出可選專案供使用者選擇
                        if list, err := listGcloudProjects(); err == nil && len(list) > 0 {
                            if len(list) == 1 {
                                s := strings.TrimSpace(list[0].ProjectID)
                                if s != "" {
                                    _ = runCommandInteractive("gcloud", "config", "set", "project", s)
	                                    if pid := getGcloudProjectID(); strings.TrimSpace(pid) != "" && pid != "(unset)" {
	                                        cfg.Project = pid
	                                        pterm.Success.Printf("Project set via gcloud: %s\n", displayLabelForProject(cfg.Project))
                                    }
                                }
                            } else {
                                options := make([]string, 0, len(list)+1)
                                labelToID := map[string]string{}
                                for _, p := range list {
                                    label := fmt.Sprintf("%s (%s)", firstNonEmpty(p.Name, p.ProjectID), p.ProjectID)
                                    options = append(options, label)
                                    labelToID[label] = p.ProjectID
                                }
                                options = append(options, "Skip")
                                choice, _ := pterm.DefaultInteractiveSelect.
                                    WithOptions(options).
                                    WithDefaultOption(options[0]).
                                    Show("Select a Google Cloud Project to set as default")
                                if id, ok := labelToID[choice]; ok && strings.TrimSpace(id) != "" {
	                                    if err := runCommandInteractive("gcloud", "config", "set", "project", id); err == nil {
	                                        if pid := getGcloudProjectID(); strings.TrimSpace(pid) != "" && pid != "(unset)" {
	                                            cfg.Project = pid
	                                            pterm.Success.Printf("Project set via gcloud: %s\n", displayLabelForProject(cfg.Project))
                                        }
                                    }
                                } else if choice != "Skip" {
                                    pterm.Warning.Println("Invalid selection; skipping.")
                                }
                            }
                        } else {
                            // 回退：手動輸入
                            manualID, _ := pterm.DefaultInteractiveTextInput.
                                Show("Enter your Google Cloud Project ID to set as default (or leave empty to skip)")
                            if s := strings.TrimSpace(manualID); s != "" {
	                                if err := runCommandInteractive("gcloud", "config", "set", "project", s); err == nil {
	                                    if pid := getGcloudProjectID(); strings.TrimSpace(pid) != "" && pid != "(unset)" {
	                                        cfg.Project = pid
	                                        pterm.Success.Printf("Project set via gcloud: %s\n", displayLabelForProject(cfg.Project))
                                    }
                                } else {
                                    pterm.Warning.Printfln("Failed to set gcloud default project: %v", err)
                                }
                            }
                        }
                    }

                    if strings.TrimSpace(cfg.Project) == "" || cfg.Project == "YOUR_GEMINI_PROJECT_ID" {
                        pterm.Warning.Println("Project still not set. You can set it later with 'aish config set providers.gemini-cli.project <PROJECT_ID>'.")
                    }
                } else {
                    pterm.Warning.Println("gcloud is not available; skipping auto-detection.")
                }
            }
        }

        // 在本步驟結束前，統一再回顯一次選定的 GCP 專案名稱
        if s := strings.TrimSpace(cfg.Project); s != "" && s != "YOUR_GEMINI_PROJECT_ID" {
            pterm.Success.Printf("Using Google Cloud Project: %s\n", displayLabelForProject(s))

            // Try to enable Gemini API for the selected project
            ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
            defer cancel2()
            if err := enableGeminiAPIsForProject(ctx2, s); err != nil {
                // Just warn, don't fail the wizard
                pterm.Warning.Printf("Could not auto-enable Gemini APIs: %v\n", err)
                pterm.Info.Println("Please enable them manually:")
                pterm.Info.Printf("  gcloud services enable cloudaicompanion.googleapis.com --project=%s\n", s)
            }
        }
    } else {
        pterm.Info.Println("AISH will attempt to use existing credentials from `~/.gemini/`.")
        pterm.Info.Println("Please ensure `gemini-cli` is installed and authenticated.")
    }

	return nil
}

// configureLanguage configures language preference
func (w *ConfigWizard) configureLanguage() error {
	pterm.DefaultHeader.Println("Language Settings")

	// Available languages
	languages := []string{
		"English",
		"繁體中文 (Traditional Chinese)",
		"简体中文 (Simplified Chinese)",
		"日本語 (Japanese)",
		"한국어 (Korean)",
		"Español (Spanish)",
		"Français (French)",
		"Deutsch (German)",
	}

	// Map display names to internal values
	languageValues := map[string]string{
		"English":                            "english",
		"繁體中文 (Traditional Chinese)":        "zh-TW",
		"简体中文 (Simplified Chinese)":        "zh-CN",
		"日本語 (Japanese)":                   "ja",
		"한국어 (Korean)":                      "ko",
		"Español (Spanish)":                  "es",
		"Français (French)":                  "fr",
		"Deutsch (German)":                   "de",
	}

	// Find current language display name
	currentDisplay := "English"
	for display, value := range languageValues {
		if value == w.config.UserPreferences.Language {
			currentDisplay = display
			break
		}
	}

	pterm.Info.Println("Select the language for AI responses:")
	pterm.Info.Println("Note: This affects the language used by the AI when generating explanations and suggestions.")

	selectedLanguage, _ := pterm.DefaultInteractiveSelect.
		WithOptions(languages).
		WithDefaultOption(currentDisplay).
		Show("Select your preferred response language")

	// Set the language value
	if value, ok := languageValues[selectedLanguage]; ok {
		w.config.UserPreferences.Language = value
	} else {
		w.config.UserPreferences.Language = "english" // fallback
	}

	return nil
}

// firstNonEmpty 回傳第一個非空字串
func firstNonEmpty(a, b string) string {
    if strings.TrimSpace(a) != "" {
        return a
    }
    return b
}

// configureErrorTriggers configures error triggers
func (w *ConfigWizard) configureErrorTriggers() error {
	pterm.DefaultHeader.Println("Error Analysis Triggers")
	pterm.Info.Println("Select which error types should trigger AI analysis:")

	errorTypes := []string{
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

	errorDescriptions := map[string]string{
		"CommandNotFound":         "Command not found",
		"FileNotFoundOrDirectory": "File or directory does not exist",
		"PermissionDenied":        "Permission denied",
		"CannotExecute":           "Cannot execute",
		"InvalidArgumentOrOption": "Invalid argument or option",
		"ResourceExists":          "Resource already exists",
		"NotADirectory":           "Not a directory",
		"TerminatedBySignal":      "Terminated by signal",
		"GenericError":            "Generic error",
	}

	// Show options
	pterm.Info.Println("Error type descriptions:")
	for _, errorType := range errorTypes {
		pterm.Printf("• %s: %s\n", errorType, errorDescriptions[errorType])
	}

	selectedTypes, _ := MultiSelectNoHelp(
		"Select error types to enable AI analysis (space to toggle, enter to confirm):",
		errorTypes,
		w.config.UserPreferences.EnabledLLMTriggers,
	)

	w.config.UserPreferences.EnabledLLMTriggers = selectedTypes
	return nil
}

// configureContext configures context settings
func (w *ConfigWizard) configureContext() error {
	pterm.DefaultHeader.Println("Context Settings")

	// Max history entries
	maxHistoryStr, _ := pterm.DefaultInteractiveTextInput.
		WithDefaultValue(fmt.Sprintf("%d", w.config.UserPreferences.Context.MaxHistoryEntries)).
		Show("Maximum command history entries (recommended: 10)")

	if maxHistory, err := strconv.Atoi(maxHistoryStr); err == nil {
		w.config.UserPreferences.Context.MaxHistoryEntries = maxHistory
	}

	// Include directory listing
	includeDir, _ := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(w.config.UserPreferences.Context.IncludeDirectories).
		Show("Include current directory file listing in context?")
	w.config.UserPreferences.Context.IncludeDirectories = includeDir

	// Filter sensitive commands
	filterSensitive, _ := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(w.config.UserPreferences.Context.FilterSensitiveCmd).
		Show("Filter commands containing sensitive info (passwords, keys, etc.)?")
	w.config.UserPreferences.Context.FilterSensitiveCmd = filterSensitive

	// Enable enhanced analysis
	enableEnhanced, _ := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(w.config.UserPreferences.Context.EnableEnhanced).
		Show("Enable enhanced context analysis?")
	w.config.UserPreferences.Context.EnableEnhanced = enableEnhanced

	return nil
}

// configureLogging configures logging settings
func (w *ConfigWizard) configureLogging() error {
	pterm.DefaultHeader.Println("Logging Settings")

	// Log level
	levels := []string{"trace", "debug", "info", "warn", "error"}
	level, _ := pterm.DefaultInteractiveSelect.
		WithOptions(levels).
		WithDefaultOption(w.config.UserPreferences.Logging.Level).
		Show("Select log level")
	w.config.UserPreferences.Logging.Level = level

	// Log format
	formats := []string{"text", "json"}
	format, _ := pterm.DefaultInteractiveSelect.
		WithOptions(formats).
		WithDefaultOption(w.config.UserPreferences.Logging.Format).
		Show("Select log format")
	w.config.UserPreferences.Logging.Format = format

	// Log output
	outputs := []string{"file", "console", "both"}
	output, _ := pterm.DefaultInteractiveSelect.
		WithOptions(outputs).
		WithDefaultOption(w.config.UserPreferences.Logging.Output).
		Show("Select log output")
	w.config.UserPreferences.Logging.Output = output

	return nil
}

// configureCache configures cache settings
func (w *ConfigWizard) configureCache() error {
	pterm.DefaultHeader.Println("Cache Settings")

	// Enable cache
	enabled, _ := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(w.config.UserPreferences.Cache.Enabled).
		Show("Enable response caching (improves speed and saves API costs)?")
	w.config.UserPreferences.Cache.Enabled = enabled

	if !enabled {
		pterm.Info.Println("Cache disabled. Skipping other cache settings.")
		return nil
	}

	// Similarity matching
	enableSimilarity, _ := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(w.config.UserPreferences.Cache.EnableSimilarity).
		Show("Enable intelligent similarity matching (reuse cache for similar queries)?")
	w.config.UserPreferences.Cache.EnableSimilarity = enableSimilarity

	if enableSimilarity {
		// Similarity threshold
		thresholdStr, _ := pterm.DefaultInteractiveTextInput.
			WithDefaultValue(fmt.Sprintf("%.2f", w.config.UserPreferences.Cache.SimilarityThreshold)).
			Show("Similarity threshold (0.0-1.0, recommended: 0.85)")

		if threshold, err := strconv.ParseFloat(thresholdStr, 64); err == nil {
			w.config.UserPreferences.Cache.SimilarityThreshold = threshold
		}
	}

	// Cache size
	maxEntriesStr, _ := pterm.DefaultInteractiveTextInput.
		WithDefaultValue(fmt.Sprintf("%d", w.config.UserPreferences.Cache.MaxEntries)).
		Show("Maximum cache entries (recommended: 1000)")

	if maxEntries, err := strconv.Atoi(maxEntriesStr); err == nil {
		w.config.UserPreferences.Cache.MaxEntries = maxEntries
	}

	return nil
}

// configureClaude configures Anthropic Claude provider via Genkit
func (w *ConfigWizard) configureClaude(cfg *config.ProviderConfig) error {
	pterm.DefaultHeader.Println("Claude (Anthropic) Configuration")
	pterm.Info.Println("Claude uses Genkit Go framework for unified LLM interaction")

	// API endpoint
	defaultEndpoint := config.ClaudeAPIEndpoint
	if cfg.APIEndpoint == "" {
		cfg.APIEndpoint = defaultEndpoint
	}

	useCustomEndpoint, _ := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(cfg.APIEndpoint != defaultEndpoint).
		Show("Do you want to use a custom API endpoint?")

	if useCustomEndpoint {
		endpoint, _ := pterm.DefaultInteractiveTextInput.
			WithDefaultValue(cfg.APIEndpoint).
			Show("Enter Claude API endpoint")
		cfg.APIEndpoint = endpoint
	} else {
		cfg.APIEndpoint = defaultEndpoint
	}

	// API key
	pterm.Info.Println("You can get your API key from https://console.anthropic.com/settings/keys")
	apiKey, _ := pterm.DefaultInteractiveTextInput.
		WithMask("*").
		WithDefaultValue(cfg.APIKey).
		Show("Enter your Anthropic API key")
	cfg.APIKey = apiKey

	// Model input (manual entry)
	if cfg.Model == "" {
		cfg.Model = config.DefaultClaudeModel
	}

	pterm.Info.Println("Common models: claude-3-5-sonnet-20241022, claude-3-5-haiku-20241022, claude-3-opus-20240229")
	pterm.Info.Println("Tip: For OpenAI-compatible endpoints, use prefix like 'openai/gpt-4' or 'openai/glm-4'")
	model, _ := pterm.DefaultInteractiveTextInput.
		WithDefaultValue(cfg.Model).
		Show("Enter model name (with optional prefix)")
	cfg.Model = strings.TrimSpace(model)

	pterm.Success.Printf("Claude configured: %s via Genkit\n", cfg.Model)
	return nil
}

// configureOllama configures local Ollama provider via Genkit
func (w *ConfigWizard) configureOllama(cfg *config.ProviderConfig) error {
	pterm.DefaultHeader.Println("Ollama (Local LLM) Configuration")
	pterm.Info.Println("Ollama uses Genkit Go framework for unified LLM interaction")
	pterm.Info.Println("Note: Ollama must be installed and running locally")

	// API endpoint (local)
	defaultEndpoint := config.OllamaAPIEndpoint
	if cfg.APIEndpoint == "" {
		cfg.APIEndpoint = defaultEndpoint
	}

	useCustomEndpoint, _ := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(cfg.APIEndpoint != defaultEndpoint).
		Show("Do you want to use a custom Ollama endpoint?")

	if useCustomEndpoint {
		endpoint, _ := pterm.DefaultInteractiveTextInput.
			WithDefaultValue(cfg.APIEndpoint).
			Show("Enter Ollama API endpoint")
		cfg.APIEndpoint = endpoint
	} else {
		cfg.APIEndpoint = defaultEndpoint
	}

	// No API key needed for Ollama
	cfg.APIKey = ""
	pterm.Info.Println("✓ No API key required for local Ollama")

	// Model input (manual entry)
	if cfg.Model == "" {
		cfg.Model = config.DefaultOllamaModel
	}

	pterm.Info.Println("Common local models: llama3.3, llama3.1, codellama, mistral, gemma, qwen")
	pterm.Info.Println("Tip: Make sure you have pulled the model with: ollama pull <model-name>")
	model, _ := pterm.DefaultInteractiveTextInput.
		WithDefaultValue(cfg.Model).
		Show("Enter model name (prefix 'ollama/' will be added if not present)")
	cfg.Model = strings.TrimSpace(model)

	pterm.Success.Printf("Ollama configured: %s via Genkit (local)\n", cfg.Model)
	pterm.Warning.Println("Remember to start Ollama with: ollama serve")
	return nil
}

// finishConfiguration completes configuration
func (w *ConfigWizard) finishConfiguration() error {
	pterm.DefaultHeader.Println("Configuration Complete")

	// Validate configuration
	pterm.Info.Println("Validating configuration...")
	fixes, err := w.config.ValidateAndFix()
	if err != nil {
		pterm.Error.Println("Configuration validation failed:", err)

		retry, _ := pterm.DefaultInteractiveConfirm.
			WithDefaultValue(true).
			Show("Run the configuration wizard again?")

		if retry {
			return w.Run()
		}
		return err
	}

	if len(fixes) > 0 {
		pterm.Info.Println("Automatically fixed the following issues:")
		for _, fix := range fixes {
			pterm.Printf("• %s\n", fix)
		}
	}

	// Save configuration
	pterm.Info.Println("Saving configuration...")
    if err := w.config.Save(); err != nil {
        return aerrors.ErrConfigSaveFailed("", err)
    }

	// Show configuration summary
	w.showConfigurationSummary()

	pterm.Success.Println("🎉 AISH configuration completed!")
	pterm.Info.Println("You can now use these commands:")
	pterm.Printf("• %s: Install the shell hook\n", pterm.LightBlue("aish setup"))
	pterm.Printf("• %s: Test AI command generation\n", pterm.LightBlue("aish -p \"your prompt\""))
	pterm.Printf("• %s: View configuration\n", pterm.LightBlue("aish config show"))

	return nil
}

// showConfigurationSummary shows configuration summary
func (w *ConfigWizard) showConfigurationSummary() {
	pterm.DefaultSection.Println("Configuration Summary")

	// Provider info
	pterm.Printf("• LLM Provider: %s\n", w.config.DefaultProvider)
	if providerCfg, exists := w.config.Providers[w.config.DefaultProvider]; exists {
		pterm.Printf("• Model: %s\n", providerCfg.Model)
		pterm.Printf("• API Endpoint: %s\n", providerCfg.APIEndpoint)
	}

	// User preferences
	pterm.Printf("• Response Language: %s\n", w.config.UserPreferences.Language)
	pterm.Printf("• Enabled Error Triggers: %d\n", len(w.config.UserPreferences.EnabledLLMTriggers))

	// Feature flags
	pterm.Printf("• Cache: %s\n", boolToStatus(w.config.UserPreferences.Cache.Enabled))
	pterm.Printf("• Similarity Matching: %s\n", boolToStatus(w.config.UserPreferences.Cache.EnableSimilarity))
	pterm.Printf("• Enhanced Context: %s\n", boolToStatus(w.config.UserPreferences.Context.EnableEnhanced))

	pterm.Println()
}

// boolToStatus converts boolean to status label
func boolToStatus(enabled bool) string {
    if enabled {
        return pterm.LightGreen("Enabled")
    }
    return pterm.LightRed("Disabled")
}

// runQuickStart performs automatic configuration with optimal defaults
func (w *ConfigWizard) runQuickStart() error {
	pterm.DefaultSection.Println("Quick Start Configuration")
	pterm.Info.Println("Setting up AISH with optimal defaults...")
	pterm.Println()

	// Step 1: Configure Gemini CLI provider with defaults
	pterm.Info.Println("✓ Configuring Gemini CLI provider...")
	w.config.DefaultProvider = "gemini-cli"

	// Initialize providers map if needed
	if w.config.Providers == nil {
		w.config.Providers = make(map[string]config.ProviderConfig)
	}

	// Set up Gemini CLI configuration
	w.config.Providers["gemini-cli"] = config.ProviderConfig{
		APIEndpoint: "https://cloudcode-pa.googleapis.com/v1internal:generateContent",
		Model:       "gemini-2.5-flash",
		Project:     "YOUR_GEMINI_PROJECT_ID", // Will be updated during OAuth
	}

	// Step 2: Set optimal defaults for user preferences
	pterm.Info.Println("✓ Configuring user preferences...")
	w.config.UserPreferences.Language = "english"
	w.config.UserPreferences.AutoExecute = false // Safe default

	// Enable all common error triggers for comprehensive coverage
	w.config.UserPreferences.EnabledLLMTriggers = []string{
		"CommandNotFound",
		"FileNotFoundOrDirectory",
		"PermissionDenied",
		"CannotExecute",
		"InvalidArgumentOrOption",
		"ResourceExists",
		"NotADirectory",
		"TerminatedBySignal",
		"GenericError",
		"NetworkError",
		"DatabaseError",
		"ConfigError",
		"DependencyError",
		"TimeoutError",
		"MemoryError",
		"DiskSpaceError",
		"PermissionError",
		"AuthenticationError",
		"InteractiveToolUsage",
	}

	// Set other optimal defaults
	w.config.UserPreferences.Context.MaxHistoryEntries = 10
	w.config.UserPreferences.Context.IncludeDirectories = true
	w.config.UserPreferences.Context.FilterSensitiveCmd = true
	w.config.UserPreferences.Context.EnableEnhanced = true

	// Step 3: Start OAuth authentication flow
	pterm.Info.Println("✓ Starting web authentication...")
	pterm.Info.Println("Your browser will open for Google account authentication...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := auth.StartWebAuthFlow(ctx); err != nil {
		pterm.Error.Printf("Web authentication failed: %v\n", err)
		pterm.Warning.Println("You can try again or set up manually with 'aish config'")
		return err
	}

	pterm.Success.Println("✓ Authentication completed successfully!")

	// Step 4: Update project configuration from OAuth credentials
	if pid := readProjectIDFromAishCreds(); pid != "" {
		cfg := w.config.Providers["gemini-cli"]
		cfg.Project = pid
		w.config.Providers["gemini-cli"] = cfg
		pterm.Success.Printf("✓ Using Google Cloud Project: %s\n", displayLabelForProject(pid))

		// Try to enable Gemini API for the detected project
		ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel2()
		if err := enableGeminiAPIsForProject(ctx2, pid); err != nil {
			pterm.Warning.Printf("Could not auto-enable Gemini APIs: %v\n", err)
			pterm.Info.Println("Please enable them manually:")
			pterm.Info.Printf("  gcloud services enable cloudaicompanion.googleapis.com --project=%s\n", pid)
		}
	}

	// Step 5: Save configuration
	pterm.Info.Println("✓ Saving configuration...")
	if err := w.config.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Step 6: Show completion message
	pterm.Println()
	pterm.DefaultHeader.WithFullWidth().Println("🎉 Quick Start Complete!")
	pterm.Success.Println("AISH is now ready to use!")
	pterm.Println()
	pterm.Info.Println("Try these commands:")
	pterm.Printf("  • aish -p \"list files sorted by size\"\n")
	pterm.Printf("  • aish -p \"create a backup folder\"\n")
	pterm.Printf("  • aish -a \"who are you?\"\n")
	pterm.Println()
	pterm.Info.Println("For advanced configuration, run: aish config")

	return nil
}

// enableGeminiAPIsForProject enables the required Google Cloud APIs for a project
func enableGeminiAPIsForProject(ctx context.Context, projectID string) error {
    // Try to get access token from OAuth credentials
    cfgPath, err := config.GetConfigPath()
    if err != nil {
        return fmt.Errorf("failed to get config path: %w", err)
    }
    dir := filepath.Dir(cfgPath)
    credsPath := filepath.Join(dir, "gemini_oauth_creds.json")

    data, err := os.ReadFile(credsPath)
    if err != nil {
        return fmt.Errorf("failed to read credentials: %w", err)
    }

    var creds map[string]interface{}
    if err := json.Unmarshal(data, &creds); err != nil {
        return fmt.Errorf("failed to parse credentials: %w", err)
    }

    accessToken, ok := creds["access_token"].(string)
    if !ok || strings.TrimSpace(accessToken) == "" {
        return fmt.Errorf("no access token found in credentials")
    }

    pterm.Info.Printf("Enabling Gemini for Google Cloud API for project %s...\n", projectID)

    // Use the Service Usage API to enable the Gemini API
    client := &http.Client{Timeout: 30 * time.Second}
    api := "cloudaicompanion.googleapis.com"

    endpoint := fmt.Sprintf("https://serviceusage.googleapis.com/v1/projects/%s/services/%s:enable",
        projectID, api)

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader("{}"))
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }
    req.Header.Set("Authorization", "Bearer "+accessToken)
    req.Header.Set("Content-Type", "application/json")

    resp, err := client.Do(req)
    if err != nil {
        return fmt.Errorf("network error: %w", err)
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)

    switch resp.StatusCode {
    case 200, 201:
        pterm.Success.Printf("✓ Gemini for Google Cloud API enabled successfully\n")
        return nil
    case 409:
        pterm.Info.Printf("✓ Gemini for Google Cloud API is already enabled\n")
        return nil
    case 403:
        return fmt.Errorf("insufficient permissions to enable APIs")
    default:
        var errResp struct {
            Error struct {
                Message string `json:"message"`
            } `json:"error"`
        }
        json.Unmarshal(body, &errResp)
        if errResp.Error.Message != "" {
            return fmt.Errorf("%s", errResp.Error.Message)
        }
        return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
    }
}

// readProjectIDFromAishCreds 嘗試從 AISH 設定目錄中的 gemini_oauth_creds.json 讀取 project_id
func readProjectIDFromAishCreds() string {
    cfgPath, err := config.GetConfigPath()
    if err != nil {
        return ""
    }
    dir := filepath.Dir(cfgPath)
    path := filepath.Join(dir, "gemini_oauth_creds.json")
    b, err := os.ReadFile(path)
    if err != nil || len(b) == 0 {
        return ""
    }
    m := map[string]any{}
    if err := json.Unmarshal(b, &m); err != nil {
        return ""
    }
    if v, ok := m["project_id"].(string); ok {
        v = strings.TrimSpace(v)
        if v != "" {
            return v
        }
    }
    return ""
}

// hasCommand returns true if the command is available in PATH
func hasCommand(name string) bool {
    _, err := exec.LookPath(name)
    return err == nil
}

// runCommandInteractive runs a command inheriting stdio (useful for auth/login flows)
func runCommandInteractive(name string, args ...string) error {
    cmd := exec.Command(name, args...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Stdin = os.Stdin
    return cmd.Run()
}

// getGcloudProjectID reads gcloud's current default project (returns empty or "(unset)" when not set)
func getGcloudProjectID() string {
    if !hasCommand("gcloud") {
        return ""
    }
    out, err := exec.Command("gcloud", "config", "get-value", "project").CombinedOutput()
    if err != nil {
        return ""
    }
    return strings.TrimSpace(string(out))
}

type gcloudProject struct {
    ProjectID       string `json:"projectId"`
    Name            string `json:"name"`
    LifecycleState  string `json:"lifecycleState"`
}

// listGcloudProjects returns ACTIVE projects visible to the current gcloud account
func listGcloudProjects() ([]gcloudProject, error) {
    if !hasCommand("gcloud") {
        return nil, fmt.Errorf("gcloud not found")
    }
    out, err := exec.Command("gcloud", "projects", "list", "--format=json").CombinedOutput()
    if err != nil {
        return nil, fmt.Errorf("gcloud list failed: %v", err)
    }
    var arr []gcloudProject
    if err := json.Unmarshal(out, &arr); err != nil {
        return nil, fmt.Errorf("parse gcloud json failed: %v", err)
    }
    // filter ACTIVE
    res := make([]gcloudProject, 0, len(arr))
    for _, p := range arr {
        if strings.EqualFold(p.LifecycleState, "ACTIVE") || p.LifecycleState == "" {
            res = append(res, p)
        }
    }
    return res, nil
}

// getGcloudProjectName returns the human-friendly project name via gcloud (best-effort)
func getGcloudProjectName(projectID string) string {
    if !hasCommand("gcloud") {
        return ""
    }
    out, err := exec.Command("gcloud", "projects", "describe", projectID, "--format=json").CombinedOutput()
    if err != nil || len(out) == 0 {
        return ""
    }
    var m struct {
        Name string `json:"name"`
    }
    if json.Unmarshal(out, &m) == nil {
        return strings.TrimSpace(m.Name)
    }
    return ""
}

// displayLabelForProject formats a project as "Name (projectId)" when name is available
// It tries OAuth (Resource Manager) first, then gcloud, finally falls back to projectId only.
func displayLabelForProject(projectID string) string {
    id := strings.TrimSpace(projectID)
    if id == "" { return "" }
    // Try OAuth CRM lookup
    if p, err := auth.GetProject(context.Background(), id); err == nil && p != nil {
        name := strings.TrimSpace(firstNonEmpty(p.DisplayName, firstNonEmpty(p.Name, p.ProjectID)))
        if name != "" && !strings.EqualFold(name, id) {
            return fmt.Sprintf("%s (%s)", name, id)
        }
    }
    // Fallback to gcloud
    if name := getGcloudProjectName(id); name != "" && !strings.EqualFold(name, id) {
        return fmt.Sprintf("%s (%s)", name, id)
    }
    return id
}

// getGcloudAccount returns the current gcloud account email (best-effort)
func getGcloudAccount() string {
    if !hasCommand("gcloud") {
        return ""
    }
    out, err := exec.Command("gcloud", "config", "get-value", "account").CombinedOutput()
    if err != nil {
        return ""
    }
    return strings.TrimSpace(string(out))
}

// ensureGcloudInstalled attempts to install Google Cloud SDK on supported OSes.
// On macOS uses Homebrew; on Linux tries apt/yum if available; otherwise returns an error with guidance.
func ensureGcloudInstalled() error {
    if hasCommand("gcloud") {
        return nil
    }
    switch runtime.GOOS {
    case "darwin":
        if hasCommand("brew") {
            pterm.Info.Println("Installing gcloud via Homebrew (brew install --cask google-cloud-sdk)...")
            if err := runCommandInteractive("brew", "install", "--cask", "google-cloud-sdk"); err != nil {
                return fmt.Errorf("brew install failed: %w", err)
            }
        } else {
            return fmt.Errorf("Homebrew not found. Install Homebrew (https://brew.sh) or install gcloud manually: https://cloud.google.com/sdk/docs/install")
        }
    case "linux":
        if hasCommand("apt-get") {
            pterm.Info.Println("Installing gcloud via apt-get...")
            if err := runCommandInteractive("sudo", "apt-get", "update"); err != nil {
                return fmt.Errorf("apt-get update failed: %w", err)
            }
            if err := runCommandInteractive("sudo", "apt-get", "install", "-y", "google-cloud-cli"); err != nil {
                return fmt.Errorf("apt-get install failed: %w", err)
            }
        } else if hasCommand("yum") || hasCommand("dnf") {
            mgr := "yum"
            if hasCommand("dnf") { mgr = "dnf" }
            pterm.Info.Printf("Installing gcloud via %s...\n", mgr)
            if err := runCommandInteractive("sudo", mgr, "install", "-y", "google-cloud-cli"); err != nil {
                return fmt.Errorf("%s install failed: %w", mgr, err)
            }
        } else {
            return fmt.Errorf("unsupported Linux package manager; install gcloud manually: https://cloud.google.com/sdk/docs/install")
        }
    case "windows":
        return fmt.Errorf("please install gcloud using winget or choco, then re-run: https://cloud.google.com/sdk/docs/install")
    default:
        return fmt.Errorf("unsupported OS for automatic installation; install gcloud manually: https://cloud.google.com/sdk/docs/install")
    }
    if !hasCommand("gcloud") {
        return fmt.Errorf("gcloud still not found after installation")
    }
    return nil
}
