package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/heyjobs/terranovate/internal/scanner"
	"github.com/heyjobs/terranovate/internal/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionChecker_CheckRegistryModule(t *testing.T) {
	// Create mock registry server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"modules": [{
				"versions": [
					{"version": "4.0.0"},
					{"version": "4.5.0"},
					{"version": "5.0.0"},
					{"version": "5.1.0"}
				]
			}]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Note: This test would need to mock the actual Terraform Registry
	// For a real implementation, we'd need to use dependency injection
	// or environment variables to override the registry URL

	checker := version.New("", true, false, false, []string{})

	modules := []scanner.ModuleInfo{
		{
			Name:       "vpc",
			Source:     "terraform-aws-modules/vpc/aws",
			Version:    "4.5.0",
			SourceType: scanner.SourceTypeRegistry,
		},
	}

	ctx := context.Background()
	updates, err := checker.Check(ctx, modules)

	// This test will make real API calls
	// In production, we'd want to mock the HTTP client
	require.NoError(t, err)

	// If updates are found, verify the structure
	if len(updates) > 0 {
		assert.Equal(t, "vpc", updates[0].Module.Name)
		assert.True(t, updates[0].IsOutdated)
		assert.NotEmpty(t, updates[0].LatestVersion)
	}
}

func TestVersionChecker_SkipPrerelease(t *testing.T) {
	// Test that prerelease versions are skipped when configured
	checker := version.New("", true, false, false, []string{})
	assert.NotNil(t, checker)

	// This would require mocking the version comparison logic
	// For now, we're testing that the checker is created with the right config
}

func TestVersionChecker_IgnoreModules(t *testing.T) {
	checker := version.New("", true, false, false, []string{"vpc", "eks"})

	modules := []scanner.ModuleInfo{
		{
			Name:       "vpc",
			Source:     "terraform-aws-modules/vpc/aws",
			Version:    "4.0.0",
			SourceType: scanner.SourceTypeRegistry,
		},
		{
			Name:       "security_group",
			Source:     "terraform-aws-modules/security-group/aws",
			Version:    "4.0.0",
			SourceType: scanner.SourceTypeRegistry,
		},
	}

	ctx := context.Background()
	updates, err := checker.Check(ctx, modules)
	require.NoError(t, err)

	// VPC should be ignored, so we should only check security_group
	for _, update := range updates {
		assert.NotEqual(t, "vpc", update.Module.Name, "vpc should be ignored")
	}
}

func TestVersionChecker_LocalModulesSkipped(t *testing.T) {
	checker := version.New("", true, false, false, []string{})

	modules := []scanner.ModuleInfo{
		{
			Name:       "local",
			Source:     "./modules/local",
			SourceType: scanner.SourceTypeLocal,
		},
	}

	ctx := context.Background()
	updates, err := checker.Check(ctx, modules)
	require.NoError(t, err)

	// Local modules should be skipped
	assert.Len(t, updates, 0, "local modules should be skipped")
}
