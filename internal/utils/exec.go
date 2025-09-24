package utils

import (
	"fmt"
	"os/exec"
	"strings"
)

// ExecResult represents the result of command execution
type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// Exec executes a command with arguments
func Exec(command string, args ...string) error {
	LogDebug("Executing: %s %s", command, strings.Join(args, " "))

	cmd := exec.Command(command, args...)
	return cmd.Run()
}

// ExecWithOutput executes a command and returns output
func ExecWithOutput(command string, args ...string) (*ExecResult, error) {
	LogDebug("Executing with output: %s %s", command, strings.Join(args, " "))

	cmd := exec.Command(command, args...)
	stdout, err := cmd.Output()

	result := &ExecResult{
		Stdout: string(stdout),
	}

	if exitError, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitError.ExitCode()
		result.Stderr = string(exitError.Stderr)
	}

	return result, err
}

// ExecChroot executes a command in chroot environment
func ExecChroot(command string, args ...string) error {
	var fullCommand string
	if len(args) == 0 {
		fullCommand = command
	} else {
		fullCommand = fmt.Sprintf("%s %s", command, strings.Join(args, " "))
	}

	LogDebug("Executing in chroot: %s", fullCommand)

	cmd := exec.Command("arch-chroot", "/mnt", "bash", "-c", fullCommand)
	return cmd.Run()
}

// ExecChrootWithOutput executes a command in chroot and returns output
func ExecChrootWithOutput(command string, args ...string) (*ExecResult, error) {
	var fullCommand string
	if len(args) == 0 {
		fullCommand = command
	} else {
		fullCommand = fmt.Sprintf("%s %s", command, strings.Join(args, " "))
	}

	LogDebug("Executing in chroot with output: %s", fullCommand)

	cmd := exec.Command("arch-chroot", "/mnt", "bash", "-c", fullCommand)
	stdout, err := cmd.Output()

	result := &ExecResult{
		Stdout: string(stdout),
	}

	if exitError, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitError.ExitCode()
		result.Stderr = string(exitError.Stderr)
	}

	return result, err
}

// ExecEval executes a command and logs success/failure
func ExecEval(err error, logMsg string) {
	if err != nil {
		var exitCode int
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = 1
		}
		Crash(fmt.Sprintf("%s ERROR: %v", logMsg, err), exitCode)
	} else {
		LogInfo("%s", logMsg)
	}
}
