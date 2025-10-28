package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	basePath := "/test/path"
	exclude := []string{".terraform", ".git"}
	include := []string{"*.tf", "*.tfvars"}
	recursive := true

	scanner := New(basePath, exclude, include, recursive)

	if scanner == nil {
		t.Fatal("New() returned nil")
	}

	if scanner.basePath != basePath {
		t.Errorf("basePath = %s, want %s", scanner.basePath, basePath)
	}

	if len(scanner.exclude) != len(exclude) {
		t.Errorf("exclude length = %d, want %d", len(scanner.exclude), len(exclude))
	}

	if len(scanner.include) != len(include) {
		t.Errorf("include length = %d, want %d", len(scanner.include), len(include))
	}

	if scanner.recursive != recursive {
		t.Errorf("recursive = %v, want %v", scanner.recursive, recursive)
	}
}

func TestDetermineSourceType(t *testing.T) {
	scanner := New(".", nil, nil, false)

	tests := []struct {
		name       string
		source     string
		wantType   SourceType
	}{
		{
			name:     "git with prefix",
			source:   "git::https://github.com/terraform-aws-modules/terraform-aws-vpc.git",
			wantType: SourceTypeGit,
		},
		{
			name:     "git ssh",
			source:   "git@github.com:terraform-aws-modules/terraform-aws-vpc.git",
			wantType: SourceTypeGit,
		},
		{
			name:     "github url without git prefix",
			source:   "github.com/terraform-aws-modules/terraform-aws-vpc",
			wantType: SourceTypeGit,
		},
		{
			name:     "registry source",
			source:   "terraform-aws-modules/vpc/aws",
			wantType: SourceTypeRegistry,
		},
		{
			name:     "registry source with namespace",
			source:   "hashicorp/consul/aws",
			wantType: SourceTypeRegistry,
		},
		{
			name:     "local relative path",
			source:   "./modules/vpc",
			wantType: SourceTypeLocal,
		},
		{
			name:     "local parent path",
			source:   "../shared-modules/vpc",
			wantType: SourceTypeLocal,
		},
		{
			name:     "local absolute path",
			source:   "/opt/terraform/modules/vpc",
			wantType: SourceTypeLocal,
		},
		{
			name:     "unknown source",
			source:   "invalid-source",
			wantType: SourceTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scanner.DetermineSourceType(tt.source)
			if got != tt.wantType {
				t.Errorf("DetermineSourceType(%s) = %s, want %s", tt.source, got, tt.wantType)
			}
		})
	}
}

func TestShouldExclude(t *testing.T) {
	scanner := New(".", []string{".terraform", ".git", ".terragrunt-cache"}, nil, false)

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "exclude .terraform",
			path: "/project/.terraform",
			want: true,
		},
		{
			name: "exclude .git",
			path: "/project/.git",
			want: true,
		},
		{
			name: "exclude terragrunt cache",
			path: "/project/env/.terragrunt-cache",
			want: true,
		},
		{
			name: "do not exclude regular directory",
			path: "/project/modules",
			want: false,
		},
		{
			name: "pattern match on basename",
			path: "/some/path/.terraform",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scanner.shouldExclude(tt.path)
			if got != tt.want {
				t.Errorf("shouldExclude(%s) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestShouldInclude(t *testing.T) {
	scanner := New(".", nil, []string{"*.tf", "*.tfvars"}, false)

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "include .tf file",
			path: "/project/main.tf",
			want: true,
		},
		{
			name: "include .tfvars file",
			path: "/project/terraform.tfvars",
			want: true,
		},
		{
			name: "exclude .txt file",
			path: "/project/README.txt",
			want: false,
		},
		{
			name: "exclude directory",
			path: "/project/modules",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scanner.shouldInclude(tt.path)
			if got != tt.want {
				t.Errorf("shouldInclude(%s) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestScan(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	// Create test files
	mainTf := `
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.0"
}

module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = ">= 19.0.0"
}

module "local_module" {
  source = "./modules/custom"
}

module "git_module" {
  source = "git::https://github.com/example/terraform-module.git?ref=v1.0.0"
}
`

	if err := os.WriteFile(filepath.Join(tmpDir, "main.tf"), []byte(mainTf), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create subdirectory with another module
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	subTf := `
module "s3" {
  source  = "terraform-aws-modules/s3-bucket/aws"
  version = "~> 3.0"
}
`

	if err := os.WriteFile(filepath.Join(subDir, "s3.tf"), []byte(subTf), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	tests := []struct {
		name          string
		recursive     bool
		wantModules   int
		checkModules  func(*testing.T, []ModuleInfo)
	}{
		{
			name:        "recursive scan",
			recursive:   true,
			wantModules: 5, // vpc, eks, local_module, git_module, s3
			checkModules: func(t *testing.T, modules []ModuleInfo) {
				// Verify we found modules from both files
				foundMain := false
				foundSub := false
				for _, m := range modules {
					if m.Name == "vpc" {
						foundMain = true
						if m.Source != "terraform-aws-modules/vpc/aws" {
							t.Errorf("vpc source = %s, want terraform-aws-modules/vpc/aws", m.Source)
						}
						if m.Version != "~> 5.0" {
							t.Errorf("vpc version = %s, want ~> 5.0", m.Version)
						}
						if m.SourceType != SourceTypeRegistry {
							t.Errorf("vpc source type = %s, want %s", m.SourceType, SourceTypeRegistry)
						}
					}
					if m.Name == "s3" {
						foundSub = true
					}
					if m.Name == "local_module" {
						if m.SourceType != SourceTypeLocal {
							t.Errorf("local_module source type = %s, want %s", m.SourceType, SourceTypeLocal)
						}
					}
					if m.Name == "git_module" {
						if m.SourceType != SourceTypeGit {
							t.Errorf("git_module source type = %s, want %s", m.SourceType, SourceTypeGit)
						}
					}
				}
				if !foundMain {
					t.Error("Did not find module from main.tf")
				}
				if !foundSub {
					t.Error("Did not find module from subdirectory")
				}
			},
		},
		{
			name:        "non-recursive scan",
			recursive:   false,
			wantModules: 4, // vpc, eks, local_module, git_module (no s3 from subdir)
			checkModules: func(t *testing.T, modules []ModuleInfo) {
				// Verify we only found modules from main.tf
				for _, m := range modules {
					if m.Name == "s3" {
						t.Error("Found module from subdirectory in non-recursive scan")
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := New(tmpDir, nil, []string{"*.tf"}, tt.recursive)
			modules, err := scanner.Scan()
			if err != nil {
				t.Fatalf("Scan() error = %v", err)
			}

			if len(modules) != tt.wantModules {
				t.Errorf("Scan() found %d modules, want %d", len(modules), tt.wantModules)
				for i, m := range modules {
					t.Logf("Module %d: %s (source: %s)", i, m.Name, m.Source)
				}
			}

			if tt.checkModules != nil {
				tt.checkModules(t, modules)
			}
		})
	}
}

func TestScanProviders(t *testing.T) {
	tmpDir := t.TempDir()

	versionsTf := `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = ">= 2.0"
    }
    random = "hashicorp/random"
  }
}
`

	if err := os.WriteFile(filepath.Join(tmpDir, "versions.tf"), []byte(versionsTf), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	scanner := New(tmpDir, nil, []string{"*.tf"}, true)
	providers, err := scanner.ScanProviders()
	if err != nil {
		t.Fatalf("ScanProviders() error = %v", err)
	}

	if len(providers) != 3 {
		t.Fatalf("ScanProviders() found %d providers, want 3", len(providers))
	}

	// Check each provider
	providerMap := make(map[string]ProviderInfo)
	for _, p := range providers {
		providerMap[p.Name] = p
	}

	// Check AWS provider
	if aws, ok := providerMap["aws"]; ok {
		if aws.Source != "hashicorp/aws" {
			t.Errorf("aws.Source = %s, want hashicorp/aws", aws.Source)
		}
		if aws.Version != "~> 5.0" {
			t.Errorf("aws.Version = %s, want ~> 5.0", aws.Version)
		}
	} else {
		t.Error("AWS provider not found")
	}

	// Check Kubernetes provider
	if k8s, ok := providerMap["kubernetes"]; ok {
		if k8s.Source != "hashicorp/kubernetes" {
			t.Errorf("kubernetes.Source = %s, want hashicorp/kubernetes", k8s.Source)
		}
		if k8s.Version != ">= 2.0" {
			t.Errorf("kubernetes.Version = %s, want >= 2.0", k8s.Version)
		}
	} else {
		t.Error("Kubernetes provider not found")
	}

	// Check Random provider (string format)
	if random, ok := providerMap["random"]; ok {
		if random.Source != "hashicorp/random" {
			t.Errorf("random.Source = %s, want hashicorp/random", random.Source)
		}
		if random.Version != "" {
			t.Errorf("random.Version = %s, want empty", random.Version)
		}
	} else {
		t.Error("Random provider not found")
	}
}

func TestScan_WithExcludes(t *testing.T) {
	tmpDir := t.TempDir()

	// Create main file
	mainContent := `
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.0"
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.tf"), []byte(mainContent), 0644); err != nil {
		t.Fatalf("Failed to write main.tf: %v", err)
	}

	// Create .terraform directory with a file
	terraformDir := filepath.Join(tmpDir, ".terraform")
	if err := os.MkdirAll(terraformDir, 0755); err != nil {
		t.Fatalf("Failed to create .terraform dir: %v", err)
	}

	terraformContent := `
module "should_not_find" {
  source = "should/not/find"
}
`
	if err := os.WriteFile(filepath.Join(terraformDir, "modules.tf"), []byte(terraformContent), 0644); err != nil {
		t.Fatalf("Failed to write .terraform/modules.tf: %v", err)
	}

	scanner := New(tmpDir, []string{".terraform"}, []string{"*.tf"}, true)
	modules, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(modules) != 1 {
		t.Errorf("Scan() found %d modules, want 1", len(modules))
	}

	// Verify we only found the vpc module
	if len(modules) > 0 && modules[0].Name != "vpc" {
		t.Errorf("Found module %s, want vpc", modules[0].Name)
	}

	// Verify we didn't find the excluded module
	for _, m := range modules {
		if m.Name == "should_not_find" {
			t.Error("Found module from excluded directory")
		}
	}
}

func TestParseFile_InvalidHCL(t *testing.T) {
	tmpDir := t.TempDir()

	invalidContent := `
module "broken" {
  source = "test"
  invalid syntax here
}
`

	invalidFile := filepath.Join(tmpDir, "invalid.tf")
	if err := os.WriteFile(invalidFile, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to write invalid file: %v", err)
	}

	scanner := New(tmpDir, nil, []string{"*.tf"}, false)
	modules, err := scanner.Scan()

	// Scan should not return error, but should skip invalid files
	if err != nil {
		t.Errorf("Scan() error = %v, want nil (should skip invalid files)", err)
	}

	// Should not find any modules
	if len(modules) > 0 {
		t.Errorf("Scan() found %d modules, want 0 (invalid file should be skipped)", len(modules))
	}
}

func TestScan_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	scanner := New(tmpDir, nil, []string{"*.tf"}, true)
	modules, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(modules) != 0 {
		t.Errorf("Scan() found %d modules in empty directory, want 0", len(modules))
	}
}

func TestScan_NonExistentDirectory(t *testing.T) {
	scanner := New("/nonexistent/path", nil, []string{"*.tf"}, true)
	_, err := scanner.Scan()
	if err == nil {
		t.Error("Scan() expected error for non-existent directory, got nil")
	}
}
