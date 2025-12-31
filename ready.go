package jira4claude

import "strings"

// IsReady returns true if an issue has no unresolved blockers.
// An issue is considered blocked if it has an inward "is blocked by" link
// where the blocking issue's status is not "Done".
func IsReady(issue *Issue) bool {
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
