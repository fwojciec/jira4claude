package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/fwojciec/jira4claude"
)

// BoardService implements jira4claude.BoardService using the Jira Agile REST API.
type BoardService struct {
	client *Client
}

// Compile-time interface verification.
var _ jira4claude.BoardService = (*BoardService)(nil)

// NewBoardService creates a new BoardService using the provided HTTP client.
func NewBoardService(client *Client) *BoardService {
	return &BoardService{client: client}
}

// ListBoards returns boards associated with the given project key.
func (s *BoardService) ListBoards(ctx context.Context, project string) ([]*jira4claude.Board, error) {
	path := "/rest/agile/1.0/board"
	if project != "" {
		path += "?projectKeyOrId=" + url.QueryEscape(project)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	body, err := s.client.DoRequest(req, http.StatusOK)
	if err != nil {
		return nil, err
	}

	var resp boardListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	boards := make([]*jira4claude.Board, len(resp.Values))
	for i, b := range resp.Values {
		boards[i] = &jira4claude.Board{
			ID:   b.ID,
			Name: b.Name,
			Type: b.Type,
		}
	}
	return boards, nil
}

// SprintService implements jira4claude.SprintService using the Jira Agile REST API.
type SprintService struct {
	client *Client
}

// Compile-time interface verification.
var _ jira4claude.SprintService = (*SprintService)(nil)

// NewSprintService creates a new SprintService using the provided HTTP client.
func NewSprintService(client *Client) *SprintService {
	return &SprintService{client: client}
}

// ListSprints returns sprints on the given board.
// If states is empty, active and future sprints are returned.
func (s *SprintService) ListSprints(ctx context.Context, boardID int, states []string) ([]*jira4claude.Sprint, error) {
	stateParam := "active,future"
	if len(states) > 0 {
		stateParam = strings.Join(states, ",")
	}
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/sprint?state=%s", boardID, url.QueryEscape(stateParam))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	body, err := s.client.DoRequest(req, http.StatusOK)
	if err != nil {
		return nil, err
	}

	var resp sprintListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	sprints := make([]*jira4claude.Sprint, len(resp.Values))
	for i, sp := range resp.Values {
		sprints[i] = &jira4claude.Sprint{
			ID:    sp.ID,
			Name:  sp.Name,
			State: sp.State,
		}
	}
	return sprints, nil
}

// boardListResponse represents the Jira Agile API board list response.
type boardListResponse struct {
	Values []boardAPIResponse `json:"values"`
}

// boardAPIResponse represents a single board in the Jira Agile API response.
type boardAPIResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// sprintListResponse represents the Jira Agile API sprint list response.
type sprintListResponse struct {
	Values []sprintListItem `json:"values"`
}

// sprintListItem represents a single sprint in the Jira Agile API response.
type sprintListItem struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}
