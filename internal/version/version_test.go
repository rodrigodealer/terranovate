package version

import (
	"testing"

	"github.com/hashicorp/go-version"
)

func TestExtractVersionFromConstraint(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		want       string
	}{
		{
			name:       "pessimistic constraint",
			constraint: "~> 5.0",
			want:       "5.0",
		},
		{
			name:       "greater than or equal",
			constraint: ">= 5.0.0",
			want:       "5.0.0",
		},
		{
			name:       "less than or equal",
			constraint: "<= 5.0.0",
			want:       "5.0.0",
		},
		{
			name:       "exact version",
			constraint: "= 5.0.0",
			want:       "5.0.0",
		},
		{
			name:       "greater than",
			constraint: "> 5.0.0",
			want:       "5.0.0",
		},
		{
			name:       "less than",
			constraint: "< 5.0.0",
			want:       "5.0.0",
		},
		{
			name:       "plain version",
			constraint: "5.0.0",
			want:       "5.0.0",
		},
		{
			name:       "version with spaces",
			constraint: "  >= 5.0.0  ",
			want:       "5.0.0",
		},
		{
			name:       "empty string",
			constraint: "",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractVersionFromConstraint(tt.constraint)
			if got != tt.want {
				t.Errorf("extractVersionFromConstraint(%q) = %q, want %q", tt.constraint, got, tt.want)
			}
		})
	}
}

func TestDetectUpdateType(t *testing.T) {
	checker := New("", false, false, false, nil)

	tests := []struct {
		name    string
		current string
		latest  string
		want    UpdateType
	}{
		{
			name:    "major update",
			current: "4.0.0",
			latest:  "5.0.0",
			want:    UpdateTypeMajor,
		},
		{
			name:    "minor update",
			current: "5.0.0",
			latest:  "5.1.0",
			want:    UpdateTypeMinor,
		},
		{
			name:    "patch update",
			current: "5.1.0",
			latest:  "5.1.1",
			want:    UpdateTypePatch,
		},
		{
			name:    "multiple major versions",
			current: "3.0.0",
			latest:  "5.0.0",
			want:    UpdateTypeMajor,
		},
		{
			name:    "multiple minor versions",
			current: "5.0.0",
			latest:  "5.5.0",
			want:    UpdateTypeMinor,
		},
		{
			name:    "multiple patch versions",
			current: "5.0.0",
			latest:  "5.0.10",
			want:    UpdateTypePatch,
		},
		{
			name:    "same version",
			current: "5.0.0",
			latest:  "5.0.0",
			want:    UpdateTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current, _ := version.NewVersion(tt.current)
			latest, _ := version.NewVersion(tt.latest)

			got := checker.detectUpdateType(current, latest)
			if got != tt.want {
				t.Errorf("detectUpdateType(%s, %s) = %s, want %s", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestShouldUpdate(t *testing.T) {
	tests := []struct {
		name       string
		current    string
		latest     string
		patchOnly  bool
		minorOnly  bool
		want       bool
	}{
		{
			name:    "no restrictions - major update",
			current: "4.0.0",
			latest:  "5.0.0",
			want:    true,
		},
		{
			name:    "no restrictions - minor update",
			current: "5.0.0",
			latest:  "5.1.0",
			want:    true,
		},
		{
			name:    "no restrictions - patch update",
			current: "5.0.0",
			latest:  "5.0.1",
			want:    true,
		},
		{
			name:      "patch only - patch available",
			current:   "5.0.0",
			latest:    "5.0.1",
			patchOnly: true,
			want:      true,
		},
		{
			name:      "patch only - minor available",
			current:   "5.0.0",
			latest:    "5.1.0",
			patchOnly: true,
			want:      false,
		},
		{
			name:      "patch only - major available",
			current:   "5.0.0",
			latest:    "6.0.0",
			patchOnly: true,
			want:      false,
		},
		{
			name:      "minor only - minor available",
			current:   "5.0.0",
			latest:    "5.1.0",
			minorOnly: true,
			want:      true,
		},
		{
			name:      "minor only - patch available",
			current:   "5.0.0",
			latest:    "5.0.1",
			minorOnly: true,
			want:      true,
		},
		{
			name:      "minor only - major available",
			current:   "5.0.0",
			latest:    "6.0.0",
			minorOnly: true,
			want:      false,
		},
		{
			name:    "current equals latest",
			current: "5.0.0",
			latest:  "5.0.0",
			want:    false,
		},
		{
			name:    "current greater than latest",
			current: "5.1.0",
			latest:  "5.0.0",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := New("", false, tt.patchOnly, tt.minorOnly, nil)
			current, _ := version.NewVersion(tt.current)
			latest, _ := version.NewVersion(tt.latest)

			got := checker.shouldUpdate(current, latest)
			if got != tt.want {
				t.Errorf("shouldUpdate(%s, %s, patchOnly=%v, minorOnly=%v) = %v, want %v",
					tt.current, tt.latest, tt.patchOnly, tt.minorOnly, got, tt.want)
			}
		})
	}
}

func TestIsIgnored(t *testing.T) {
	ignoreList := []string{"test-module", "local-module", "dev-module"}
	checker := New("", false, false, false, ignoreList)

	tests := []struct {
		name       string
		moduleName string
		want       bool
	}{
		{
			name:       "ignored module",
			moduleName: "test-module",
			want:       true,
		},
		{
			name:       "another ignored module",
			moduleName: "local-module",
			want:       true,
		},
		{
			name:       "not ignored module",
			moduleName: "vpc-module",
			want:       false,
		},
		{
			name:       "partial match not ignored",
			moduleName: "test-module-extended",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checker.isIgnored(tt.moduleName)
			if got != tt.want {
				t.Errorf("isIgnored(%s) = %v, want %v", tt.moduleName, got, tt.want)
			}
		})
	}
}

func TestParseGitSource(t *testing.T) {
	checker := New("", false, false, false, nil)

	tests := []struct {
		name      string
		source    string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "git https url",
			source:    "git::https://github.com/terraform-aws-modules/terraform-aws-vpc.git",
			wantOwner: "terraform-aws-modules",
			wantRepo:  "terraform-aws-vpc",
			wantErr:   false,
		},
		{
			name:      "git ssh url",
			source:    "git@github.com:terraform-aws-modules/terraform-aws-vpc.git",
			wantOwner: "terraform-aws-modules",
			wantRepo:  "terraform-aws-vpc",
			wantErr:   false,
		},
		{
			name:      "github url without git prefix",
			source:    "github.com/hashicorp/terraform",
			wantOwner: "hashicorp",
			wantRepo:  "terraform",
			wantErr:   false,
		},
		{
			name:      "git url with ref parameter",
			source:    "git::https://github.com/example/module.git?ref=v1.0.0",
			wantOwner: "example",
			wantRepo:  "module",
			wantErr:   false,
		},
		{
			name:    "invalid source",
			source:  "not-a-git-url",
			wantErr: true,
		},
		{
			name:    "gitlab url",
			source:  "git::https://gitlab.com/example/module.git",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := checker.parseGitSource(tt.source)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseGitSource(%s) error = %v, wantErr %v", tt.source, err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if owner != tt.wantOwner {
					t.Errorf("parseGitSource(%s) owner = %s, want %s", tt.source, owner, tt.wantOwner)
				}
				if repo != tt.wantRepo {
					t.Errorf("parseGitSource(%s) repo = %s, want %s", tt.source, repo, tt.wantRepo)
				}
			}
		})
	}
}

func TestExtractGitVersion(t *testing.T) {
	checker := New("", false, false, false, nil)

	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "ref parameter with version",
			source: "git::https://github.com/example/module.git?ref=v1.0.0",
			want:   "v1.0.0",
		},
		{
			name:   "ref parameter with branch",
			source: "git::https://github.com/example/module.git?ref=main",
			want:   "main",
		},
		{
			name:   "ref parameter with multiple params",
			source: "git::https://github.com/example/module.git?depth=1&ref=v2.0.0",
			want:   "v2.0.0",
		},
		{
			name:   "no ref parameter",
			source: "git::https://github.com/example/module.git",
			want:   "",
		},
		{
			name:   "empty source",
			source: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checker.extractGitVersion(tt.source)
			if got != tt.want {
				t.Errorf("extractGitVersion(%s) = %s, want %s", tt.source, got, tt.want)
			}
		})
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name           string
		githubToken    string
		skipPrerelease bool
		patchOnly      bool
		minorOnly      bool
		ignoreModules  []string
	}{
		{
			name:           "with token",
			githubToken:    "ghp_test123",
			skipPrerelease: true,
			patchOnly:      false,
			minorOnly:      false,
			ignoreModules:  []string{"test"},
		},
		{
			name:           "without token",
			githubToken:    "",
			skipPrerelease: false,
			patchOnly:      true,
			minorOnly:      false,
			ignoreModules:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := New(tt.githubToken, tt.skipPrerelease, tt.patchOnly, tt.minorOnly, tt.ignoreModules)

			if checker == nil {
				t.Fatal("New() returned nil")
			}

			if checker.httpClient == nil {
				t.Error("httpClient is nil")
			}

			if checker.githubClient == nil {
				t.Error("githubClient is nil")
			}

			if checker.skipPrerelease != tt.skipPrerelease {
				t.Errorf("skipPrerelease = %v, want %v", checker.skipPrerelease, tt.skipPrerelease)
			}

			if checker.patchOnly != tt.patchOnly {
				t.Errorf("patchOnly = %v, want %v", checker.patchOnly, tt.patchOnly)
			}

			if checker.minorOnly != tt.minorOnly {
				t.Errorf("minorOnly = %v, want %v", checker.minorOnly, tt.minorOnly)
			}

			if len(checker.ignoreModules) != len(tt.ignoreModules) {
				t.Errorf("ignoreModules length = %d, want %d", len(checker.ignoreModules), len(tt.ignoreModules))
			}

			if checker.cache == nil {
				t.Error("cache is nil")
			}

			if !checker.cache.IsMemoryOnly() {
				t.Error("cache should be memory-only")
			}
		})
	}
}

func TestUpdateType_String(t *testing.T) {
	tests := []struct {
		updateType UpdateType
		want       string
	}{
		{UpdateTypeMajor, "major"},
		{UpdateTypeMinor, "minor"},
		{UpdateTypePatch, "patch"},
		{UpdateTypeUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := string(tt.updateType)
			if got != tt.want {
				t.Errorf("UpdateType string = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestResourceChangesSummary(t *testing.T) {
	// Test that ResourceChangesSummary structure works as expected
	summary := &ResourceChangesSummary{
		HasChanges:   true,
		TotalReplace: 2,
		TotalDelete:  1,
		TotalModify:  3,
		ResourcesToReplace: []ResourceChange{
			{
				Address:      "aws_instance.web",
				ResourceType: "aws_instance",
				Action:       "replace",
				Reason:       "instance type changed",
			},
		},
		ResourcesToDelete: []ResourceChange{
			{
				Address:      "aws_s3_bucket.old",
				ResourceType: "aws_s3_bucket",
				Action:       "delete",
			},
		},
		ResourcesToModify: []ResourceChange{
			{
				Address:      "aws_security_group.main",
				ResourceType: "aws_security_group",
				Action:       "update",
			},
		},
	}

	if !summary.HasChanges {
		t.Error("HasChanges should be true")
	}

	if summary.TotalReplace != 2 {
		t.Errorf("TotalReplace = %d, want 2", summary.TotalReplace)
	}

	if len(summary.ResourcesToReplace) != 1 {
		t.Errorf("ResourcesToReplace length = %d, want 1", len(summary.ResourcesToReplace))
	}

	if summary.ResourcesToReplace[0].Address != "aws_instance.web" {
		t.Errorf("First replace address = %s, want aws_instance.web", summary.ResourcesToReplace[0].Address)
	}
}

func TestUpdateInfo(t *testing.T) {
	// Test that UpdateInfo structure works as expected
	updateInfo := UpdateInfo{
		CurrentVersion:        "4.0.0",
		LatestVersion:         "5.0.0",
		IsOutdated:            true,
		HasBreakingChange:     true,
		BreakingChangeDetails: "Major version change",
		ChangelogURL:          "https://example.com/changelog",
		UpdateType:            UpdateTypeMajor,
	}

	if !updateInfo.IsOutdated {
		t.Error("IsOutdated should be true")
	}

	if !updateInfo.HasBreakingChange {
		t.Error("HasBreakingChange should be true")
	}

	if updateInfo.UpdateType != UpdateTypeMajor {
		t.Errorf("UpdateType = %s, want %s", updateInfo.UpdateType, UpdateTypeMajor)
	}

	if updateInfo.CurrentVersion != "4.0.0" {
		t.Errorf("CurrentVersion = %s, want 4.0.0", updateInfo.CurrentVersion)
	}
}
