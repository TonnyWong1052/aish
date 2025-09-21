package errors

import (
	"fmt"
	"os"

	"github.com/pterm/pterm"
)

// ErrorHandler 定義錯誤處理器接口
type ErrorHandler interface {
	Handle(err error)
	HandleWithContext(err error, context map[string]interface{})
}

// ConsoleErrorHandler 控制台錯誤處理器
type ConsoleErrorHandler struct {
	debugMode bool
}

// NewConsoleErrorHandler 創建新的控制台錯誤處理器
func NewConsoleErrorHandler(debugMode bool) *ConsoleErrorHandler {
	return &ConsoleErrorHandler{
		debugMode: debugMode,
	}
}

// Handle 處理錯誤並輸出到控制台
func (h *ConsoleErrorHandler) Handle(err error) {
	h.HandleWithContext(err, nil)
}

// HandleWithContext 處理錯誤並包含上下文信息
func (h *ConsoleErrorHandler) HandleWithContext(err error, context map[string]interface{}) {
	if err == nil {
		return
	}

	if aishErr, ok := GetAishError(err); ok {
		h.handleAishError(aishErr, context)
	} else {
		h.handleGenericError(err, context)
	}
}

// handleAishError 處理 AISH 特定錯誤
func (h *ConsoleErrorHandler) handleAishError(aishErr *AishError, context map[string]interface{}) {
	// 如果錯誤不面向用戶且非調試模式，則靜默處理
	if !aishErr.IsUserFacing() && !h.debugMode {
		return
	}

	// 根據錯誤代碼選擇適當的顯示方式
	switch aishErr.Code {
	case ErrUserCancel:
		pterm.Info.Println("操作已取消")
		return
	case ErrProviderAuth:
		pterm.Error.Println("認證失敗")
		pterm.Info.Println("請檢查您的 API 密鑰或運行 'aish configure' 重新配置")
		if h.debugMode && aishErr.Details != "" {
			pterm.Debug.Println("詳細信息:", aishErr.Details)
		}
	case ErrProviderQuota:
		pterm.Error.Println("API 配額已用盡")
		pterm.Info.Println("請檢查您的 API 使用情況或升級您的計劃")
	case ErrConfigMissing:
		pterm.Error.Println("配置文件缺失或不完整")
		pterm.Info.Println("請運行 'aish configure' 進行配置")
	case ErrHookInstall:
		pterm.Error.Println("Shell hook 安裝失敗")
		pterm.Info.Println("請檢查文件權限並重試")
		if h.debugMode {
			pterm.Debug.Println("錯誤詳情:", aishErr.Error())
		}
	default:
		// 通用錯誤處理
		if aishErr.IsUserFacing() {
			pterm.Error.Println(h.formatUserMessage(aishErr))

			// 如果錯誤可重試，提示用戶
			if aishErr.IsRetryable() {
				pterm.Info.Println("此錯誤可能是暫時的，請稍後重試")
			}
		}

		// 調試模式下顯示詳細信息
		if h.debugMode {
			pterm.Debug.Println("錯誤代碼:", aishErr.Code)
			if aishErr.Details != "" {
				pterm.Debug.Println("詳細信息:", aishErr.Details)
			}
			if len(aishErr.Context) > 0 {
				pterm.Debug.Println("上下文:", aishErr.Context)
			}
			if aishErr.Stack != "" {
				pterm.Debug.Println("堆棧:", aishErr.Stack)
			}
		}
	}

	// 顯示額外的上下文信息
	if context != nil && len(context) > 0 && h.debugMode {
		pterm.Debug.Println("額外上下文:", context)
	}
}

// handleGenericError 處理通用錯誤
func (h *ConsoleErrorHandler) handleGenericError(err error, context map[string]interface{}) {
	pterm.Error.Println("發生錯誤:", err.Error())

	if context != nil && len(context) > 0 && h.debugMode {
		pterm.Debug.Println("上下文:", context)
	}
}

// formatUserMessage 格式化面向用戶的錯誤消息
func (h *ConsoleErrorHandler) formatUserMessage(aishErr *AishError) string {
	message := aishErr.Message
	if aishErr.Details != "" && h.debugMode {
		message = fmt.Sprintf("%s (%s)", message, aishErr.Details)
	}
	return message
}

// ExitOnError 遇到錯誤時退出程序
func ExitOnError(err error) {
	if err == nil {
		return
	}

	handler := NewConsoleErrorHandler(os.Getenv("AISH_DEBUG") != "")
	handler.Handle(err)

	// 根據錯誤類型決定退出代碼
	if aishErr, ok := GetAishError(err); ok {
		switch aishErr.Code {
		case ErrUserCancel:
			os.Exit(130) // 用戶取消操作
		case ErrConfigMissing, ErrConfigValidation:
			os.Exit(78) // 配置錯誤
		case ErrPermission:
			os.Exit(77) // 權限錯誤
		case ErrProviderAuth:
			os.Exit(79) // 認證錯誤
		default:
			os.Exit(1) // 通用錯誤
		}
	} else {
		os.Exit(1) // 未知錯誤
	}
}

// LogError 記錄錯誤到日誌系統
func LogError(err error) {
	if err == nil {
		return
	}

	// 延遲導入日誌包以避免循環依賴
	// 這裡我們使用一個簡單的實現，實際使用時會在main中初始化日誌系統
	if os.Getenv("AISH_DEBUG") != "" {
		if aishErr, ok := GetAishError(err); ok {
			fmt.Fprintf(os.Stderr, "[ERROR] Code: %s, Message: %s", aishErr.Code, aishErr.Message)
			if aishErr.Details != "" {
				fmt.Fprintf(os.Stderr, ", Details: %s", aishErr.Details)
			}
			if len(aishErr.Context) > 0 {
				fmt.Fprintf(os.Stderr, ", Context: %+v", aishErr.Context)
			}
			fmt.Fprintf(os.Stderr, "\n")
		} else {
			fmt.Fprintf(os.Stderr, "[ERROR] %s\n", err.Error())
		}
	}
}
