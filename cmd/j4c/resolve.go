package main

import (
	"context"
	"fmt"
	"strconv"
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
		if len(users) == 0 {
			return "", &jira4claude.Error{
				Code:    jira4claude.ENotFound,
				Message: "no users found matching " + value,
			}
		}
		return users[0].AccountID, nil
	}

	return value, nil
}

// ResolveSprint resolves a sprint name or numeric ID to a sprint integer ID.
// If nameOrID is a valid integer, it is returned directly.
// Otherwise, all boards for the project are searched and the first sprint
// matching the name (case-insensitive) among active and future sprints is used.
func ResolveSprint(ctx context.Context, nameOrID string, project string, boardSvc jira4claude.BoardService, sprintSvc jira4claude.SprintService) (int, error) {
	if id, err := strconv.Atoi(nameOrID); err == nil {
		return id, nil
	}

	boards, err := boardSvc.ListBoards(ctx, project)
	if err != nil {
		return 0, fmt.Errorf("listing boards for sprint resolution: %w", err)
	}

	for _, board := range boards {
		sprints, err := sprintSvc.ListSprints(ctx, board.ID, nil)
		if err != nil {
			continue
		}
		for _, s := range sprints {
			if strings.EqualFold(s.Name, nameOrID) {
				return s.ID, nil
			}
		}
	}

	return 0, &jira4claude.Error{
		Code:    jira4claude.ENotFound,
		Message: fmt.Sprintf("sprint %q not found in project %q", nameOrID, project),
	}
}
