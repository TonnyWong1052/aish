# Windows/PowerShell Support for AISH

This document provides comprehensive information about Windows and PowerShell support in AISH (AI Shell).

## Overview

AISH now includes full support for Windows PowerShell, enabling Windows users to benefit from AI-powered shell error analysis and command generation.

## Requirements

- **Operating System**: Windows 10 or later
- **PowerShell**: Version 5.1 or later (included in Windows 10/11)
  - PowerShell 7+ recommended for best experience
- **Go**: Version 1.23 or later (for building from source)
- **Optional**: PSReadLine module (for better command capture)

## Installation

### Method 1: Automated Installation Script (Recommended)

```powershell
# Clone the repository
git clone https://github.com/TonnyWong1052/aish.git
cd aish

# Run the installation script
.\scripts\install.ps1

# Or with automatic initialization
.\scripts\install.ps1 -WithInit
```

### Method 2: Manual Installation

```powershell
# Build the binary
go build -o aish.exe .\cmd\aish

# Create user bin directory
$binDir = "$env:USERPROFILE\bin"
New-Item -ItemType Directory -Path $binDir -Force

# Copy binary
Copy-Item aish.exe $binDir\

# Add to PATH permanently
$currentPath = [Environment]::GetEnvironmentVariable('Path', [EnvironmentVariableTarget]::User)
if ($currentPath -notlike "*$binDir*") {
    [Environment]::SetEnvironmentVariable('Path', "$currentPath;$binDir", [EnvironmentVariableTarget]::User)
}

# Restart PowerShell, then initialize
aish init
```

## PowerShell Hook Features

The PowerShell hook (`hook.ps1`) provides:

### ✅ Implemented Features

1. **Command Capture**: Automatically captures failed commands for AI analysis
2. **Exit Code Detection**: Monitors `$LASTEXITCODE` and `$?` for error detection
3. **Sensitive Data Sanitization**: Redacts API keys, tokens, passwords before sending to AI
4. **Skip Patterns**: Configurable command patterns to skip (e.g., interactive tools)
5. **User-Installed Command Filtering**: Option to skip non-system commands
6. **Error Pattern Matching**: Detects common Windows error patterns
7. **PSReadLine Integration**: Uses PSReadLine for better command history when available
8. **Prompt Hook**: Integrates with PowerShell prompt function for seamless operation

### 🔒 Security Features

- **Automatic Redaction**: Masks `--api-key`, `--token`, `--password` parameters
- **Environment Variable Protection**: Redacts variables containing `SECRET`, `TOKEN`, `PASSWORD`, `API_KEY`
- **Self-Protection**: Skips AISH's own commands to prevent infinite loops
- **Secure Storage**: State files stored in `$env:USERPROFILE\.config\aish`

### ⚙️ Configuration Options

```powershell
# Skip specific command patterns (space-separated globs)
$env:AISH_SKIP_COMMAND_PATTERNS = "npm* yarn* pnpm*"

# Skip all non-system commands (default: 1)
$env:AISH_SKIP_ALL_USER_COMMANDS = "1"

# Custom system directory whitelist (semicolon-separated)
$env:AISH_SYSTEM_DIR_WHITELIST = "C:\Windows\System32;C:\Windows"

# Disable hook temporarily
$env:AISH_HOOK_DISABLED = "1"

# Bypass hook for single command
$env:AISH_CAPTURE_OFF = 1; some-command; $env:AISH_CAPTURE_OFF = $null
```

## Differences from Bash/Zsh Hooks

### Architecture Differences

| Feature | Bash/Zsh | PowerShell |
|---------|-----------|------------|
| **Hook Mechanism** | `trap DEBUG` / `preexec`/`precmd` | Prompt function override |
| **Output Capture** | `tee` with process substitution | Error stream + state files |
| **Command History** | Built-in `BASH_COMMAND` | `Get-History` / PSReadLine |
| **Error Stream** | Separate stderr (fd 2) | `$Error` automatic variable |
| **Exit Codes** | Direct `$?` check | `$LASTEXITCODE` + `$?` |

### Current Limitations on Windows

1. **Output Capture Limitations**:
   - PowerShell doesn't support `tee` with process substitution like bash
   - Full stdout/stderr capture requires PowerShell's error stream analysis
   - Some output may not be captured for commands that manage their own output streams

2. **No Background Process Redirection**:
   - Unlike bash/zsh which use `tee` to capture output in real-time
   - PowerShell hook relies on post-execution analysis

3. **PSReadLine Dependency**:
   - Better command capture requires PSReadLine module
   - Basic functionality works without it, but with reduced accuracy

### Workarounds Implemented

1. **Error Stream Analysis**: Captures PowerShell's `$Error[0]` for recent errors
2. **State File Approach**: Uses temporary files for command and output storage
3. **Exit Code Detection**: Checks both `$LASTEXITCODE` and `$?` for comprehensive error detection

## Error Classification on Windows

The Windows hook recognizes these error patterns:

- `not recognized` → CommandNotFound
- `cannot find` → FileNotFoundOrDirectory  
- `No such file` → FileNotFoundOrDirectory
- `Access is denied` → PermissionDenied
- `cannot execute` → CannotExecute
- `invalid argument` / `invalid option` → InvalidArgumentOrOption
- `already exists` → ResourceExists
- `is not a directory` → NotADirectory

## File Locations

- **Hook File**: `$PROFILE` (typically `$env:USERPROFILE\Documents\PowerShell\Microsoft.PowerShell_profile.ps1`)
- **Config Directory**: `$env:USERPROFILE\.config\aish`
- **State Files**:
  - `last_command`: Sanitized command that was executed
  - `last_stdout`: Captured stdout (limited)
  - `last_stderr`: Captured error output
- **Binary Location**: `$env:USERPROFILE\bin\aish.exe` (default)

## Troubleshooting

### Common Issues

See [TROUBLESHOOTING.md](./TROUBLESHOOTING.md#windows-issues) for detailed Windows-specific troubleshooting.

Quick fixes:

```powershell
# Hook not working? Reload profile
. $PROFILE

# Command not found? Check PATH
$env:Path -split ';' | Select-String "bin"

# Verify hook is installed
Get-Content $PROFILE | Select-String "AISH"

# Check PowerShell version (need 5.1+)
$PSVersionTable.PSVersion

# Install PSReadLine for better capture
Install-Module -Name PSReadLine -Scope CurrentUser -Force
```

### Debug Mode

```powershell
# Enable debug output
$env:AISH_DEBUG = "1"

# Run a failing command
Get-Item C:\nonexistent

# Check captured error
Get-Content "$env:USERPROFILE\.config\aish\last_stderr"
```

## Future Improvements

Planned enhancements for Windows support:

1. **Transcript-Based Capture**: Implement `Start-Transcript` for better output capture
2. **Enhanced PSReadLine Integration**: Deeper integration with PSReadLine hooks
3. **Windows Terminal Integration**: Special features for Windows Terminal users
4. **ConPTY Support**: Better console output handling on Windows 10+
5. **PowerShell 7+ Features**: Utilize newer PowerShell features when available

## Contributing

Windows-specific contributions are welcome! Areas that need attention:

- Testing on different Windows versions (10, 11, Server)
- Testing with different PowerShell versions (5.1, 7.x)
- Improving output capture mechanisms
- Adding Windows-specific error patterns
- Performance optimizations for Windows

## References

- [PowerShell Documentation](https://docs.microsoft.com/en-us/powershell/)
- [PSReadLine Module](https://github.com/PowerShell/PSReadLine)
- [Windows Terminal](https://github.com/microsoft/terminal)
- [AISH Main Documentation](../README.md)
- [Troubleshooting Guide](./TROUBLESHOOTING.md)
