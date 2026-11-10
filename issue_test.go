package jira4claude_test

import (
	"encoding/json"
	"testing"

	"github.com/fwojciec/jira4claude"
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
