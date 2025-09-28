package shell

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/TonnyWong1052/aish/internal/config"
)

//go:embed assets/hook.sh assets/hook.ps1
var embeddedHooks embed.FS

const (
	hookStartMarker = config.HookStartMarker
	hookEndMarker   = config.HookEndMarker
)

// InstallHook installs the shell hook for the current OS.
func InstallHook() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	if runtime.GOOS == "windows" {
		return installWindowsHook()
	}

	// Create ~/bin directory if it doesn't exist
	binDir := filepath.Join(home, "bin")
	if err := os.MkdirAll(binDir, config.DefaultDirPermissions); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Copy the current binary to ~/bin/aish
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	targetPath := filepath.Join(binDir, "aish")
	// If we're already running from the target path, skip copying to avoid cp errors
	if filepath.Clean(currentExe) != filepath.Clean(targetPath) {
		if err := copyFile(currentExe, targetPath); err != nil {
			return fmt.Errorf("failed to copy binary to ~/bin: %w", err)
		}
	}

	// Make it executable
	if err := os.Chmod(targetPath, config.DefaultExecPermissions); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	// Install hooks for both bash and zsh
	if err := installBashHook(home); err != nil {
		return fmt.Errorf("failed to install bash hook: %w", err)
	}

	if err := installZshHook(home); err != nil {
		return fmt.Errorf("failed to install zsh hook: %w", err)
	}

	return nil
}

// UninstallHook removes the shell hook for the current OS.
func UninstallHook() (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, fmt.Errorf("failed to get user home directory: %w", err)
	}

	if runtime.GOOS == "windows" {
		return removeWindowsHook()
	}

	removed := false

	// Remove bash hook
	if bashRemoved, err := removeBashHook(home); err != nil {
		return false, fmt.Errorf("failed to remove bash hook: %w", err)
	} else if bashRemoved {
		removed = true
	}

	// Remove zsh hook
	if zshRemoved, err := removeZshHook(home); err != nil {
		return false, fmt.Errorf("failed to remove zsh hook: %w", err)
	} else if zshRemoved {
		removed = true
	}

	return removed, nil
}

// installBashHook installs the hook for bash
func installBashHook(home string) error {
	bashrcPath := filepath.Join(home, ".bashrc")
	bashProfilePath := filepath.Join(home, ".bash_profile")

	hookCode, err := getHookCode()
	if err != nil {
		return fmt.Errorf("failed to get hook code: %w", err)
	}

	// Try .bashrc first, then .bash_profile
	for _, path := range []string{bashrcPath, bashProfilePath} {
		if _, err := os.Stat(path); err == nil {
			return addHookToFile(path, hookCode)
		}
	}

	// If neither exists, create .bashrc
	return addHookToFile(bashrcPath, hookCode)
}

// installZshHook installs the hook for zsh
func installZshHook(home string) error {
	zshrcPath := filepath.Join(home, ".zshrc")
	hookCode, err := getHookCode()
	if err != nil {
		return fmt.Errorf("failed to get hook code: %w", err)
	}
	return addHookToFile(zshrcPath, hookCode)
}

// removeBashHook removes the hook from bash config files
func removeBashHook(home string) (bool, error) {
	removed := false
	for _, fileName := range []string{".bashrc", ".bash_profile"} {
		path := filepath.Join(home, fileName)
		if fileRemoved, err := removeHookFromFile(path); err != nil {
			return false, err
		} else if fileRemoved {
			removed = true
		}
	}
	return removed, nil
}

// removeZshHook removes the hook from zsh config files
func removeZshHook(home string) (bool, error) {
	path := filepath.Join(home, ".zshrc")
	return removeHookFromFile(path)
}

// getHookCode returns the shell hook code
func getHookCode() (string, error) {
	data, err := embeddedHooks.ReadFile("assets/hook.sh")
	if err != nil {
		return "", fmt.Errorf("failed to read embedded hook.sh: %w", err)
	}
	return string(data), nil
}

// getWindowsHookCode returns the PowerShell hook code.
func getWindowsHookCode() (string, error) {
	data, err := embeddedHooks.ReadFile("assets/hook.ps1")
	if err != nil {
		return "", fmt.Errorf("failed to read embedded hook.ps1: %w", err)
	}
	return string(data), nil
}

// addHookToFile adds the hook code to a shell config file
func addHookToFile(filePath, hookCode string) error {
	// Read existing content
	content, err := os.ReadFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	contentStr := string(content)

	// Check if hook is already installed
	if strings.Contains(contentStr, hookStartMarker) {
		// Replace existing hook block to keep it up to date
		startMarker := hookStartMarker
		endMarker := hookEndMarker
		startIndex := strings.Index(contentStr, startMarker)
		endIndex := strings.Index(contentStr, endMarker)
		if startIndex != -1 && endIndex != -1 {
			// find end of line after endMarker
			tailIdx := strings.Index(contentStr[endIndex:], "\n")
			if tailIdx != -1 {
				endIndex = endIndex + tailIdx
			} else {
				endIndex = len(contentStr) - 1
			}
			contentStr = contentStr[:startIndex] + hookCode + contentStr[endIndex+1:]
		} else {
			// markers inconsistent; append new hook
			contentStr += hookCode
		}
	} else {
		// Append the hook
		contentStr += hookCode
	}

	// Write back to file
	return os.WriteFile(filePath, []byte(contentStr), 0644)
}

// removeHookFromFile removes the hook code from a shell config file
func removeHookFromFile(filePath string) (bool, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // File doesn't exist, nothing to remove
		}
		return false, err
	}

	contentStr := string(content)

	// Check if hook exists
	startMarker := hookStartMarker
	endMarker := hookEndMarker

	startIndex := strings.Index(contentStr, startMarker)
	if startIndex == -1 {
		return false, nil // Hook not found
	}

	endIndex := strings.Index(contentStr, endMarker)
	if endIndex == -1 {
		return false, fmt.Errorf("found start marker but no end marker in %s", filePath)
	}

	// Remove the hook section (including the end marker line)
	endIndex = strings.Index(contentStr[endIndex:], "\n")
	if endIndex != -1 {
		endIndex += len(contentStr[:strings.Index(contentStr, endMarker)])
	} else {
		endIndex = len(contentStr) - 1
	}

	newContent := contentStr[:startIndex] + contentStr[endIndex+1:]

	// Write back to file
	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return false, err
	}

	return true, nil
}

// installWindowsHook installs the hook for PowerShell.
func installWindowsHook() error {
	profilePath, err := resolvePowerShellProfilePath()
	if err != nil {
		return err
	}

	// Ensure the directory for the profile exists.
	profileDir := filepath.Dir(profilePath)
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return fmt.Errorf("failed to create PowerShell profile directory: %w", err)
	}

	hookCode, err := getWindowsHookCode()
	if err != nil {
		return fmt.Errorf("failed to get windows hook code: %w", err)
	}
	return addHookToFile(profilePath, hookCode)
}

// removeWindowsHook removes the hook from PowerShell profile.
func removeWindowsHook() (bool, error) {
	profilePath, err := resolvePowerShellProfilePath()
	if err != nil {
		// If PowerShell isn't installed or fails, we can't determine the path.
		// We'll consider the hook not installed.
		return false, nil
	}

	return removeHookFromFile(profilePath)
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// '/Y' overwrites destination file without prompting.
		cmd = exec.Command("cmd", "/C", "copy", "/Y", src, dst)
	} else {
		// Use cp command for better handling of permissions and metadata
		cmd = exec.Command("cp", src, dst)
	}
	return cmd.Run()
}

// GetHookFilePath returns the path to the hook file.
func GetHookFilePath() (string, error) {
	if runtime.GOOS == "windows" {
		return resolvePowerShellProfilePath()
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	hookCandidates := []string{
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".bash_profile"),
	}
	for _, candidate := range hookCandidates {
		if fileContainsHook(candidate) {
			return candidate, nil
		}
	}

	shell := os.Getenv("SHELL")
	switch {
	case strings.Contains(shell, "zsh"):
		return filepath.Join(home, ".zshrc"), nil
	case strings.Contains(shell, "bash"):
		bashrc := filepath.Join(home, ".bashrc")
		bashProfile := filepath.Join(home, ".bash_profile")
		if fileExists(bashrc) || !fileExists(bashProfile) {
			return bashrc, nil
		}
		return bashProfile, nil
	default:
		if fileExists(filepath.Join(home, ".zshrc")) {
			return filepath.Join(home, ".zshrc"), nil
		}
		return filepath.Join(home, ".bashrc"), nil
	}
}

func fileContainsHook(path string) bool {
	if path == "" {
		return false
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), hookStartMarker)
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func resolvePowerShellProfilePath() (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "echo $PROFILE")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get PowerShell profile path: %w", err)
	}
	profilePath := strings.TrimSpace(string(out))
	if profilePath == "" {
		return "", fmt.Errorf("PowerShell profile path is empty; cannot locate hook")
	}
	return profilePath, nil
}
