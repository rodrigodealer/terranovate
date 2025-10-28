package terraform

import (
	"testing"
)

func TestNewSchemaComparator(t *testing.T) {
	comparator := NewSchemaComparator()

	if comparator == nil {
		t.Fatal("NewSchemaComparator() returned nil")
	}

	if comparator.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestCompareSchemaStructures(t *testing.T) {
	comparator := NewSchemaComparator()

	tests := []struct {
		name                  string
		current               *ModuleSchema
		latest                *ModuleSchema
		wantHasChanges        bool
		wantAddedRequiredVars int
		wantRemovedVars       int
		wantChangedVarTypes   int
		wantRemovedOutputs    int
		wantAddedOutputs      int
	}{
		{
			name:           "nil schemas",
			current:        nil,
			latest:         nil,
			wantHasChanges: false,
		},
		{
			name: "no changes",
			current: &ModuleSchema{
				Variables: map[string]Variable{
					"vpc_cidr": {Type: "string", Required: true},
				},
				Outputs: map[string]Output{
					"vpc_id": {Description: "VPC ID"},
				},
			},
			latest: &ModuleSchema{
				Variables: map[string]Variable{
					"vpc_cidr": {Type: "string", Required: true},
				},
				Outputs: map[string]Output{
					"vpc_id": {Description: "VPC ID"},
				},
			},
			wantHasChanges: false,
		},
		{
			name: "added required variable",
			current: &ModuleSchema{
				Variables: map[string]Variable{
					"vpc_cidr": {Type: "string", Required: true},
				},
				Outputs: map[string]Output{},
			},
			latest: &ModuleSchema{
				Variables: map[string]Variable{
					"vpc_cidr":          {Type: "string", Required: true},
					"availability_zone": {Type: "string", Required: true, Description: "AZ for the VPC"},
				},
				Outputs: map[string]Output{},
			},
			wantHasChanges:        true,
			wantAddedRequiredVars: 1,
		},
		{
			name: "added optional variable - no breaking change",
			current: &ModuleSchema{
				Variables: map[string]Variable{
					"vpc_cidr": {Type: "string", Required: true},
				},
				Outputs: map[string]Output{},
			},
			latest: &ModuleSchema{
				Variables: map[string]Variable{
					"vpc_cidr": {Type: "string", Required: true},
					"tags":     {Type: "map(string)", Required: false},
				},
				Outputs: map[string]Output{},
			},
			wantHasChanges:        false, // Optional vars don't count as breaking
			wantAddedRequiredVars: 0,
		},
		{
			name: "removed variable",
			current: &ModuleSchema{
				Variables: map[string]Variable{
					"vpc_cidr":          {Type: "string", Required: true},
					"availability_zone": {Type: "string", Required: false},
				},
				Outputs: map[string]Output{},
			},
			latest: &ModuleSchema{
				Variables: map[string]Variable{
					"vpc_cidr": {Type: "string", Required: true},
				},
				Outputs: map[string]Output{},
			},
			wantHasChanges:  true,
			wantRemovedVars: 1,
		},
		{
			name: "changed variable type",
			current: &ModuleSchema{
				Variables: map[string]Variable{
					"instance_count": {Type: "number", Required: true},
				},
				Outputs: map[string]Output{},
			},
			latest: &ModuleSchema{
				Variables: map[string]Variable{
					"instance_count": {Type: "string", Required: true},
				},
				Outputs: map[string]Output{},
			},
			wantHasChanges:      true,
			wantChangedVarTypes: 1,
		},
		{
			name: "removed output",
			current: &ModuleSchema{
				Variables: map[string]Variable{},
				Outputs: map[string]Output{
					"vpc_id":         {Description: "VPC ID"},
					"subnet_ids":     {Description: "Subnet IDs"},
					"deprecated_out": {Description: "Old output"},
				},
			},
			latest: &ModuleSchema{
				Variables: map[string]Variable{},
				Outputs: map[string]Output{
					"vpc_id":     {Description: "VPC ID"},
					"subnet_ids": {Description: "Subnet IDs"},
				},
			},
			wantHasChanges:     true,
			wantRemovedOutputs: 1,
		},
		{
			name: "added output - not tracked as breaking",
			current: &ModuleSchema{
				Variables: map[string]Variable{},
				Outputs: map[string]Output{
					"vpc_id": {Description: "VPC ID"},
				},
			},
			latest: &ModuleSchema{
				Variables: map[string]Variable{},
				Outputs: map[string]Output{
					"vpc_id":     {Description: "VPC ID"},
					"subnet_ids": {Description: "Subnet IDs"},
				},
			},
			wantHasChanges:   false, // Added outputs don't count
			wantAddedOutputs: 0,      // Not tracking additions currently
		},
		{
			name: "multiple breaking changes",
			current: &ModuleSchema{
				Variables: map[string]Variable{
					"vpc_cidr":       {Type: "string", Required: true},
					"old_var":        {Type: "string", Required: false},
					"instance_count": {Type: "number", Required: true},
				},
				Outputs: map[string]Output{
					"vpc_id":     {Description: "VPC ID"},
					"old_output": {Description: "Deprecated"},
				},
			},
			latest: &ModuleSchema{
				Variables: map[string]Variable{
					"vpc_cidr":       {Type: "string", Required: true},
					"new_req_var":    {Type: "string", Required: true, Description: "New required"},
					"instance_count": {Type: "string", Required: true}, // type changed
				},
				Outputs: map[string]Output{
					"vpc_id": {Description: "VPC ID"},
				},
			},
			wantHasChanges:        true,
			wantAddedRequiredVars: 1,
			wantRemovedVars:       1,
			wantChangedVarTypes:   1,
			wantRemovedOutputs:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := comparator.compareSchemaStructures(tt.current, tt.latest)

			if changes == nil {
				t.Fatal("compareSchemaStructures() returned nil")
			}

			if changes.HasChanges != tt.wantHasChanges {
				t.Errorf("HasChanges = %v, want %v", changes.HasChanges, tt.wantHasChanges)
			}

			if len(changes.AddedRequiredVars) != tt.wantAddedRequiredVars {
				t.Errorf("AddedRequiredVars count = %d, want %d",
					len(changes.AddedRequiredVars), tt.wantAddedRequiredVars)
			}

			if len(changes.RemovedVars) != tt.wantRemovedVars {
				t.Errorf("RemovedVars count = %d, want %d",
					len(changes.RemovedVars), tt.wantRemovedVars)
			}

			if len(changes.ChangedVarTypes) != tt.wantChangedVarTypes {
				t.Errorf("ChangedVarTypes count = %d, want %d",
					len(changes.ChangedVarTypes), tt.wantChangedVarTypes)
			}

			if len(changes.RemovedOutputs) != tt.wantRemovedOutputs {
				t.Errorf("RemovedOutputs count = %d, want %d",
					len(changes.RemovedOutputs), tt.wantRemovedOutputs)
			}

			// Verify specific details for some tests
			if tt.name == "changed variable type" && len(changes.ChangedVarTypes) > 0 {
				change := changes.ChangedVarTypes[0]
				if change.Name != "instance_count" {
					t.Errorf("Changed variable name = %s, want instance_count", change.Name)
				}
				if change.Type != "number → string" {
					t.Errorf("Changed type = %s, want 'number → string'", change.Type)
				}
			}

			if tt.name == "added required variable" && len(changes.AddedRequiredVars) > 0 {
				change := changes.AddedRequiredVars[0]
				if change.Name != "availability_zone" {
					t.Errorf("Added variable name = %s, want availability_zone", change.Name)
				}
				if !change.Required {
					t.Error("Added variable should be marked as required")
				}
			}
		})
	}
}

func TestHasBreakingSchemaChanges(t *testing.T) {
	tests := []struct {
		name    string
		changes *SchemaChanges
		want    bool
	}{
		{
			name:    "nil changes",
			changes: nil,
			want:    false,
		},
		{
			name: "no changes",
			changes: &SchemaChanges{
				HasChanges: false,
			},
			want: false,
		},
		{
			name: "added required variable",
			changes: &SchemaChanges{
				HasChanges: true,
				AddedRequiredVars: []VariableChange{
					{Name: "new_var", Type: "string", Required: true},
				},
			},
			want: true,
		},
		{
			name: "removed variable",
			changes: &SchemaChanges{
				HasChanges: true,
				RemovedVars: []VariableChange{
					{Name: "old_var", Type: "string"},
				},
			},
			want: true,
		},
		{
			name: "changed variable type",
			changes: &SchemaChanges{
				HasChanges: true,
				ChangedVarTypes: []VariableChange{
					{Name: "var", Type: "string → number"},
				},
			},
			want: true,
		},
		{
			name: "removed output",
			changes: &SchemaChanges{
				HasChanges: true,
				RemovedOutputs: []OutputChange{
					{Name: "old_output"},
				},
			},
			want: true,
		},
		{
			name: "only added outputs - not breaking",
			changes: &SchemaChanges{
				HasChanges: true,
				AddedOutputs: []OutputChange{
					{Name: "new_output"},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasBreakingSchemaChanges(tt.changes)
			if got != tt.want {
				t.Errorf("HasBreakingSchemaChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestModuleSchemaStructure(t *testing.T) {
	// Test ModuleSchema structure
	schema := &ModuleSchema{
		Variables: map[string]Variable{
			"vpc_cidr": {
				Type:        "string",
				Description: "CIDR block for VPC",
				Default:     "10.0.0.0/16",
				Required:    false,
			},
			"availability_zone": {
				Type:        "string",
				Description: "AZ for resources",
				Required:    true,
			},
		},
		Outputs: map[string]Output{
			"vpc_id": {
				Description: "ID of the VPC",
			},
			"subnet_ids": {
				Description: "List of subnet IDs",
			},
		},
	}

	if len(schema.Variables) != 2 {
		t.Errorf("Variables count = %d, want 2", len(schema.Variables))
	}

	if len(schema.Outputs) != 2 {
		t.Errorf("Outputs count = %d, want 2", len(schema.Outputs))
	}

	// Check specific variable
	vpcCidr, exists := schema.Variables["vpc_cidr"]
	if !exists {
		t.Fatal("vpc_cidr variable not found")
	}

	if vpcCidr.Type != "string" {
		t.Errorf("vpc_cidr type = %s, want string", vpcCidr.Type)
	}

	if vpcCidr.Required {
		t.Error("vpc_cidr should not be required (has default)")
	}

	// Check required variable
	az, exists := schema.Variables["availability_zone"]
	if !exists {
		t.Fatal("availability_zone variable not found")
	}

	if !az.Required {
		t.Error("availability_zone should be required")
	}

	// Check output
	vpcId, exists := schema.Outputs["vpc_id"]
	if !exists {
		t.Fatal("vpc_id output not found")
	}

	if vpcId.Description != "ID of the VPC" {
		t.Errorf("vpc_id description = %s, want 'ID of the VPC'", vpcId.Description)
	}
}

func TestVariableChange(t *testing.T) {
	change := VariableChange{
		Name:        "instance_type",
		Type:        "string",
		Required:    true,
		Description: "EC2 instance type",
	}

	if change.Name != "instance_type" {
		t.Errorf("Name = %s, want instance_type", change.Name)
	}

	if change.Type != "string" {
		t.Errorf("Type = %s, want string", change.Type)
	}

	if !change.Required {
		t.Error("Required should be true")
	}

	if change.Description != "EC2 instance type" {
		t.Errorf("Description = %s, want 'EC2 instance type'", change.Description)
	}
}

func TestOutputChange(t *testing.T) {
	change := OutputChange{
		Name:        "instance_id",
		Description: "ID of the EC2 instance",
	}

	if change.Name != "instance_id" {
		t.Errorf("Name = %s, want instance_id", change.Name)
	}

	if change.Description != "ID of the EC2 instance" {
		t.Errorf("Description = %s, want 'ID of the EC2 instance'", change.Description)
	}
}

func TestSchemaChangesStructure(t *testing.T) {
	changes := &SchemaChanges{
		HasChanges: true,
		AddedRequiredVars: []VariableChange{
			{Name: "new_var", Type: "string", Required: true},
		},
		RemovedVars: []VariableChange{
			{Name: "old_var", Type: "string"},
		},
		ChangedVarTypes: []VariableChange{
			{Name: "changed_var", Type: "string → number"},
		},
		RemovedOutputs: []OutputChange{
			{Name: "old_output"},
		},
		AddedOutputs: []OutputChange{
			{Name: "new_output"},
		},
	}

	if !changes.HasChanges {
		t.Error("HasChanges should be true")
	}

	if len(changes.AddedRequiredVars) != 1 {
		t.Errorf("AddedRequiredVars length = %d, want 1", len(changes.AddedRequiredVars))
	}

	if len(changes.RemovedVars) != 1 {
		t.Errorf("RemovedVars length = %d, want 1", len(changes.RemovedVars))
	}

	if len(changes.ChangedVarTypes) != 1 {
		t.Errorf("ChangedVarTypes length = %d, want 1", len(changes.ChangedVarTypes))
	}

	if len(changes.RemovedOutputs) != 1 {
		t.Errorf("RemovedOutputs length = %d, want 1", len(changes.RemovedOutputs))
	}

	if len(changes.AddedOutputs) != 1 {
		t.Errorf("AddedOutputs length = %d, want 1", len(changes.AddedOutputs))
	}
}
