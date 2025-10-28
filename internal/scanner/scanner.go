package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/rs/zerolog/log"
)

// ModuleInfo represents a Terraform module found during scanning
type ModuleInfo struct {
	// Name of the module block
	Name string

	// Source of the module (registry or git URL)
	Source string

	// Version constraint (may be empty for git sources)
	Version string

	// File path where the module was found
	FilePath string

	// Line number in the file
	Line int

	// SourceType indicates if it's a registry or git source
	SourceType SourceType
}

// ProviderInfo represents a Terraform provider found during scanning
type ProviderInfo struct {
	// Name of the provider (e.g., "aws", "google", "azurerm")
	Name string

	// Source of the provider (e.g., "hashicorp/aws")
	Source string

	// Version constraint (e.g., "~> 5.0", ">= 5.0.0")
	Version string

	// File path where the provider was found
	FilePath string

	// Line number in the file
	Line int
}

// ResourceInfo represents a Terraform resource or data source found during scanning
type ResourceInfo struct {
	// Type of the resource (e.g., "aws_instance", "google_compute_instance")
	Type string

	// Name of the resource
	Name string

	// File path where the resource was found
	FilePath string

	// Line number in the file
	Line int

	// IsDataSource indicates if this is a data source (true) or resource (false)
	IsDataSource bool
}

// SourceType indicates the type of module source
type SourceType string

const (
	// SourceTypeRegistry indicates a Terraform Registry module
	SourceTypeRegistry SourceType = "registry"

	// SourceTypeGit indicates a Git repository source
	SourceTypeGit SourceType = "git"

	// SourceTypeLocal indicates a local path source
	SourceTypeLocal SourceType = "local"

	// SourceTypeUnknown indicates an unknown source type
	SourceTypeUnknown SourceType = "unknown"
)

// Scanner scans Terraform files for module usage
type Scanner struct {
	basePath  string
	exclude   []string
	include   []string
	recursive bool
}

// New creates a new Scanner instance
func New(basePath string, exclude []string, include []string, recursive bool) *Scanner {
	return &Scanner{
		basePath:  basePath,
		exclude:   exclude,
		include:   include,
		recursive: recursive,
	}
}

// Scan scans the configured path for Terraform modules
func (s *Scanner) Scan() ([]ModuleInfo, error) {
	var modules []ModuleInfo

	err := filepath.Walk(s.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories if not recursive
		if info.IsDir() {
			if !s.recursive && path != s.basePath {
				return filepath.SkipDir
			}
			// Check if directory should be excluded
			if s.shouldExclude(path) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file matches include patterns
		if !s.shouldInclude(path) {
			return nil
		}

		log.Debug().Str("file", path).Msg("scanning file")

		fileModules, err := s.parseFile(path)
		if err != nil {
			log.Warn().Err(err).Str("file", path).Msg("failed to parse file")
			return nil // Continue with other files
		}

		modules = append(modules, fileModules...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	return modules, nil
}

// ScanProviders scans the configured path for Terraform provider requirements
func (s *Scanner) ScanProviders() ([]ProviderInfo, error) {
	var providers []ProviderInfo

	err := filepath.Walk(s.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories if not recursive
		if info.IsDir() {
			if !s.recursive && path != s.basePath {
				return filepath.SkipDir
			}
			// Check if directory should be excluded
			if s.shouldExclude(path) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file matches include patterns
		if !s.shouldInclude(path) {
			return nil
		}

		log.Debug().Str("file", path).Msg("scanning file for providers")

		fileProviders, err := s.parseProviders(path)
		if err != nil {
			log.Warn().Err(err).Str("file", path).Msg("failed to parse file for providers")
			return nil // Continue with other files
		}

		providers = append(providers, fileProviders...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	return providers, nil
}

// ScanResources scans the configured path for Terraform resources and data sources
func (s *Scanner) ScanResources() ([]ResourceInfo, error) {
	var resources []ResourceInfo

	err := filepath.Walk(s.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories if not recursive
		if info.IsDir() {
			if !s.recursive && path != s.basePath {
				return filepath.SkipDir
			}
			// Check if directory should be excluded
			if s.shouldExclude(path) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file matches include patterns
		if !s.shouldInclude(path) {
			return nil
		}

		log.Debug().Str("file", path).Msg("scanning file for resources")

		fileResources, err := s.parseResources(path)
		if err != nil {
			log.Warn().Err(err).Str("file", path).Msg("failed to parse file for resources")
			return nil // Continue with other files
		}

		resources = append(resources, fileResources...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	return resources, nil
}

// parseFile parses a single Terraform file for module blocks
func (s *Scanner) parseFile(path string) ([]ModuleInfo, error) {
	parser := hclparse.NewParser()

	file, diags := parser.ParseHCLFile(path)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse errors: %s", diags.Error())
	}

	var modules []ModuleInfo

	content, _, diags := file.Body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "module",
				LabelNames: []string{"name"},
			},
		},
	})

	if diags.HasErrors() {
		return nil, fmt.Errorf("content errors: %s", diags.Error())
	}

	for _, block := range content.Blocks {
		if block.Type != "module" {
			continue
		}

		moduleInfo := ModuleInfo{
			Name:     block.Labels[0],
			FilePath: path,
			Line:     block.DefRange.Start.Line,
		}

		// Extract source and version attributes
		attrs, diags := block.Body.JustAttributes()
		if diags.HasErrors() {
			log.Warn().Err(fmt.Errorf("%s", diags.Error())).
				Str("module", moduleInfo.Name).
				Msg("failed to extract attributes")
			continue
		}

		if sourceAttr, ok := attrs["source"]; ok {
			val, diags := sourceAttr.Expr.Value(nil)
			if !diags.HasErrors() && val.Type().FriendlyName() == "string" {
				moduleInfo.Source = val.AsString()
				moduleInfo.SourceType = s.DetermineSourceType(moduleInfo.Source)
			}
		}

		if versionAttr, ok := attrs["version"]; ok {
			val, diags := versionAttr.Expr.Value(nil)
			if !diags.HasErrors() && val.Type().FriendlyName() == "string" {
				moduleInfo.Version = val.AsString()
			}
		}

		// Only include modules with valid sources
		if moduleInfo.Source != "" {
			modules = append(modules, moduleInfo)
			log.Debug().
				Str("name", moduleInfo.Name).
				Str("source", moduleInfo.Source).
				Str("version", moduleInfo.Version).
				Str("type", string(moduleInfo.SourceType)).
				Msg("found module")
		}
	}

	return modules, nil
}

// DetermineSourceType determines the type of module source
func (s *Scanner) DetermineSourceType(source string) SourceType {
	// Check for git sources
	if strings.HasPrefix(source, "git::") ||
		strings.HasPrefix(source, "git@") ||
		strings.Contains(source, "github.com") && !strings.Contains(source, "registry.terraform.io") {
		return SourceTypeGit
	}

	// Check for local paths
	if strings.HasPrefix(source, "./") ||
		strings.HasPrefix(source, "../") ||
		strings.HasPrefix(source, "/") {
		return SourceTypeLocal
	}

	// Check for registry sources (namespace/name/provider or registry.terraform.io)
	if strings.Contains(source, "/") && !strings.Contains(source, "://") {
		parts := strings.Split(source, "/")
		if len(parts) >= 2 {
			return SourceTypeRegistry
		}
	}

	return SourceTypeUnknown
}

// shouldExclude checks if a path should be excluded
func (s *Scanner) shouldExclude(path string) bool {
	for _, pattern := range s.exclude {
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err == nil && matched {
			return true
		}
		// Also check if the pattern matches any part of the path
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

// shouldInclude checks if a file should be included
func (s *Scanner) shouldInclude(path string) bool {
	for _, pattern := range s.include {
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err == nil && matched {
			return true
		}
	}
	return false
}

// parseProviders parses a single Terraform file for required_providers blocks
func (s *Scanner) parseProviders(path string) ([]ProviderInfo, error) {
	parser := hclparse.NewParser()

	file, diags := parser.ParseHCLFile(path)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse errors: %s", diags.Error())
	}

	var providers []ProviderInfo

	// Look for terraform blocks
	content, _, diags := file.Body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type: "terraform",
			},
		},
	})

	if diags.HasErrors() {
		return nil, fmt.Errorf("content errors: %s", diags.Error())
	}

	for _, terraformBlock := range content.Blocks {
		if terraformBlock.Type != "terraform" {
			continue
		}

		// Look for required_providers block inside terraform block
		terraformContent, _, diags := terraformBlock.Body.PartialContent(&hcl.BodySchema{
			Blocks: []hcl.BlockHeaderSchema{
				{
					Type: "required_providers",
				},
			},
		})

		if diags.HasErrors() {
			continue
		}

		for _, requiredProvidersBlock := range terraformContent.Blocks {
			if requiredProvidersBlock.Type != "required_providers" {
				continue
			}

			// Extract provider attributes
			attrs, diags := requiredProvidersBlock.Body.JustAttributes()
			if diags.HasErrors() {
				log.Warn().Err(fmt.Errorf("%s", diags.Error())).
					Msg("failed to extract provider attributes")
				continue
			}

			for providerName, providerAttr := range attrs {
				providerInfo := ProviderInfo{
					Name:     providerName,
					FilePath: path,
					Line:     providerAttr.Range.Start.Line,
				}

				// Provider can be specified as a string (just source) or object with source and version
				val, diags := providerAttr.Expr.Value(nil)
				if diags.HasErrors() {
					continue
				}

				// Handle object format: { source = "...", version = "..." }
				if val.Type().IsObjectType() {
					// Check if source attribute exists before accessing
					if val.Type().HasAttribute("source") {
						sourceVal := val.GetAttr("source")
						if !sourceVal.IsNull() && sourceVal.Type().FriendlyName() == "string" {
							providerInfo.Source = sourceVal.AsString()
						}
					}

					// Check if version attribute exists before accessing
					if val.Type().HasAttribute("version") {
						versionVal := val.GetAttr("version")
						if !versionVal.IsNull() && versionVal.Type().FriendlyName() == "string" {
							providerInfo.Version = versionVal.AsString()
						}
					}
				} else if val.Type().FriendlyName() == "string" {
					// Handle string format (just source, no version)
					providerInfo.Source = val.AsString()
				}

				// Only include providers with valid sources
				if providerInfo.Source != "" {
					providers = append(providers, providerInfo)
					log.Debug().
						Str("name", providerInfo.Name).
						Str("source", providerInfo.Source).
						Str("version", providerInfo.Version).
						Msg("found provider")
				}
			}
		}
	}

	return providers, nil
}

// parseResources parses a single Terraform file for resource and data blocks
func (s *Scanner) parseResources(path string) ([]ResourceInfo, error) {
	parser := hclparse.NewParser()

	file, diags := parser.ParseHCLFile(path)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse errors: %s", diags.Error())
	}

	var resources []ResourceInfo

	// Look for resource and data blocks
	content, _, diags := file.Body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "resource",
				LabelNames: []string{"type", "name"},
			},
			{
				Type:       "data",
				LabelNames: []string{"type", "name"},
			},
		},
	})

	if diags.HasErrors() {
		return nil, fmt.Errorf("content errors: %s", diags.Error())
	}

	for _, block := range content.Blocks {
		if block.Type != "resource" && block.Type != "data" {
			continue
		}

		resourceInfo := ResourceInfo{
			Type:         block.Labels[0],
			Name:         block.Labels[1],
			FilePath:     path,
			Line:         block.DefRange.Start.Line,
			IsDataSource: block.Type == "data",
		}

		resources = append(resources, resourceInfo)
		log.Debug().
			Str("type", resourceInfo.Type).
			Str("name", resourceInfo.Name).
			Bool("is_data_source", resourceInfo.IsDataSource).
			Msg("found resource")
	}

	return resources, nil
}
