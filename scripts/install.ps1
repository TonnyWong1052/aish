# AISH Installation Script for Windows PowerShell
# This script builds and installs the 'aish' CLI tool on Windows

param(
    [switch]$WithInit = $false,
    [switch]$Help = $false
)

# --- Colors for output ---
function Write-Info {
    param([string]$Message)
    Write-Host "INFO: $Message" -ForegroundColor Blue
}

function Write-Success {
    param([string]$Message)
    Write-Host "SUCCESS: $Message" -ForegroundColor Green
}

function Write-Warning {
    param([string]$Message)
    Write-Host "WARNING: $Message" -ForegroundColor Yellow
}

function Write-ErrorMsg {
    param([string]$Message)
    Write-Host "ERROR: $Message" -ForegroundColor Red
}

# --- Help message ---
if ($Help) {
    Write-Host "Usage: .\install.ps1 [-WithInit] [-Help]"
    Write-Host "  -WithInit    Run 'aish init' after installation"
    Write-Host "  -Help        Show this help message"
    exit 0
}

# --- Configuration ---
$BinaryName = "aish.exe"
$BinarySource = ".\bin\$BinaryName"

# Potential installation directories (in order of preference)
$UserBinDir = Join-Path $env:USERPROFILE "bin"
$UserLocalBinDir = Join-Path $env:USERPROFILE ".local\bin"
$ProgramFilesDir = "$env:ProgramFiles\aish"

# --- Check if Go is installed ---
Write-Info "Checking for Go installation..."
try {
    $goVersion = & go version 2>$null
    if ($LASTEXITCODE -ne 0) {
        throw "Go not found"
    }
    Write-Success "Found Go: $goVersion"
} catch {
    Write-ErrorMsg "Go is not installed or not in PATH."
    Write-Info "Please install Go from https://go.dev/dl/ and try again."
    exit 1
}

# --- Build the binary ---
Write-Info "Building aish binary..."

# Create bin directory if it doesn't exist
if (-not (Test-Path ".\bin")) {
    New-Item -ItemType Directory -Path ".\bin" -Force | Out-Null
}

# Build the binary
try {
    & go build -o $BinarySource .\cmd\aish
    if ($LASTEXITCODE -ne 0) {
        throw "Build failed"
    }
    Write-Success "Binary built successfully at $BinarySource"
} catch {
    Write-ErrorMsg "Failed to build binary. Please check for compilation errors."
    exit 1
}

# Verify binary exists
if (-not (Test-Path $BinarySource)) {
    Write-ErrorMsg "Binary file not found after build: $BinarySource"
    exit 1
}

# --- Install the binary ---
$InstalledTo = $null
$UsedAdmin = $false

function Try-InstallBinary {
    param(
        [string]$TargetDir,
        [bool]$RequireAdmin = $false
    )
    
    if ([string]::IsNullOrWhiteSpace($TargetDir)) {
        return $false
    }
    
    Write-Info "Attempting to install '$BinaryName' to '$TargetDir'..."
    
    # Create directory if it doesn't exist (for user directories)
    if ($TargetDir.StartsWith($env:USERPROFILE)) {
        if (-not (Test-Path $TargetDir)) {
            try {
                New-Item -ItemType Directory -Path $TargetDir -Force | Out-Null
            } catch {
                Write-Warning "Failed to create directory: $TargetDir"
                return $false
            }
        }
    }
    
    $TargetPath = Join-Path $TargetDir $BinaryName
    
    # Try to copy the binary
    try {
        Copy-Item -Path $BinarySource -Destination $TargetPath -Force -ErrorAction Stop
        $script:InstalledTo = $TargetDir
        return $true
    } catch {
        if ($RequireAdmin) {
            Write-Warning "Installation to '$TargetDir' requires administrator privileges."
            Write-Info "Please run this script as Administrator or choose a user directory."
        } else {
            Write-Warning "Failed to copy binary to '$TargetDir': $_"
        }
        return $false
    }
}

# Try user directories first (no admin needed)
$installed = $false
foreach ($dir in @($UserBinDir, $UserLocalBinDir)) {
    if (Try-InstallBinary -TargetDir $dir -RequireAdmin $false) {
        $installed = $true
        break
    }
}

# If user directories failed, optionally try system directory
if (-not $installed) {
    Write-Warning "Could not install to user directories."
    $response = Read-Host "Try installing to Program Files (requires admin)? [y/N]"
    
    if ($response -match '^[Yy]$') {
        # Check if running as admin
        $isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
        
        if (-not $isAdmin) {
            Write-ErrorMsg "This script is not running with administrator privileges."
            Write-Info "Please run PowerShell as Administrator and try again."
            exit 1
        }
        
        if (Try-InstallBinary -TargetDir $ProgramFilesDir -RequireAdmin $true) {
            $installed = $true
            $UsedAdmin = $true
        }
    }
}

if (-not $installed) {
    Write-ErrorMsg "Failed to install binary to any location."
    Write-Host "Tried: $UserBinDir, $UserLocalBinDir, and optionally $ProgramFilesDir"
    Write-Host "You can manually copy '$BinarySource' to a directory in your PATH."
    exit 1
}

if ($UsedAdmin) {
    Write-Success "Binary installed to: $InstalledTo (with administrator privileges)"
} else {
    Write-Success "Binary installed to: $InstalledTo (no admin required)"
}

# --- Check if installation directory is in PATH ---
$pathDirs = $env:Path -split ';'
$inPath = $false
foreach ($dir in $pathDirs) {
    if ($dir -eq $InstalledTo) {
        $inPath = $true
        break
    }
}

if (-not $inPath) {
    Write-Warning "'$InstalledTo' is not in your PATH."
    Write-Info "To use 'aish' directly, you need to add it to your PATH."
    Write-Host ""
    Write-Host "To add it permanently, run this command in an administrator PowerShell:"
    Write-Host "  [Environment]::SetEnvironmentVariable('Path', `$env:Path + ';$InstalledTo', [EnvironmentVariableTarget]::User)" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Or add it to your PowerShell profile by adding this line:"
    Write-Host "  `$env:Path += ';$InstalledTo'" -ForegroundColor Yellow
    Write-Host ""
} else {
    Write-Info "'$InstalledTo' is already in your PATH."
}

# --- Run aish init if requested ---
if ($WithInit) {
    Write-Info "Running 'aish init' as requested (-WithInit)..."
    $aishPath = Join-Path $InstalledTo $BinaryName
    
    if (Test-Path $aishPath) {
        try {
            & $aishPath init
            if ($LASTEXITCODE -eq 0) {
                Write-Success "Init completed successfully."
            } else {
                Write-Warning "Init command failed. Please run 'aish init' manually later."
            }
        } catch {
            Write-Warning "Failed to run 'aish init': $_"
            Write-Info "Please run 'aish init' manually later."
        }
    } else {
        Write-Warning "Could not find aish binary at: $aishPath"
    }
} else {
    Write-Info "Skipping auto-run of 'aish init' during install."
    Write-Info "You can manually run 'aish init' later to install hooks and configure provider."
}

# --- Final Instructions ---
Write-Host ""
Write-Host "--- " -NoNewline
Write-Success "Installation Complete!"
Write-Host " ---"
Write-Info "Next steps:"
Write-Info "1. Restart your PowerShell terminal or run 'refreshenv' to update PATH."
Write-Info "2. Run 'aish init' to install the shell hook and configure provider."
Write-Info "3. You can adjust settings later with 'aish config'."
Write-Host ""
Write-Host "Enjoy using aish!" -ForegroundColor Cyan

exit 0
