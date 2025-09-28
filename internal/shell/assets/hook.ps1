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

# Load user preferences if present
$envFile = Join-Path $AISH_STATE_DIR "env.ps1"
if (Test-Path $envFile) { . $envFile }

# Decide whether to skip a command (interactive tools or user-installed commands)
function __aish_ShouldSkipCmd([string]$cmdLine) {
    if ([string]::IsNullOrWhiteSpace($cmdLine)) { return $false }
    $first = ($cmdLine.Trim() -split '\s+')[0]

    # Skip aish itself and known interactive tools
    switch -Wildcard ($first) {
        'aish*' { return $true }
        '*\aish*' { return $true }
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
            $wl = $DefaultWindowsSystemDirWhitelist
        }
        $dirs = $wl -split '[;:]'
        foreach ($d in $dirs) {
            if ([string]::IsNullOrWhiteSpace($d)) { continue }
            $prefix = ($d.TrimEnd('\\')) + '\\*'
            if ($resolved -like $prefix) { return $false }
        }
        return $true
    }

    return $false
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
                # Honor per-invocation bypass and skip rules
                if ($env:AISH_CAPTURE_OFF) { return " " }
                if (-not (__aish_ShouldSkipCmd $last_command)) {
                    # In PowerShell, output is captured via transcript, not redirection here.
                    # The 'aish capture' command will read from the transcript if needed.
                    aish capture $lastExitCode $last_command
                }
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