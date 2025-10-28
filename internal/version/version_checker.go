package version

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/v66/github"
	"github.com/hashicorp/go-version"
	"github.com/heyjobs/terranovate/internal/ai"
	"github.com/heyjobs/terranovate/internal/cache"
	"github.com/heyjobs/terranovate/internal/scanner"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

// UpdateInfo represents available update information for a module
type UpdateInfo struct {
	Module                scanner.ModuleInfo
	CurrentVersion        string
	LatestVersion         string
	IsOutdated            bool
	HasBreakingChange     bool
	BreakingChangeDetails string
	ChangelogURL          string
	UpdateType            UpdateType
	ResourceChanges       *ResourceChangesSummary
	SchemaChanges         interface{} // Will hold *terraform.SchemaChanges
	AIAnalysis            *ai.AIAnalysis // AI-powered breaking change detection
}

// ResourceChangesSummary summarizes infrastructure changes from terraform plan
type ResourceChangesSummary struct {
	HasChanges       bool
	ResourcesToReplace []ResourceChange
	ResourcesToDelete  []ResourceChange
	ResourcesToModify  []ResourceChange
	TotalReplace     int
	TotalDelete      int
	TotalModify      int
}

// ResourceChange represents a single resource change
type ResourceChange struct {
	Address      string
	ResourceType string
	Action       string
	Reason       string
}

// UpdateType indicates the type of version update
type UpdateType string

const (
	// UpdateTypeMajor indicates a major version update (breaking changes)
	UpdateTypeMajor UpdateType = "major"

	// UpdateTypeMinor indicates a minor version update (new features)
	UpdateTypeMinor UpdateType = "minor"

	// UpdateTypePatch indicates a patch version update (bug fixes)
	UpdateTypePatch UpdateType = "patch"

	// UpdateTypeUnknown indicates an unknown update type
	UpdateTypeUnknown UpdateType = "unknown"
)

// Checker checks for module version updates
type Checker struct {
	httpClient     *http.Client
	githubClient   *github.Client
	skipPrerelease bool
	patchOnly      bool
	minorOnly      bool
	ignoreModules  []string
	cache          *cache.RepositoryCache
	aiAnalyzer     AIAnalyzer // Optional AI analyzer for breaking change detection
}

// AIAnalyzer interface for AI-powered breaking change detection
type AIAnalyzer interface {
	AnalyzeBreakingChanges(ctx context.Context, moduleName, currentVersion, latestVersion, changelogURL string) (*ai.AIAnalysis, error)
}

// New creates a new version Checker
func New(githubToken string, skipPrerelease, patchOnly, minorOnly bool, ignoreModules []string) *Checker {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Read token from environment if not provided
	if githubToken == "" {
		githubToken = os.Getenv("GITHUB_TOKEN")
	}

	var githubClient *github.Client
	if githubToken != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: githubToken},
		)
		tc := oauth2.NewClient(context.Background(), ts)
		githubClient = github.NewClient(tc)
		log.Debug().Msg("using authenticated GitHub client")
	} else {
		githubClient = github.NewClient(nil)
		log.Warn().Msg("no GitHub token provided, using unauthenticated client (rate limited to 60 requests/hour)")
	}

	// Create memory-only cache with 24 hour TTL (no disk persistence)
	repoCache := cache.NewMemoryOnly(24 * time.Hour)
	log.Debug().Msg("using in-memory cache for repository data")

	return &Checker{
		httpClient:     httpClient,
		githubClient:   githubClient,
		skipPrerelease: skipPrerelease,
		patchOnly:      patchOnly,
		minorOnly:      minorOnly,
		ignoreModules:  ignoreModules,
		cache:          repoCache,
		aiAnalyzer:     nil, // Will be set via SetAIAnalyzer if needed
	}
}

// SetAIAnalyzer sets the AI analyzer for breaking change detection
func (c *Checker) SetAIAnalyzer(analyzer AIAnalyzer) {
	c.aiAnalyzer = analyzer
}

// Check checks for updates for the given modules
func (c *Checker) Check(ctx context.Context, modules []scanner.ModuleInfo) ([]UpdateInfo, error) {
	var updates []UpdateInfo

	for _, module := range modules {
		// Skip ignored modules
		if c.isIgnored(module.Name) {
			log.Debug().Str("module", module.Name).Msg("skipping ignored module")
			continue
		}

		// Skip local modules
		if module.SourceType == scanner.SourceTypeLocal {
			log.Debug().Str("module", module.Name).Msg("skipping local module")
			continue
		}

		var updateInfo UpdateInfo
		var err error

		switch module.SourceType {
		case scanner.SourceTypeRegistry:
			updateInfo, err = c.checkRegistryModule(ctx, module)
		case scanner.SourceTypeGit:
			updateInfo, err = c.checkGitModule(ctx, module)
		default:
			log.Warn().
				Str("module", module.Name).
				Str("source", module.Source).
				Msg("unsupported source type")
			continue
		}

		if err != nil {
			log.Warn().Err(err).
				Str("module", module.Name).
				Msg("failed to check module version")
			continue
		}

		if updateInfo.IsOutdated {
			// Perform AI analysis if analyzer is configured
			if c.aiAnalyzer != nil {
				aiAnalysis, err := c.aiAnalyzer.AnalyzeBreakingChanges(
					ctx,
					module.Name,
					updateInfo.CurrentVersion,
					updateInfo.LatestVersion,
					updateInfo.ChangelogURL,
				)
				if err != nil {
					log.Warn().Err(err).
						Str("module", module.Name).
						Msg("AI analysis failed, skipping")
				} else {
					updateInfo.AIAnalysis = aiAnalysis
					log.Debug().
						Str("module", module.Name).
						Bool("ai_breaking_changes", aiAnalysis.HasBreakingChanges).
						Str("confidence", aiAnalysis.Confidence).
						Msg("AI analysis completed")
				}
			}

			updates = append(updates, updateInfo)
			log.Info().
				Str("module", module.Name).
				Str("current", updateInfo.CurrentVersion).
				Str("latest", updateInfo.LatestVersion).
				Msg("update available")
		}
	}

	return updates, nil
}

// checkRegistryModule checks for updates from Terraform Registry
func (c *Checker) checkRegistryModule(ctx context.Context, module scanner.ModuleInfo) (UpdateInfo, error) {
	updateInfo := UpdateInfo{
		Module:         module,
		CurrentVersion: extractVersionFromConstraint(module.Version),
	}

	// Parse module source (namespace/name/provider)
	parts := strings.Split(module.Source, "/")
	if len(parts) < 3 {
		return updateInfo, fmt.Errorf("invalid registry source format: %s", module.Source)
	}

	namespace := parts[0]
	name := parts[1]
	provider := parts[2]

	// Query Terraform Registry API
	url := fmt.Sprintf("https://registry.terraform.io/v1/modules/%s/%s/%s/versions",
		namespace, name, provider)

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
		Modules []struct {
			Versions []struct {
				Version string `json:"version"`
			} `json:"versions"`
		} `json:"modules"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&registryResp); err != nil {
		return updateInfo, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(registryResp.Modules) == 0 || len(registryResp.Modules[0].Versions) == 0 {
		return updateInfo, fmt.Errorf("no versions found")
	}

	// Parse and filter versions
	var versions []*version.Version
	for _, v := range registryResp.Modules[0].Versions {
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
	if module.Version != "" {
		cleanVersion := extractVersionFromConstraint(module.Version)
		currentVersion, err := version.NewVersion(cleanVersion)
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
		updateInfo.LatestVersion = "latest"
		log.Debug().
			Str("module", module.Name).
			Msg("module has no version constraint, already using latest")
	}

	// Set changelog URL
	updateInfo.ChangelogURL = fmt.Sprintf("https://registry.terraform.io/modules/%s/%s/%s/%s",
		namespace, name, provider, latestVersion.String())

	return updateInfo, nil
}

// checkGitModule checks for updates from Git repository
func (c *Checker) checkGitModule(ctx context.Context, module scanner.ModuleInfo) (UpdateInfo, error) {
	updateInfo := UpdateInfo{
		Module:         module,
		CurrentVersion: module.Version,
	}

	// Extract GitHub repository from source
	owner, repo, err := c.parseGitSource(module.Source)
	if err != nil {
		return updateInfo, err
	}

	repoKey := fmt.Sprintf("%s/%s", owner, repo)

	// Try to get tags from cache first
	var tagNames []string
	var tags []*github.RepositoryTag

	if c.cache != nil {
		cachedTags, found := c.cache.Get(repoKey)
		if found {
			tagNames = cachedTags
			log.Debug().Str("repository", repoKey).Msg("using cached tags")
		}
	}

	// If not in cache, fetch from GitHub API
	if tagNames == nil {
		tags, _, err = c.githubClient.Repositories.ListTags(ctx, owner, repo, &github.ListOptions{
			PerPage: 100,
		})
		if err != nil {
			return updateInfo, fmt.Errorf("failed to list tags: %w", err)
		}

		if len(tags) == 0 {
			return updateInfo, fmt.Errorf("no tags found")
		}

		// Extract tag names and cache them
		tagNames = make([]string, 0, len(tags))
		for _, tag := range tags {
			tagNames = append(tagNames, tag.GetName())
		}

		// Store in cache
		if c.cache != nil {
			c.cache.Set(repoKey, tagNames)
		}
	}

	// Parse versions from tags
	var versions []*version.Version
	for _, tagName := range tagNames {
		// Remove 'v' prefix if present
		cleanTagName := strings.TrimPrefix(tagName, "v")

		ver, err := version.NewVersion(cleanTagName)
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
		return updateInfo, fmt.Errorf("no valid version tags found")
	}

	// Sort versions
	sort.Sort(version.Collection(versions))

	// Get latest version
	latestVersion := versions[len(versions)-1]
	updateInfo.LatestVersion = latestVersion.String()

	// Extract current version from source (if using ref parameter)
	currentVersion := c.extractGitVersion(module.Source)
	if currentVersion != "" {
		updateInfo.CurrentVersion = currentVersion
		current, err := version.NewVersion(strings.TrimPrefix(currentVersion, "v"))
		if err == nil {
			updateInfo.IsOutdated = c.shouldUpdate(current, latestVersion)

			// Detect breaking changes and update type
			updateInfo.UpdateType = c.detectUpdateType(current, latestVersion)
			updateInfo.HasBreakingChange = updateInfo.UpdateType == UpdateTypeMajor

			if updateInfo.HasBreakingChange {
				updateInfo.BreakingChangeDetails = fmt.Sprintf(
					"Major version upgrade from %s to %s may contain breaking changes. Please review the changelog carefully.",
					current.String(), latestVersion.String())
			}
		}
	} else {
		// No ref specified means using default branch (latest)
		updateInfo.IsOutdated = false
		updateInfo.UpdateType = UpdateTypeUnknown
		updateInfo.CurrentVersion = "default branch"
		updateInfo.LatestVersion = "latest"
		log.Debug().
			Str("module", module.Name).
			Msg("module has no ref parameter, already using default branch (latest)")
	}

	// Set changelog URL
	updateInfo.ChangelogURL = fmt.Sprintf("https://github.com/%s/%s/releases/tag/v%s",
		owner, repo, latestVersion.String())

	return updateInfo, nil
}

// parseGitSource extracts owner and repo from git source
func (c *Checker) parseGitSource(source string) (string, string, error) {
	// Handle different git source formats
	// git::https://github.com/owner/repo.git?ref=v1.0.0
	// git@github.com:owner/repo.git
	// github.com/owner/repo

	re := regexp.MustCompile(`github\.com[/:]([^/]+)/([^.?]+)`)
	matches := re.FindStringSubmatch(source)

	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not parse GitHub repository from source: %s", source)
	}

	return matches[1], matches[2], nil
}

// extractGitVersion extracts version from git source ref parameter
func (c *Checker) extractGitVersion(source string) string {
	// Look for ref= or tag= parameter
	re := regexp.MustCompile(`[?&]ref=([^&]+)`)
	matches := re.FindStringSubmatch(source)

	if len(matches) >= 2 {
		return matches[1]
	}

	return ""
}

// shouldUpdate determines if an update should be performed based on version constraints
func (c *Checker) shouldUpdate(current, latest *version.Version) bool {
	if current.GreaterThanOrEqual(latest) {
		return false
	}

	// Check version update policy
	currentSegments := current.Segments()
	latestSegments := latest.Segments()

	if len(currentSegments) < 3 || len(latestSegments) < 3 {
		return true
	}

	if c.patchOnly {
		// Only update if major and minor are the same
		return currentSegments[0] == latestSegments[0] &&
			currentSegments[1] == latestSegments[1]
	}

	if c.minorOnly {
		// Only update if major is the same
		return currentSegments[0] == latestSegments[0]
	}

	return true
}

// isIgnored checks if a module should be ignored
func (c *Checker) isIgnored(moduleName string) bool {
	for _, ignored := range c.ignoreModules {
		if ignored == moduleName {
			return true
		}
	}
	return false
}

// detectUpdateType determines the type of update based on semantic versioning
func (c *Checker) detectUpdateType(current, latest *version.Version) UpdateType {
	currentSegments := current.Segments()
	latestSegments := latest.Segments()

	// Ensure we have at least major.minor.patch
	if len(currentSegments) < 3 || len(latestSegments) < 3 {
		return UpdateTypeUnknown
	}

	currentMajor := currentSegments[0]
	currentMinor := currentSegments[1]
	currentPatch := currentSegments[2]

	latestMajor := latestSegments[0]
	latestMinor := latestSegments[1]
	latestPatch := latestSegments[2]

	// Major version change (breaking changes)
	if latestMajor > currentMajor {
		return UpdateTypeMajor
	}

	// Minor version change (new features, backwards compatible)
	if latestMajor == currentMajor && latestMinor > currentMinor {
		return UpdateTypeMinor
	}

	// Patch version change (bug fixes, backwards compatible)
	if latestMajor == currentMajor && latestMinor == currentMinor && latestPatch > currentPatch {
		return UpdateTypePatch
	}

	return UpdateTypeUnknown
}

// extractVersionFromConstraint extracts a clean version string from a Terraform version constraint
// Supports: "~> 5.0", ">= 5.0.0", "= 5.0.0", "5.0.0"
func extractVersionFromConstraint(versionConstraint string) string {
	if versionConstraint == "" {
		return ""
	}

	// Remove common constraint operators
	versionConstraint = strings.TrimSpace(versionConstraint)
	versionConstraint = strings.TrimPrefix(versionConstraint, "~>")
	versionConstraint = strings.TrimPrefix(versionConstraint, ">=")
	versionConstraint = strings.TrimPrefix(versionConstraint, "<=")
	versionConstraint = strings.TrimPrefix(versionConstraint, "=")
	versionConstraint = strings.TrimPrefix(versionConstraint, ">")
	versionConstraint = strings.TrimPrefix(versionConstraint, "<")
	versionConstraint = strings.TrimSpace(versionConstraint)

	return versionConstraint
}
