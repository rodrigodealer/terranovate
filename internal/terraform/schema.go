package terraform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/heyjobs/terranovate/internal/scanner"
	"github.com/rs/zerolog/log"
)

// SchemaChanges represents changes in module schema (inputs/outputs)
type SchemaChanges struct {
	HasChanges          bool
	AddedRequiredVars   []VariableChange
	RemovedVars         []VariableChange
	ChangedVarTypes     []VariableChange
	RemovedOutputs      []OutputChange
	AddedOutputs        []OutputChange
}

// VariableChange represents a change in a module variable
type VariableChange struct {
	Name        string
	Type        string
	Required    bool
	Description string
}

// OutputChange represents a change in a module output
type OutputChange struct {
	Name        string
	Description string
}

// ModuleSchema represents the schema of a Terraform module
type ModuleSchema struct {
	Variables map[string]Variable `json:"variables"`
	Outputs   map[string]Output   `json:"outputs"`
}

// Variable represents a module input variable
type Variable struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Default     interface{} `json:"default"`
	Required    bool   `json:"required"`
}

// Output represents a module output
type Output struct {
	Description string `json:"description"`
}

// SchemaComparator compares module schemas between versions
type SchemaComparator struct {
	httpClient *http.Client
}

// NewSchemaComparator creates a new schema comparator
func NewSchemaComparator() *SchemaComparator {
	return &SchemaComparator{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CompareSchemas compares schemas between two module versions
func (sc *SchemaComparator) CompareSchemas(ctx context.Context, module scanner.ModuleInfo, currentVersion, latestVersion string) (*SchemaChanges, error) {
	// Only support registry modules for now
	if module.SourceType != scanner.SourceTypeRegistry {
		return nil, nil // Schema comparison not available for non-registry modules
	}

	currentSchema, err := sc.fetchRegistrySchema(ctx, module.Source, currentVersion)
	if err != nil {
		log.Debug().Err(err).Msg("failed to fetch current schema")
		return nil, nil // Don't fail, just skip schema comparison
	}

	latestSchema, err := sc.fetchRegistrySchema(ctx, module.Source, latestVersion)
	if err != nil {
		log.Debug().Err(err).Msg("failed to fetch latest schema")
		return nil, nil
	}

	return sc.compareSchemaStructures(currentSchema, latestSchema), nil
}

// fetchRegistrySchema fetches module schema from Terraform Registry
func (sc *SchemaComparator) fetchRegistrySchema(ctx context.Context, source, version string) (*ModuleSchema, error) {
	// Parse module source (namespace/name/provider)
	parts := []rune(source)
	var namespace, name, provider string

	// Simple parsing - split by /
	slashCount := 0
	var part string
	for _, r := range parts {
		if r == '/' {
			slashCount++
			if slashCount == 1 {
				namespace = part
				part = ""
			} else if slashCount == 2 {
				name = part
				part = ""
			}
		} else {
			part += string(r)
		}
	}
	provider = part

	if namespace == "" || name == "" || provider == "" {
		return nil, fmt.Errorf("invalid module source format")
	}

	// Fetch module details from registry
	url := fmt.Sprintf("https://registry.terraform.io/v1/modules/%s/%s/%s/%s",
		namespace, name, provider, version)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := sc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	var moduleData struct {
		Root struct {
			Inputs  []struct {
				Name        string      `json:"name"`
				Type        string      `json:"type"`
				Description string      `json:"description"`
				Default     interface{} `json:"default"`
				Required    bool        `json:"required"`
			} `json:"inputs"`
			Outputs []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"outputs"`
		} `json:"root"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&moduleData); err != nil {
		return nil, err
	}

	// Convert to our schema format
	schema := &ModuleSchema{
		Variables: make(map[string]Variable),
		Outputs:   make(map[string]Output),
	}

	for _, input := range moduleData.Root.Inputs {
		schema.Variables[input.Name] = Variable{
			Type:        input.Type,
			Description: input.Description,
			Default:     input.Default,
			Required:    input.Required,
		}
	}

	for _, output := range moduleData.Root.Outputs {
		schema.Outputs[output.Name] = Output{
			Description: output.Description,
		}
	}

	return schema, nil
}

// compareSchemaStructures compares two schemas and returns the differences
func (sc *SchemaComparator) compareSchemaStructures(current, latest *ModuleSchema) *SchemaChanges {
	changes := &SchemaChanges{
		HasChanges:        false,
		AddedRequiredVars: []VariableChange{},
		RemovedVars:       []VariableChange{},
		ChangedVarTypes:   []VariableChange{},
		RemovedOutputs:    []OutputChange{},
		AddedOutputs:      []OutputChange{},
	}

	if current == nil || latest == nil {
		return changes
	}

	// Check for added and changed variables
	for name, latestVar := range latest.Variables {
		currentVar, exists := current.Variables[name]

		if !exists {
			// New variable added
			if latestVar.Required {
				changes.AddedRequiredVars = append(changes.AddedRequiredVars, VariableChange{
					Name:        name,
					Type:        latestVar.Type,
					Required:    true,
					Description: latestVar.Description,
				})
				changes.HasChanges = true
			}
		} else if currentVar.Type != latestVar.Type {
			// Variable type changed
			changes.ChangedVarTypes = append(changes.ChangedVarTypes, VariableChange{
				Name:        name,
				Type:        fmt.Sprintf("%s → %s", currentVar.Type, latestVar.Type),
				Description: latestVar.Description,
			})
			changes.HasChanges = true
		}
	}

	// Check for removed variables
	for name, currentVar := range current.Variables {
		if _, exists := latest.Variables[name]; !exists {
			changes.RemovedVars = append(changes.RemovedVars, VariableChange{
				Name:        name,
				Type:        currentVar.Type,
				Required:    currentVar.Required,
				Description: currentVar.Description,
			})
			changes.HasChanges = true
		}
	}

	// Check for removed outputs
	for name, currentOutput := range current.Outputs {
		if _, exists := latest.Outputs[name]; !exists {
			changes.RemovedOutputs = append(changes.RemovedOutputs, OutputChange{
				Name:        name,
				Description: currentOutput.Description,
			})
			changes.HasChanges = true
		}
	}

	// Check for added outputs (informational only)
	for name, latestOutput := range latest.Outputs {
		if _, exists := current.Outputs[name]; !exists {
			changes.AddedOutputs = append(changes.AddedOutputs, OutputChange{
				Name:        name,
				Description: latestOutput.Description,
			})
		}
	}

	return changes
}

// HasBreakingSchemaChanges determines if schema changes are breaking
func HasBreakingSchemaChanges(changes *SchemaChanges) bool {
	if changes == nil {
		return false
	}

	// Breaking changes: added required vars, removed vars, changed types, removed outputs
	return len(changes.AddedRequiredVars) > 0 ||
		len(changes.RemovedVars) > 0 ||
		len(changes.ChangedVarTypes) > 0 ||
		len(changes.RemovedOutputs) > 0
}
