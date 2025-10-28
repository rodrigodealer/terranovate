package scanner

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

// ToolingDetection holds information about detected IaC tooling
type ToolingDetection struct {
	UsesTerramate  bool
	UsesTerragrunt bool
	RootPath       string
}

// DetectTooling detects if the project uses Terramate or Terragrunt
func DetectTooling(basePath string) *ToolingDetection {
	detection := &ToolingDetection{
		RootPath: basePath,
	}

	// Check for Terramate
	// Look for terramate.tm.hcl, stack.tm.hcl, or config.tm.hcl files
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on error
		}

		if info.IsDir() {
			// Skip .terraform directories
			if strings.Contains(path, ".terraform") {
				return filepath.SkipDir
			}
			return nil
		}

		fileName := filepath.Base(path)

		// Check for Terramate files
		if strings.HasSuffix(fileName, ".tm.hcl") ||
			fileName == "terramate.tm.hcl" ||
			fileName == "stack.tm.hcl" ||
			fileName == "config.tm.hcl" {
			detection.UsesTerramate = true
			log.Debug().Str("file", path).Msg("detected Terramate usage")
		}

		// Check for Terragrunt files
		if fileName == "terragrunt.hcl" {
			detection.UsesTerragrunt = true
			log.Debug().Str("file", path).Msg("detected Terragrunt usage")
		}

		// Stop early if both are detected
		if detection.UsesTerramate && detection.UsesTerragrunt {
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil {
		log.Debug().Err(err).Msg("error during tooling detection")
	}

	if detection.UsesTerramate {
		log.Info().Msg("detected Terramate - providers may be centrally managed")
	}
	if detection.UsesTerragrunt {
		log.Info().Msg("detected Terragrunt - providers may be centrally managed")
	}

	return detection
}

// FindProviderGenerationSource attempts to find the source file for generated providers
// in Terramate or Terragrunt configurations
func FindProviderGenerationSource(basePath string, tooling *ToolingDetection) string {
	if tooling.UsesTerragrunt {
		// Look for root terragrunt.hcl with generate blocks
		rootConfig := filepath.Join(basePath, "terragrunt.hcl")
		if _, err := os.Stat(rootConfig); err == nil {
			return rootConfig
		}

		// Look for parent terragrunt.hcl files
		parent := filepath.Dir(basePath)
		for i := 0; i < 5; i++ { // Look up to 5 levels
			parentConfig := filepath.Join(parent, "terragrunt.hcl")
			if _, err := os.Stat(parentConfig); err == nil {
				return parentConfig
			}
			parent = filepath.Dir(parent)
			if parent == "/" || parent == "." {
				break
			}
		}
	}

	if tooling.UsesTerramate {
		// Look for _generated_providers.tf or similar
		var generatedFile string
		filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}

			fileName := filepath.Base(path)
			if strings.Contains(fileName, "_generated") && strings.HasSuffix(fileName, ".tf") {
				generatedFile = path
				return filepath.SkipAll
			}

			return nil
		})

		if generatedFile != "" {
			return generatedFile
		}

		// Look for stack config
		stackConfig := filepath.Join(basePath, "stack.tm.hcl")
		if _, err := os.Stat(stackConfig); err == nil {
			return stackConfig
		}
	}

	return ""
}
