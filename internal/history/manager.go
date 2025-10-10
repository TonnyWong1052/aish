package history

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Manager maintains persistent write flow for history records, avoiding rewriting the entire file on each operation.
type Manager struct {
	mu           sync.RWMutex
	entries      []Entry // Store with latest records first
	file         *os.File
	writer       *bufio.Writer
	needsRewrite bool
	maxEntries   int
	closed       bool
}

var (
	managerOnce sync.Once
	managerInst *Manager
	managerErr  error
)

func getDefaultManager() (*Manager, error) {
	managerOnce.Do(func() {
		managerInst, managerErr = newManager()
	})
	return managerInst, managerErr
}

func newManager() (*Manager, error) {
	path, err := getHistoryPath()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}

	entries, needsRewrite, err := loadExistingEntries(path)
	if err != nil {
		_ = file.Close()
		return nil, err
	}

	mgr := &Manager{
		entries:      entries,
		file:         file,
		writer:       bufio.NewWriter(file),
		needsRewrite: needsRewrite,
		maxEntries:   determineHistoryLimit(),
	}

	mgr.enforceLimitLocked()

	if mgr.needsRewrite {
		if err := mgr.rewriteLocked(); err != nil {
			_ = file.Close()
			return nil, err
		}
		return mgr, nil
	}

	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		_ = file.Close()
		return nil, err
	}

	return mgr, nil
}

func (m *Manager) Append(entry Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return errors.New("history manager closed")
	}

	m.entries = append([]Entry{entry}, m.entries...)
	m.enforceLimitLocked()

	if m.needsRewrite {
		return m.rewriteLocked()
	}

	if err := m.writeEntry(entry); err != nil {
		m.needsRewrite = true
		return err
	}

	return m.writer.Flush()
}

func (m *Manager) Entries() []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	copied := make([]Entry, len(m.entries))
	copy(copied, m.entries)
	return copied
}

func (m *Manager) Replace(entries []Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return errors.New("history manager closed")
	}

	m.entries = cloneEntries(entries)
	m.enforceLimitLocked()
	return m.rewriteLocked()
}

func (m *Manager) Clear() error {
	return m.Replace(nil)
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	var err error
	if m.needsRewrite {
		err = m.rewriteLocked()
	} else {
		err = m.writer.Flush()
	}

	if cerr := m.file.Close(); err == nil {
		err = cerr
	}

	m.closed = true
	return err
}

func (m *Manager) enforceLimitLocked() {
	if m.maxEntries > 0 && len(m.entries) > m.maxEntries {
		m.entries = m.entries[:m.maxEntries]
		m.needsRewrite = true
	}
}

func (m *Manager) writeEntry(entry Entry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := m.writer.Write(data); err != nil {
		return err
	}
	if err := m.writer.WriteByte('\n'); err != nil {
		return err
	}
	return nil
}

func (m *Manager) rewriteLocked() error {
	if _, err := m.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := m.file.Truncate(0); err != nil {
		return err
	}
	m.writer.Reset(m.file)

	for i := len(m.entries) - 1; i >= 0; i-- {
		if err := m.writeEntry(m.entries[i]); err != nil {
			return err
		}
	}

	if err := m.writer.Flush(); err != nil {
		return err
	}
	if _, err := m.file.Seek(0, io.SeekEnd); err != nil {
		return err
	}

	m.needsRewrite = false
	return nil
}

func loadExistingEntries(path string) ([]Entry, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []Entry{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return []Entry{}, false, nil
	}

	if data[0] == '[' {
		var hist History
		if err := json.Unmarshal(data, &hist); err != nil {
			return nil, false, err
		}
		return cloneEntries(hist.Entries), true, nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var chronological []Entry
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, false, err
		}
		chronological = append(chronological, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, false, err
	}

	reversed := make([]Entry, len(chronological))
	for i := range chronological {
		reversed[i] = chronological[len(chronological)-1-i]
	}

	return reversed, false, nil
}

func cloneEntries(entries []Entry) []Entry {
	if len(entries) == 0 {
		return []Entry{}
	}
	cloned := make([]Entry, len(entries))
	copy(cloned, entries)
	return cloned
}
