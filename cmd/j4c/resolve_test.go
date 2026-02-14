package main_test

import (
	"context"
	"testing"

	"github.com/fwojciec/jira4claude"
	main "github.com/fwojciec/jira4claude/cmd/j4c"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveAssignee(t *testing.T) {
	t.Parallel()

	t.Run("returns empty string for empty input", func(t *testing.T) {
		t.Parallel()

		userSvc := &mock.UserService{}

		result, err := main.ResolveAssignee(context.Background(), "", userSvc)

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("calls GetMyself for me input", func(t *testing.T) {
		t.Parallel()

		userSvc := &mock.UserService{
			GetMyselfFn: func(ctx context.Context) (*jira4claude.User, error) {
				return &jira4claude.User{AccountID: "myself-123"}, nil
			},
		}

		result, err := main.ResolveAssignee(context.Background(), "me", userSvc)

		require.NoError(t, err)
		assert.Equal(t, "myself-123", result)
	})

	t.Run("calls FindUsers for input containing @", func(t *testing.T) {
		t.Parallel()

		userSvc := &mock.UserService{
			FindUsersFn: func(ctx context.Context, query string) ([]*jira4claude.User, error) {
				assert.Equal(t, "user@example.com", query)
				return []*jira4claude.User{
					{AccountID: "found-456", DisplayName: "Found User", Email: "user@example.com"},
				}, nil
			},
		}

		result, err := main.ResolveAssignee(context.Background(), "user@example.com", userSvc)

		require.NoError(t, err)
		assert.Equal(t, "found-456", result)
	})

	t.Run("returns first match when FindUsers returns multiple users", func(t *testing.T) {
		t.Parallel()

		userSvc := &mock.UserService{
			FindUsersFn: func(ctx context.Context, query string) ([]*jira4claude.User, error) {
				return []*jira4claude.User{
					{AccountID: "first-user"},
					{AccountID: "second-user"},
				}, nil
			},
		}

		result, err := main.ResolveAssignee(context.Background(), "user@example.com", userSvc)

		require.NoError(t, err)
		assert.Equal(t, "first-user", result)
	})

	t.Run("passes through account ID as-is", func(t *testing.T) {
		t.Parallel()

		userSvc := &mock.UserService{}

		result, err := main.ResolveAssignee(context.Background(), "5b10ac8d82e05b22cc7d4ef5", userSvc)

		require.NoError(t, err)
		assert.Equal(t, "5b10ac8d82e05b22cc7d4ef5", result)
	})

	t.Run("returns error when GetMyself fails", func(t *testing.T) {
		t.Parallel()

		userSvc := &mock.UserService{
			GetMyselfFn: func(ctx context.Context) (*jira4claude.User, error) {
				return nil, &jira4claude.Error{Code: jira4claude.EUnauthorized, Message: "not authenticated"}
			},
		}

		_, err := main.ResolveAssignee(context.Background(), "me", userSvc)

		require.Error(t, err)
		assert.Equal(t, jira4claude.EUnauthorized, jira4claude.ErrorCode(err))
	})

	t.Run("returns error when FindUsers fails", func(t *testing.T) {
		t.Parallel()

		userSvc := &mock.UserService{
			FindUsersFn: func(ctx context.Context, query string) ([]*jira4claude.User, error) {
				return nil, &jira4claude.Error{Code: jira4claude.ENotFound, Message: "no users found"}
			},
		}

		_, err := main.ResolveAssignee(context.Background(), "nobody@example.com", userSvc)

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})
}
