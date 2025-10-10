package config

import (
	"testing"
)

func TestValidatorAddError(t *testing.T) {
	validator := NewValidator()

	validator.AddError("test_field", "test_value", "test message")

	if !validator.HasErrors() {
		t.Error("驗證器應該有錯誤")
	}

	errors := validator.GetErrors()
	if len(errors) != 1 {
		t.Errorf("期望 1 個錯誤，得到 %d", len(errors))
	}

	if errors[0].Field != "test_field" {
		t.Errorf("期望字段名 'test_field'，得到 '%s'", errors[0].Field)
	}
}

func TestValidateBasicConfig(t *testing.T) {
	// 測試默認提供商為空
	cfg := &Config{
		DefaultProvider: "",
		Providers:       make(map[string]ProviderConfig),
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("空的默認提供商應該導致驗證失敗")
	}

	// 測試默認提供商不在提供商列表中
	cfg.DefaultProvider = "nonexistent"
	err = cfg.Validate()
	if err == nil {
		t.Error("不存在的默認提供商應該導致驗證失敗")
	}
}

func TestValidateOpenAIProvider(t *testing.T) {
	cfg := &Config{
		DefaultProvider: "openai",
		Providers: map[string]ProviderConfig{
			"openai": {
				APIEndpoint: "https://api.openai.com/v1",
				APIKey:      "YOUR_OPENAI_API_KEY", // 佔位符
				Model:       "",                    // 空模型
			},
		},
		UserPreferences: UserPreferences{
			Language: "english",
			Context: ContextConfig{
				MaxHistoryEntries: 10,
			},
			Logging: LoggingConfig{
				Level:      "info",
				Format:     "text",
				Output:     "file",
				LogFile:    "/tmp/test.log",
				MaxSize:    10,
				MaxBackups: 5,
			},
			Cache: CacheConfig{
				Enabled:             true,
				MaxEntries:          1000,
				DefaultTTLHours:     24,
				SuggestionTTLHours:  6,
				CommandTTLHours:     24,
				EnableSimilarity:    true,
				SimilarityThreshold: 0.85,
				MaxSimilarityCache:  500,
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("未配置的 API 密鑰和空模型應該導致驗證失敗")
	}
}

func TestValidateURL(t *testing.T) {
	validator := NewValidator()

	// 測試有效 URL
	err := validator.validateURL("https://api.openai.com/v1")
	if err != nil {
		t.Errorf("有效 URL 不應該導致錯誤: %v", err)
	}

	// 測試無效 URL
	err = validator.validateURL("invalid-url")
	if err == nil {
		t.Error("無效 URL 應該導致錯誤")
	}

	// 測試不支持的協議
	err = validator.validateURL("ftp://example.com")
	if err == nil {
		t.Error("不支持的協議應該導致錯誤")
	}
}

func TestValidateContextConfig(t *testing.T) {
	validator := NewValidator()

	// 測試負數最大歷史條目數
	context := ContextConfig{
		MaxHistoryEntries: -1,
	}

	validator.validateContextConfig("test", context)

	if !validator.HasErrors() {
		t.Error("負數最大歷史條目數應該導致驗證錯誤")
	}
}

func TestValidateLoggingConfig(t *testing.T) {
	validator := NewValidator()

	// 測試無效日誌級別
	logging := LoggingConfig{
		Level:      "invalid",
		Format:     "text",
		Output:     "file",
		LogFile:    "/tmp/test.log",
		MaxSize:    10,
		MaxBackups: 5,
	}

	validator.validateLoggingConfig("test", logging)

	if !validator.HasErrors() {
		t.Error("無效日誌級別應該導致驗證錯誤")
	}
}

func TestValidateCacheConfig(t *testing.T) {
	validator := NewValidator()

	// 測試無效緩存配置
	cache := CacheConfig{
		MaxEntries:          -1,  // 負數
		DefaultTTLHours:     0,   // 零值
		SimilarityThreshold: 1.5, // 超出範圍
	}

	validator.validateCacheConfig("test", cache)

	if !validator.HasErrors() {
		t.Error("無效緩存配置應該導致驗證錯誤")
	}
}

func TestValidateAndFix(t *testing.T) {
	cfg := &Config{
		DefaultProvider: "", // 空的默認提供商
		Providers: map[string]ProviderConfig{
			"openai": {
				APIEndpoint: "https://api.openai.com/v1",
				APIKey:      "test-key",
				Model:       "gpt-4",
			},
		},
		UserPreferences: UserPreferences{
			Language: "english",
			Context: ContextConfig{
				MaxHistoryEntries: 0, // 將被修復為 10
			},
			Logging: LoggingConfig{
				Level:      "info",
				Format:     "text",
				Output:     "file",
				LogFile:    "", // 將被設置
				MaxSize:    0,  // 將被修復為 10
				MaxBackups: -1, // 將被修復為 5
			},
			Cache: CacheConfig{
				Enabled:             true,
				MaxEntries:          1000,
				DefaultTTLHours:     24,
				SuggestionTTLHours:  6,
				CommandTTLHours:     24,
				EnableSimilarity:    true,
				SimilarityThreshold: 0.85,
				MaxSimilarityCache:  500,
			},
		},
	}

	fixes, err := cfg.ValidateAndFix()
	if err != nil {
		t.Errorf("ValidateAndFix 應該成功: %v", err)
	}

	if len(fixes) == 0 {
		t.Error("應該有修復項目")
	}

	// 檢查是否被正確修復
	if cfg.DefaultProvider == "" {
		t.Error("默認提供商應該被設置")
	}

	if cfg.UserPreferences.Context.MaxHistoryEntries != 10 {
		t.Error("最大歷史條目數應該被修復為 10")
	}
}
