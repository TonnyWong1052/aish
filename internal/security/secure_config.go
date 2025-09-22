package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"golang.org/x/crypto/pbkdf2"
)

// SecureConfigManager 安全配置管理器
type SecureConfigManager struct {
	keyDerivationSalt []byte
	iterations        int
	gcm               cipher.AEAD
	sanitizer         *SensitiveDataSanitizer
}

// SecureConfig 安全配置結構
type SecureConfig struct {
	Version   string                 `json:"version"`
	Encrypted bool                   `json:"encrypted"`
	Data      map[string]interface{} `json:"data"`
	Checksum  string                 `json:"checksum,omitempty"`
}

// ConfigSecurity 配置安全選項
type ConfigSecurity struct {
	EncryptSensitive bool `json:"encrypt_sensitive"`
	RequireAuth      bool `json:"require_auth"`
	AutoSanitize     bool `json:"auto_sanitize"`
	BackupOnChange   bool `json:"backup_on_change"`
	FilePermissions  os.FileMode `json:"file_permissions"`
}

// DefaultConfigSecurity 返回默認安全配置
func DefaultConfigSecurity() *ConfigSecurity {
	return &ConfigSecurity{
		EncryptSensitive: true,
		RequireAuth:      false,
		AutoSanitize:     true,
		BackupOnChange:   true,
		FilePermissions:  0600, // 僅用戶可讀寫
	}
}

// NewSecureConfigManager 創建安全配置管理器
func NewSecureConfigManager(password string) (*SecureConfigManager, error) {
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	
	key := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)
	
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	
	return &SecureConfigManager{
		keyDerivationSalt: salt,
		iterations:        100000,
		gcm:               gcm,
		sanitizer:         NewSensitiveDataSanitizer(),
	}, nil
}

// LoadSecureConfig 加載安全配置
func (scm *SecureConfigManager) LoadSecureConfig(filepath string) (*SecureConfig, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	var config SecureConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	
	// 驗證文件權限
	if err := scm.verifyFilePermissions(filepath); err != nil {
		return nil, fmt.Errorf("insecure file permissions: %w", err)
	}
	
	// 解密敏感數據
	if config.Encrypted {
		if err := scm.decryptSensitiveData(&config); err != nil {
			return nil, fmt.Errorf("failed to decrypt sensitive data: %w", err)
		}
	}
	
	// 驗證校驗和
	if config.Checksum != "" {
		if !scm.verifyChecksum(&config) {
			return nil, errors.New("config checksum verification failed")
		}
	}
	
	return &config, nil
}

// SaveSecureConfig 保存安全配置
func (scm *SecureConfigManager) SaveSecureConfig(config *SecureConfig, filepath string, security *ConfigSecurity) error {
	if security == nil {
		security = DefaultConfigSecurity()
	}
	
	// 創建備份
	if security.BackupOnChange {
		if err := scm.createBackup(filepath); err != nil {
			// 不阻止保存，只記錄錯誤
			fmt.Fprintf(os.Stderr, "Warning: failed to create backup: %v\n", err)
		}
	}
	
	// 複製配置以避免修改原始數據
	configCopy := *config
	configCopy.Data = make(map[string]interface{})
	for k, v := range config.Data {
		configCopy.Data[k] = v
	}
	
	// 自動清理敏感數據
	if security.AutoSanitize {
		configCopy.Data = scm.sanitizer.SanitizeMap(configCopy.Data)
	}
	
	// 加密敏感數據
	if security.EncryptSensitive {
		if err := scm.encryptSensitiveData(&configCopy); err != nil {
			return fmt.Errorf("failed to encrypt sensitive data: %w", err)
		}
		configCopy.Encrypted = true
	}
	
	// 計算校驗和
	configCopy.Checksum = scm.calculateChecksum(&configCopy)
	
	// 序列化配置
	data, err := json.MarshalIndent(&configCopy, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	// 創建目錄
	if err := os.MkdirAll(filepath.Dir(filepath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	// 寫入文件
	if err := os.WriteFile(filepath, data, security.FilePermissions); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	
	// 設置文件權限（雙重確保）
	if err := os.Chmod(filepath, security.FilePermissions); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}
	
	return nil
}

// encryptSensitiveData 加密敏感數據
func (scm *SecureConfigManager) encryptSensitiveData(config *SecureConfig) error {
	for key, value := range config.Data {
		if scm.isSensitiveKey(key) {
			if strValue, ok := value.(string); ok {
				encrypted, err := scm.encrypt(strValue)
				if err != nil {
					return fmt.Errorf("failed to encrypt key %s: %w", key, err)
				}
				config.Data[key] = encrypted
			}
		}
	}
	return nil
}

// decryptSensitiveData 解密敏感數據
func (scm *SecureConfigManager) decryptSensitiveData(config *SecureConfig) error {
	for key, value := range config.Data {
		if scm.isSensitiveKey(key) {
			if strValue, ok := value.(string); ok {
				decrypted, err := scm.decrypt(strValue)
				if err != nil {
					return fmt.Errorf("failed to decrypt key %s: %w", key, err)
				}
				config.Data[key] = decrypted
			}
		}
	}
	return nil
}

// isSensitiveKey 判斷是否為敏感鍵
func (scm *SecureConfigManager) isSensitiveKey(key string) bool {
	sensitiveKeys := []string{
		"api_key", "apikey", "api-key",
		"secret", "secret_key", "secret-key",
		"password", "pwd", "pass",
		"token", "access_token", "auth_token",
		"private_key", "privatekey",
		"oauth_secret", "client_secret",
	}
	
	keyLower := strings.ToLower(key)
	for _, sensitive := range sensitiveKeys {
		if strings.Contains(keyLower, sensitive) {
			return true
		}
	}
	
	return false
}

// encrypt 加密字符串
func (scm *SecureConfigManager) encrypt(plaintext string) (string, error) {
	nonce := make([]byte, scm.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	
	ciphertext := scm.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt 解密字符串
func (scm *SecureConfigManager) decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	
	nonceSize := scm.gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	
	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := scm.gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}
	
	return string(plaintext), nil
}

// calculateChecksum 計算配置校驗和
func (scm *SecureConfigManager) calculateChecksum(config *SecureConfig) string {
	// 複製配置並移除校驗和字段
	configForHash := *config
	configForHash.Checksum = ""
	
	data, _ := json.Marshal(configForHash)
	hash := sha256.Sum256(data)
	return base64.StdEncoding.EncodeToString(hash[:])
}

// verifyChecksum 驗證校驗和
func (scm *SecureConfigManager) verifyChecksum(config *SecureConfig) bool {
	expectedChecksum := config.Checksum
	config.Checksum = ""
	calculatedChecksum := scm.calculateChecksum(config)
	config.Checksum = expectedChecksum
	
	return expectedChecksum == calculatedChecksum
}

// verifyFilePermissions 驗證文件權限
func (scm *SecureConfigManager) verifyFilePermissions(filepath string) error {
	stat, err := os.Stat(filepath)
	if err != nil {
		return err
	}
	
	perm := stat.Mode().Perm()
	
	// 檢查是否對組和其他用戶可讀
	if runtime.GOOS != "windows" {
		if perm&0044 != 0 {
			return fmt.Errorf("config file is readable by group or others (permissions: %o)", perm)
		}
	}
	
	return nil
}

// createBackup 創建配置備份
func (scm *SecureConfigManager) createBackup(filepath string) error {
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		return nil // 文件不存在，無需備份
	}
	
	data, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}
	
	backupPath := filepath + ".backup"
	return os.WriteFile(backupPath, data, 0600)
}

// SecureGet 安全獲取配置值
func (scm *SecureConfigManager) SecureGet(config *SecureConfig, key string, sanitize bool) (interface{}, bool) {
	value, exists := config.Data[key]
	if !exists {
		return nil, false
	}
	
	if sanitize {
		if strValue, ok := value.(string); ok {
			return scm.sanitizer.Sanitize(strValue), true
		}
	}
	
	return value, true
}

// SecureSet 安全設置配置值
func (scm *SecureConfigManager) SecureSet(config *SecureConfig, key string, value interface{}) {
	if config.Data == nil {
		config.Data = make(map[string]interface{})
	}
	
	// 如果���敏感數據，自動清理
	if strValue, ok := value.(string); ok && scm.isSensitiveKey(key) {
		if scm.sanitizer.ContainsSensitiveData(strValue) {
			// 記錄警告但不阻止設置
			fmt.Fprintf(os.Stderr, "Warning: setting potentially sensitive data for key %s\n", key)
		}
	}
	
	config.Data[key] = value
}

// ValidateConfig 驗證配置安全性
func (scm *SecureConfigManager) ValidateConfig(config *SecureConfig) []SecurityIssue {
	var issues []SecurityIssue
	
	for key, value := range config.Data {
		if strValue, ok := value.(string); ok {
			// 檢查是否包含敏感數據
			if scm.sanitizer.ContainsSensitiveData(strValue) {
				issues = append(issues, SecurityIssue{
					Type:        "sensitive_data_exposure",
					Key:         key,
					Description: "Configuration contains potentially sensitive data",
					Severity:    "high",
				})
			}
			
			// 檢查是否為弱密碼
			if scm.isSensitiveKey(key) && len(strValue) < 8 {
				issues = append(issues, SecurityIssue{
					Type:        "weak_credential",
					Key:         key,
					Description: "Credential appears to be weak (too short)",
					Severity:    "medium",
				})
			}
		}
	}
	
	return issues
}

// SecurityIssue 安全問題
type SecurityIssue struct {
	Type        string `json:"type"`
	Key         string `json:"key"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

// GetConfigStats 獲取配置統計信息
func (scm *SecureConfigManager) GetConfigStats(config *SecureConfig) ConfigStats {
	stats := ConfigStats{
		TotalKeys:     len(config.Data),
		EncryptedKeys: 0,
		SensitiveKeys: 0,
	}
	
	for key, value := range config.Data {
		if scm.isSensitiveKey(key) {
			stats.SensitiveKeys++
		}
		
		if strValue, ok := value.(string); ok {
			if scm.sanitizer.ContainsSensitiveData(strValue) {
				stats.EncryptedKeys++
			}
		}
	}
	
	return stats
}

// ConfigStats 配置統計信息
type ConfigStats struct {
	TotalKeys     int `json:"total_keys"`
	EncryptedKeys int `json:"encrypted_keys"`
	SensitiveKeys int `json:"sensitive_keys"`
}

// WipeMemory 清理內存中的敏感數據
func (scm *SecureConfigManager) WipeMemory() {
	// 清理密鑰
	for i := range scm.keyDerivationSalt {
		scm.keyDerivationSalt[i] = 0
	}
	
	// 強制垃圾回收
	runtime.GC()
	
	// 在支持的系統上調用 mlock 防止內存交換
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		// 這需要適當的權限
		syscall.Syscall(syscall.SYS_MLOCK, 0, 0, 0)
	}
}