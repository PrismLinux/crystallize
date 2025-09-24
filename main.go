package main

import (
	"crystallize-cli/internal/cli"
	"crystallize-cli/internal/utils"
	"fmt"
	"os"
)

func main() {
	// Set up panic handler
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Panic occurred: %v\n", r)
			os.Exit(1)
		}
	}()

	if err := cli.Execute(); err != nil {
		utils.LogError("Command execution failed: %v", err)
		os.Exit(1)
	}
}
