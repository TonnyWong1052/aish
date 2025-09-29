package ui

import (
	"fmt"
	"os"
	"strings"

	"atomicgo.dev/cursor"
	"atomicgo.dev/keyboard"
	"atomicgo.dev/keyboard/keys"
)

// MultiSelectNoHelp renders a simple multiselect without the default PTerm help line.
// Keys: space = toggle current, a = toggle all, i = invert, enter = confirm.
// Arrow keys move the cursor; no type-to-filter.
func MultiSelectNoHelp(prompt string, options []string, defaultOptions []string) ([]string, error) {
	if len(options) == 0 {
		return nil, fmt.Errorf("no options provided")
	}

	// Initialize selection state
	selected := make([]bool, len(options))
	if len(defaultOptions) > 0 {
		for i, opt := range options {
			for _, def := range defaultOptions {
				if opt == def {
					selected[i] = true
					break
				}
			}
		}
	}

	selectedIdx := 0
	maxHeight := 5
	top := 0
	if maxHeight > len(options) {
		maxHeight = len(options)
	}

	// Print prompt once
	if prompt != "" {
		fmt.Fprint(os.Stdout, prompt)
		if !strings.HasSuffix(prompt, "\n") {
			fmt.Fprintln(os.Stdout)
		}
	}

	// Calculate visible window
	updateWindow := func() {
		if selectedIdx < top {
			top = selectedIdx
		}
		if selectedIdx >= top+maxHeight {
			top = selectedIdx - maxHeight + 1
		}
	}

	// Render a single line
	renderLine := func(i int) string {
		checkmark := "[x]"  // 未選中顯示 [x]
		if selected[i] {
			checkmark = "[o]"  // 選中顯示 [o]
		}
		prefix := "  "
		if i == selectedIdx {
			prefix = "> "
		}
		return fmt.Sprintf("%s%s %s", prefix, checkmark, options[i])
	}

	// Initial render: print all visible lines
	updateWindow()
	end := top + maxHeight
	if end > len(options) {
		end = len(options)
	}
	for i := top; i < end; i++ {
		fmt.Fprintln(os.Stdout, renderLine(i))
	}

	cursor.Hide()
	defer cursor.Show()

	lineCount := end - top

	if err := keyboard.Listen(func(k keys.Key) (bool, error) {
		switch k.Code {
		case keys.CtrlC:
			return true, fmt.Errorf("cancelled")
		case keys.Up, keys.CtrlP:
			if selectedIdx > 0 {
				selectedIdx--
			} else {
				selectedIdx = len(options) - 1
			}
		case keys.Down, keys.CtrlN:
			if selectedIdx < len(options)-1 {
				selectedIdx++
			} else {
				selectedIdx = 0
			}
		case keys.Space:
			selected[selectedIdx] = !selected[selectedIdx]
		case keys.RuneKey:
			// Letter-based actions
			switch k.String() {
			case "a":
				// Toggle all
				allSelected := true
				for _, v := range selected {
					if !v {
						allSelected = false
						break
					}
				}
				for i := range selected {
					selected[i] = !allSelected
				}
			case "i":
				// Invert selection
				for i := range selected {
					selected[i] = !selected[i]
				}
			}
		case keys.Enter:
			return true, nil
		}

    // Redraw: always repaint the visible window to keep alignment stable
    updateWindow()
    end := top + maxHeight
    if end > len(options) {
        end = len(options)
    }

    // Move to the top of the block and redraw all lines from column 0
    cursor.Up(lineCount)
    cursor.StartOfLine()
    for i := top; i < end; i++ {
        // 清除本行並重畫（逐行輸出，保證左對齊且避免殘影）
        fmt.Fprint(os.Stdout, "\r\033[K")
        fmt.Fprintln(os.Stdout, renderLine(i))
    }
    lineCount = end - top

		return false, nil
	}); err != nil {
		return nil, err
	}

	// Build result
	var result []string
	for i, v := range selected {
		if v {
			result = append(result, options[i])
		}
	}
	return result, nil
}
