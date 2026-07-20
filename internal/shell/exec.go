// Package shell provides a unified interface for executing system commands.
package shell

import (
	"os/exec"
	"strings"
)

// Run executes a command and returns stdout (trimmed).
func Run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// RunSilent executes a command discarding all output.
func RunSilent(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

// RunInput executes a command with stdin input.
func RunInput(input, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// LookPath returns the full path of a binary, or an error.
func LookPath(bin string) (string, error) {
	return exec.LookPath(bin)
}

// Has checks if a binary exists on PATH.
func Has(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

