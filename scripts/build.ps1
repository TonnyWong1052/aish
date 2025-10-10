# AISH Build Script for Windows PowerShell
# Builds the aish CLI tool

param(
    [string]$OutputDir = ".\bin",
    [string]$OutputName = "aish.exe",
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

function Write-ErrorMsg {
    param([string]$Message)
    Write-Host "ERROR: $Message" -ForegroundColor Red
}

# --- Help message ---
if ($Help) {
    Write-Host "Usage: .\build.ps1 [-OutputDir <dir>] [-OutputName <name>] [-Help]"
    Write-Host "  -OutputDir    Output directory for the binary (default: .\bin)"
    Write-Host "  -OutputName   Name of the output binary (default: aish.exe)"
    Write-Host "  -Help         Show this help message"
    exit 0
}

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

# --- Create output directory ---
if (-not (Test-Path $OutputDir)) {
    Write-Info "Creating output directory: $OutputDir"
    New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
}

# --- Build the binary ---
$OutputPath = Join-Path $OutputDir $OutputName
Write-Info "Building aish binary to: $OutputPath"

try {
    & go build -o $OutputPath .\cmd\aish
    if ($LASTEXITCODE -ne 0) {
        throw "Build failed with exit code $LASTEXITCODE"
    }
    Write-Success "Build completed successfully!"
    Write-Info "Binary location: $OutputPath"
} catch {
    Write-ErrorMsg "Failed to build binary: $_"
    exit 1
}

# --- Verify binary was created ---
if (-not (Test-Path $OutputPath)) {
    Write-ErrorMsg "Binary file was not created at expected location: $OutputPath"
    exit 1
}

$fileInfo = Get-Item $OutputPath
Write-Success "Binary size: $([math]::Round($fileInfo.Length / 1MB, 2)) MB"

exit 0
