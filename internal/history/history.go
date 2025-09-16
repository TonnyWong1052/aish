package history

import (
	"log"
	"os"
	"path/filepath"
	"powerful-cli/internal/classification"
	"powerful-cli/internal/config"
	"time"
)

// Entry represents a single command record in the history.
type Entry struct {
	Timestamp time.Time                `json:"timestamp"`
	Command   string                   `json:"command"`
	Stdout    string                   `json:"stdout"`
	Stderr    string                   `json:"stderr"`
	ExitCode  int                      `json:"exit_code"`
	ErrorType classification.ErrorType `json:"error_type"`
}

// History holds all the recorded entries.
type History struct {
	Entries []Entry `json:"entries"`
}

const defaultMaxHistorySize = 100

func determineHistoryLimit() int {
	if cfg, err := config.Load(); err == nil && cfg.UserPreferences.MaxHistorySize > 0 {
		return cfg.UserPreferences.MaxHistorySize
	}
	return defaultMaxHistorySize
}

func getHistoryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "aish", "history.json"), nil
}

func Add(entry Entry) error {
	mgr, err := getDefaultManager()
	if err != nil {
		return err
	}
	return mgr.Append(entry)
}

// Load 透過長駐管理器回傳現有歷史紀錄。
func Load() (*History, error) {
	mgr, err := getDefaultManager()
	if err != nil {
		return nil, err
	}
	return &History{Entries: mgr.Entries()}, nil
}

// Clear 透過管理器清空歷史檔案並保持檔案格式一致。
func Clear() error {
	mgr, err := getDefaultManager()
	if err != nil {
		return err
	}
	return mgr.Clear()
}

// Close 強制刷新並關閉預設歷史管理器，供 CLI 結束時釋放資源。
func Close() error {
	if managerInst == nil {
		return nil
	}

	err := managerInst.Close()
	if err != nil {
		log.Printf("aish history: failed to close manager: %v", err)
	}
	return err
}
