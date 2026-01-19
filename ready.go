package jira4claude

import "strings"

// IsReady returns true if an issue is available to work on.
// An issue is NOT ready if:
//   - It has a resolved status (Done, Won't Do)
//   - It has an inward "is blocked by" link where the blocking issue is not Done
func IsReady(issue *Issue) bool {
	// Resolved issues are not ready to work on
	if issue.Status == StatusDone || issue.Status == StatusWontDo {
		return false
	}

	for _, link := range issue.Links {
		// Only check inward links (this issue is blocked BY another)
		if link.InwardIssue == nil {
			continue
		}
		// Only check "is blocked by" relationship (case-insensitive)
		if !strings.EqualFold(link.Type.Inward, LinkInwardBlockedBy) {
			continue
		}
		// If the blocker is not Done, this issue is not ready
		if link.InwardIssue.Status != StatusDone {
			return false
		}
	}
	return true
}
