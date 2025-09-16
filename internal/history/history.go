package history

import (
	"encoding/json"
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

func getHistoryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "aish", "history.json"), nil
}

// Add appends a new entry to the history.
func (h *History) Save() error {
	path, err := getHistoryPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func Add(entry Entry) error {
	hist, err := Load()
	if err != nil {
		return err
	}
	hist.Entries = append([]Entry{entry}, hist.Entries...) // Prepend to keep it sorted by newest
	cfg, err := config.Load()
	if err == nil && cfg.UserPreferences.MaxHistorySize > 0 {
		if len(hist.Entries) > cfg.UserPreferences.MaxHistorySize {
			hist.Entries = hist.Entries[:cfg.UserPreferences.MaxHistorySize]
		}
	} else {
		// Fallback to a default limit if config loading fails or size is not set
		const defaultMaxHistorySize = 100
		if len(hist.Entries) > defaultMaxHistorySize {
			hist.Entries = hist.Entries[:defaultMaxHistorySize]
		}
	}
	return hist.Save()
}

// Load reads the history from the file.
func Load() (*History, error) {
	path, err := getHistoryPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// History file does not exist, return an empty history.
		return &History{Entries: []Entry{}}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return &History{Entries: []Entry{}}, nil
	}

	var hist History
	if err := json.Unmarshal(data, &hist); err != nil {
		return nil, err
	}

	return &hist, nil
}

// Clear removes all entries from history by overwriting the file with an empty set.
func Clear() error {
	empty := &History{Entries: []Entry{}}
	return empty.Save()
}
