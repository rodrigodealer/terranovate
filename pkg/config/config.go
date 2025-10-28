package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the Terranovate configuration
type Config struct {
	// Terraform settings
	Terraform TerraformConfig `yaml:"terraform"`

	// GitHub settings
	GitHub GitHubConfig `yaml:"github"`

	// Notifier settings
	Notifier NotifierConfig `yaml:"notifier"`

	// Scanner settings
	Scanner ScannerConfig `yaml:"scanner"`

	// Version checking settings
	VersionCheck VersionCheckConfig `yaml:"version_check"`

	// OpenAI settings for AI-powered breaking change detection
	OpenAI OpenAIConfig `yaml:"openai"`
}

// TerraformConfig holds Terraform-related configuration
type TerraformConfig struct {
	// Path to Terraform binary (optional, will use PATH if not specified)
	BinaryPath string `yaml:"binary_path,omitempty"`

	// Working directory for Terraform operations
	WorkingDir string `yaml:"working_dir,omitempty"`

	// Additional environment variables
	Env map[string]string `yaml:"env,omitempty"`
}

// GitHubConfig holds GitHub API configuration
type GitHubConfig struct {
	// GitHub token for API authentication
	Token string `yaml:"token,omitempty"`

	// Base URL for GitHub API (for GitHub Enterprise)
	BaseURL string `yaml:"base_url,omitempty"`

	// Owner/organization name
	Owner string `yaml:"owner,omitempty"`

	// Repository name
	Repo string `yaml:"repo,omitempty"`

	// Base branch for PRs (default: main)
	BaseBranch string `yaml:"base_branch,omitempty"`

	// PR labels to apply
	Labels []string `yaml:"labels,omitempty"`

	// PR reviewers to assign
	Reviewers []string `yaml:"reviewers,omitempty"`
}

// NotifierConfig holds notification configuration
type NotifierConfig struct {
	// Enable Slack notifications
	Slack SlackConfig `yaml:"slack,omitempty"`

	// Output format (json, text)
	OutputFormat string `yaml:"output_format,omitempty"`
}

// SlackConfig holds Slack notification settings
type SlackConfig struct {
	// Webhook URL
	WebhookURL string `yaml:"webhook_url,omitempty"`

	// Channel to post to
	Channel string `yaml:"channel,omitempty"`

	// Enable Slack notifications
	Enabled bool `yaml:"enabled"`
}

// ScannerConfig holds scanner configuration
type ScannerConfig struct {
	// Paths to exclude from scanning
	Exclude []string `yaml:"exclude,omitempty"`

	// File patterns to include (default: *.tf)
	Include []string `yaml:"include,omitempty"`

	// Recurse into subdirectories
	Recursive bool `yaml:"recursive"`
}

// VersionCheckConfig holds version checking configuration
type VersionCheckConfig struct {
	// Skip pre-release versions
	SkipPrerelease bool `yaml:"skip_prerelease"`

	// Only check for patch updates
	PatchOnly bool `yaml:"patch_only"`

	// Only check for minor updates
	MinorOnly bool `yaml:"minor_only"`

	// Modules to ignore
	IgnoreModules []string `yaml:"ignore_modules,omitempty"`

	// Providers to ignore when checking for unused providers
	IgnoreUnusedProviders []string `yaml:"ignore_unused_providers,omitempty"`

	// Display filter: controls which updates to show in output
	// - "all": Show all updates (patch, minor, major) - default
	// - "major-only": Show only major version updates
	// - "minor-and-above": Show minor and major updates (hide patches)
	// - "critical-only": Show only major updates (same as major-only)
	DisplayFilter string `yaml:"display_filter,omitempty"`
}

// OpenAIConfig holds OpenAI API configuration
type OpenAIConfig struct {
	// OpenAI API key
	APIKey string `yaml:"api_key,omitempty"`

	// Enable OpenAI-powered breaking change detection
	Enabled bool `yaml:"enabled"`

	// Model to use (default: gpt-4o-mini)
	Model string `yaml:"model,omitempty"`

	// Base URL for OpenAI API (for custom endpoints)
	BaseURL string `yaml:"base_url,omitempty"`

	// Minimum confidence level to display AI assessments
	// Options: "low", "medium", "high"
	// - "low": Show all AI assessments (default)
	// - "medium": Show only medium and high confidence assessments
	// - "high": Show only high confidence assessments
	MinConfidence string `yaml:"min_confidence,omitempty"`
}

// Load reads and parses the configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	cfg.setDefaults()

	// Override with environment variables
	cfg.overrideWithEnv()

	return &cfg, nil
}

// setDefaults applies default values to configuration
func (c *Config) setDefaults() {
	if c.GitHub.BaseBranch == "" {
		c.GitHub.BaseBranch = "main"
	}

	if c.Notifier.OutputFormat == "" {
		c.Notifier.OutputFormat = "text"
	}

	if len(c.Scanner.Include) == 0 {
		c.Scanner.Include = []string{"*.tf"}
	}

	c.Scanner.Recursive = true

	if c.OpenAI.Model == "" {
		c.OpenAI.Model = "gpt-4o-mini"
	}

	if c.OpenAI.BaseURL == "" {
		c.OpenAI.BaseURL = "https://api.openai.com/v1"
	}

	if c.OpenAI.MinConfidence == "" {
		c.OpenAI.MinConfidence = "low"
	}

	if c.VersionCheck.DisplayFilter == "" {
		c.VersionCheck.DisplayFilter = "all"
	}
}

// overrideWithEnv overrides configuration with environment variables
// Environment variables always take precedence over config file values
func (c *Config) overrideWithEnv() {
	// GitHub token from environment
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		c.GitHub.Token = token
	}

	// OpenAI API key from environment
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		c.OpenAI.APIKey = apiKey
	}

	// OpenAI base URL from environment
	if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
		c.OpenAI.BaseURL = baseURL
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.GitHub.Token == "" {
		// Try to get from environment
		c.GitHub.Token = os.Getenv("GITHUB_TOKEN")
		if c.GitHub.Token == "" {
			return fmt.Errorf("github token is required (set in config or GITHUB_TOKEN env var)")
		}
	}

	return nil
}

// Default returns a default configuration
func Default() *Config {
	cfg := &Config{
		Terraform: TerraformConfig{
			WorkingDir: ".",
		},
		GitHub: GitHubConfig{
			BaseBranch: "main",
			BaseURL:    "https://api.github.com",
		},
		Notifier: NotifierConfig{
			OutputFormat: "text",
		},
		Scanner: ScannerConfig{
			Include:   []string{"*.tf"},
			Recursive: true,
		},
		VersionCheck: VersionCheckConfig{
			SkipPrerelease: true,
		},
	}

	return cfg
}
