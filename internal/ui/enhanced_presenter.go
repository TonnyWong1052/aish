package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/pterm/pterm"
)

// EnhancedPresenter 增強型用戶界面展示器
type EnhancedPresenter struct {
	config      *PresenterConfig
	spinner     *pterm.SpinnerPrinter
	progressBar *pterm.ProgressbarPrinter
	theme       *Theme
}

// PresenterConfig 展示器配置
type PresenterConfig struct {
	EnableColors   bool          `json:"enable_colors"`
	EnableEmojis   bool          `json:"enable_emojis"`
	AnimationSpeed time.Duration `json:"animation_speed"`
	ProgressStyle  string        `json:"progress_style"`
	ShowTimestamps bool          `json:"show_timestamps"`
	Locale         string        `json:"locale"`
	MaxLineLength  int           `json:"max_line_length"`
	AutoWrap       bool          `json:"auto_wrap"`
}

// Theme 主題配置
type Theme struct {
	Primary    pterm.Color `json:"primary"`
	Secondary  pterm.Color `json:"secondary"`
	Success    pterm.Color `json:"success"`
	Warning    pterm.Color `json:"warning"`
	Error      pterm.Color `json:"error"`
	Info       pterm.Color `json:"info"`
	Background pterm.Color `json:"background"`
	Text       pterm.Color `json:"text"`
}

// MessageType 消息類型
type MessageType string

const (
	MessageInfo    MessageType = "info"
	MessageSuccess MessageType = "success"
	MessageWarning MessageType = "warning"
	MessageError   MessageType = "error"
	MessageDebug   MessageType = "debug"
)

// DefaultPresenterConfig 返回默認展示器配置
func DefaultPresenterConfig() *PresenterConfig {
	return &PresenterConfig{
		EnableColors:   true,
		EnableEmojis:   true,
		AnimationSpeed: 100 * time.Millisecond,
		ProgressStyle:  "modern",
		ShowTimestamps: false,
		Locale:         "zh-TW",
		MaxLineLength:  80,
		AutoWrap:       true,
	}
}

// DefaultTheme 返回默認主題
func DefaultTheme() *Theme {
	return &Theme{
		Primary:    pterm.FgBlue,
		Secondary:  pterm.FgCyan,
		Success:    pterm.FgGreen,
		Warning:    pterm.FgYellow,
		Error:      pterm.FgRed,
		Info:       pterm.FgBlue,
		Background: pterm.BgDefault,
		Text:       pterm.FgDefault,
	}
}

// NewEnhancedPresenter 創建增強型展示器
func NewEnhancedPresenter(config *PresenterConfig) *EnhancedPresenter {
	if config == nil {
		config = DefaultPresenterConfig()
	}

	return &EnhancedPresenter{
		config: config,
		theme:  DefaultTheme(),
	}
}

// ShowMessage 顯示消息
func (ep *EnhancedPresenter) ShowMessage(msgType MessageType, title, message string) {
	timestamp := ""
	if ep.config.ShowTimestamps {
		timestamp = fmt.Sprintf("[%s] ", time.Now().Format("15:04:05"))
	}

	// 自動換行處理
	if ep.config.AutoWrap && len(message) > ep.config.MaxLineLength {
		message = ep.wrapText(message, ep.config.MaxLineLength)
	}

	switch msgType {
	case MessageSuccess:
		ep.showSuccess(timestamp, title, message)
	case MessageWarning:
		ep.showWarning(timestamp, title, message)
	case MessageError:
		ep.showError(timestamp, title, message)
	case MessageInfo:
		ep.showInfo(timestamp, title, message)
	case MessageDebug:
		ep.showDebug(timestamp, title, message)
	default:
		ep.showInfo(timestamp, title, message)
	}
}

// showSuccess 顯示成功消息
func (ep *EnhancedPresenter) showSuccess(timestamp, title, message string) {
	emoji := ""
	if ep.config.EnableEmojis {
		emoji = "✅ "
	}

	if ep.config.EnableColors {
		pterm.NewStyle(ep.theme.Success).Printf("%s%s%s", timestamp, emoji, title)
		if message != "" {
			pterm.Println()
			pterm.DefaultBasicText.Print(ep.formatMessage(message))
		}
		pterm.Println()
	} else {
		fmt.Printf("%s%s%s\n", timestamp, emoji, title)
		if message != "" {
			fmt.Println(message)
		}
	}
}

// showWarning 顯示警告消息
func (ep *EnhancedPresenter) showWarning(timestamp, title, message string) {
	emoji := ""
	if ep.config.EnableEmojis {
		emoji = "⚠️  "
	}

	if ep.config.EnableColors {
		pterm.NewStyle(ep.theme.Warning).Printf("%s%s%s", timestamp, emoji, title)
		if message != "" {
			pterm.Println()
			pterm.DefaultBasicText.Print(ep.formatMessage(message))
		}
		pterm.Println()
	} else {
		fmt.Printf("%s%s%s\n", timestamp, emoji, title)
		if message != "" {
			fmt.Println(message)
		}
	}
}

// showError 顯示錯誤消息
func (ep *EnhancedPresenter) showError(timestamp, title, message string) {
	emoji := ""
	if ep.config.EnableEmojis {
		emoji = "❌ "
	}

	if ep.config.EnableColors {
		pterm.NewStyle(ep.theme.Error).Printf("%s%s%s", timestamp, emoji, title)
		if message != "" {
			pterm.Println()
			pterm.DefaultBasicText.Print(ep.formatMessage(message))
		}
		pterm.Println()
	} else {
		fmt.Printf("%s%s%s\n", timestamp, emoji, title)
		if message != "" {
			fmt.Println(message)
		}
	}
}

// showInfo 顯示信息消息
func (ep *EnhancedPresenter) showInfo(timestamp, title, message string) {
	emoji := ""
	if ep.config.EnableEmojis {
		emoji = "ℹ️  "
	}

	if ep.config.EnableColors {
		pterm.NewStyle(ep.theme.Info).Printf("%s%s%s", timestamp, emoji, title)
		if message != "" {
			pterm.Println()
			pterm.DefaultBasicText.Print(ep.formatMessage(message))
		}
		pterm.Println()
	} else {
		fmt.Printf("%s%s%s\n", timestamp, emoji, title)
		if message != "" {
			fmt.Println(message)
		}
	}
}

// showDebug 顯示調試消息
func (ep *EnhancedPresenter) showDebug(timestamp, title, message string) {
	emoji := ""
	if ep.config.EnableEmojis {
		emoji = "🐛 "
	}

	if ep.config.EnableColors {
		pterm.NewStyle(ep.theme.Secondary).Printf("%s%s%s", timestamp, emoji, title)
		if message != "" {
			pterm.Println()
			pterm.DefaultBasicText.Print(ep.formatMessage(message))
		}
		pterm.Println()
	} else {
		fmt.Printf("%s%s%s\n", timestamp, emoji, title)
		if message != "" {
			fmt.Println(message)
		}
	}
}

// StartLoading 開始加載動畫
func (ep *EnhancedPresenter) StartLoading(message string) {
	if ep.spinner != nil {
		_ = ep.spinner.Stop()
	}

	if ep.config.EnableColors {
		ep.spinner, _ = pterm.DefaultSpinner.
			WithText(message).
			WithDelay(ep.config.AnimationSpeed).
			Start()
	} else {
		fmt.Printf("%s...\n", message)
	}
}

// UpdateLoading 更新加載消息
func (ep *EnhancedPresenter) UpdateLoading(message string) {
	if ep.spinner != nil {
		ep.spinner.UpdateText(message)
	}
}

// StopLoading 停止加載動畫
func (ep *EnhancedPresenter) StopLoading(success bool, message string) {
	if ep.spinner != nil {
		if success {
			ep.spinner.Success(message)
		} else {
			ep.spinner.Fail(message)
		}
		ep.spinner = nil
	} else if message != "" {
		if success {
			ep.ShowMessage(MessageSuccess, "", message)
		} else {
			ep.ShowMessage(MessageError, "", message)
		}
	}
}

// ShowProgress 顯示進度條
func (ep *EnhancedPresenter) ShowProgress(title string, current, total int) {
	if ep.progressBar == nil {
		if ep.config.EnableColors {
			ep.progressBar, _ = pterm.DefaultProgressbar.
				WithTitle(title).
				WithTotal(total).
				Start()
		}
	}

	if ep.progressBar != nil {
		ep.progressBar.Current = current
		if current >= total {
			_, _ = ep.progressBar.Stop()
			ep.progressBar = nil
		}
	} else {
		percentage := float64(current) / float64(total) * 100
		fmt.Printf("\r%s: %.1f%% (%d/%d)", title, percentage, current, total)
		if current >= total {
			fmt.Println()
		}
	}
}

// ShowTable 顯示表格
func (ep *EnhancedPresenter) ShowTable(headers []string, rows [][]string, title string) {
	if ep.config.EnableColors {
		table := pterm.DefaultTable.WithHasHeader(len(headers) > 0)

		if title != "" {
			pterm.DefaultHeader.Printf("%s", title)
			pterm.Println()
		}

		data := make([][]string, 0, len(rows)+1)
		if len(headers) > 0 {
			data = append(data, headers)
		}
		data = append(data, rows...)

		_ = table.WithData(data).Render()
	} else {
		if title != "" {
			fmt.Printf("=== %s ===\n", title)
		}

		// 簡單的文本表格
		if len(headers) > 0 {
			fmt.Println(strings.Join(headers, "\t"))
			fmt.Println(strings.Repeat("-", len(strings.Join(headers, "\t"))))
		}

		for _, row := range rows {
			fmt.Println(strings.Join(row, "\t"))
		}
		fmt.Println()
	}
}

// ShowPanel 顯示面板
func (ep *EnhancedPresenter) ShowPanel(title, content string, panelType MessageType) {
	if ep.config.EnableColors {
		var style pterm.Style

		switch panelType {
		case MessageSuccess:
			style = *pterm.NewStyle(ep.theme.Success, pterm.BgDefault)
		case MessageWarning:
			style = *pterm.NewStyle(ep.theme.Warning, pterm.BgDefault)
		case MessageError:
			style = *pterm.NewStyle(ep.theme.Error, pterm.BgDefault)
		default:
			style = *pterm.NewStyle(ep.theme.Info, pterm.BgDefault)
		}

		panel := pterm.DefaultBox.
			WithTitle(title).
			WithTitleTopCenter().
			WithBoxStyle(&style)

		panel.Println(content)
	} else {
		fmt.Printf("┌─ %s ─┐\n", title)
		for _, line := range strings.Split(content, "\n") {
			fmt.Printf("│ %s │\n", line)
		}
		fmt.Printf("└%s┘\n", strings.Repeat("��", len(title)+2))
	}
}

// ShowList 顯示列表
func (ep *EnhancedPresenter) ShowList(items []string, title string, ordered bool) {
	if title != "" {
		ep.ShowMessage(MessageInfo, title, "")
	}

	if ep.config.EnableColors {
		if ordered {
			for i, item := range items {
				pterm.Printf("  %d. %s\n", i+1, item)
			}
		} else {
			bulletItems := make([]pterm.BulletListItem, len(items))
			for i, item := range items {
				bulletItems[i] = pterm.BulletListItem{Level: 0, Text: item}
			}
			list := pterm.DefaultBulletList.WithItems(bulletItems)
			_ = list.Render()
		}
	} else {
		for i, item := range items {
			if ordered {
				fmt.Printf("  %d. %s\n", i+1, item)
			} else {
				fmt.Printf("  • %s\n", item)
			}
		}
	}
	pterm.Println()
}

// ConfirmAction 確認操作
func (ep *EnhancedPresenter) ConfirmAction(message string, defaultYes bool) bool {
	if ep.config.EnableColors {
		prompt := ""
		if defaultYes {
			prompt = " (Y/n)"
		} else {
			prompt = " (y/N)"
		}

		result, _ := pterm.DefaultInteractiveConfirm.
			WithDefaultValue(defaultYes).
			WithDefaultText(message + prompt).
			Show()
		return result
	} else {
		// 簡化的確認實現
		prompt := ""
		if defaultYes {
			prompt = " (Y/n): "
		} else {
			prompt = " (y/N): "
		}

		fmt.Print(message + prompt)
		// 這裡應該讀取用戶輸入，簡化實現返回默認值
		return defaultYes
	}
}

// SelectOption 選擇選項
func (ep *EnhancedPresenter) SelectOption(message string, options []string) (string, error) {
	if ep.config.EnableColors {
		result, err := pterm.DefaultInteractiveSelect.
			WithDefaultText(message).
			WithOptions(options).
			Show()
		return result, err
	} else {
		// 簡化的選擇實現
		fmt.Println(message)
		for i, option := range options {
			fmt.Printf("  %d. %s\n", i+1, option)
		}
		// 這裡應該讀取用戶輸入，簡化實現返回第一個選項
		if len(options) > 0 {
			return options[0], nil
		}
		return "", fmt.Errorf("no options available")
	}
}

// GetInput 獲取用戶輸入
func (ep *EnhancedPresenter) GetInput(prompt string, defaultValue string) (string, error) {
	if ep.config.EnableColors {
		result, err := pterm.DefaultInteractiveTextInput.
			WithDefaultText(prompt).
			WithDefaultValue(defaultValue).
			Show()
		return result, err
	} else {
		fmt.Printf("%s", prompt)
		if defaultValue != "" {
			fmt.Printf(" [%s]", defaultValue)
		}
		fmt.Print(": ")
		// 這裡應該讀取用戶輸入，簡化實現返回默認值
		return defaultValue, nil
	}
}

// wrapText 文本自動換行
func (ep *EnhancedPresenter) wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var lines []string
	var currentLine strings.Builder

	for _, word := range words {
		if currentLine.Len() > 0 && currentLine.Len()+len(word)+1 > width {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
		}

		if currentLine.Len() > 0 {
			currentLine.WriteString(" ")
		}
		currentLine.WriteString(word)
	}

	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return strings.Join(lines, "\n")
}

// formatMessage 格式化消息
func (ep *EnhancedPresenter) formatMessage(message string) string {
	if ep.config.AutoWrap && ep.config.MaxLineLength > 0 {
		return ep.wrapText(message, ep.config.MaxLineLength)
	}
	return message
}

// SetTheme 設置主題
func (ep *EnhancedPresenter) SetTheme(theme *Theme) {
	ep.theme = theme
}

// GetTheme 獲取當前主題
func (ep *EnhancedPresenter) GetTheme() *Theme {
	return ep.theme
}

// UpdateConfig 更新配置
func (ep *EnhancedPresenter) UpdateConfig(config *PresenterConfig) {
	ep.config = config
}

// ShowStatusBar 顯示狀態欄
func (ep *EnhancedPresenter) ShowStatusBar(items []StatusItem) {
	if !ep.config.EnableColors {
		// 簡化文本狀態欄
		var parts []string
		for _, item := range items {
			parts = append(parts, fmt.Sprintf("%s: %s", item.Label, item.Value))
		}
		fmt.Printf("[%s]\n", strings.Join(parts, " | "))
		return
	}

	// 彩色狀態欄
	var panelRows [][]pterm.Panel
	var currentRow []pterm.Panel

	for _, item := range items {
		color := ep.theme.Info
		switch item.Type {
		case "success":
			color = ep.theme.Success
		case "warning":
			color = ep.theme.Warning
		case "error":
			color = ep.theme.Error
		}

		panel := pterm.Panel{
			Data: fmt.Sprintf("%s\n%s",
				pterm.NewStyle(color).Sprint(item.Label),
				item.Value),
		}
		currentRow = append(currentRow, panel)

		// 每3個panel為一行，或者在最後一個item時添加到panelRows
		if len(currentRow) == 3 {
			panelRows = append(panelRows, currentRow)
			currentRow = []pterm.Panel{}
		}
	}

	// 添加剩餘的panel
	if len(currentRow) > 0 {
		panelRows = append(panelRows, currentRow)
	}

	_ = pterm.DefaultPanel.WithPanels(panelRows).Render()
}

// StatusItem 狀態項
type StatusItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Type  string `json:"type"` // success, warning, error, info
}
