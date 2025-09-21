package ui

import (
	"fmt"
	"strings"

	"atomicgo.dev/cursor"
	"atomicgo.dev/keyboard"
	"atomicgo.dev/keyboard/keys"
	"github.com/pterm/pterm"
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

	// Render function returns the full content string for the area
	render := func() string {
		var b strings.Builder
		if prompt != "" {
			b.WriteString(prompt)
			if !strings.HasSuffix(prompt, "\n") {
				b.WriteString("\n")
			}
		}

		// Clamp window
		if selectedIdx < top {
			top = selectedIdx
		}
		if selectedIdx >= top+maxHeight {
			top = selectedIdx - maxHeight + 1
		}
		end := top + maxHeight
		if end > len(options) {
			end = len(options)
		}

		for i := top; i < end; i++ {
			checkmark := fmt.Sprintf("[%s]", pterm.ThemeDefault.Checkmark.Unchecked)
			if selected[i] {
				checkmark = fmt.Sprintf("[%s]", pterm.ThemeDefault.Checkmark.Checked)
			}
			if i == selectedIdx {
				b.WriteString(pterm.Sprintf("%s %s %s\n", pterm.ThemeDefault.SecondaryStyle.Sprint(">"), checkmark, options[i]))
			} else {
				b.WriteString(pterm.Sprintf("  %s %s\n", checkmark, options[i]))
			}
		}

		// Intentionally no extra help line here
		return b.String()
	}

	area, err := pterm.DefaultArea.Start(render())
	if err != nil {
		return nil, err
	}
	defer area.Stop()

	cursor.Hide()
	defer cursor.Show()

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
			area.Update(render())
		case keys.Down, keys.CtrlN:
			if selectedIdx < len(options)-1 {
				selectedIdx++
			} else {
				selectedIdx = 0
			}
			area.Update(render())
		case keys.Space:
			selected[selectedIdx] = !selected[selectedIdx]
			area.Update(render())
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
				area.Update(render())
			case "i":
				// Invert selection
				for i := range selected {
					selected[i] = !selected[i]
				}
				area.Update(render())
			}
		case keys.Enter:
			return true, nil
		}
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
