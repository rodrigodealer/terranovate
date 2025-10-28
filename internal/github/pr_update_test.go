package github

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heyjobs/terranovate/internal/scanner"
	"github.com/heyjobs/terranovate/internal/terraform"
	"github.com/heyjobs/terranovate/internal/version"
)

func TestUpdateModuleVersion(t *testing.T) {
	tests := []struct {
		name            string
		fileContent     string
		update          version.UpdateInfo
		wantContains    string
		wantNotContains string
		wantErr         bool
	}{
		{
			name: "update registry module version",
			fileContent: `
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "4.0.0"

  cidr = "10.0.0.0/16"
}
`,
			update: version.UpdateInfo{
				Module: scanner.ModuleInfo{
					Name:       "vpc",
					Source:     "terraform-aws-modules/vpc/aws",
					SourceType: scanner.SourceTypeRegistry,
				},
				CurrentVersion: "4.0.0",
				LatestVersion:  "5.0.0",
			},
			wantContains:    `version = "5.0.0"`,
			wantNotContains: `version = "4.0.0"`,
			wantErr:         false,
		},
		{
			name: "update git module ref",
			fileContent: `
module "custom" {
  source = "git::https://github.com/example/module.git?ref=v1.0.0"
}
`,
			update: version.UpdateInfo{
				Module: scanner.ModuleInfo{
					Name:       "custom",
					Source:     "git::https://github.com/example/module.git?ref=v1.0.0",
					SourceType: "git",
				},
				CurrentVersion: "v1.0.0",
				LatestVersion:  "2.0.0",
			},
			wantContains:    "ref=v2.0.0",
			wantNotContains: "ref=v1.0.0",
			wantErr:         false,
		},
		{
			name: "add version to module without version",
			fileContent: `
module "vpc" {
  source = "terraform-aws-modules/vpc/aws"

  cidr = "10.0.0.0/16"
}
`,
			update: version.UpdateInfo{
				Module: scanner.ModuleInfo{
					Name:       "vpc",
					Source:     "terraform-aws-modules/vpc/aws",
					SourceType: scanner.SourceTypeRegistry,
				},
				CurrentVersion: "",
				LatestVersion:  "5.0.0",
			},
			wantContains: `version = "5.0.0"`,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory and file
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "main.tf")

			if err := os.WriteFile(filePath, []byte(tt.fileContent), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Update the file path in the update info
			tt.update.Module.FilePath = filePath

			// Create PR creator
			creator, _ := NewPRCreator("test-token", "testorg", "testrepo", "main", tmpDir, nil, nil)

			// Update the module version
			err := creator.updateModuleVersion(tt.update)

			if (err != nil) != tt.wantErr {
				t.Errorf("updateModuleVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Read the updated file
				updatedContent, err := os.ReadFile(filePath)
				if err != nil {
					t.Fatalf("Failed to read updated file: %v", err)
				}

				content := string(updatedContent)

				if !strings.Contains(content, tt.wantContains) {
					t.Errorf("Updated file does not contain %q\nContent:\n%s",
						tt.wantContains, content)
				}

				if tt.wantNotContains != "" && strings.Contains(content, tt.wantNotContains) {
					t.Errorf("Updated file should not contain %q\nContent:\n%s",
						tt.wantNotContains, content)
				}
			}
		})
	}
}

func TestUpdateProviderVersion(t *testing.T) {
	tests := []struct {
		name            string
		fileContent     string
		update          version.ProviderUpdateInfo
		wantContains    string
		wantNotContains string
		wantErr         bool
	}{
		{
			name: "update provider version with pessimistic constraint",
			fileContent: `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.0"
    }
  }
}
`,
			update: version.ProviderUpdateInfo{
				Provider: scanner.ProviderInfo{
					Name:    "aws",
					Source:  "hashicorp/aws",
					Version: "~> 4.0",
				},
				CurrentVersion: "4.0",
				LatestVersion:  "5.0",
			},
			wantContains:    `version = "~> 5.0"`,
			wantNotContains: `version = "~> 4.0"`,
			wantErr:         false,
		},
		{
			name: "update provider version with exact constraint",
			fileContent: `
terraform {
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "= 2.20.0"
    }
  }
}
`,
			update: version.ProviderUpdateInfo{
				Provider: scanner.ProviderInfo{
					Name:    "kubernetes",
					Source:  "hashicorp/kubernetes",
					Version: "= 2.20.0",
				},
				CurrentVersion: "2.20.0",
				LatestVersion:  "2.21.0",
			},
			wantContains:    `version = "= 2.21.0"`,
			wantNotContains: `version = "= 2.20.0"`,
			wantErr:         false,
		},
		{
			name: "update provider version with >= constraint",
			fileContent: `
terraform {
  required_providers {
    random = {
      source  = "hashicorp/random"
      version = ">= 3.0.0"
    }
  }
}
`,
			update: version.ProviderUpdateInfo{
				Provider: scanner.ProviderInfo{
					Name:    "random",
					Source:  "hashicorp/random",
					Version: ">= 3.0.0",
				},
				CurrentVersion: "3.0.0",
				LatestVersion:  "3.5.0",
			},
			wantContains:    `version = ">= 3.5.0"`,
			wantNotContains: `version = ">= 3.0.0"`,
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory and file
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "versions.tf")

			if err := os.WriteFile(filePath, []byte(tt.fileContent), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Update the file path in the update info
			tt.update.Provider.FilePath = filePath

			// Create PR creator
			creator, _ := NewPRCreator("test-token", "testorg", "testrepo", "main", tmpDir, nil, nil)

			// Update the provider version
			err := creator.updateProviderVersion(tt.update)

			if (err != nil) != tt.wantErr {
				t.Errorf("updateProviderVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Read the updated file
				updatedContent, err := os.ReadFile(filePath)
				if err != nil {
					t.Fatalf("Failed to read updated file: %v", err)
				}

				content := string(updatedContent)

				if !strings.Contains(content, tt.wantContains) {
					t.Errorf("Updated file does not contain %q\nContent:\n%s",
						tt.wantContains, content)
				}

				if tt.wantNotContains != "" && strings.Contains(content, tt.wantNotContains) {
					t.Errorf("Updated file should not contain %q\nContent:\n%s",
						tt.wantNotContains, content)
				}
			}
		})
	}
}

func TestReplaceProviderVersionInLineEdgeCases(t *testing.T) {
	creator, _ := NewPRCreator("test-token", "testorg", "testrepo", "main", ".", nil, nil)

	tests := []struct {
		name    string
		line    string
		current string
		latest  string
		want    string
	}{
		{
			name:    "simple version no constraint",
			line:    `      version = "4.0.0"`,
			current: "4.0.0",
			latest:  "5.0.0",
			want:    `      version = "5.0.0"`,
		},
		{
			name:    "version with ~> constraint",
			line:    `      version = "~> 4.0"`,
			current: "4.0",
			latest:  "5.0",
			want:    `      version = "~> 5.0"`,
		},
		{
			name:    "version with >= constraint",
			line:    `      version = ">= 4.0.0"`,
			current: "4.0.0",
			latest:  "5.0.0",
			want:    `      version = ">= 5.0.0"`,
		},
		{
			name:    "version with = constraint",
			line:    `      version = "= 4.0.0"`,
			current: "4.0.0",
			latest:  "5.0.0",
			want:    `      version = "= 5.0.0"`,
		},
		{
			name:    "version with <= constraint",
			line:    `      version = "<= 4.0.0"`,
			current: "4.0.0",
			latest:  "5.0.0",
			want:    `      version = "<= 5.0.0"`,
		},
		{
			name:    "version with > constraint",
			line:    `      version = "> 3.0.0"`,
			current: "3.0.0",
			latest:  "4.0.0",
			want:    `      version = "> 4.0.0"`,
		},
		{
			name:    "version with < constraint",
			line:    `      version = "< 5.0.0"`,
			current: "5.0.0",
			latest:  "6.0.0",
			want:    `      version = "< 6.0.0"`,
		},
		{
			name:    "no quotes in line",
			line:    `      version = noquotes`,
			current: "4.0.0",
			latest:  "5.0.0",
			want:    `      version = noquotes`,
		},
		{
			name:    "compact spacing",
			line:    `version="4.0.0"`,
			current: "4.0.0",
			latest:  "5.0.0",
			want:    `version="5.0.0"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := creator.replaceProviderVersionInLine(tt.line, tt.current, tt.latest)
			if got != tt.want {
				t.Errorf("replaceProviderVersionInLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGeneratePRBodyWithSchemaChanges(t *testing.T) {
	creator, _ := NewPRCreator("test-token", "testorg", "testrepo", "main", ".", nil, nil)

	update := version.UpdateInfo{
		Module: scanner.ModuleInfo{
			Name:     "vpc",
			Source:   "terraform-aws-modules/vpc/aws",
			FilePath: "main.tf",
			Line:     10,
		},
		CurrentVersion:    "4.0.0",
		LatestVersion:     "5.0.0",
		HasBreakingChange: true,
		UpdateType:        version.UpdateTypeMajor,
		SchemaChanges: &terraform.SchemaChanges{
			HasChanges: true,
			AddedRequiredVars: []terraform.VariableChange{
				{Name: "new_required_var", Type: "string", Required: true, Description: "New required variable"},
			},
			RemovedVars: []terraform.VariableChange{
				{Name: "old_var", Type: "string"},
			},
			ChangedVarTypes: []terraform.VariableChange{
				{Name: "instance_count", Type: "number → string"},
			},
			RemovedOutputs: []terraform.OutputChange{
				{Name: "deprecated_output"},
			},
		},
	}

	body := creator.generatePRBody(update, nil)

	expectedStrings := []string{
		"API/Schema Changes Detected",
		"New Required Variables",
		"new_required_var",
		"Removed Variables",
		"old_var",
		"Changed Variable Types",
		"instance_count",
		"number → string",
		"Removed Outputs",
		"deprecated_output",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(body, expected) {
			t.Errorf("PR body does not contain %q\nBody:\n%s", expected, body)
		}
	}
}

func TestGenerateProviderPRBodyWithBreakingChange(t *testing.T) {
	creator, _ := NewPRCreator("test-token", "testorg", "testrepo", "main", ".", nil, nil)

	update := version.ProviderUpdateInfo{
		Provider: scanner.ProviderInfo{
			Name:     "aws",
			Source:   "hashicorp/aws",
			FilePath: "versions.tf",
			Line:     5,
		},
		CurrentVersion:        "4.0.0",
		LatestVersion:         "5.0.0",
		HasBreakingChange:     true,
		BreakingChangeDetails: "Major version upgrade with breaking changes",
		UpdateType:            version.UpdateTypeMajor,
		ChangelogURL:          "https://registry.terraform.io/providers/hashicorp/aws/5.0.0",
	}

	body := creator.generateProviderPRBody(update)

	expectedStrings := []string{
		"Breaking Change Warning",
		"Major version upgrade with breaking changes",
		"Review provider upgrade guide",
		"Check for deprecated resources",
		"🔴 Major",
		"Review Checklist",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(body, expected) {
			t.Errorf("Provider PR body does not contain %q\nBody:\n%s", expected, body)
		}
	}
}
