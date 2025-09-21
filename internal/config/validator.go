package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/TonnyWong1052/aish/internal/errors"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string `json:"field"`
	Value   string `json:"value,omitempty"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	if e.Value != "" {
		return fmt.Sprintf("Config field '%s' value '%s' is invalid: %s", e.Field, e.Value, e.Message)
	}
	return fmt.Sprintf("Config field '%s' is invalid: %s", e.Field, e.Message)
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

// AddError adds a validation error
func (v *Validator) AddError(field, value, message string) {
	v.errors = append(v.errors, ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
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
		return errors.WrapError(validator.GetErrors(), errors.ErrConfigValidation, "configuration validation failed")
	}

	return nil
}

// validateBasicConfig validates basic configuration
func (v *Validator) validateBasicConfig(c *Config) {
	// Validate default provider
	if c.DefaultProvider == "" {
		v.AddError("default_provider", c.DefaultProvider, "Default provider cannot be empty")
	} else {
		// Check if default provider exists in provider list
		if _, exists := c.Providers[c.DefaultProvider]; !exists {
			v.AddError("default_provider", c.DefaultProvider, "Default provider does not exist in provider configuration")
		}
	}

	// Validate provider configuration cannot be empty
	if len(c.Providers) == 0 {
		v.AddError("providers", "", "Must configure at least one provider")
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
	// 對預設模板值採取寬鬆策略：不作為致命錯誤，允許離線模式先行使用
	if provider.APIKey == "" || provider.APIKey == "YOUR_OPENAI_API_KEY" {
		// 跳過致命錯誤，改由命令路徑在實際使用時檢查
		// v.AddError(fieldPrefix+".api_key", provider.APIKey, "OpenAI API 密鑰未設置")
	}

	if provider.APIEndpoint == "" {
		v.AddError(fieldPrefix+".api_endpoint", provider.APIEndpoint, "OpenAI API 端點不能為空")
	}

	// 驗證常見的 OpenAI 模型名稱
	validModels := []string{
		"gpt-3.5-turbo", "gpt-3.5-turbo-16k", "gpt-4", "gpt-4-32k", "gpt-4-turbo", "gpt-4o",
	}
	if provider.Model != "" && !v.isValidModel(provider.Model, validModels) {
		// 這只是警告，不是錯誤，因為可能有新的模型
		// v.AddError(fieldPrefix+".model", provider.Model, "可能不支持的 OpenAI 模型")
	}
}

// validateGeminiProvider 驗證 Gemini 提供商配置
func (v *Validator) validateGeminiProvider(fieldPrefix string, provider ProviderConfig) {
	if provider.APIKey == "" || provider.APIKey == "YOUR_GEMINI_API_KEY" {
		// 跳過致命錯誤，允許未配置情況下啟動（將退回離線/本地邏輯）
		// v.AddError(fieldPrefix+".api_key", provider.APIKey, "Gemini API 密鑰未設置")
	}

	if provider.APIEndpoint == "" {
		v.AddError(fieldPrefix+".api_endpoint", provider.APIEndpoint, "Gemini API 端點不能為空")
	}

	// 驗證常見的 Gemini 模型名稱
	validModels := []string{
		"gemini-pro", "gemini-pro-vision",
	}
	if provider.Model != "" && !v.isValidModel(provider.Model, validModels) {
		// 這只是警告，不是錯誤
		// v.AddError(fieldPrefix+".model", provider.Model, "可能不支持的 Gemini 模型")
	}
}

// validateGeminiCLIProvider 驗證 Gemini CLI 提供商配置
func (v *Validator) validateGeminiCLIProvider(fieldPrefix string, provider ProviderConfig) {
	if provider.Project == "" || provider.Project == "YOUR_GEMINI_PROJECT_ID" {
		// 跳過致命錯誤，允許未配置情況下啟動
		// v.AddError(fieldPrefix+".project", provider.Project, "Google Cloud 項目 ID 未設置")
	}

	if provider.APIEndpoint == "" {
		v.AddError(fieldPrefix+".api_endpoint", provider.APIEndpoint, "Gemini CLI API 端點不能為空")
	}
}

// validateUserPreferences 驗證用戶偏好設置
func (v *Validator) validateUserPreferences(c *Config) {
	prefs := c.UserPreferences

	// 驗證語言設置
	validLanguages := []string{"english", "chinese", "japanese"}
	if prefs.Language != "" && !v.contains(validLanguages, prefs.Language) {
		v.AddError("user_preferences.language", prefs.Language, "不支持的語言設置")
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

	// 修復語言：若為空或不在允許清單，回退為 english
	validLanguages := []string{"english", "chinese", "japanese"}
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
		fixes = append(fixes, "修復語言為 english")
	}

	// 在修復後進行驗證
	if err := c.Validate(); err != nil {
		return fixes, err
	}

	return fixes, nil
}
