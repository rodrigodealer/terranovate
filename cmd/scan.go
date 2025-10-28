package cmd

import (
	"fmt"
	"os"

	"github.com/heyjobs/terranovate/internal/scanner"
	"github.com/heyjobs/terranovate/pkg/config"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var scanPath string

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan Terraform files for module usage",
	Long: `Scan scans the specified path for Terraform files and extracts
all module blocks, including their sources and versions.

Example:
  terranovate scan --path ./infrastructure`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		cfg, err := loadConfig()
		if err != nil {
			log.Warn().Err(err).Msg("failed to load config, using defaults")
			cfg = config.Default()
		}

		// Override with command-line flags
		if scanPath != "" {
			cfg.Scanner.Include = []string{"*.tf"}
		}

		path := scanPath
		if path == "" {
			path = "."
		}

		// Create scanner
		s := scanner.New(
			path,
			cfg.Scanner.Exclude,
			cfg.Scanner.Include,
			cfg.Scanner.Recursive,
		)

		// Scan for modules
		log.Info().Str("path", path).Msg("scanning for terraform modules")
		modules, err := s.Scan()
		if err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		// Output results
		if len(modules) == 0 {
			fmt.Println("No Terraform modules found.")
			return nil
		}

		fmt.Printf("Found %d module(s):\n\n", len(modules))
		for i, module := range modules {
			fmt.Printf("%d. %s\n", i+1, module.Name)
			fmt.Printf("   Source: %s\n", module.Source)
			fmt.Printf("   Version: %s\n", module.Version)
			fmt.Printf("   Type: %s\n", module.SourceType)
			fmt.Printf("   File: %s:%d\n", module.FilePath, module.Line)
			fmt.Println()
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().StringVarP(&scanPath, "path", "p", "",
		"path to scan for Terraform files (default: current directory)")
}

// loadConfig loads the configuration file
func loadConfig() (*config.Config, error) {
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", cfgFile)
	}

	return config.Load(cfgFile)
}
