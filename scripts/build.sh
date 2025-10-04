#!/bin/bash

# This script builds the 'aish' CLI for multiple platforms.

# --- Configuration ---
OUTPUT_DIR="./dist"
BINARY_NAME="aish"
SOURCE_PATH="./cmd/aish"

# --- Colors for output ---
COLOR_GREEN='\033[0;32m'
COLOR_BLUE='\033[0;34m'
COLOR_NC='\033[0m' # No Color

# --- Helper Functions ---
function print_info() {
  echo -e "${COLOR_BLUE}INFO: $1${COLOR_NC}"
}

function print_success() {
  echo -e "${COLOR_GREEN}SUCCESS: $1${COLOR_NC}"
}

# --- Main Build Logic ---
print_info "Starting cross-platform build..."

# Clean previous builds
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# --- Build for Linux ---
print_info "Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -o "${OUTPUT_DIR}/${BINARY_NAME}-linux-amd64" "$SOURCE_PATH"

# --- Build for macOS ---
print_info "Building for macOS (amd64)..."
GOOS=darwin GOARCH=amd64 go build -o "${OUTPUT_DIR}/${BINARY_NAME}-darwin-amd64" "$SOURCE_PATH"

# --- Build for Windows ---
print_info "Building for Windows (amd64)..."
GOOS=windows GOARCH=amd64 go build -o "${OUTPUT_DIR}/${BINARY_NAME}-windows-amd64.exe" "$SOURCE_PATH"

print_success "All builds completed successfully."
print_info "Binaries are located in the '$OUTPUT_DIR' directory."

exit 0