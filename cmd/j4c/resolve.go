package main

import (
	"context"
	"strings"

	"github.com/fwojciec/jira4claude"
)

// ResolveAssignee resolves an assignee value to a Jira account ID.
// Detection logic: empty → empty, "me" → GetMyself, contains "@" → FindUsers first match, else → passthrough.
func ResolveAssignee(ctx context.Context, value string, userSvc jira4claude.UserService) (string, error) {
	if value == "" {
		return "", nil
	}

	if value == "me" {
		user, err := userSvc.GetMyself(ctx)
		if err != nil {
			return "", err
		}
		return user.AccountID, nil
	}

	if strings.Contains(value, "@") {
		users, err := userSvc.FindUsers(ctx, value)
		if err != nil {
			return "", err
		}
		return users[0].AccountID, nil
	}

	return value, nil
}
