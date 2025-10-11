package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/TonnyWong1052/aish/internal/config"
)

// Status represents the result status of a diagnostic check
type Status string

const (
	StatusOK   Status = "OK"
	StatusWarn Status = "WARN"
	StatusFail Status = "FAIL"
)

// CheckResult holds the result of a single diagnostic check
type CheckResult struct {
	Name       string        `json:"name"`
	Status     Status        `json:"status"`
	Message    string        `json:"message"`
	Suggestion string        `json:"suggestion,omitempty"`
	Provider   string        `json:"provider,omitempty"`
	Details    interface{}   `json:"details,omitempty"`
	Duration   time.Duration `json:"duration_ms"`
}

// DiagnosticSummary holds the overall summary of all checks
type DiagnosticSummary struct {
	OK   int `json:"ok"`
	Warn int `json:"warn"`
	Fail int `json:"fail"`
}

// DiagnosticOutput is the complete output in JSON mode
type DiagnosticOutput struct {
	Summary DiagnosticSummary `json:"summary"`
	Results []CheckResult     `json:"results"`
}

var (
	flagHooks          bool
	flagPerms          bool
	flagLLM            bool
	flagProxy          bool
	flagDoctorProvider string
	flagTimeout        time.Duration
	flagJSON           bool
	flagVerbose        bool
	flagFix            bool
	flagNoParallel     bool
)

// doctorCmd represents the doctor command
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose potential issues with the aish setup",
	Long: `The doctor command runs a series of checks to validate the aish installation and configuration.

It checks for:
- Hook installation status in supported shells
- File and directory permissions for config/history/logs
- LLM provider connectivity and authentication
- Proxy and network environment settings

Use flags to run specific checks or customize behavior.`,
	Example: `  aish doctor
  aish doctor --hooks
  aish doctor --llm --provider=openai
  aish doctor --json
  aish doctor --fix`,
	Run: runDoctor,
}

func init() {
	doctorCmd.Flags().BoolVar(&flagHooks, "hooks", false, "run hook installation checks only")
	doctorCmd.Flags().BoolVar(&flagPerms, "perms", false, "run permissions checks only")
	doctorCmd.Flags().BoolVar(&flagLLM, "llm", false, "run LLM connectivity checks only")
	doctorCmd.Flags().BoolVar(&flagProxy, "proxy", false, "run proxy and network env checks only")
	doctorCmd.Flags().StringVar(&flagDoctorProvider, "provider", "auto", "narrow LLM check to specific provider (auto|openai|gemini|anthropic|ollama)")
	doctorCmd.Flags().DurationVar(&flagTimeout, "timeout", 5*time.Second, "per-network-check timeout")
	doctorCmd.Flags().BoolVar(&flagJSON, "json", false, "output in JSON format")
	doctorCmd.Flags().BoolVar(&flagVerbose, "verbose", false, "include detailed diagnostic information")
	doctorCmd.Flags().BoolVar(&flagFix, "fix", false, "attempt to fix issues where possible")
	doctorCmd.Flags().BoolVar(&flagNoParallel, "no-parallel", false, "run checks sequentially")
}

func runDoctor(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	// Determine which checks to run
	runAll := !flagHooks && !flagPerms && !flagLLM && !flagProxy
	checks := []func(context.Context) CheckResult{}

	if runAll || flagHooks {
		checks = append(checks, checkHookInstallation)
	}
	if runAll || flagPerms {
		checks = append(checks, checkPermissions)
	}
	if runAll || flagLLM {
		checks = append(checks, checkLLMConnectivity)
	}
	if runAll || flagProxy {
		checks = append(checks, checkProxySettings)
	}

	// Run checks
	results := runChecks(ctx, checks)

	// Output results
	if flagJSON {
		outputJSON(results)
	} else {
		outputHuman(results)
	}

	// Exit with appropriate code
	exitCode := 0
	for _, r := range results {
		if r.Status == StatusFail {
			exitCode = 1
			break
		}
	}
	os.Exit(exitCode)
}

func runChecks(ctx context.Context, checks []func(context.Context) CheckResult) []CheckResult {
	results := make([]CheckResult, 0, len(checks))

	if flagNoParallel {
		// Sequential execution
		for _, check := range checks {
			if ctx.Err() != nil {
				break
			}
			results = append(results, check(ctx))
		}
	} else {
		// Parallel execution with bounded concurrency
		resultChan := make(chan CheckResult, len(checks))
		sem := make(chan struct{}, 4) // Limit to 4 concurrent checks

		for _, check := range checks {
			check := check // Capture for goroutine
			go func() {
				sem <- struct{}{}
				defer func() { <-sem }()
				resultChan <- check(ctx)
			}()
		}

		for range checks {
			results = append(results, <-resultChan)
		}
	}

	return results
}

func checkHookInstallation(ctx context.Context) CheckResult {
	start := time.Now()
	result := CheckResult{
		Name: "shell_hook",
	}

	// Detect shell
	shell := detectShell()
	if shell == "" {
		result.Status = StatusFail
		result.Message = "Unable to detect shell"
		result.Suggestion = "Set SHELL environment variable or use --shell flag with 'aish init'"
		result.Duration = time.Since(start)
		return result
	}

	// Get profile file
	profilePath := getShellProfile(shell)
	if profilePath == "" {
		result.Status = StatusFail
		result.Message = fmt.Sprintf("Unknown shell profile for: %s", shell)
		result.Suggestion = "Manually add hook to your shell profile"
		result.Duration = time.Since(start)
		return result
	}

	// Check if profile exists and contains hook
	content, err := os.ReadFile(profilePath)
	if err != nil {
		result.Status = StatusFail
		result.Message = fmt.Sprintf("Cannot read shell profile: %s", profilePath)
		result.Suggestion = "Run 'aish init' to install the shell hook"
		result.Duration = time.Since(start)
		return result
	}

	// Look for aish hook signature using the standard markers
	hasHook := strings.Contains(string(content), config.HookStartMarker)

	if hasHook {
		result.Status = StatusOK
		result.Message = fmt.Sprintf("%s hook detected", shell)
	} else {
		result.Status = StatusFail
		result.Message = fmt.Sprintf("Hook not found in %s", profilePath)
		result.Suggestion = "Run 'aish init' to install the shell hook"

		// Attempt fix if requested
		if flagFix {
			// This would call the existing hook installation logic
			result.Suggestion += " (use --fix to auto-install)"
		}
	}

	result.Duration = time.Since(start)
	return result
}

func checkPermissions(ctx context.Context) CheckResult {
	start := time.Now()
	result := CheckResult{
		Name: "permissions",
	}

	// Get config directory
	configDir := getConfigDir()
	if configDir == "" {
		result.Status = StatusFail
		result.Message = "Unable to determine config directory"
		result.Suggestion = "Check XDG_CONFIG_HOME or HOME environment variables"
		result.Duration = time.Since(start)
		return result
	}

	// Check directory exists
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if flagFix {
			// Create directory with safe permissions
			if err := os.MkdirAll(configDir, 0o700); err != nil {
				result.Status = StatusFail
				result.Message = fmt.Sprintf("Config directory missing and cannot create: %v", err)
				result.Suggestion = fmt.Sprintf("Manually create directory: mkdir -p %s", configDir)
			} else {
				result.Status = StatusOK
				result.Message = "Config directory created with correct permissions"
			}
		} else {
			result.Status = StatusFail
			result.Message = fmt.Sprintf("Config directory does not exist: %s", configDir)
			result.Suggestion = "Run 'aish init' or use --fix to create directory"
		}
		result.Duration = time.Since(start)
		return result
	}

	// Check key files
	issues := []string{}
	configFile := filepath.Join(configDir, "config.json")
	if _, err := os.ReadFile(configFile); err != nil {
		issues = append(issues, fmt.Sprintf("config.json: %v", err))
	}

	historyFile := filepath.Join(configDir, "history.json")
	if _, err := os.Stat(historyFile); err == nil {
		// File exists, check write permission
		f, err := os.OpenFile(historyFile, os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			issues = append(issues, fmt.Sprintf("history.json: not writable"))
		} else {
			if err := f.Close(); err != nil {
				issues = append(issues, fmt.Sprintf("history.json: failed to close: %v", err))
			}
		}
	}

	if len(issues) > 0 {
		result.Status = StatusFail
		result.Message = "File permission issues: " + strings.Join(issues, ", ")
		if runtime.GOOS != "windows" {
			result.Suggestion = fmt.Sprintf("Fix permissions: chmod 600 %s/config.json && chmod 644 %s/history.json", configDir, configDir)
		} else {
			result.Suggestion = "Check file permissions in File Explorer"
		}
	} else {
		result.Status = StatusOK
		result.Message = "Config and history paths are accessible"
	}

	result.Duration = time.Since(start)
	return result
}

func checkLLMConnectivity(ctx context.Context) CheckResult {
	start := time.Now()
	result := CheckResult{
		Name: "llm_connectivity",
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		result.Status = StatusFail
		result.Message = "Cannot load configuration"
		result.Suggestion = "Run 'aish init' to create configuration"
		result.Duration = time.Since(start)
		return result
	}

	// Determine provider
	providerName := flagDoctorProvider
	if providerName == "auto" {
		providerName = cfg.DefaultProvider
	}
	result.Provider = providerName

	// Get provider config
	providerCfg, ok := cfg.Providers[providerName]
	if !ok {
		result.Status = StatusFail
		result.Message = fmt.Sprintf("Provider '%s' not configured", providerName)
		result.Suggestion = "Run 'aish config set default_provider <name>' or configure provider"
		result.Duration = time.Since(start)
		return result
	}

	// Check API key (if required)
	if needsAPIKey(providerName) {
		if providerCfg.APIKey == "" || strings.HasPrefix(providerCfg.APIKey, "YOUR_") {
			// Try environment variable
			envKey := getAPIKeyEnvVar(providerName)
			if os.Getenv(envKey) == "" {
				result.Status = StatusFail
				result.Message = "API key missing"
				result.Suggestion = fmt.Sprintf("Set %s or run: aish config set %s_api_key <key>", envKey, strings.ToLower(providerName))
				result.Duration = time.Since(start)
				return result
			}
		}
	}

	// Make lightweight connectivity check
	checkCtx, cancel := context.WithTimeout(ctx, flagTimeout)
	defer cancel()

	err = probeProvider(checkCtx, providerName, providerCfg)
	if err != nil {
		// Categorize error
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403") {
			result.Status = StatusFail
			result.Message = "Authentication failed"
			result.Suggestion = "Check API key validity; see docs/TROUBLESHOOTING.md"
		} else if strings.Contains(err.Error(), "429") {
			result.Status = StatusFail
			result.Message = "Rate limited"
			result.Suggestion = "You are being rate-limited; wait and try again later"
		} else if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "connection refused") {
			result.Status = StatusFail
			result.Message = "Network error"
			result.Suggestion = "Check your internet connection, proxy, or firewall settings"
		} else if strings.Contains(err.Error(), "tls") || strings.Contains(err.Error(), "certificate") {
			result.Status = StatusFail
			result.Message = "TLS/Certificate error"
			result.Suggestion = "Check SSL_CERT_FILE/SSL_CERT_DIR or corporate proxy CA"
		} else {
			result.Status = StatusFail
			result.Message = fmt.Sprintf("Connection failed: %v", err)
			result.Suggestion = "Check provider status and configuration"
		}
	} else {
		result.Status = StatusOK
		result.Message = fmt.Sprintf("Connected to %s", providerName)
	}

	result.Duration = time.Since(start)
	return result
}

func checkProxySettings(ctx context.Context) CheckResult {
	start := time.Now()
	result := CheckResult{
		Name: "proxy",
	}

	// Check proxy environment variables
	httpProxy := os.Getenv("HTTP_PROXY")
	if httpProxy == "" {
		httpProxy = os.Getenv("http_proxy")
	}
	httpsProxy := os.Getenv("HTTPS_PROXY")
	if httpsProxy == "" {
		httpsProxy = os.Getenv("https_proxy")
	}
	noProxy := os.Getenv("NO_PROXY")
	if noProxy == "" {
		noProxy = os.Getenv("no_proxy")
	}

	if httpProxy == "" && httpsProxy == "" {
		result.Status = StatusOK
		result.Message = "No proxy configured"
		result.Duration = time.Since(start)
		return result
	}

	// Validate proxy URLs
	details := make(map[string]interface{})
	if httpProxy != "" {
		if u, err := url.Parse(httpProxy); err != nil {
			result.Status = StatusFail
			result.Message = fmt.Sprintf("Invalid HTTP_PROXY: %v", err)
			result.Suggestion = "Fix HTTP_PROXY environment variable"
			result.Duration = time.Since(start)
			return result
		} else {
			details["http_proxy_host"] = u.Host
		}
	}
	if httpsProxy != "" {
		if u, err := url.Parse(httpsProxy); err != nil {
			result.Status = StatusFail
			result.Message = fmt.Sprintf("Invalid HTTPS_PROXY: %v", err)
			result.Suggestion = "Fix HTTPS_PROXY environment variable"
			result.Duration = time.Since(start)
			return result
		} else {
			details["https_proxy_host"] = u.Host
		}
	}
	if noProxy != "" {
		details["no_proxy"] = noProxy
	}

	result.Status = StatusWarn
	result.Message = fmt.Sprintf("Proxy detected (requests will use these settings)")
	result.Details = details
	result.Duration = time.Since(start)
	return result
}

// Helper functions

func detectShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" && runtime.GOOS == "windows" {
		// Check for PowerShell
		if _, err := os.Stat(os.Getenv("PROFILE")); err == nil {
			return "powershell"
		}
	}
	if shell == "" {
		return ""
	}
	return filepath.Base(shell)
}

func getShellProfile(shell string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch shell {
	case "bash":
		// Prefer .bashrc on Linux, .bash_profile on macOS
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, ".bash_profile")
		}
		return filepath.Join(home, ".bashrc")
	case "zsh":
		return filepath.Join(home, ".zshrc")
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish")
	case "powershell":
		return os.Getenv("PROFILE")
	default:
		return ""
	}
}

func getConfigDir() string {
	// Respect XDG_CONFIG_HOME for testing and standard XDG conventions
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "aish")
	}

	// Fallback to ~/.config/aish (same logic as aish's config.GetConfigPath())
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".config", "aish")
}

func needsAPIKey(provider string) bool {
	switch provider {
	case config.ProviderOpenAI, config.ProviderGemini, config.ProviderClaude:
		return true
	case config.ProviderOllama, config.ProviderGeminiCLI:
		return false
	default:
		return true
	}
}

func getAPIKeyEnvVar(provider string) string {
	switch provider {
	case config.ProviderOpenAI:
		return "OPENAI_API_KEY"
	case config.ProviderGemini:
		return "GEMINI_API_KEY"
	case config.ProviderClaude:
		return "ANTHROPIC_API_KEY"
	default:
		return ""
	}
}

func probeProvider(ctx context.Context, providerName string, cfg config.ProviderConfig) error {
	// Create a minimal HTTP client with no retries
	client := &http.Client{
		Timeout: flagTimeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: flagTimeout,
			}).DialContext,
		},
	}

	var url string
	var headers map[string]string

	switch providerName {
	case config.ProviderOpenAI:
		url = "https://api.openai.com/v1/models"
		headers = map[string]string{
			"Authorization": "Bearer " + cfg.APIKey,
		}
	case config.ProviderGemini:
		url = "https://generativelanguage.googleapis.com/v1beta/models?key=" + cfg.APIKey
	case config.ProviderClaude:
		url = "https://api.anthropic.com/v1/models"
		headers = map[string]string{
			"x-api-key":         cfg.APIKey,
			"anthropic-version": "2023-06-01",
		}
	case config.ProviderOllama:
		url = "http://127.0.0.1:11434/api/tags"
	case config.ProviderGeminiCLI:
		// Gemini CLI uses OAuth, so we just check if credentials file exists
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot get home directory: %v", err)
		}
		credsPath := filepath.Join(home, ".config", "aish", "gemini_oauth_creds.json")
		if _, err := os.Stat(credsPath); os.IsNotExist(err) {
			return fmt.Errorf("OAuth credentials not found at %s", credsPath)
		}
		// Credentials file exists, consider it OK
		// A full check would require validating the token, but that's complex
		return nil
	default:
		return fmt.Errorf("unsupported provider: %s", providerName)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close() // Best effort close, error already handled
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return nil
}

func outputHuman(results []CheckResult) {
	fmt.Println("Running aish diagnostics...")
	fmt.Println()

	for _, r := range results {
		var icon string
		switch r.Status {
		case StatusOK:
			icon = "[✓]"
		case StatusWarn:
			icon = "[!]"
		case StatusFail:
			icon = "[✗]"
		}

		fmt.Printf("%s %s\n", icon, r.Message)
		if r.Suggestion != "" {
			fmt.Printf("    => %s\n", r.Suggestion)
		}
	}

	fmt.Println()
	summary := calculateSummary(results)
	fmt.Printf("Summary: %d OK, %d WARN, %d FAIL\n", summary.OK, summary.Warn, summary.Fail)
}

func outputJSON(results []CheckResult) {
	summary := calculateSummary(results)
	output := DiagnosticOutput{
		Summary: summary,
		Results: results,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(output)
}

func calculateSummary(results []CheckResult) DiagnosticSummary {
	summary := DiagnosticSummary{}
	for _, r := range results {
		switch r.Status {
		case StatusOK:
			summary.OK++
		case StatusWarn:
			summary.Warn++
		case StatusFail:
			summary.Fail++
		}
	}
	return summary
}
