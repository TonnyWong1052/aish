package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetHookCode(t *testing.T) {
	hookCode := getHookCode()

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
	err = os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Add hook
	hookCode := getHookCode()
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

	err = os.WriteFile(testFile, []byte(contentWithHook), 0644)
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
	err = os.WriteFile(testFile, []byte(content), 0644)
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
