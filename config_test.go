package jira4claude_test

import (
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	t.Parallel()

	t.Run("has Server and Project fields", func(t *testing.T) {
		t.Parallel()

		cfg := jira4claude.Config{
			Server:  "https://example.atlassian.net",
			Project: "TEST",
		}

		assert.Equal(t, "https://example.atlassian.net", cfg.Server)
		assert.Equal(t, "TEST", cfg.Project)
	})
}
