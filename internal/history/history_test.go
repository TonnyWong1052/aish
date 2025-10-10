package history

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/TonnyWong1052/aish/internal/classification"
)

func TestEntry(t *testing.T) {
	entry := Entry{
		Timestamp: time.Now(),
		Command:   "ls -la",
		Stdout:    "output",
		Stderr:    "error",
		ExitCode:  1,
		ErrorType: classification.CommandNotFound,
	}

	// Test JSON marshaling
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal entry: %v", err)
	}

	// Test JSON unmarshaling
	var decoded Entry
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal entry: %v", err)
	}

	if decoded.Command != entry.Command {
		t.Errorf("Expected command %s, got %s", entry.Command, decoded.Command)
	}

	if decoded.ExitCode != entry.ExitCode {
		t.Errorf("Expected exit code %d, got %d", entry.ExitCode, decoded.ExitCode)
	}

	if decoded.ErrorType != entry.ErrorType {
		t.Errorf("Expected error type %v, got %v", entry.ErrorType, decoded.ErrorType)
	}
}

func TestHistory(t *testing.T) {
	hist := History{
		Entries: []Entry{
			{
				Timestamp: time.Now(),
				Command:   "cmd1",
				ExitCode:  0,
			},
			{
				Timestamp: time.Now(),
				Command:   "cmd2",
				ExitCode:  1,
			},
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(hist)
	if err != nil {
		t.Fatalf("Failed to marshal history: %v", err)
	}

	// Test JSON unmarshaling
	var decoded History
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal history: %v", err)
	}

	if len(decoded.Entries) != len(hist.Entries) {
		t.Errorf("Expected %d entries, got %d", len(hist.Entries), len(decoded.Entries))
	}

	if decoded.Entries[0].Command != "cmd1" {
		t.Errorf("Expected command cmd1, got %s", decoded.Entries[0].Command)
	}
}

func TestDetermineHistoryLimit(t *testing.T) {
	// Without config, should return default
	limit := determineHistoryLimit()
	if limit <= 0 {
		t.Errorf("Expected positive limit, got %d", limit)
	}
}

func TestGetHistoryPath(t *testing.T) {
	path, err := getHistoryPath()
	if err != nil {
		t.Fatalf("getHistoryPath failed: %v", err)
	}

	if path == "" {
		t.Error("Expected non-empty path")
	}

	// Should contain history.json
	if !strings.Contains(path, "history.json") {
		t.Errorf("Path %s doesn't contain history.json", path)
	}
}

func TestCloseWithoutInit(t *testing.T) {
	// Reset manager
	managerInst = nil

	// Close without initializing should not error
	err := Close()
	if err != nil {
		t.Errorf("Close without init should not error, got: %v", err)
	}
}
