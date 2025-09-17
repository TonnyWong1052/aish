package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Confirm displays a prompt and waits for a yes/no answer.
func Confirm(prompt string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(prompt)
		input, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}
		input = strings.TrimSpace(strings.ToLower(input))
		if input == "y" || input == "yes" {
			return true, nil
		}
		if input == "n" || input == "no" {
			return false, nil
		}
	}
}
