package github

import (
	"strings"
	"testing"

	"github.com/heyjobs/terranovate/internal/scanner"
	"github.com/heyjobs/terranovate/internal/terraform"
	"github.com/heyjobs/terranovate/internal/version"
)

func TestNewPRCreator(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		owner       string
		repo        string
		baseBranch  string
		workingDir  string
		labels      []string
		reviewers   []string
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid configuration",
			token:      "ghp_test123",
			owner:      "testorg",
			repo:       "testrepo",
			baseBranch: "main",
			workingDir: "/project",
			labels:     []string{"terraform"},
			reviewers:  []string{"reviewer1"},
			wantErr:    false,
		},
		{
			name:        "missing token",
			token:       "",
			owner:       "testorg",
			repo:        "testrepo",
			baseBranch:  "main",
			workingDir:  "/project",
			wantErr:     true,
			errContains: "token is required",
		},
		{
			name:        "missing owner",
			token:       "ghp_test123",
			owner:       "",
			repo:        "testrepo",
			baseBranch:  "main",
			workingDir:  "/project",
			wantErr:     true,
			errContains: "owner and repo are required",
		},
		{
			name:        "missing repo",
			token:       "ghp_test123",
			owner:       "testorg",
			repo:        "",
			baseBranch:  "main",
			workingDir:  "/project",
			wantErr:     true,
			errContains: "owner and repo are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creator, err := NewPRCreator(tt.token, tt.owner, tt.repo, tt.baseBranch, tt.workingDir, tt.labels, tt.reviewers)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewPRCreator() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if err != nil && tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("NewPRCreator() error = %v, should contain %q", err, tt.errContains)
				}
				return
			}

			if creator == nil {
				t.Fatal("NewPRCreator() returned nil creator")
			}

			if creator.owner != tt.owner {
				t.Errorf("creator.owner = %s, want %s", creator.owner, tt.owner)
			}

			if creator.repo != tt.repo {
				t.Errorf("creator.repo = %s, want %s", creator.repo, tt.repo)
			}

			if creator.baseBranch != tt.baseBranch {
				t.Errorf("creator.baseBranch = %s, want %s", creator.baseBranch, tt.baseBranch)
			}

			if creator.workingDir != tt.workingDir {
				t.Errorf("creator.workingDir = %s, want %s", creator.workingDir, tt.workingDir)
			}

			if len(creator.labels) != len(tt.labels) {
				t.Errorf("creator.labels length = %d, want %d", len(creator.labels), len(tt.labels))
			}

			if len(creator.reviewers) != len(tt.reviewers) {
				t.Errorf("creator.reviewers length = %d, want %d", len(creator.reviewers), len(tt.reviewers))
			}

			if creator.client == nil {
				t.Error("creator.client is nil")
			}
		})
	}
}

func TestSanitizeBranchName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple name",
			input: "feature-branch",
			want:  "feature-branch",
		},
		{
			name:  "with special characters",
			input: "feature/branch",
			want:  "feature-branch",
		},
		{
			name:  "with spaces",
			input: "feature branch",
			want:  "feature-branch",
		},
		{
			name:  "with multiple special characters",
			input: "feature@#$%branch",
			want:  "feature-branch",
		},
		{
			name:  "with consecutive special characters",
			input: "feature///branch",
			want:  "feature-branch",
		},
		{
			name:  "with leading and trailing hyphens",
			input: "-feature-branch-",
			want:  "feature-branch",
		},
		{
			name:  "uppercase",
			input: "FEATURE-BRANCH",
			want:  "feature-branch",
		},
		{
			name:  "mixed case with special chars",
			input: "Feature/Branch@123",
			want:  "feature-branch-123",
		},
		{
			name:  "underscores preserved",
			input: "feature_branch_name",
			want:  "feature_branch_name",
		},
		{
			name:  "numbers preserved",
			input: "feature-123-branch",
			want:  "feature-123-branch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeBranchName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeBranchName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGeneratePRBody(t *testing.T) {
	creator, _ := NewPRCreator("ghp_test", "testorg", "testrepo", "main", "/project", nil, nil)

	tests := []struct {
		name         string
		update       version.UpdateInfo
		planResult   *terraform.PlanResult
		wantContains []string
	}{
		{
			name: "basic update without breaking changes",
			update: version.UpdateInfo{
				Module: scanner.ModuleInfo{
					Name:     "vpc",
					Source:   "terraform-aws-modules/vpc/aws",
					FilePath: "main.tf",
					Line:     10,
				},
				CurrentVersion: "4.0.0",
				LatestVersion:  "4.1.0",
				UpdateType:     version.UpdateTypeMinor,
				ChangelogURL:   "https://example.com/changelog",
			},
			planResult: nil,
			wantContains: []string{
				"Terraform Module Update",
				"vpc",
				"4.0.0",
				"4.1.0",
				"Minor Update 🟡",
				"main.tf:10",
			},
		},
		{
			name: "breaking change update",
			update: version.UpdateInfo{
				Module: scanner.ModuleInfo{
					Name:   "vpc",
					Source: "terraform-aws-modules/vpc/aws",
				},
				CurrentVersion:        "4.0.0",
				LatestVersion:         "5.0.0",
				HasBreakingChange:     true,
				BreakingChangeDetails: "Major version change",
				UpdateType:            version.UpdateTypeMajor,
			},
			planResult: nil,
			wantContains: []string{
				"BREAKING CHANGE WARNING",
				"Major version change",
				"Review Checklist for Breaking Changes",
			},
		},
		{
			name: "with plan results",
			update: version.UpdateInfo{
				Module: scanner.ModuleInfo{
					Name: "vpc",
				},
				CurrentVersion: "4.0.0",
				LatestVersion:  "4.1.0",
			},
			planResult: &terraform.PlanResult{
				Success:    true,
				HasChanges: true,
				Output:     "Plan: 5 to add, 0 to change, 0 to destroy",
			},
			wantContains: []string{
				"Terraform Plan Results",
				"Plan succeeded",
				"This update will make infrastructure changes",
			},
		},
		{
			name: "with resource changes",
			update: version.UpdateInfo{
				Module: scanner.ModuleInfo{
					Name: "vpc",
				},
				CurrentVersion: "4.0.0",
				LatestVersion:  "5.0.0",
				ResourceChanges: &version.ResourceChangesSummary{
					HasChanges:   true,
					TotalReplace: 2,
					TotalDelete:  1,
					ResourcesToReplace: []version.ResourceChange{
						{
							Address:      "aws_vpc.main",
							ResourceType: "aws_vpc",
							Reason:       "cidr_block change forces replacement",
						},
					},
				},
			},
			planResult: nil,
			wantContains: []string{
				"Resource Changes Detected",
				"Resources to be REPLACED (2)",
				"aws_vpc.main",
				"cidr_block change forces replacement",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := creator.generatePRBody(tt.update, tt.planResult)

			for _, want := range tt.wantContains {
				if !strings.Contains(body, want) {
					t.Errorf("generatePRBody() does not contain %q\nBody:\n%s", want, body)
				}
			}
		})
	}
}

func TestGenerateProviderPRBody(t *testing.T) {
	creator, _ := NewPRCreator("ghp_test", "testorg", "testrepo", "main", "/project", nil, nil)

	tests := []struct {
		name         string
		update       version.ProviderUpdateInfo
		wantContains []string
	}{
		{
			name: "basic provider update",
			update: version.ProviderUpdateInfo{
				Provider: scanner.ProviderInfo{
					Name:     "aws",
					Source:   "hashicorp/aws",
					FilePath: "versions.tf",
					Line:     5,
				},
				CurrentVersion: "4.0.0",
				LatestVersion:  "5.0.0",
				UpdateType:     version.UpdateTypeMajor,
				ChangelogURL:   "https://registry.terraform.io/providers/hashicorp/aws/5.0.0",
			},
			wantContains: []string{
				"Provider Update",
				"aws",
				"hashicorp/aws",
				"4.0.0",
				"5.0.0",
				"versions.tf:5",
				"Review Checklist",
			},
		},
		{
			name: "breaking change provider update",
			update: version.ProviderUpdateInfo{
				Provider: scanner.ProviderInfo{
					Name:   "aws",
					Source: "hashicorp/aws",
				},
				CurrentVersion:        "4.0.0",
				LatestVersion:         "5.0.0",
				HasBreakingChange:     true,
				BreakingChangeDetails: "Major version with breaking changes",
				UpdateType:            version.UpdateTypeMajor,
			},
			wantContains: []string{
				"Breaking Change Warning",
				"Major version with breaking changes",
				"Review provider upgrade guide",
			},
		},
		{
			name: "minor provider update",
			update: version.ProviderUpdateInfo{
				Provider: scanner.ProviderInfo{
					Name:   "kubernetes",
					Source: "hashicorp/kubernetes",
				},
				CurrentVersion: "2.20.0",
				LatestVersion:  "2.21.0",
				UpdateType:     version.UpdateTypeMinor,
			},
			wantContains: []string{
				"kubernetes",
				"2.20.0",
				"2.21.0",
				"🟡 Minor",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := creator.generateProviderPRBody(tt.update)

			for _, want := range tt.wantContains {
				if !strings.Contains(body, want) {
					t.Errorf("generateProviderPRBody() does not contain %q\nBody:\n%s", want, body)
				}
			}
		})
	}
}

func TestReplaceProviderVersionInLine(t *testing.T) {
	creator, _ := NewPRCreator("ghp_test", "testorg", "testrepo", "main", "/project", nil, nil)

	tests := []struct {
		name           string
		line           string
		currentVersion string
		latestVersion  string
		want           string
	}{
		{
			name:           "simple version",
			line:           `      version = "4.0.0"`,
			currentVersion: "4.0.0",
			latestVersion:  "5.0.0",
			want:           `      version = "5.0.0"`,
		},
		{
			name:           "pessimistic constraint",
			line:           `      version = "~> 4.0"`,
			currentVersion: "4.0",
			latestVersion:  "5.0",
			want:           `      version = "~> 5.0"`,
		},
		{
			name:           "greater than or equal",
			line:           `      version = ">= 4.0.0"`,
			currentVersion: "4.0.0",
			latestVersion:  "5.0.0",
			want:           `      version = ">= 5.0.0"`,
		},
		{
			name:           "exact version with equals",
			line:           `      version = "= 4.0.0"`,
			currentVersion: "4.0.0",
			latestVersion:  "5.0.0",
			want:           `      version = "= 5.0.0"`,
		},
		{
			name:           "less than or equal",
			line:           `      version = "<= 4.0.0"`,
			currentVersion: "4.0.0",
			latestVersion:  "5.0.0",
			want:           `      version = "<= 5.0.0"`,
		},
		{
			name:           "greater than",
			line:           `      version = "> 3.0.0"`,
			currentVersion: "3.0.0",
			latestVersion:  "4.0.0",
			want:           `      version = "> 4.0.0"`,
		},
		{
			name:           "less than",
			line:           `      version = "< 4.0.0"`,
			currentVersion: "4.0.0",
			latestVersion:  "5.0.0",
			want:           `      version = "< 5.0.0"`,
		},
		{
			name:           "no quotes",
			line:           `      version = hello`,
			currentVersion: "4.0.0",
			latestVersion:  "5.0.0",
			want:           `      version = hello`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := creator.replaceProviderVersionInLine(tt.line, tt.currentVersion, tt.latestVersion)
			if got != tt.want {
				t.Errorf("replaceProviderVersionInLine() = %q, want %q", got, tt.want)
			}
		})
	}
}
