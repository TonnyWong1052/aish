package security

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewSecureConfigManager(t *testing.T) {
	password := "test-password-123"

	scm, err := NewSecureConfigManager(password)
	if err != nil {
		t.Fatalf("Failed to create SecureConfigManager: %v", err)
	}

	if scm == nil {
		t.Error("SecureConfigManager should not be nil")
	}

	if scm.gcm == nil {
		t.Error("GCM cipher should be initialized")
	}

	if scm.sanitizer == nil {
		t.Error("Sanitizer should be initialized")
	}
}

func TestDefaultConfigSecurity(t *testing.T) {
	security := DefaultConfigSecurity()

	if !security.EncryptSensitive {
		t.Error("EncryptSensitive should be true by default")
	}

	if !security.AutoSanitize {
		t.Error("AutoSanitize should be true by default")
	}

	if !security.BackupOnChange {
		t.Error("BackupOnChange should be true by default")
	}

	if security.FilePermissions != 0o600 {
		t.Errorf("FilePermissions should be 0600, got %o", security.FilePermissions)
	}
}

func TestSaveAndLoadSecureConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.json")
	password := "test-password-123"

	scm, err := NewSecureConfigManager(password)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Create test config
	config := &SecureConfig{
		Version:   "1.0",
		Encrypted: false,
		Data: map[string]interface{}{
			"username": "testuser",
			"setting":  "value",
		},
	}

	// Save config
	err = scm.SaveSecureConfig(config, configPath, DefaultConfigSecurity())
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Check file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file should exist")
	}

	// Load config
	loadedConfig, err := scm.LoadSecureConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loadedConfig.Version != config.Version {
		t.Errorf("Expected version %s, got %s", config.Version, loadedConfig.Version)
	}

	if loadedConfig.Data["username"] != "testuser" {
		t.Error("Username should match")
	}
}

func TestSaveSecureConfig_WithEncryption(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "encrypted_config.json")
	password := "secure-password-456"

	scm, err := NewSecureConfigManager(password)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	config := &SecureConfig{
		Version:   "1.0",
		Encrypted: false,
		Data: map[string]interface{}{
			"api_key": "secret-key-123",
		},
	}

	security := DefaultConfigSecurity()
	security.EncryptSensitive = true

	err = scm.SaveSecureConfig(config, configPath, security)
	if err != nil {
		t.Fatalf("Failed to save encrypted config: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Encrypted config file should exist")
	}
}

func TestLoadSecureConfig_FileNotFound(t *testing.T) {
	scm, err := NewSecureConfigManager("password")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_, err = scm.LoadSecureConfig("/nonexistent/path/config.json")
	if err == nil {
		t.Error("Should return error for nonexistent file")
	}
}

func TestLoadSecureConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	err := os.WriteFile(configPath, []byte("invalid json content"), 0o600)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	scm, err := NewSecureConfigManager("password")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_, err = scm.LoadSecureConfig(configPath)
	if err == nil {
		t.Error("Should return error for invalid JSON")
	}
}

func TestSecureConfig_JSONMarshaling(t *testing.T) {
	config := &SecureConfig{
		Version:   "1.0",
		Encrypted: true,
		Data: map[string]interface{}{
			"key": "value",
		},
		Checksum: "abc123",
	}

	// Marshal to JSON
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	// Unmarshal back
	var loaded SecureConfig
	err = json.Unmarshal(data, &loaded)
	if err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	if loaded.Version != config.Version {
		t.Error("Version mismatch after marshal/unmarshal")
	}

	if loaded.Encrypted != config.Encrypted {
		t.Error("Encrypted flag mismatch after marshal/unmarshal")
	}

	if loaded.Checksum != config.Checksum {
		t.Error("Checksum mismatch after marshal/unmarshal")
	}
}

func TestSaveSecureConfig_WithBackup(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	password := "test-password"

	scm, err := NewSecureConfigManager(password)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Create initial config
	config1 := &SecureConfig{
		Version: "1.0",
		Data: map[string]interface{}{
			"value": "first",
		},
	}

	security := DefaultConfigSecurity()
	security.BackupOnChange = true

	// Save first version
	err = scm.SaveSecureConfig(config1, configPath, security)
	if err != nil {
		t.Fatalf("Failed to save first config: %v", err)
	}

	// Update config
	config2 := &SecureConfig{
		Version: "2.0",
		Data: map[string]interface{}{
			"value": "second",
		},
	}

	// Save second version (should create backup)
	err = scm.SaveSecureConfig(config2, configPath, security)
	if err != nil {
		t.Fatalf("Failed to save second config: %v", err)
	}

	// Verify main config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Main config should exist")
	}
}

func TestConfigSecurity_CustomSettings(t *testing.T) {
	security := &ConfigSecurity{
		EncryptSensitive: false,
		RequireAuth:      true,
		AutoSanitize:     false,
		BackupOnChange:   false,
		FilePermissions:  0o644,
	}

	if security.EncryptSensitive {
		t.Error("EncryptSensitive should be false")
	}

	if !security.RequireAuth {
		t.Error("RequireAuth should be true")
	}

	if security.AutoSanitize {
		t.Error("AutoSanitize should be false")
	}

	if security.BackupOnChange {
		t.Error("BackupOnChange should be false")
	}

	if security.FilePermissions != 0o644 {
		t.Errorf("Expected permissions 0644, got %o", security.FilePermissions)
	}
}

func TestSaveSecureConfig_NoBackup(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "no_backup_config.json")
	password := "test-password"

	scm, err := NewSecureConfigManager(password)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	config := &SecureConfig{
		Version: "1.0",
		Data: map[string]interface{}{
			"setting": "value",
		},
	}

	security := DefaultConfigSecurity()
	security.BackupOnChange = false

	err = scm.SaveSecureConfig(config, configPath, security)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file should exist")
	}
}

func TestSaveSecureConfig_NoSanitize(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "no_sanitize_config.json")
	password := "test-password"

	scm, err := NewSecureConfigManager(password)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	config := &SecureConfig{
		Version: "1.0",
		Data: map[string]interface{}{
			"api_key": "apikey=secret-value",
		},
	}

	security := DefaultConfigSecurity()
	security.AutoSanitize = false
	security.EncryptSensitive = false // Disable encryption for simpler test

	err = scm.SaveSecureConfig(config, configPath, security)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file should exist")
	}

	// Read raw file to verify data was saved
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	// Verify the file contains valid JSON
	var rawConfig map[string]interface{}
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		t.Error("Config file should contain valid JSON")
	}
}

func TestNewSecureConfigManager_EmptyPassword(t *testing.T) {
	scm, err := NewSecureConfigManager("")
	if err != nil {
		t.Fatalf("Should handle empty password: %v", err)
	}

	if scm == nil {
		t.Error("Should create manager even with empty password")
	}
}
