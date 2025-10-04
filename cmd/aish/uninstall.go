package main

import (
    "errors"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
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
    Long:  "Removes the shell hooks from your shell config, deletes installed binaries from common locations (/usr/local/bin, /opt/homebrew/bin, ~/bin) and PATH, and deletes the configuration directory at ~/.config/aish.",
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

        // 2) 移除所有可能安裝位置的 aish 二進位（包含 PATH 中出現者）
        removeAllBinaries()

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

// removeAllBinaries 掃描常見路徑與 PATH，嘗試刪除所有 aish 可執行檔
func removeAllBinaries() {
    exeName := "aish"
    if runtime.GOOS == "windows" {
        exeName = "aish.exe"
    }

    // 分類候選路徑：使用者目錄 vs 系統目錄
    userCandidates := make(map[string]struct{})
    systemCandidates := make(map[string]struct{})

    if home, err := os.UserHomeDir(); err == nil {
        userCandidates[filepath.Join(home, "bin", exeName)] = struct{}{}
        userCandidates[filepath.Join(home, ".local", "bin", exeName)] = struct{}{}
    }

    systemCandidates[filepath.Join("/usr/local/bin", exeName)] = struct{}{}
    systemCandidates[filepath.Join("/opt/homebrew/bin", exeName)] = struct{}{}

    // 目前正在執行的二進位（分類為使用者或系統）
    if curr, err := os.Executable(); err == nil {
        if home, _ := os.UserHomeDir(); home != "" && strings.HasPrefix(curr, home) {
            userCandidates[curr] = struct{}{}
        } else {
            systemCandidates[curr] = struct{}{}
        }
    }

    // 從 PATH 補充（分類為使用者或系統）
    pathSep := string(os.PathListSeparator)
    home, _ := os.UserHomeDir()
    for _, dir := range strings.Split(os.Getenv("PATH"), pathSep) {
        dir = strings.TrimSpace(dir)
        if dir == "" {
            continue
        }
        candidate := filepath.Join(dir, exeName)
        if home != "" && strings.HasPrefix(dir, home) {
            userCandidates[candidate] = struct{}{}
        } else {
            systemCandidates[candidate] = struct{}{}
        }
    }

    removedUser := 0
    removedSystem := 0
    failedUser := 0
    failedSystem := 0

    // 優先刪除使用者目錄中的二進位（不需要 sudo）
    for p := range userCandidates {
        if st, err := os.Lstat(p); err == nil && (st.Mode().IsRegular() || (st.Mode()&os.ModeSymlink) != 0) {
            if tryRemoveUserPath(p) {
                removedUser++
            } else {
                failedUser++
            }
        }
    }

    // 詢問是否要刪除系統目錄中的二進位
    needSystemRemoval := false
    for p := range systemCandidates {
        if st, err := os.Lstat(p); err == nil && (st.Mode().IsRegular() || (st.Mode()&os.ModeSymlink) != 0) {
            needSystemRemoval = true
            break
        }
    }

    if needSystemRemoval {
        pterm.Warning.Println("Found aish binaries in system directories that may require administrator privileges to remove.")
        confirmed, _ := pterm.DefaultInteractiveConfirm.
            WithDefaultValue(false).
            Show("Do you want to attempt to remove system binaries (may require sudo)?")

        if confirmed {
            for p := range systemCandidates {
                if st, err := os.Lstat(p); err == nil && (st.Mode().IsRegular() || (st.Mode()&os.ModeSymlink) != 0) {
                    if tryRemoveSystemPath(p) {
                        removedSystem++
                    } else {
                        failedSystem++
                    }
                }
            }
        } else {
            pterm.Info.Println("Skipping system binary removal.")
        }
    }

    // 報告結果
    if removedUser > 0 {
        pterm.Success.Printfln("Removed %d binary(ies) from user directories", removedUser)
    }
    if removedSystem > 0 {
        pterm.Success.Printfln("Removed %d binary(ies) from system directories", removedSystem)
    }
    if failedUser > 0 || failedSystem > 0 {
        pterm.Warning.Printfln("Failed to remove %d binary(ies)", failedUser+failedSystem)
    }
    if removedUser == 0 && removedSystem == 0 && failedUser == 0 && failedSystem == 0 {
        pterm.Info.Println("No aish binary found in common locations or PATH.")
    }
}

// tryRemoveUserPath 嘗試刪除使用者目錄中的檔案（不使用 sudo）
func tryRemoveUserPath(p string) bool {
    if err := os.Remove(p); err == nil {
        pterm.Success.Printfln("Binary removed: %s", p)
        return true
    } else {
        pterm.Error.Printfln("Failed to remove user binary: %s (err=%v)", p, err)
        return false
    }
}

// tryRemoveSystemPath 嘗試刪除系統目錄中的檔案（可能需要 sudo）
func tryRemoveSystemPath(p string) bool {
    if err := os.Remove(p); err == nil {
        pterm.Success.Printfln("Binary removed: %s", p)
        return true
    } else {
        // 若權限不足，嘗試 sudo rm -f
        if errors.Is(err, os.ErrPermission) || isEPERM(err) {
            if _, lookErr := exec.LookPath("sudo"); lookErr == nil {
                pterm.Info.Println("Requesting administrator privileges to remove system binary...")
                cmd := exec.Command("sudo", "rm", "-f", p)
                if runErr := cmd.Run(); runErr == nil {
                    pterm.Success.Printfln("Binary removed (sudo): %s", p)
                    return true
                }
            }
        }
        pterm.Error.Printfln("Failed to remove system binary: %s (err=%v)", p, err)
        return false
    }
}

// isEPERM 判斷錯誤訊息是否為作業系統層級的權限錯誤（非標準包裝）
func isEPERM(err error) bool {
    // 最保守做法：檢查錯誤字串包含 'permission denied'
    // 避免平台特定 errno 匯入（跨平台簡化）
    return strings.Contains(strings.ToLower(err.Error()), "permission denied")
}
