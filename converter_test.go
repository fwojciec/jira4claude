package jira4claude_test

import (
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConversionIssue_String(t *testing.T) {
	t.Parallel()

	t.Run("includes kind, node and detail", func(t *testing.T) {
		t.Parallel()
		issue := jira4claude.ConversionIssue{Kind: jira4claude.ConversionDropped, Node: "Image", Detail: "not supported"}
		assert.Equal(t, "dropped Image: not supported", issue.String())
	})

	t.Run("omits empty detail", func(t *testing.T) {
		t.Parallel()
		issue := jira4claude.ConversionIssue{Kind: jira4claude.ConversionDegraded, Node: "emoji"}
		assert.Equal(t, "degraded emoji", issue.String())
	})
}

func TestDroppedContentError(t *testing.T) {
	t.Parallel()

	t.Run("nil when nothing was dropped", func(t *testing.T) {
		t.Parallel()
		issues := []jira4claude.ConversionIssue{
			{Kind: jira4claude.ConversionDegraded, Node: "emoji"},
		}
		assert.NoError(t, jira4claude.DroppedContentError("description", issues))
		assert.NoError(t, jira4claude.DroppedContentError("description", nil))
	})

	t.Run("validation error naming every dropped node", func(t *testing.T) {
		t.Parallel()
		issues := []jira4claude.ConversionIssue{
			{Kind: jira4claude.ConversionDropped, Node: "HTMLBlock"},
			{Kind: jira4claude.ConversionDegraded, Node: "emoji"},
			{Kind: jira4claude.ConversionDropped, Node: "Image"},
		}

		err := jira4claude.DroppedContentError("description", issues)

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.Equal(t, 1, jira4claude.ExitCode(err))
		msg := jira4claude.ErrorMessage(err)
		assert.Contains(t, msg, "description")
		assert.Contains(t, msg, "HTMLBlock")
		assert.Contains(t, msg, "Image")
		assert.NotContains(t, msg, "emoji")
		assert.Contains(t, msg, "--force")
	})

	t.Run("protected-node tokens say to put the token back", func(t *testing.T) {
		t.Parallel()
		issues := []jira4claude.ConversionIssue{
			{Kind: jira4claude.ConversionDropped, Node: "{{mention:2 Jane Doe}}"},
			{Kind: jira4claude.ConversionDropped, Node: "Image"},
		}

		msg := jira4claude.ErrorMessage(jira4claude.DroppedContentError("description", issues))

		assert.Contains(t, msg, "{{mention:2 Jane Doe}}")
		assert.Contains(t, msg, "put each token back")
		assert.Contains(t, msg, "Image")
		assert.Contains(t, msg, "--force")
	})

	t.Run("only tokens: no advice to remove content", func(t *testing.T) {
		t.Parallel()
		issues := []jira4claude.ConversionIssue{
			{Kind: jira4claude.ConversionDropped, Node: "{{mediaSingle:1 shot.png}}"},
		}

		msg := jira4claude.ErrorMessage(jira4claude.DroppedContentError("description", issues))

		assert.Contains(t, msg, "{{mediaSingle:1 shot.png}}")
		assert.Contains(t, msg, "put each token back")
		assert.Contains(t, msg, "--force")
		assert.NotContains(t, msg, "Remove it")
	})
}

func TestDroppedIssues(t *testing.T) {
	t.Parallel()

	issues := []jira4claude.ConversionIssue{
		{Kind: jira4claude.ConversionDropped, Node: "Image"},
		{Kind: jira4claude.ConversionDegraded, Node: "emoji"},
	}

	assert.Equal(t, []jira4claude.ConversionIssue{issues[0]}, jira4claude.DroppedIssues(issues))
	assert.Nil(t, jira4claude.DroppedIssues(issues[1:]))
}
