package main

import (
	"fmt"
	"os"
	"os/exec"
)

// executeCommand prints and runs a command, streaming its output.
func executeCommand(command string) {
	fmt.Println("Executing:", command)
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}
