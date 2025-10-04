package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TonnyWong1052/aish/internal/config"
	"github.com/TonnyWong1052/aish/internal/shell"
	"github.com/TonnyWong1052/aish/internal/ui"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initializes aish by installing the shell hook and configuring the LLM provider",
	Long: `This command provides a guided setup for first-time users.
It will:
1. Install the necessary shell hook for error capturing.
2. Walk you through configuring your preferred LLM provider and API key.`,
	Run: func(cmd *cobra.Command, args []string) {
		pterm.DefaultSection.Println("Step 1: Installing Shell Hook")
		pterm.Info.Println("Invoking hook installer...")
		fmt.Println("[aish] Hook: starting installation/update")
		if err := shell.InstallHook(); err != nil {
			pterm.Error.Printfln("Failed to install shell hook: %v", err)
			fmt.Println("[aish] Hook: install failed")
		} else {
			pterm.Success.Println("Shell hook installed/updated successfully.")
			fmt.Println("[aish] Hook: install completed")
		}
		pterm.Println() // Add some spacing

		pterm.DefaultSection.Println("Step 2: Configuring LLM Provider")
		// 依 TTY 能力選擇互動式或純文字精靈
		if isInteractiveTTY() {
			pterm.Info.Println("Launching configuration wizard (interactive mode)...")
		} else {
			pterm.Info.Println("Launching configuration wizard (plain text mode)...")
		}
		fmt.Println("[aish] Config: starting wizard")

		// 允許使用者在已有配置（含無效配置）時重新初始化
		reset, _ := cmd.Flags().GetBool("reset")

		var cfg *config.Config
		var err error

		if reset {
			// 備份並移除舊配置，確保重新初始化
			if cfgPath, e := config.GetConfigPath(); e == nil {
				if _, statErr := os.Stat(cfgPath); statErr == nil {
					ts := time.Now().Format("20060102-150405")
					backup := filepath.Join(filepath.Dir(cfgPath), fmt.Sprintf("config.reinit.%s.json", ts))
					_ = os.Rename(cfgPath, backup)
					pterm.Warning.Printfln("Existing config moved to: %s", backup)
				}
			}
			// 重新加載（會創建預設）
			cfg, err = config.Load()
		} else {
			// 嘗試正常載入；若驗證失敗，降級為寬鬆加載（Legacy）
			cfg, err = config.Load()
			if err != nil {
				pterm.Warning.Printfln("Config load failed (%v), falling back to legacy load...", err)
				cfg, err = config.LoadLegacy()
				if err != nil {
					// 最終回退：備份現有檔並創建全新配置
					if cfgPath, e := config.GetConfigPath(); e == nil {
						if _, statErr := os.Stat(cfgPath); statErr == nil {
							ts := time.Now().Format("20060102-150405")
							backup := filepath.Join(filepath.Dir(cfgPath), fmt.Sprintf("config.recovery.%s.json", ts))
							_ = os.Rename(cfgPath, backup)
							pterm.Warning.Printfln("Invalid config backed up to: %s", backup)
						}
					}
					cfg, err = config.Load()
				}
			}
		}

		if err != nil {
			pterm.Error.Printfln("Failed to load or create config: %v", err)
			fmt.Println("[aish] Config: wizard aborted")
			os.Exit(1)
		}

		if cfg.Providers == nil {
			cfg.Providers = make(map[string]config.ProviderConfig)
		}

		// Use the proper AISH configuration wizard for LLM provider setup
		if isInteractiveTTY() {
			// Interactive mode - use the full configuration wizard
			wizard := ui.NewConfigWizard(cfg, true) // true = enable advanced settings prompt
			if err := wizard.Run(); err != nil {
				pterm.Error.Printfln("Configuration wizard failed: %v", err)
				fmt.Println("[aish] Config: wizard failed")
				os.Exit(1)
			}
		} else {
			// Non-interactive mode - use simple text-based configuration
			if err := runSimpleProviderConfig(cfg); err != nil {
				pterm.Error.Printfln("Configuration failed: %v", err)
				fmt.Println("[aish] Config: wizard failed")
				os.Exit(1)
			}
		}

		fmt.Println("[aish] Config: wizard finished (you can run 'aish config' anytime)")
	},
}

// runSimpleProviderConfig provides a simple text-based configuration for non-interactive environments
func runSimpleProviderConfig(cfg *config.Config) error {
	reader := bufio.NewReader(os.Stdin)

	// Show available providers
	providers := []string{"openai", "gemini", "gemini-cli", "claude", "ollama"}
	fmt.Printf("Available providers: %s\n", strings.Join(providers, ", "))
	fmt.Println("Tip: 'gemini-cli' is recommended (OAuth login, no API key, easy setup, often higher free usage)")
	fmt.Println("      'ollama' for local models (no API key, runs on your machine)")
	fmt.Printf("Default provider [%s]: ", cfg.DefaultProvider)

	provider, _ := reader.ReadString('\n')
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = cfg.DefaultProvider
	}

	// Validate provider
	validProvider := false
	for _, p := range providers {
		if p == provider {
			validProvider = true
			break
		}
	}
	if !validProvider {
		return fmt.Errorf("invalid provider: %s", provider)
	}

	cfg.DefaultProvider = provider

	// Get existing provider config or create new one
	providerCfg, exists := cfg.Providers[provider]
	if !exists {
		// Set up default configuration based on provider type
		switch provider {
		case "openai":
			providerCfg = config.ProviderConfig{
				APIEndpoint: config.OpenAIAPIEndpoint,
				Model:       config.DefaultOpenAIModel,
			}
		case "gemini":
			providerCfg = config.ProviderConfig{
				APIEndpoint: config.GeminiAPIEndpoint,
				Model:       config.DefaultGeminiModel,
			}
		case "gemini-cli":
			providerCfg = config.ProviderConfig{
				APIEndpoint: config.GeminiCLIAPIEndpoint,
				Model:       config.DefaultGeminiCLIModel,
				Project:     "YOUR_GEMINI_PROJECT_ID",
			}
		case "claude":
			providerCfg = config.ProviderConfig{
				APIEndpoint: config.ClaudeAPIEndpoint,
				Model:       config.DefaultClaudeModel,
			}
		case "ollama":
			providerCfg = config.ProviderConfig{
				APIEndpoint: config.OllamaAPIEndpoint,
				Model:       config.DefaultOllamaModel,
			}
		}
	}

	// Configure API endpoint
	fmt.Printf("API endpoint [%s]: ", providerCfg.APIEndpoint)
	endpoint, _ := reader.ReadString('\n')
	endpoint = strings.TrimSpace(endpoint)
	if endpoint != "" {
		providerCfg.APIEndpoint = endpoint
	}

	// Configure model
	fmt.Printf("Model [%s]: ", providerCfg.Model)
	model, _ := reader.ReadString('\n')
	model = strings.TrimSpace(model)
	if model != "" {
		providerCfg.Model = model
	}

	// Configure provider-specific settings
	switch provider {
	case "openai", "gemini":
		fmt.Print("API Key: ")
		apiKey, _ := reader.ReadString('\n')
		apiKey = strings.TrimSpace(apiKey)
		if apiKey != "" {
			providerCfg.APIKey = apiKey
		}
	case "claude":
		fmt.Println("Get your API key from: https://console.anthropic.com/settings/keys")
		fmt.Print("Anthropic API Key: ")
		apiKey, _ := reader.ReadString('\n')
		apiKey = strings.TrimSpace(apiKey)
		if apiKey != "" {
			providerCfg.APIKey = apiKey
		}
	case "gemini-cli":
		fmt.Print("Project ID (hidden): ")
		projectID, _ := reader.ReadString('\n')
		projectID = strings.TrimSpace(projectID)
		if projectID != "" {
			providerCfg.Project = projectID
		}
	case "ollama":
		fmt.Println("✓ No API key required for local Ollama")
		fmt.Println("Make sure Ollama is installed and running: ollama serve")
		providerCfg.APIKey = ""
	}

	// Save the configuration
	cfg.Providers[provider] = providerCfg

	// Test configuration (optional)
	pterm.Info.Println("Testing configuration...")

	// Save the configuration
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	pterm.Success.Println("Configuration saved.")
	return nil
}

func init() {
	// 提供 --reset 旗標允許使用者重新初始化（備份舊配置並重建）
	initCmd.Flags().Bool("reset", false, "Reinitialize configuration (backup old config and start fresh)")
}
