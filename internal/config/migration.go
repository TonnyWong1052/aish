package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	aerrors "github.com/TonnyWong1052/aish/internal/errors"
)

// ConfigVersion configuration version constants
const (
	ConfigVersion1 = "1.0.0" // Original version
	ConfigVersion2 = "1.1.0" // Added logging configuration
	CurrentVersion = ConfigVersion2
)

// VersionedConfig versioned configuration structure
type VersionedConfig struct {
	Version string          `json:"version"`
	Data    json.RawMessage `json:"data"`
}

// LegacyConfig legacy configuration structure (v1.0.0)
type LegacyConfig struct {
	Enabled         bool                      `json:"enabled"`
	DefaultProvider string                    `json:"default_provider"`
	Providers       map[string]ProviderConfig `json:"providers"`
	UserPreferences LegacyUserPreferences     `json:"user_preferences"`
}

// LegacyUserPreferences legacy user preferences
type LegacyUserPreferences struct {
	Language           string        `json:"language"`
	EnabledLLMTriggers []string      `json:"enabled_llm_triggers"`
	Context            ContextConfig `json:"context"`
}

// MigrationResult migration result
type MigrationResult struct {
	FromVersion string   `json:"from_version"`
	ToVersion   string   `json:"to_version"`
	Changes     []string `json:"changes"`
	BackupPath  string   `json:"backup_path,omitempty"`
}

// Migrator configuration migrator
type Migrator struct {
	configPath string
}

// NewMigrator creates a new configuration migrator
func NewMigrator(configPath string) *Migrator {
	return &Migrator{
		configPath: configPath,
	}
}

// LoadWithMigration loads configuration and performs migration if needed
func LoadWithMigration() (*Config, *MigrationResult, error) {
	path, err := GetConfigPath()
    if err != nil {
        return nil, nil, aerrors.ErrConfigLoadFailed(path, err)
    }

	migrator := NewMigrator(path)
	return migrator.LoadAndMigrate()
}

// LoadAndMigrate loads configuration and performs necessary migration
func (m *Migrator) LoadAndMigrate() (*Config, *MigrationResult, error) {
	// Check if configuration file exists
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		// Configuration file doesn't exist, create new default configuration
		cfg := newDefaultConfig()
		cfg.UserPreferences.Logging.LogFile = m.getDefaultLogPath()

		// Set version information (although new config doesn't need migration, for consistency)
		if err := m.saveVersionedConfig(cfg, CurrentVersion); err != nil {
			return nil, nil, err
		}

		return cfg, nil, nil
	}

	// 讀取現有配置
	data, err := os.ReadFile(m.configPath)
    if err != nil {
        return nil, nil, aerrors.ErrConfigLoadFailed(m.configPath, err)
    }

	// 嘗試解析帶版本的配置
	var versionedConfig VersionedConfig
	if err := json.Unmarshal(data, &versionedConfig); err != nil {
		// 可能是舊格式，嘗試解析為舊版本配置
		return m.migrateFromLegacy(data)
	}

	// 檢查是否需要遷移
	if versionedConfig.Version == CurrentVersion {
		// 配置已是最新版本，直接加載
		var cfg Config
        if err := json.Unmarshal(versionedConfig.Data, &cfg); err != nil {
            return nil, nil, aerrors.ErrConfigLoadFailed(m.configPath, err)
        }
		return &cfg, nil, nil
	}

	// 執行遷移
	return m.migrate(versionedConfig)
}

// migrateFromLegacy 從舊版本格式遷移
func (m *Migrator) migrateFromLegacy(data []byte) (*Config, *MigrationResult, error) {
	// 先嘗試解析為舊格式
	var legacyCfg LegacyConfig
	if err := json.Unmarshal(data, &legacyCfg); err != nil {
		// 如果仍然失敗，可能是新格式但沒有版本信息
		var cfg Config
        if err := json.Unmarshal(data, &cfg); err != nil {
            return nil, nil, aerrors.ErrConfigLoadFailed(m.configPath, err)
        }
		// 添加缺失的版本信息並保存
		cfg.UserPreferences.Logging = LoggingConfig{
			Level:      "info",
			Format:     "text",
			Output:     "file",
			LogFile:    m.getDefaultLogPath(),
			MaxSize:    10,
			MaxBackups: 5,
		}
		if err := m.saveVersionedConfig(&cfg, CurrentVersion); err != nil {
			return nil, nil, err
		}
		return &cfg, &MigrationResult{
			FromVersion: "unknown",
			ToVersion:   CurrentVersion,
			Changes:     []string{"添加版本信息", "添加日誌配置"},
		}, nil
	}

	// 創建備份
	backupPath, err := m.createBackup()
	if err != nil {
		return nil, nil, err
	}

	// 遷移到新格式
	newCfg := &Config{
		Enabled:         legacyCfg.Enabled,
		DefaultProvider: legacyCfg.DefaultProvider,
		Providers:       legacyCfg.Providers,
		UserPreferences: UserPreferences{
			Language:           legacyCfg.UserPreferences.Language,
			EnabledLLMTriggers: legacyCfg.UserPreferences.EnabledLLMTriggers,
			Context:            legacyCfg.UserPreferences.Context,
			Logging: LoggingConfig{
				Level:      "info",
				Format:     "text",
				Output:     "file",
				LogFile:    m.getDefaultLogPath(),
				MaxSize:    10,
				MaxBackups: 5,
			},
		},
	}

	// 保存遷移後的配置
	if err := m.saveVersionedConfig(newCfg, CurrentVersion); err != nil {
		return nil, nil, err
	}

	result := &MigrationResult{
		FromVersion: ConfigVersion1,
		ToVersion:   CurrentVersion,
		Changes:     []string{"添加日誌配置", "更新配置格式版本"},
		BackupPath:  backupPath,
	}

	return newCfg, result, nil
}

// migrate 執行版本間的遷移
func (m *Migrator) migrate(versionedConfig VersionedConfig) (*Config, *MigrationResult, error) {
	fromVersion := versionedConfig.Version
	changes := []string{}

	// 創建備份
	backupPath, err := m.createBackup()
	if err != nil {
		return nil, nil, err
	}

	// 解析當前配置
	var cfg Config
    if err := json.Unmarshal(versionedConfig.Data, &cfg); err != nil {
        return nil, nil, aerrors.ErrConfigLoadFailed(m.configPath, err)
    }

	// 根據版本執行相應的遷移邏輯
	switch fromVersion {
	case ConfigVersion1:
		// 從 1.0.0 遷移到 1.1.0
		if cfg.UserPreferences.Logging.Level == "" {
			cfg.UserPreferences.Logging = LoggingConfig{
				Level:      "info",
				Format:     "text",
				Output:     "file",
				LogFile:    m.getDefaultLogPath(),
				MaxSize:    10,
				MaxBackups: 5,
			}
			changes = append(changes, "添加日誌配置")
		}
		fallthrough // 繼續到下一個版本的遷移

	case ConfigVersion2:
		// 未來版本的遷移邏輯
		// 目前 ConfigVersion2 就是最新版本，無需額外遷移

    default:
        return nil, nil, aerrors.NewError(aerrors.ErrConfigValidation, fmt.Sprintf("不支持的配置版本: %s", fromVersion))
	}

	// 保存遷移後的配置
	if err := m.saveVersionedConfig(&cfg, CurrentVersion); err != nil {
		return nil, nil, err
	}

	result := &MigrationResult{
		FromVersion: fromVersion,
		ToVersion:   CurrentVersion,
		Changes:     changes,
		BackupPath:  backupPath,
	}

	return &cfg, result, nil
}

// saveVersionedConfig 保存帶版本的配置
func (m *Migrator) saveVersionedConfig(cfg *Config, version string) error {
	// 確保配置目錄存在
    if err := os.MkdirAll(filepath.Dir(m.configPath), 0755); err != nil {
        return aerrors.ErrConfigSaveFailed(m.configPath, err)
    }

	// 序列化配置數據
	configData, err := json.MarshalIndent(cfg, "", "  ")
    if err != nil {
        return aerrors.ErrConfigSaveFailed(m.configPath, err)
    }

	// 創建帶版本的配置結構
	versionedConfig := VersionedConfig{
		Version: version,
		Data:    configData,
	}

	// 序列化完整的配置
	data, err := json.MarshalIndent(versionedConfig, "", "  ")
    if err != nil {
        return aerrors.ErrConfigSaveFailed(m.configPath, err)
    }

	// 寫入文件
	return os.WriteFile(m.configPath, data, 0644)
}

// createBackup 創建配置文件備份
func (m *Migrator) createBackup() (string, error) {
    if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
        return "", nil // 文件不存在，無需備份
    }

	// 生成備份文件名
	dir := filepath.Dir(m.configPath)
	name := filepath.Base(m.configPath)
	ext := filepath.Ext(name)
	nameWithoutExt := name[:len(name)-len(ext)]

	backupPath := filepath.Join(dir, fmt.Sprintf("%s.backup%s", nameWithoutExt, ext))

	// 如果備份文件已存在，添加時間戳
	if _, err := os.Stat(backupPath); err == nil {
		backupPath = filepath.Join(dir, fmt.Sprintf("%s.backup.%d%s", nameWithoutExt,
			os.Getpid(), ext))
	}

	// 複製文件
    data, err := os.ReadFile(m.configPath)
    if err != nil {
        return "", aerrors.ErrFileSystemError("backup_read", m.configPath, err)
    }

    if err := os.WriteFile(backupPath, data, 0644); err != nil {
        return "", aerrors.ErrFileSystemError("backup_write", backupPath, err)
    }

	return backupPath, nil
}

// getDefaultLogPath 獲取默認日誌文件路徑
func (m *Migrator) getDefaultLogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "aish", "logs", "aish.log")
}

// CheckConfigVersion 檢查配置文件版本
func CheckConfigVersion(configPath string) (string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	var versionedConfig VersionedConfig
	if err := json.Unmarshal(data, &versionedConfig); err != nil {
		// 可能是舊格式，返回舊版本標識
		return ConfigVersion1, nil
	}

	return versionedConfig.Version, nil
}
