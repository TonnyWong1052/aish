package prompt

import (
	"encoding/json"
	"fmt"
	"os"
)

// Manager handles loading and accessing prompts.
type Manager struct {
	prompts map[string]map[string]string
}

// NewManager creates a prompt manager from a file.
func NewManager(path string) (*Manager, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var prompts map[string]map[string]string
	if err := json.Unmarshal(data, &prompts); err != nil {
		return nil, err
	}

	return &Manager{prompts: prompts}, nil
}

// NewDefaultManager creates a prompt manager with built-in default prompts.
func NewDefaultManager() *Manager {
	defaultPrompts := map[string]map[string]string{
		"generate_command": {
			"en":    "You are a shell command generator for macOS. Output ONLY a single-line JSON object with the exact schema: {\"command\":\"<shell>\"}. No prose, no markdown, no extra keys. Use a safe, single command.\nPrompt: {{.Prompt}}\nJSON:",
			"zh-TW": "你是 macOS 的指令產生器。僅輸出一行 JSON，結構嚴格為：{\"command\":\"<shell>\"}。不要輸出說明、Markdown 或多餘鍵。請產生安全且可執行的單一指令。\n提示：{{.Prompt}}\nJSON：",
		},
		"get_suggestion": {
			"en":    "You are a shell debugging assistant on macOS. Output ONLY one JSON object with schema: {\"explanation\":\"...\",\"command\":\"<shell>\"}. Do not include markdown or extra keys.\nCommand: {{.Command}}\nExit Code: {{.ExitCode}}\nStdout:\n{{.Stdout}}\nStderr:\n{{.Stderr}}\nJSON:",
			"zh-TW": "你是 macOS 的指令除錯助理。僅輸出一個 JSON 物件，結構嚴格為：{\"explanation\":\"...\",\"command\":\"<shell>\"}。不要包含 Markdown 或多餘鍵。\n指令：{{.Command}}\n結束代碼：{{.ExitCode}}\n標準輸出：\n{{.Stdout}}\n標準錯誤：\n{{.Stderr}}\nJSON：",
		},
		"get_enhanced_suggestion": {
			"en":    "You are a shell debugging assistant on macOS with enhanced context awareness. Output ONLY one JSON object with schema: {\"explanation\":\"...\",\"command\":\"<shell>\"}. Do not include markdown or extra keys.\n\nFailed Command: {{.Command}}\nExit Code: {{.ExitCode}}\nStdout:\n{{.Stdout}}\nStderr:\n{{.Stderr}}\n\nContext Information:\nWorking Directory: {{.WorkingDirectory}}\nShell: {{.ShellType}}\n\n{{if .RecentCommands}}Recent Command History:\n{{range $index, $cmd := .RecentCommands}}{{add $index 1}}. {{$cmd}}\n{{end}}{{end}}\n{{if .DirectoryListing}}Directory Contents:\n{{range .DirectoryListing}}{{.}}\n{{end}}{{end}}\nJSON:",
			"zh-TW": "你是具備進階上下文感知的 macOS 指令除錯助理。僅輸出一個 JSON 物件，結構嚴格為：{\"explanation\":\"...\",\"command\":\"<shell>\"}。不要包含 Markdown 或多餘鍵。\n\n失敗指令：{{.Command}}\n結束代碼：{{.ExitCode}}\n標準輸出：\n{{.Stdout}}\n標準錯誤：\n{{.Stderr}}\n\n上下文資訊：\n工作目錄：{{.WorkingDirectory}}\n終端類型：{{.ShellType}}\n\n{{if .RecentCommands}}最近指令歷史：\n{{range $index, $cmd := .RecentCommands}}{{add $index 1}}. {{$cmd}}\n{{end}}{{end}}\n{{if .DirectoryListing}}目錄內容：\n{{range .DirectoryListing}}{{.}}\n{{end}}{{end}}\nJSON：",
		},
	}
	return &Manager{prompts: defaultPrompts}
}

// GetPrompt returns a prompt by key.
func (m *Manager) GetPrompt(key string, lang string) (string, error) {
	if langPrompts, ok := m.prompts[key]; ok {
		if prompt, ok := langPrompts[lang]; ok {
			return prompt, nil
		}
		// Fallback to English if the specified language is not found
		if prompt, ok := langPrompts["en"]; ok {
			return prompt, nil
		}
	}
	return "", fmt.Errorf("prompt with key '%s' not found", key)
}
