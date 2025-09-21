package ui

import (
    "context"
    "fmt"
    "strconv"
    "strings"

    "github.com/TonnyWong1052/aish/internal/config"
    "github.com/TonnyWong1052/aish/internal/errors"
    "github.com/TonnyWong1052/aish/internal/llm/openai"
    "github.com/TonnyWong1052/aish/internal/prompt"

	"github.com/pterm/pterm"
)

// ConfigWizard configuration wizard
type ConfigWizard struct {
	config *config.Config
}

// NewConfigWizard creates a new configuration wizard
func NewConfigWizard(cfg *config.Config) *ConfigWizard {
	return &ConfigWizard{config: cfg}
}

// Run runs the configuration wizard
func (w *ConfigWizard) Run() error {
	// Show welcome message
	w.showWelcome()

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
		pterm.DefaultSection.Printf("Step %d/%d: %s", i+1, len(steps), step.Name)

		if err := step.Handler(); err != nil {
			if errors.HasCode(err, errors.ErrUserCancel) {
				pterm.Info.Println("Configuration cancelled")
				return err
			}
			return err
		}

		pterm.Success.Printf("✓ %s completed\n", step.Name)
		pterm.Println() // Add empty line separator
	}

	return nil
}

// ConfigStep configuration step
type ConfigStep struct {
	Name    string
	Handler func() error
}

// showWelcome shows welcome message
func (w *ConfigWizard) showWelcome() {
	pterm.DefaultHeader.WithFullWidth().Println("AISH Configuration Wizard")
	pterm.Info.Println("Welcome to AISH (AI Shell)!")
	pterm.Info.Println("This wizard will help you set up AISH's various features.")
	pterm.Info.Println("You can cancel the configuration at any time by pressing Ctrl+C.")
	pterm.Println()
}

// configureProvider configures LLM provider
func (w *ConfigWizard) configureProvider() error {
	// Show provider options
	providers := []string{"openai", "gemini", "gemini-cli"}
	descriptions := map[string]string{
		"openai":     "OpenAI GPT series models (requires API key)",
		"gemini":     "Google Gemini public API (requires API key)",
		"gemini-cli": "Google Cloud Code private API (requires OAuth)",
	}

	pterm.Info.Println("Available LLM providers:")
	for _, provider := range providers {
		pterm.Printf("• %s: %s\n", provider, descriptions[provider])
	}

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

    // Project ID
    pterm.Info.Println("You need a Google Cloud Project ID.")
    // 兩行 QA 風格提示，避免在 Show 標籤中使用換行導致游標錯位
    pterm.Println("Enter your Google Cloud Project ID:")
    projectID, _ := pterm.DefaultInteractiveTextInput.
        Show(">")
    cfg.Project = projectID

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

	return nil
}

// configureLanguage configures language preference
func (w *ConfigWizard) configureLanguage() error {
    pterm.DefaultHeader.Println("Language Settings")
    // Only English is supported for now. Set both UI and response language to English.
    pterm.Info.Println("Only English is supported at the moment. Setting response language to English.")
    w.config.UserPreferences.Language = "english"
    return nil
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
        return errors.ErrConfigSaveFailed("", err)
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
