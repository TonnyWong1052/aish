package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"atomicgo.dev/keyboard/keys"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/TonnyWong1052/aish/internal/classification"
	"github.com/TonnyWong1052/aish/internal/config"
	"github.com/TonnyWong1052/aish/internal/shell"
)

// hookCmd is the parent command for hook-related operations
var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Manage the aish shell hook",
	Long:  `Install, enable, disable, or uninstall the shell hook that allows aish to capture errors.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// hookInstallCmd contains the logic from the original setupCmd
var hookInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Installs or updates the shell hook",
	Run: func(cmd *cobra.Command, args []string) {
		if err := shell.InstallHook(); err != nil {
			pterm.Error.Printfln("Failed to install hook: %v", err)
			os.Exit(1)
		} else {
			pterm.Success.Println("Shell hook installed/updated successfully.")
		}
	},
}

// hookEnableCmd contains the logic from the original enableCmd
var hookEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enables the aish error capture hook",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			pterm.Error.Printfln("Failed to load config: %v", err)
			os.Exit(1)
		}
		cfg.Enabled = true
		if err := cfg.Save(); err != nil {
			pterm.Error.Printfln("Failed to save config: %v", err)
			os.Exit(1)
		}
		pterm.Success.Println("aish has been enabled.")
	},
}

// hookDisableCmd contains the logic from the original disableCmd
var hookDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disables the aish error capture hook",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			pterm.Error.Printfln("Failed to load config: %v", err)
			os.Exit(1)
		}
		cfg.Enabled = false
		if err := cfg.Save(); err != nil {
			pterm.Error.Printfln("Failed to save config: %v", err)
			os.Exit(1)
		}
		pterm.Success.Println("aish has been disabled.")
		pterm.Info.Println("The shell hook is still installed. You can re-enable aish anytime by running 'aish hook enable'.")
	},
}

// hookUninstallCmd contains the logic from the original uninstallCmd
var hookUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Removes the shell hook and all related aish files",
	Run: func(cmd *cobra.Command, args []string) {
		pterm.Warning.Println("This will remove all aish related files, including the binary, configuration, and history.")
		confirmed, _ := pterm.DefaultInteractiveConfirm.
			WithDefaultValue(false).
			Show("Are you sure you want to continue?")

		if !confirmed {
			pterm.Info.Println("Uninstallation cancelled.")
			return
		}

		pterm.Info.Println("Uninstalling aish...")

		if uninstalled, err := shell.UninstallHook(); err != nil {
			pterm.Error.Printfln("Failed to uninstall shell hook: %v", err)
		} else if uninstalled {
			pterm.Success.Println("Shell hook uninstalled.")
		}

		home, err := os.UserHomeDir()
		if err == nil {
			binaryPath := home + "/bin/aish"
			if _, err := os.Stat(binaryPath); err == nil {
				if err := os.Remove(binaryPath); err == nil {
					pterm.Success.Println("Binary removed from ~/bin/aish")
				} else {
					pterm.Error.Printfln("Failed to remove binary: %v", err)
				}
			}
		}

		configPath, err := config.GetConfigPath()
		if err == nil {
			configDir := strings.Replace(configPath, "/config.json", "", 1)
			if _, err := os.Stat(configDir); !os.IsNotExist(err) {
				if err := os.RemoveAll(configDir); err == nil {
					pterm.Success.Println("Configuration directory removed.")
				} else {
					pterm.Error.Printfln("Failed to remove config directory: %v", err)
				}
			}
		}

		pterm.Success.Println("Uninstallation complete.")
		pterm.Info.Println("Please restart your terminal for changes to take effect.")
	},
}

var hookInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Configure hook triggers and preferences",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			pterm.Error.Printfln("Failed to load config: %v", err)
			os.Exit(1)
		}

		availableTriggers := classification.AllErrorTypeStrings()

		defaultSelection := intersectStrings(cfg.UserPreferences.EnabledLLMTriggers, availableTriggers)
		if len(defaultSelection) == 0 {
			defaultSelection = []string{
				string(classification.CommandNotFound),
				string(classification.FileNotFoundOrDirectory),
			}
		}

		pterm.DefaultSection.
			WithIndentCharacter(" ").
			WithLevel(1).
			Println("Auto-trigger Error Types")
		pterm.Info.Println("Use arrows to move, space to toggle, enter to confirm.")

		selected, err := pterm.DefaultInteractiveMultiselect.
			WithOptions(availableTriggers).
			WithDefaultOptions(defaultSelection).
			WithFilter(false).
			WithMaxHeight(len(availableTriggers)).
			WithKeySelect(keys.Space).
			WithKeyConfirm(keys.Enter).
			Show("Please select your options:")
		if err != nil {
			pterm.Error.Printfln("Cancelled: %v", err)
			return
		}

		if len(selected) == 0 {
			pterm.Warning.Println("No error types selected; hook auto-trigger will be disabled until you re-run this command.")
		}

		cfg.UserPreferences.EnabledLLMTriggers = selected
		if err := cfg.Save(); err != nil {
			pterm.Error.Printfln("Failed to save config: %v", err)
			os.Exit(1)
		}

		if len(selected) > 0 {
			pterm.Success.Printfln("Auto-trigger will run for: %s", strings.Join(selected, ", "))
		} else {
			pterm.Success.Println("Hook auto-trigger has been disabled.")
		}

		if hookPath, err := shell.GetHookFilePath(); err == nil {
			pterm.Info.Printfln("Current shell hook file: %s", hookPath)
		}

		// Additional preferences: skip user-installed commands and custom skip patterns
		pterm.DefaultSection.WithIndentCharacter(" ").WithLevel(1).Println("Skip Rules")
		skipAll, _ := pterm.DefaultInteractiveConfirm.
			WithDefaultValue(true).
			Show("Skip all user-installed commands (Homebrew/npm/pipx/etc.)?")

		currentPatterns := os.Getenv("AISH_SKIP_COMMAND_PATTERNS")
		patterns, _ := pterm.DefaultInteractiveTextInput.
			WithDefaultText(currentPatterns).
			Show("Extra skip patterns (whitespace-separated globs, optional):")

		// Persist to env files that hook sources on each shell startup
		configPath, err := config.GetConfigPath()
		if err == nil {
			stateDir := strings.Replace(configPath, "/config.json", "", 1)
			if err := os.MkdirAll(stateDir, 0o755); err == nil {
				_ = os.WriteFile(filepath.Join(stateDir, "env.sh"), []byte(buildHookEnvSH(skipAll, patterns)), 0o644)
				_ = os.WriteFile(filepath.Join(stateDir, "env.ps1"), []byte(buildHookEnvPS(skipAll, patterns)), 0o644)
				pterm.Success.Printfln("Saved hook preferences to %s", stateDir)
				pterm.Info.Println("Reload your shell: source ~/.zshrc (or ~/.bashrc) to take effect.")
			} else {
				pterm.Warning.Printfln("Failed to create state dir for env files: %v", err)
			}
		} else {
			pterm.Warning.Printfln("Failed to resolve config path: %v", err)
		}
	},
}

func init() {
	hookCmd.AddCommand(hookInstallCmd)
	hookCmd.AddCommand(hookEnableCmd)
	hookCmd.AddCommand(hookDisableCmd)
	hookCmd.AddCommand(hookUninstallCmd)
	hookCmd.AddCommand(hookInitCmd)
}

func intersectStrings(values, allowed []string) []string {
	allowedSet := make(map[string]struct{}, len(values))
	for _, v := range values {
		allowedSet[v] = struct{}{}
	}

	result := make([]string, 0, len(values))
	for _, opt := range allowed {
		if _, ok := allowedSet[opt]; ok {
			result = append(result, opt)
		}
	}

	return result
}

// buildHookEnvSH builds env file content for POSIX shells
func buildHookEnvSH(skipAll bool, patterns string) string {
	b := &strings.Builder{}
	fmt.Fprintln(b, "# Generated by aish hook init")
	if skipAll {
		fmt.Fprintln(b, "export AISH_SKIP_ALL_USER_COMMANDS=1")
	} else {
		fmt.Fprintln(b, "export AISH_SKIP_ALL_USER_COMMANDS=0")
	}
	if strings.TrimSpace(patterns) != "" {
		fmt.Fprintf(b, "export AISH_SKIP_COMMAND_PATTERNS=\"%s\"\n", strings.TrimSpace(patterns))
	}
	return b.String()
}

// buildHookEnvPS builds env file content for PowerShell
func buildHookEnvPS(skipAll bool, patterns string) string {
	b := &strings.Builder{}
	fmt.Fprintln(b, "# Generated by aish hook init")
	if skipAll {
		fmt.Fprintln(b, "$env:AISH_SKIP_ALL_USER_COMMANDS = '1'")
	} else {
		fmt.Fprintln(b, "$env:AISH_SKIP_ALL_USER_COMMANDS = '0'")
	}
	if strings.TrimSpace(patterns) != "" {
		fmt.Fprintf(b, "$env:AISH_SKIP_COMMAND_PATTERNS = '%s'\n", strings.TrimSpace(patterns))
	}
	return b.String()
}
