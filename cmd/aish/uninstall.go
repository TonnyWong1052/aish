package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/TonnyWong1052/aish/internal/config"
	"github.com/TonnyWong1052/aish/internal/shell"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// uninstallCmd 提供頂層卸載指令，等同於 hook uninstall，但更直覺
var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstalls aish (hooks, binary, and configuration)",
	Long:  "Removes the shell hooks from your shell config, deletes the installed binary at ~/bin/aish, and deletes the configuration directory at ~/.config/aish.",
	Run: func(cmd *cobra.Command, args []string) {
		pterm.Warning.Println("This will remove aish hooks, binary, and configuration directory.")
		confirmed, _ := pterm.DefaultInteractiveConfirm.
			WithDefaultValue(false).
			Show("Are you sure you want to continue?")

		if !confirmed {
			pterm.Info.Println("Uninstallation cancelled.")
			return
		}

		pterm.Info.Println("Uninstalling aish...")

		// 1) 移除 shell hooks
		if uninstalled, err := shell.UninstallHook(); err != nil {
			pterm.Error.Printfln("Failed to uninstall shell hook: %v", err)
		} else if uninstalled {
			pterm.Success.Println("Shell hook uninstalled.")
		} else {
			pterm.Info.Println("No shell hook entries found.")
		}

		// 2) 移除 ~/bin/aish 二進位
		home, err := os.UserHomeDir()
		if err == nil {
			binaryPath := filepath.Join(home, "bin", "aish")
			if _, err := os.Stat(binaryPath); err == nil {
				if err := os.Remove(binaryPath); err == nil {
					pterm.Success.Printfln("Binary removed: %s", binaryPath)
				} else {
					pterm.Error.Printfln("Failed to remove binary: %v", err)
				}
			} else {
				pterm.Info.Printf("Binary not found at %s\n", binaryPath)
			}
		}

		// 3) 移除設定資料夾 ~/.config/aish
		configPath, err := config.GetConfigPath()
		if err == nil {
			configDir := strings.Replace(configPath, string(filepath.Separator)+"config.json", "", 1)
			if _, err := os.Stat(configDir); !os.IsNotExist(err) {
				if err := os.RemoveAll(configDir); err == nil {
					pterm.Success.Printfln("Configuration directory removed: %s", configDir)
				} else {
					pterm.Error.Printfln("Failed to remove config directory: %v", err)
				}
			} else {
				pterm.Info.Printf("Config directory not found: %s\n", configDir)
			}
		}

		pterm.Success.Println("Uninstallation complete.")
		pterm.Info.Println("Please restart your terminal for changes to take effect.")
	},
}

func init() {
	// 註冊頂層卸載指令
	rootCmd.AddCommand(uninstallCmd)
}
