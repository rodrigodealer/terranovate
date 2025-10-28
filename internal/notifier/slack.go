package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/heyjobs/terranovate/internal/version"
	"github.com/rs/zerolog/log"
)

// Notifier sends notifications about module updates
type Notifier struct {
	slackWebhookURL string
	slackChannel    string
	httpClient      *http.Client
}

// New creates a new Notifier instance
func New(slackWebhookURL, slackChannel string) *Notifier {
	return &Notifier{
		slackWebhookURL: slackWebhookURL,
		slackChannel:    slackChannel,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// NotificationData represents the data to be sent in notifications
type NotificationData struct {
	Updates         []version.UpdateInfo         `json:"updates"`
	ProviderUpdates []version.ProviderUpdateInfo `json:"provider_updates,omitempty"`
	TotalUpdates    int                          `json:"total_updates"`
	Repository      string                       `json:"repository,omitempty"`
	Timestamp       time.Time                    `json:"timestamp"`
}

// SendSlack sends a Slack notification about available updates
func (n *Notifier) SendSlack(ctx context.Context, data NotificationData) error {
	if n.slackWebhookURL == "" {
		return fmt.Errorf("slack webhook URL not configured")
	}

	log.Info().Msg("sending slack notification")

	message := n.buildSlackMessage(data)

	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal slack message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", n.slackWebhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send slack message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}

	log.Info().Msg("slack notification sent successfully")
	return nil
}

// OutputJSON outputs the notification data as JSON
func (n *Notifier) OutputJSON(data NotificationData) (string, error) {
	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(output), nil
}

// OutputText outputs the notification data as human-readable text
func (n *Notifier) OutputText(data NotificationData) string {
	if data.TotalUpdates == 0 {
		return "No module updates available."
	}

	// Count breaking changes
	breakingChanges := 0
	for _, update := range data.Updates {
		if update.HasBreakingChange {
			breakingChanges++
		}
	}

	var output string
	output += fmt.Sprintf("Found %d module update(s)", data.TotalUpdates)
	if breakingChanges > 0 {
		output += fmt.Sprintf(" (%d with breaking changes)", breakingChanges)
	}
	output += ":\n\n"

	for i, update := range data.Updates {
		icon := "📦"
		if update.HasBreakingChange {
			icon = "⚠️"
		}

		output += fmt.Sprintf("%s %d. %s", icon, i+1, update.Module.Name)
		if update.UpdateType != "" && update.UpdateType != "unknown" {
			output += fmt.Sprintf(" (%s)", update.UpdateType)
		}
		output += "\n"

		output += fmt.Sprintf("   Source: %s\n", update.Module.Source)
		output += fmt.Sprintf("   Current: %s → Latest: %s\n", update.CurrentVersion, update.LatestVersion)

		if update.HasBreakingChange {
			output += fmt.Sprintf("   ⚠️ BREAKING CHANGE: %s\n", update.BreakingChangeDetails)
		}

		output += fmt.Sprintf("   File: %s:%d\n", update.Module.FilePath, update.Module.Line)
		if update.ChangelogURL != "" {
			output += fmt.Sprintf("   Changelog: %s\n", update.ChangelogURL)
		}
		output += "\n"
	}

	if breakingChanges > 0 {
		output += fmt.Sprintf("⚠️ Warning: %d update(s) may contain breaking changes.\n", breakingChanges)
	}

	return output
}

// OutputMarkdown outputs the notification data as GitHub-flavored markdown for PR comments
func (n *Notifier) OutputMarkdown(data NotificationData) string {
	var output string

	// Header
	output += "## 🔍 Terranovate Dependency Check\n\n"

	totalUpdates := data.TotalUpdates + len(data.ProviderUpdates)
	if totalUpdates == 0 {
		output += "✨ **All modules and providers are up to date!**\n"
		return output
	}

	// Count breaking changes
	breakingChanges := 0
	for _, update := range data.Updates {
		if update.HasBreakingChange {
			breakingChanges++
		}
	}
	for _, update := range data.ProviderUpdates {
		if update.HasBreakingChange {
			breakingChanges++
		}
	}

	// Summary
	if breakingChanges > 0 {
		output += "⚠️ **Warning**: Found updates with potential breaking changes!\n\n"
	}

	output += fmt.Sprintf("**Summary**: %d update(s) available", totalUpdates)
	if breakingChanges > 0 {
		output += fmt.Sprintf(" (%d with potential breaking changes)", breakingChanges)
	}
	output += "\n\n"

	// Module updates section
	if len(data.Updates) > 0 {
		output += "### 📦 Module Updates\n\n"

		for i, update := range data.Updates {
			// Update header
			icon := "📦"
			if update.HasBreakingChange {
				icon = "⚠️"
			}

			output += fmt.Sprintf("<details>\n<summary>%s <strong>%d. %s</strong>", icon, i+1, update.Module.Name)
			if update.UpdateType != "" && update.UpdateType != "unknown" {
				output += fmt.Sprintf(" <code>%s update</code>", update.UpdateType)
			}
			output += fmt.Sprintf(" - <code>%s</code> → <code>%s</code></summary>\n\n", update.CurrentVersion, update.LatestVersion)

			// Details
			output += "| Field | Value |\n"
			output += "|-------|-------|\n"
			output += fmt.Sprintf("| **Source** | `%s` |\n", update.Module.Source)
			output += fmt.Sprintf("| **Current Version** | `%s` |\n", update.CurrentVersion)
			output += fmt.Sprintf("| **Latest Version** | `%s` |\n", update.LatestVersion)
			output += fmt.Sprintf("| **Update Type** | `%s` |\n", update.UpdateType)
			output += fmt.Sprintf("| **File** | `%s:%d` |\n", update.Module.FilePath, update.Module.Line)

			if update.ChangelogURL != "" {
				output += fmt.Sprintf("| **Changelog** | [View](%s) |\n", update.ChangelogURL)
			}

			// Breaking change warning
			if update.HasBreakingChange {
				output += "\n> ⚠️ **Breaking Change**\n>\n"
				output += fmt.Sprintf("> %s\n", update.BreakingChangeDetails)
			}

			// Resource changes
			if update.ResourceChanges != nil && update.ResourceChanges.HasChanges {
				output += "\n**Resource Changes:**\n\n"

				if update.ResourceChanges.TotalReplace > 0 {
					output += fmt.Sprintf("- ⚠️ **%d resource(s) will be REPLACED**\n", update.ResourceChanges.TotalReplace)
					for _, rc := range update.ResourceChanges.ResourcesToReplace {
						output += fmt.Sprintf("  - `%s` (%s)\n", rc.Address, rc.Reason)
					}
				}

				if update.ResourceChanges.TotalDelete > 0 {
					output += fmt.Sprintf("- 🗑️ **%d resource(s) will be DELETED**\n", update.ResourceChanges.TotalDelete)
					for _, rc := range update.ResourceChanges.ResourcesToDelete {
						output += fmt.Sprintf("  - `%s`\n", rc.Address)
					}
				}

				if update.ResourceChanges.TotalModify > 0 {
					output += fmt.Sprintf("- 📝 **%d resource(s) will be MODIFIED**\n", update.ResourceChanges.TotalModify)
				}
			}

			output += "\n</details>\n\n"
		}
	}

	// Provider updates section
	if len(data.ProviderUpdates) > 0 {
		output += "### 🔌 Provider Updates\n\n"

		for i, update := range data.ProviderUpdates {
			// Update header
			icon := "📦"
			if update.HasBreakingChange {
				icon = "⚠️"
			}

			output += fmt.Sprintf("<details>\n<summary>%s <strong>%d. %s</strong>", icon, i+1, update.Provider.Name)
			if update.UpdateType != "" && update.UpdateType != "unknown" {
				output += fmt.Sprintf(" <code>%s update</code>", update.UpdateType)
			}
			output += fmt.Sprintf(" - <code>%s</code> → <code>%s</code></summary>\n\n", update.CurrentVersion, update.LatestVersion)

			// Details
			output += "| Field | Value |\n"
			output += "|-------|-------|\n"
			output += fmt.Sprintf("| **Source** | `%s` |\n", update.Provider.Source)
			output += fmt.Sprintf("| **Current Version** | `%s` |\n", update.CurrentVersion)
			output += fmt.Sprintf("| **Latest Version** | `%s` |\n", update.LatestVersion)
			output += fmt.Sprintf("| **Update Type** | `%s` |\n", update.UpdateType)
			output += fmt.Sprintf("| **File** | `%s:%d` |\n", update.Provider.FilePath, update.Provider.Line)

			if update.ChangelogURL != "" {
				output += fmt.Sprintf("| **Documentation** | [View](%s) |\n", update.ChangelogURL)
			}

			// Breaking change warning
			if update.HasBreakingChange {
				output += "\n> ⚠️ **Breaking Change**\n>\n"
				output += fmt.Sprintf("> %s\n", update.BreakingChangeDetails)
			}

			output += "\n</details>\n\n"
		}
	}

	// Footer with recommendations
	if breakingChanges > 0 {
		output += "---\n\n"
		output += "### ⚠️ Action Required\n\n"
		output += fmt.Sprintf("**%d update(s) contain potential breaking changes.**\n\n", breakingChanges)
		output += "**Recommendations:**\n"
		output += "- 📖 Review changelogs and documentation carefully\n"
		output += "- 🧪 Test in a non-production environment first\n"
		output += "- 🔍 Check for deprecated features or API changes\n"
		output += "- 👥 Coordinate with your team before applying updates\n"
	}

	output += "\n---\n"
	output += fmt.Sprintf("*🤖 Generated by [Terranovate](https://github.com/heyjobs/terranovate) at %s*\n", data.Timestamp.Format("2006-01-02 15:04:05 UTC"))

	return output
}

// buildSlackMessage builds a Slack message payload
func (n *Notifier) buildSlackMessage(data NotificationData) map[string]interface{} {
	// Count breaking changes
	breakingChanges := 0
	for _, update := range data.Updates {
		if update.HasBreakingChange {
			breakingChanges++
		}
	}

	messageText := fmt.Sprintf("🔔 Terranovate: %d module update(s) available", data.TotalUpdates)
	if breakingChanges > 0 {
		messageText = fmt.Sprintf("⚠️ Terranovate: %d module update(s) available (%d with breaking changes)", data.TotalUpdates, breakingChanges)
	}

	message := map[string]interface{}{
		"text": messageText,
	}

	if n.slackChannel != "" {
		message["channel"] = n.slackChannel
	}

	// Build attachments for each update
	var attachments []map[string]interface{}

	for _, update := range data.Updates {
		// Set color based on breaking change or update type
		color := "good" // green for patch updates
		if update.HasBreakingChange {
			color = "danger" // red for breaking changes
		} else if update.UpdateType == "minor" {
			color = "warning" // orange for minor updates
		}

		title := update.Module.Name
		if update.UpdateType != "" && update.UpdateType != "unknown" {
			title += fmt.Sprintf(" (%s update)", update.UpdateType)
		}

		fields := []map[string]interface{}{
			{
				"title": "Source",
				"value": update.Module.Source,
				"short": false,
			},
			{
				"title": "Current Version",
				"value": update.CurrentVersion,
				"short": true,
			},
			{
				"title": "Latest Version",
				"value": update.LatestVersion,
				"short": true,
			},
		}

		// Add breaking change warning
		if update.HasBreakingChange {
			fields = append(fields, map[string]interface{}{
				"title": "⚠️ Breaking Change",
				"value": update.BreakingChangeDetails,
				"short": false,
			})
		}

		fields = append(fields, map[string]interface{}{
			"title": "File",
			"value": fmt.Sprintf("%s:%d", update.Module.FilePath, update.Module.Line),
			"short": false,
		})

		attachment := map[string]interface{}{
			"color":      color,
			"title":      title,
			"title_link": update.ChangelogURL,
			"fields":     fields,
			"footer":     "Terranovate",
			"ts":         data.Timestamp.Unix(),
		}

		attachments = append(attachments, attachment)
	}

	if len(attachments) > 0 {
		message["attachments"] = attachments
	}

	return message
}
