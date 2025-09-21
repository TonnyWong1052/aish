package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/TonnyWong1052/aish/internal/errors"
)

// Encryptor encryption interface
type Encryptor interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

// AESEncryptor AES encryption implementation
type AESEncryptor struct {
	key []byte
}

// NewAESEncryptor creates a new AES encryptor
func NewAESEncryptor(passphrase string) *AESEncryptor {
	// Use SHA256 to generate key from passphrase
	key := sha256.Sum256([]byte(passphrase))
	return &AESEncryptor{
		key: key[:],
	}
}

// Encrypt encrypts string
func (e *AESEncryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	// Create AES cipher
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", errors.WrapError(err, errors.ErrCacheError, "Failed to create AES cipher")
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", errors.WrapError(err, errors.ErrCacheError, "創建 GCM 失敗")
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", errors.WrapError(err, errors.ErrCacheError, "生成 nonce 失敗")
	}

	// Encrypt data
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Use base64 encoding
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts string
func (e *AESEncryptor) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	// base64 decode
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", errors.WrapError(err, errors.ErrCacheError, "base64 解碼失敗")
	}

	// Create AES cipher
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", errors.WrapError(err, errors.ErrCacheError, "Failed to create AES cipher")
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", errors.WrapError(err, errors.ErrCacheError, "創建 GCM 失敗")
	}

	// Check data length
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.NewError(errors.ErrCacheError, "加密數據長度不足")
	}

	// Separate nonce and ciphertext
	nonce, cipherData := data[:nonceSize], data[nonceSize:]

	// Decrypt data
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return "", errors.WrapError(err, errors.ErrCacheError, "解密失敗")
	}

	return string(plaintext), nil
}

// SecretManager secret manager
type SecretManager struct {
	encryptor Encryptor
	keyFile   string
}

// ValidateEncryption validates if encryption system works properly
func (sm *SecretManager) ValidateEncryption() error {
	return ValidateEncryption(sm.encryptor)
}

// NewSecretManager creates a new secret manager
func NewSecretManager(configDir string) (*SecretManager, error) {
	keyFile := filepath.Join(configDir, ".secret_key")

	// Try to load existing key
	passphrase, err := loadOrCreatePassphrase(keyFile)
	if err != nil {
		return nil, err
	}

	return &SecretManager{
		encryptor: NewAESEncryptor(passphrase),
		keyFile:   keyFile,
	}, nil
}

// EncryptString encrypts string
func (sm *SecretManager) EncryptString(plaintext string) (string, error) {
	return sm.encryptor.Encrypt(plaintext)
}

// DecryptString decrypts string
func (sm *SecretManager) DecryptString(ciphertext string) (string, error) {
	return sm.encryptor.Decrypt(ciphertext)
}

// EncryptAPIKey encrypts API key
func (sm *SecretManager) EncryptAPIKey(apiKey string) (string, error) {
	if apiKey == "" || isPlaceholderKey(apiKey) {
		return apiKey, nil // Don't encrypt placeholder
	}
	return sm.EncryptString(apiKey)
}

// DecryptAPIKey decrypts API key
func (sm *SecretManager) DecryptAPIKey(encryptedKey string) (string, error) {
	if encryptedKey == "" || isPlaceholderKey(encryptedKey) {
		return encryptedKey, nil // Placeholder doesn't need decryption
	}
	return sm.DecryptString(encryptedKey)
}

// isPlaceholderKey checks if it's a placeholder key
func isPlaceholderKey(key string) bool {
	placeholders := []string{
		"YOUR_OPENAI_API_KEY",
		"YOUR_GEMINI_API_KEY",
		"YOUR_GEMINI_PROJECT_ID",
	}

	for _, placeholder := range placeholders {
		if key == placeholder {
			return true
		}
	}
	return false
}

// loadOrCreatePassphrase loads or creates passphrase
func loadOrCreatePassphrase(keyFile string) (string, error) {
	// Try to read existing key file
	if data, err := os.ReadFile(keyFile); err == nil {
		// Decrypt stored passphrase (using machine fingerprint as key)
		machineKey := getMachineFingerprint()
		encryptor := NewAESEncryptor(machineKey)
		passphrase, err := encryptor.Decrypt(string(data))
		if err == nil {
			return passphrase, nil
		}
	}

	// Generate new passphrase
	passphrase, err := generateRandomPassphrase()
	if err != nil {
		return "", err
	}

	// Save passphrase (encrypted using machine fingerprint)
	if err := savePassphrase(keyFile, passphrase); err != nil {
		return "", err
	}

	return passphrase, nil
}

// generateRandomPassphrase generates random passphrase
func generateRandomPassphrase() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", errors.WrapError(err, errors.ErrCacheError, "生成隨機密碼短語失敗")
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

// savePassphrase saves passphrase
func savePassphrase(keyFile, passphrase string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(keyFile), 0700); err != nil {
		return errors.ErrFileSystemError("create_key_dir", filepath.Dir(keyFile), err)
	}

	// Encrypt passphrase using machine fingerprint
	machineKey := getMachineFingerprint()
	encryptor := NewAESEncryptor(machineKey)
	encrypted, err := encryptor.Encrypt(passphrase)
	if err != nil {
		return err
	}

	// Write to file (set strict permissions)
	if err := os.WriteFile(keyFile, []byte(encrypted), 0600); err != nil {
		return errors.ErrFileSystemError("write_key_file", keyFile, err)
	}

	return nil
}

// getMachineFingerprint gets machine fingerprint
func getMachineFingerprint() string {
	// Create machine-specific fingerprint
	// Use multiple system characteristics to create relatively stable fingerprint

	var components []string

	// Hostname
	if hostname, err := os.Hostname(); err == nil {
		components = append(components, hostname)
	}

	// User home directory
	if home, err := os.UserHomeDir(); err == nil {
		components = append(components, home)
	}

	// Environment variables (choose stable ones)
	if user := os.Getenv("USER"); user != "" {
		components = append(components, user)
	}
	if logname := os.Getenv("LOGNAME"); logname != "" {
		components = append(components, logname)
	}

	// If not enough components, add default values
	if len(components) == 0 {
		components = append(components, "aish-default-machine-key")
	}

	// Combine all components
	fingerprint := ""
	for _, component := range components {
		fingerprint += component + "|"
	}

	// Use SHA256 to generate final fingerprint
	hash := sha256.Sum256([]byte(fingerprint))
	return fmt.Sprintf("%x", hash)
}

// SecureString secure string to avoid plaintext in memory
type SecureString struct {
	encrypted string
	encryptor Encryptor
}

// NewSecureString creates a new secure string
func NewSecureString(plaintext string, encryptor Encryptor) (*SecureString, error) {
	encrypted, err := encryptor.Encrypt(plaintext)
	if err != nil {
		return nil, err
	}

	return &SecureString{
		encrypted: encrypted,
		encryptor: encryptor,
	}, nil
}

// Get gets decrypted string
func (ss *SecureString) Get() (string, error) {
	return ss.encryptor.Decrypt(ss.encrypted)
}

// IsEmpty checks if empty
func (ss *SecureString) IsEmpty() bool {
	return ss.encrypted == ""
}

// Clear clears secure string
func (ss *SecureString) Clear() {
	ss.encrypted = ""
}

// String implements Stringer interface (won't expose plaintext)
func (ss *SecureString) String() string {
	return "[ENCRYPTED]"
}

// ValidateEncryption validates if encryption system works properly
func ValidateEncryption(encryptor Encryptor) error {
	testData := "test-encryption-validation"

	// Encryption test
	encrypted, err := encryptor.Encrypt(testData)
	if err != nil {
		return errors.WrapError(err, errors.ErrCacheError, "加密驗證失敗")
	}

	// Decryption test
	decrypted, err := encryptor.Decrypt(encrypted)
	if err != nil {
		return errors.WrapError(err, errors.ErrCacheError, "解密驗證失敗")
	}

	// Verify result
	if decrypted != testData {
		return errors.NewError(errors.ErrCacheError, "加密驗證失敗：解密結果不匹配")
	}

	return nil
}
