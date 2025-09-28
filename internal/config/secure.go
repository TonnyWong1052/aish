package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/TonnyWong1052/aish/internal/crypto"
	aerrors "github.com/TonnyWong1052/aish/internal/errors"
)

// SecureConfig 安全配置管理器
type SecureConfig struct {
	*Config
	secretManager *crypto.SecretManager
}

// NewSecureConfig 創建新的安全配置管理器
func NewSecureConfig(cfg *Config) (*SecureConfig, error) {
	// 獲取配置目錄
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}
	configDir := filepath.Dir(configPath)

	// 創建秘密管理器
	secretManager, err := crypto.NewSecretManager(configDir)
    if err != nil {
        return nil, aerrors.WrapError(err, aerrors.ErrConfigLoad, "創建秘密管理器失敗")
    }

	return &SecureConfig{
		Config:        cfg,
		secretManager: secretManager,
	}, nil
}

// LoadSecure 加載並解密配置
func LoadSecure() (*SecureConfig, error) {
	// 加載基本配置
	cfg, err := Load()
	if err != nil {
		return nil, err
	}

	// 創建安全配置
	secureCfg, err := NewSecureConfig(cfg)
	if err != nil {
		return nil, err
	}

	// 解密敏感信息
	if err := secureCfg.DecryptSensitiveData(); err != nil {
		// 解密失敗不應該導致程序無法啟動
		// 可能是首次運行或密鑰丟失，記錄警告但繼續
		if os.Getenv("AISH_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[WARNING] 解密敏感數據失敗: %v\n", err)
		}
	}

	return secureCfg, nil
}

// Save 保存並加密配置
func (sc *SecureConfig) Save() error {
	// 先加密敏感信息
	if err := sc.EncryptSensitiveData(); err != nil {
		return err
	}

	// 保存配置
	return sc.Config.Save()
}

// EncryptSensitiveData 加密敏感數據
func (sc *SecureConfig) EncryptSensitiveData() error {
	// 加密各個提供商的 API 密鑰
	for providerName, providerConfig := range sc.Providers {
		if providerConfig.APIKey != "" {
			encryptedKey, err := sc.secretManager.EncryptAPIKey(providerConfig.APIKey)
            if err != nil {
                return aerrors.WrapError(err, aerrors.ErrConfigSave,
                    fmt.Sprintf("加密 %s 提供商的 API 密鑰失敗", providerName))
            }
			providerConfig.APIKey = encryptedKey
			sc.Providers[providerName] = providerConfig
		}
	}

	return nil
}

// DecryptSensitiveData 解密敏感數據
func (sc *SecureConfig) DecryptSensitiveData() error {
	// 解密各個提供商的 API 密鑰
	for providerName, providerConfig := range sc.Providers {
		if providerConfig.APIKey != "" {
			decryptedKey, err := sc.secretManager.DecryptAPIKey(providerConfig.APIKey)
			if err != nil {
				// 解密失敗時保持原值，可能是未加密的舊配置
				continue
			}
			providerConfig.APIKey = decryptedKey
			sc.Providers[providerName] = providerConfig
		}
	}

	return nil
}

// GetDecryptedAPIKey 獲取解密後的 API 密鑰
func (sc *SecureConfig) GetDecryptedAPIKey(providerName string) (string, error) {
	providerConfig, exists := sc.Providers[providerName]
	if !exists {
        return "", aerrors.NewError(aerrors.ErrProviderNotFound,
			fmt.Sprintf("提供商 '%s' 不存在", providerName))
	}

	return sc.secretManager.DecryptAPIKey(providerConfig.APIKey)
}

// SetAPIKey 設置並加密 API 密鑰
func (sc *SecureConfig) SetAPIKey(providerName, apiKey string) error {
	providerConfig, exists := sc.Providers[providerName]
	if !exists {
		providerConfig = ProviderConfig{}
	}

	// 加密 API 密鑰
	encryptedKey, err := sc.secretManager.EncryptAPIKey(apiKey)
    if err != nil {
        return aerrors.WrapError(err, aerrors.ErrConfigSave,
			fmt.Sprintf("加密 %s 提供商的 API 密鑰失敗", providerName))
    }

	providerConfig.APIKey = encryptedKey
	sc.Providers[providerName] = providerConfig

	return nil
}

// ValidateEncryption 驗證加密系統
func (sc *SecureConfig) ValidateEncryption() error {
	return sc.secretManager.ValidateEncryption()
}

// CreateSecureBackup 創建安全備份（敏感信息已加密）
func (sc *SecureConfig) CreateSecureBackup() (string, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return "", err
	}

	backupPath := configPath + ".backup"

	// 先加密敏感信息
	if err := sc.EncryptSensitiveData(); err != nil {
		return "", err
	}

	// 創建備份
	data, err := json.MarshalIndent(sc.Config, "", "  ")
    if err != nil {
        return "", aerrors.ErrConfigSaveFailed(backupPath, err)
    }

    if err := os.WriteFile(backupPath, data, 0600); err != nil {
        return "", aerrors.ErrConfigSaveFailed(backupPath, err)
    }

	return backupPath, nil
}

// IsEncrypted 檢查 API 密鑰是否已加密
func (sc *SecureConfig) IsEncrypted(providerName string) bool {
	providerConfig, exists := sc.Providers[providerName]
	if !exists || providerConfig.APIKey == "" {
		return false
	}

	// 嘗試解密，如果成功說明是加密的
	_, err := sc.secretManager.DecryptAPIKey(providerConfig.APIKey)
	return err == nil
}

// MigrateToEncryption 將現有明文配置遷移到加密
func (sc *SecureConfig) MigrateToEncryption() ([]string, error) {
	var migrated []string

	for providerName, providerConfig := range sc.Providers {
		if providerConfig.APIKey != "" && !sc.IsEncrypted(providerName) {
			// 這是明文密鑰，需要加密
			if err := sc.SetAPIKey(providerName, providerConfig.APIKey); err != nil {
				return migrated, err
			}
			migrated = append(migrated, providerName)
		}
	}

	if len(migrated) > 0 {
		// 保存加密後的配置
		if err := sc.Save(); err != nil {
			return migrated, err
		}
	}

	return migrated, nil
}

// GetSecretManager 獲取秘密管理器（用於其他需要加密的組件）
func (sc *SecureConfig) GetSecretManager() *crypto.SecretManager {
	return sc.secretManager
}
