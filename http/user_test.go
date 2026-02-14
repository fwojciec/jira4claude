package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fwojciec/jira4claude"
	jirahttp "github.com/fwojciec/jira4claude/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserService_GetMyself(t *testing.T) {
	t.Parallel()

	t.Run("returns authenticated user from API response", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/rest/api/3/myself", r.URL.Path)
			assert.Equal(t, http.MethodGet, r.Method)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"accountId": "abc123",
				"displayName": "Test User",
				"emailAddress": "test@example.com"
			}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewUserService(client)

		user, err := svc.GetMyself(context.Background())

		require.NoError(t, err)
		assert.Equal(t, "abc123", user.AccountID)
		assert.Equal(t, "Test User", user.DisplayName)
		assert.Equal(t, "test@example.com", user.Email)
	})

	t.Run("returns error on API failure", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"errorMessages": ["Not authenticated"]}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewUserService(client)

		_, err := svc.GetMyself(context.Background())

		require.Error(t, err)
		assert.Equal(t, jira4claude.EUnauthorized, jira4claude.ErrorCode(err))
	})
}

func TestUserService_FindUsers(t *testing.T) {
	t.Parallel()

	t.Run("returns matching users for email query", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/rest/api/3/user/search", r.URL.Path)
			assert.Equal(t, "test@example.com", r.URL.Query().Get("query"))
			assert.Equal(t, http.MethodGet, r.Method)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{
					"accountId": "user1",
					"displayName": "User One",
					"emailAddress": "user1@example.com"
				},
				{
					"accountId": "user2",
					"displayName": "User Two",
					"emailAddress": "user2@example.com"
				}
			]`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewUserService(client)

		users, err := svc.FindUsers(context.Background(), "test@example.com")

		require.NoError(t, err)
		require.Len(t, users, 2)
		assert.Equal(t, "user1", users[0].AccountID)
		assert.Equal(t, "User One", users[0].DisplayName)
		assert.Equal(t, "user1@example.com", users[0].Email)
		assert.Equal(t, "user2", users[1].AccountID)
	})

	t.Run("returns ENotFound error when no users found", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewUserService(client)

		_, err := svc.FindUsers(context.Background(), "nobody@example.com")

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})

	t.Run("returns error on API failure", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errorMessages": ["Server error"]}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token", jirahttp.WithMaxRetries(0))
		svc := jirahttp.NewUserService(client)

		_, err := svc.FindUsers(context.Background(), "test@example.com")

		require.Error(t, err)
		assert.Equal(t, jira4claude.EInternal, jira4claude.ErrorCode(err))
	})
}
