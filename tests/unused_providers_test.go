package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/heyjobs/terranovate/internal/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectUnusedProviders_AllUsed(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "terranovate-test-unused-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create providers.tf file
	providersFile := filepath.Join(tmpDir, "providers.tf")
	providersContent := `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    google = {
      source  = "hashicorp/google"
      version = ">= 4.0.0"
    }
  }
}
`
	err = os.WriteFile(providersFile, []byte(providersContent), 0644)
	require.NoError(t, err)

	// Create main.tf file with resources using both providers
	mainFile := filepath.Join(tmpDir, "main.tf")
	mainContent := `
resource "aws_instance" "web" {
  ami           = "ami-0c55b159cbfafe1f0"
  instance_type = "t2.micro"
}

resource "google_compute_instance" "default" {
  name         = "test"
  machine_type = "f1-micro"
  zone         = "us-central1-a"
}
`
	err = os.WriteFile(mainFile, []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create scanner
	s := scanner.New(tmpDir, []string{}, []string{"*.tf"}, false)

	// Scan for providers
	providers, err := s.ScanProviders()
	require.NoError(t, err)
	require.Len(t, providers, 2)

	// Scan for resources
	resources, err := s.ScanResources()
	require.NoError(t, err)
	require.Len(t, resources, 2)

	// Detect unused providers
	unusedProviders := scanner.DetectUnusedProviders(providers, resources, []scanner.ModuleInfo{}, []string{})

	// All providers should be used
	assert.Len(t, unusedProviders, 0, "should find no unused providers")
}

func TestDetectUnusedProviders_SomeUnused(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "terranovate-test-unused-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create providers.tf file with 3 providers
	providersFile := filepath.Join(tmpDir, "providers.tf")
	providersContent := `
terraform {
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
	err = os.WriteFile(providersFile, []byte(providersContent), 0644)
	require.NoError(t, err)

	// Create main.tf file with resources using only AWS
	mainFile := filepath.Join(tmpDir, "main.tf")
	mainContent := `
resource "aws_instance" "web" {
  ami           = "ami-0c55b159cbfafe1f0"
  instance_type = "t2.micro"
}

resource "aws_s3_bucket" "data" {
  bucket = "my-bucket"
}
`
	err = os.WriteFile(mainFile, []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create scanner
	s := scanner.New(tmpDir, []string{}, []string{"*.tf"}, false)

	// Scan for providers
	providers, err := s.ScanProviders()
	require.NoError(t, err)
	require.Len(t, providers, 3)

	// Scan for resources
	resources, err := s.ScanResources()
	require.NoError(t, err)
	require.Len(t, resources, 2)

	// Detect unused providers
	unusedProviders := scanner.DetectUnusedProviders(providers, resources, []scanner.ModuleInfo{}, []string{})

	// Should find 2 unused providers (google and azurerm)
	assert.Len(t, unusedProviders, 2, "should find 2 unused providers")

	unusedNames := make(map[string]bool)
	for _, up := range unusedProviders {
		unusedNames[up.Provider.Name] = true
	}

	assert.True(t, unusedNames["google"], "google should be unused")
	assert.True(t, unusedNames["azurerm"], "azurerm should be unused")
	assert.False(t, unusedNames["aws"], "aws should not be in unused list")
}

func TestDetectUnusedProviders_AllUnused(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "terranovate-test-unused-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create providers.tf file
	providersFile := filepath.Join(tmpDir, "providers.tf")
	providersContent := `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    google = {
      source  = "hashicorp/google"
      version = ">= 4.0.0"
    }
  }
}
`
	err = os.WriteFile(providersFile, []byte(providersContent), 0644)
	require.NoError(t, err)

	// Create main.tf file with NO resources
	mainFile := filepath.Join(tmpDir, "main.tf")
	mainContent := `
# No resources defined
`
	err = os.WriteFile(mainFile, []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create scanner
	s := scanner.New(tmpDir, []string{}, []string{"*.tf"}, false)

	// Scan for providers
	providers, err := s.ScanProviders()
	require.NoError(t, err)
	require.Len(t, providers, 2)

	// Scan for resources
	resources, err := s.ScanResources()
	require.NoError(t, err)
	require.Len(t, resources, 0)

	// Detect unused providers
	unusedProviders := scanner.DetectUnusedProviders(providers, resources, []scanner.ModuleInfo{}, []string{})

	// All providers should be unused
	assert.Len(t, unusedProviders, 2, "should find 2 unused providers")
}

func TestDetectUnusedProviders_WithIgnoreList(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "terranovate-test-unused-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create providers.tf file with 3 providers
	providersFile := filepath.Join(tmpDir, "providers.tf")
	providersContent := `
terraform {
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
	err = os.WriteFile(providersFile, []byte(providersContent), 0644)
	require.NoError(t, err)

	// Create main.tf file with NO resources
	mainFile := filepath.Join(tmpDir, "main.tf")
	mainContent := `
# No resources
`
	err = os.WriteFile(mainFile, []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create scanner
	s := scanner.New(tmpDir, []string{}, []string{"*.tf"}, false)

	// Scan for providers
	providers, err := s.ScanProviders()
	require.NoError(t, err)
	require.Len(t, providers, 3)

	// Scan for resources
	resources, err := s.ScanResources()
	require.NoError(t, err)

	// Detect unused providers with ignore list
	ignoreList := []string{"aws", "google"}
	unusedProviders := scanner.DetectUnusedProviders(providers, resources, []scanner.ModuleInfo{}, ignoreList)

	// Should only find azurerm as unused (aws and google are ignored)
	assert.Len(t, unusedProviders, 1, "should find 1 unused provider")
	assert.Equal(t, "azurerm", unusedProviders[0].Provider.Name)
}

func TestDetectUnusedProviders_DataSources(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "terranovate-test-unused-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create providers.tf file
	providersFile := filepath.Join(tmpDir, "providers.tf")
	providersContent := `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}
`
	err = os.WriteFile(providersFile, []byte(providersContent), 0644)
	require.NoError(t, err)

	// Create main.tf file with only data sources (no resources)
	mainFile := filepath.Join(tmpDir, "main.tf")
	mainContent := `
data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"]
}

data "aws_vpc" "default" {
  default = true
}
`
	err = os.WriteFile(mainFile, []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create scanner
	s := scanner.New(tmpDir, []string{}, []string{"*.tf"}, false)

	// Scan for providers
	providers, err := s.ScanProviders()
	require.NoError(t, err)
	require.Len(t, providers, 1)

	// Scan for resources (includes data sources)
	resources, err := s.ScanResources()
	require.NoError(t, err)
	require.Len(t, resources, 2)

	// Detect unused providers
	unusedProviders := scanner.DetectUnusedProviders(providers, resources, []scanner.ModuleInfo{}, []string{})

	// Provider should be marked as used (data sources count as usage)
	assert.Len(t, unusedProviders, 0, "should find no unused providers - data sources count as usage")
}

func TestDetectUnusedProviders_UtilityProviders(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "terranovate-test-unused-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create providers.tf file with utility providers
	providersFile := filepath.Join(tmpDir, "providers.tf")
	providersContent := `
terraform {
  required_providers {
    null = {
      source  = "hashicorp/null"
      version = "~> 3.0"
    }
    random = {
      source  = "hashicorp/random"
      version = ">= 3.0.0"
    }
  }
}
`
	err = os.WriteFile(providersFile, []byte(providersContent), 0644)
	require.NoError(t, err)

	// Create main.tf file with NO resources
	mainFile := filepath.Join(tmpDir, "main.tf")
	mainContent := `
# No resources
`
	err = os.WriteFile(mainFile, []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create scanner
	s := scanner.New(tmpDir, []string{}, []string{"*.tf"}, false)

	// Scan for providers
	providers, err := s.ScanProviders()
	require.NoError(t, err)
	require.Len(t, providers, 2)

	// Scan for resources
	resources, err := s.ScanResources()
	require.NoError(t, err)

	// Detect unused providers
	unusedProviders := scanner.DetectUnusedProviders(providers, resources, []scanner.ModuleInfo{}, []string{})

	// Should find both as unused, but with utility provider suggestion
	assert.Len(t, unusedProviders, 2, "should find 2 unused providers")

	for _, up := range unusedProviders {
		// Check that utility providers have special suggestion
		assert.Contains(t, up.Suggestion, "utility provider", "utility providers should have special suggestion")
	}
}

func TestScanResources(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "terranovate-test-resources-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create main.tf file with various resources
	mainFile := filepath.Join(tmpDir, "main.tf")
	mainContent := `
resource "aws_instance" "web" {
  ami           = "ami-0c55b159cbfafe1f0"
  instance_type = "t2.micro"
}

resource "aws_s3_bucket" "data" {
  bucket = "my-bucket"
}

data "aws_ami" "ubuntu" {
  most_recent = true
}

resource "google_compute_instance" "default" {
  name         = "test"
  machine_type = "f1-micro"
}
`
	err = os.WriteFile(mainFile, []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create scanner
	s := scanner.New(tmpDir, []string{}, []string{"*.tf"}, false)

	// Scan for resources
	resources, err := s.ScanResources()
	require.NoError(t, err)

	// Should find 4 resources (3 resources + 1 data source)
	assert.Len(t, resources, 4, "should find 4 resources")

	// Count resources by type
	resourceCount := 0
	dataSourceCount := 0
	for _, r := range resources {
		if r.IsDataSource {
			dataSourceCount++
		} else {
			resourceCount++
		}
	}

	assert.Equal(t, 3, resourceCount, "should have 3 resources")
	assert.Equal(t, 1, dataSourceCount, "should have 1 data source")

	// Verify resource types
	resourceTypes := make(map[string]bool)
	for _, r := range resources {
		resourceTypes[r.Type] = true
	}

	assert.True(t, resourceTypes["aws_instance"], "should find aws_instance")
	assert.True(t, resourceTypes["aws_s3_bucket"], "should find aws_s3_bucket")
	assert.True(t, resourceTypes["aws_ami"], "should find aws_ami data source")
	assert.True(t, resourceTypes["google_compute_instance"], "should find google_compute_instance")
}

func TestDetectUnusedProviders_WithModules(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "terranovate-test-unused-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create providers.tf file with AWS and Google
	providersFile := filepath.Join(tmpDir, "providers.tf")
	providersContent := `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    google = {
      source  = "hashicorp/google"
      version = ">= 4.0.0"
    }
  }
}
`
	err = os.WriteFile(providersFile, []byte(providersContent), 0644)
	require.NoError(t, err)

	// Create main.tf file with AWS modules but NO resources
	mainFile := filepath.Join(tmpDir, "main.tf")
	mainContent := `
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.0.0"
}

module "eks" {
  source = "git::https://github.com/terraform-aws-modules/terraform-aws-eks.git?ref=v19.0.0"
}
`
	err = os.WriteFile(mainFile, []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create scanner
	s := scanner.New(tmpDir, []string{}, []string{"*.tf"}, false)

	// Scan for providers
	providers, err := s.ScanProviders()
	require.NoError(t, err)
	require.Len(t, providers, 2)

	// Scan for resources (should be empty)
	resources, err := s.ScanResources()
	require.NoError(t, err)
	require.Len(t, resources, 0)

	// Scan for modules
	modules, err := s.Scan()
	require.NoError(t, err)
	require.Len(t, modules, 2)

	// Detect unused providers
	unusedProviders := scanner.DetectUnusedProviders(providers, resources, modules, []string{})

	// AWS should be inferred as used from modules, only Google should be unused
	assert.Len(t, unusedProviders, 1, "should find 1 unused provider")
	assert.Equal(t, "google", unusedProviders[0].Provider.Name, "google should be unused")
}

func TestGetUsedProviderNames(t *testing.T) {
	resources := []scanner.ResourceInfo{
		{Type: "aws_instance", Name: "web"},
		{Type: "aws_s3_bucket", Name: "data"},
		{Type: "google_compute_instance", Name: "default"},
		{Type: "google_compute_network", Name: "vpc"},
		{Type: "azurerm_resource_group", Name: "rg"},
	}

	providerNames := scanner.GetUsedProviderNames(resources)

	// Should extract unique provider names
	assert.Len(t, providerNames, 3, "should find 3 unique providers")

	nameMap := make(map[string]bool)
	for _, name := range providerNames {
		nameMap[name] = true
	}

	assert.True(t, nameMap["aws"], "should find aws provider")
	assert.True(t, nameMap["google"], "should find google provider")
	assert.True(t, nameMap["azurerm"], "should find azurerm provider")
}
