package history

import (
    "log"
    "os"
    "path/filepath"
    "github.com/TonnyWong1052/aish/internal/classification"
    "github.com/TonnyWong1052/aish/internal/config"
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

// Load returns existing history records through persistent manager.
func Load() (*History, error) {
	mgr, err := getDefaultManager()
	if err != nil {
		return nil, err
	}
	return &History{Entries: mgr.Entries()}, nil
}

// Clear clears history file through manager and maintains consistent file format.
func Clear() error {
	mgr, err := getDefaultManager()
	if err != nil {
		return err
	}
	return mgr.Clear()
}

// Close forces flush and closes default history manager for resource release when CLI ends.
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
