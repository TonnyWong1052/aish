package ui

import (
    "strings"

    "github.com/TonnyWong1052/aish/internal/config"
)

// SettingType defines the UI type for a setting item
type SettingType string

const (
    SettingTypeGroup   SettingType = "group"   // Section header
    SettingTypeBoolean SettingType = "boolean" // Toggle switch
    SettingTypeSelect  SettingType = "select"  // Dropdown menu
    SettingTypeText    SettingType = "text"    // Text input
    SettingTypeAction  SettingType = "action"  // Action button
    SettingTypeInfo    SettingType = "info"    // Read-only information
)

// SettingOption represents an option in a dropdown menu
type SettingOption struct {
	Value       string // Value stored in config
	DisplayName string // Name shown in UI
}

// SettingItem represents a configurable item
type SettingItem struct {
    ID          string          // Unique internal ID
    DisplayName string          // Name shown in UI
    Description string          // Help text
    Type        SettingType     // UI control type
    Options     []SettingOption // Options for select type
    Action      func() error    // Function for action type
    GetValue    func(cfg *config.Config) interface{}
    SetValue    func(cfg *config.Config, value interface{})
}

// GetSettingsDefinition returns all setting items definition
func GetSettingsDefinition(cfg *config.Config) []*SettingItem {
    settings := []*SettingItem{
        // Main header
        {
            DisplayName: "Settings",
            Type:        SettingTypeGroup,
        },

    // AISH Preferences
    {
        DisplayName: "AISH Preferences",
        Type:        SettingTypeGroup,
    },
		{
			ID:          "user_preferences.auto_execute",
			DisplayName: "Auto-execute",
			Description: "自動執行產生的指令（請謹慎啟用）",
			Type:        SettingTypeBoolean,
			GetValue:    func(c *config.Config) interface{} { return c.UserPreferences.AutoExecute },
			SetValue:    func(c *config.Config, v interface{}) { c.UserPreferences.AutoExecute = v.(bool) },
		},
		{
			ID:          "user_preferences.show_tips",
			DisplayName: "Show tips",
			Description: "顯示使用提示",
			Type:        SettingTypeBoolean,
			GetValue:    func(c *config.Config) interface{} { return c.UserPreferences.ShowTips },
			SetValue:    func(c *config.Config, v interface{}) { c.UserPreferences.ShowTips = v.(bool) },
		},
		{
			ID:          "user_preferences.verbose_output",
			DisplayName: "Verbose output",
			Description: "顯示詳細診斷資訊",
			Type:        SettingTypeBoolean,
			GetValue:    func(c *config.Config) interface{} { return c.UserPreferences.VerboseOutput },
			SetValue:    func(c *config.Config, v interface{}) { c.UserPreferences.VerboseOutput = v.(bool) },
		},
		{
			ID:          "user_preferences.language",
			DisplayName: "Language",
			Description: "AI 回應語言",
			Type:        SettingTypeSelect,
            Options: []SettingOption{
                {Value: "english", DisplayName: "English"},
                {Value: "zh-TW", DisplayName: "繁體中文"},
                {Value: "zh-CN", DisplayName: "简体中文"},
                {Value: "ja", DisplayName: "日本語"},
                {Value: "ko", DisplayName: "한국어"},
                {Value: "es", DisplayName: "Español"},
                {Value: "fr", DisplayName: "Français"},
                {Value: "de", DisplayName: "Deutsch"},
            },
			GetValue: func(c *config.Config) interface{} { return c.UserPreferences.Language },
			SetValue: func(c *config.Config, v interface{}) { c.UserPreferences.Language = v.(string) },
		},

    // LLM Providers
    {
        DisplayName: "LLM Providers",
        Type:        SettingTypeGroup,
    },
		{
			ID:          "default_provider",
			DisplayName: "Default provider",
			Description: "選擇預設使用的 LLM 供應商",
			Type:        SettingTypeSelect,
			Options: []SettingOption{
				{Value: config.ProviderOpenAI, DisplayName: "OpenAI"},
				{Value: config.ProviderGemini, DisplayName: "Gemini API"},
				{Value: config.ProviderGeminiCLI, DisplayName: "Gemini CLI"},
			},
			GetValue: func(c *config.Config) interface{} { return c.DefaultProvider },
			SetValue: func(c *config.Config, v interface{}) { c.DefaultProvider = v.(string) },
		},
    // API Host（Gemini CLI 不允許編輯，其餘可編輯）
    func() *SettingItem {
        item := &SettingItem{
            ID:          "provider.api_endpoint",
            DisplayName: "API Host",
            Description: "目前預設供應商的 API 端點",
            Type:        SettingTypeText,
            GetValue: func(c *config.Config) interface{} {
                if p, ok := c.Providers[c.DefaultProvider]; ok {
                    return p.APIEndpoint
                }
                return ""
            },
            SetValue: func(c *config.Config, v interface{}) {
                if p, ok := c.Providers[c.DefaultProvider]; ok {
                    p.APIEndpoint, _ = v.(string)
                    c.Providers[c.DefaultProvider] = p
                }
            },
        }
        // 僅在 gemini-cli 時改為唯讀資訊
        if cfg.DefaultProvider == config.ProviderGeminiCLI {
            item.Type = SettingTypeInfo
            // 保持 GetValue，移除 SetValue 以避免誤觸
            item.SetValue = nil
        }
        return item
    }(),

    // Model（可編輯）
    {
        ID:          "provider.model",
        DisplayName: "Model",
        Description: "目前預設供應商的模型",
        Type:        SettingTypeText,
        GetValue: func(c *config.Config) interface{} {
            if p, ok := c.Providers[c.DefaultProvider]; ok {
                return p.Model
            }
            return ""
        },
        SetValue: func(c *config.Config, v interface{}) {
            if p, ok := c.Providers[c.DefaultProvider]; ok {
                p.Model, _ = v.(string)
                c.Providers[c.DefaultProvider] = p
            }
        },
    },

    // API Key（可編輯，顯示時遮罩）
    {
        ID:          "provider.api_key",
        DisplayName: "API Key",
        Description: "目前預設供應商的 API Key（輸入後將儲存）",
        Type:        SettingTypeText,
        GetValue: func(c *config.Config) interface{} {
            if p, ok := c.Providers[c.DefaultProvider]; ok {
                key := strings.TrimSpace(p.APIKey)
                if key == "" {
                    return ""
                }
                if len(key) <= 4 {
                    return "****"
                }
                return strings.Repeat("*", len(key)-4) + key[len(key)-4:]
            }
            return ""
        },
        SetValue: func(c *config.Config, v interface{}) {
            if p, ok := c.Providers[c.DefaultProvider]; ok {
                p.APIKey, _ = v.(string)
                c.Providers[c.DefaultProvider] = p
            }
        },
    },
		// 移除「啟動完整設定精靈」動作項，避免在設定頁面出現 [Action]

    // Shell Integration
    {
        DisplayName: "Shell Integration",
        Type:        SettingTypeGroup,
    },
    {
        ID:          "enabled",
        DisplayName: "Enable AI error analysis",
        Description: "允許 aish 自動分析 shell 錯誤",
        Type:        SettingTypeBoolean,
        GetValue:    func(c *config.Config) interface{} { return c.Enabled },
        SetValue:    func(c *config.Config, v interface{}) { c.Enabled = v.(bool) },
    },

        // Error Triggers (multi-select via action)
        {
            DisplayName: "Error triggers",
            Type:        SettingTypeGroup,
        },
        func() *SettingItem {
            return &SettingItem{
                ID:          "user_preferences.enabled_llm_triggers",
                DisplayName: "Configure error types (multi-select)",
                Description: "選擇哪些錯誤類型會觸發 AI 分析（空白鍵切換、Enter 確認）",
                Type:        SettingTypeAction,
                Action: func() error {
                    // 與精靈一致的錯誤類型列表
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
                    // 以 MultiSelectNoHelp 呈現
                    selected, err := MultiSelectNoHelp(
                        "Select error types to enable AI analysis (space to toggle, enter to confirm):",
                        errorTypes,
                        cfg.UserPreferences.EnabledLLMTriggers,
                    )
                    if err != nil {
                        return err
                    }
                    cfg.UserPreferences.EnabledLLMTriggers = selected
                    return cfg.Save()
                },
                // 顯示目前選取摘要
                GetValue: func(c *config.Config) interface{} {
                    return strings.Join(c.UserPreferences.EnabledLLMTriggers, ", ")
                },
            }
        }(),
    }

    return settings
}
