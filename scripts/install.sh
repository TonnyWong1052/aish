#!/bin/bash

# This script installs the 'aish' CLI.

# --- Configuration ---
# Destination directory for the binary.
# Default is '$HOME/bin', a common practice for user-installed executables.
INSTALL_DIR="$HOME/bin"
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
      echo "  --with-init    Run 'aish init' after install (with fallback)"
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

# 2. Create the installation directory if it doesn't exist
if [ ! -d "$INSTALL_DIR" ]; then
  print_info "Creating installation directory at '$INSTALL_DIR'..."
  mkdir -p "$INSTALL_DIR"
fi

# 3. Copy the binary to the installation directory
print_info "Installing '$BINARY_NAME' from '$BINARY_SOURCE' to '$INSTALL_DIR'..."
cp "$BINARY_SOURCE" "$INSTALL_DIR/"
if [ $? -ne 0 ]; then
  print_warning "Failed to copy binary. You might need to run with sudo, or check permissions for '$INSTALL_DIR'."
  exit 1
fi
print_success "Binary installed successfully."

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
  "$INSTALL_DIR/$BINARY_NAME" init
  if [ $? -ne 0 ]; then
    print_warning "Init command failed. Falling back to 'hook install' and 'config'."
    "$INSTALL_DIR/$BINARY_NAME" hook install || true
    "$INSTALL_DIR/$BINARY_NAME" config || true
  fi
  print_success "Init/config steps executed."
else
  print_info "Skipping auto-run of 'aish init' during install."
  print_info "You can manually run 'aish init' later to install hooks and configure provider."
fi

# --- Final Instructions ---
echo -e "\n--- ${COLOR_GREEN}Installation Complete!${COLOR_NC} ---"
print_info "Next steps:"
print_info "1. ${COLOR_YELLOW}Restart your terminal${COLOR_NC} or source your shell config file (e.g., 'source ~/.zshrc')."
print_info "2. Optionally run '${COLOR_YELLOW}aish init${COLOR_NC}' to install the shell hook and configure provider, or run '${COLOR_YELLOW}aish hook install${COLOR_NC}' and '${COLOR_YELLOW}aish config${COLOR_NC}' separately."
echo -e "Enjoy using aish!"

exit 0
