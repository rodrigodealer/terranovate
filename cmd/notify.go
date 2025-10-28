package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/heyjobs/terranovate/internal/notifier"
	"github.com/heyjobs/terranovate/internal/scanner"
	"github.com/heyjobs/terranovate/internal/version"
	"github.com/heyjobs/terranovate/pkg/config"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	notifyPath   string
	notifyFormat string
)

// notifyCmd represents the notify command
var notifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Send notifications about module updates",
	Long: `Notify scans for outdated modules and sends notifications
via Slack or outputs results in JSON/text format.

Example:
  terranovate notify --format slack
  terranovate notify --format json
  terranovate notify --format text`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Load configuration
		cfg, err := loadConfig()
		if err != nil {
			log.Warn().Err(err).Msg("failed to load config, using defaults")
			cfg = config.Default()
		}

		path := notifyPath
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

		// Prepare notification data
		data := notifier.NotificationData{
			Updates:      updates,
			TotalUpdates: len(updates),
			Repository:   fmt.Sprintf("%s/%s", cfg.GitHub.Owner, cfg.GitHub.Repo),
			Timestamp:    time.Now(),
		}

		// Create notifier
		n := notifier.New(
			cfg.Notifier.Slack.WebhookURL,
			cfg.Notifier.Slack.Channel,
		)

		// Determine output format
		format := notifyFormat
		if format == "" {
			format = cfg.Notifier.OutputFormat
		}

		// Send notification or output results
		switch format {
		case "slack":
			if !cfg.Notifier.Slack.Enabled {
				return fmt.Errorf("slack notifications not enabled in config")
			}
			if err := n.SendSlack(ctx, data); err != nil {
				return fmt.Errorf("failed to send slack notification: %w", err)
			}
			fmt.Println("✓ Slack notification sent successfully")

		case "json":
			output, err := n.OutputJSON(data)
			if err != nil {
				return fmt.Errorf("failed to generate JSON output: %w", err)
			}
			fmt.Println(output)

		case "text":
			output := n.OutputText(data)
			fmt.Println(output)

		default:
			return fmt.Errorf("unsupported format: %s (use slack, json, or text)", format)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(notifyCmd)

	notifyCmd.Flags().StringVarP(&notifyPath, "path", "p", "",
		"path to scan for Terraform files (default: current directory)")
	notifyCmd.Flags().StringVarP(&notifyFormat, "format", "f", "",
		"output format: slack, json, or text (default from config)")
}
