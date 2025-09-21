package ui

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AnimationStyle 定義不同的載入動畫風格
type AnimationStyle int

const (
	StyleSpinner AnimationStyle = iota
	StyleDots
	StyleWave
	StyleProgress
)

// AnimatedSpinner 提供增強的載入動畫，支援時間計數和多種樣式
type AnimatedSpinner struct {
	message   string
	style     AnimationStyle
	isRunning bool
	startTime time.Time
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.RWMutex
}

// NewAnimatedSpinner 建立新的動畫載入器
func NewAnimatedSpinner(message string, style AnimationStyle) *AnimatedSpinner {
	return &AnimatedSpinner{
		message: message,
		style:   style,
	}
}

// Start 開始動畫
func (s *AnimatedSpinner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return
	}

	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.isRunning = true
	s.startTime = time.Now()

	s.wg.Add(1)
	go s.animate()
}

// Stop 停止動畫
func (s *AnimatedSpinner) Stop(success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}

	s.cancel()
	s.wg.Wait()
	s.isRunning = false

	// 清除當前行並顯示結果
	fmt.Print("\r\033[K")

	duration := time.Since(s.startTime)
	if success {
		fmt.Printf("✅ %s (%.1fs)\n", s.message, duration.Seconds())
	} else {
		fmt.Printf("❌ %s failed (%.1fs)\n", s.message, duration.Seconds())
	}
}

// animate 執行動畫循環
func (s *AnimatedSpinner) animate() {
	defer s.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	frame := 0

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.displayFrame(frame)
			frame++
		}
	}
}

// displayFrame 顯示動畫幀
func (s *AnimatedSpinner) displayFrame(frame int) {
	s.mu.RLock()
	duration := time.Since(s.startTime)
	s.mu.RUnlock()

	var animation string

	switch s.style {
	case StyleSpinner:
		chars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		animation = chars[frame%len(chars)]
	case StyleDots:
		dots := frame%4 + 1
		animation = ""
		for i := 0; i < dots; i++ {
			animation += "●"
		}
		for i := dots; i < 4; i++ {
			animation += "○"
		}
	case StyleWave:
		chars := []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█", "▇", "▆", "▅", "▄", "▃", "▂"}
		animation = chars[frame%len(chars)]
	case StyleProgress:
		chars := []string{"▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"}
		pos := frame % 8
		bar := ""
		for i := 0; i < 8; i++ {
			if i < pos {
				bar += "█"
			} else if i == pos {
				bar += chars[frame%len(chars)]
			} else {
				bar += "░"
			}
		}
		animation = "[" + bar + "]"
	}

	// 顯示動畫與時間計數
	fmt.Printf("\r%s %s (%.1fs)", animation, s.message, duration.Seconds())
}

// IsRunning 檢查是否正在運行
func (s *AnimatedSpinner) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}

// UpdateMessage 更新顯示訊息
func (s *AnimatedSpinner) UpdateMessage(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = message
}
