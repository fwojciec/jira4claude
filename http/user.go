package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/fwojciec/jira4claude"
)

// UserService implements jira4claude.UserService using the Jira REST API.
type UserService struct {
	client *Client
}

// Compile-time interface verification.
var _ jira4claude.UserService = (*UserService)(nil)

// NewUserService creates a new UserService using the provided HTTP client.
func NewUserService(client *Client) *UserService {
	return &UserService{client: client}
}

// GetMyself returns the currently authenticated user.
func (s *UserService) GetMyself(ctx context.Context) (*jira4claude.User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/rest/api/3/myself", nil)
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

	var resp userResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	return mapUser(&resp), nil
}

// FindUsers searches for users matching the given query string.
func (s *UserService) FindUsers(ctx context.Context, query string) ([]*jira4claude.User, error) {
	path := "/rest/api/3/user/search?query=" + url.QueryEscape(query)

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

	var resp []userResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	if len(resp) == 0 {
		return nil, &jira4claude.Error{
			Code:    jira4claude.ENotFound,
			Message: fmt.Sprintf("no users found matching query %q", query),
		}
	}

	users := make([]*jira4claude.User, len(resp))
	for i := range resp {
		users[i] = mapUser(&resp[i])
	}

	return users, nil
}
