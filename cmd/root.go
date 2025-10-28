package cmd

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	verbose bool
	jsonLog bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "terranovate",
	Short: "Automated Terraform module update tool",
	Long: `Terranovate automatically detects outdated Terraform modules,
validates updates via terraform plan, and opens automated Pull Requests.

Supports both Terraform Registry and Git source modules.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Setup logging
		if jsonLog {
			log.Logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
		} else {
			log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
				With().Timestamp().Logger()
		}

		if verbose {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		} else {
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", ".terranovate.yaml",
		"config file (default is .terranovate.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false,
		"verbose output")
	rootCmd.PersistentFlags().BoolVar(&jsonLog, "json-log", false,
		"output logs in JSON format")
}
