package jira4claude_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func paragraph(text string) jira4claude.ADFNode {
	return jira4claude.ADFNode{
		Type:    "paragraph",
		Content: []jira4claude.ADFNode{{Type: "text", Text: text}},
	}
}

func TestDescriptionUpdate(t *testing.T) {
	t.Parallel()

	t.Run("zero value keeps the description", func(t *testing.T) {
		t.Parallel()

		var u jira4claude.DescriptionUpdate

		assert.Equal(t, jira4claude.DescriptionKeep, u.Mode())
		assert.Nil(t, u.Document())
	})

	t.Run("set writes the given document", func(t *testing.T) {
		t.Parallel()

		doc := &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{paragraph("new")}}

		u := jira4claude.SetDescription(doc)

		assert.Equal(t, jira4claude.DescriptionSet, u.Mode())
		assert.Equal(t, doc, u.Document())
	})

	t.Run("clear writes no document", func(t *testing.T) {
		t.Parallel()

		u := jira4claude.ClearDescription()

		assert.Equal(t, jira4claude.DescriptionClear, u.Mode())
		assert.Nil(t, u.Document())
	})

	t.Run("append puts the new nodes after the current ones", func(t *testing.T) {
		t.Parallel()

		media := jira4claude.ADFNode{
			Type:  "mediaSingle",
			Attrs: json.RawMessage(`{"layout":"center"}`),
			Content: []jira4claude.ADFNode{{
				Type:  "media",
				Attrs: json.RawMessage(`{"id":"abc","type":"file","collection":"c"}`),
			}},
		}
		current := &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{paragraph("old"), media}}
		addition := &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{paragraph("new")}}

		u := jira4claude.AppendDescription(current, addition)

		assert.Equal(t, jira4claude.DescriptionAppend, u.Mode())
		doc := u.Document()
		require.NotNil(t, doc)
		assert.Equal(t, "doc", doc.Type)
		assert.Equal(t, 1, doc.Version)
		assert.Equal(t, []jira4claude.ADFNode{paragraph("old"), media, paragraph("new")}, doc.Content)
	})

	t.Run("append does not modify the current document", func(t *testing.T) {
		t.Parallel()

		currentContent := make([]jira4claude.ADFNode, 1, 4) // spare capacity invites aliasing
		currentContent[0] = paragraph("old")
		current := &jira4claude.ADFNode{Type: "doc", Version: 1, Content: currentContent}
		addition := &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{paragraph("new")}}

		doc := jira4claude.AppendDescription(current, addition).Document()
		doc.Content[0] = paragraph("changed")

		assert.Equal(t, []jira4claude.ADFNode{paragraph("old")}, current.Content)
	})

	t.Run("append to no description behaves like set", func(t *testing.T) {
		t.Parallel()

		addition := &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{paragraph("new")}}

		u := jira4claude.AppendDescription(nil, addition)

		assert.Equal(t, jira4claude.DescriptionSet, u.Mode())
		assert.Equal(t, addition, u.Document())
	})
}

func TestParseTimestamp(t *testing.T) {
	t.Parallel()

	want := time.Date(2026, 11, 9, 10, 15, 30, 123_000_000, time.UTC)

	for _, tc := range []struct{ name, value string }{
		{"the form issue view --json prints", "2026-11-09T10:15:30.123Z"},
		{"RFC 3339 with an offset", "2026-11-09T11:15:30.123+01:00"},
		{"the form Jira returns", "2026-11-09T10:15:30.123+0000"},
	} {
		t.Run("accepts "+tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := jira4claude.ParseTimestamp(tc.value)

			require.NoError(t, err)
			assert.True(t, want.Equal(got), "got %v", got)
		})
	}

	t.Run("accepts whole seconds", func(t *testing.T) {
		t.Parallel()

		got, err := jira4claude.ParseTimestamp("2026-11-09T10:15:30Z")

		require.NoError(t, err)
		assert.True(t, time.Date(2026, 11, 9, 10, 15, 30, 0, time.UTC).Equal(got))
	})

	for _, value := range []string{"", "yesterday", "2026-11-09", "2026-11-09 10:15:30"} {
		t.Run("rejects "+value, func(t *testing.T) {
			t.Parallel()

			_, err := jira4claude.ParseTimestamp(value)

			require.Error(t, err)
			assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
			assert.Contains(t, err.Error(), "--if-updated")
		})
	}
}

func TestFormatTimestamp(t *testing.T) {
	t.Parallel()

	t.Run("keeps milliseconds, even when they are zero", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "2026-11-09T10:15:30.120Z",
			jira4claude.FormatTimestamp(time.Date(2026, 11, 9, 10, 15, 30, 120_000_000, time.UTC)))
		assert.Equal(t, "2026-11-09T10:15:30.000Z",
			jira4claude.FormatTimestamp(time.Date(2026, 11, 9, 10, 15, 30, 0, time.UTC)))
	})

	t.Run("reads back with ParseTimestamp", func(t *testing.T) {
		t.Parallel()

		ts := time.Date(2026, 11, 9, 10, 15, 30, 7_000_000, time.FixedZone("", 3600))

		got, err := jira4claude.ParseTimestamp(jira4claude.FormatTimestamp(ts))

		require.NoError(t, err)
		assert.True(t, ts.Equal(got))
	})
}

func TestCheckUpdated(t *testing.T) {
	t.Parallel()

	updated := time.Date(2026, 11, 9, 10, 15, 30, 123_000_000, time.UTC)
	issue := &jira4claude.Issue{Key: "TEST-1", Updated: updated}

	t.Run("passes when the instants are equal", func(t *testing.T) {
		t.Parallel()

		assert.NoError(t, jira4claude.CheckUpdated(issue, updated.In(time.FixedZone("", 3600))))
	})

	t.Run("compares at millisecond precision", func(t *testing.T) {
		t.Parallel()

		assert.NoError(t, jira4claude.CheckUpdated(issue, updated.Add(999*time.Microsecond)))
	})

	t.Run("a different time is a conflict that names both times", func(t *testing.T) {
		t.Parallel()

		err := jira4claude.CheckUpdated(issue, updated.Add(-time.Millisecond))

		require.Error(t, err)
		assert.Equal(t, jira4claude.EConflict, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "TEST-1")
		assert.Contains(t, err.Error(), "2026-11-09T10:15:30.123Z")
		assert.Contains(t, err.Error(), "2026-11-09T10:15:30.122Z")
		assert.Contains(t, err.Error(), "Nothing was written")
	})

	t.Run("an issue without an updated time cannot be checked", func(t *testing.T) {
		t.Parallel()

		err := jira4claude.CheckUpdated(&jira4claude.Issue{Key: "TEST-1"}, updated)

		require.Error(t, err)
		assert.NotEqual(t, jira4claude.EConflict, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "Nothing was written")
	})
}

// reattachingConverter is a converter whose Reattach checks the original it
// is given and returns the given result.
func reattachingConverter(t *testing.T, wantOriginal, result *jira4claude.ADFNode, issues []jira4claude.ConversionIssue) *mock.Converter {
	t.Helper()
	return &mock.Converter{
		ReattachFn: func(original, edited *jira4claude.ADFNode) (*jira4claude.ADFNode, []jira4claude.ConversionIssue) {
			assert.Equal(t, wantOriginal, original)
			return result, issues
		},
	}
}

func TestReplaceDescription(t *testing.T) {
	t.Parallel()

	current := &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{paragraph("old")}}
	edited := &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{paragraph("new")}}
	reattached := &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{paragraph("new"), paragraph("kept")}}
	lost := jira4claude.ConversionIssue{Kind: jira4claude.ConversionDropped, Node: "{{media:1 shot.png}}", Detail: "missing"}
	changed := jira4claude.ConversionIssue{Kind: jira4claude.ConversionDegraded, Node: "emoji", Detail: "as text"}

	t.Run("returns the re-attached document", func(t *testing.T) {
		t.Parallel()

		conv := reattachingConverter(t, current, reattached, []jira4claude.ConversionIssue{changed})

		doc, issues, err := jira4claude.ReplaceDescription(conv, current, edited, false)

		require.NoError(t, err)
		assert.Equal(t, reattached, doc)
		assert.Equal(t, []jira4claude.ConversionIssue{changed}, issues)
	})

	t.Run("refuses when an existing node would be lost", func(t *testing.T) {
		t.Parallel()

		conv := reattachingConverter(t, current, reattached, []jira4claude.ConversionIssue{lost})

		doc, _, err := jira4claude.ReplaceDescription(conv, current, edited, false)

		require.Error(t, err)
		assert.Nil(t, doc)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "{{media:1 shot.png}}")
	})

	t.Run("force returns the re-attached document and the loss", func(t *testing.T) {
		t.Parallel()

		conv := reattachingConverter(t, current, reattached, []jira4claude.ConversionIssue{lost})

		doc, issues, err := jira4claude.ReplaceDescription(conv, current, edited, true)

		require.NoError(t, err)
		assert.Equal(t, reattached, doc)
		assert.Equal(t, []jira4claude.ConversionIssue{lost}, issues)
	})
}

func TestLiteralTokens(t *testing.T) {
	t.Parallel()

	doc := &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{paragraph("see {{media:1 shot.png}}")}}

	t.Run("token text with no node to restore is a degraded warning", func(t *testing.T) {
		t.Parallel()

		conv := reattachingConverter(t, nil, doc, []jira4claude.ConversionIssue{{
			Kind:   jira4claude.ConversionDropped,
			Node:   "{{media:1 shot.png}}",
			Detail: "the token names no protected node of the original document, so it is left as text",
		}})

		issues := jira4claude.LiteralTokens(conv, doc)

		require.Len(t, issues, 1)
		assert.Equal(t, jira4claude.ConversionDegraded, issues[0].Kind)
		assert.Equal(t, "{{media:1 shot.png}}", issues[0].Node)
		assert.Contains(t, issues[0].Detail, "plain text")
	})

	t.Run("text without tokens has no issues", func(t *testing.T) {
		t.Parallel()

		conv := reattachingConverter(t, nil, doc, nil)

		assert.Empty(t, jira4claude.LiteralTokens(conv, doc))
	})
}

func TestCheckClearDescription(t *testing.T) {
	t.Parallel()

	current := &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{paragraph("old")}}
	lost := jira4claude.ConversionIssue{
		Kind:   jira4claude.ConversionDropped,
		Node:   "{{mention:1 Jane Doe}}",
		Detail: "the token is not in the new text, so the mention would be removed",
	}

	t.Run("a description without protected nodes clears", func(t *testing.T) {
		t.Parallel()

		conv := reattachingConverter(t, current, &jira4claude.ADFNode{Type: "doc"}, nil)

		issues, err := jira4claude.CheckClearDescription(conv, current, false)

		require.NoError(t, err)
		assert.Empty(t, issues)
	})

	t.Run("no description clears without converting", func(t *testing.T) {
		t.Parallel()

		// A zero-value mock panics on any call.
		issues, err := jira4claude.CheckClearDescription(&mock.Converter{}, nil, false)

		require.NoError(t, err)
		assert.Empty(t, issues)
	})

	t.Run("protected nodes refuse and are named", func(t *testing.T) {
		t.Parallel()

		conv := reattachingConverter(t, current, &jira4claude.ADFNode{Type: "doc"}, []jira4claude.ConversionIssue{lost})

		_, err := jira4claude.CheckClearDescription(conv, current, false)

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "{{mention:1 Jane Doe}}")
		assert.Contains(t, err.Error(), "--force")
		assert.NotContains(t, err.Error(), "put each token back")
	})

	t.Run("force reports each removed node", func(t *testing.T) {
		t.Parallel()

		conv := reattachingConverter(t, current, &jira4claude.ADFNode{Type: "doc"}, []jira4claude.ConversionIssue{lost})

		issues, err := jira4claude.CheckClearDescription(conv, current, true)

		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, jira4claude.ConversionDropped, issues[0].Kind)
		assert.Equal(t, "{{mention:1 Jane Doe}}", issues[0].Node)
		assert.Contains(t, issues[0].Detail, "cleared")
	})
}
