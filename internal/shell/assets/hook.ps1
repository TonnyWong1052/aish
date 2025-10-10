# AISH (AI Shell) Hook - Start

# State file locations
if (-not (Test-Path Env:AISH_STATE_DIR)) {
    $env:AISH_STATE_DIR = "$env:USERPROFILE\.config\aish"
}
$Global:AISH_STATE_DIR = $env:AISH_STATE_DIR
$Global:AISH_STDOUT_FILE = Join-Path $AISH_STATE_DIR "last_stdout"
$Global:AISH_STDERR_FILE = Join-Path $AISH_STATE_DIR "last_stderr"
$Global:AISH_LAST_CMD_FILE = Join-Path $AISH_STATE_DIR "last_command"

if (-not (Test-Path $AISH_STATE_DIR)) {
    New-Item -ItemType Directory -Path $AISH_STATE_DIR -Force | Out-Null
}

# Load user preferences if present
$envFile = Join-Path $AISH_STATE_DIR "env.ps1"
if (Test-Path $envFile) { . $envFile }

# Default: skip all user-installed commands (can override with AISH_SKIP_ALL_USER_COMMANDS=0)
if (-not $env:AISH_SKIP_ALL_USER_COMMANDS) { $env:AISH_SKIP_ALL_USER_COMMANDS = '1' }

# Default Windows system directory whitelist
$Global:DefaultWindowsSystemDirWhitelist = "C:\Windows\System32;C:\Windows;C:\Windows\SysWOW64;C:\Program Files\PowerShell\7;C:\Windows\System32\WindowsPowerShell\v1.0"

# Sanitize command by masking sensitive information
function Global:__aish_SanitizeCmd([string]$cmd) {
    if ([string]::IsNullOrWhiteSpace($cmd)) { return $cmd }
    
    # Mask flag form --key=VALUE
    $cmd = $cmd -replace '--(api[_-]?key|token|password|passwd|secret|bearer)=([^\s]+)', '--$1=***REDACTED***'
    # Mask flag form --key VALUE
    $cmd = $cmd -replace '--(api[_-]?key|token|password|passwd|secret|bearer)\s+([^\s]+)', '--$1 ***REDACTED***'
    # Mask environment variable form FOO_TOKEN=VALUE
    $cmd = $cmd -replace '([A-Za-z_][A-Za-z0-9_]*((SECRET)|(TOKEN)|(PASSWORD)|(API[_-]?KEY)|(ACCESS[_-]?KEY)|(BEARER))[A-Za-z0-9_]*)=([^\s]+)', '$1=***REDACTED***'
    
    return $cmd
}

# Check if error should trigger AISH analysis
function Global:__aish_ShouldTrigger([int]$exitCode) {
    # Only consider non-zero exit codes
    if ($exitCode -eq 0) { return $false }
    
    # Skip user-initiated cancellation (Ctrl+C typically returns exit code 1 in PowerShell)
    # We rely on stderr content to determine if it's a real error
    
    # If stderr doesn't exist or is empty, still allow reporting (conservative mode)
    if (-not (Test-Path $Global:AISH_STDERR_FILE) -or (Get-Item $Global:AISH_STDERR_FILE).Length -eq 0) {
        return $true
    }
    
    # Check for common error patterns
    $stderrContent = Get-Content $Global:AISH_STDERR_FILE -Raw -ErrorAction SilentlyContinue
    if ($stderrContent -match '(not recognized|cannot find|No such file|Access is denied|cannot execute|invalid (argument|option)|already exists|is not a directory)') {
        return $true
    }
    
    return $true
}

# Decide whether to skip a command (interactive tools or user-installed commands)
function Global:__aish_ShouldSkipCmd([string]$cmdLine) {
    if ([string]::IsNullOrWhiteSpace($cmdLine)) { return $false }
    $first = ($cmdLine.Trim() -split '\s+')[0]

    # Skip aish itself and known interactive tools
    switch -Wildcard ($first) {
        'aish*' { return $true }
        '*\aish*' { return $true }
        'aish.exe*' { return $true }
        '*\aish.exe*' { return $true }
        'claude' { return $true }
        '*\claude' { return $true }
        'npm' { return $true }
        '*\npm' { return $true }
        'npx' { return $true }
        '*\npx' { return $true }
        'yarn' { return $true }
        '*\yarn' { return $true }
        'pnpm' { return $true }
        '*\pnpm' { return $true }
        'brew' { return $true }
        '*\brew' { return $true }
        default {}
    }

    # User-defined skip patterns (whitespace separated globs)
    if ($env:AISH_SKIP_COMMAND_PATTERNS) {
        $patterns = [regex]::Split($env:AISH_SKIP_COMMAND_PATTERNS, '\s+')
        foreach ($p in $patterns) {
            if ($first -like $p -or $cmdLine -like $p) { return $true }
        }
    }

    # Skip all user-installed commands when enabled
    if ($env:AISH_SKIP_ALL_USER_COMMANDS -eq '1') {
        $resolved = $null
        try { $resolved = (Get-Command $first -ErrorAction Stop).Path } catch {}
        if (-not $resolved) { return $false } # builtins/aliases/functions → treat as system

        $wl = $env:AISH_SYSTEM_DIR_WHITELIST
        if ([string]::IsNullOrWhiteSpace($wl)) {
            $wl = $Global:DefaultWindowsSystemDirWhitelist
        }
        $dirs = $wl -split '[;:]'
        foreach ($d in $dirs) {
            if ([string]::IsNullOrWhiteSpace($d)) { continue }
            $prefix = ($d.TrimEnd('\')) + '\*'
            if ($resolved -like $prefix) { return $false }
        }
        return $true
    }

    return $false
}

# Check if PSReadLine is available for better command capture
$Global:AISH_USE_PSREADLINE = $null -ne (Get-Module -Name PSReadLine -ListAvailable)

# Capture command execution using PSReadLine (if available) or prompt hook
if ($Global:AISH_USE_PSREADLINE -and -not $env:AISH_HOOK_DISABLED) {
    # Use PSReadLine's AddToHistory event for better command capture
    Set-PSReadLineOption -AddToHistoryHandler {
        param([string]$command)
        
        # Store the command for later analysis
        if (-not [string]::IsNullOrWhiteSpace($command)) {
            $sanitized = __aish_SanitizeCmd $command
            Set-Content -Path $Global:AISH_LAST_CMD_FILE -Value $sanitized -NoNewline -ErrorAction SilentlyContinue
            
            # Clear previous stdout/stderr
            Set-Content -Path $Global:AISH_STDOUT_FILE -Value "" -NoNewline -ErrorAction SilentlyContinue
            Set-Content -Path $Global:AISH_STDERR_FILE -Value "" -NoNewline -ErrorAction SilentlyContinue
        }
        
        return $true  # Add to history
    }
}

# Override prompt to capture command results
if (-not $env:AISH_HOOK_DISABLED) {
    # Save original prompt if not already saved
    if (-not (Test-Path Function:__aish_original_prompt)) {
        if (Test-Path Function:prompt) {
            Copy-Item Function:prompt Function:__aish_original_prompt
        } else {
            function Global:__aish_original_prompt { "PS $($executionContext.SessionState.Path.CurrentLocation)$('>' * ($nestedPromptLevel + 1)) " }
        }
    }

    function Global:prompt {
        # Capture the last exit code
        $lastExitCode = $LASTEXITCODE
        $lastSuccess = $?
        
        # Determine effective exit code (PowerShell doesn't always set $LASTEXITCODE)
        $effectiveExitCode = if ($lastExitCode) { $lastExitCode } elseif (-not $lastSuccess) { 1 } else { 0 }
        
        # Get the last command from history
        $lastCommand = $null
        $history = Get-History -Count 1 -ErrorAction SilentlyContinue
        if ($history) {
            $lastCommand = $history.CommandLine
        }
        
        # Analyze errors if conditions are met
        if ($effectiveExitCode -ne 0 -and $lastCommand) {
            # Check if we should skip this command
            if (-not (__aish_ShouldSkipCmd $lastCommand)) {
                # Check if error should trigger analysis
                if (__aish_ShouldTrigger $effectiveExitCode) {
                    # Honor per-invocation bypass
                    if (-not $env:AISH_CAPTURE_OFF) {
                        # Check if aish is available
                        if (Get-Command aish -ErrorAction SilentlyContinue) {
                            # Capture stderr from error stream (PowerShell-specific)
                            # Note: PowerShell's error stream is separate from stderr
                            $errorOutput = $Error[0] | Out-String
                            if (-not [string]::IsNullOrWhiteSpace($errorOutput)) {
                                Add-Content -Path $Global:AISH_STDERR_FILE -Value $errorOutput -ErrorAction SilentlyContinue
                            }
                            
                            # Save sanitized command
                            $sanitized = __aish_SanitizeCmd $lastCommand
                            Set-Content -Path $Global:AISH_LAST_CMD_FILE -Value $sanitized -NoNewline -ErrorAction SilentlyContinue
                            
                            # Set environment variables for aish to read
                            $env:AISH_STDOUT_FILE = $Global:AISH_STDOUT_FILE
                            $env:AISH_STDERR_FILE = $Global:AISH_STDERR_FILE
                            
                            # Call aish capture
                            try {
                                & aish capture $effectiveExitCode $sanitized 2>$null
                            } catch {
                                # Silently ignore aish errors
                            }
                        }
                    }
                }
            }
        }
        
        # Call original prompt
        & __aish_original_prompt
        
        # Restore LASTEXITCODE so it's available to the user
        $global:LASTEXITCODE = $lastExitCode
    }
}

# AISH (AI Shell) Hook - End
