package terraform

import (
	"strings"
	"testing"

	"github.com/heyjobs/terranovate/internal/version"
)

func TestAnalyzeResourceChanges(t *testing.T) {
	tests := []struct {
		name       string
		planResult *PlanResult
		want       *version.ResourceChangesSummary
	}{
		{
			name:       "nil plan result",
			planResult: nil,
			want: &version.ResourceChangesSummary{
				HasChanges: false,
			},
		},
		{
			name: "empty changes",
			planResult: &PlanResult{
				DetailedChanges: []ResourceChange{},
			},
			want: &version.ResourceChangesSummary{
				HasChanges: false,
			},
		},
		{
			name: "resource replacement",
			planResult: &PlanResult{
				DetailedChanges: []ResourceChange{
					{
						Address:      "aws_instance.web",
						ResourceType: "aws_instance",
						Action:       []string{"delete", "create"},
						ReplaceTriggers: []string{"ami"},
					},
				},
			},
			want: &version.ResourceChangesSummary{
				HasChanges:   true,
				TotalReplace: 1,
				ResourcesToReplace: []version.ResourceChange{
					{
						Address:      "aws_instance.web",
						ResourceType: "aws_instance",
						Action:       "delete, create",
						Reason:       "Attribute 'ami' requires replacement",
					},
				},
				ResourcesToDelete: []version.ResourceChange{},
				ResourcesToModify: []version.ResourceChange{},
			},
		},
		{
			name: "resource deletion",
			planResult: &PlanResult{
				DetailedChanges: []ResourceChange{
					{
						Address:      "aws_s3_bucket.old",
						ResourceType: "aws_s3_bucket",
						Action:       []string{"delete"},
					},
				},
			},
			want: &version.ResourceChangesSummary{
				HasChanges:  true,
				TotalDelete: 1,
				ResourcesToDelete: []version.ResourceChange{
					{
						Address:      "aws_s3_bucket.old",
						ResourceType: "aws_s3_bucket",
						Action:       "delete",
					},
				},
				ResourcesToReplace: []version.ResourceChange{},
				ResourcesToModify:  []version.ResourceChange{},
			},
		},
		{
			name: "resource modification",
			planResult: &PlanResult{
				DetailedChanges: []ResourceChange{
					{
						Address:      "aws_security_group.main",
						ResourceType: "aws_security_group",
						Action:       []string{"update"},
					},
				},
			},
			want: &version.ResourceChangesSummary{
				HasChanges:  true,
				TotalModify: 1,
				ResourcesToModify: []version.ResourceChange{
					{
						Address:      "aws_security_group.main",
						ResourceType: "aws_security_group",
						Action:       "update",
					},
				},
				ResourcesToReplace: []version.ResourceChange{},
				ResourcesToDelete:  []version.ResourceChange{},
			},
		},
		{
			name: "mixed changes",
			planResult: &PlanResult{
				DetailedChanges: []ResourceChange{
					{
						Address:         "aws_instance.web",
						ResourceType:    "aws_instance",
						Action:          []string{"delete", "create"},
						ReplaceTriggers: []string{"instance_type", "ami"},
					},
					{
						Address:      "aws_s3_bucket.old",
						ResourceType: "aws_s3_bucket",
						Action:       []string{"delete"},
					},
					{
						Address:      "aws_security_group.main",
						ResourceType: "aws_security_group",
						Action:       []string{"update"},
					},
				},
			},
			want: &version.ResourceChangesSummary{
				HasChanges:   true,
				TotalReplace: 1,
				TotalDelete:  1,
				TotalModify:  1,
			},
		},
		{
			name: "replacement with no triggers",
			planResult: &PlanResult{
				DetailedChanges: []ResourceChange{
					{
						Address:      "aws_db_instance.main",
						ResourceType: "aws_db_instance",
						Action:       []string{"delete", "create"},
					},
				},
			},
			want: &version.ResourceChangesSummary{
				HasChanges:   true,
				TotalReplace: 1,
				ResourcesToReplace: []version.ResourceChange{
					{
						Address:      "aws_db_instance.main",
						ResourceType: "aws_db_instance",
						Action:       "delete, create",
						Reason:       "Module update requires resource replacement",
					},
				},
				ResourcesToDelete: []version.ResourceChange{},
				ResourcesToModify: []version.ResourceChange{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnalyzeResourceChanges(tt.planResult)

			if got.HasChanges != tt.want.HasChanges {
				t.Errorf("HasChanges = %v, want %v", got.HasChanges, tt.want.HasChanges)
			}

			if got.TotalReplace != tt.want.TotalReplace {
				t.Errorf("TotalReplace = %d, want %d", got.TotalReplace, tt.want.TotalReplace)
			}

			if got.TotalDelete != tt.want.TotalDelete {
				t.Errorf("TotalDelete = %d, want %d", got.TotalDelete, tt.want.TotalDelete)
			}

			if got.TotalModify != tt.want.TotalModify {
				t.Errorf("TotalModify = %d, want %d", got.TotalModify, tt.want.TotalModify)
			}

			// Check detailed changes if specified
			if len(tt.want.ResourcesToReplace) > 0 && len(got.ResourcesToReplace) > 0 {
				if got.ResourcesToReplace[0].Address != tt.want.ResourcesToReplace[0].Address {
					t.Errorf("First replacement address = %s, want %s",
						got.ResourcesToReplace[0].Address, tt.want.ResourcesToReplace[0].Address)
				}
				if got.ResourcesToReplace[0].Reason != tt.want.ResourcesToReplace[0].Reason {
					t.Errorf("First replacement reason = %s, want %s",
						got.ResourcesToReplace[0].Reason, tt.want.ResourcesToReplace[0].Reason)
				}
			}
		})
	}
}

func TestBuildChangeReason(t *testing.T) {
	tests := []struct {
		name   string
		change ResourceChange
		want   string
	}{
		{
			name: "no triggers",
			change: ResourceChange{
				Address:         "aws_instance.web",
				ReplaceTriggers: []string{},
			},
			want: "Module update requires resource replacement",
		},
		{
			name: "single trigger",
			change: ResourceChange{
				Address:         "aws_instance.web",
				ReplaceTriggers: []string{"ami"},
			},
			want: "Attribute 'ami' requires replacement",
		},
		{
			name: "multiple triggers",
			change: ResourceChange{
				Address:         "aws_instance.web",
				ReplaceTriggers: []string{"ami", "instance_type", "availability_zone"},
			},
			want: "Attributes ami, instance_type, availability_zone require replacement",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildChangeReason(tt.change)
			if got != tt.want {
				t.Errorf("buildChangeReason() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHasCriticalChanges(t *testing.T) {
	tests := []struct {
		name    string
		summary *version.ResourceChangesSummary
		want    bool
	}{
		{
			name:    "nil summary",
			summary: nil,
			want:    false,
		},
		{
			name: "no changes",
			summary: &version.ResourceChangesSummary{
				HasChanges: false,
			},
			want: false,
		},
		{
			name: "only modifications",
			summary: &version.ResourceChangesSummary{
				HasChanges:  true,
				TotalModify: 5,
			},
			want: false,
		},
		{
			name: "has replacements",
			summary: &version.ResourceChangesSummary{
				HasChanges:   true,
				TotalReplace: 2,
			},
			want: true,
		},
		{
			name: "has deletions",
			summary: &version.ResourceChangesSummary{
				HasChanges:  true,
				TotalDelete: 1,
			},
			want: true,
		},
		{
			name: "has both replacements and deletions",
			summary: &version.ResourceChangesSummary{
				HasChanges:   true,
				TotalReplace: 2,
				TotalDelete:  3,
			},
			want: true,
		},
		{
			name: "has all types of changes",
			summary: &version.ResourceChangesSummary{
				HasChanges:   true,
				TotalReplace: 1,
				TotalDelete:  1,
				TotalModify:  5,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasCriticalChanges(tt.summary)
			if got != tt.want {
				t.Errorf("HasCriticalChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatResourceChanges(t *testing.T) {
	tests := []struct {
		name         string
		summary      *version.ResourceChangesSummary
		want         string
		wantContains []string
	}{
		{
			name:    "nil summary",
			summary: nil,
			want:    "No resource changes detected",
		},
		{
			name: "no changes",
			summary: &version.ResourceChangesSummary{
				HasChanges: false,
			},
			want: "No resource changes detected",
		},
		{
			name: "only replacements",
			summary: &version.ResourceChangesSummary{
				HasChanges:   true,
				TotalReplace: 3,
			},
			wantContains: []string{
				"3 resource(s) will be REPLACED",
			},
		},
		{
			name: "only deletions",
			summary: &version.ResourceChangesSummary{
				HasChanges:  true,
				TotalDelete: 2,
			},
			wantContains: []string{
				"2 resource(s) will be DELETED",
			},
		},
		{
			name: "only modifications",
			summary: &version.ResourceChangesSummary{
				HasChanges:  true,
				TotalModify: 5,
			},
			wantContains: []string{
				"5 resource(s) will be MODIFIED",
			},
		},
		{
			name: "mixed changes",
			summary: &version.ResourceChangesSummary{
				HasChanges:   true,
				TotalReplace: 2,
				TotalDelete:  1,
				TotalModify:  3,
			},
			wantContains: []string{
				"2 resource(s) will be REPLACED",
				"1 resource(s) will be DELETED",
				"3 resource(s) will be MODIFIED",
			},
		},
		{
			name: "changes but no counts",
			summary: &version.ResourceChangesSummary{
				HasChanges: true,
			},
			want: "Minor changes only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatResourceChanges(tt.summary)

			if tt.want != "" {
				if got != tt.want {
					t.Errorf("FormatResourceChanges() = %q, want %q", got, tt.want)
				}
			}

			for _, wantStr := range tt.wantContains {
				if !strings.Contains(got, wantStr) {
					t.Errorf("FormatResourceChanges() does not contain %q\nGot: %s", wantStr, got)
				}
			}
		})
	}
}

func TestFormatPlanOutput(t *testing.T) {
	runner := &Runner{
		workingDir: ".",
	}

	tests := []struct {
		name    string
		add     int
		change  int
		destroy int
		want    string
	}{
		{
			name:    "no changes",
			add:     0,
			change:  0,
			destroy: 0,
			want:    "No changes. Infrastructure is up-to-date.",
		},
		{
			name:    "only additions",
			add:     5,
			change:  0,
			destroy: 0,
			want:    "Plan: 5 to add",
		},
		{
			name:    "only changes",
			add:     0,
			change:  3,
			destroy: 0,
			want:    "Plan: 3 to change",
		},
		{
			name:    "only destroy",
			add:     0,
			change:  0,
			destroy: 2,
			want:    "Plan: 2 to destroy",
		},
		{
			name:    "mixed operations",
			add:     5,
			change:  3,
			destroy: 2,
			want:    "Plan: 5 to add, 3 to change, 2 to destroy",
		},
		{
			name:    "add and change",
			add:     10,
			change:  5,
			destroy: 0,
			want:    "Plan: 10 to add, 5 to change",
		},
		{
			name:    "change and destroy",
			add:     0,
			change:  3,
			destroy: 1,
			want:    "Plan: 3 to change, 1 to destroy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runner.formatPlanOutput(tt.add, tt.change, tt.destroy)
			if got != tt.want {
				t.Errorf("formatPlanOutput(%d, %d, %d) = %q, want %q",
					tt.add, tt.change, tt.destroy, got, tt.want)
			}
		})
	}
}

func TestPlanResult(t *testing.T) {
	// Test PlanResult structure
	result := &PlanResult{
		Success:          true,
		HasChanges:       true,
		Output:           "Plan: 5 to add, 3 to change, 2 to destroy",
		ResourcesAdd:     5,
		ResourcesChange:  3,
		ResourcesDestroy: 2,
		DetailedChanges: []ResourceChange{
			{
				Address:      "aws_instance.web",
				ResourceType: "aws_instance",
				Action:       []string{"create"},
			},
		},
	}

	if !result.Success {
		t.Error("PlanResult.Success should be true")
	}

	if !result.HasChanges {
		t.Error("PlanResult.HasChanges should be true")
	}

	if result.ResourcesAdd != 5 {
		t.Errorf("ResourcesAdd = %d, want 5", result.ResourcesAdd)
	}

	if result.ResourcesChange != 3 {
		t.Errorf("ResourcesChange = %d, want 3", result.ResourcesChange)
	}

	if result.ResourcesDestroy != 2 {
		t.Errorf("ResourcesDestroy = %d, want 2", result.ResourcesDestroy)
	}

	if len(result.DetailedChanges) != 1 {
		t.Errorf("DetailedChanges length = %d, want 1", len(result.DetailedChanges))
	}
}

func TestResourceChange(t *testing.T) {
	// Test ResourceChange structure
	change := ResourceChange{
		Address:         "aws_vpc.main",
		ResourceType:    "aws_vpc",
		Action:          []string{"delete", "create"},
		ReplaceTriggers: []string{"cidr_block"},
	}

	if change.Address != "aws_vpc.main" {
		t.Errorf("Address = %s, want aws_vpc.main", change.Address)
	}

	if change.ResourceType != "aws_vpc" {
		t.Errorf("ResourceType = %s, want aws_vpc", change.ResourceType)
	}

	if len(change.Action) != 2 {
		t.Errorf("Action length = %d, want 2", len(change.Action))
	}

	if len(change.ReplaceTriggers) != 1 {
		t.Errorf("ReplaceTriggers length = %d, want 1", len(change.ReplaceTriggers))
	}

	if change.ReplaceTriggers[0] != "cidr_block" {
		t.Errorf("ReplaceTriggers[0] = %s, want cidr_block", change.ReplaceTriggers[0])
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name       string
		workingDir string
		binaryPath string
		env        map[string]string
		wantErr    bool
	}{
		{
			name:       "default working dir",
			workingDir: "",
			binaryPath: "",
			env:        nil,
			wantErr:    false,
		},
		{
			name:       "current directory",
			workingDir: ".",
			binaryPath: "",
			env:        nil,
			wantErr:    false,
		},
		{
			name:       "with environment variables",
			workingDir: ".",
			binaryPath: "",
			env: map[string]string{
				"TF_LOG": "DEBUG",
			},
			wantErr: false,
		},
		{
			name:       "non-existent directory",
			workingDir: "/this/path/does/not/exist",
			binaryPath: "",
			env:        nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner, err := New(tt.workingDir, tt.binaryPath, tt.env)

			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if runner == nil {
					t.Fatal("New() returned nil runner")
				}

				expectedDir := tt.workingDir
				if expectedDir == "" {
					expectedDir = "."
				}

				if runner.workingDir != expectedDir {
					t.Errorf("workingDir = %s, want %s", runner.workingDir, expectedDir)
				}

				if runner.binaryPath == "" {
					t.Error("binaryPath should be set")
				}

				if tt.env != nil && len(runner.env) != len(tt.env) {
					t.Errorf("env length = %d, want %d", len(runner.env), len(tt.env))
				}
			}
		})
	}
}
