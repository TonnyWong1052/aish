package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/pterm/pterm"
)

// Suggestion represents the data to be presented to the user.
// It decouples the UI from the internal LLM suggestion format.
type Suggestion struct {
	Explanation string
	Command     string
	Title       string // e.g., "AI Suggestion" or "Generated Command"
}

// Presenter handles the standardized display of suggestions and user interaction.
type Presenter struct {
	spinner *pterm.SpinnerPrinter
}

// NewPresenter creates a new Presenter.
func NewPresenter() *Presenter {
	return &Presenter{}
}

// Render displays a suggestion and handles user input.
// Returns the user's new prompt, whether to proceed, and any error.
func (p *Presenter) Render(suggestion Suggestion) (string, bool, error) {
	pterm.DefaultHeader.Println(suggestion.Title)

	if suggestion.Explanation != "" {
		pterm.Println(pterm.Red("Explanation:"))
		pterm.Println(suggestion.Explanation)
		pterm.Println()
	}

	pterm.Println(pterm.Green("Suggested Command:"))
	pterm.Println(pterm.LightGreen(suggestion.Command))
	pterm.Println()

	pterm.Println("Options:")
	pterm.Println(pterm.LightWhite("  [Enter] - Execute the suggested command"))
	pterm.Println(pterm.LightWhite("  [n/no]  - Reject and exit"))
	pterm.Println(pterm.LightWhite("  [other] - Provide a new prompt for a different suggestion"))
	pterm.Println()
	pterm.Print("Select an option: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", false, fmt.Errorf("error reading user input: %w", err)
	}
	input = strings.TrimSpace(strings.ToLower(input))

	switch input {
	case "": // Enter
		return "", true, nil
	case "n", "no":
		pterm.Warning.Println("Operation cancelled by user.")
		return "", false, nil
	default:
		return input, true, nil
	}
}

// ShowLoading displays a spinner with a message.
func (p *Presenter) ShowLoading(message string) {
	p.spinner, _ = pterm.DefaultSpinner.Start(message)
}

// StopLoading stops the spinner.
func (p *Presenter) StopLoading(success bool) {
	if p.spinner != nil {
		if success {
			p.spinner.Success("SUCCESS")
		} else {
			p.spinner.Fail()
		}
		p.spinner = nil
	}
}