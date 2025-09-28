package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/TonnyWong1052/aish/internal/config"
	"github.com/TonnyWong1052/aish/internal/shell"
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
		// 轉交給與 `aish config` 相同的邏輯，確保可用鍵盤選單等互動式體驗
		_ = configCmd.Flags().Set("from-init", "true")
		runConfigureLogic(configCmd, nil)
		_ = configCmd.Flags().Set("from-init", "false")
		fmt.Println("[aish] Config: wizard finished (you can run 'aish config' anytime)")
	},
}

func init() {
	// 提供 --reset 旗標允許使用者重新初始化（備份舊配置並重建）
	initCmd.Flags().Bool("reset", false, "Reinitialize configuration (backup old config and start fresh)")
}
