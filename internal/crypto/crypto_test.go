package crypto

import (
	"testing"
)

func TestAESEncryptor(t *testing.T) {
	passphrase := "test-passphrase-for-testing"
	encryptor := NewAESEncryptor(passphrase)

	// 測試基本加密和解密
	plaintext := "這是一個測試字符串"

	ciphertext, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("加密失敗: %v", err)
	}

	if ciphertext == plaintext {
		t.Error("密文不應該與明文相同")
	}

	decrypted, err := encryptor.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("解密失敗: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("期望解密結果 '%s'，得到 '%s'", plaintext, decrypted)
	}
}

func TestAESEncryptorEmptyString(t *testing.T) {
	passphrase := "test-passphrase"
	encryptor := NewAESEncryptor(passphrase)

	// 測試空字符串
	ciphertext, err := encryptor.Encrypt("")
	if err != nil {
		t.Fatalf("加密空字符串失敗: %v", err)
	}

	if ciphertext != "" {
		t.Error("加密空字符串應該返回空字符串")
	}

	decrypted, err := encryptor.Decrypt("")
	if err != nil {
		t.Fatalf("解密空字符串失敗: %v", err)
	}

	if decrypted != "" {
		t.Error("解密空字符串應該返回空字符串")
	}
}

func TestAESEncryptorDifferentKeys(t *testing.T) {
	encryptor1 := NewAESEncryptor("passphrase1")
	encryptor2 := NewAESEncryptor("passphrase2")

	plaintext := "測試數據"

	// 使用第一個加密器加密
	ciphertext, err := encryptor1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("加密失敗: %v", err)
	}

	// 使用第二個加密器嘗試解密（應該失敗）
	_, err = encryptor2.Decrypt(ciphertext)
	if err == nil {
		t.Error("使用不同密鑰解密應該失敗")
	}
}

func TestNewSecretManager(t *testing.T) {
	tempDir := t.TempDir()

	sm, err := NewSecretManager(tempDir)
	if err != nil {
		t.Fatalf("創建秘密管理器失敗: %v", err)
	}

	if sm == nil {
		t.Error("秘密管理器不應該為空")
	}

	// 測試驗證加密
	err = sm.ValidateEncryption()
	if err != nil {
		t.Errorf("驗證加密失敗: %v", err)
	}
}

func TestSecretManagerAPIKey(t *testing.T) {
	tempDir := t.TempDir()

	sm, err := NewSecretManager(tempDir)
	if err != nil {
		t.Fatalf("創建秘密管理器失敗: %v", err)
	}

	// 測試佔位符密鑰（不應該被加密）
	placeholder := "YOUR_OPENAI_API_KEY"
	encrypted, err := sm.EncryptAPIKey(placeholder)
	if err != nil {
		t.Fatalf("加密佔位符密鑰失敗: %v", err)
	}

	if encrypted != placeholder {
		t.Error("佔位符密鑰不應該被加密")
	}

	// 測試真實密鑰
	realKey := "sk-1234567890abcdef"
	encrypted, err = sm.EncryptAPIKey(realKey)
	if err != nil {
		t.Fatalf("加密真實密鑰失敗: %v", err)
	}

	if encrypted == realKey {
		t.Error("真實密鑰應該被加密")
	}

	// 測試解密
	decrypted, err := sm.DecryptAPIKey(encrypted)
	if err != nil {
		t.Fatalf("解密密鑰失敗: %v", err)
	}

	if decrypted != realKey {
		t.Errorf("期望解密結果 '%s'，得到 '%s'", realKey, decrypted)
	}
}

func TestIsPlaceholderKey(t *testing.T) {
	placeholders := []string{
		"YOUR_OPENAI_API_KEY",
		"YOUR_GEMINI_API_KEY",
		"YOUR_GEMINI_PROJECT_ID",
	}

	for _, placeholder := range placeholders {
		if !isPlaceholderKey(placeholder) {
			t.Errorf("'%s' 應該被識別為佔位符", placeholder)
		}
	}

	realKeys := []string{
		"sk-1234567890abcdef",
		"AIzaSyDOCAbC123DEF456",
		"my-project-id",
	}

	for _, realKey := range realKeys {
		if isPlaceholderKey(realKey) {
			t.Errorf("'%s' 不應該被識別為佔位符", realKey)
		}
	}
}

func TestSecureString(t *testing.T) {
	passphrase := "test-passphrase"
	encryptor := NewAESEncryptor(passphrase)

	plaintext := "這是機密信息"

	secureStr, err := NewSecureString(plaintext, encryptor)
	if err != nil {
		t.Fatalf("創建安全字符串失敗: %v", err)
	}

	// 測試 String() 方法不暴露明文
	if secureStr.String() == plaintext {
		t.Error("String() 方法不應該返回明文")
	}

	// 測試獲取明文
	retrieved, err := secureStr.Get()
	if err != nil {
		t.Fatalf("獲取明文失敗: %v", err)
	}

	if retrieved != plaintext {
		t.Errorf("期望 '%s'，得到 '%s'", plaintext, retrieved)
	}

	// 測試清除
	secureStr.Clear()
	if !secureStr.IsEmpty() {
		t.Error("清除後應該為空")
	}
}

func TestValidateEncryption(t *testing.T) {
	encryptor := NewAESEncryptor("test-passphrase")

	err := ValidateEncryption(encryptor)
	if err != nil {
		t.Errorf("驗證加密應該成功: %v", err)
	}
}

func TestGenerateRandomPassphrase(t *testing.T) {
	passphrase1, err := generateRandomPassphrase()
	if err != nil {
		t.Fatalf("生成隨機密碼短語失敗: %v", err)
	}

	passphrase2, err := generateRandomPassphrase()
	if err != nil {
		t.Fatalf("生成隨機密碼短語失敗: %v", err)
	}

	if passphrase1 == passphrase2 {
		t.Error("兩次生成的密碼短語不應該相同")
	}

	if len(passphrase1) == 0 {
		t.Error("生成的密碼短語不應該為空")
	}
}

func TestGetMachineFingerprint(t *testing.T) {
	fingerprint1 := getMachineFingerprint()
	fingerprint2 := getMachineFingerprint()

	if fingerprint1 != fingerprint2 {
		t.Error("同一台機器的指紋應該相同")
	}

	if len(fingerprint1) == 0 {
		t.Error("機器指紋不應該為空")
	}
}

func TestSecretManagerPersistence(t *testing.T) {
	tempDir := t.TempDir()

	// 創建第一個秘密管理器
	sm1, err := NewSecretManager(tempDir)
	if err != nil {
		t.Fatalf("創建第一個秘密管理器失敗: %v", err)
	}

	testData := "持久化測試數據"
	encrypted1, err := sm1.EncryptString(testData)
	if err != nil {
		t.Fatalf("加密失敗: %v", err)
	}

	// 創建第二個秘密管理器（應該使用相同的密鑰）
	sm2, err := NewSecretManager(tempDir)
	if err != nil {
		t.Fatalf("創建第二個秘密管理器失敗: %v", err)
	}

	// 應該能夠解密第一個管理器加密的數據
	decrypted, err := sm2.DecryptString(encrypted1)
	if err != nil {
		t.Fatalf("解密失敗: %v", err)
	}

	if decrypted != testData {
		t.Errorf("期望 '%s'，得到 '%s'", testData, decrypted)
	}
}
