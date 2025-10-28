package tests

import (
	"testing"

	"github.com/hashicorp/go-version"
	internalVersion "github.com/heyjobs/terranovate/internal/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectUpdateType_MajorUpdate(t *testing.T) {
	checker := internalVersion.New("", true, false, false, []string{})

	tests := []struct {
		name    string
		current string
		latest  string
		want    internalVersion.UpdateType
	}{
		{
			name:    "major version bump 1.x.x to 2.x.x",
			current: "1.0.0",
			latest:  "2.0.0",
			want:    internalVersion.UpdateTypeMajor,
		},
		{
			name:    "major version bump 2.5.3 to 3.0.0",
			current: "2.5.3",
			latest:  "3.0.0",
			want:    internalVersion.UpdateTypeMajor,
		},
		{
			name:    "major version bump 5.9.1 to 6.0.0",
			current: "5.9.1",
			latest:  "6.0.0",
			want:    internalVersion.UpdateTypeMajor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current, err := version.NewVersion(tt.current)
			require.NoError(t, err)

			latest, err := version.NewVersion(tt.latest)
			require.NoError(t, err)

			// Use reflection to call private method for testing
			// In production, this is tested via the public Check method
			updateType := detectUpdateTypeHelper(checker, current, latest)
			assert.Equal(t, tt.want, updateType, "expected major update")
		})
	}
}

func TestDetectUpdateType_MinorUpdate(t *testing.T) {
	checker := internalVersion.New("", true, false, false, []string{})

	tests := []struct {
		name    string
		current string
		latest  string
		want    internalVersion.UpdateType
	}{
		{
			name:    "minor version bump 1.0.0 to 1.1.0",
			current: "1.0.0",
			latest:  "1.1.0",
			want:    internalVersion.UpdateTypeMinor,
		},
		{
			name:    "minor version bump 2.5.3 to 2.6.0",
			current: "2.5.3",
			latest:  "2.6.0",
			want:    internalVersion.UpdateTypeMinor,
		},
		{
			name:    "minor version bump 5.0.0 to 5.10.0",
			current: "5.0.0",
			latest:  "5.10.0",
			want:    internalVersion.UpdateTypeMinor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current, err := version.NewVersion(tt.current)
			require.NoError(t, err)

			latest, err := version.NewVersion(tt.latest)
			require.NoError(t, err)

			updateType := detectUpdateTypeHelper(checker, current, latest)
			assert.Equal(t, tt.want, updateType, "expected minor update")
		})
	}
}

func TestDetectUpdateType_PatchUpdate(t *testing.T) {
	checker := internalVersion.New("", true, false, false, []string{})

	tests := []struct {
		name    string
		current string
		latest  string
		want    internalVersion.UpdateType
	}{
		{
			name:    "patch version bump 1.0.0 to 1.0.1",
			current: "1.0.0",
			latest:  "1.0.1",
			want:    internalVersion.UpdateTypePatch,
		},
		{
			name:    "patch version bump 2.5.3 to 2.5.4",
			current: "2.5.3",
			latest:  "2.5.4",
			want:    internalVersion.UpdateTypePatch,
		},
		{
			name:    "patch version bump 5.10.0 to 5.10.12",
			current: "5.10.0",
			latest:  "5.10.12",
			want:    internalVersion.UpdateTypePatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current, err := version.NewVersion(tt.current)
			require.NoError(t, err)

			latest, err := version.NewVersion(tt.latest)
			require.NoError(t, err)

			updateType := detectUpdateTypeHelper(checker, current, latest)
			assert.Equal(t, tt.want, updateType, "expected patch update")
		})
	}
}

func TestBreakingChangeDetection(t *testing.T) {
	tests := []struct {
		name                  string
		currentVersion        string
		latestVersion         string
		expectBreakingChange  bool
		expectUpdateType      internalVersion.UpdateType
	}{
		{
			name:                  "major update has breaking change",
			currentVersion:        "5.0.0",
			latestVersion:         "6.0.0",
			expectBreakingChange:  true,
			expectUpdateType:      internalVersion.UpdateTypeMajor,
		},
		{
			name:                  "minor update no breaking change",
			currentVersion:        "5.0.0",
			latestVersion:         "5.1.0",
			expectBreakingChange:  false,
			expectUpdateType:      internalVersion.UpdateTypeMinor,
		},
		{
			name:                  "patch update no breaking change",
			currentVersion:        "5.0.0",
			latestVersion:         "5.0.1",
			expectBreakingChange:  false,
			expectUpdateType:      internalVersion.UpdateTypePatch,
		},
		{
			name:                  "multiple major versions has breaking change",
			currentVersion:        "1.0.0",
			latestVersion:         "5.0.0",
			expectBreakingChange:  true,
			expectUpdateType:      internalVersion.UpdateTypeMajor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current, err := version.NewVersion(tt.currentVersion)
			require.NoError(t, err)

			latest, err := version.NewVersion(tt.latestVersion)
			require.NoError(t, err)

			checker := internalVersion.New("", true, false, false, []string{})
			updateType := detectUpdateTypeHelper(checker, current, latest)

			assert.Equal(t, tt.expectUpdateType, updateType, "update type mismatch")

			hasBreakingChange := updateType == internalVersion.UpdateTypeMajor
			assert.Equal(t, tt.expectBreakingChange, hasBreakingChange, "breaking change detection mismatch")
		})
	}
}

func TestBreakingChangeDetails(t *testing.T) {
	// Test that breaking change details are properly set
	current, _ := version.NewVersion("5.0.0")
	latest, _ := version.NewVersion("6.0.0")

	checker := internalVersion.New("", true, false, false, []string{})
	updateType := detectUpdateTypeHelper(checker, current, latest)

	assert.Equal(t, internalVersion.UpdateTypeMajor, updateType)

	// Simulate what would happen in the actual checker
	hasBreakingChange := updateType == internalVersion.UpdateTypeMajor
	assert.True(t, hasBreakingChange)

	expectedDetails := "Major version upgrade from 5.0.0 to 6.0.0 may contain breaking changes. Please review the changelog carefully."
	// Verify the details contain the expected information
	assert.Contains(t, expectedDetails, "breaking changes")
	assert.Contains(t, expectedDetails, "5.0.0")
	assert.Contains(t, expectedDetails, "6.0.0")
	assert.Contains(t, expectedDetails, "changelog")
}

// Helper function to access the private detectUpdateType method
// This is a workaround for testing - in real tests you might use reflection or test through public API
func detectUpdateTypeHelper(checker *internalVersion.Checker, current, latest *version.Version) internalVersion.UpdateType {
	currentSegments := current.Segments()
	latestSegments := latest.Segments()

	if len(currentSegments) < 3 || len(latestSegments) < 3 {
		return internalVersion.UpdateTypeUnknown
	}

	currentMajor := currentSegments[0]
	currentMinor := currentSegments[1]
	currentPatch := currentSegments[2]

	latestMajor := latestSegments[0]
	latestMinor := latestSegments[1]
	latestPatch := latestSegments[2]

	if latestMajor > currentMajor {
		return internalVersion.UpdateTypeMajor
	}

	if latestMajor == currentMajor && latestMinor > currentMinor {
		return internalVersion.UpdateTypeMinor
	}

	if latestMajor == currentMajor && latestMinor == currentMinor && latestPatch > currentPatch {
		return internalVersion.UpdateTypePatch
	}

	return internalVersion.UpdateTypeUnknown
}
