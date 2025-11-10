package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		wantErr     bool
		validate    func(*testing.T, *Config)
	}{
		{
			name: "valid config with all fields",
			yamlContent: `
terraform:
  binary_path: /usr/bin/terraform
  working_dir: /path/to/project
  env:
    TF_LOG: DEBUG

github:
  token: ghp_test123
  base_url: https://github.example.com
  owner: myorg
  repo: myrepo
  base_branch: develop
  labels:
    - terraform
    - automation
  reviewers:
    - reviewer1
    - reviewer2

notifier:
  output_format: json
  slack:
    webhook_url: https://hooks.slack.com/test
    channel: '#terraform'
    enabled: true

scanner:
  exclude:
    - .terraform
    - .git
  include:
    - '*.tf'
    - '*.tfvars'
  recursive: true

version_check:
  skip_prerelease: true
  patch_only: false
  minor_only: true
  ignore_modules:
    - test-module
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Terraform.BinaryPath != "/usr/bin/terraform" {
					t.Errorf("Terraform.BinaryPath = %s, want /usr/bin/terraform", cfg.Terraform.BinaryPath)
				}
				if cfg.GitHub.Token != "ghp_test123" {
					t.Errorf("GitHub.Token = %s, want ghp_test123", cfg.GitHub.Token)
				}
				if cfg.GitHub.BaseBranch != "develop" {
					t.Errorf("GitHub.BaseBranch = %s, want develop", cfg.GitHub.BaseBranch)
				}
				if cfg.Notifier.OutputFormat != "json" {
					t.Errorf("Notifier.OutputFormat = %s, want json", cfg.Notifier.OutputFormat)
				}
				if !cfg.Notifier.Slack.Enabled {
					t.Error("Notifier.Slack.Enabled = false, want true")
				}
				if cfg.VersionCheck.MinorOnly != true {
					t.Error("VersionCheck.MinorOnly = false, want true")
				}
			},
		},
		{
			name: "minimal valid config",
			yamlContent: `
github:
  token: ghp_minimal
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				// Check defaults are applied
				if cfg.GitHub.BaseBranch != "main" {
					t.Errorf("GitHub.BaseBranch = %s, want main (default)", cfg.GitHub.BaseBranch)
				}
				if cfg.Notifier.OutputFormat != "text" {
					t.Errorf("Notifier.OutputFormat = %s, want text (default)", cfg.Notifier.OutputFormat)
				}
				if len(cfg.Scanner.Include) != 1 || cfg.Scanner.Include[0] != "*.tf" {
					t.Errorf("Scanner.Include = %v, want [*.tf] (default)", cfg.Scanner.Include)
				}
				if !cfg.Scanner.Recursive {
					t.Error("Scanner.Recursive = false, want true (default)")
				}
			},
		},
		{
			name:        "invalid yaml",
			yamlContent: `invalid: yaml: content: [`,
			wantErr:     true,
		},
		{
			name:        "empty file",
			yamlContent: ``,
			wantErr:     false,
			validate: func(t *testing.T, cfg *Config) {
				// Defaults should be applied
				if cfg.GitHub.BaseBranch != "main" {
					t.Errorf("GitHub.BaseBranch = %s, want main", cfg.GitHub.BaseBranch)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment variables that could interfere with tests
			oldGitHubToken := os.Getenv("GITHUB_TOKEN")
			oldOpenAIKey := os.Getenv("OPENAI_API_KEY")
			oldOpenAIBaseURL := os.Getenv("OPENAI_BASE_URL")
			os.Unsetenv("GITHUB_TOKEN")
			os.Unsetenv("OPENAI_API_KEY")
			os.Unsetenv("OPENAI_BASE_URL")
			defer func() {
				if oldGitHubToken != "" {
					os.Setenv("GITHUB_TOKEN", oldGitHubToken)
				}
				if oldOpenAIKey != "" {
					os.Setenv("OPENAI_API_KEY", oldOpenAIKey)
				}
				if oldOpenAIBaseURL != "" {
					os.Setenv("OPENAI_BASE_URL", oldOpenAIBaseURL)
				}
			}()

			// Create temporary file
			tmpFile, err := os.CreateTemp("", "config-test-*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			// Write test content
			if _, err := tmpFile.WriteString(tt.yamlContent); err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}
			tmpFile.Close()

			// Load config
			cfg, err := Load(tmpFile.Name())
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Load() expected error for non-existent file, got nil")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		envVars map[string]string
		wantErr bool
	}{
		{
			name: "valid config with token",
			config: &Config{
				GitHub: GitHubConfig{
					Token: "ghp_test123",
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with token from env",
			config: &Config{
				GitHub: GitHubConfig{},
			},
			envVars: map[string]string{
				"GITHUB_TOKEN": "ghp_from_env",
			},
			wantErr: false,
		},
		{
			name: "missing token",
			config: &Config{
				GitHub: GitHubConfig{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			// If token was set from env, verify it was populated
			if !tt.wantErr && len(tt.envVars) > 0 {
				if tt.config.GitHub.Token == "" {
					t.Error("Expected token to be populated from environment")
				}
			}
		})
	}
}

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg == nil {
		t.Fatal("Default() returned nil")
	}

	// Test default values
	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"Terraform.WorkingDir", cfg.Terraform.WorkingDir, "."},
		{"GitHub.BaseBranch", cfg.GitHub.BaseBranch, "main"},
		{"GitHub.BaseURL", cfg.GitHub.BaseURL, "https://api.github.com"},
		{"Notifier.OutputFormat", cfg.Notifier.OutputFormat, "text"},
		{"Scanner.Include", len(cfg.Scanner.Include), 1},
		{"Scanner.Recursive", cfg.Scanner.Recursive, true},
		{"VersionCheck.SkipPrerelease", cfg.VersionCheck.SkipPrerelease, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}

	// Verify Scanner.Include default value
	if cfg.Scanner.Include[0] != "*.tf" {
		t.Errorf("Scanner.Include[0] = %s, want *.tf", cfg.Scanner.Include[0])
	}
}

func TestSetDefaults(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		validate func(*testing.T, *Config)
	}{
		{
			name: "empty config gets all defaults",
			config: &Config{
				GitHub:   GitHubConfig{},
				Notifier: NotifierConfig{},
				Scanner:  ScannerConfig{},
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.GitHub.BaseBranch != "main" {
					t.Errorf("BaseBranch = %s, want main", cfg.GitHub.BaseBranch)
				}
				if cfg.Notifier.OutputFormat != "text" {
					t.Errorf("OutputFormat = %s, want text", cfg.Notifier.OutputFormat)
				}
				if len(cfg.Scanner.Include) != 1 || cfg.Scanner.Include[0] != "*.tf" {
					t.Errorf("Include = %v, want [*.tf]", cfg.Scanner.Include)
				}
				if !cfg.Scanner.Recursive {
					t.Error("Recursive = false, want true")
				}
			},
		},
		{
			name: "existing values are preserved",
			config: &Config{
				GitHub: GitHubConfig{
					BaseBranch: "develop",
				},
				Notifier: NotifierConfig{
					OutputFormat: "json",
				},
				Scanner: ScannerConfig{
					Include: []string{"*.tfvars"},
				},
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.GitHub.BaseBranch != "develop" {
					t.Errorf("BaseBranch = %s, want develop", cfg.GitHub.BaseBranch)
				}
				if cfg.Notifier.OutputFormat != "json" {
					t.Errorf("OutputFormat = %s, want json", cfg.Notifier.OutputFormat)
				}
				if len(cfg.Scanner.Include) != 1 || cfg.Scanner.Include[0] != "*.tfvars" {
					t.Errorf("Include = %v, want [*.tfvars]", cfg.Scanner.Include)
				}
				// Recursive should still be set to true
				if !cfg.Scanner.Recursive {
					t.Error("Recursive = false, want true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.setDefaults()
			tt.validate(t, tt.config)
		})
	}
}

func TestLoadIntegration(t *testing.T) {
	// Test loading a realistic config file
	configContent := `
terraform:
  working_dir: ./terraform

github:
  token: ghp_integration_test
  owner: testorg
  repo: testrepo
  labels:
    - terraform
    - dependencies

scanner:
  exclude:
    - .terraform
    - .terragrunt-cache
  recursive: true

version_check:
  skip_prerelease: true
  ignore_modules:
    - local-module
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify loaded values
	if cfg.Terraform.WorkingDir != "./terraform" {
		t.Errorf("WorkingDir = %s, want ./terraform", cfg.Terraform.WorkingDir)
	}

	if cfg.GitHub.Owner != "testorg" {
		t.Errorf("Owner = %s, want testorg", cfg.GitHub.Owner)
	}

	if len(cfg.GitHub.Labels) != 2 {
		t.Errorf("Labels length = %d, want 2", len(cfg.GitHub.Labels))
	}

	if len(cfg.Scanner.Exclude) != 2 {
		t.Errorf("Exclude length = %d, want 2", len(cfg.Scanner.Exclude))
	}

	if !cfg.VersionCheck.SkipPrerelease {
		t.Error("SkipPrerelease = false, want true")
	}

	// Verify defaults were applied
	if cfg.GitHub.BaseBranch != "main" {
		t.Errorf("BaseBranch = %s, want main (default)", cfg.GitHub.BaseBranch)
	}
}
