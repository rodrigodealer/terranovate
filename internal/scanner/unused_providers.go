package scanner

import (
	"strings"

	"github.com/rs/zerolog/log"
)

// UnusedProviderInfo represents a provider that is declared but not used
type UnusedProviderInfo struct {
	Provider   ProviderInfo
	Suggestion string
}

// DetectUnusedProviders identifies providers that are declared in required_providers
// but not actually used by any resources, data sources, or modules
func DetectUnusedProviders(providers []ProviderInfo, resources []ResourceInfo, modules []ModuleInfo, ignoreProviders []string) []UnusedProviderInfo {
	// Build a map of provider names that are actually used
	usedProviders := make(map[string]bool)

	// Check resources and data sources
	for _, resource := range resources {
		providerName := extractProviderFromResourceType(resource.Type)
		if providerName != "" {
			usedProviders[providerName] = true
			log.Debug().
				Str("resource_type", resource.Type).
				Str("extracted_provider", providerName).
				Msg("extracted provider from resource type")
		}
	}

	// Infer provider usage from module sources
	for _, module := range modules {
		providerName := inferProviderFromModuleSource(module.Source)
		if providerName != "" {
			usedProviders[providerName] = true
			log.Debug().
				Str("module", module.Name).
				Str("source", module.Source).
				Str("inferred_provider", providerName).
				Msg("inferred provider from module source")
		}
	}

	// Build ignore map for quick lookup
	ignoreMap := make(map[string]bool)
	for _, provider := range ignoreProviders {
		ignoreMap[provider] = true
	}

	// Check which declared providers are not used
	var unusedProviders []UnusedProviderInfo

	for _, provider := range providers {
		// Skip if in ignore list
		if ignoreMap[provider.Name] {
			log.Debug().
				Str("provider", provider.Name).
				Msg("skipping provider in ignore list")
			continue
		}

		// Check if provider is used
		if !usedProviders[provider.Name] {
			suggestion := generateSuggestion(provider)
			unusedProviders = append(unusedProviders, UnusedProviderInfo{
				Provider:   provider,
				Suggestion: suggestion,
			})

			log.Debug().
				Str("provider", provider.Name).
				Str("source", provider.Source).
				Str("file", provider.FilePath).
				Int("line", provider.Line).
				Msg("found unused provider")
		}
	}

	return unusedProviders
}

// inferProviderFromModuleSource infers which provider a module likely uses based on its source
// Examples:
//   - "terraform-aws-modules/vpc/aws" -> "aws"
//   - "terraform-google-modules/network/google" -> "google"
//   - "git::https://github.com/terraform-aws-modules/terraform-aws-eks.git" -> "aws"
func inferProviderFromModuleSource(source string) string {
	// Common patterns in module sources that indicate provider usage
	providerPatterns := map[string]string{
		"terraform-aws-modules":    "aws",
		"terraform-google-modules": "google",
		"terraform-azurerm-modules": "azurerm",
		"Azure/":                   "azurerm",
		"/aws":                     "aws",
		"/google":                  "google",
		"/azurerm":                 "azurerm",
		"/azure":                   "azurerm",
		"aws-":                     "aws",
		"google-":                  "google",
		"azurerm-":                 "azurerm",
		"gcp-":                     "google",
	}

	sourceLower := strings.ToLower(source)

	// Check for common patterns
	for pattern, provider := range providerPatterns {
		if strings.Contains(sourceLower, pattern) {
			return provider
		}
	}

	// No clear provider inference possible
	return ""
}

// extractProviderFromResourceType extracts the provider name from a resource type
// Example: "aws_instance" -> "aws", "google_compute_instance" -> "google"
// Special cases: "null_resource" -> "null", "random_string" -> "random"
func extractProviderFromResourceType(resourceType string) string {
	// Resource type format is typically: provider_resource
	// Examples: aws_instance, google_compute_instance, azurerm_resource_group

	parts := strings.SplitN(resourceType, "_", 2)
	if len(parts) < 2 {
		// Some resources might not follow standard naming
		log.Debug().
			Str("resource_type", resourceType).
			Msg("resource type does not follow standard naming convention")
		return ""
	}

	providerName := parts[0]

	// Handle special cases where provider name differs from resource prefix
	// Most providers follow the pattern, but there are edge cases
	switch providerName {
	case "template":
		// template_file, template_dir - these are deprecated, but map to "template" provider
		return "template"
	case "tls":
		// tls_private_key, tls_cert_request - map to "tls" provider
		return "tls"
	default:
		return providerName
	}
}

// generateSuggestion provides a helpful suggestion for the unused provider
func generateSuggestion(provider ProviderInfo) string {
	// Check if it's a commonly used utility provider
	utilityProviders := map[string]bool{
		"null":     true,
		"random":   true,
		"time":     true,
		"external": true,
		"local":    true,
		"archive":  true,
		"http":     true,
		"template": true,
		"tls":      true,
	}

	if utilityProviders[provider.Name] {
		return "This is a utility provider. Verify if it's used in modules or for implicit operations."
	}

	return "Consider removing this provider if it's not needed, or check if resources are defined in child modules."
}

// GetUsedProviderNames extracts a list of unique provider names from resources
func GetUsedProviderNames(resources []ResourceInfo) []string {
	usedProviders := make(map[string]bool)

	for _, resource := range resources {
		providerName := extractProviderFromResourceType(resource.Type)
		if providerName != "" {
			usedProviders[providerName] = true
		}
	}

	// Convert map to slice
	var providerNames []string
	for provider := range usedProviders {
		providerNames = append(providerNames, provider)
	}

	return providerNames
}
