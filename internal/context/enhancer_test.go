package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewEnhancer(t *testing.T) {
	config := Config{
		MaxHistoryEntries:  5,
		IncludeDirectories: true,
		FilterSensitiveCmd: true,
	}

	enhancer := NewEnhancer(config)

	if enhancer.maxHistoryEntries != 5 {
		t.Errorf("Expected maxHistoryEntries to be 5, got %d", enhancer.maxHistoryEntries)
	}

	if !enhancer.includeDirectories {
		t.Error("Expected includeDirectories to be true")
	}

	if !enhancer.filterSensitiveCmd {
		t.Error("Expected filterSensitiveCmd to be true")
	}
}

func TestNewEnhancerWithDefaults(t *testing.T) {
	config := Config{} // Empty config

	enhancer := NewEnhancer(config)

	if enhancer.maxHistoryEntries != 10 {
		t.Errorf("Expected default maxHistoryEntries to be 10, got %d", enhancer.maxHistoryEntries)
	}
}

func TestDetectShellType(t *testing.T) {
	enhancer := NewEnhancer(Config{})

	// Test with zsh
	oldShell := os.Getenv("SHELL")
	defer os.Setenv("SHELL", oldShell)

	os.Setenv("SHELL", "/bin/zsh")
	shellType := enhancer.detectShellType()
	if shellType != "zsh" {
		t.Errorf("Expected zsh, got %s", shellType)
	}

	// Test with bash
	os.Setenv("SHELL", "/bin/bash")
	shellType = enhancer.detectShellType()
	if shellType != "bash" {
		t.Errorf("Expected bash, got %s", shellType)
	}

	// Test with unknown shell
	os.Setenv("SHELL", "/bin/fish")
	shellType = enhancer.detectShellType()
	if shellType != "unknown" {
		t.Errorf("Expected unknown, got %s", shellType)
	}
}

func TestIsSensitiveCommand(t *testing.T) {
	enhancer := NewEnhancer(Config{FilterSensitiveCmd: true})

	testCases := []struct {
		command   string
		sensitive bool
	}{
		{"ls -la", false},
		{"echo 'hello world'", false},
		{"export API_KEY=secret", true},
		{"ssh-keygen -t rsa", true},
		{"openssl genrsa -out private.key", true},
		{"curl -H 'Authorization: Bearer token123'", true},
		{"cd /home/user", false},
		{"PASSWORD=123 ./script.sh", true},
		{"npm install", false},
	}

	for _, tc := range testCases {
		result := enhancer.isSensitiveCommand(tc.command)
		if result != tc.sensitive {
			t.Errorf("Command '%s': expected sensitive=%v, got %v", tc.command, tc.sensitive, result)
		}
	}
}

func TestGetDirectoryListing(t *testing.T) {
	enhancer := NewEnhancer(Config{IncludeDirectories: true})

	// Create a temporary directory with some files
	tmpDir, err := os.MkdirTemp("", "aish_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	testFiles := []string{"file1.txt", "file2.go", "subdir"}
	for _, name := range testFiles {
		if name == "subdir" {
			os.Mkdir(filepath.Join(tmpDir, name), 0755)
		} else {
			os.WriteFile(filepath.Join(tmpDir, name), []byte("test content"), 0644)
		}
	}

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Test directory listing
	listing, err := enhancer.getDirectoryListing()
	if err != nil {
		t.Errorf("getDirectoryListing failed: %v", err)
	}

	if len(listing) == 0 {
		t.Error("Expected non-empty directory listing")
	}

	// Check if all test files appear in the listing
	listingText := strings.Join(listing, "\n")
	for _, file := range testFiles {
		if !strings.Contains(listingText, file) {
			t.Errorf("Expected file '%s' to appear in listing", file)
		}
	}
}

func TestEnhanceContext(t *testing.T) {
	enhancer := NewEnhancer(Config{
		MaxHistoryEntries:  10,
		IncludeDirectories: true,
		FilterSensitiveCmd: true,
	})

	ctx, err := enhancer.EnhanceContext()
	if err != nil {
		t.Errorf("EnhanceContext failed: %v", err)
	}

	if ctx == nil {
		t.Fatal("Expected non-nil context")
	}

	// Check if working directory is set
	if ctx.WorkingDirectory == "" {
		t.Error("Expected working directory to be set")
	}

	// Check if shell type is detected
	if ctx.ShellType == "" {
		t.Error("Expected shell type to be detected")
	}

	// Recent commands and directory listing might be empty in test environment
	// but should not cause errors
}

func TestFormatForPrompt(t *testing.T) {
	ctx := &EnhancedContext{
		WorkingDirectory: "/home/user/project",
		ShellType:        "zsh",
		RecentCommands:   []string{"ls -la", "cd ..", "git status"},
		DirectoryListing: []string{"-rw-r--r-- 1 user user 123 Jan 1 12:00 file1.txt", "drwxr-xr-x 2 user user 456 Jan 1 12:00 subdir"},
	}

	formatted := ctx.FormatForPrompt()

	expectedStrings := []string{
		"Working Directory: /home/user/project",
		"Shell: zsh",
		"Recent Commands:",
		"1. ls -la",
		"2. cd ..",
		"3. git status",
		"Directory Listing:",
		"file1.txt",
		"subdir",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(formatted, expected) {
			t.Errorf("Expected formatted output to contain '%s', got:\n%s", expected, formatted)
		}
	}
}

func TestReadHistoryFromFile_ZshFormat(t *testing.T) {
	enhancer := NewEnhancer(Config{MaxHistoryEntries: 3})

	// Create temporary history file with zsh format
	tmpFile, err := os.CreateTemp("", "zsh_history")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	historyContent := `: 1640995200:0;ls -la
: 1640995210:0;cd project
: 1640995220:0;git status
: 1640995230:0;vim README.md
`
	os.WriteFile(tmpFile.Name(), []byte(historyContent), 0644)

	commands, err := enhancer.readHistoryFromFile(tmpFile.Name())
	if err != nil {
		t.Errorf("readHistoryFromFile failed: %v", err)
	}

	expectedCommands := []string{"vim README.md", "git status", "cd project"}
	if len(commands) != len(expectedCommands) {
		t.Errorf("Expected %d commands, got %d", len(expectedCommands), len(commands))
	}

	for i, expected := range expectedCommands {
		if i < len(commands) && commands[i] != expected {
			t.Errorf("Command %d: expected '%s', got '%s'", i, expected, commands[i])
		}
	}
}

func TestReadHistoryFromFile_BashFormat(t *testing.T) {
	enhancer := NewEnhancer(Config{MaxHistoryEntries: 2})

	// Create temporary history file with bash format
	tmpFile, err := os.CreateTemp("", "bash_history")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	historyContent := `ls -la
cd project
git status
vim README.md
`
	os.WriteFile(tmpFile.Name(), []byte(historyContent), 0644)

	commands, err := enhancer.readHistoryFromFile(tmpFile.Name())
	if err != nil {
		t.Errorf("readHistoryFromFile failed: %v", err)
	}

	expectedCommands := []string{"vim README.md", "git status"}
	if len(commands) != len(expectedCommands) {
		t.Errorf("Expected %d commands, got %d", len(expectedCommands), len(commands))
	}

	for i, expected := range expectedCommands {
		if i < len(commands) && commands[i] != expected {
			t.Errorf("Command %d: expected '%s', got '%s'", i, expected, commands[i])
		}
	}
}
