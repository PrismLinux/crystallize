package cli

import (
	"crystallize-cli/internal/config"
	"crystallize-cli/internal/utils"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	verbose int
)

var rootCmd = &cobra.Command{
	Use:     "crystallize-cli",
	Short:   "Crystallize Linux installation tool",
	Long:    "A comprehensive PrismLinux installation tool with automated partitioning, package management, and desktop environment setup.",
	Version: "0.3.0",
}

var configCmd = &cobra.Command{
	Use:   "config [config-file]",
	Short: "Read and execute Crystallize installation config",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, err := filepath.Abs(args[0])
		if err != nil {
			return fmt.Errorf("failed to resolve config path: %w", err)
		}

		utils.LogInfo("Reading configuration from: %s", configPath)
		return config.ReadConfig(configPath)
	},
}

func init() {
	rootCmd.PersistentFlags().CountVarP(&verbose, "verbose", "v", "verbose output (-v, -vv, -vvv)")

	rootCmd.AddCommand(configCmd)

	cobra.OnInitialize(initConfig)
}

func initConfig() {
	utils.InitLogging(verbose)
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}
