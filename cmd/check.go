package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/heyjobs/terranovate/internal/ai"
	"github.com/heyjobs/terranovate/internal/notifier"
	"github.com/heyjobs/terranovate/internal/scanner"
	"github.com/heyjobs/terranovate/internal/version"
	"github.com/heyjobs/terranovate/pkg/config"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	checkPath            string
	checkFormat          string
	checkUnusedProviders bool
	displayFilter        string
)

// shouldDisplayUpdate determines if an update should be displayed based on filter
func shouldDisplayUpdate(updateType version.UpdateType, filter string) bool {
	switch filter {
	case "major-only", "critical-only":
		return updateType == version.UpdateTypeMajor
	case "minor-and-above":
		return updateType == version.UpdateTypeMajor || updateType == version.UpdateTypeMinor
	case "all":
		return true
	default:
		return true // Default to showing all
	}
}

// shouldDisplayAIAnalysis determines if an update with AI analysis should be displayed based on confidence level
func shouldDisplayAIAnalysis(aiAnalysis *ai.BreakingChangeAnalysis, minConfidence string) bool {
	// If no AI analysis, always display
	if aiAnalysis == nil {
		return true
	}

	// Map confidence levels to numeric values for comparison
	confidenceLevel := map[string]int{
		"low":    1,
		"medium": 2,
		"high":   3,
	}

	// Get minimum required confidence level (default to low if invalid)
	minLevel, ok := confidenceLevel[minConfidence]
	if !ok {
		minLevel = 1 // Default to low
	}

	// Get actual confidence level (default to low if invalid)
	actualLevel, ok := confidenceLevel[aiAnalysis.Confidence]
	if !ok {
		actualLevel = 1 // Default to low
	}

	// Display if actual confidence meets or exceeds minimum
	return actualLevel >= minLevel
}

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for module updates",
	Long: `Check scans Terraform files and compares module versions
with the latest available versions from Terraform Registry or Git repositories.

Example:
  terranovate check --path ./infrastructure`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Load configuration
		cfg, err := loadConfig()
		if err != nil {
			log.Warn().Err(err).Msg("failed to load config, using defaults")
			cfg = config.Default()
		}

		path := checkPath
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

		// Detect if using Terramate or Terragrunt
		tooling := scanner.DetectTooling(path)

		// Determine display filter (CLI flag overrides config)
		filter := displayFilter
		if filter == "" {
			filter = cfg.VersionCheck.DisplayFilter
		}
		if filter == "" {
			filter = "all"
		}

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

		// Configure AI analyzer if enabled
		if cfg.OpenAI.Enabled && cfg.OpenAI.APIKey != "" {
			log.Info().Msg("AI-powered breaking change detection enabled")
			aiAnalyzer := ai.NewAdapter(cfg.OpenAI.APIKey, cfg.OpenAI.Model, cfg.OpenAI.BaseURL)
			checker.SetAIAnalyzer(aiAnalyzer)
		} else if cfg.OpenAI.Enabled {
			log.Warn().Msg("AI analysis enabled but no API key configured (set OPENAI_API_KEY env var or in config)")
		}

		// Check for updates
		log.Info().Msg("checking for module updates")
		updates, err := checker.Check(ctx, modules)
		if err != nil {
			return fmt.Errorf("version check failed: %w", err)
		}

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

		// Check for unused providers if enabled
		var unusedProviders []scanner.UnusedProviderInfo
		if checkUnusedProviders && len(providers) > 0 {
			log.Info().Str("path", path).Msg("scanning for terraform resources")
			resources, err := s.ScanResources()
			if err != nil {
				log.Warn().Err(err).Msg("resource scan failed")
			} else {
				log.Info().Int("count", len(resources)).Msg("resources found")

				// Get ignore list from config
				ignoreProviders := cfg.VersionCheck.IgnoreUnusedProviders
				if ignoreProviders == nil {
					ignoreProviders = []string{}
				}

				log.Info().Msg("checking for unused providers")
				unusedProviders = scanner.DetectUnusedProviders(providers, resources, modules, ignoreProviders)
				if len(unusedProviders) > 0 {
					log.Info().Int("count", len(unusedProviders)).Msg("unused providers found")
				}
			}
		}

		// Apply display filter
		if filter != "all" {
			log.Info().Str("filter", filter).Msg("applying display filter")

			// Filter module updates
			var filteredUpdates []version.UpdateInfo
			for _, update := range updates {
				if shouldDisplayUpdate(update.UpdateType, filter) {
					filteredUpdates = append(filteredUpdates, update)
				} else {
					log.Debug().
						Str("module", update.Module.Name).
						Str("update_type", string(update.UpdateType)).
						Msg("filtered out update")
				}
			}
			originalCount := len(updates)
			updates = filteredUpdates
			if originalCount > len(updates) {
				log.Info().
					Int("original", originalCount).
					Int("filtered", len(updates)).
					Int("hidden", originalCount-len(updates)).
					Msg("filtered module updates")
			}

			// Filter provider updates
			var filteredProviders []version.ProviderUpdateInfo
			for _, update := range providerUpdates {
				if shouldDisplayUpdate(update.UpdateType, filter) {
					filteredProviders = append(filteredProviders, update)
				} else {
					log.Debug().
						Str("provider", update.Provider.Name).
						Str("update_type", string(update.UpdateType)).
						Msg("filtered out provider update")
				}
			}
			originalProviderCount := len(providerUpdates)
			providerUpdates = filteredProviders
			if originalProviderCount > len(providerUpdates) {
				log.Info().
					Int("original", originalProviderCount).
					Int("filtered", len(providerUpdates)).
					Int("hidden", originalProviderCount-len(providerUpdates)).
					Msg("filtered provider updates")
			}
		}

		// Apply AI confidence filter if configured
		minConfidence := cfg.OpenAI.MinConfidence
		if minConfidence != "" && minConfidence != "low" && cfg.OpenAI.Enabled {
			log.Info().Str("min_confidence", minConfidence).Msg("applying AI confidence filter")

			// Filter module updates by AI confidence
			var filteredUpdates []version.UpdateInfo
			for _, update := range updates {
				if shouldDisplayAIAnalysis(update.AIAnalysis, minConfidence) {
					filteredUpdates = append(filteredUpdates, update)
				} else {
					log.Debug().
						Str("module", update.Module.Name).
						Str("confidence", update.AIAnalysis.Confidence).
						Msg("filtered out update due to low AI confidence")
				}
			}
			originalCount := len(updates)
			updates = filteredUpdates
			if originalCount > len(updates) {
				log.Info().
					Int("original", originalCount).
					Int("filtered", len(updates)).
					Int("hidden", originalCount-len(updates)).
					Msg("filtered updates by AI confidence")
			}

			// Filter provider updates by AI confidence
			var filteredProviders []version.ProviderUpdateInfo
			for _, update := range providerUpdates {
				if shouldDisplayAIAnalysis(update.AIAnalysis, minConfidence) {
					filteredProviders = append(filteredProviders, update)
				} else {
					log.Debug().
						Str("provider", update.Provider.Name).
						Str("confidence", update.AIAnalysis.Confidence).
						Msg("filtered out provider update due to low AI confidence")
				}
			}
			originalProviderCount := len(providerUpdates)
			providerUpdates = filteredProviders
			if originalProviderCount > len(providerUpdates) {
				log.Info().
					Int("original", originalProviderCount).
					Int("filtered", len(providerUpdates)).
					Int("hidden", originalProviderCount-len(providerUpdates)).
					Msg("filtered provider updates by AI confidence")
			}
		}

		// Check if markdown format is requested
		if checkFormat == "markdown" {
			n := notifier.New("", "")
			data := notifier.NotificationData{
				Updates:         updates,
				ProviderUpdates: providerUpdates,
				TotalUpdates:    len(updates),
				Timestamp:       time.Now(),
			}
			output := n.OutputMarkdown(data)
			fmt.Println(output)
			return nil
		}

		// Output results (default text format)
		if len(updates) == 0 && len(providerUpdates) == 0 && len(unusedProviders) == 0 {
			if filter != "all" {
				fmt.Printf("✨ No %s updates found! (filter: %s)\n", filter, filter)
			} else {
				fmt.Println("✨ All modules and providers are up to date!")
			}
			return nil
		}

		// Show filter info if active
		if filter != "all" {
			fmt.Printf("ℹ️  Display filter active: %s\n", filter)
			switch filter {
			case "major-only", "critical-only":
				fmt.Println("   Showing only major version updates")
			case "minor-and-above":
				fmt.Println("   Showing minor and major version updates (hiding patches)")
			}
			fmt.Println()
		}

		// Show AI confidence filter info if active
		if minConfidence != "" && minConfidence != "low" && cfg.OpenAI.Enabled {
			fmt.Printf("ℹ️  AI confidence filter active: %s\n", minConfidence)
			switch minConfidence {
			case "high":
				fmt.Println("   Showing only high confidence AI assessments")
			case "medium":
				fmt.Println("   Showing medium and high confidence AI assessments")
			}
			fmt.Println()
		}

		// Count breaking changes
		breakingChanges := 0
		for _, update := range updates {
			if update.HasBreakingChange {
				breakingChanges++
			}
		}

		fmt.Printf("🔍 Found %d update(s) available", len(updates))
		if breakingChanges > 0 {
			fmt.Printf(" (%d with potential breaking changes ⚠️)", breakingChanges)
		}
		fmt.Println()

		for i, update := range updates {
			// Add warning emoji for breaking changes
			icon := "📦"
			if update.HasBreakingChange {
				icon = "⚠️ "
			}

			fmt.Printf("%s %d. %s", icon, i+1, update.Module.Name)
			if update.UpdateType != "" && update.UpdateType != version.UpdateTypeUnknown {
				fmt.Printf(" (%s update)", update.UpdateType)
			}
			fmt.Println()

			fmt.Printf("   📍 Source: %s\n", update.Module.Source)
			fmt.Printf("   🔄 Current: %s → Latest: %s\n", update.CurrentVersion, update.LatestVersion)

			// Highlight breaking changes
			if update.HasBreakingChange {
				fmt.Printf("   ⚠️  BREAKING CHANGE: %s\n", update.BreakingChangeDetails)
			}

			// Display AI analysis if available
			if update.AIAnalysis != nil {
				fmt.Printf("   🤖 AI Analysis (%s confidence):\n", update.AIAnalysis.Confidence)
				fmt.Printf("      %s\n", update.AIAnalysis.Summary)
				if len(update.AIAnalysis.Details) > 0 {
					for _, detail := range update.AIAnalysis.Details {
						fmt.Printf("      • %s\n", detail)
					}
				}
			}

			// Show resource changes if available
			if update.ResourceChanges != nil && update.ResourceChanges.HasChanges {
				fmt.Println("   Resource Changes:")

				if update.ResourceChanges.TotalReplace > 0 {
					fmt.Printf("   ⚠️  %d resource(s) will be REPLACED:\n", update.ResourceChanges.TotalReplace)
					for _, rc := range update.ResourceChanges.ResourcesToReplace {
						fmt.Printf("      - %s (%s)\n", rc.Address, rc.Reason)
					}
				}

				if update.ResourceChanges.TotalDelete > 0 {
					fmt.Printf("   🗑️  %d resource(s) will be DELETED:\n", update.ResourceChanges.TotalDelete)
					for _, rc := range update.ResourceChanges.ResourcesToDelete {
						fmt.Printf("      - %s\n", rc.Address)
					}
				}

				if update.ResourceChanges.TotalModify > 0 {
					fmt.Printf("   📝 %d resource(s) will be MODIFIED\n", update.ResourceChanges.TotalModify)
				}
			}

			fmt.Printf("   📄 File: %s:%d\n", update.Module.FilePath, update.Module.Line)
			if update.ChangelogURL != "" {
				fmt.Printf("   📋 Changelog: %s\n", update.ChangelogURL)
			}
			fmt.Println()
		}

		// Summary warning for breaking changes
		if breakingChanges > 0 {
			fmt.Printf("⚠️  Warning: %d update(s) may contain breaking changes.\n", breakingChanges)
			fmt.Println("   Please review changelogs carefully before applying these updates.")
		}

		// Display provider updates
		if len(providerUpdates) > 0 {
			fmt.Println("\n" + strings.Repeat("=", 60))
			fmt.Println("🔌 Provider Updates")
			fmt.Println(strings.Repeat("=", 60) + "\n")

			// Group provider updates by unique provider + version combination
			type providerKey struct {
				Name           string
				Source         string
				CurrentVersion string
				LatestVersion  string
			}
			grouped := make(map[providerKey][]version.ProviderUpdateInfo)

			for _, update := range providerUpdates {
				key := providerKey{
					Name:           update.Provider.Name,
					Source:         update.Provider.Source,
					CurrentVersion: update.CurrentVersion,
					LatestVersion:  update.LatestVersion,
				}
				grouped[key] = append(grouped[key], update)
			}

			// Count breaking changes in providers
			providerBreakingChanges := 0
			uniqueUpdates := 0
			for _, updates := range grouped {
				uniqueUpdates++
				if updates[0].HasBreakingChange {
					providerBreakingChanges++
				}
			}

			fmt.Printf("🔍 Found %d unique provider update(s) in %d location(s)", uniqueUpdates, len(providerUpdates))
			if providerBreakingChanges > 0 {
				fmt.Printf(" (%d with potential breaking changes ⚠️)", providerBreakingChanges)
			}
			fmt.Println()

			// Show message if providers are centrally managed
			if tooling.UsesTerramate || tooling.UsesTerragrunt {
				if tooling.UsesTerramate {
					fmt.Println("ℹ️  Terramate detected - providers may be centrally managed")
				}
				if tooling.UsesTerragrunt {
					fmt.Println("ℹ️  Terragrunt detected - providers may be centrally managed via generate blocks")
				}

				centralConfig := scanner.FindProviderGenerationSource(path, tooling)
				if centralConfig != "" {
					fmt.Printf("   Update the provider version in: %s\n", centralConfig)
				}
				fmt.Println()
			}

			i := 1
			for _, updates := range grouped {
				update := updates[0] // Use first one for details

				// Add warning emoji for breaking changes
				icon := "📦"
				if update.HasBreakingChange {
					icon = "⚠️ "
				}

				fmt.Printf("%s %d. %s", icon, i, update.Provider.Name)
				if update.UpdateType != "" && update.UpdateType != version.UpdateTypeUnknown {
					fmt.Printf(" (%s update)", update.UpdateType)
				}
				fmt.Println()

				fmt.Printf("   📍 Source: %s\n", update.Provider.Source)
				fmt.Printf("   🔄 Current: %s → Latest: %s\n", update.CurrentVersion, update.LatestVersion)

				// Highlight breaking changes
				if update.HasBreakingChange {
					fmt.Printf("   ⚠️  BREAKING CHANGE: %s\n", update.BreakingChangeDetails)
				}

				// Display AI analysis if available
				if update.AIAnalysis != nil {
					fmt.Printf("   🤖 AI Analysis (%s confidence):\n", update.AIAnalysis.Confidence)
					fmt.Printf("      %s\n", update.AIAnalysis.Summary)
					if len(update.AIAnalysis.Details) > 0 {
						for _, detail := range update.AIAnalysis.Details {
							fmt.Printf("      • %s\n", detail)
						}
					}
				}

				// Display file locations (limit to avoid overwhelming output)
				const maxLocationsToShow = 5
				if len(updates) == 1 {
					fmt.Printf("   📄 File: %s:%d\n", update.Provider.FilePath, update.Provider.Line)
				} else if len(updates) <= maxLocationsToShow {
					fmt.Printf("   📄 Found in %d files:\n", len(updates))
					for _, u := range updates {
						fmt.Printf("      - %s:%d\n", u.Provider.FilePath, u.Provider.Line)
					}
				} else {
					fmt.Printf("   📄 Found in %d files (showing first %d):\n", len(updates), maxLocationsToShow)
					for idx, u := range updates {
						if idx >= maxLocationsToShow {
							break
						}
						fmt.Printf("      - %s:%d\n", u.Provider.FilePath, u.Provider.Line)
					}
					fmt.Printf("      ... and %d more locations\n", len(updates)-maxLocationsToShow)
				}

				if update.ChangelogURL != "" {
					fmt.Printf("   📚 Documentation: %s\n", update.ChangelogURL)
				}
				fmt.Println()
				i++
			}

			// Summary warning for breaking changes in providers
			if providerBreakingChanges > 0 {
				fmt.Printf("⚠️  Warning: %d provider update(s) may contain breaking changes.\n", providerBreakingChanges)
				fmt.Println("   Please review provider documentation carefully before applying these updates.")
			}
		}

		// Display unused providers
		if len(unusedProviders) > 0 {
			fmt.Println("\n" + strings.Repeat("=", 60))
			fmt.Println("🔌 Unused Providers")
			fmt.Println(strings.Repeat("=", 60) + "\n")

			fmt.Printf("🔍 Found %d unused provider(s)\n\n", len(unusedProviders))

			for i, unused := range unusedProviders {
				fmt.Printf("⚠️  %d. %s\n", i+1, unused.Provider.Name)
				fmt.Printf("   📍 Source: %s\n", unused.Provider.Source)
				if unused.Provider.Version != "" {
					fmt.Printf("   🔖 Version: %s\n", unused.Provider.Version)
				}
				fmt.Printf("   📄 File: %s:%d\n", unused.Provider.FilePath, unused.Provider.Line)
				fmt.Printf("   💡 Suggestion: %s\n", unused.Suggestion)
				fmt.Println()
			}

			fmt.Printf("ℹ️  These providers are declared in required_providers but not used by any resources.\n")
			fmt.Println("   Consider removing them to keep your configuration clean.")
			fmt.Println("   Use --check-unused-providers=false to disable this check.")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)

	checkCmd.Flags().StringVarP(&checkPath, "path", "p", "",
		"path to scan for Terraform files (default: current directory)")
	checkCmd.Flags().StringVarP(&checkFormat, "format", "f", "",
		"output format: text (default) or markdown (for PR comments)")
	checkCmd.Flags().BoolVar(&checkUnusedProviders, "check-unused-providers", true,
		"check for unused providers (default: true)")
	checkCmd.Flags().StringVar(&displayFilter, "display-filter", "",
		"filter updates to display: all, major-only, minor-and-above, critical-only (default: all)")
}
