#!/bin/bash

# This script installs the 'aish' CLI.

# --- Configuration ---
# Destination directory preference order (we will try in this order):
# 1) $HOME/bin (user directory, no sudo required)
# 2) $HOME/.local/bin (XDG standard, no sudo required)
# 3) /usr/local/bin (system-wide, may require sudo)
# 4) /opt/homebrew/bin (Homebrew on Apple Silicon, may require sudo)
INSTALL_DIR=""
# Name of the binary to install.
BINARY_NAME="aish"
# Source path of the binary. Prefer ./bin/aish if present.
BINARY_SOURCE="./bin/${BINARY_NAME}"

# --- Colors for output ---
COLOR_GREEN='\033[0;32m'
COLOR_YELLOW='\033[0;33m'
COLOR_BLUE='\033[0;34m'
COLOR_NC='\033[0m' # No Color

# --- Helper Functions ---
function print_info() {
  echo -e "${COLOR_BLUE}INFO: $1${COLOR_NC}"
}

function print_success() {
  echo -e "${COLOR_GREEN}SUCCESS: $1${COLOR_NC}"
}

function print_warning() {
  echo -e "${COLOR_YELLOW}WARNING: $1${COLOR_NC}"
}

# --- Argument Parsing ---
RUN_INIT=0

for arg in "$@"; do
  case "$arg" in
    --with-init)
      RUN_INIT=1
      ;;
    -h|--help)
      echo "Usage: $0 [--with-init]"
      echo "  --with-init    Run 'aish init' after install"
      exit 0
      ;;
    *)
      print_warning "Unknown option: $arg"
      ;;
  esac
done

# --- Main Installation Logic ---

# 1. Ensure we have a fresh binary in ./bin
if [ ! -f "$BINARY_SOURCE" ]; then
  print_warning "The '$BINARY_SOURCE' binary was not found."
  print_info "Building it now with 'go build -o ./bin/$BINARY_NAME ./cmd/aish'..."
  mkdir -p ./bin
  go build -o "./bin/$BINARY_NAME" ./cmd/aish
  if [ $? -ne 0 ]; then
    print_warning "Go build failed. Please fix any compilation errors and run this script again."
    exit 1
  fi
  print_success "Binary built successfully."
fi

# 2. Copy the binary to a preferred installation directory
INSTALLED_TO=""
USED_SUDO=false

try_install() {
  local target_dir="$1"
  local require_sudo="$2"  # "true" or "false"

  if [ -z "$target_dir" ]; then
    return 1
  fi

  print_info "Attempting to install '$BINARY_NAME' to '$target_dir'..."

  # Create directory if needed (for user directories)
  if [[ "$target_dir" == "$HOME"* ]]; then
    mkdir -p "$target_dir" 2>/dev/null || true
  fi

  # Try direct copy first
  if cp "$BINARY_SOURCE" "$target_dir/" 2>/dev/null; then
    INSTALLED_TO="$target_dir"
    return 0
  fi

  # Only try sudo for system directories if explicitly allowed
  if [ "$require_sudo" = "true" ] && command -v sudo >/dev/null 2>&1; then
    print_warning "This location requires administrator privileges."
    print_info "You can cancel (Ctrl+C) to avoid using sudo."
    if sudo cp "$BINARY_SOURCE" "$target_dir/" 2>/dev/null; then
      INSTALLED_TO="$target_dir"
      USED_SUDO=true
      return 0
    fi
  fi

  return 1
}

# Try user directories first (no sudo needed)
for candidate in "$HOME/bin" "$HOME/.local/bin"; do
  if try_install "$candidate" "false"; then
    INSTALL_DIR="$candidate"
    break
  fi
done

# If user directories failed, optionally try system directories
if [ -z "$INSTALLED_TO" ]; then
  print_warning "Could not install to user directories."
  print_info "Would you like to try system directories (requires sudo)?"

  read -r -p "Try system directories? [y/N]: " response
  if [[ "$response" =~ ^[Yy]$ ]]; then
    for candidate in "/usr/local/bin" "/opt/homebrew/bin"; do
      if try_install "$candidate" "true"; then
        INSTALL_DIR="$candidate"
        break
      fi
    done
  fi
fi

if [ -z "$INSTALLED_TO" ]; then
  print_warning "Failed to install binary to any location."
  echo "Tried: $HOME/bin, $HOME/.local/bin, and optionally /usr/local/bin, /opt/homebrew/bin"
  echo "You can manually copy './bin/$BINARY_NAME' to a directory in your PATH."
  exit 1
fi

if [ "$USED_SUDO" = "true" ]; then
  print_success "Binary installed to: $INSTALLED_TO (with sudo)"
else
  print_success "Binary installed to: $INSTALLED_TO (no sudo required)"
fi

# 4. Check if the installation directory is in the PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
  print_warning "'$INSTALL_DIR' is not in your PATH."
  print_info "To use '$BINARY_NAME' directly, you need to add it to your PATH."
  
  SHELL_CONFIG_FILE=""
  CURRENT_SHELL=$(basename "$SHELL")
  
  if [ "$CURRENT_SHELL" = "zsh" ]; then
    SHELL_CONFIG_FILE="$HOME/.zshrc"
  elif [ "$CURRENT_SHELL" = "bash" ]; then
    # Check for .bash_profile, .bash_login, then .profile
    if [ -f "$HOME/.bash_profile" ]; then
      SHELL_CONFIG_FILE="$HOME/.bash_profile"
    elif [ -f "$HOME/.bash_login" ]; then
      SHELL_CONFIG_FILE="$HOME/.bash_login"
    else
      SHELL_CONFIG_FILE="$HOME/.profile"
    fi
  else
    print_warning "Could not determine your shell config file. Please add the following line to it manually:"
  fi

  if [ -n "$SHELL_CONFIG_FILE" ]; then
    echo -e "\nPlease add the following line to your '$SHELL_CONFIG_FILE':"
    echo -e "  ${COLOR_YELLOW}export PATH=\"\$PATH:$INSTALL_DIR\"${COLOR_NC}"
    echo -e "\nAfter adding it, restart your terminal or run 'source $SHELL_CONFIG_FILE' for the changes to take effect."
  fi
else
  print_info "'$INSTALL_DIR' is already in your PATH."
fi

# 5. 視參數決定是否執行 init
if [ "$RUN_INIT" -eq 1 ]; then
  print_info "Running '$BINARY_NAME init' as requested (--with-init)..."
  if "$INSTALL_DIR/$BINARY_NAME" init; then
    print_success "Init completed successfully."
  else
    print_warning "Init command failed. Please run 'aish init' manually later."
  fi
else
  print_info "Skipping auto-run of 'aish init' during install."
  print_info "You can manually run 'aish init' later to install hooks and configure provider."
fi

# --- Final Instructions ---
echo -e "\n--- ${COLOR_GREEN}Installation Complete!${COLOR_NC} ---"
print_info "Next steps:"
print_info "1. ${COLOR_YELLOW}Restart your terminal${COLOR_NC} or source your shell config file (e.g., 'source ~/.zshrc')."
print_info "2. Optionally run '${COLOR_YELLOW}aish init${COLOR_NC}' to install the shell hook and configure provider. You can adjust settings later with '${COLOR_YELLOW}aish config${COLOR_NC}'."
echo -e "Enjoy using aish!"

exit 0
