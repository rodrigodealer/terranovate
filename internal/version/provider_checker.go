package version

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/heyjobs/terranovate/internal/ai"
	"github.com/heyjobs/terranovate/internal/scanner"
	"github.com/rs/zerolog/log"
)

// ProviderUpdateInfo represents available update information for a provider
type ProviderUpdateInfo struct {
	Provider              scanner.ProviderInfo
	CurrentVersion        string
	LatestVersion         string
	IsOutdated            bool
	HasBreakingChange     bool
	BreakingChangeDetails string
	ChangelogURL          string
	UpdateType            UpdateType
	AIAnalysis            *ai.AIAnalysis // AI-powered breaking change detection
}

// CheckProviders checks for updates for the given providers
func (c *Checker) CheckProviders(ctx context.Context, providers []scanner.ProviderInfo) ([]ProviderUpdateInfo, error) {
	var updates []ProviderUpdateInfo

	for _, provider := range providers {
		updateInfo, err := c.checkProvider(ctx, provider)
		if err != nil {
			log.Warn().Err(err).
				Str("provider", provider.Name).
				Str("file", fmt.Sprintf("%s:%d", provider.FilePath, provider.Line)).
				Msg("failed to check provider version")
			continue
		}

		if updateInfo.IsOutdated {
			// Perform AI analysis if analyzer is configured
			if c.aiAnalyzer != nil {
				aiAnalysis, err := c.aiAnalyzer.AnalyzeBreakingChanges(
					ctx,
					provider.Name,
					updateInfo.CurrentVersion,
					updateInfo.LatestVersion,
					updateInfo.ChangelogURL,
				)
				if err != nil {
					log.Warn().Err(err).
						Str("provider", provider.Name).
						Msg("AI analysis failed, skipping")
				} else {
					updateInfo.AIAnalysis = aiAnalysis
					log.Debug().
						Str("provider", provider.Name).
						Bool("ai_breaking_changes", aiAnalysis.HasBreakingChanges).
						Str("confidence", aiAnalysis.Confidence).
						Msg("AI analysis completed")
				}
			}

			updates = append(updates, updateInfo)
			log.Info().
				Str("provider", provider.Name).
				Str("current", updateInfo.CurrentVersion).
				Str("latest", updateInfo.LatestVersion).
				Msg("provider update available")
		}
	}

	return updates, nil
}

// checkProvider checks for updates from Terraform Registry
func (c *Checker) checkProvider(ctx context.Context, provider scanner.ProviderInfo) (ProviderUpdateInfo, error) {
	updateInfo := ProviderUpdateInfo{
		Provider:       provider,
		CurrentVersion: extractVersionFromConstraint(provider.Version),
	}

	// Parse provider source (namespace/name)
	parts := strings.Split(provider.Source, "/")
	if len(parts) < 2 {
		return updateInfo, fmt.Errorf("invalid provider source format: %s", provider.Source)
	}

	namespace := parts[0]
	providerType := parts[1]

	// Query Terraform Registry API for provider versions
	url := fmt.Sprintf("https://registry.terraform.io/v1/providers/%s/%s/versions",
		namespace, providerType)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return updateInfo, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return updateInfo, fmt.Errorf("failed to query registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return updateInfo, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	var registryResp struct {
		Versions []struct {
			Version   string   `json:"version"`
			Protocols []string `json:"protocols"`
		} `json:"versions"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&registryResp); err != nil {
		return updateInfo, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(registryResp.Versions) == 0 {
		return updateInfo, fmt.Errorf("no versions found")
	}

	// Parse and filter versions
	var versions []*version.Version
	for _, v := range registryResp.Versions {
		ver, err := version.NewVersion(v.Version)
		if err != nil {
			continue
		}

		// Skip prerelease if configured
		if c.skipPrerelease && ver.Prerelease() != "" {
			continue
		}

		versions = append(versions, ver)
	}

	if len(versions) == 0 {
		return updateInfo, fmt.Errorf("no valid versions found")
	}

	// Sort versions
	sort.Sort(version.Collection(versions))

	// Get latest version
	latestVersion := versions[len(versions)-1]
	updateInfo.LatestVersion = latestVersion.String()

	// Check if update is available
	if updateInfo.CurrentVersion != "" {
		currentVersion, err := version.NewVersion(updateInfo.CurrentVersion)
		if err != nil {
			return updateInfo, fmt.Errorf("invalid current version: %w", err)
		}

		updateInfo.IsOutdated = c.shouldUpdate(currentVersion, latestVersion)

		// Detect breaking changes and update type
		updateInfo.UpdateType = c.detectUpdateType(currentVersion, latestVersion)
		updateInfo.HasBreakingChange = updateInfo.UpdateType == UpdateTypeMajor

		if updateInfo.HasBreakingChange {
			updateInfo.BreakingChangeDetails = fmt.Sprintf(
				"Major version upgrade from %s to %s may contain breaking changes. Please review the changelog carefully.",
				currentVersion.String(), latestVersion.String())
		}
	} else {
		// No version specified means using latest already
		updateInfo.IsOutdated = false
		updateInfo.UpdateType = UpdateTypeUnknown
		log.Debug().
			Str("provider", provider.Name).
			Msg("provider has no version constraint, already using latest")
	}

	// Set changelog URL
	updateInfo.ChangelogURL = fmt.Sprintf("https://registry.terraform.io/providers/%s/%s/%s",
		namespace, providerType, latestVersion.String())

	return updateInfo, nil
}
