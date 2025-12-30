package jira4claude_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/stretchr/testify/assert"
)

func TestError_Error(t *testing.T) {
	t.Parallel()

	t.Run("returns message when set", func(t *testing.T) {
		t.Parallel()

		err := &jira4claude.Error{
			Code:    jira4claude.ENotFound,
			Message: "issue not found",
		}

		assert.Equal(t, "issue not found", err.Error())
	})

	t.Run("returns inner error message when message is empty", func(t *testing.T) {
		t.Parallel()

		inner := errors.New("underlying error")
		err := &jira4claude.Error{
			Code:  jira4claude.EInternal,
			Inner: inner,
		}

		assert.Equal(t, "underlying error", err.Error())
	})

	t.Run("returns generic message when both message and inner are empty", func(t *testing.T) {
		t.Parallel()

		err := &jira4claude.Error{
			Code: jira4claude.EInternal,
		}

		assert.Equal(t, "an error occurred", err.Error())
	})
}

func TestError_Unwrap(t *testing.T) {
	t.Parallel()

	t.Run("returns inner error", func(t *testing.T) {
		t.Parallel()

		inner := errors.New("inner error")
		err := &jira4claude.Error{
			Code:  jira4claude.EInternal,
			Inner: inner,
		}

		assert.Equal(t, inner, err.Unwrap())
	})

	t.Run("returns nil when no inner error", func(t *testing.T) {
		t.Parallel()

		err := &jira4claude.Error{
			Code:    jira4claude.ENotFound,
			Message: "not found",
		}

		assert.NoError(t, err.Unwrap())
	})
}

func TestErrorCode(t *testing.T) {
	t.Parallel()

	t.Run("returns code from jira4claude.Error", func(t *testing.T) {
		t.Parallel()

		err := &jira4claude.Error{
			Code:    jira4claude.ENotFound,
			Message: "not found",
		}

		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})

	t.Run("returns code from wrapped jira4claude.Error", func(t *testing.T) {
		t.Parallel()

		inner := &jira4claude.Error{
			Code:    jira4claude.EUnauthorized,
			Message: "unauthorized",
		}
		wrapped := errors.Join(errors.New("context"), inner)

		assert.Equal(t, jira4claude.EUnauthorized, jira4claude.ErrorCode(wrapped))
	})

	t.Run("returns code from deeply wrapped jira4claude.Error", func(t *testing.T) {
		t.Parallel()

		inner := &jira4claude.Error{
			Code:    jira4claude.ENotFound,
			Message: "not found",
		}
		wrapped := fmt.Errorf("context 1: %w", inner)
		deeplyWrapped := fmt.Errorf("context 2: %w", wrapped)

		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(deeplyWrapped))
	})

	t.Run("returns EInternal for non-jira4claude errors", func(t *testing.T) {
		t.Parallel()

		err := errors.New("some error")

		assert.Equal(t, jira4claude.EInternal, jira4claude.ErrorCode(err))
	})

	t.Run("returns empty string for nil error", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, jira4claude.ErrorCode(nil))
	})
}

func TestErrorMessage(t *testing.T) {
	t.Parallel()

	t.Run("returns message from jira4claude.Error", func(t *testing.T) {
		t.Parallel()

		err := &jira4claude.Error{
			Code:    jira4claude.ENotFound,
			Message: "issue not found",
		}

		assert.Equal(t, "issue not found", jira4claude.ErrorMessage(err))
	})

	t.Run("returns message from wrapped jira4claude.Error", func(t *testing.T) {
		t.Parallel()

		inner := &jira4claude.Error{
			Code:    jira4claude.EForbidden,
			Message: "access denied",
		}
		wrapped := errors.Join(errors.New("context"), inner)

		assert.Equal(t, "access denied", jira4claude.ErrorMessage(wrapped))
	})

	t.Run("returns Error() for non-jira4claude errors", func(t *testing.T) {
		t.Parallel()

		err := errors.New("some error message")

		assert.Equal(t, "some error message", jira4claude.ErrorMessage(err))
	})

	t.Run("returns empty string for nil error", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, jira4claude.ErrorMessage(nil))
	})
}

func TestErrorCodes(t *testing.T) {
	t.Parallel()

	// Verify all required error codes are defined as non-empty strings
	codes := []string{
		jira4claude.ENotFound,
		jira4claude.EConflict,
		jira4claude.EUnauthorized,
		jira4claude.EForbidden,
		jira4claude.EValidation,
		jira4claude.ERateLimit,
		jira4claude.EInternal,
	}

	for _, code := range codes {
		assert.NotEmpty(t, code, "error code should not be empty")
	}

	// Verify all codes are unique
	seen := make(map[string]bool)
	for _, code := range codes {
		assert.False(t, seen[code], "error code %q should be unique", code)
		seen[code] = true
	}
}
