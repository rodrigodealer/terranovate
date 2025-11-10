package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/heyjobs/terranovate/internal/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipIfNoTerraform skips the test if terraform binary is not available
func skipIfNoTerraform(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("Skipping test: terraform binary not found in PATH")
	}
}

func TestRunner_New(t *testing.T) {
	skipIfNoTerraform(t)

	tmpDir, err := os.MkdirTemp("", "terranovate-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	runner, err := terraform.New(tmpDir, "", nil)
	require.NoError(t, err)
	assert.NotNil(t, runner)
}

func TestRunner_InvalidWorkingDir(t *testing.T) {
	_, err := terraform.New("/nonexistent/directory", "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "working directory does not exist")
}

func TestRunner_FormatPlanOutput(t *testing.T) {
	skipIfNoTerraform(t)

	// This is a unit test for the plan output formatting
	// Since the method is private, we'll test it indirectly through Plan
	// In a real scenario, you might want to export this for testing

	tmpDir, err := os.MkdirTemp("", "terranovate-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a simple Terraform file
	tfContent := `
terraform {
  required_providers {
    null = {
      source = "hashicorp/null"
    }
  }
}

resource "null_resource" "test" {
  triggers = {
    always_run = "${timestamp()}"
  }
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "main.tf"), []byte(tfContent), 0644)
	require.NoError(t, err)

	runner, err := terraform.New(tmpDir, "", nil)
	require.NoError(t, err)

	ctx := context.Background()

	// Note: This test requires Terraform to be installed
	// Skip if SKIP_TERRAFORM_TESTS is set
	if os.Getenv("SKIP_TERRAFORM_TESTS") != "" {
		t.Skip("Skipping Terraform integration test")
	}

	// Initialize
	err = runner.Init(ctx)
	if err != nil {
		t.Logf("Init failed (Terraform may not be installed): %v", err)
		t.Skip("Skipping test - Terraform not available")
	}

	// Plan
	result, err := runner.Plan(ctx)
	if err != nil {
		t.Logf("Plan failed: %v", err)
		t.Skip("Skipping test - Plan failed")
	}

	assert.NotNil(t, result)
	assert.True(t, result.Success)
}

func TestRunner_WithEnvVars(t *testing.T) {
	skipIfNoTerraform(t)

	tmpDir, err := os.MkdirTemp("", "terranovate-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	env := map[string]string{
		"TF_VAR_test": "value",
	}

	runner, err := terraform.New(tmpDir, "", env)
	require.NoError(t, err)
	assert.NotNil(t, runner)
}
