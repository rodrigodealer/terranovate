package cmd

import (
	"context"
	"fmt"

	"github.com/heyjobs/terranovate/internal/github"
	"github.com/heyjobs/terranovate/internal/scanner"
	"github.com/heyjobs/terranovate/internal/terraform"
	"github.com/heyjobs/terranovate/internal/version"
	"github.com/heyjobs/terranovate/pkg/config"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	prPath   string
	prRepo   string
	prOwner  string
	skipPlan bool
)

// prCmd represents the pr command
var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Create pull requests for module updates",
	Long: `PR scans for outdated modules, validates them with terraform plan,
and creates GitHub pull requests for each update.

Example:
  terranovate pr --repo heyjobs/platform-infra --path ./infrastructure`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Load configuration
		cfg, err := loadConfig()
		if err != nil {
			log.Warn().Err(err).Msg("failed to load config, using defaults")
			cfg = config.Default()
		}

		// Validate configuration
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("invalid configuration: %w", err)
		}

		// Override with command-line flags
		if prRepo != "" {
			cfg.GitHub.Repo = prRepo
		}
		if prOwner != "" {
			cfg.GitHub.Owner = prOwner
		}

		// Parse repo if provided as owner/repo
		if cfg.GitHub.Repo != "" && cfg.GitHub.Owner == "" {
			owner, repo, err := parseRepo(cfg.GitHub.Repo)
			if err == nil {
				cfg.GitHub.Owner = owner
				cfg.GitHub.Repo = repo
			}
		}

		if cfg.GitHub.Owner == "" || cfg.GitHub.Repo == "" {
			return fmt.Errorf("github owner and repo are required (use --repo owner/repo)")
		}

		path := prPath
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

		if len(modules) == 0 {
			fmt.Println("No Terraform modules found.")
			return nil
		}

		log.Info().Int("count", len(modules)).Msg("modules found")

		// Create version checker
		checker := version.New(
			cfg.GitHub.Token,
			cfg.VersionCheck.SkipPrerelease,
			cfg.VersionCheck.PatchOnly,
			cfg.VersionCheck.MinorOnly,
			cfg.VersionCheck.IgnoreModules,
		)

		// Check for updates
		log.Info().Msg("checking for module updates")
		updates, err := checker.Check(ctx, modules)
		if err != nil {
			return fmt.Errorf("version check failed: %w", err)
		}

		if len(updates) == 0 {
			fmt.Println("✨ All modules are up to date!")
			return nil
		}

		fmt.Printf("Found %d update(s) available\n\n", len(updates))

		// Create PR creator
		prCreator, err := github.NewPRCreator(
			cfg.GitHub.Token,
			cfg.GitHub.Owner,
			cfg.GitHub.Repo,
			cfg.GitHub.BaseBranch,
			path,
			cfg.GitHub.Labels,
			cfg.GitHub.Reviewers,
		)
		if err != nil {
			return fmt.Errorf("failed to create PR creator: %w", err)
		}

		// Create Terraform runner for plan validation
		var runner *terraform.Runner
		if !skipPlan {
			runner, err = terraform.New(path, cfg.Terraform.BinaryPath, cfg.Terraform.Env)
			if err != nil {
				log.Warn().Err(err).Msg("failed to create terraform runner, skipping plan validation")
			}
		}

		// Create schema comparator
		schemaComp := terraform.NewSchemaComparator()

		// Scan for providers
		log.Info().Str("path", path).Msg("scanning for terraform providers")
		providers, err := s.ScanProviders()
		if err != nil {
			log.Warn().Err(err).Msg("provider scan failed")
			providers = nil
		}

		log.Info().Int("count", len(providers)).Msg("providers found")

		// Check for provider updates
		var providerUpdates []version.ProviderUpdateInfo
		if len(providers) > 0 {
			log.Info().Msg("checking for provider updates")
			providerUpdates, err = checker.CheckProviders(ctx, providers)
			if err != nil {
				log.Warn().Err(err).Msg("provider version check failed")
				providerUpdates = nil
			}
		}

		totalUpdates := len(updates) + len(providerUpdates)
		if totalUpdates == 0 {
			fmt.Println("✨ All modules and providers are up to date!")
			return nil
		}

		fmt.Printf("Found %d update(s) available (%d modules, %d providers)\\n\\n",
			totalUpdates, len(updates), len(providerUpdates))

		// Create PRs for each module update
		successCount := 0
		for i, update := range updates {
			fmt.Printf("[%d/%d] Processing module %s...\n", i+1, totalUpdates, update.Module.Name)

			// Perform schema comparison
			if update.CurrentVersion != "" && update.LatestVersion != "" {
				schemaChanges, err := schemaComp.CompareSchemas(ctx, update.Module, update.CurrentVersion, update.LatestVersion)
				if err == nil && schemaChanges != nil {
					update.SchemaChanges = schemaChanges

					// Check if schema changes are breaking
					if terraform.HasBreakingSchemaChanges(schemaChanges) {
						update.HasBreakingChange = true
						if update.BreakingChangeDetails == "" {
							update.BreakingChangeDetails = "This update has breaking API changes (added required variables, removed variables/outputs, or changed types)."
						} else {
							update.BreakingChangeDetails += " This update also has breaking API changes."
						}
					}
				}
			}

			var planResult *terraform.PlanResult
			if runner != nil && !skipPlan {
				// Run plan to validate
				log.Info().Str("module", update.Module.Name).Msg("running terraform plan")
				if err := runner.Init(ctx); err != nil {
					log.Warn().Err(err).Msg("terraform init failed, creating PR without plan")
				} else {
					planResult, err = runner.Plan(ctx)
					if err != nil {
						log.Warn().Err(err).Msg("terraform plan failed, creating PR with error")
					} else if planResult != nil && planResult.Success {
						// Analyze resource changes from the plan
						update.ResourceChanges = terraform.AnalyzeResourceChanges(planResult)

						// Enhance breaking change detection with resource analysis
						if terraform.HasCriticalChanges(update.ResourceChanges) {
							update.HasBreakingChange = true
							if update.BreakingChangeDetails == "" {
								update.BreakingChangeDetails = "This update will cause resource replacements or deletions. Please review carefully."
							} else {
								update.BreakingChangeDetails += " Additionally, this update will cause resource replacements or deletions."
							}
						}
					}
				}
			}

			// Create PR
			pr, err := prCreator.CreatePR(ctx, update, planResult)
			if err != nil {
				log.Error().Err(err).Str("module", update.Module.Name).Msg("failed to create PR")
				fmt.Printf("  ✗ Failed to create PR: %v\n\n", err)
				continue
			}

			successCount++
			fmt.Printf("  ✓ PR created: %s\n", pr.GetHTMLURL())
			fmt.Printf("  #%d: %s\n\n", pr.GetNumber(), pr.GetTitle())
		}

		// Create PRs for each provider update
		for i, providerUpdate := range providerUpdates {
			fmt.Printf("[%d/%d] Processing provider %s...\n", len(updates)+i+1, totalUpdates, providerUpdate.Provider.Name)

			// Create PR for provider update
			pr, err := prCreator.CreateProviderPR(ctx, providerUpdate)
			if err != nil {
				log.Error().Err(err).Str("provider", providerUpdate.Provider.Name).Msg("failed to create PR")
				fmt.Printf("  ✗ Failed to create PR: %v\n\n", err)
				continue
			}

			successCount++
			fmt.Printf("  ✓ PR created: %s\n", pr.GetHTMLURL())
			fmt.Printf("  #%d: %s\n\n", pr.GetNumber(), pr.GetTitle())
		}

		fmt.Printf("\n✓ Successfully created %d/%d pull request(s)\n", successCount, totalUpdates)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(prCmd)

	prCmd.Flags().StringVarP(&prPath, "path", "p", "",
		"path to Terraform working directory (default: current directory)")
	prCmd.Flags().StringVarP(&prRepo, "repo", "r", "",
		"GitHub repository (format: owner/repo)")
	prCmd.Flags().StringVar(&prOwner, "owner", "",
		"GitHub repository owner")
	prCmd.Flags().BoolVar(&skipPlan, "skip-plan", false,
		"skip terraform plan validation")
}

// parseRepo parses owner/repo format
func parseRepo(repo string) (string, string, error) {
	parts := []rune(repo)
	slashIdx := -1
	for i, r := range parts {
		if r == '/' {
			slashIdx = i
			break
		}
	}

	if slashIdx == -1 || slashIdx == 0 || slashIdx == len(parts)-1 {
		return "", "", fmt.Errorf("invalid repo format, expected owner/repo")
	}

	return string(parts[:slashIdx]), string(parts[slashIdx+1:]), nil
}
