package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/pterm/pterm"
)

// HookSelectionDisplay implements hook selection with "(Press <space> to select, <a> to toggle all, <i> to invert selection, and <enter> to proceed)" rule.
func HookSelectionDisplay(options []string, title string, defaultOptions []string) ([]string, error) {
	pterm.DefaultHeader.Println(title)

	// Initialize selection state
	selected := make(map[int]bool)
	for i, opt := range options {
		for _, defOpt := range defaultOptions {
			if opt == defOpt {
				selected[i] = true
				break
			}
		}
	}

	// Display options and instructions
	for {
		displayOptions(options, selected)

		pterm.Println("\nInstructions:")
		pterm.Println("  <space> - Select/deselect the current item")
		pterm.Println("  <a> - Toggle all on/off")
		pterm.Println("  <i> - Invert selection")
		pterm.Println("  <enter> - Confirm selection and continue")
		pterm.Println("  <q> - Quit without saving")
		pterm.Println()

		// Get user input
		reader := bufio.NewReader(os.Stdin)
		pterm.Print("Enter action: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		input = strings.TrimSpace(input)

		switch input {
		case "":
			// Enter key, confirm selection
			var result []string
			for i, isSelected := range selected {
				if isSelected {
					result = append(result, options[i])
				}
			}
			return result, nil

		case "q":
			// Quit without saving
			return nil, fmt.Errorf("user cancelled operation")

		case "a":
			// Toggle all on/off
			allSelected := true
			for _, isSelected := range selected {
				if !isSelected {
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

		case " ":
			// Space key, user needs to specify item to select
			pterm.Println("Enter item number to select (1-" + fmt.Sprint(len(options)) + "): ")
			numInput, err := reader.ReadString('\n')
			if err != nil {
				return nil, err
			}

			numInput = strings.TrimSpace(numInput)
			var index int
			_, err = fmt.Sscanf(numInput, "%d", &index)
			if err != nil || index < 1 || index > len(options) {
				pterm.Error.Println("Invalid item number")
				continue
			}

			selected[index-1] = !selected[index-1]

		default:
			// Try to parse as a number (direct item selection)
			var index int
			_, err := fmt.Sscanf(input, "%d", &index)
			if err == nil && index >= 1 && index <= len(options) {
				selected[index-1] = !selected[index-1]
			} else {
				pterm.Error.Println("Invalid input")
			}
		}
	}
}

// displayOptions displays the list of options with their selection status.
func displayOptions(options []string, selected map[int]bool) {
	pterm.Println("\nAvailable options:")
	for i, option := range options {
		if selected[i] {
			pterm.Println(fmt.Sprintf("  [%d] %s %s", i+1, pterm.Green("[✓]"), option))
		} else {
			pterm.Println(fmt.Sprintf("  [%d] %s %s", i+1, pterm.Red("[ ]"), option))
		}
	}
}
