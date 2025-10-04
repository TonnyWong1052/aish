# AISH Troubleshooting Guide

This guide helps you diagnose and resolve common issues with AISH (AI Shell). If you can't find a solution here, please check our [GitHub Issues](https://github.com/TonnyWong1052/aish/issues) or create a new issue.

## Table of Contents

- [Installation Issues](#installation-issues)
- [Configuration Problems](#configuration-problems)
- [LLM Provider Issues](#llm-provider-issues)
- [Shell Hook Problems](#shell-hook-problems)
- [Performance Issues](#performance-issues)
- [Security and Privacy](#security-and-privacy)
- [Platform-Specific Issues](#platform-specific-issues)
- [Advanced Debugging](#advanced-debugging)

## Installation Issues

### Issue: Command not found after installation

**Symptoms:**
```bash
$ aish
bash: aish: command not found
```

**Solutions:**

1. **Check if binary is in PATH:**
```bash
# Check if aish is installed
which aish

# If installed via script, check ~/bin
ls ~/bin/aish

# Add to PATH if needed
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

2. **Verify installation method:**
```bash
# If using Homebrew
brew list aish

# If using manual installation
ls -la ~/bin/aish
chmod +x ~/bin/aish
```

3. **Reinstall if necessary:**
```bash
# Remove existing installation
rm ~/bin/aish

# Reinstall using script
curl -sSL https://raw.githubusercontent.com/TonnyWong1052/aish/main/scripts/install.sh | bash
```

### Issue: Permission denied during installation

**Symptoms:**
```bash
install.sh: Permission denied
```

**Solutions:**

1. **Make script executable:**
```bash
chmod +x install.sh
./install.sh
```

2. **Use curl with pipe:**
```bash
curl -sSL https://raw.githubusercontent.com/TonnyWong1052/aish/main/scripts/install.sh | bash
```

3. **Manual installation:**
```bash
# Download and build manually
git clone https://github.com/TonnyWong1052/aish.git
cd aish
go build -o aish ./cmd/aish
mkdir -p ~/bin
mv aish ~/bin/
```

### Issue: Go build failures

**Symptoms:**
```bash
go: requires go version >= 1.23.0
```

**Solutions:**

1. **Update Go version:**
```bash
# Check current Go version
go version

# Install latest Go from https://golang.org/dl/
# Or use version manager like g
curl -sSL https://git.io/g-install | sh
g install latest
```

2. **Use alternative installation:**
```bash
# Use pre-built binaries instead
wget https://github.com/TonnyWong1052/aish/releases/latest/download/aish-linux-amd64
chmod +x aish-linux-amd64
mv aish-linux-amd64 ~/bin/aish
```

## Configuration Problems

### Issue: AISH not starting after installation

**Symptoms:**
```bash
$ aish init
Error: Failed to load config
```

**Solutions:**

1. **Check configuration directory:**
```bash
# Check if config directory exists
ls -la ~/.config/aish/

# Create if missing
mkdir -p ~/.config/aish/

# Check permissions
chmod 755 ~/.config/aish/
```

2. **Reset configuration:**
```bash
# Backup existing config
mv ~/.config/aish/config.json ~/.config/aish/config.json.backup

# Reinitialize
aish init
```

3. **Check configuration syntax:**
```bash
# Validate JSON syntax
cat ~/.config/aish/config.json | jq .

# If jq not available, check manually
cat ~/.config/aish/config.json
```

### Issue: Configuration corruption

**Symptoms:**
```bash
Error: invalid character '}' looking for beginning of object key string
```

**Solutions:**

1. **Restore from backup:**
```bash
# Check for backup files
ls ~/.config/aish/*.backup

# Restore if available
cp ~/.config/aish/config.json.backup ~/.config/aish/config.json
```

2. **Create minimal configuration:**
```bash
# Create basic config
cat > ~/.config/aish/config.json << 'EOF'
{
    "default_provider": "gemini-cli",
    "providers": {},
    "user_preferences": {
        "language": "en",
        "enabled_llm_triggers": ["CommandNotFound", "FileNotFoundOrDirectory", "PermissionDenied"]
    }
}
EOF
```

3. **Reconfigure provider:**
```bash
aish config set default_provider gemini-cli
aish init
```

## LLM Provider Issues

### Issue: Gemini CLI authentication failed

**Symptoms:**
```bash
Error: Gemini CLI verification failed: authentication failed
```

**Solutions:**

1. **Check Gemini CLI installation:**
```bash
# Verify gemini-cli is installed
which gemini-cli

# Install if missing
# Follow: https://github.com/google/generative-ai-cli
```

2. **Re-authenticate with Google:**
```bash
# Login to Google account
gemini-cli auth login

# Verify authentication
gemini-cli auth list
```

3. **Check project configuration:**
```bash
# Verify project ID
aish config get providers.gemini-cli.project

# Set correct project if needed
aish config set providers.gemini-cli.project "your-project-id"
```

4. **Test connection manually:**
```bash
# Test Gemini CLI directly
echo "Hello" | gemini-cli p

# Debug AISH connection
AISH_DEBUG_GEMINI=1 aish -p "test prompt"
```

### Issue: OpenAI API key invalid

**Symptoms:**
```bash
Error: OpenAI request failed: invalid API key
```

**Solutions:**

1. **Verify API key format:**
```bash
# Check API key format (should start with sk-)
aish config get providers.openai.api_key
# Should show: sk-***MASKED***
```

2. **Update API key:**
```bash
# Get new API key from: https://platform.openai.com/api-keys
aish config set providers.openai.api_key "sk-your-new-key"
```

3. **Test API key directly:**
```bash
# Test with curl
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer sk-your-key" \
  -H "Content-Type: application/json"
```

4. **Check rate limits:**
```bash
# Check if rate limited
curl -I https://api.openai.com/v1/models \
  -H "Authorization: Bearer sk-your-key"
```

### Issue: Network connectivity problems

**Symptoms:**
```bash
Error: request failed: dial tcp: lookup api.openai.com: no such host
```

**Solutions:**

1. **Check internet connection:**
```bash
# Test basic connectivity
ping google.com

# Test specific API endpoints
ping api.openai.com
ping generativelanguage.googleapis.com
```

2. **Check proxy settings:**
```bash
# Check proxy environment variables
echo $http_proxy
echo $https_proxy

# Configure if behind corporate proxy
export https_proxy=http://proxy.company.com:8080
```

3. **Test with different provider:**
```bash
# Switch to Gemini CLI (offline-capable)
aish config set default_provider gemini-cli
```

4. **Check firewall settings:**
```bash
# Test API access directly
curl -v https://api.openai.com/v1/models
```

## Shell Hook Problems

### Issue: Hook not triggering

**Symptoms:**
- Commands fail but AISH doesn't analyze them
- No automatic AI assistance

**Solutions:**

1. **Check hook installation:**
```bash
# For bash users
grep -n "aish" ~/.bashrc ~/.bash_profile

# For zsh users
grep -n "aish" ~/.zshrc

# Reinstall hook if missing (runs hook installer)
aish init
```

2. **Reload shell configuration:**
```bash
# For bash
source ~/.bashrc

# For zsh
source ~/.zshrc

# Or restart terminal
```

3. **Check hook function:**
```bash
# Verify hook function exists (bash)
declare -f aish_capture

# Verify hook function exists (zsh)
which aish_capture
```

4. **Manual hook verification:**
```bash
# Test hook manually
aish capture 1 "test command"
```

### Issue: Hook causing shell slowdown

**Symptoms:**
- Terminal feels slow or unresponsive
- Delayed command execution

**Solutions:**

1. **Disable hook temporarily:**
```bash
export AISH_CAPTURE_OFF=1
```

2. **Skip problematic commands:**
```bash
# Skip specific commands
export AISH_SKIP_COMMAND_PATTERNS="slow_command other_command"

# Skip all user-installed commands
export AISH_SKIP_ALL_USER_COMMANDS=1
```

3. **Check hook configuration:**
```bash
# View current hook settings
aish config show

# Reduce enabled triggers
aish config set user_preferences.enabled_llm_triggers '["CommandNotFound"]'
```

4. **Debug hook performance:**
```bash
# Enable debug mode
export AISH_DEBUG=1

# Run commands and check timing
time ls /nonexistent
```

### Issue: Hook conflicts with other tools

**Symptoms:**
- Interactive tools (like `fzf`, `vim`) behave strangely
- Output redirection issues

**Solutions:**

1. **Add problematic tools to skip list:**
```bash
export AISH_SKIP_COMMAND_PATTERNS="fzf vim nvim emacs"
```

2. **Disable hook for specific sessions:**
```bash
# Temporary disable
export AISH_CAPTURE_OFF=1

# Run problematic command
fzf

# Re-enable
unset AISH_CAPTURE_OFF
```

3. **Check TTY handling:**
```bash
# Verify TTY availability
tty

# Check if running in proper terminal
echo $TERM
```

## Performance Issues

### Issue: Slow AI responses

**Symptoms:**
- AISH takes long time to respond
- Timeout errors

**Solutions:**

1. **Check network latency:**
```bash
# Test API response time
time curl -s https://api.openai.com/v1/models >/dev/null
```

2. **Switch to faster provider:**
```bash
# Use Gemini CLI (often faster)
aish config set default_provider gemini-cli

# Or use different model
aish config set providers.openai.model "gpt-3.5-turbo"
```

3. **Enable caching:**
```bash
# Check if caching is enabled
aish config get cache.enabled

# Enable caching
aish config set cache.enabled true
aish config set cache.ttl 3600
```

4. **Reduce context size:**
```bash
# Limit history context
aish config set context.max_history_entries 5
aish config set context.max_output_length 1000
```

### Issue: High memory usage

**Symptoms:**
- System running out of memory
- AISH process using excessive RAM

**Solutions:**

1. **Check memory usage:**
```bash
# Monitor AISH processes
ps aux | grep aish

# Check system memory
free -h
```

2. **Clear cache:**
```bash
# Clear AISH cache
rm -rf ~/.config/aish/cache/*

# Restart AISH
```

3. **Reduce cache size:**
```bash
# Limit cache size
aish config set cache.max_size 100
aish config set cache.max_entries 50
```

4. **Disable features:**
```bash
# Disable analytics
aish config set analytics.enabled false

# Disable prewarming
aish config set cache.prewarming.enabled false
```

## Security and Privacy

### Issue: Sensitive data in logs

**Symptoms:**
- API keys visible in debug output
- Personal information in error messages

**Solutions:**

1. **Check sanitization settings:**
```bash
# Verify sanitization is enabled
aish config get security.enable_sanitization

# Enable if disabled
aish config set security.enable_sanitization true
```

2. **Clear sensitive logs:**
```bash
# Remove debug logs
rm -f ~/.config/aish/debug.log

# Clear shell history if needed
history -c
```

3. **Test sanitization:**
```bash
# Test with fake sensitive data
echo "api_key=test123" | aish -p "analyze this error"
# Should show: api_key=***REDACTED***
```

4. **Report sanitization gaps:**
If you find unsanitized sensitive data, please report it as a security issue.

### Issue: Encrypted data corruption

**Symptoms:**
```bash
Error: failed to decrypt API key: cipher: message authentication failed
```

**Solutions:**

1. **Reset encryption:**
```bash
# Backup config
cp ~/.config/aish/config.json ~/.config/aish/config.json.backup

# Remove encryption key
rm ~/.config/aish/.secret_key

# Reconfigure
aish init
```

2. **Manual decryption:**
```bash
# Check if encryption key exists
ls -la ~/.config/aish/.secret_key

# Reset if corrupted
rm ~/.config/aish/.secret_key
aish config set providers.openai.api_key "your-key"
```

## Platform-Specific Issues

### macOS Issues

**Issue: Homebrew installation fails**
```bash
# Update Homebrew
brew update

# Force install
brew install --force TonnyWong1052/aish/aish

# Check for conflicts
brew doctor
```

**Issue: Gatekeeper blocking execution**
```bash
# Allow unsigned binary
xattr -d com.apple.quarantine ~/bin/aish

# Or build from source
go build -o aish ./cmd/aish
```

### Linux Issues

**Issue: Missing dependencies**
```bash
# Install missing packages (Ubuntu/Debian)
sudo apt-get update
sudo apt-get install curl ca-certificates

# For other distributions, use appropriate package manager
```

**Issue: AppArmor/SELinux restrictions**
```bash
# Check SELinux status
sestatus

# Temporarily disable if needed (NOT recommended for production)
sudo setenforce 0
```

### Windows Issues

**Issue: PowerShell execution policy**
```powershell
# Check current policy
Get-ExecutionPolicy

# Allow script execution
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

**Issue: Windows Defender blocking**
```powershell
# Add exclusion for AISH directory
Add-MpPreference -ExclusionPath "$env:USERPROFILE\bin"
```

## Advanced Debugging

### Enable Debug Mode

1. **Environment variables:**
```bash
# Enable general debug mode
export AISH_DEBUG=1

# Enable Gemini-specific debugging
export AISH_DEBUG_GEMINI=1

# Enable network debugging
export AISH_DEBUG_NETWORK=1
```

2. **Configuration debugging:**
```bash
# Show detailed config
aish config show --debug

# Validate configuration
aish config validate
```

3. **Provider debugging:**
```bash
# Test provider connectivity
aish verify-connection

# Test with minimal request
echo "test" | aish -p "simple test" --debug
```

### Collect Diagnostic Information

1. **System information:**
```bash
# AISH version
aish version

# System information
uname -a

# Shell information
echo $SHELL
echo $0
```

2. **Configuration dump:**
```bash
# Export sanitized config
aish config export --sanitized > aish-config.json
```

3. **Log analysis:**
```bash
# Check recent logs
tail -100 ~/.config/aish/debug.log

# Search for errors
grep -i error ~/.config/aish/debug.log
```

### Performance Profiling

1. **Response time measurement:**
```bash
# Measure command generation time
time aish -p "list files"

# Measure analysis time
time aish capture 1 "ls /nonexistent"
```

2. **Memory profiling:**
```bash
# Monitor memory usage
valgrind --tool=memcheck aish -p "test"

# Or use built-in profiling
GODEBUG=gctrace=1 aish -p "test"
```

### Creating Bug Reports

When reporting issues, please include:

1. **Environment information:**
   - OS and version
   - Shell type and version
   - AISH version
   - Go version (if building from source)

2. **Configuration:**
   - Sanitized config (`aish config export --sanitized`)
   - Provider being used
   - Any custom environment variables

3. **Steps to reproduce:**
   - Exact commands run
   - Expected behavior
   - Actual behavior

4. **Debug output:**
   - Output with `AISH_DEBUG=1`
   - Relevant log entries
   - Error messages

5. **Sample data:**
   - Example commands that trigger the issue
   - Sanitized error outputs

### Getting Help

If you can't resolve an issue:

1. **Search existing issues:** [GitHub Issues](https://github.com/TonnyWong1052/aish/issues)
2. **Check discussions:** [GitHub Discussions](https://github.com/TonnyWong1052/aish/discussions)
3. **Create new issue:** Use the appropriate issue template
4. **Community support:** Join our Discord/Slack (if available)

Please provide as much detail as possible when seeking help, including the diagnostic information mentioned above.
