package history

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// OptimizedManager high-performance history record manager
type OptimizedManager struct {
	mu                 sync.RWMutex
	entries            []Entry
	batchBuffer        []Entry
	maxEntries         int
	batchSize          int
	flushInterval      time.Duration
	compressionEnabled bool

	// Batch write control
	pendingWrites int
	lastFlushTime time.Time

	// File management
	currentFile *os.File
	archiveDir  string
	stats       ManagerStats

	// Control channels
	flushChan chan struct{}
	stopChan  chan struct{}
	stopped   bool
}

// ManagerStats manager statistics information
type ManagerStats struct {
	TotalEntries   int64
	BatchWrites    int64
	Compressions   int64
	AvgBatchSize   float64
	AvgFlushTime   time.Duration
	DiskUsageMB    float64
	LastCompaction time.Time
}

// OptimizedManagerConfig 優化管理器配置
type OptimizedManagerConfig struct {
	MaxEntries         int
	BatchSize          int
	FlushInterval      time.Duration
	CompressionEnabled bool
	ArchiveAfterDays   int
	MaxDiskUsageMB     float64
}

// DefaultOptimizedConfig 默認優化配置
func DefaultOptimizedConfig() OptimizedManagerConfig {
	return OptimizedManagerConfig{
		MaxEntries:         1000,
		BatchSize:          10, // 10 條記錄批量寫入
		FlushInterval:      5 * time.Second,
		CompressionEnabled: true,
		ArchiveAfterDays:   30,
		MaxDiskUsageMB:     100.0, // 100MB 磁盤使用限制
	}
}

// NewOptimizedManager 創建新的優化歷史管理器
func NewOptimizedManager(config OptimizedManagerConfig) (*OptimizedManager, error) {
	path, err := getHistoryPath()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	// 創建歸檔目錄
	archiveDir := filepath.Join(filepath.Dir(path), "archive")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}

	mgr := &OptimizedManager{
		maxEntries:         config.MaxEntries,
		batchSize:          config.BatchSize,
		flushInterval:      config.FlushInterval,
		compressionEnabled: config.CompressionEnabled,
		currentFile:        file,
		archiveDir:         archiveDir,
		batchBuffer:        make([]Entry, 0, config.BatchSize),
		flushChan:          make(chan struct{}, 1),
		stopChan:           make(chan struct{}),
		lastFlushTime:      time.Now(),
	}

	// 加載現有條目
	if err := mgr.loadExistingEntries(); err != nil {
		file.Close()
		return nil, err
	}

	// 啟動後台處理協程
	go mgr.backgroundProcessor()

	return mgr, nil
}

// AppendBatch 批量添加條目
func (om *OptimizedManager) AppendBatch(entries []Entry) error {
	om.mu.Lock()
	defer om.mu.Unlock()

	if om.stopped {
		return fmt.Errorf("manager is stopped")
	}

	// 添加到批量緩衝區
	om.batchBuffer = append(om.batchBuffer, entries...)
	om.pendingWrites += len(entries)

	// 檢查是否需要立即刷新
	if len(om.batchBuffer) >= om.batchSize {
		return om.flushBatchLocked()
	}

	// 觸發定時刷新
	om.triggerFlush()
	return nil
}

// Append 添加單個條目（高性能版本）
func (om *OptimizedManager) Append(entry Entry) error {
	return om.AppendBatch([]Entry{entry})
}

// Entries 獲取所有條目
func (om *OptimizedManager) Entries() []Entry {
	om.mu.RLock()
	defer om.mu.RUnlock()

	// 合併內存中的條目和緩衝區的條目
	combined := make([]Entry, 0, len(om.entries)+len(om.batchBuffer))

	// 添加緩衝區中的最新條目
	for i := len(om.batchBuffer) - 1; i >= 0; i-- {
		combined = append(combined, om.batchBuffer[i])
	}

	// 添加已持久化的條目
	combined = append(combined, om.entries...)

	return combined
}

// Flush 立即刷新所有緩衝的條目
func (om *OptimizedManager) Flush() error {
	om.mu.Lock()
	defer om.mu.Unlock()

	return om.flushBatchLocked()
}

// Close 關閉管理器
func (om *OptimizedManager) Close() error {
	om.mu.Lock()
	defer om.mu.Unlock()

	if om.stopped {
		return nil
	}

	om.stopped = true
	close(om.stopChan)

	// 最後一次刷新
	if err := om.flushBatchLocked(); err != nil {
		return err
	}

	if om.currentFile != nil {
		return om.currentFile.Close()
	}

	return nil
}

// GetStats 獲取統計信息
func (om *OptimizedManager) GetStats() ManagerStats {
	om.mu.RLock()
	defer om.mu.RUnlock()

	stats := om.stats
	stats.TotalEntries = int64(len(om.entries) + len(om.batchBuffer))

	// 計算磁盤使用
	if info, err := om.currentFile.Stat(); err == nil {
		stats.DiskUsageMB = float64(info.Size()) / (1024 * 1024)
	}

	return stats
}

// Compact 壓縮歷史文件
func (om *OptimizedManager) Compact() error {
	om.mu.Lock()
	defer om.mu.Unlock()

	// 首先刷新所有緩衝
	if err := om.flushBatchLocked(); err != nil {
		return err
	}

	// 執行歸檔和壓縮
	return om.compactAndArchive()
}

// 內部方法

func (om *OptimizedManager) backgroundProcessor() {
	flushTicker := time.NewTicker(om.flushInterval)
	defer flushTicker.Stop()

	for {
		select {
		case <-flushTicker.C:
			om.periodicFlush()
		case <-om.flushChan:
			om.periodicFlush()
		case <-om.stopChan:
			return
		}
	}
}

func (om *OptimizedManager) periodicFlush() {
	om.mu.Lock()
	defer om.mu.Unlock()

	if len(om.batchBuffer) > 0 {
		_ = om.flushBatchLocked()
	}
}

func (om *OptimizedManager) flushBatchLocked() error {
	if len(om.batchBuffer) == 0 {
		return nil
	}

	start := time.Now()

	// 準備批量寫入數據
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)

	for _, entry := range om.batchBuffer {
		if err := encoder.Encode(entry); err != nil {
			return fmt.Errorf("encode entry: %w", err)
		}
	}

	// 寫入文件
	if _, err := om.currentFile.Write(buffer.Bytes()); err != nil {
		return fmt.Errorf("write batch: %w", err)
	}

	if err := om.currentFile.Sync(); err != nil {
		return fmt.Errorf("sync file: %w", err)
	}

	// 更新內存中的條目
	newEntries := make([]Entry, len(om.batchBuffer))
	copy(newEntries, om.batchBuffer)

	// 反轉順序（最新的在前）
	for i := len(newEntries)/2 - 1; i >= 0; i-- {
		opp := len(newEntries) - 1 - i
		newEntries[i], newEntries[opp] = newEntries[opp], newEntries[i]
	}

	om.entries = append(newEntries, om.entries...)

	// 強制執行大小限制
	if len(om.entries) > om.maxEntries {
		om.entries = om.entries[:om.maxEntries]
	}

	// 更新統計
	batchSize := len(om.batchBuffer)
	om.stats.BatchWrites++
	om.stats.AvgBatchSize = (om.stats.AvgBatchSize + float64(batchSize)) / 2
	flushTime := time.Since(start)
	om.stats.AvgFlushTime = (om.stats.AvgFlushTime + flushTime) / 2
	om.lastFlushTime = time.Now()

	// 清空緩衝區
	om.batchBuffer = om.batchBuffer[:0]
	om.pendingWrites = 0

	return nil
}

func (om *OptimizedManager) triggerFlush() {
	select {
	case om.flushChan <- struct{}{}:
	default:
		// 通道已滿，忽略
	}
}

func (om *OptimizedManager) loadExistingEntries() error {
	// 重置文件指針
	if _, err := om.currentFile.Seek(0, 0); err != nil {
		return err
	}

	var entries []Entry
	scanner := bufio.NewScanner(om.currentFile)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // 跳過無效條目
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// 反轉順序（最新的在前）
	for i := len(entries)/2 - 1; i >= 0; i-- {
		opp := len(entries) - 1 - i
		entries[i], entries[opp] = entries[opp], entries[i]
	}

	om.entries = entries

	// 移動到文件末尾以便追加
	if _, err := om.currentFile.Seek(0, 2); err != nil {
		return err
	}

	return nil
}

func (om *OptimizedManager) compactAndArchive() error {
	// 創建歸檔文件
	archiveFile := filepath.Join(om.archiveDir, fmt.Sprintf("history_%s.json.gz",
		time.Now().Format("2006-01-02_15-04-05")))

	file, err := os.Create(archiveFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// 使用 gzip 壓縮
	var writer *gzip.Writer
	if om.compressionEnabled {
		writer = gzip.NewWriter(file)
		defer writer.Close()
	}

	// 寫入歸檔數據
	encoder := json.NewEncoder(file)
	if writer != nil {
		encoder = json.NewEncoder(writer)
	}

	archive := struct {
		ArchivedAt time.Time `json:"archived_at"`
		Entries    []Entry   `json:"entries"`
	}{
		ArchivedAt: time.Now(),
		Entries:    om.entries,
	}

	if err := encoder.Encode(archive); err != nil {
		return err
	}

	if writer != nil {
		if err := writer.Close(); err != nil {
			return err
		}
	}

	// 重建主文件
	if err := om.currentFile.Truncate(0); err != nil {
		return err
	}
	if _, err := om.currentFile.Seek(0, 0); err != nil {
		return err
	}

	// 只保留最近的條目
	recentEntries := om.entries
	if len(recentEntries) > om.maxEntries/2 {
		recentEntries = recentEntries[:om.maxEntries/2]
	}

	// 重新寫入最近的條目
	encoder = json.NewEncoder(om.currentFile)
	for i := len(recentEntries) - 1; i >= 0; i-- {
		if err := encoder.Encode(recentEntries[i]); err != nil {
			return err
		}
	}

	om.entries = recentEntries
	om.stats.Compressions++
	om.stats.LastCompaction = time.Now()

	return om.currentFile.Sync()
}
