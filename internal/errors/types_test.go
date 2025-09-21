package errors

import (
	"errors"
	"testing"
)

func TestNewError(t *testing.T) {
	err := NewError(ErrConfigLoad, "測試錯誤")

	if err.Code != ErrConfigLoad {
		t.Errorf("期望錯誤代碼 %s，得到 %s", ErrConfigLoad, err.Code)
	}

	if err.Message != "測試錯誤" {
		t.Errorf("期望錯誤信息 '測試錯誤'，得到 '%s'", err.Message)
	}

	if err.IsRetryable() {
		t.Error("新錯誤不應該是可重試的")
	}

	if !err.IsUserFacing() {
		t.Error("新錯誤應該是面向用戶的")
	}
}

func TestNewRetryableError(t *testing.T) {
	err := NewRetryableError(ErrNetwork, "網絡錯誤")

	if !err.IsRetryable() {
		t.Error("可重試錯誤應該是可重試的")
	}

	if err.Code != ErrNetwork {
		t.Errorf("期望錯誤代碼 %s，得到 %s", ErrNetwork, err.Code)
	}
}

func TestNewInternalError(t *testing.T) {
	err := NewInternalError(ErrContextEnhance, "內部錯誤")

	if err.IsUserFacing() {
		t.Error("內部錯誤不應該是面向用戶的")
	}
}

func TestWrapError(t *testing.T) {
	originalErr := errors.New("原始錯誤")
	wrappedErr := WrapError(originalErr, ErrConfigLoad, "包裝錯誤")

	if wrappedErr.Cause != originalErr {
		t.Error("包裝錯誤應該保持原始錯誤")
	}

	if wrappedErr.Unwrap() != originalErr {
		t.Error("Unwrap() 應該返回原始錯誤")
	}
}

func TestWrapErrorNil(t *testing.T) {
	wrappedErr := WrapError(nil, ErrConfigLoad, "包裝空錯誤")

	if wrappedErr != nil {
		t.Error("包裝空錯誤應該返回 nil")
	}
}

func TestWithContext(t *testing.T) {
	err := NewError(ErrConfigLoad, "測試錯誤")
	err.WithContext("key", "value")

	if err.Context["key"] != "value" {
		t.Error("上下文應該被正確設置")
	}
}

func TestErrorString(t *testing.T) {
	err := NewError(ErrConfigLoad, "測試錯誤")
	expected := "CONFIG_LOAD: 測試錯誤"

	if err.Error() != expected {
		t.Errorf("期望錯誤字符串 '%s'，得到 '%s'", expected, err.Error())
	}

	// 測試帶詳情的錯誤
	err.Details = "詳細信息"
	expectedWithDetails := "CONFIG_LOAD: 測試錯誤 (詳細信息)"

	if err.Error() != expectedWithDetails {
		t.Errorf("期望錯誤字符串 '%s'，得到 '%s'", expectedWithDetails, err.Error())
	}
}

func TestIsAishError(t *testing.T) {
	aishErr := NewError(ErrConfigLoad, "AISH 錯誤")
	regularErr := errors.New("普通錯誤")

	if !IsAishError(aishErr) {
		t.Error("應該識別為 AISH 錯誤")
	}

	if IsAishError(regularErr) {
		t.Error("不應該識別為 AISH 錯誤")
	}
}

func TestGetAishError(t *testing.T) {
	aishErr := NewError(ErrConfigLoad, "AISH 錯誤")
	regularErr := errors.New("普通錯誤")

	retrievedErr, ok := GetAishError(aishErr)
	if !ok || retrievedErr != aishErr {
		t.Error("應該能夠獲取 AISH 錯誤")
	}

	_, ok = GetAishError(regularErr)
	if ok {
		t.Error("不應該能夠從普通錯誤獲取 AISH 錯誤")
	}
}

func TestHasCode(t *testing.T) {
	aishErr := NewError(ErrConfigLoad, "AISH 錯誤")
	regularErr := errors.New("普通錯誤")

	if !HasCode(aishErr, ErrConfigLoad) {
		t.Error("應該具有正確的錯誤代碼")
	}

	if HasCode(aishErr, ErrNetwork) {
		t.Error("不應該具有錯誤的錯誤代碼")
	}

	if HasCode(regularErr, ErrConfigLoad) {
		t.Error("普通錯誤不應該具有特定錯誤代碼")
	}
}
