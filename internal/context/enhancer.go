package context

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ContextEnhancer provides advanced context analysis functionality
type ContextEnhancer struct {
	maxHistoryEntries  int
	includeDirectories bool
	filterSensitiveCmd bool
}

// Config defines configuration options for context enhancer
type Config struct {
	MaxHistoryEntries  int  // Maximum history entries (default 10)
	IncludeDirectories bool // Whether to include directory listing (default true)
	FilterSensitiveCmd bool // Whether to filter sensitive commands (default true)
}

// EnhancedContext contains enhanced context information
type EnhancedContext struct {
	RecentCommands   []string // Recent command history
	DirectoryListing []string // Current directory file listing
	WorkingDirectory string   // Current working directory
	ShellType        string   // Shell type (bash/zsh)
}

// NewEnhancer 創建一個新的上下文增強器
func NewEnhancer(config Config) *ContextEnhancer {
	if config.MaxHistoryEntries == 0 {
		config.MaxHistoryEntries = 10
	}

	return &ContextEnhancer{
		maxHistoryEntries:  config.MaxHistoryEntries,
		includeDirectories: config.IncludeDirectories,
		filterSensitiveCmd: config.FilterSensitiveCmd,
	}
}

// EnhanceContext 增強上下文信息
func (e *ContextEnhancer) EnhanceContext() (*EnhancedContext, error) {
	ctx := &EnhancedContext{}

	// 獲取當前工作目錄
	wd, err := os.Getwd()
	if err == nil {
		ctx.WorkingDirectory = wd
	}

	// 檢測 Shell 類型並獲取命令歷史
	shellType := e.detectShellType()
	ctx.ShellType = shellType

	recentCommands, err := e.getRecentCommands(shellType)
	if err == nil {
		ctx.RecentCommands = recentCommands
	}

	// 獲取目錄列表 (如果啟用)
	if e.includeDirectories {
		dirListing, err := e.getDirectoryListing()
		if err == nil {
			ctx.DirectoryListing = dirListing
		}
	}

	return ctx, nil
}

// detectShellType 檢測當前 Shell 類型
func (e *ContextEnhancer) detectShellType() string {
	shell := os.Getenv("SHELL")
	if strings.Contains(shell, "zsh") {
		return "zsh"
	} else if strings.Contains(shell, "bash") {
		return "bash"
	}
	return "unknown"
}

// getRecentCommands 獲取最近的命令歷史
func (e *ContextEnhancer) getRecentCommands(shellType string) ([]string, error) {
	var historyFile string
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	switch shellType {
	case "zsh":
		historyFile = filepath.Join(home, ".zsh_history")
	case "bash":
		historyFile = filepath.Join(home, ".bash_history")
	default:
		// 嘗試使用 history 命令
		return e.getHistoryFromCommand()
	}

	return e.readHistoryFromFile(historyFile)
}

// readHistoryFromFile 從歷史檔案讀取命令
func (e *ContextEnhancer) readHistoryFromFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		// 如果文件不存在，嘗試使用 history 命令
		return e.getHistoryFromCommand()
	}
	defer file.Close()

	var commands []string
	var allCommands []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// 處理 zsh 歷史格式 (: timestamp:duration;command)
		if strings.HasPrefix(line, ":") && strings.Contains(line, ";") {
			parts := strings.SplitN(line, ";", 2)
			if len(parts) == 2 {
				line = parts[1]
			}
		}

		// 過濾空行和敏感命令
		if line != "" && (!e.filterSensitiveCmd || !e.isSensitiveCommand(line)) {
			allCommands = append(allCommands, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// 返回最近的命令 (從末尾開始)
	start := len(allCommands) - e.maxHistoryEntries
	if start < 0 {
		start = 0
	}

	// 反轉順序，使最新的命令在前面
	for i := len(allCommands) - 1; i >= start; i-- {
		commands = append(commands, allCommands[i])
	}

	return commands, nil
}

// getHistoryFromCommand 使用 history 命令獲取歷史
func (e *ContextEnhancer) getHistoryFromCommand() ([]string, error) {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("history %d | sed 's/^[ ]*[0-9]*[ ]*//'", e.maxHistoryEntries))
	output, err := cmd.Output()
	if err != nil {
		return []string{}, nil // 返回空數組而不是錯誤
	}

	lines := strings.Split(string(output), "\n")
	var commands []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && (!e.filterSensitiveCmd || !e.isSensitiveCommand(line)) {
			commands = append(commands, line)
		}
	}

	return commands, nil
}

// getDirectoryListing 獲取當前目錄的文件列表
func (e *ContextEnhancer) getDirectoryListing() ([]string, error) {
	cmd := exec.Command("ls", "-la")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	var files []string

	// 跳過第一行 (總計) 和空行
	for i, line := range lines {
		if i > 0 && strings.TrimSpace(line) != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

// isSensitiveCommand 檢查是否為敏感命令
func (e *ContextEnhancer) isSensitiveCommand(cmd string) bool {
	sensitiveKeywords := []string{
		"password",
		"passwd",
		"api_key",
		"secret",
		"token",
		"auth",
		"credential",
		"private",
		"ssh-keygen",
		"openssl",
	}

	cmdLower := strings.ToLower(cmd)
	for _, keyword := range sensitiveKeywords {
		if strings.Contains(cmdLower, keyword) {
			return true
		}
	}

	return false
}

// FormatForPrompt 將增強的上下文格式化為適合 LLM 的字符串
func (ctx *EnhancedContext) FormatForPrompt() string {
	var parts []string

	if ctx.WorkingDirectory != "" {
		parts = append(parts, fmt.Sprintf("Working Directory: %s", ctx.WorkingDirectory))
	}

	if ctx.ShellType != "" {
		parts = append(parts, fmt.Sprintf("Shell: %s", ctx.ShellType))
	}

	if len(ctx.RecentCommands) > 0 {
		parts = append(parts, "Recent Commands:")
		for i, cmd := range ctx.RecentCommands {
			parts = append(parts, fmt.Sprintf("  %d. %s", i+1, cmd))
		}
	}

	if len(ctx.DirectoryListing) > 0 {
		parts = append(parts, "Directory Listing:")
		for _, file := range ctx.DirectoryListing {
			parts = append(parts, fmt.Sprintf("  %s", file))
		}
	}

	return strings.Join(parts, "\n")
}
