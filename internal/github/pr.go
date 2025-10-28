package github

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v66/github"
	"github.com/heyjobs/terranovate/internal/terraform"
	"github.com/heyjobs/terranovate/internal/version"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

// PRCreator creates pull requests for module updates
type PRCreator struct {
	client     *github.Client
	owner      string
	repo       string
	baseBranch string
	labels     []string
	reviewers  []string
	workingDir string
}

// NewPRCreator creates a new PR creator instance
func NewPRCreator(token, owner, repo, baseBranch, workingDir string, labels, reviewers []string) (*PRCreator, error) {
	if token == "" {
		return nil, fmt.Errorf("github token is required")
	}

	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)

	return &PRCreator{
		client:     github.NewClient(tc),
		owner:      owner,
		repo:       repo,
		baseBranch: baseBranch,
		labels:     labels,
		reviewers:  reviewers,
		workingDir: workingDir,
	}, nil
}

// CreatePR creates a pull request for a module update
func (p *PRCreator) CreatePR(ctx context.Context, update version.UpdateInfo, planResult *terraform.PlanResult) (*github.PullRequest, error) {
	// Create branch name
	branchName := fmt.Sprintf("terranovate/%s-%s",
		sanitizeBranchName(update.Module.Name),
		update.LatestVersion)

	log.Info().
		Str("branch", branchName).
		Str("module", update.Module.Name).
		Msg("creating pull request")

	// Create and checkout new branch
	if err := p.createBranch(branchName); err != nil {
		return nil, fmt.Errorf("failed to create branch: %w", err)
	}

	// Update module version in file
	if err := p.updateModuleVersion(update); err != nil {
		return nil, fmt.Errorf("failed to update module version: %w", err)
	}

	// Commit changes
	commitMsg := fmt.Sprintf("Update %s to %s", update.Module.Name, update.LatestVersion)
	if err := p.commitChanges(commitMsg); err != nil {
		return nil, fmt.Errorf("failed to commit changes: %w", err)
	}

	// Push branch
	if err := p.pushBranch(branchName); err != nil {
		return nil, fmt.Errorf("failed to push branch: %w", err)
	}

	// Create PR
	title := fmt.Sprintf("Update Terraform module %s to %s", update.Module.Name, update.LatestVersion)
	if update.HasBreakingChange {
		title = fmt.Sprintf("⚠️ [BREAKING] Update Terraform module %s to %s", update.Module.Name, update.LatestVersion)
	}
	body := p.generatePRBody(update, planResult)

	pr, _, err := p.client.PullRequests.Create(ctx, p.owner, p.repo, &github.NewPullRequest{
		Title:               github.String(title),
		Head:                github.String(branchName),
		Base:                github.String(p.baseBranch),
		Body:                github.String(body),
		MaintainerCanModify: github.Bool(true),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create PR: %w", err)
	}

	// Prepare labels
	labels := make([]string, len(p.labels))
	copy(labels, p.labels)

	// Add breaking-change label if applicable
	if update.HasBreakingChange {
		labels = append(labels, "breaking-change")
	}

	// Add update type label
	if update.UpdateType != "" && update.UpdateType != "unknown" {
		labels = append(labels, string(update.UpdateType)+"-update")
	}

	// Add labels
	if len(labels) > 0 {
		if _, _, err := p.client.Issues.AddLabelsToIssue(ctx, p.owner, p.repo, pr.GetNumber(), labels); err != nil {
			log.Warn().Err(err).Msg("failed to add labels to PR")
		}
	}

	// Add reviewers if configured
	if len(p.reviewers) > 0 {
		reviewersReq := github.ReviewersRequest{
			Reviewers: p.reviewers,
		}
		if _, _, err := p.client.PullRequests.RequestReviewers(ctx, p.owner, p.repo, pr.GetNumber(), reviewersReq); err != nil {
			log.Warn().Err(err).Msg("failed to request reviewers")
		}
	}

	log.Info().
		Str("url", pr.GetHTMLURL()).
		Int("number", pr.GetNumber()).
		Msg("pull request created successfully")

	return pr, nil
}

// createBranch creates and checks out a new git branch
func (p *PRCreator) createBranch(branchName string) error {
	// Fetch latest changes
	if err := p.runGitCommand("fetch", "origin", p.baseBranch); err != nil {
		return err
	}

	// Checkout base branch
	if err := p.runGitCommand("checkout", p.baseBranch); err != nil {
		return err
	}

	// Pull latest changes
	if err := p.runGitCommand("pull", "origin", p.baseBranch); err != nil {
		return err
	}

	// Create new branch
	if err := p.runGitCommand("checkout", "-b", branchName); err != nil {
		return err
	}

	return nil
}

// updateModuleVersion updates the module version in the Terraform file
func (p *PRCreator) updateModuleVersion(update version.UpdateInfo) error {
	filePath := update.Module.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(p.workingDir, filePath)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Replace version in the module block
	oldContent := string(content)
	newContent := oldContent

	// Handle different version specification formats
	if update.CurrentVersion != "" {
		// Replace version = "old" with version = "new"
		oldVersion := fmt.Sprintf(`version = "%s"`, update.CurrentVersion)
		newVersion := fmt.Sprintf(`version = "%s"`, update.LatestVersion)
		newContent = strings.Replace(newContent, oldVersion, newVersion, 1)

		// Also try single quotes
		oldVersion = fmt.Sprintf(`version = '%s'`, update.CurrentVersion)
		newVersion = fmt.Sprintf(`version = '%s'`, update.LatestVersion)
		newContent = strings.Replace(newContent, oldVersion, newVersion, 1)
	} else {
		// Add version attribute if it doesn't exist
		// Find the module block and add version
		moduleLine := fmt.Sprintf(`module "%s"`, update.Module.Name)
		if strings.Contains(newContent, moduleLine) {
			// Add version after module declaration
			replacement := fmt.Sprintf("%s {\n  version = \"%s\"\n", moduleLine, update.LatestVersion)
			newContent = strings.Replace(newContent, moduleLine+" {", replacement, 1)
		}
	}

	// For git sources, update the ref parameter
	if update.Module.SourceType == "git" && update.CurrentVersion != "" {
		oldRef := fmt.Sprintf("ref=%s", update.CurrentVersion)
		newRef := fmt.Sprintf("ref=v%s", update.LatestVersion)
		newContent = strings.Replace(newContent, oldRef, newRef, 1)
	}

	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// commitChanges commits the changes to git
func (p *PRCreator) commitChanges(message string) error {
	if err := p.runGitCommand("add", "."); err != nil {
		return err
	}

	fullMessage := fmt.Sprintf("%s\n\nAutomated update by Terranovate", message)
	if err := p.runGitCommand("commit", "-m", fullMessage); err != nil {
		return err
	}

	return nil
}

// pushBranch pushes the branch to origin
func (p *PRCreator) pushBranch(branchName string) error {
	return p.runGitCommand("push", "-u", "origin", branchName)
}

// runGitCommand executes a git command in the working directory
func (p *PRCreator) runGitCommand(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = p.workingDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Debug().Strs("args", args).Msg("running git command")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
	}

	return nil
}

// generatePRBody generates the pull request body
func (p *PRCreator) generatePRBody(update version.UpdateInfo, planResult *terraform.PlanResult) string {
	var body strings.Builder

	body.WriteString("## Terraform Module Update\n\n")

	// Add breaking change warning at the top if applicable
	if update.HasBreakingChange {
		body.WriteString("## ⚠️ BREAKING CHANGE WARNING\n\n")
		body.WriteString(fmt.Sprintf("> **%s**\n\n", update.BreakingChangeDetails))
		body.WriteString("This is a **major version** update and may require code changes. ")
		body.WriteString("Please review the changelog carefully and test thoroughly before merging.\n\n")
		body.WriteString("---\n\n")
	}

	updateTypeLabel := ""
	if update.UpdateType != "" {
		switch update.UpdateType {
		case "major":
			updateTypeLabel = " (Major Update 🔴)"
		case "minor":
			updateTypeLabel = " (Minor Update 🟡)"
		case "patch":
			updateTypeLabel = " (Patch Update 🟢)"
		}
	}

	body.WriteString(fmt.Sprintf("This PR updates the Terraform module **%s** from version `%s` to `%s`%s.\n\n",
		update.Module.Name, update.CurrentVersion, update.LatestVersion, updateTypeLabel))

	body.WriteString("### Module Details\n\n")
	body.WriteString(fmt.Sprintf("- **Source**: `%s`\n", update.Module.Source))
	body.WriteString(fmt.Sprintf("- **File**: `%s:%d`\n", update.Module.FilePath, update.Module.Line))
	body.WriteString(fmt.Sprintf("- **Current Version**: `%s`\n", update.CurrentVersion))
	body.WriteString(fmt.Sprintf("- **Latest Version**: `%s`\n", update.LatestVersion))
	if update.UpdateType != "" && update.UpdateType != "unknown" {
		body.WriteString(fmt.Sprintf("- **Update Type**: %s\n", update.UpdateType))
	}
	body.WriteString("\n")

	if update.ChangelogURL != "" {
		body.WriteString(fmt.Sprintf("📋 [View Changelog](%s)\n\n", update.ChangelogURL))
	}

	// Add schema change details if available
	if update.SchemaChanges != nil {
		// Type assert to terraform.SchemaChanges
		if schemaChanges, ok := update.SchemaChanges.(*terraform.SchemaChanges); ok && schemaChanges.HasChanges {
			body.WriteString("### 📋 API/Schema Changes Detected\n\n")

			if len(schemaChanges.AddedRequiredVars) > 0 {
				body.WriteString(fmt.Sprintf("#### ⚠️ New Required Variables (%d)\n\n", len(schemaChanges.AddedRequiredVars)))
				body.WriteString("The following required variables have been added and must be provided:\n\n")
				for _, v := range schemaChanges.AddedRequiredVars {
					body.WriteString(fmt.Sprintf("- `%s` (%s)\n", v.Name, v.Type))
					if v.Description != "" {
						body.WriteString(fmt.Sprintf("  - %s\n", v.Description))
					}
				}
				body.WriteString("\n")
			}

			if len(schemaChanges.RemovedVars) > 0 {
				body.WriteString(fmt.Sprintf("#### 🗑️ Removed Variables (%d)\n\n", len(schemaChanges.RemovedVars)))
				for _, v := range schemaChanges.RemovedVars {
					body.WriteString(fmt.Sprintf("- `%s` (%s)\n", v.Name, v.Type))
				}
				body.WriteString("\n")
			}

			if len(schemaChanges.ChangedVarTypes) > 0 {
				body.WriteString(fmt.Sprintf("#### ⚙️ Changed Variable Types (%d)\n\n", len(schemaChanges.ChangedVarTypes)))
				for _, v := range schemaChanges.ChangedVarTypes {
					body.WriteString(fmt.Sprintf("- `%s`: %s\n", v.Name, v.Type))
				}
				body.WriteString("\n")
			}

			if len(schemaChanges.RemovedOutputs) > 0 {
				body.WriteString(fmt.Sprintf("#### 📤 Removed Outputs (%d)\n\n", len(schemaChanges.RemovedOutputs)))
				body.WriteString("The following outputs have been removed:\n\n")
				for _, o := range schemaChanges.RemovedOutputs {
					body.WriteString(fmt.Sprintf("- `%s`\n", o.Name))
				}
				body.WriteString("\n")
			}
		}
	}

	// Add resource change details if available
	if update.ResourceChanges != nil && update.ResourceChanges.HasChanges {
		body.WriteString("### 🔍 Resource Changes Detected\n\n")

		if update.ResourceChanges.TotalReplace > 0 {
			body.WriteString(fmt.Sprintf("#### ⚠️ Resources to be REPLACED (%d)\n\n", update.ResourceChanges.TotalReplace))
			body.WriteString("The following resources will be destroyed and recreated:\n\n")
			for _, rc := range update.ResourceChanges.ResourcesToReplace {
				body.WriteString(fmt.Sprintf("- `%s` (%s)\n", rc.Address, rc.ResourceType))
				if rc.Reason != "" {
					body.WriteString(fmt.Sprintf("  - Reason: %s\n", rc.Reason))
				}
			}
			body.WriteString("\n")
		}

		if update.ResourceChanges.TotalDelete > 0 {
			body.WriteString(fmt.Sprintf("#### 🗑️ Resources to be DELETED (%d)\n\n", update.ResourceChanges.TotalDelete))
			for _, rc := range update.ResourceChanges.ResourcesToDelete {
				body.WriteString(fmt.Sprintf("- `%s` (%s)\n", rc.Address, rc.ResourceType))
			}
			body.WriteString("\n")
		}

		if update.ResourceChanges.TotalModify > 0 {
			body.WriteString(fmt.Sprintf("#### 📝 Resources to be MODIFIED (%d)\n\n", update.ResourceChanges.TotalModify))
			body.WriteString("Some resource attributes will be updated in-place.\n\n")
		}
	}

	// Add specific guidance for breaking changes
	if update.HasBreakingChange {
		body.WriteString("### Review Checklist for Breaking Changes\n\n")
		body.WriteString("Before merging this PR, please ensure you have:\n\n")
		body.WriteString("- [ ] Reviewed the [changelog](" + update.ChangelogURL + ") for breaking changes\n")
		body.WriteString("- [ ] Identified any code that needs to be updated\n")
		body.WriteString("- [ ] Tested the changes in a non-production environment\n")
		body.WriteString("- [ ] Updated any documentation affected by this change\n")
		body.WriteString("- [ ] Verified the Terraform plan output below\n")
		if update.ResourceChanges != nil && (update.ResourceChanges.TotalReplace > 0 || update.ResourceChanges.TotalDelete > 0) {
			body.WriteString("- [ ] Confirmed resource replacements/deletions are acceptable\n")
		}
		body.WriteString("\n")
	}

	if planResult != nil {
		body.WriteString("### Terraform Plan Results\n\n")
		if planResult.Success {
			body.WriteString("✅ Plan succeeded\n\n")
			body.WriteString(fmt.Sprintf("```\n%s\n```\n\n", planResult.Output))

			if planResult.HasChanges {
				body.WriteString("⚠️ **This update will make infrastructure changes.**\n\n")
				body.WriteString("Please review the plan carefully before merging.\n\n")
			} else {
				body.WriteString("✨ No infrastructure changes detected.\n\n")
			}
		} else {
			body.WriteString("❌ Plan failed\n\n")
			body.WriteString(fmt.Sprintf("```\n%s\n```\n\n", planResult.ErrorMessage))
			body.WriteString("⚠️ **Please review and fix the errors before merging.**\n\n")
		}
	}

	body.WriteString("---\n")
	body.WriteString("🤖 *This PR was automatically created by [Terranovate](https://github.com/heyjobs/terranovate)*\n")

	return body.String()
}

// CreateProviderPR creates a pull request for a provider update
func (p *PRCreator) CreateProviderPR(ctx context.Context, update version.ProviderUpdateInfo) (*github.PullRequest, error) {
	// Create branch name
	branchName := fmt.Sprintf("terranovate/provider-%s-%s",
		sanitizeBranchName(update.Provider.Name),
		update.LatestVersion)

	log.Info().
		Str("branch", branchName).
		Str("provider", update.Provider.Name).
		Msg("creating pull request for provider update")

	// Create and checkout new branch
	if err := p.createBranch(branchName); err != nil {
		return nil, fmt.Errorf("failed to create branch: %w", err)
	}

	// Update provider version in file
	if err := p.updateProviderVersion(update); err != nil {
		return nil, fmt.Errorf("failed to update provider version: %w", err)
	}

	// Commit changes
	commitMsg := fmt.Sprintf("Update provider %s to %s", update.Provider.Name, update.LatestVersion)
	if err := p.commitChanges(commitMsg); err != nil {
		return nil, fmt.Errorf("failed to commit changes: %w", err)
	}

	// Push branch
	if err := p.pushBranch(branchName); err != nil {
		return nil, fmt.Errorf("failed to push branch: %w", err)
	}

	// Create PR
	title := fmt.Sprintf("Update Terraform provider %s to %s", update.Provider.Name, update.LatestVersion)
	if update.HasBreakingChange {
		title = fmt.Sprintf("⚠️ [BREAKING] Update Terraform provider %s to %s", update.Provider.Name, update.LatestVersion)
	}
	body := p.generateProviderPRBody(update)

	pr, _, err := p.client.PullRequests.Create(ctx, p.owner, p.repo, &github.NewPullRequest{
		Title:               github.String(title),
		Head:                github.String(branchName),
		Base:                github.String(p.baseBranch),
		Body:                github.String(body),
		MaintainerCanModify: github.Bool(true),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create PR: %w", err)
	}

	// Prepare labels
	labels := make([]string, len(p.labels))
	copy(labels, p.labels)
	labels = append(labels, "provider")

	// Add breaking-change label if applicable
	if update.HasBreakingChange {
		labels = append(labels, "breaking-change")
	}

	// Add update type label
	if update.UpdateType != "" && update.UpdateType != "unknown" {
		labels = append(labels, string(update.UpdateType)+"-update")
	}

	// Add labels
	if len(labels) > 0 {
		if _, _, err := p.client.Issues.AddLabelsToIssue(ctx, p.owner, p.repo, pr.GetNumber(), labels); err != nil {
			log.Warn().Err(err).Msg("failed to add labels to PR")
		}
	}

	// Add reviewers if configured
	if len(p.reviewers) > 0 {
		reviewersReq := github.ReviewersRequest{
			Reviewers: p.reviewers,
		}
		if _, _, err := p.client.PullRequests.RequestReviewers(ctx, p.owner, p.repo, pr.GetNumber(), reviewersReq); err != nil {
			log.Warn().Err(err).Msg("failed to request reviewers")
		}
	}

	log.Info().
		Str("url", pr.GetHTMLURL()).
		Int("number", pr.GetNumber()).
		Msg("pull request created successfully for provider")

	return pr, nil
}

// updateProviderVersion updates the provider version in the Terraform file
func (p *PRCreator) updateProviderVersion(update version.ProviderUpdateInfo) error {
	filePath := update.Provider.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(p.workingDir, filePath)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Find the required_providers block and update the version
	inTerraformBlock := false
	inRequiredProvidersBlock := false
	providerIndent := ""
	updated := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track terraform block
		if strings.HasPrefix(trimmed, "terraform") && strings.Contains(trimmed, "{") {
			inTerraformBlock = true
			continue
		}

		// Track required_providers block
		if inTerraformBlock && strings.HasPrefix(trimmed, "required_providers") && strings.Contains(trimmed, "{") {
			inRequiredProvidersBlock = true
			continue
		}

		// Check if we're at the provider line
		if inRequiredProvidersBlock && strings.Contains(line, update.Provider.Name) {
			// Detect indentation
			for _, c := range line {
				if c == ' ' || c == '\t' {
					providerIndent += string(c)
				} else {
					break
				}
			}

			// Look for version specification in this line or next lines
			if strings.Contains(line, "version") {
				// Version is on the same line
				lines[i] = p.replaceProviderVersionInLine(line, update.CurrentVersion, update.LatestVersion)
				updated = true
				break
			} else {
				// Look ahead for version in subsequent lines
				for j := i + 1; j < len(lines) && j < i+10; j++ {
					if strings.Contains(lines[j], "version") {
						lines[j] = p.replaceProviderVersionInLine(lines[j], update.CurrentVersion, update.LatestVersion)
						updated = true
						break
					}
					// Stop if we've exited the provider block
					if strings.TrimSpace(lines[j]) == "}" {
						break
					}
				}
				if updated {
					break
				}
			}
		}

		// Track block exits
		if trimmed == "}" {
			if inRequiredProvidersBlock {
				inRequiredProvidersBlock = false
			} else if inTerraformBlock {
				inTerraformBlock = false
			}
		}
	}

	if !updated {
		return fmt.Errorf("could not find provider version to update in file %s", filePath)
	}

	newContent := strings.Join(lines, "\n")
	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// replaceProviderVersionInLine replaces the version in a provider line
func (p *PRCreator) replaceProviderVersionInLine(line, currentVersion, latestVersion string) string {
	// Extract current version from line (handles different formats)
	// Examples:
	//   version = "~> 5.0"
	//   version = ">= 5.0.0"
	//   version = "5.0.0"

	// Find the version value in quotes
	versionStart := strings.Index(line, "\"")
	if versionStart == -1 {
		return line
	}

	versionEnd := strings.Index(line[versionStart+1:], "\"")
	if versionEnd == -1 {
		return line
	}

	oldValue := line[versionStart+1 : versionStart+1+versionEnd]

	// Determine the constraint operator to preserve
	constraint := ""
	if strings.HasPrefix(oldValue, "~>") {
		constraint = "~> "
	} else if strings.HasPrefix(oldValue, ">=") {
		constraint = ">= "
	} else if strings.HasPrefix(oldValue, "<=") {
		constraint = "<= "
	} else if strings.HasPrefix(oldValue, "=") {
		constraint = "= "
	} else if strings.HasPrefix(oldValue, ">") {
		constraint = "> "
	} else if strings.HasPrefix(oldValue, "<") {
		constraint = "< "
	}

	newValue := constraint + latestVersion
	return strings.Replace(line, "\""+oldValue+"\"", "\""+newValue+"\"", 1)
}

// generateProviderPRBody generates the PR body for a provider update
func (p *PRCreator) generateProviderPRBody(update version.ProviderUpdateInfo) string {
	var body strings.Builder

	// Add breaking change warning banner if applicable
	if update.HasBreakingChange {
		body.WriteString("## ⚠️ Breaking Change Warning\n\n")
		body.WriteString(update.BreakingChangeDetails)
		body.WriteString("\n\n")
		body.WriteString("**Please review the provider documentation and upgrade guide carefully before merging.**\n\n")
		body.WriteString("---\n\n")
	}

	body.WriteString("## Provider Update\n\n")
	body.WriteString(fmt.Sprintf("Updates the **%s** provider.\n\n", update.Provider.Name))

	body.WriteString("### Update Details\n\n")
	body.WriteString(fmt.Sprintf("- **Provider**: `%s`\n", update.Provider.Source))
	body.WriteString(fmt.Sprintf("- **Current Version**: `%s`\n", update.CurrentVersion))
	body.WriteString(fmt.Sprintf("- **New Version**: `%s`\n", update.LatestVersion))

	if update.UpdateType != "" && update.UpdateType != version.UpdateTypeUnknown {
		updateTypeLabel := ""
		switch update.UpdateType {
		case version.UpdateTypeMajor:
			updateTypeLabel = "🔴 Major"
		case version.UpdateTypeMinor:
			updateTypeLabel = "🟡 Minor"
		case version.UpdateTypePatch:
			updateTypeLabel = "🟢 Patch"
		}
		body.WriteString(fmt.Sprintf("- **Update Type**: %s\n", updateTypeLabel))
	}

	body.WriteString(fmt.Sprintf("- **File**: `%s:%d`\n", update.Provider.FilePath, update.Provider.Line))
	body.WriteString("\n")

	if update.ChangelogURL != "" {
		body.WriteString(fmt.Sprintf("📖 [View provider documentation](%s)\n\n", update.ChangelogURL))
	}

	// Add review checklist
	body.WriteString("### Review Checklist\n\n")

	if update.HasBreakingChange {
		body.WriteString("- [ ] Review provider upgrade guide and breaking changes\n")
		body.WriteString("- [ ] Check for deprecated resources or data sources\n")
		body.WriteString("- [ ] Verify all resource configurations are compatible\n")
	}

	body.WriteString("- [ ] Review provider changelog\n")
	body.WriteString("- [ ] Run `terraform init -upgrade` to update provider\n")
	body.WriteString("- [ ] Run `terraform plan` to verify no unexpected changes\n")
	body.WriteString("- [ ] Test in non-production environment first\n")

	if update.HasBreakingChange {
		body.WriteString("- [ ] Update resource configurations if needed\n")
		body.WriteString("- [ ] Communicate changes to team\n")
	}

	body.WriteString("\n")
	body.WriteString("---\n")
	body.WriteString("🤖 *This PR was automatically created by [Terranovate](https://github.com/heyjobs/terranovate)*\n")

	return body.String()
}

// sanitizeBranchName sanitizes a string to be used as a git branch name
func sanitizeBranchName(name string) string {
	// Replace invalid characters with hyphens
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, name)

	// Remove consecutive hyphens
	for strings.Contains(name, "--") {
		name = strings.Replace(name, "--", "-", -1)
	}

	// Trim hyphens from start and end
	name = strings.Trim(name, "-")

	return strings.ToLower(name)
}
