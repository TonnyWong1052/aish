# AISH - AI Shell

**[English](#english) | [繁體中文](./README_zh.md) | [简体中文](./README_zh_cn.md) | [日本語](./README_ja.md)**

A powerful command-line tool that integrates with Google Gemini AI to provide intelligent terminal assistance. AISH automatically captures command errors, analyzes them with AI, and offers smart suggestions and corrections.

> Latest stable release: **v0.0.1**

![AISH Demo](./demo/demo.gif)

## Core Features & Architecture

AISH integrates with your shell environment and LLM providers to provide intelligent command assistance:

![AISH System Architecture](./demo/system_architecture.png)

### Key Features

-   **🤖 Intelligent Error Analysis**: Automatically captures command execution errors, provides AI-driven error classification and analysis, and offers intelligent correction suggestions with explanations.
-   **🔧 Multi-LLM Provider Support**: Supports various LLM providers, including OpenAI (GPT series), Google Gemini (Official API), and Gemini CLI (Cloud Code private API).
-   **📝 Natural Language Command Generation**: Generates shell commands from natural language prompts in English, Chinese, and Japanese. It also provides basic command suggestions in offline mode.
-   **📊 History Tracking and Replay**: Saves error analysis history, allows re-analysis of past errors, and includes a feature to clear the history.
-   **🎯 Smart Shell Hook**: Automatically integrates with your shell (bash/zsh) to capture command outputs and errors in real-time, providing seamless AI assistance without manual intervention.

### System Components

- **🔗 Shell Hook**: Automatically captures command execution and errors from your terminal
- **🧠 Error Classifier**: Intelligently categorizes different types of command failures
- **🤖 LLM Providers**: Multiple AI providers (OpenAI, Gemini, Gemini CLI) for intelligent analysis
- **📚 History Manager**: Persistent storage for command history and analysis results
- **⚙️ Configuration System**: Manages user preferences and provider settings
- **🛡️ Security Layer**: Automatically redacts sensitive information before AI analysis

## Installation & Configuration

### 1. Using Homebrew (Easiest)

If you're on macOS or Linux with Homebrew installed:

```bash
brew tap TonnyWong1052/aish
brew install aish
```

### 2. Using the Installation Script (Recommended)

The script will build the binary, install it into `~/bin`, and provide instructions for adding it to your `PATH`.

```bash
# Clone the repository
git clone https://github.com/TonnyWong1052/aish.git
cd aish

# Run the installation script
./scripts/install.sh

# To automatically run initialization (install hook + config) after installation
./scripts/install.sh --with-init
```

### 3. Manual Installation

If you prefer to build and install manually:

```bash
# 1. Build the application
go build -o aish ./cmd/aish

# 2. Move the binary to a directory in your PATH
mkdir -p ~/bin
mv aish ~/bin
```

### LLM Provider Configuration

After installation, configure AISH with your preferred LLM provider:

```bash
# Initialize AISH configuration
aish init
```

#### 🚀 Gemini CLI (Recommended)
- **Free access** to Google's Gemini models
- **No API key required** (uses your Google account authentication)
- **Higher rate limits** compared to official API

```bash
# Install Gemini CLI: https://github.com/google/generative-ai-cli
aish init  # Select "gemini-cli" when prompted
```

#### 🔑 Alternative: Official Gemini API
```bash
# Get API key: https://aistudio.google.com/app/apikey
aish init  # Select "gemini" and enter your API key
```

#### 🤖 OpenAI GPT (Alternative)
```bash
aish init  # Select "openai" and enter your API key
```

The setup wizard will guide you through provider selection, API key setup, and shell hook installation.

## 🎯 Shell Hook - The Magic Behind AISH

The **Shell Hook** is the core component that makes AISH truly intelligent and seamless. It automatically integrates with your shell environment to provide real-time AI assistance without any manual intervention.

### What the Hook Does

The Shell Hook automatically:

- **🔍 Captures Command Output**: Monitors both stdout and stderr from every command you run
- **🚨 Detects Errors**: Intelligently identifies when commands fail (non-zero exit codes)
- **🛡️ Filters Noise**: Skips user-initiated interruptions (Ctrl+C, Ctrl+\) and AISH's own commands
- **🔒 Sanitizes Sensitive Data**: Automatically redacts API keys, tokens, passwords, and other sensitive information before sending to AI
- **⚡ Triggers AI Analysis**: Automatically calls AISH when errors are detected, providing instant feedback

### Supported Shells

- **🐚 Bash**: Full integration with command interception
- **🐚 Zsh**: Seamless integration with native hooks
- **🪟 PowerShell**: Windows environment support

### Security Features

- **🔐 Automatic Redaction**: Sensitive parameters like `--api-key`, `--token`, `--password` are automatically masked
- **🛡️ Environment Variable Protection**: Variables containing `SECRET`, `TOKEN`, `PASSWORD`, `API_KEY` are redacted
- **🚫 Self-Protection**: Prevents infinite loops by ignoring AISH's own commands
- **📁 Secure Storage**: All temporary files are stored in `~/.config/aish/` with proper permissions

### Advanced Configuration

Skip specific commands or user-installed tools:

```bash
# Skip specific command patterns
export AISH_SKIP_COMMAND_PATTERNS="claude gh* fzf"

# Skip all user-installed commands (Homebrew/npm/pipx/etc.)
export AISH_SKIP_ALL_USER_COMMANDS=1

# One-off bypass
AISH_CAPTURE_OFF=1 <your-command>
```

The hook is automatically installed when you run `aish init` and modifies your shell configuration files.

### 🏷️ Error Classification System

The Hook includes an intelligent error classification system that categorizes different types of command failures for more targeted AI analysis:

#### **CommandNotFound** 🔍
- **Trigger**: `command not found` errors
- **Examples**: 
  ```bash
  $ unknowncmd
  bash: unknowncmd: command not found
  ```
- **AI Response**: Suggests similar commands, installation instructions, or typo corrections

#### **FileNotFoundOrDirectory** 📁
- **Trigger**: `No such file or directory` errors
- **Examples**:
  ```bash
  $ cat /nonexistent/file
  cat: /nonexistent/file: No such file or directory
  ```
- **AI Response**: Suggests correct file paths, directory listings, or file creation

#### **PermissionDenied** 🔒
- **Trigger**: `Permission denied` errors
- **Examples**:
  ```bash
  $ cat /root/secret
  cat: /root/secret: Permission denied
  ```
- **AI Response**: Suggests permission fixes, sudo usage, or alternative approaches

#### **CannotExecute** ⚠️
- **Trigger**: `cannot execute binary file` errors
- **Examples**:
  ```bash
  $ ./script
  bash: ./script: cannot execute binary file
  ```
- **AI Response**: Suggests making files executable, checking file types, or interpreter issues

#### **InvalidArgumentOrOption** ❌
- **Trigger**: `invalid argument` or `invalid option` errors
- **Examples**:
  ```bash
  $ ls -Z
  ls: invalid option -- 'Z'
  ```
- **AI Response**: Suggests correct command syntax, available options, or usage examples

#### **ResourceExists** 📄
- **Trigger**: `File exists` errors
- **Examples**:
  ```bash
  $ mkdir /tmp/test
  mkdir: /tmp/test: File exists
  ```
- **AI Response**: Suggests overwrite options, different names, or removal strategies

#### **NotADirectory** 📂
- **Trigger**: `is not a directory` errors
- **Examples**:
  ```bash
  $ cd /etc/passwd
  cd: /etc/passwd: is not a directory
  ```
- **AI Response**: Suggests correct directory paths or file vs directory operations

#### **TerminatedBySignal** ⏹️
- **Trigger**: Exit codes > 128 (signal termination)
- **Examples**:
  ```bash
  $ long-running-command
  ^C  # Ctrl+C interruption (exit code 130)
  ```
- **AI Response**: Explains signal termination, suggests resuming or alternative approaches

#### **GenericError** 🔧
- **Trigger**: All other error types
- **Examples**: Custom application errors, network issues, etc.
- **AI Response**: General troubleshooting advice and context-specific solutions

### 🎯 Classification Benefits

- **🎯 Targeted Responses**: Each error type receives specialized AI analysis
- **📚 Learning Context**: AI understands the specific nature of each failure
- **⚡ Faster Resolution**: More accurate suggestions based on error category
- **🔄 Consistent Handling**: Standardized approach to common error patterns


## Usage & Examples

### 🚨 Automatic Error Analysis
When you make a mistake, AISH's Shell Hook automatically captures the error and provides intelligent feedback:

```bash
$ ls /nonexistent
ls: cannot access '/nonexistent': No such file or directory

🧠 AISH Analysis:
┌─ Error Explanation ─────────────────────────────────────────┐
│ The 'ls' command failed because the path '/nonexistent'    │
│ does not exist on your filesystem.                         │
└────────────────────────────────────────────────────────────┘

💡 Suggestion: Check if the directory path is correct.
   You can use 'ls /' to see the contents of the root directory.

🔧 Corrected Command:
   ls /

Press [Enter] to run the corrected command, or any other key to dismiss.
```

### 🤖 Natural Language Command Generation
Generate shell commands from plain English:

```bash
$ aish -p "find all .go files in the current directory"
# AISH generates: find . -name "*.go"
```

### 📊 History and Replay
Review and re-analyze past errors:

```bash
$ aish history
📋 Recent Error Analysis History:
   1. [2 min ago] ls /nonexistent - File not found
   2. [15 min ago] git push origin main - Authentication failed
   3. [1 hour ago] docker run nginx - Port already in use
```

## Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
