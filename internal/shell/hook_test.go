package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetHookCode(t *testing.T) {
	hookCode, err := getHookCode()
	if err != nil {
		t.Fatalf("Failed to get hook code: %v", err)
	}

	// Check that hook code contains expected components
	expectedComponents := []string{
		"# AISH (AI Shell) Hook - Start",
		"# AISH (AI Shell) Hook - End",
		"__aish_should_trigger",
		"__aish_sanitize_cmd",
		"_aish_preexec",
		"_aish_precmd",
		"ZSH_VERSION",
	}

	for _, component := range expectedComponents {
		if !strings.Contains(hookCode, component) {
			t.Errorf("Hook code missing expected component: %s", component)
		}
	}

	// Verify that Ctrl+C filtering is included
	if !strings.Contains(hookCode, "130") || !strings.Contains(hookCode, "131") {
		t.Error("Hook code missing Ctrl+C signal filtering")
	}
}

func TestAddHookToFile(t *testing.T) {
	// Create a temporary file
	tmpDir, err := os.MkdirTemp("", "aish_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, ".bashrc")

	// Write initial content
	initialContent := "# This is a test .bashrc\nexport PATH=$PATH:~/bin\n"
	err = os.WriteFile(testFile, []byte(initialContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Add hook
	hookCode, err := getHookCode()
	if err != nil {
		t.Fatalf("Failed to get hook code: %v", err)
	}
	err = addHookToFile(testFile, hookCode)
	if err != nil {
		t.Fatalf("Failed to add hook: %v", err)
	}

	// Read and verify content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	contentStr := string(content)

	// Should contain original content
	if !strings.Contains(contentStr, "# This is a test .bashrc") {
		t.Error("Original content not preserved")
	}

	// Should contain hook
	if !strings.Contains(contentStr, "# AISH (AI Shell) Hook - Start") {
		t.Error("Hook not added")
	}

	// Adding hook again should not duplicate it
	err = addHookToFile(testFile, hookCode)
	if err != nil {
		t.Fatalf("Failed to add hook again: %v", err)
	}

	// Count occurrences of hook start marker
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file again: %v", err)
	}

	occurrences := strings.Count(string(newContent), "# AISH (AI Shell) Hook - Start")
	if occurrences != 1 {
		t.Errorf("Expected 1 hook occurrence, got %d", occurrences)
	}
}

func TestRemoveHookFromFile(t *testing.T) {
	// Create a temporary file with hook
	tmpDir, err := os.MkdirTemp("", "aish_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, ".bashrc")

	// Write content with hook
	contentWithHook := `# This is a test .bashrc
export PATH=$PATH:~/bin

# AISH (AI Shell) Hook - Start
__aish_hook() {
    local exit_code=$?
    echo "Hook executed"
}
# AISH (AI Shell) Hook - End

# More content after hook
alias ll='ls -la'
`

	err = os.WriteFile(testFile, []byte(contentWithHook), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Remove hook
	removed, err := removeHookFromFile(testFile)
	if err != nil {
		t.Fatalf("Failed to remove hook: %v", err)
	}

	if !removed {
		t.Error("Expected hook to be removed, but removed=false")
	}

	// Read and verify content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	contentStr := string(content)

	// Should not contain hook
	if strings.Contains(contentStr, "# AISH (AI Shell) Hook - Start") {
		t.Error("Hook still present after removal")
	}

	// Should contain original content
	if !strings.Contains(contentStr, "# This is a test .bashrc") {
		t.Error("Original content before hook not preserved")
	}

	if !strings.Contains(contentStr, "alias ll='ls -la'") {
		t.Error("Original content after hook not preserved")
	}
}

func TestRemoveHookFromNonExistentFile(t *testing.T) {
	removed, err := removeHookFromFile("/nonexistent/file")
	if err != nil {
		t.Fatalf("Expected no error for non-existent file, got: %v", err)
	}

	if removed {
		t.Error("Expected removed=false for non-existent file")
	}
}

func TestRemoveHookFromFileWithoutHook(t *testing.T) {
	// Create a temporary file without hook
	tmpDir, err := os.MkdirTemp("", "aish_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, ".bashrc")

	// Write content without hook
	content := "# This is a test .bashrc\nexport PATH=$PATH:~/bin\n"
	err = os.WriteFile(testFile, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Try to remove hook
	removed, err := removeHookFromFile(testFile)
	if err != nil {
		t.Fatalf("Failed to process file: %v", err)
	}

	if removed {
		t.Error("Expected removed=false for file without hook")
	}
}

func TestFileExists(t *testing.T) {
	// Test with existing file
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	if !fileExists(tmpFile.Name()) {
		t.Error("fileExists should return true for existing file")
	}

	// Test with non-existent file
	if fileExists("/nonexistent/file/path") {
		t.Error("fileExists should return false for non-existent file")
	}

	// Test with empty path
	if fileExists("") {
		t.Error("fileExists should return false for empty path")
	}
}

func TestFileContainsHook(t *testing.T) {
	// Create temp file with hook
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "# AISH (AI Shell) Hook - Start\nSome hook code\n# AISH (AI Shell) Hook - End\n"
	tmpFile.WriteString(content)
	tmpFile.Close()

	if !fileContainsHook(tmpFile.Name()) {
		t.Error("fileContainsHook should return true for file with hook")
	}

	// Test with file without hook
	tmpFile2, err := os.CreateTemp("", "test2")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile2.Name())

	tmpFile2.WriteString("# Some other content\n")
	tmpFile2.Close()

	if fileContainsHook(tmpFile2.Name()) {
		t.Error("fileContainsHook should return false for file without hook")
	}

	// Test with non-existent file
	if fileContainsHook("/nonexistent/file") {
		t.Error("fileContainsHook should return false for non-existent file")
	}

	// Test with empty path
	if fileContainsHook("") {
		t.Error("fileContainsHook should return false for empty path")
	}
}

func TestGetHookFilePath(t *testing.T) {
	// This test is environment-dependent, so we just check it doesn't crash
	path, err := GetHookFilePath()
	if err != nil {
		t.Fatalf("GetHookFilePath failed: %v", err)
	}

	if path == "" {
		t.Error("GetHookFilePath should return non-empty path")
	}

	// Path should end with a recognized shell config file
	if !strings.HasSuffix(path, ".zshrc") &&
		!strings.HasSuffix(path, ".bashrc") &&
		!strings.HasSuffix(path, ".bash_profile") &&
		!strings.Contains(path, "Microsoft.PowerShell_profile.ps1") {
		t.Logf("Warning: unexpected hook file path: %s", path)
	}
}

func TestAddHookToFile_NewFile(t *testing.T) {
	// Test adding hook to a non-existent file (should create it)
	tmpDir, err := os.MkdirTemp("", "aish_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "new_file.sh")

	hookCode, err := getHookCode()
	if err != nil {
		t.Fatalf("Failed to get hook code: %v", err)
	}

	err = addHookToFile(testFile, hookCode)
	if err != nil {
		t.Fatalf("Failed to add hook to new file: %v", err)
	}

	// Verify file was created
	if !fileExists(testFile) {
		t.Error("File should be created")
	}

	// Verify hook was added
	if !fileContainsHook(testFile) {
		t.Error("Hook should be in the new file")
	}
}

func TestAddHookToFile_UpdateExisting(t *testing.T) {
	// Test updating an existing hook
	tmpDir, err := os.MkdirTemp("", "aish_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, ".zshrc")

	// Write content with old hook
	oldHook := `# AISH (AI Shell) Hook - Start
old_hook_function() {
    echo "old hook"
}
# AISH (AI Shell) Hook - End
`
	err = os.WriteFile(testFile, []byte(oldHook), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Add new hook
	newHookCode, err := getHookCode()
	if err != nil {
		t.Fatalf("Failed to get hook code: %v", err)
	}

	err = addHookToFile(testFile, newHookCode)
	if err != nil {
		t.Fatalf("Failed to update hook: %v", err)
	}

	// Read content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	contentStr := string(content)

	// Old hook should be replaced
	if strings.Contains(contentStr, "old_hook_function") {
		t.Error("Old hook should be replaced")
	}

	// Should only have one hook start marker
	occurrences := strings.Count(contentStr, "# AISH (AI Shell) Hook - Start")
	if occurrences != 1 {
		t.Errorf("Expected 1 hook start marker, got %d", occurrences)
	}
}

func TestRemoveHookFromFile_InconsistentMarkers(t *testing.T) {
	// Test file with start marker but no end marker
	tmpDir, err := os.MkdirTemp("", "aish_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, ".bashrc")

	// Write content with only start marker
	content := `# Some content
# AISH (AI Shell) Hook - Start
hook_code_here
# Missing end marker
`
	err = os.WriteFile(testFile, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Try to remove hook
	_, err = removeHookFromFile(testFile)
	if err == nil {
		t.Error("Should return error for inconsistent markers")
	}

	if !strings.Contains(err.Error(), "no end marker") {
		t.Errorf("Error message should mention missing end marker, got: %v", err)
	}
}

func TestAddHookToFile_InconsistentMarkers(t *testing.T) {
	// Test adding hook when file has incomplete existing markers
	tmpDir, err := os.MkdirTemp("", "aish_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, ".bashrc")

	// Write content with only start marker (inconsistent state)
	content := `# Some content
# AISH (AI Shell) Hook - Start
incomplete_hook
`
	err = os.WriteFile(testFile, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Add hook - should append new hook
	hookCode, err := getHookCode()
	if err != nil {
		t.Fatalf("Failed to get hook code: %v", err)
	}

	err = addHookToFile(testFile, hookCode)
	if err != nil {
		t.Fatalf("Failed to add hook: %v", err)
	}

	// Should have hook added
	if !fileContainsHook(testFile) {
		t.Error("Hook should be added")
	}
}

func TestCopyFile(t *testing.T) {
	// Create source file
	tmpDir, err := os.MkdirTemp("", "aish_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcFile := filepath.Join(tmpDir, "source.txt")
	dstFile := filepath.Join(tmpDir, "dest.txt")

	content := "test content for copy"
	err = os.WriteFile(srcFile, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Copy file
	err = copyFile(srcFile, dstFile)
	if err != nil {
		t.Fatalf("Failed to copy file: %v", err)
	}

	// Verify destination exists
	if !fileExists(dstFile) {
		t.Error("Destination file should exist")
	}

	// Verify content matches
	dstContent, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(dstContent) != content {
		t.Errorf("Content mismatch: expected %q, got %q", content, string(dstContent))
	}
}
