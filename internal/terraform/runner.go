package terraform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/rs/zerolog/log"
)

// PlanResult represents the result of a Terraform plan
type PlanResult struct {
	Success      bool
	HasChanges   bool
	Output       string
	ErrorMessage string
	ResourcesAdd int
	ResourcesChange int
	ResourcesDestroy int
	DetailedChanges []ResourceChange
}

// ResourceChange represents a detailed resource change from plan
type ResourceChange struct {
	Address      string
	ResourceType string
	Action       []string // create, update, delete, replace
	ReplaceTriggers []string // attributes that trigger replacement
}

// Runner executes Terraform commands
type Runner struct {
	workingDir string
	binaryPath string
	env        map[string]string
}

// New creates a new Terraform Runner
func New(workingDir, binaryPath string, env map[string]string) (*Runner, error) {
	if workingDir == "" {
		workingDir = "."
	}

	// Ensure working directory exists
	if _, err := os.Stat(workingDir); err != nil {
		return nil, fmt.Errorf("working directory does not exist: %w", err)
	}

	// Find Terraform binary if not specified
	if binaryPath == "" {
		var err error
		binaryPath, err = exec.LookPath("terraform")
		if err != nil {
			return nil, fmt.Errorf("terraform binary not found in PATH: %w", err)
		}
	}

	return &Runner{
		workingDir: workingDir,
		binaryPath: binaryPath,
		env:        env,
	}, nil
}

// Init runs terraform init
func (r *Runner) Init(ctx context.Context) error {
	log.Info().Str("dir", r.workingDir).Msg("running terraform init")

	tf, err := r.newTerraform()
	if err != nil {
		return err
	}

	if err := tf.Init(ctx, tfexec.Upgrade(false)); err != nil {
		return fmt.Errorf("terraform init failed: %w", err)
	}

	log.Info().Msg("terraform init completed successfully")
	return nil
}

// Plan runs terraform plan and returns the result
func (r *Runner) Plan(ctx context.Context) (*PlanResult, error) {
	log.Info().Str("dir", r.workingDir).Msg("running terraform plan")

	tf, err := r.newTerraform()
	if err != nil {
		return nil, err
	}

	// Create a temporary file for the plan output
	planFile := filepath.Join(r.workingDir, ".terranovate-plan")
	defer os.Remove(planFile)

	hasChanges, err := tf.Plan(ctx, tfexec.Out(planFile))
	if err != nil {
		return &PlanResult{
			Success:      false,
			ErrorMessage: err.Error(),
		}, fmt.Errorf("terraform plan failed: %w", err)
	}

	// Get plan details
	plan, err := tf.ShowPlanFile(ctx, planFile)
	if err != nil {
		log.Warn().Err(err).Msg("failed to parse plan file, using basic result")
		return &PlanResult{
			Success:    true,
			HasChanges: hasChanges,
			Output:     "Plan completed but details unavailable",
		}, nil
	}

	// Count resources and analyze detailed changes
	resourcesAdd := 0
	resourcesChange := 0
	resourcesDestroy := 0
	var detailedChanges []ResourceChange

	if plan.ResourceChanges != nil {
		for _, rc := range plan.ResourceChanges {
			if rc.Change != nil {
				actions := rc.Change.Actions

				// Create detailed resource change entry
				change := ResourceChange{
					Address:      rc.Address,
					ResourceType: rc.Type,
					Action:       make([]string, 0, len(actions)),
				}

				// Convert actions to strings
				for _, action := range actions {
					change.Action = append(change.Action, string(action))
				}

				// Detect replacements and analyze replace triggers
				isReplace := false
				for _, action := range actions {
					switch action {
					case "create":
						resourcesAdd++
					case "update":
						resourcesChange++
					case "delete":
						resourcesDestroy++
					}

					// Check if this is a replace operation (delete + create)
					if action == "delete" {
						for _, a := range actions {
							if a == "create" {
								isReplace = true
								break
							}
						}
					}
				}

				// Extract replace triggers from the action reasons
				// The tfexec library provides this information in the ActionReason field
				if isReplace {
					// Try to identify what changed by looking at the resource type
					// This is a simplified approach - full diff analysis would be more complex
					if rc.Type != "" {
						change.ReplaceTriggers = append(change.ReplaceTriggers,
							"One or more immutable attributes changed")
					}
				}

				detailedChanges = append(detailedChanges, change)
			}
		}
	}

	// Generate human-readable output
	output := r.formatPlanOutput(resourcesAdd, resourcesChange, resourcesDestroy)

	result := &PlanResult{
		Success:          true,
		HasChanges:       hasChanges,
		Output:           output,
		ResourcesAdd:     resourcesAdd,
		ResourcesChange:  resourcesChange,
		ResourcesDestroy: resourcesDestroy,
		DetailedChanges:  detailedChanges,
	}

	log.Info().
		Bool("has_changes", hasChanges).
		Int("add", resourcesAdd).
		Int("change", resourcesChange).
		Int("destroy", resourcesDestroy).
		Msg("terraform plan completed")

	return result, nil
}

// Validate runs terraform validate
func (r *Runner) Validate(ctx context.Context) error {
	log.Info().Str("dir", r.workingDir).Msg("running terraform validate")

	tf, err := r.newTerraform()
	if err != nil {
		return err
	}

	diags, err := tf.Validate(ctx)
	if err != nil {
		return fmt.Errorf("terraform validate failed: %w", err)
	}

	if !diags.Valid {
		var errMsgs []string
		for _, diag := range diags.Diagnostics {
			errMsgs = append(errMsgs, diag.Summary)
		}
		return fmt.Errorf("validation errors: %s", strings.Join(errMsgs, "; "))
	}

	log.Info().Msg("terraform validate completed successfully")
	return nil
}

// Format runs terraform fmt on the working directory
func (r *Runner) Format(ctx context.Context) error {
	log.Info().Str("dir", r.workingDir).Msg("running terraform fmt")

	tf, err := r.newTerraform()
	if err != nil {
		return err
	}

	if err := tf.FormatWrite(ctx); err != nil {
		return fmt.Errorf("terraform fmt failed: %w", err)
	}

	log.Info().Msg("terraform fmt completed successfully")
	return nil
}

// newTerraform creates a new tfexec.Terraform instance
func (r *Runner) newTerraform() (*tfexec.Terraform, error) {
	tf, err := tfexec.NewTerraform(r.workingDir, r.binaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create terraform executor: %w", err)
	}

	// Set environment variables
	if r.env != nil {
		for key, value := range r.env {
			if err := tf.SetEnv(map[string]string{key: value}); err != nil {
				return nil, fmt.Errorf("failed to set env var %s: %w", key, err)
			}
		}
	}

	return tf, nil
}

// formatPlanOutput formats the plan result in a human-readable way
func (r *Runner) formatPlanOutput(add, change, destroy int) string {
	var parts []string

	if add > 0 {
		parts = append(parts, fmt.Sprintf("%d to add", add))
	}
	if change > 0 {
		parts = append(parts, fmt.Sprintf("%d to change", change))
	}
	if destroy > 0 {
		parts = append(parts, fmt.Sprintf("%d to destroy", destroy))
	}

	if len(parts) == 0 {
		return "No changes. Infrastructure is up-to-date."
	}

	return fmt.Sprintf("Plan: %s", strings.Join(parts, ", "))
}
