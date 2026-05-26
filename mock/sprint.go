package mock

import (
	"context"

	"github.com/fwojciec/jira4claude"
)

// Compile-time interface verifications.
var _ jira4claude.BoardService = (*BoardService)(nil)
var _ jira4claude.SprintService = (*SprintService)(nil)

// BoardService is a mock implementation of jira4claude.BoardService.
// Calling ListBoards without setting ListBoardsFn will panic.
type BoardService struct {
	ListBoardsFn func(ctx context.Context, project string) ([]*jira4claude.Board, error)
}

func (s *BoardService) ListBoards(ctx context.Context, project string) ([]*jira4claude.Board, error) {
	return s.ListBoardsFn(ctx, project)
}

// SprintService is a mock implementation of jira4claude.SprintService.
// Calling ListSprints without setting ListSprintsFn will panic.
type SprintService struct {
	ListSprintsFn func(ctx context.Context, boardID int, states []string) ([]*jira4claude.Sprint, error)
}

func (s *SprintService) ListSprints(ctx context.Context, boardID int, states []string) ([]*jira4claude.Sprint, error) {
	return s.ListSprintsFn(ctx, boardID, states)
}
