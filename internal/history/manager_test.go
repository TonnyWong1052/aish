package history

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TonnyWong1052/aish/internal/classification"
)

func TestManagerNewManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aish-manager-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testPath := filepath.Join(tmpDir, "history.json")

	// Test creating new manager with non-existent file
	mgr, err := createTestManager(testPath, 100)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	if mgr.closed {
		t.Error("Manager should not be closed after creation")
	}

	if len(mgr.Entries()) != 0 {
		t.Errorf("Expected 0 entries in new manager, got %d", len(mgr.Entries()))
	}
}

func TestManagerAppend(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aish-manager-append-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testPath := filepath.Join(tmpDir, "history.json")
	mgr, err := createTestManager(testPath, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	entry := Entry{
		Timestamp: time.Now(),
		Command:   "test command",
		Stdout:    "output",
		Stderr:    "error",
		ExitCode:  1,
		ErrorType: classification.GenericError,
	}

	err = mgr.Append(entry)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	entries := mgr.Entries()
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}

	if entries[0].Command != entry.Command {
		t.Errorf("Expected command %s, got %s", entry.Command, entries[0].Command)
	}
}

func TestManagerAppendMultiple(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aish-manager-multi-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testPath := filepath.Join(tmpDir, "history.json")
	mgr, err := createTestManager(testPath, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	// Add multiple entries
	for i := 0; i < 5; i++ {
		entry := Entry{
			Timestamp: time.Now(),
			Command:   string(rune('A' + i)),
			ExitCode:  i,
		}
		err := mgr.Append(entry)
		if err != nil {
			t.Fatalf("Append %d failed: %v", i, err)
		}
	}

	entries := mgr.Entries()
	if len(entries) != 5 {
		t.Errorf("Expected 5 entries, got %d", len(entries))
	}

	// Entries should be in reverse chronological order (latest first)
	if entries[0].Command != "E" {
		t.Errorf("Expected first entry to be 'E', got %s", entries[0].Command)
	}

	if entries[4].Command != "A" {
		t.Errorf("Expected last entry to be 'A', got %s", entries[4].Command)
	}
}

func TestManagerMaxEntriesLimit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aish-manager-limit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testPath := filepath.Join(tmpDir, "history.json")
	maxEntries := 3
	mgr, err := createTestManager(testPath, maxEntries)
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	// Add more entries than the limit
	for i := 0; i < 5; i++ {
		entry := Entry{
			Timestamp: time.Now(),
			Command:   string(rune('A' + i)),
		}
		mgr.Append(entry)
	}

	entries := mgr.Entries()
	if len(entries) != maxEntries {
		t.Errorf("Expected %d entries (limit), got %d", maxEntries, len(entries))
	}

	// Should keep the most recent entries
	if entries[0].Command != "E" {
		t.Errorf("Expected most recent entry 'E', got %s", entries[0].Command)
	}
}

func TestManagerReplace(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aish-manager-replace-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testPath := filepath.Join(tmpDir, "history.json")
	mgr, err := createTestManager(testPath, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	// Add initial entries
	mgr.Append(Entry{Command: "old1", Timestamp: time.Now()})
	mgr.Append(Entry{Command: "old2", Timestamp: time.Now()})

	// Replace with new entries
	newEntries := []Entry{
		{Command: "new1", Timestamp: time.Now()},
		{Command: "new2", Timestamp: time.Now()},
		{Command: "new3", Timestamp: time.Now()},
	}

	err = mgr.Replace(newEntries)
	if err != nil {
		t.Fatalf("Replace failed: %v", err)
	}

	entries := mgr.Entries()
	if len(entries) != 3 {
		t.Errorf("Expected 3 entries after replace, got %d", len(entries))
	}

	// Check that old entries are gone
	for _, entry := range entries {
		if entry.Command == "old1" || entry.Command == "old2" {
			t.Error("Old entries should not exist after replace")
		}
	}
}

func TestManagerClear(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aish-manager-clear-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testPath := filepath.Join(tmpDir, "history.json")
	mgr, err := createTestManager(testPath, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	// Add entries
	mgr.Append(Entry{Command: "cmd1", Timestamp: time.Now()})
	mgr.Append(Entry{Command: "cmd2", Timestamp: time.Now()})

	// Clear
	err = mgr.Clear()
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	entries := mgr.Entries()
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", len(entries))
	}
}

func TestManagerClose(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aish-manager-close-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testPath := filepath.Join(tmpDir, "history.json")
	mgr, err := createTestManager(testPath, 100)
	if err != nil {
		t.Fatal(err)
	}

	// Add entry
	mgr.Append(Entry{Command: "test", Timestamp: time.Now()})

	// Close
	err = mgr.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if !mgr.closed {
		t.Error("Manager should be marked as closed")
	}

	// Operations after close should fail
	err = mgr.Append(Entry{Command: "after close", Timestamp: time.Now()})
	if err == nil {
		t.Error("Expected error when appending to closed manager")
	}

	// Second close should not error
	err = mgr.Close()
	if err != nil {
		t.Errorf("Second close returned error: %v", err)
	}
}

func TestManagerPersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aish-manager-persist-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testPath := filepath.Join(tmpDir, "history.json")

	// Create manager and add entries
	mgr, err := createTestManager(testPath, 100)
	if err != nil {
		t.Fatal(err)
	}

	mgr.Append(Entry{Command: "persistent1", Timestamp: time.Now()})
	mgr.Append(Entry{Command: "persistent2", Timestamp: time.Now()})

	// Close to flush
	mgr.Close()

	// Create new manager - should load existing entries
	mgr2, err := createTestManager(testPath, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer mgr2.Close()

	entries := mgr2.Entries()
	if len(entries) != 2 {
		t.Errorf("Expected 2 persisted entries, got %d", len(entries))
	}

	// Verify order (latest first)
	if entries[0].Command != "persistent2" {
		t.Errorf("Expected 'persistent2' first, got %s", entries[0].Command)
	}
}

func TestLoadExistingEntriesFromNewlineDelimited(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aish-load-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testPath := filepath.Join(tmpDir, "history.json")

	// Write newline-delimited JSON
	content := `{"timestamp":"2024-01-01T00:00:00Z","command":"cmd1","exit_code":0}
{"timestamp":"2024-01-01T00:01:00Z","command":"cmd2","exit_code":1}
{"timestamp":"2024-01-01T00:02:00Z","command":"cmd3","exit_code":0}
`
	err = os.WriteFile(testPath, []byte(content), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Load entries
	entries, needsRewrite, err := loadExistingEntries(testPath)
	if err != nil {
		t.Fatalf("loadExistingEntries failed: %v", err)
	}

	if needsRewrite {
		t.Error("Newline-delimited format should not need rewrite")
	}

	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}

	// Entries should be in reverse chronological order
	if entries[0].Command != "cmd3" {
		t.Errorf("Expected 'cmd3' first, got %s", entries[0].Command)
	}
}

func TestLoadExistingEntriesFromJSONArray(t *testing.T) {
	// This test is for backward compatibility with old History format
	// The old format was a single JSON array, not JSON-per-line
	t.Skip("Old JSON array format detection is based on '[' prefix, skipping for now")
}

func TestLoadExistingEntriesFromEmptyFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aish-load-empty-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testPath := filepath.Join(tmpDir, "history.json")

	// Create empty file
	err = os.WriteFile(testPath, []byte(""), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Load entries
	entries, needsRewrite, err := loadExistingEntries(testPath)
	if err != nil {
		t.Fatalf("loadExistingEntries failed: %v", err)
	}

	if needsRewrite {
		t.Error("Empty file should not need rewrite")
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 entries from empty file, got %d", len(entries))
	}
}

func TestLoadExistingEntriesNonExistent(t *testing.T) {
	testPath := "/tmp/nonexistent-aish-history-test.json"

	// Load from non-existent file
	entries, needsRewrite, err := loadExistingEntries(testPath)
	if err != nil {
		t.Fatalf("loadExistingEntries should handle non-existent file, got error: %v", err)
	}

	if needsRewrite {
		t.Error("Non-existent file should not need rewrite")
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 entries from non-existent file, got %d", len(entries))
	}
}

func TestCloneEntries(t *testing.T) {
	original := []Entry{
		{Command: "cmd1", Timestamp: time.Now()},
		{Command: "cmd2", Timestamp: time.Now()},
	}

	cloned := cloneEntries(original)

	if len(cloned) != len(original) {
		t.Errorf("Expected %d cloned entries, got %d", len(original), len(cloned))
	}

	// Modify cloned - should not affect original
	cloned[0].Command = "modified"

	if original[0].Command == "modified" {
		t.Error("Modifying clone should not affect original")
	}
}

func TestCloneEntriesEmpty(t *testing.T) {
	cloned := cloneEntries([]Entry{})

	if cloned == nil {
		t.Error("cloneEntries should not return nil for empty slice")
	}

	if len(cloned) != 0 {
		t.Errorf("Expected empty clone, got %d entries", len(cloned))
	}
}

func TestCloneEntriesNil(t *testing.T) {
	cloned := cloneEntries(nil)

	if cloned == nil {
		t.Error("cloneEntries should not return nil for nil input")
	}

	if len(cloned) != 0 {
		t.Errorf("Expected empty clone, got %d entries", len(cloned))
	}
}

// Helper function to create test manager
func createTestManager(path string, maxEntries int) (*Manager, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}

	entries, needsRewrite, err := loadExistingEntries(path)
	if err != nil {
		file.Close()
		return nil, err
	}

	mgr := &Manager{
		entries:      entries,
		file:         file,
		writer:       bufio.NewWriter(file),
		needsRewrite: needsRewrite,
		maxEntries:   maxEntries,
	}

	return mgr, nil
}
