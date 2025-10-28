package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/heyjobs/terranovate/internal/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanner_Scan(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "terranovate-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test Terraform file
	testTF := `
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.0.0"

  name = "my-vpc"
  cidr = "10.0.0.0/16"
}

module "security_group" {
  source = "terraform-aws-modules/security-group/aws"
  version = "4.9.0"
}

module "eks" {
  source = "git::https://github.com/terraform-aws-modules/terraform-aws-eks.git?ref=v19.0.0"
}

module "local" {
  source = "./modules/local"
}
`

	tfFile := filepath.Join(tmpDir, "main.tf")
	err = os.WriteFile(tfFile, []byte(testTF), 0644)
	require.NoError(t, err)

	// Create scanner
	s := scanner.New(tmpDir, []string{}, []string{"*.tf"}, true)

	// Scan
	modules, err := s.Scan()
	require.NoError(t, err)

	// Assertions
	assert.Len(t, modules, 4, "should find 4 modules")

	// Check VPC module
	vpcModule := findModule(modules, "vpc")
	require.NotNil(t, vpcModule, "vpc module should be found")
	assert.Equal(t, "terraform-aws-modules/vpc/aws", vpcModule.Source)
	assert.Equal(t, "5.0.0", vpcModule.Version)
	assert.Equal(t, scanner.SourceTypeRegistry, vpcModule.SourceType)

	// Check security group module
	sgModule := findModule(modules, "security_group")
	require.NotNil(t, sgModule, "security_group module should be found")
	assert.Equal(t, "terraform-aws-modules/security-group/aws", sgModule.Source)
	assert.Equal(t, "4.9.0", sgModule.Version)

	// Check git module
	eksModule := findModule(modules, "eks")
	require.NotNil(t, eksModule, "eks module should be found")
	assert.Equal(t, scanner.SourceTypeGit, eksModule.SourceType)

	// Check local module
	localModule := findModule(modules, "local")
	require.NotNil(t, localModule, "local module should be found")
	assert.Equal(t, scanner.SourceTypeLocal, localModule.SourceType)
}

func TestScanner_ExcludePatterns(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "terranovate-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create subdirectories
	err = os.Mkdir(filepath.Join(tmpDir, "modules"), 0755)
	require.NoError(t, err)
	err = os.Mkdir(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	// Create test files
	mainTF := `module "test" { source = "test/test" }`
	err = os.WriteFile(filepath.Join(tmpDir, "main.tf"), []byte(mainTF), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "modules", "mod.tf"), []byte(mainTF), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, ".terraform", "test.tf"), []byte(mainTF), 0644)
	require.NoError(t, err)

	// Create scanner with exclusions
	s := scanner.New(tmpDir, []string{".terraform"}, []string{"*.tf"}, true)

	// Scan
	modules, err := s.Scan()
	require.NoError(t, err)

	// Should not include files from .terraform directory
	assert.Len(t, modules, 2, "should find 2 modules (excluding .terraform)")
}

func TestScanner_SourceTypeDetection(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		expectType scanner.SourceType
	}{
		{
			name:       "registry module",
			source:     "terraform-aws-modules/vpc/aws",
			expectType: scanner.SourceTypeRegistry,
		},
		{
			name:       "git https",
			source:     "git::https://github.com/owner/repo.git",
			expectType: scanner.SourceTypeGit,
		},
		{
			name:       "git ssh",
			source:     "git@github.com:owner/repo.git",
			expectType: scanner.SourceTypeGit,
		},
		{
			name:       "github shorthand",
			source:     "github.com/owner/repo",
			expectType: scanner.SourceTypeGit,
		},
		{
			name:       "local relative",
			source:     "./modules/local",
			expectType: scanner.SourceTypeLocal,
		},
		{
			name:       "local parent",
			source:     "../modules/shared",
			expectType: scanner.SourceTypeLocal,
		},
		{
			name:       "local absolute",
			source:     "/opt/terraform/modules",
			expectType: scanner.SourceTypeLocal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := scanner.New(".", []string{}, []string{"*.tf"}, true)
			sourceType := s.DetermineSourceType(tt.source)
			assert.Equal(t, tt.expectType, sourceType)
		})
	}
}

// Helper function to find a module by name
func findModule(modules []scanner.ModuleInfo, name string) *scanner.ModuleInfo {
	for _, m := range modules {
		if m.Name == name {
			return &m
		}
	}
	return nil
}
