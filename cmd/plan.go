package cmd

import (
	"context"
	"fmt"

	"github.com/heyjobs/terranovate/internal/terraform"
	"github.com/heyjobs/terranovate/pkg/config"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var planPath string

// planCmd represents the plan command
var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Run terraform plan to validate changes",
	Long: `Plan runs terraform init and terraform plan in the specified directory
to validate that the infrastructure changes are valid.

This command is typically used after updating module versions to ensure
the changes don't break the infrastructure.

Example:
  terranovate plan --path ./infrastructure`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Load configuration
		cfg, err := loadConfig()
		if err != nil {
			log.Warn().Err(err).Msg("failed to load config, using defaults")
			cfg = config.Default()
		}

		path := planPath
		if path == "" {
			path = cfg.Terraform.WorkingDir
		}
		if path == "" {
			path = "."
		}

		// Create Terraform runner
		runner, err := terraform.New(path, cfg.Terraform.BinaryPath, cfg.Terraform.Env)
		if err != nil {
			return fmt.Errorf("failed to create terraform runner: %w", err)
		}

		// Run terraform init
		fmt.Println("Running terraform init...")
		if err := runner.Init(ctx); err != nil {
			return fmt.Errorf("terraform init failed: %w", err)
		}
		fmt.Println("✓ Terraform init completed")

		// Run terraform plan
		fmt.Println("Running terraform plan...")
		planResult, err := runner.Plan(ctx)
		if err != nil {
			return fmt.Errorf("terraform plan failed: %w", err)
		}

		// Output results
		if planResult.Success {
			fmt.Println("✓ Terraform plan completed successfully")
			fmt.Println(planResult.Output)

			if planResult.HasChanges {
				fmt.Println("\n⚠️  Infrastructure changes detected:")
				if planResult.ResourcesAdd > 0 {
					fmt.Printf("   + %d resource(s) to add\n", planResult.ResourcesAdd)
				}
				if planResult.ResourcesChange > 0 {
					fmt.Printf("   ~ %d resource(s) to change\n", planResult.ResourcesChange)
				}
				if planResult.ResourcesDestroy > 0 {
					fmt.Printf("   - %d resource(s) to destroy\n", planResult.ResourcesDestroy)
				}
			} else {
				fmt.Println("\n✨ No infrastructure changes required")
			}
		} else {
			fmt.Println("✗ Terraform plan failed")
			fmt.Println(planResult.ErrorMessage)
			return fmt.Errorf("plan validation failed")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(planCmd)

	planCmd.Flags().StringVarP(&planPath, "path", "p", "",
		"path to Terraform working directory (default: current directory)")
}
