package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"powerful-cli/internal/config"
	"powerful-cli/internal/errors"

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

	// Create temporary OpenAI provider to fetch models
	ctx := context.Background()
	client := &http.Client{Timeout: 10 * time.Second}

	apiURL := strings.TrimSuffix(cfg.APIEndpoint, "/") + "/models"
	// Use POST by default, fallback to GET if server returns 405
	postReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader("{}"))
	if err != nil {
		return "", fmt.Errorf("failed to create POST request: %w", err)
	}
	postReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	postReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(postReq)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}

	if resp.StatusCode == http.StatusMethodNotAllowed {
		// Fallback to GET for endpoints that only support GET
		resp.Body.Close()
		getReq, gerr := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if gerr != nil {
			return "", fmt.Errorf("failed to create GET request: %w", gerr)
		}
		getReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		getReq.Header.Set("Content-Type", "application/json")
		resp, err = client.Do(getReq)
		if err != nil {
			return "", fmt.Errorf("API request failed: %w", err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API returned error status code: %d", resp.StatusCode)
	}

	var modelsResp struct {
		Object string `json:"object"`
		Data   []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return "", fmt.Errorf("解析響應失敗: %w", err)
	}

	if modelsResp.Error != nil {
		return "", fmt.Errorf("API 錯誤: %s", modelsResp.Error.Message)
	}

	if len(modelsResp.Data) == 0 {
		return "", fmt.Errorf("未找到任何可用模型")
	}

	// 按類別分組模型
	gptModels := []string{}
	otherModels := []string{}

	for _, model := range modelsResp.Data {
		if strings.Contains(model.ID, "gpt-") {
			gptModels = append(gptModels, model.ID)
		} else {
			otherModels = append(otherModels, model.ID)
		}
	}

	pterm.Success.Printf("找到 %d 個可用模型\n", len(modelsResp.Data))

	// 構建選項列表
	allOptions := []string{}

	if len(gptModels) > 0 {
		pterm.Info.Printf("GPT 模型 (%d 個):\n", len(gptModels))
		for _, model := range gptModels {
			pterm.Printf("  • %s\n", model)
			allOptions = append(allOptions, model)
		}
	}

	if len(otherModels) > 0 {
		pterm.Info.Printf("其他模型 (%d 個):\n", len(otherModels))
		for _, model := range otherModels {
			pterm.Printf("  • %s\n", model)
			allOptions = append(allOptions, model)
		}
	}

	// 添加手動輸入選項
	allOptions = append(allOptions, "手動輸入其他模型名稱")

	// 設置默認選項
	defaultOption := cfg.Model
	if defaultOption == "" && len(gptModels) > 0 {
		// 優先選擇常用的 GPT 模型
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
		// 如果沒找到常用模型，使用第一個 GPT 模型
		if defaultOption == "" {
			defaultOption = gptModels[0]
		}
	}

	selectedModel, _ := pterm.DefaultInteractiveSelect.
		WithOptions(allOptions).
		WithDefaultOption(defaultOption).
		Show("選擇模型")

	if selectedModel == "手動輸入其他模型名稱" {
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

	allOptions := append(commonModels, "手動輸入其他模型名稱")

	selectedModel, _ := pterm.DefaultInteractiveSelect.
		WithOptions(allOptions).
		WithDefaultOption(cfg.Model).
		Show("選擇模型")

	if selectedModel == "手動輸入其他模型名稱" {
		return w.inputCustomModel(cfg)
	}

	return selectedModel, nil
}

// inputCustomModel manually input model name
func (w *ConfigWizard) inputCustomModel(cfg *config.ProviderConfig) (string, error) {
	pterm.Info.Println("您可以輸入任何 OpenAI 支持的模型名稱")
	pterm.Info.Println("例如: gpt-4o, gpt-4, gpt-3.5-turbo, text-davinci-003 等")

	customModel, _ := pterm.DefaultInteractiveTextInput.
		WithDefaultValue(cfg.Model).
		Show("輸入模型名稱")

	if strings.TrimSpace(customModel) == "" {
		return "", fmt.Errorf("模型名稱不能為空")
	}

	return strings.TrimSpace(customModel), nil
}

// configureGemini configures Gemini provider
func (w *ConfigWizard) configureGemini(cfg *config.ProviderConfig) error {
	pterm.DefaultHeader.Println("Gemini 配置")

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
			Show("輸入 Gemini API 端點")
		cfg.APIEndpoint = endpoint
	} else {
		cfg.APIEndpoint = defaultEndpoint
	}

	// API key
	pterm.Info.Println("您可以在 https://makersuite.google.com/app/apikey 獲取 API 密鑰")
	apiKey, _ := pterm.DefaultInteractiveTextInput.
		WithMask("*").
		WithDefaultValue(cfg.APIKey).
		Show("輸入您的 Gemini API 密鑰")
	cfg.APIKey = apiKey

	// Model selection
	commonModels := []string{
		"gemini-pro", "gemini-pro-vision",
	}

	if cfg.Model == "" {
		cfg.Model = "gemini-pro"
	}

	model, _ := pterm.DefaultInteractiveSelect.
		WithOptions(append(commonModels, "其他 (手動輸入)")).
		WithDefaultOption(cfg.Model).
		Show("選擇模型")

	if model == "其他 (手動輸入)" {
		customModel, _ := pterm.DefaultInteractiveTextInput.
			WithDefaultValue(cfg.Model).
			Show("輸入模型名稱")
		cfg.Model = customModel
	} else {
		cfg.Model = model
	}

	return nil
}

// configureGeminiCLI configures Gemini CLI provider
func (w *ConfigWizard) configureGeminiCLI(cfg *config.ProviderConfig) error {
	pterm.DefaultHeader.Println("Gemini CLI 配置")

	// API endpoint
	defaultEndpoint := "https://cloudcode-pa.googleapis.com/v1internal"
	if cfg.APIEndpoint == "" {
		cfg.APIEndpoint = defaultEndpoint
	}

	useCustomEndpoint, _ := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(cfg.APIEndpoint != defaultEndpoint).
		Show("Do you want to use a custom API endpoint?")

	if useCustomEndpoint {
		endpoint, _ := pterm.DefaultInteractiveTextInput.
			WithDefaultValue(cfg.APIEndpoint).
			Show("輸入 Gemini CLI API 端點")
		cfg.APIEndpoint = endpoint
	} else {
		cfg.APIEndpoint = defaultEndpoint
	}

	// 項目 ID
	pterm.Info.Println("您需要 Google Cloud 項目 ID")
	projectID, _ := pterm.DefaultInteractiveTextInput.
		WithDefaultValue(cfg.Project).
		Show("輸入您的 Google Cloud 項目 ID")
	cfg.Project = projectID

	// 模型選擇（僅支援 2.5 系列）
	commonModels := []string{
		"gemini-2.5-pro",
		"gemini-2.5-flash",
	}

	if cfg.Model == "" {
		cfg.Model = "gemini-2.5-flash"
	}

	model, _ := pterm.DefaultInteractiveSelect.
		WithOptions(append(commonModels, "其他 (手動輸入)")).
		WithDefaultOption(cfg.Model).
		Show("選擇模型")

	if model == "其他 (手動輸入)" {
		customModel, _ := pterm.DefaultInteractiveTextInput.
			WithDefaultValue(cfg.Model).
			Show("輸入模型名稱")
		cfg.Model = customModel
	} else {
		cfg.Model = model
	}

	return nil
}

// configureLanguage configures language preference
func (w *ConfigWizard) configureLanguage() error {
	pterm.DefaultHeader.Println("語言設置")

	languages := []string{"english", "zh-TW", "zh-CN", "japanese", "korean", "spanish", "french", "german", "italian", "portuguese", "russian", "arabic"}
	languageNames := map[string]string{
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

	pterm.Info.Println("選擇 AI 響應的語言:")
	for _, lang := range languages {
		pterm.Printf("• %s\n", languageNames[lang])
	}

	selectedLanguage, _ := pterm.DefaultInteractiveSelect.
		WithOptions(languages).
		WithDefaultOption(w.config.UserPreferences.Language).
		Show("選擇語言")

	w.config.UserPreferences.Language = selectedLanguage
	return nil
}

// configureErrorTriggers configures error triggers
func (w *ConfigWizard) configureErrorTriggers() error {
	pterm.DefaultHeader.Println("錯誤分析觸發器")
	pterm.Info.Println("選擇哪些類型的錯誤應該觸發 AI 分析:")

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
		"CommandNotFound":         "命令未找到",
		"FileNotFoundOrDirectory": "文件或目錄不存在",
		"PermissionDenied":        "權限被拒絕",
		"CannotExecute":           "無法執行",
		"InvalidArgumentOrOption": "無效參數或選項",
		"ResourceExists":          "資源已存在",
		"NotADirectory":           "不是目錄",
		"TerminatedBySignal":      "被信號終止",
		"GenericError":            "一般錯誤",
	}

	// 顯示選項
	pterm.Info.Println("錯誤類型說明:")
	for _, errorType := range errorTypes {
		pterm.Printf("• %s: %s\n", errorType, errorDescriptions[errorType])
	}

	selectedTypes, _ := MultiSelectNoHelp(
		"選擇要啟用 AI 分析的錯誤類型 (空格選擇，回車確認):",
		errorTypes,
		w.config.UserPreferences.EnabledLLMTriggers,
	)

	w.config.UserPreferences.EnabledLLMTriggers = selectedTypes
	return nil
}

// configureContext 配置上下文設置
func (w *ConfigWizard) configureContext() error {
	pterm.DefaultHeader.Println("上下文增強設置")

	// 最大歷史條目數
	maxHistoryStr, _ := pterm.DefaultInteractiveTextInput.
		WithDefaultValue(fmt.Sprintf("%d", w.config.UserPreferences.Context.MaxHistoryEntries)).
		Show("最大命令歷史條目數 (推薦: 10)")

	if maxHistory, err := strconv.Atoi(maxHistoryStr); err == nil {
		w.config.UserPreferences.Context.MaxHistoryEntries = maxHistory
	}

	// 是否包含目錄列表
	includeDir, _ := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(w.config.UserPreferences.Context.IncludeDirectories).
		Show("在上下文中包含當前目錄文件列表？")
	w.config.UserPreferences.Context.IncludeDirectories = includeDir

	// 是否過濾敏感命令
	filterSensitive, _ := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(w.config.UserPreferences.Context.FilterSensitiveCmd).
		Show("過濾包含敏感信息的命令 (如密碼、密鑰等)？")
	w.config.UserPreferences.Context.FilterSensitiveCmd = filterSensitive

	// 是否啟用增強分析
	enableEnhanced, _ := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(w.config.UserPreferences.Context.EnableEnhanced).
		Show("啟用增強上下文分析？")
	w.config.UserPreferences.Context.EnableEnhanced = enableEnhanced

	return nil
}

// configureLogging 配置日誌設置
func (w *ConfigWizard) configureLogging() error {
	pterm.DefaultHeader.Println("日誌設置")

	// 日誌級別
	levels := []string{"trace", "debug", "info", "warn", "error"}
	level, _ := pterm.DefaultInteractiveSelect.
		WithOptions(levels).
		WithDefaultOption(w.config.UserPreferences.Logging.Level).
		Show("選擇日誌級別")
	w.config.UserPreferences.Logging.Level = level

	// 日誌格式
	formats := []string{"text", "json"}
	format, _ := pterm.DefaultInteractiveSelect.
		WithOptions(formats).
		WithDefaultOption(w.config.UserPreferences.Logging.Format).
		Show("選擇日誌格式")
	w.config.UserPreferences.Logging.Format = format

	// 日誌輸出
	outputs := []string{"file", "console", "both"}
	output, _ := pterm.DefaultInteractiveSelect.
		WithOptions(outputs).
		WithDefaultOption(w.config.UserPreferences.Logging.Output).
		Show("選擇日誌輸出方式")
	w.config.UserPreferences.Logging.Output = output

	return nil
}

// configureCache 配置緩存設置
func (w *ConfigWizard) configureCache() error {
	pterm.DefaultHeader.Println("緩存設置")

	// 是否啟用緩存
	enabled, _ := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(w.config.UserPreferences.Cache.Enabled).
		Show("啟用響應緩存 (可提高響應速度並節省 API 費用)？")
	w.config.UserPreferences.Cache.Enabled = enabled

	if !enabled {
		pterm.Info.Println("緩存已禁用，跳過其他緩存設置")
		return nil
	}

	// 相似度匹配
	enableSimilarity, _ := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(w.config.UserPreferences.Cache.EnableSimilarity).
		Show("啟用智能相似度匹配 (為相似問題提供緩存響應)？")
	w.config.UserPreferences.Cache.EnableSimilarity = enableSimilarity

	if enableSimilarity {
		// 相似度閾值
		thresholdStr, _ := pterm.DefaultInteractiveTextInput.
			WithDefaultValue(fmt.Sprintf("%.2f", w.config.UserPreferences.Cache.SimilarityThreshold)).
			Show("相似度閾值 (0.0-1.0，推薦: 0.85)")

		if threshold, err := strconv.ParseFloat(thresholdStr, 64); err == nil {
			w.config.UserPreferences.Cache.SimilarityThreshold = threshold
		}
	}

	// 緩存大小
	maxEntriesStr, _ := pterm.DefaultInteractiveTextInput.
		WithDefaultValue(fmt.Sprintf("%d", w.config.UserPreferences.Cache.MaxEntries)).
		Show("最大緩存條目數 (推薦: 1000)")

	if maxEntries, err := strconv.Atoi(maxEntriesStr); err == nil {
		w.config.UserPreferences.Cache.MaxEntries = maxEntries
	}

	return nil
}

// finishConfiguration 完成配置
func (w *ConfigWizard) finishConfiguration() error {
	pterm.DefaultHeader.Println("配置完成")

	// 驗證配置
	pterm.Info.Println("正在驗證配置...")
	fixes, err := w.config.ValidateAndFix()
	if err != nil {
		pterm.Error.Println("配置驗證失敗:", err)

		retry, _ := pterm.DefaultInteractiveConfirm.
			WithDefaultValue(true).
			Show("是否重新運行配置向導？")

		if retry {
			return w.Run()
		}
		return err
	}

	if len(fixes) > 0 {
		pterm.Info.Println("自動修復了以下配置問題:")
		for _, fix := range fixes {
			pterm.Printf("• %s\n", fix)
		}
	}

	// 保存配置
	pterm.Info.Println("正在保存配置...")
	if err := w.config.Save(); err != nil {
		return errors.ErrConfigSaveFailed("", err)
	}

	// 顯示配置摘要
	w.showConfigurationSummary()

	pterm.Success.Println("🎉 AISH 配置完成！")
	pterm.Info.Println("您現在可以使用以下命令:")
	pterm.Printf("• %s: 安裝 shell hook\n", pterm.LightBlue("aish setup"))
	pterm.Printf("• %s: 測試 AI 命令生成\n", pterm.LightBlue("aish -p \"您的提示\""))
	pterm.Printf("• %s: 查看配置\n", pterm.LightBlue("aish config show"))

	return nil
}

// showConfigurationSummary 顯示配置摘要
func (w *ConfigWizard) showConfigurationSummary() {
	pterm.DefaultSection.Println("配置摘要")

	// 提供商信息
	pterm.Printf("• LLM 提供商: %s\n", w.config.DefaultProvider)
	if providerCfg, exists := w.config.Providers[w.config.DefaultProvider]; exists {
		pterm.Printf("• 模型: %s\n", providerCfg.Model)
		pterm.Printf("• API 端點: %s\n", providerCfg.APIEndpoint)
	}

	// 用戶偏好
	pterm.Printf("• 響應語言: %s\n", w.config.UserPreferences.Language)
	pterm.Printf("• 啟用的錯誤觸發器: %d 種\n", len(w.config.UserPreferences.EnabledLLMTriggers))

	// 功能狀態
	pterm.Printf("• 緩存: %s\n", boolToStatus(w.config.UserPreferences.Cache.Enabled))
	pterm.Printf("• 相似度匹配: %s\n", boolToStatus(w.config.UserPreferences.Cache.EnableSimilarity))
	pterm.Printf("• 上下文增強: %s\n", boolToStatus(w.config.UserPreferences.Context.EnableEnhanced))

	pterm.Println()
}

// boolToStatus 將布爾值轉換為狀態字符串
func boolToStatus(enabled bool) string {
	if enabled {
		return pterm.LightGreen("啟用")
	}
	return pterm.LightRed("禁用")
}
