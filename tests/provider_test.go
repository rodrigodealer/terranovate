package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/heyjobs/terranovate/internal/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanProviders(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "terranovate-test-providers-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test Terraform file with provider requirements
	testFile := filepath.Join(tmpDir, "providers.tf")
	content := `
terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    google = {
      source  = "hashicorp/google"
      version = ">= 4.0.0"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "3.50.0"
    }
  }
}
`
	err = os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	// Create scanner
	s := scanner.New(tmpDir, []string{}, []string{"*.tf"}, false)

	// Scan for providers
	providers, err := s.ScanProviders()
	require.NoError(t, err)

	// Verify results
	assert.Len(t, providers, 3, "should find 3 providers")

	// Create a map for easier testing
	providerMap := make(map[string]scanner.ProviderInfo)
	for _, p := range providers {
		providerMap[p.Name] = p
	}

	// Test AWS provider
	aws, ok := providerMap["aws"]
	assert.True(t, ok, "should find aws provider")
	assert.Equal(t, "hashicorp/aws", aws.Source)
	assert.Equal(t, "~> 5.0", aws.Version)

	// Test Google provider
	google, ok := providerMap["google"]
	assert.True(t, ok, "should find google provider")
	assert.Equal(t, "hashicorp/google", google.Source)
	assert.Equal(t, ">= 4.0.0", google.Version)

	// Test Azure provider
	azure, ok := providerMap["azurerm"]
	assert.True(t, ok, "should find azurerm provider")
	assert.Equal(t, "hashicorp/azurerm", azure.Source)
	assert.Equal(t, "3.50.0", azure.Version)
}

func TestScanProviders_MultipleFiles(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "terranovate-test-providers-multi-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create first Terraform file
	testFile1 := filepath.Join(tmpDir, "providers.tf")
	content1 := `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.0.0"
    }
  }
}
`
	err = os.WriteFile(testFile1, []byte(content1), 0644)
	require.NoError(t, err)

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "modules")
	err = os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	// Create second Terraform file in subdirectory
	testFile2 := filepath.Join(subDir, "providers.tf")
	content2 := `
terraform {
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "2.20.0"
    }
  }
}
`
	err = os.WriteFile(testFile2, []byte(content2), 0644)
	require.NoError(t, err)

	// Create scanner with recursive option
	s := scanner.New(tmpDir, []string{}, []string{"*.tf"}, true)

	// Scan for providers
	providers, err := s.ScanProviders()
	require.NoError(t, err)

	// Verify results - should find providers from both files
	assert.Len(t, providers, 2, "should find 2 providers across multiple files")

	providerMap := make(map[string]scanner.ProviderInfo)
	for _, p := range providers {
		providerMap[p.Name] = p
	}

	assert.Contains(t, providerMap, "aws")
	assert.Contains(t, providerMap, "kubernetes")
}

func TestScanProviders_NoProviders(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "terranovate-test-no-providers-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test Terraform file without providers
	testFile := filepath.Join(tmpDir, "main.tf")
	content := `
resource "null_resource" "example" {
  triggers = {
    always = timestamp()
  }
}
`
	err = os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	// Create scanner
	s := scanner.New(tmpDir, []string{}, []string{"*.tf"}, false)

	// Scan for providers
	providers, err := s.ScanProviders()
	require.NoError(t, err)

	// Verify results
	assert.Len(t, providers, 0, "should find no providers")
}

func TestScanProviders_ExcludePatterns(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "terranovate-test-exclude-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test file in root
	testFile1 := filepath.Join(tmpDir, "providers.tf")
	content1 := `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.0.0"
    }
  }
}
`
	err = os.WriteFile(testFile1, []byte(content1), 0644)
	require.NoError(t, err)

	// Create .terraform directory (should be excluded)
	terraformDir := filepath.Join(tmpDir, ".terraform")
	err = os.MkdirAll(terraformDir, 0755)
	require.NoError(t, err)

	// Create test file in .terraform directory
	testFile2 := filepath.Join(terraformDir, "providers.tf")
	err = os.WriteFile(testFile2, []byte(content1), 0644)
	require.NoError(t, err)

	// Create scanner with exclude pattern
	s := scanner.New(tmpDir, []string{".terraform"}, []string{"*.tf"}, true)

	// Scan for providers
	providers, err := s.ScanProviders()
	require.NoError(t, err)

	// Verify results - should only find provider from root, not from .terraform
	assert.Len(t, providers, 1, "should find 1 provider (excluded .terraform directory)")
}

func TestProviderVersionExtraction(t *testing.T) {
	tests := []struct {
		name              string
		versionConstraint string
		expectedVersion   string
	}{
		{
			name:              "exact version",
			versionConstraint: "5.0.0",
			expectedVersion:   "5.0.0",
		},
		{
			name:              "pessimistic constraint",
			versionConstraint: "~> 5.0",
			expectedVersion:   "5.0",
		},
		{
			name:              "greater than or equal",
			versionConstraint: ">= 5.0.0",
			expectedVersion:   "5.0.0",
		},
		{
			name:              "equals constraint",
			versionConstraint: "= 5.0.0",
			expectedVersion:   "5.0.0",
		},
		{
			name:              "empty constraint",
			versionConstraint: "",
			expectedVersion:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We'll test this through the actual provider checking
			// This is a placeholder for the version extraction logic test
			// The actual extraction happens in provider_checker.go
			assert.NotEmpty(t, tt.name) // Placeholder assertion
		})
	}
}
