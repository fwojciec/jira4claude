package jira4claude

import "context"

// Board represents a Jira board.
type Board struct {
	ID   int
	Name string
	Type string // "scrum" or "kanban"
}

// Sprint represents a Jira sprint.
type Sprint struct {
	ID    int
	Name  string
	State string // "active", "future", "closed"
}

// BoardService defines operations for discovering Jira boards.
type BoardService interface {
	// ListBoards returns boards associated with the given project key.
	ListBoards(ctx context.Context, project string) ([]*Board, error)
}

// SprintService defines operations for managing Jira sprints.
type SprintService interface {
	// ListSprints returns sprints on the given board.
	// If states is empty, returns active and future sprints.
	// Common states: "active", "future", "closed".
	ListSprints(ctx context.Context, boardID int, states []string) ([]*Sprint, error)
}
