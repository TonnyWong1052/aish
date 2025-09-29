package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/TonnyWong1052/aish/internal/config"
	"github.com/TonnyWong1052/aish/internal/llm/openai"
	"github.com/TonnyWong1052/aish/internal/prompt"
	"github.com/TonnyWong1052/aish/internal/ui"
	"os"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration settings",
	Run: func(cmd *cobra.Command, args []string) {
		runConfigureLogic(cmd, args)
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			pterm.Error.Printfln("Failed to load config: %v", err)
			os.Exit(1)
		}

		pterm.DefaultSection.Println("Current Configuration")

		providerCfg, ok := cfg.Providers[cfg.DefaultProvider]
		if !ok {
			pterm.Warning.Println("Default provider not set. Please run 'aish config'.")
			return
		}

		items := []pterm.BulletListItem{
			{Level: 0, Text: fmt.Sprintf("Default Provider: %s", cfg.DefaultProvider)},
			{Level: 1, Text: fmt.Sprintf("API Host: %s", providerCfg.APIEndpoint)},
			{Level: 1, Text: fmt.Sprintf("Model: %s", providerCfg.Model)},
		}
		if cfg.DefaultProvider == "gemini-cli" {
			items = append(items, pterm.BulletListItem{Level: 1, Text: fmt.Sprintf("Project: %s", revealOrNull(providerCfg.Project))})
		}
		pterm.DefaultBulletList.WithItems(items).Render()
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := strings.TrimSpace(args[0])
		cfg, err := config.Load()
		if err != nil {
			pterm.Error.Printfln("Failed to load config: %v", err)
			os.Exit(1)
		}
		lower := strings.ToLower(key)
		switch lower {
		case "default_provider":
			fmt.Println(cfg.DefaultProvider)
			return
		case "user_preferences.language", "language":
			fmt.Println(cfg.UserPreferences.Language)
			return
	case "auto_execute", "auto-execute", "user_preferences.auto_execute":
			if cfg.UserPreferences.AutoExecute {
				fmt.Println("true")
			} else {
				fmt.Println("false")
			}
			return
		case "user_preferences.enabled_llm_triggers", "enabled_llm_triggers":
			if len(cfg.UserPreferences.EnabledLLMTriggers) == 0 {
				fmt.Println("")
			} else {
				fmt.Println(strings.Join(cfg.UserPreferences.EnabledLLMTriggers, ","))
			}
			return
		}
		if strings.HasPrefix(lower, "providers.") {
			parts := strings.Split(lower, ".")
			if len(parts) != 3 {
				pterm.Error.Println("Use providers.<name>.<field>, fields: api_endpoint|model|api_key|project")
				os.Exit(1)
			}
			name := parts[1]
			field := parts[2]
			pc, ok := cfg.Providers[name]
			if !ok {
				pterm.Error.Printfln("Provider not found: %s", name)
				os.Exit(1)
			}
			switch field {
			case "api_endpoint":
				fmt.Println(pc.APIEndpoint)
			case "model":
				fmt.Println(pc.Model)
			case "api_key":
				fmt.Println(maskIfSet(pc.APIKey))
			case "project":
				fmt.Println(revealOrNull(pc.Project))
			default:
				pterm.Error.Println("Unknown field. Use one of: api_endpoint|model|api_key|project")
				os.Exit(1)
			}
			return
		}
		pterm.Error.Printfln("Unsupported key: %s", key)
		os.Exit(1)
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key := strings.TrimSpace(args[0])
		value := strings.TrimSpace(args[1])
		cfg, err := config.Load()
		if err != nil {
			pterm.Error.Printfln("Failed to load config: %v", err)
			os.Exit(1)
		}
		lower := strings.ToLower(key)
		switch lower {
		case "default_provider":
			if _, ok := cfg.Providers[value]; !ok {
				pterm.Error.Printfln("Unknown provider: %s", value)
				os.Exit(1)
			}
			cfg.DefaultProvider = value
		case "user_preferences.language", "language":
			cfg.UserPreferences.Language = value
		case "auto_execute", "auto-execute", "user_preferences.auto_execute":
			switch strings.ToLower(value) {
			case "true", "1", "yes", "on", "enable", "enabled":
				cfg.UserPreferences.AutoExecute = true
			case "false", "0", "no", "off", "disable", "disabled":
				cfg.UserPreferences.AutoExecute = false
			default:
				pterm.Error.Printfln("Invalid value for auto_execute: %s. Use: true/false, 1/0, yes/no, on/off", value)
				os.Exit(1)
			}
		case "user_preferences.enabled_llm_triggers", "enabled_llm_triggers":
			// 逗號分隔清單；允許空字串代表清空
			var list []string
			for _, part := range strings.Split(value, ",") {
				p := strings.TrimSpace(part)
				if p != "" {
					list = append(list, p)
				}
			}
			cfg.UserPreferences.EnabledLLMTriggers = list
		default:
			if strings.HasPrefix(lower, "providers.") {
				parts := strings.Split(lower, ".")
				if len(parts) != 3 {
					pterm.Error.Println("Use providers.<name>.<field>, fields: api_endpoint|model|api_key|project")
					os.Exit(1)
				}
				name := parts[1]
				field := parts[2]
				pc := cfg.Providers[name]
				switch field {
				case "api_endpoint":
					pc.APIEndpoint = value
				case "model":
					pc.Model = value
				case "api_key":
					pc.APIKey = value
				case "project":
					pc.Project = value
				default:
					pterm.Error.Println("Unknown field. Use one of: api_endpoint|model|api_key|project")
					os.Exit(1)
				}
				cfg.Providers[name] = pc
			} else {
				pterm.Error.Printfln("Unsupported key: %s", key)
				os.Exit(1)
			}
		}
		if err := cfg.Save(); err != nil {
			pterm.Error.Printfln("Failed to save config: %v", err)
			os.Exit(1)
		}
		pterm.Success.Println("Updated.")
	},
}

// runConfigureLogic contains the logic from the original configureCmd
func runConfigureLogic(cmd *cobra.Command, args []string) {
	// If --interactive is not explicitly specified, enable interactive wizard by default in TTY
	interactiveFlag, _ := cmd.Flags().GetBool("interactive")
	interactive := interactiveFlag
	if !cmd.Flags().Changed("interactive") {
		interactive = isInteractiveTTY()
	}
	_, _ = cmd.Flags().GetBool("from-init")

	cfg, err := config.Load()
	if err != nil {
		errorHandler := ui.NewErrorHandler(flagDebug)
		userErr := errorHandler.CreateConfigurationError(
			"Unable to load AISH configuration.",
			[]string{
				"Run 'aish init' to create initial configuration",
				"Check if configuration file is corrupted",
				"Verify ~/.config/aish/ directory permissions",
			},
		)
		userErr.Cause = err
		errorHandler.HandleError(userErr)
		os.Exit(1)
	}
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]config.ProviderConfig)
	}

	// Execute TUI wizard when interactive flag is true (either explicitly set or auto-detected in TTY)
	if interactive {
		pterm.Info.Println("Running interactive TUI configuration...")
		// Use the new settings TUI system
		if err := ui.RunSettingsTUI(cfg); err != nil {
			pterm.Error.Printfln("Configuration failed: %v", err)
			os.Exit(1)
		}
		pterm.Success.Println("Configuration saved.")
		return
	} else {
		if err := plainConfigureWizard(cfg); err != nil {
			pterm.Error.Printfln("Configuration failed: %v", err)
			os.Exit(1)
		}
		pterm.Success.Println("Configuration saved.")
		return
	}
}

// maskIfSet masks non-empty keys for display
func maskIfSet(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	if len(v) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(v)-4) + v[len(v)-4:]
}

// hideIfSet hides sensitive values completely for display
// to avoid leaking real identifiers in prompts.
func hideIfSet(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	return "hidden"
}

// revealOrNull shows the raw value when present; otherwise prints 'null'
func revealOrNull(v string) string {
    if strings.TrimSpace(v) == "" {
        return "null"
    }
    return v
}

// isInteractiveTTY checks if in interactive TTY environment
func isInteractiveTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// shouldOmitV1 returns true when the given endpoint already embeds a version-like
// path segment (e.g., /v, /v1, /v1beta, /v1internal), in which case we should not
// auto-append /v1 again.
func shouldOmitV1(endpoint string) bool {
	e := strings.TrimSpace(strings.ToLower(endpoint))
	// Normalize trailing slash for checks
	e = strings.TrimSuffix(e, "/")
	// Heuristic: if the path already contains a version segment starting with '/v'
	// we avoid appending '/v1'. This covers '/v', '/v1', '/v1beta', '/v1internal', etc.
	// It also works with many proxy endpoints like '.../v'.
	// Safe default when path has no '/v' segment: return false (we will manage '/v1').
	// Note: We intentionally keep this heuristic simple to avoid overfitting.
	// If later runtime detection finds otherwise, provider-level fallback will handle it.
	return strings.Contains(e, "/v")
}

// plainConfigureWizard provides a plain text configuration flow that doesn't depend on TUI
func plainConfigureWizard(cfg *config.Config) error {
	reader := bufio.NewReader(os.Stdin)

	// --- Provider Selection ---
	defProv := cfg.DefaultProvider
	if defProv == "" {
		defProv = "openai"
	}

	pterm.Println("Available providers: openai, gemini, gemini-cli")
	pterm.Println(fmt.Sprintf("Default provider [%s]:", defProv))
	fmt.Print(">: ")
	provider, _ := reader.ReadString('\n')
	if provider := strings.TrimSpace(provider); provider != "" {
		cfg.DefaultProvider = provider
	} else {
		cfg.DefaultProvider = defProv
	}

	// Initialize provider config if needed
	pc := cfg.Providers[cfg.DefaultProvider]

	// Set defaults based on provider
	switch cfg.DefaultProvider {
	case "openai":
		if pc.APIEndpoint == "" {
			pc.APIEndpoint = "https://api.openai.com/v1"
		}
		if pc.Model == "" {
			pc.Model = "gpt-4"
		}
	case "gemini":
		if pc.APIEndpoint == "" {
			pc.APIEndpoint = "https://generativelanguage.googleapis.com/v1"
		}
		if pc.Model == "" {
			pc.Model = "gemini-pro"
		}
	case "gemini-cli":
		pc.APIEndpoint = "https://cloudcode-pa.googleapis.com/v1internal"
		if pc.Model == "" {
			pc.Model = "gemini-2.5-flash"
		}
	}

	// --- API Configuration ---
	if cfg.DefaultProvider != "gemini-cli" {
		pterm.Println(fmt.Sprintf("API endpoint [%s]:", pc.APIEndpoint))
		fmt.Print(">: ")
		endpoint, _ := reader.ReadString('\n')
		if endpoint := strings.TrimSpace(endpoint); endpoint != "" {
			pc.APIEndpoint = endpoint
		}
	}

	pterm.Println(fmt.Sprintf("Model [%s]:", pc.Model))
	fmt.Print(">: ")
	model, _ := reader.ReadString('\n')
	if model := strings.TrimSpace(model); model != "" {
		pc.Model = model
	}

	pterm.Println(fmt.Sprintf("API Key %s:", maskIfSet(pc.APIKey)))
	fmt.Print(">: ")
	apiKey, _ := reader.ReadString('\n')
	if apiKey := strings.TrimSpace(apiKey); apiKey != "" {
		pc.APIKey = apiKey
	}

	// For gemini-cli, also ask for project if not set
	if cfg.DefaultProvider == "gemini-cli" {
		pterm.Println(fmt.Sprintf("Project ID %s:", hideIfSet(pc.Project)))
		fmt.Print(">: ")
		project, _ := reader.ReadString('\n')
		if project := strings.TrimSpace(project); project != "" {
			pc.Project = project
		}
	}

	// Update provider config
	cfg.Providers[cfg.DefaultProvider] = pc

	// Test the configuration
	pterm.Info.Println("Testing configuration...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pm, err := prompt.NewManager("")
	if err != nil {
		pterm.Warning.Printfln("Warning: Could not initialize prompt manager: %v", err)
	} else {
		provider, err := openai.NewProvider(pc, pm)
		if err != nil {
			pterm.Warning.Printfln("Warning: Could not initialize provider: %v", err)
		} else {
			models, err := provider.VerifyConnection(ctx)
			if err != nil {
				pterm.Warning.Printfln("Warning: Could not verify connection: %v", err)
			} else {
				pterm.Success.Printfln("Connection verified. Available models: %s", strings.Join(models, ", "))
			}
		}
	}

	// Set defaults
	if cfg.UserPreferences.Language == "" {
		cfg.UserPreferences.Language = "english"
	}
	if cfg.UserPreferences.MaxHistorySize == 0 {
		cfg.UserPreferences.MaxHistorySize = 100
	}

	cfg.Enabled = true

	return cfg.Save()
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	
	configCmd.Flags().Bool("interactive", false, "Use interactive TUI configuration wizard")
	configCmd.Flags().Bool("from-init", false, "Internal flag for init command")
	configCmd.Flags().MarkHidden("from-init")
}
