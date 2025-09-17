package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	hookStartMarker = "# AISH (AI Shell) Hook - Start"
	hookEndMarker   = "# AISH (AI Shell) Hook - End"
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

	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Create ~/bin directory if it doesn't exist
	binDir := filepath.Join(home, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
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
	if err := os.Chmod(targetPath, 0755); err != nil {
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
	if err != nil {
		return false, fmt.Errorf("failed to get user home directory: %w", err)
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

	hookCode := getHookCode()

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
	hookCode := getHookCode()
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
func getHookCode() string {
	return `
# AISH (AI Shell) Hook - Start

# State file locations
if [ -z "$AISH_STATE_DIR" ]; then
    AISH_STATE_DIR="$HOME/.config/aish"
fi
AISH_STDOUT_FILE="$AISH_STATE_DIR/last_stdout"
AISH_STDERR_FILE="$AISH_STATE_DIR/last_stderr"
AISH_LAST_CMD_FILE="$AISH_STATE_DIR/last_command"
mkdir -p "$AISH_STATE_DIR" >/dev/null 2>&1 || true

# Sensitive information masking: replace common sensitive parameter values in commands with ***REDACTED***
__aish_sanitize_cmd() {
    local _c="$1"
    # Flag form --key=VALUE / --key VALUE (use double quotes to avoid nested quote issues)
    _c=$(printf "%s" "$_c" | sed -E "s/--(api[_-]?key|token|password|passwd|secret|bearer)=([^[:space:]]+)/--\\1=***REDACTED***/g")
    _c=$(printf "%s" "$_c" | sed -E "s/--(api[_-]?key|token|password|passwd|secret|bearer)[[:space:]]+([^[:space:]]+)/--\\1 ***REDACTED***/g")
    # Environment variable form FOO_TOKEN=VALUE or ...SECRET...=VALUE
    _c=$(printf "%s" "$_c" | sed -E "s/([A-Za-z_][A-Za-z0-9_]*((SECRET)|(TOKEN)|(PASSWORD)|(API[_-]?KEY)|(ACCESS[_-]?KEY)|(BEARER))[A-Za-z0-9_]*)=([^[:space:]]+)/\\1=***REDACTED***/g")
    echo "$_c"
}

# Common error keywords for pre-filtering on hook side to reduce invalid triggers
__aish_should_trigger() {
    local exit_code="$1"
    # Only consider non-zero exit codes
    if [ "$exit_code" -eq 0 ]; then
        return 1
    fi
    # Skip user-initiated cancellation/termination (Ctrl+C=SIGINT=130, Ctrl+\=SIGQUIT=131, SIGTERM=143)
    if [ "$exit_code" -eq 130 ] || [ "$exit_code" -eq 131 ] || [ "$exit_code" -eq 143 ]; then
        return 1
    fi
    # If stderr doesn't exist, still allow reporting, let application side filter (conservative mode)
    if [ ! -s "$AISH_STDERR_FILE" ]; then
        return 0
    fi
    # Align with classifier keywords for preliminary judgment
    if grep -Eiq '(command not found|No such file or directory|Permission denied|cannot execute binary file|invalid (argument|option)|File exists|is not a directory)' "$AISH_STDERR_FILE"; then
        return 0
    fi
    return 1
}

# General: avoid recursive triggering when executing aish itself
__aish_should_skip_cmd() {
    case "$1" in
        aish*|*/aish*) return 0;;
        *) return 1;;
    esac
}

if [ -n "$ZSH_VERSION" ]; then
    # zsh version: use preexec/precmd for pre/post wrapping
    __aish_capture_on=0

    _aish_preexec() {
        local cmd="$1"
        __aish_should_skip_cmd "$cmd" && return
        : > "$AISH_STDOUT_FILE"
        : > "$AISH_STDERR_FILE"
        local _sanitized
        _sanitized="$(__aish_sanitize_cmd "$cmd")"
        printf "%s" "$_sanitized" > "$AISH_LAST_CMD_FILE"
        # Save original FD and redirect
        exec 4>&1 5>&2
        exec 1> >(tee -a "$AISH_STDOUT_FILE") 2> >(tee -a "$AISH_STDERR_FILE" >&2)
        __aish_capture_on=1
    }

    _aish_precmd() {
        local exit_code=$?
        if [ "$__aish_capture_on" = "1" ]; then
            # Restore FD
            exec 1>&4 4>&- 2>&5 5>&-
            __aish_capture_on=0
        fi
        local last_command
        last_command=$(cat "$AISH_LAST_CMD_FILE" 2>/dev/null)
        if [ $exit_code -ne 0 ] && [ -n "$last_command" ] && command -v aish >/dev/null 2>&1; then
            __aish_should_trigger "$exit_code" || return $exit_code
            __aish_should_skip_cmd "$last_command" && return
            AISH_STDOUT_FILE="$AISH_STDOUT_FILE" AISH_STDERR_FILE="$AISH_STDERR_FILE" \
                aish capture "$exit_code" "$last_command" 2>/dev/null
        fi
        return $exit_code
    }

    autoload -Uz add-zsh-hook
    add-zsh-hook preexec _aish_preexec
    add-zsh-hook precmd  _aish_precmd

else
    # bash version: use trap DEBUG and PROMPT_COMMAND implementation
    __aish_capture_on=0

    _aish_preexec() {
        # Skip internal or aish itself
        case "$BASH_COMMAND" in
            _aish_*|aish*|*/aish*) return ;;
        esac
        if [ "$__aish_capture_on" = "1" ]; then
            return
        fi
        : > "$AISH_STDOUT_FILE"
        : > "$AISH_STDERR_FILE"
        local _sanitized
        _sanitized="$(__aish_sanitize_cmd "$BASH_COMMAND")"
        printf "%s" "$_sanitized" > "$AISH_LAST_CMD_FILE"
        exec 4>&1 5>&2
        exec 1> >(tee -a "$AISH_STDOUT_FILE") 2> >(tee -a "$AISH_STDERR_FILE" >&2)
        __aish_capture_on=1
    }

    _aish_postcmd() {
        local exit_code=$?
        if [ "$__aish_capture_on" = "1" ]; then
            exec 1>&4 4>&- 2>&5 5>&-
            __aish_capture_on=0
        fi
        local last_command
        last_command=$(cat "$AISH_LAST_CMD_FILE" 2>/dev/null)
        if [ $exit_code -ne 0 ] && [ -n "$last_command" ] && command -v aish >/dev/null 2>&1; then
            __aish_should_trigger "$exit_code" || return $exit_code
            __aish_should_skip_cmd "$last_command" && return
            AISH_STDOUT_FILE="$AISH_STDOUT_FILE" AISH_STDERR_FILE="$AISH_STDERR_FILE" \
                aish capture "$exit_code" "$last_command" 2>/dev/null
        fi
        return $exit_code
    }

    # Install hook (preserve original PROMPT_COMMAND)
    trap '_aish_preexec' DEBUG
    if [[ $PROMPT_COMMAND != *"_aish_postcmd"* ]]; then
        if [ -z "$PROMPT_COMMAND" ]; then
            PROMPT_COMMAND="_aish_postcmd"
        else
            PROMPT_COMMAND="_aish_postcmd; $PROMPT_COMMAND"
        fi
    fi
fi

# AISH (AI Shell) Hook - End
`
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

// getWindowsHookCode returns the PowerShell hook code.
func getWindowsHookCode() string {
	return `
# AISH (AI Shell) Hook - Start

# State file locations
if (-not (Test-Path Env:AISH_STATE_DIR)) {
    $env:AISH_STATE_DIR = "$env:USERPROFILE\.config\aish"
}
$AISH_STATE_DIR = $env:AISH_STATE_DIR
$AISH_STDOUT_FILE = Join-Path $AISH_STATE_DIR "last_stdout"
$AISH_STDERR_FILE = Join-Path $AISH_STATE_DIR "last_stderr"
$AISH_LAST_CMD_FILE = Join-Path $AISH_STATE_DIR "last_command"

if (-not (Test-Path $AISH_STATE_DIR)) {
    New-Item -ItemType Directory -Path $AISH_STATE_DIR -Force | Out-Null
}

# Override the prompt function to capture command output
if ((Get-Command "prompt" -CommandType Function).ScriptBlock.ToString() -notmatch "AISH") {
    $private:__aish_original_prompt = (Get-Command "prompt" -CommandType Function).ScriptBlock
    function prompt {
        # Capture exit code of the last command
        $lastExitCode = $LastExitCode

        # Run original prompt to display it
        & $private:__aish_original_prompt

        if ($lastExitCode -ne 0) {
            $last_command = Get-Content $AISH_LAST_CMD_FILE -Raw -ErrorAction SilentlyContinue
            if ($last_command -and (Get-Command aish -ErrorAction SilentlyContinue)) {
                # In PowerShell, output is captured via transcript, not redirection here.
                # The 'aish capture' command will read from the transcript if needed.
                aish capture $lastExitCode $last_command
            }
        }
        
        # This part is tricky in PowerShell. We rely on Start-Transcript for capture.
        # For simplicity, we'll just log the command before it runs.
        # A more robust solution might involve pre-command hooks if available.
        Register-EngineEvent -SourceIdentifier PowerShell.OnIdle -Action {
            $cmd = Get-History -Count 1
            if ($cmd) {
                $cmd.CommandLine | Out-File $AISH_LAST_CMD_FILE
            }
        } | Out-Null
        
        return " " # Return a space to satisfy the prompt function contract
    }
}

# AISH (AI Shell) Hook - End
`
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

	hookCode := getWindowsHookCode()
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
