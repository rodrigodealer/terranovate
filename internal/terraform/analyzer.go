package terraform

import (
	"fmt"
	"strings"

	"github.com/heyjobs/terranovate/internal/version"
)

// AnalyzeResourceChanges converts detailed terraform changes into a summary for version checking
func AnalyzeResourceChanges(planResult *PlanResult) *version.ResourceChangesSummary {
	if planResult == nil || len(planResult.DetailedChanges) == 0 {
		return &version.ResourceChangesSummary{
			HasChanges: false,
		}
	}

	summary := &version.ResourceChangesSummary{
		HasChanges:         true,
		ResourcesToReplace: []version.ResourceChange{},
		ResourcesToDelete:  []version.ResourceChange{},
		ResourcesToModify:  []version.ResourceChange{},
	}

	for _, change := range planResult.DetailedChanges {
		isReplace := false
		isDelete := false
		isUpdate := false

		// Determine action type
		hasCreate := false
		hasDelete := false

		for _, action := range change.Action {
			switch action {
			case "create":
				hasCreate = true
			case "delete":
				hasDelete = true
			case "update":
				isUpdate = true
			}
		}

		// Replace is delete + create
		if hasCreate && hasDelete {
			isReplace = true
		} else if hasDelete {
			isDelete = true
		}

		// Build reason string
		reason := buildChangeReason(change)

		resourceChange := version.ResourceChange{
			Address:      change.Address,
			ResourceType: change.ResourceType,
			Action:       strings.Join(change.Action, ", "),
			Reason:       reason,
		}

		// Categorize the change
		if isReplace {
			summary.ResourcesToReplace = append(summary.ResourcesToReplace, resourceChange)
			summary.TotalReplace++
		} else if isDelete {
			summary.ResourcesToDelete = append(summary.ResourcesToDelete, resourceChange)
			summary.TotalDelete++
		} else if isUpdate {
			summary.ResourcesToModify = append(summary.ResourcesToModify, resourceChange)
			summary.TotalModify++
		}
	}

	return summary
}

// buildChangeReason creates a human-readable reason for the resource change
func buildChangeReason(change ResourceChange) string {
	if len(change.ReplaceTriggers) == 0 {
		return "Module update requires resource replacement"
	}

	if len(change.ReplaceTriggers) == 1 {
		return fmt.Sprintf("Attribute '%s' requires replacement", change.ReplaceTriggers[0])
	}

	return fmt.Sprintf("Attributes %s require replacement",
		strings.Join(change.ReplaceTriggers, ", "))
}

// HasCriticalChanges checks if the resource changes include critical operations
func HasCriticalChanges(summary *version.ResourceChangesSummary) bool {
	if summary == nil {
		return false
	}

	// Replacements and deletions are considered critical
	return summary.TotalReplace > 0 || summary.TotalDelete > 0
}

// FormatResourceChanges creates a human-readable summary of resource changes
func FormatResourceChanges(summary *version.ResourceChangesSummary) string {
	if summary == nil || !summary.HasChanges {
		return "No resource changes detected"
	}

	var parts []string

	if summary.TotalReplace > 0 {
		parts = append(parts, fmt.Sprintf("⚠️  %d resource(s) will be REPLACED", summary.TotalReplace))
	}

	if summary.TotalDelete > 0 {
		parts = append(parts, fmt.Sprintf("🗑️  %d resource(s) will be DELETED", summary.TotalDelete))
	}

	if summary.TotalModify > 0 {
		parts = append(parts, fmt.Sprintf("📝 %d resource(s) will be MODIFIED", summary.TotalModify))
	}

	if len(parts) == 0 {
		return "Minor changes only"
	}

	return strings.Join(parts, "\n")
}
