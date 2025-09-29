package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	aerrors "github.com/TonnyWong1052/aish/internal/errors"
)

// ValidationError represents a configuration validation error with enhanced user guidance
type ValidationError struct {
	Field       string   `json:"field"`
	Value       string   `json:"value,omitempty"`
	Message     string   `json:"message"`
	Suggestions []string `json:"suggestions,omitempty"` // Actionable suggestions for fixing the error
	Severity    string   `json:"severity"`              // "error", "warning", "info"
}

func (e ValidationError) Error() string {
	var severity string
	switch e.Severity {
	case "warning":
		severity = "⚠️  WARNING"
	case "info":
		severity = "ℹ️  INFO"
	default:
		severity = "❌ ERROR"
	}

	var result string
	if e.Value != "" {
		result = fmt.Sprintf("%s: Config field '%s' value '%s' is invalid: %s", severity, e.Field, e.Value, e.Message)
	} else {
		result = fmt.Sprintf("%s: Config field '%s' is invalid: %s", severity, e.Field, e.Message)
	}

	if len(e.Suggestions) > 0 {
		result += "\n  Suggestions:"
		for _, suggestion := range e.Suggestions {
			result += fmt.Sprintf("\n  - %s", suggestion)
		}
	}

	return result
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "No validation errors"
	}
	if len(e) == 1 {
		return e[0].Error()
	}

	var messages []string
	for _, err := range e {
		messages = append(messages, err.Error())
	}
	return fmt.Sprintf("Found %d configuration errors:\n- %s", len(e), strings.Join(messages, "\n- "))
}

// Validator configuration validator
type Validator struct {
	errors []ValidationError
}

// NewValidator creates a new configuration validator
func NewValidator() *Validator {
	return &Validator{
		errors: make([]ValidationError, 0),
	}
}

// AddError adds a validation error with default severity
func (v *Validator) AddError(field, value, message string) {
	v.AddErrorWithSuggestions(field, value, message, nil, "error")
}

// AddWarning adds a validation warning
func (v *Validator) AddWarning(field, value, message string, suggestions []string) {
	v.AddErrorWithSuggestions(field, value, message, suggestions, "warning")
}

// AddInfo adds informational validation message
func (v *Validator) AddInfo(field, value, message string, suggestions []string) {
	v.AddErrorWithSuggestions(field, value, message, suggestions, "info")
}

// AddErrorWithSuggestions adds a validation error with suggestions and severity
func (v *Validator) AddErrorWithSuggestions(field, value, message string, suggestions []string, severity string) {
	v.errors = append(v.errors, ValidationError{
		Field:       field,
		Value:       value,
		Message:     message,
		Suggestions: suggestions,
		Severity:    severity,
	})
}

// HasErrors checks if there are validation errors
func (v *Validator) HasErrors() bool {
	return len(v.errors) > 0
}

// GetErrors gets the validation error list
func (v *Validator) GetErrors() ValidationErrors {
	return ValidationErrors(v.errors)
}

// Validate validates configuration
func (c *Config) Validate() error {
	validator := NewValidator()

	// Validate basic configuration
	validator.validateBasicConfig(c)

	// Validate provider configuration
	validator.validateProviders(c)

	// Validate user preferences
	validator.validateUserPreferences(c)

    if validator.HasErrors() {
        return aerrors.WrapError(validator.GetErrors(), aerrors.ErrConfigValidation, "configuration validation failed")
    }

	return nil
}

// validateBasicConfigForInit validates basic configuration with lenient rules for initialization
func (v *Validator) validateBasicConfigForInit(c *Config) {
	// 對於初始化，只檢查必要的結構性問題，不檢查API密鑰等
	if c.DefaultProvider == "" && len(c.Providers) > 0 {
		// 設置第一個可用的提供商
		for name := range c.Providers {
			c.DefaultProvider = name
			break
		}
	}

	// 確保至少有一個提供商定義（即使沒有完全配置）
	if len(c.Providers) == 0 {
		v.AddErrorWithSuggestions("providers", "",
			"必須配置至少一個LLM提供商",
			[]string{
				"運行 'aish init' 來設置LLM提供商",
				"手動配置OpenAI: 'aish config set providers.openai.api_key YOUR_KEY'",
				"使用Gemini CLI (無需API密鑰): 'aish config set default_provider gemini-cli'",
			}, "error")
	}
}

// validateBasicConfig validates basic configuration
func (v *Validator) validateBasicConfig(c *Config) {
	// Validate default provider
	if c.DefaultProvider == "" {
		v.AddErrorWithSuggestions("default_provider", c.DefaultProvider,
			"默認提供商不能為空",
			[]string{
				"運行 'aish init' 來設置默認提供商",
				"使用 'aish config set default_provider <provider>' 設置默認提供商",
				"可選提供商: openai, gemini, gemini-cli",
			}, "error")
	} else {
		// Check if default provider exists in provider list
		if _, exists := c.Providers[c.DefaultProvider]; !exists {
			availableProviders := make([]string, 0, len(c.Providers))
			for name := range c.Providers {
				availableProviders = append(availableProviders, name)
			}
			suggestions := []string{
				fmt.Sprintf("設置為可用的提供商之一: %v", availableProviders),
				"運行 'aish config show' 查看當前配置",
				"運行 'aish init' 重新配置",
			}
			if len(availableProviders) > 0 {
				suggestions = append(suggestions, fmt.Sprintf("使用 'aish config set default_provider %s' 設置為第一個可用提供商", availableProviders[0]))
			}
			v.AddErrorWithSuggestions("default_provider", c.DefaultProvider,
				"默認提供商在提供商配置中不存在",
				suggestions, "error")
		}
	}

	// Validate provider configuration cannot be empty
	if len(c.Providers) == 0 {
		v.AddErrorWithSuggestions("providers", "",
			"必須配置至少一個LLM提供商",
			[]string{
				"運行 'aish init' 來設置LLM提供商",
				"手動配置OpenAI: 'aish config set providers.openai.api_key YOUR_KEY'",
				"使用Gemini CLI (無需API密鑰): 'aish config set default_provider gemini-cli'",
				"查看支持的提供商: openai, gemini, gemini-cli",
			}, "error")
	}

	// Add helpful info for first-time setup
	if !c.Enabled {
		v.AddInfo("enabled", "false",
			"AISH當前已禁用",
			[]string{
				"使用 'aish config set enabled true' 啟用",
				"運行 'aish init' 進行完整設置",
			})
	}
}

// validateProvidersForInit validates provider configuration with lenient rules for initialization
func (v *Validator) validateProvidersForInit(c *Config) {
	// 對於初始化，只檢查基本的結構性問題，不檢查API密鑰或項目ID
	supportedProviders := make(map[string]bool)
	for _, provider := range GetSupportedProviders() {
		supportedProviders[provider] = true
	}

	for name, provider := range c.Providers {
		fieldPrefix := fmt.Sprintf("providers.%s", name)

		// Check if provider name is supported
		if !supportedProviders[name] {
			v.AddError(fieldPrefix, name, "unsupported provider type")
			continue
		}

		// 只檢查端點URL格式（如果提供）
		if provider.APIEndpoint != "" {
			if err := v.validateURL(provider.APIEndpoint); err != nil {
				v.AddError(fieldPrefix+".api_endpoint", provider.APIEndpoint, err.Error())
			}
		}

		// 確保模型名稱不為完全空
		if provider.Model == "" {
			v.AddError(fieldPrefix+".model", provider.Model, "模型名稱不能為空")
		}
	}
}

// validateProviders validates provider configuration
func (v *Validator) validateProviders(c *Config) {
	supportedProviders := make(map[string]bool)
	for _, provider := range GetSupportedProviders() {
		supportedProviders[provider] = true
	}

	for name, provider := range c.Providers {
		fieldPrefix := fmt.Sprintf("providers.%s", name)

		// Check if provider name is supported
		if !supportedProviders[name] {
			v.AddError(fieldPrefix, name, "unsupported provider type")
			continue
		}

		// 驗證 API 端點
		if provider.APIEndpoint != "" {
			if err := v.validateURL(provider.APIEndpoint); err != nil {
				v.AddError(fieldPrefix+".api_endpoint", provider.APIEndpoint, err.Error())
			}
		}

		// 根據提供商類型驗證必需字段
		switch name {
		case ProviderOpenAI:
			v.validateOpenAIProvider(fieldPrefix, provider)
		case ProviderGemini:
			v.validateGeminiProvider(fieldPrefix, provider)
		case ProviderGeminiCLI:
			v.validateGeminiCLIProvider(fieldPrefix, provider)
		}

		// 驗證模型名稱不能為空
		if provider.Model == "" {
			v.AddError(fieldPrefix+".model", provider.Model, "模型名稱不能為空")
		}
	}
}

// validateOpenAIProvider 驗證 OpenAI 提供商配置
func (v *Validator) validateOpenAIProvider(fieldPrefix string, provider ProviderConfig) {
	// API Key validation with helpful guidance
	if provider.APIKey == "" || provider.APIKey == "YOUR_OPENAI_API_KEY" {
		v.AddWarning(fieldPrefix+".api_key", provider.APIKey,
			"OpenAI API密鑰未設置或使用預設值",
			[]string{
				"從 https://platform.openai.com/api-keys 獲取API密鑰",
				"使用命令設置: 'aish config set providers.openai.api_key sk-your-key'",
				"確保密鑰以 'sk-' 開頭",
				"檢查API密鑰是否有足夠的額度",
			})
	} else if !strings.HasPrefix(provider.APIKey, "sk-") && !strings.HasPrefix(provider.APIKey, "pk-") {
		v.AddWarning(fieldPrefix+".api_key", provider.APIKey,
			"OpenAI API密鑰格式可能不正確",
			[]string{
				"OpenAI API密鑰通常以 'sk-' 開頭",
				"確認密鑰是從 https://platform.openai.com/api-keys 複製的",
				"檢查是否有額外的空格或字符",
			})
	}

	if provider.APIEndpoint == "" {
		v.AddErrorWithSuggestions(fieldPrefix+".api_endpoint", provider.APIEndpoint,
			"OpenAI API端點不能為空",
			[]string{
				"使用官方端點: 'aish config set providers.openai.api_endpoint https://api.openai.com/v1'",
				"或使用兼容的API端點",
				"確保端點支持OpenAI API格式",
			}, "error")
	}

	// Model validation with suggestions
	recommendedModels := []string{
		"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-4", "gpt-3.5-turbo",
	}
	if provider.Model == "" {
		v.AddWarning(fieldPrefix+".model", provider.Model,
			"OpenAI模型未指定",
			[]string{
				"推薦模型: " + strings.Join(recommendedModels, ", "),
				"設置模型: 'aish config set providers.openai.model gpt-4o'",
				"查看可用模型: https://platform.openai.com/docs/models",
			})
	} else if !v.isValidModel(provider.Model, recommendedModels) {
		v.AddInfo(fieldPrefix+".model", provider.Model,
			"使用非標準OpenAI模型",
			[]string{
				"確認模型存在且可用",
				"推薦模型: " + strings.Join(recommendedModels, ", "),
				"檢查模型權限和額度限制",
			})
	}
}

// validateGeminiProvider 驗證 Gemini 提供商配置
func (v *Validator) validateGeminiProvider(fieldPrefix string, provider ProviderConfig) {
	if provider.APIKey == "" || provider.APIKey == "YOUR_GEMINI_API_KEY" {
		v.AddWarning(fieldPrefix+".api_key", provider.APIKey,
			"Gemini API密鑰未設置或使用預設值",
			[]string{
				"從 https://aistudio.google.com/app/apikey 獲取API密鑰",
				"使用命令設置: 'aish config set providers.gemini.api_key YOUR_KEY'",
				"或使用免費的Gemini CLI: 'aish config set default_provider gemini-cli'",
				"Gemini API提供免費額度供測試使用",
			})
	}

	if provider.APIEndpoint == "" {
		v.AddErrorWithSuggestions(fieldPrefix+".api_endpoint", provider.APIEndpoint,
			"Gemini API端點不能為空",
			[]string{
				"使用官方端點: 'aish config set providers.gemini.api_endpoint https://generativelanguage.googleapis.com/v1'",
				"確保端點支持Gemini API格式",
			}, "error")
	}

	// 驗證常見的 Gemini 模型名稱
	recommendedModels := []string{
		"gemini-1.5-pro", "gemini-1.5-flash", "gemini-pro", "gemini-pro-vision",
	}
	if provider.Model == "" {
		v.AddWarning(fieldPrefix+".model", provider.Model,
			"Gemini模型未指定",
			[]string{
				"推薦模型: " + strings.Join(recommendedModels, ", "),
				"設置模型: 'aish config set providers.gemini.model gemini-1.5-pro'",
				"查看可用模型: https://ai.google.dev/models/gemini",
			})
	} else if !v.isValidModel(provider.Model, recommendedModels) {
		v.AddInfo(fieldPrefix+".model", provider.Model,
			"使用非標準Gemini模型",
			[]string{
				"確認模型存在且可用",
				"推薦模型: " + strings.Join(recommendedModels, ", "),
				"檢查模型是否在您的區域可用",
			})
	}
}

// validateGeminiCLIProvider 驗證 Gemini CLI 提供商配置
func (v *Validator) validateGeminiCLIProvider(fieldPrefix string, provider ProviderConfig) {
	if provider.Project == "" || provider.Project == "YOUR_GEMINI_PROJECT_ID" {
		v.AddWarning(fieldPrefix+".project", provider.Project,
			"Google Cloud項目ID未設置或使用預設值",
			[]string{
				"安裝Gemini CLI: https://github.com/google/generative-ai-cli",
				"登錄Google Cloud: 'gemini-cli auth login'",
				"設置項目ID: 'aish config set providers.gemini-cli.project YOUR_PROJECT_ID'",
				"查看當前項目: 'gcloud config get-value project'",
				"Gemini CLI是推薦的免費選項",
			})
	}

	if provider.APIEndpoint == "" {
		v.AddErrorWithSuggestions(fieldPrefix+".api_endpoint", provider.APIEndpoint,
			"Gemini CLI API端點不能為空",
			[]string{
				"使用默認端點: 'aish config set providers.gemini-cli.api_endpoint https://cloudcode-pa.googleapis.com/v1internal'",
				"確保已安裝和配置Gemini CLI",
			}, "error")
	}

	// Add helpful info for Gemini CLI setup
	v.AddInfo(fieldPrefix, "",
		"Gemini CLI是免費且易於設置的選項",
		[]string{
			"無需API密鑰，使用Google帳戶認證",
			"運行 'gemini-cli auth login' 進行身份驗證",
			"更高的免費使用限制",
			"適合個人和開發使用",
		})
}

// validateUserPreferences 驗證用戶偏好設置
func (v *Validator) validateUserPreferences(c *Config) {
	prefs := c.UserPreferences

	// 驗證語言設置
	validLanguages := []string{
		"english", "en",
		"zh-TW", "zh-CN", "zh", "chinese",
		"ja", "japanese",
		"ko", "korean",
		"es", "spanish",
		"fr", "french",
		"de", "german",
	}
	if prefs.Language != "" && !v.contains(validLanguages, prefs.Language) {
		v.AddWarning("user_preferences.language", prefs.Language,
			"Unsupported language setting",
			[]string{
				"Supported languages: english/en, zh-TW/zh-CN/chinese, ja/japanese, ko/korean, es/spanish, fr/french, de/german",
				"Set language: 'aish config set language english' for English",
				"Use ISO codes (en, zh-TW, ja, ko, es, fr, de) or full names (english, chinese, japanese, etc.)",
				"Default language is English",
			})
	} else if prefs.Language == "" {
		v.AddInfo("user_preferences.language", "",
			"Language not specified, using default (English)",
			[]string{
				"Set language explicitly: 'aish config set language en'",
				"Available languages: en, zh, ja",
			})
	}

	// 驗證上下文配置
	v.validateContextConfig("user_preferences.context", prefs.Context)

	// 驗證日誌配置
	v.validateLoggingConfig("user_preferences.logging", prefs.Logging)

	// 驗證緩存配置
	v.validateCacheConfig("user_preferences.cache", prefs.Cache)
}

// validateContextConfig 驗證上下文配置
func (v *Validator) validateContextConfig(fieldPrefix string, context ContextConfig) {
	if context.MaxHistoryEntries < 0 {
		v.AddError(fieldPrefix+".max_history_entries", fmt.Sprintf("%d", context.MaxHistoryEntries), "最大歷史條目數不能為負數")
	}
	if context.MaxHistoryEntries > 100 {
		v.AddError(fieldPrefix+".max_history_entries", fmt.Sprintf("%d", context.MaxHistoryEntries), "最大歷史條目數不應超過 100")
	}
}

// validateLoggingConfig 驗證日誌配置
func (v *Validator) validateLoggingConfig(fieldPrefix string, logging LoggingConfig) {
	// 驗證日誌級別
	validLevels := GetValidLogLevels()
	if logging.Level != "" && !v.contains(validLevels, logging.Level) {
		v.AddError(fieldPrefix+".level", logging.Level, "無效的日誌級別")
	}

	// 驗證日誌格式
	validFormats := GetValidLogFormats()
	if logging.Format != "" && !v.contains(validFormats, logging.Format) {
		v.AddError(fieldPrefix+".format", logging.Format, "無效的日誌格式")
	}

	// 驗證輸出類型
	validOutputs := GetValidLogOutputs()
	if logging.Output != "" && !v.contains(validOutputs, logging.Output) {
		v.AddError(fieldPrefix+".output", logging.Output, "無效的日誌輸出類型")
	}

	// 驗證日誌文件路徑
	if logging.Output == "file" || logging.Output == "both" {
		if logging.LogFile == "" {
			v.AddError(fieldPrefix+".log_file", logging.LogFile, "使用文件輸出時日誌文件路徑不能為空")
		} else {
			// 檢查日誌文件目錄是否可以創建
			logDir := filepath.Dir(logging.LogFile)
			if err := os.MkdirAll(logDir, DefaultDirPermissions); err != nil {
				v.AddError(fieldPrefix+".log_file", logging.LogFile, fmt.Sprintf("無法創建日誌目錄: %s", err.Error()))
			}
		}
	}

	// 驗證文件大小設置
	if logging.MaxSize <= 0 {
		v.AddError(fieldPrefix+".max_size", fmt.Sprintf("%d", logging.MaxSize), "最大文件大小必須大於 0")
	}
	if logging.MaxSize > 1000 {
		v.AddError(fieldPrefix+".max_size", fmt.Sprintf("%d", logging.MaxSize), "最大文件大小不應超過 1000MB")
	}

	// 驗證備份文件數量
	if logging.MaxBackups < 0 {
		v.AddError(fieldPrefix+".max_backups", fmt.Sprintf("%d", logging.MaxBackups), "最大備份文件數量不能為負數")
	}
	if logging.MaxBackups > 100 {
		v.AddError(fieldPrefix+".max_backups", fmt.Sprintf("%d", logging.MaxBackups), "最大備份文件數量不應超過 100")
	}
}

// validateCacheConfig 驗證緩存配置
func (v *Validator) validateCacheConfig(fieldPrefix string, cache CacheConfig) {
	// 驗證最大條目數
	if cache.MaxEntries < 0 {
		v.AddError(fieldPrefix+".max_entries", fmt.Sprintf("%d", cache.MaxEntries), "最大緩存條目數不能為負數")
	}
	if cache.MaxEntries > 10000 {
		v.AddError(fieldPrefix+".max_entries", fmt.Sprintf("%d", cache.MaxEntries), "最大緩存條目數不應超過 10000")
	}

	// 驗證TTL設置
	if cache.DefaultTTLHours <= 0 {
		v.AddError(fieldPrefix+".default_ttl_hours", fmt.Sprintf("%d", cache.DefaultTTLHours), "默認TTL必須大於 0")
	}
	if cache.DefaultTTLHours > 168 { // 7天
		v.AddError(fieldPrefix+".default_ttl_hours", fmt.Sprintf("%d", cache.DefaultTTLHours), "默認TTL不應超過 168 小時（7天）")
	}

	if cache.SuggestionTTLHours <= 0 {
		v.AddError(fieldPrefix+".suggestion_ttl_hours", fmt.Sprintf("%d", cache.SuggestionTTLHours), "建議緩存TTL必須大於 0")
	}
	if cache.SuggestionTTLHours > 72 { // 3天
		v.AddError(fieldPrefix+".suggestion_ttl_hours", fmt.Sprintf("%d", cache.SuggestionTTLHours), "建議緩存TTL不應超過 72 小時（3天）")
	}

	if cache.CommandTTLHours <= 0 {
		v.AddError(fieldPrefix+".command_ttl_hours", fmt.Sprintf("%d", cache.CommandTTLHours), "命令緩存TTL必須大於 0")
	}
	if cache.CommandTTLHours > 168 { // 7天
		v.AddError(fieldPrefix+".command_ttl_hours", fmt.Sprintf("%d", cache.CommandTTLHours), "命令緩存TTL不應超過 168 小時（7天）")
	}

	// 驗證相似度設置
	if cache.SimilarityThreshold < 0.0 || cache.SimilarityThreshold > 1.0 {
		v.AddError(fieldPrefix+".similarity_threshold", fmt.Sprintf("%.2f", cache.SimilarityThreshold), "相似度閾值必須在 0.0 到 1.0 之間")
	}

	if cache.MaxSimilarityCache < 0 {
		v.AddError(fieldPrefix+".max_similarity_cache", fmt.Sprintf("%d", cache.MaxSimilarityCache), "相似度緩存最大條目數不能為負數")
	}
	if cache.MaxSimilarityCache > 5000 {
		v.AddError(fieldPrefix+".max_similarity_cache", fmt.Sprintf("%d", cache.MaxSimilarityCache), "相似度緩存最大條目數不應超過 5000")
	}
}

// validateURL 驗證 URL 格式
func (v *Validator) validateURL(urlStr string) error {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("URL 格式無效: %s", err.Error())
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL 必須使用 http 或 https 協議")
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("URL 必須包含主機名")
	}

	return nil
}

// isValidModel 檢查模型是否在有效列表中
func (v *Validator) isValidModel(model string, validModels []string) bool {
	return v.contains(validModels, model)
}

// contains 檢查切片是否包含特定字符串
func (v *Validator) contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func defaultLogFilePath() string {
	if home, err := os.UserHomeDir(); err == nil {
		candidateDir := filepath.Join(home, DefaultConfigDir, DefaultLogDir)
		if err := os.MkdirAll(candidateDir, DefaultDirPermissions); err == nil {
			return filepath.Join(candidateDir, DefaultLogFileName)
		}
	}

	fallbackDir := filepath.Join(os.TempDir(), AppName, DefaultLogDir)
	_ = os.MkdirAll(fallbackDir, DefaultDirPermissions)
	return filepath.Join(fallbackDir, DefaultLogFileName)
}

// ValidateAndFix 驗證配置並自動修復簡單問題
func (c *Config) ValidateAndFix() ([]string, error) {
	var fixes []string

	// 修復空的默認提供商
	if c.DefaultProvider == "" && len(c.Providers) > 0 {
		// 選擇第一個可用的提供商
		for name := range c.Providers {
			c.DefaultProvider = name
			fixes = append(fixes, fmt.Sprintf("設置默認提供商為: %s", name))
			break
		}
	}

	// 修復日誌文件路徑
	if c.UserPreferences.Logging.LogFile == "" {
		c.UserPreferences.Logging.LogFile = defaultLogFilePath()
		fixes = append(fixes, "設置默認日誌文件路徑")
	}

	// 修復無效的配置值
	if c.UserPreferences.Context.MaxHistoryEntries <= 0 {
		c.UserPreferences.Context.MaxHistoryEntries = 10
		fixes = append(fixes, "修復最大歷史條目數為 10")
	}

	if c.UserPreferences.Logging.MaxSize <= 0 {
		c.UserPreferences.Logging.MaxSize = 10
		fixes = append(fixes, "修復日誌文件最大大小為 10MB")
	}

	if c.UserPreferences.Logging.MaxBackups < 0 {
		c.UserPreferences.Logging.MaxBackups = 5
		fixes = append(fixes, "修復日誌備份數量為 5")
	}

	// 修復語言:若為空或不在允許清單,回退為 english
	validLanguages := []string{
		"english", "en",
		"zh-tw", "zh-cn", "zh", "chinese",
		"ja", "japanese",
		"ko", "korean",
		"es", "spanish",
		"fr", "french",
		"de", "german",
	}
	lang := strings.ToLower(strings.TrimSpace(c.UserPreferences.Language))
	isValid := false
	for _, v := range validLanguages {
		if lang == v {
			isValid = true
			break
		}
	}
	if !isValid {
		c.UserPreferences.Language = "english"
		fixes = append(fixes, "Fixed language to english (English)")
	}

	// 在修復後進行基本驗證（不包括警告，只檢查致命錯誤）
	validator := NewValidator()
	validator.validateBasicConfigForInit(c)  // Use lenient validation for init
	validator.validateProvidersForInit(c)    // Use lenient validation for init

	// 只檢查嚴重錯誤
	for _, validationErr := range validator.GetErrors() {
		if validationErr.Severity == "error" {
			return fixes, aerrors.WrapError(validator.GetErrors(), aerrors.ErrConfigValidation, "configuration validation failed after fixes")
		}
	}

	return fixes, nil
}

// ValidateWithRecommendations provides comprehensive validation with actionable recommendations
func (c *Config) ValidateWithRecommendations() (ValidationErrors, []string, error) {
	validator := NewValidator()

	// Validate basic configuration
	validator.validateBasicConfig(c)

	// Validate provider configuration
	validator.validateProviders(c)

	// Validate user preferences
	validator.validateUserPreferences(c)

	// Add configuration health checks
	validator.addConfigurationHealthChecks(c)

	errors := validator.GetErrors()
	recommendations := validator.generateRecommendations(c)

	// Only return fatal errors if any exist
	var fatalErrors ValidationErrors
	for _, err := range errors {
		if err.Severity == "error" {
			fatalErrors = append(fatalErrors, err)
		}
	}

	if len(fatalErrors) > 0 {
		return errors, recommendations, fatalErrors
	}

	return errors, recommendations, nil
}

// addConfigurationHealthChecks adds proactive health checks and optimization suggestions
func (v *Validator) addConfigurationHealthChecks(c *Config) {
	// Check if using recommended setup
	if c.DefaultProvider == "gemini-cli" {
		v.AddInfo("setup", "gemini-cli",
			"Using recommended Gemini CLI provider",
			[]string{
				"Gemini CLI offers free usage and easy setup",
				"Ensure authentication with 'gemini-cli auth login'",
				"Check status with 'gemini-cli auth list'",
			})
	}

	// Check if multiple providers are configured
	configuredProviders := 0
	for _, provider := range c.Providers {
		if (provider.APIKey != "" && provider.APIKey != "YOUR_OPENAI_API_KEY" && provider.APIKey != "YOUR_GEMINI_API_KEY") ||
			(provider.Project != "" && provider.Project != "YOUR_GEMINI_PROJECT_ID") {
			configuredProviders++
		}
	}

	if configuredProviders == 0 {
		v.AddWarning("providers", "",
			"No providers fully configured",
			[]string{
				"Run 'aish init' to set up a provider",
				"Recommended: Use Gemini CLI for free usage",
				"Or configure OpenAI/Gemini with API keys",
			})
	} else if configuredProviders > 1 {
		v.AddInfo("providers", "",
			"Multiple providers configured - excellent for fallback",
			[]string{
				"Switch providers with 'aish config set default_provider <name>'",
				"Use different providers for different use cases",
			})
	}

	// Performance optimization suggestions
	if !c.UserPreferences.Cache.Enabled {
		v.AddInfo("cache", "disabled",
			"Caching is disabled - may affect performance",
			[]string{
				"Enable caching: 'aish config set user_preferences.cache.enabled true'",
				"Caching improves response times for similar queries",
				"Reduces API usage and costs",
			})
	}
}

// generateRecommendations provides general setup and optimization recommendations
func (v *Validator) generateRecommendations(c *Config) []string {
	var recommendations []string

	// First-time setup recommendations
	if c.DefaultProvider == "" || len(c.Providers) == 0 {
		recommendations = append(recommendations,
			"🚀 First-time setup: Run 'aish init' for guided configuration",
			"💡 For free usage: Choose Gemini CLI during setup",
			"📚 Read the setup guide: https://github.com/TonnyWong1052/aish/blob/main/README.md",
		)
	}

	// Performance recommendations
	if c.UserPreferences.Context.MaxHistoryEntries > 50 {
		recommendations = append(recommendations,
			"⚡ Performance: Consider reducing max_history_entries to 20-30 for faster responses",
		)
	}

	// Security recommendations
	for providerName, provider := range c.Providers {
		if provider.APIKey != "" && !strings.HasPrefix(provider.APIKey, "sk-") && providerName == "openai" {
			recommendations = append(recommendations,
				"🔒 Security: Verify your OpenAI API key format (should start with 'sk-')",
			)
		}
	}

	// Usage optimization
	if len(c.UserPreferences.EnabledLLMTriggers) > 10 {
		recommendations = append(recommendations,
			"🎯 Optimization: Consider reducing enabled triggers for better performance",
		)
	}

	return recommendations
}
