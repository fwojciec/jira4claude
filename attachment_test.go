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

func TestToAttachmentViews(t *testing.T) {
	t.Parallel()

	t.Run("copies each attachment in order", func(t *testing.T) {
		t.Parallel()

		views := jira4claude.ToAttachmentViews([]*jira4claude.Attachment{
			{
				ID:       "10000",
				Filename: "picture.jpg",
				MimeType: "image/jpeg",
				Size:     23123,
				Author:   &jira4claude.User{DisplayName: "Mia Krystof"},
				Created:  time.Date(2022, 10, 6, 7, 32, 47, 0, time.UTC),
			},
			{ID: "10001", Filename: "log.txt", MimeType: "text/plain", Size: 2460},
		})

		assert.Equal(t, []jira4claude.AttachmentView{
			{
				ID:       "10000",
				Filename: "picture.jpg",
				MimeType: "image/jpeg",
				Size:     23123,
				Author:   "Mia Krystof",
				Created:  "2022-10-06T07:32:47Z",
			},
			{ID: "10001", Filename: "log.txt", MimeType: "text/plain", Size: 2460},
		}, views)
	})

	t.Run("an attachment without a created time has no created value", func(t *testing.T) {
		t.Parallel()

		views := jira4claude.ToAttachmentViews([]*jira4claude.Attachment{{ID: "1"}})

		require.Len(t, views, 1)
		assert.Empty(t, views[0].Created)
	})

	t.Run("no attachments is an empty, non-nil slice", func(t *testing.T) {
		t.Parallel()

		views := jira4claude.ToAttachmentViews(nil)

		assert.NotNil(t, views)
		assert.Empty(t, views)
	})
}

func TestAttachmentView_JSON(t *testing.T) {
	t.Parallel()

	t.Run("uses camelCase keys and the exact byte size", func(t *testing.T) {
		t.Parallel()

		data, err := json.Marshal(jira4claude.AttachmentView{
			ID:       "10000",
			Filename: "picture.jpg",
			MimeType: "image/jpeg",
			Size:     23123,
			Author:   "Mia Krystof",
			Created:  "2022-10-06T07:32:47Z",
		})

		require.NoError(t, err)
		assert.JSONEq(t, `{
			"id": "10000",
			"filename": "picture.jpg",
			"mimeType": "image/jpeg",
			"size": 23123,
			"author": "Mia Krystof",
			"created": "2022-10-06T07:32:47Z"
		}`, string(data))
	})

	t.Run("omits an unknown author and created time", func(t *testing.T) {
		t.Parallel()

		data, err := json.Marshal(jira4claude.AttachmentView{ID: "1", Filename: "a", MimeType: "text/plain", Size: 0})

		require.NoError(t, err)
		assert.JSONEq(t, `{"id":"1","filename":"a","mimeType":"text/plain","size":0}`, string(data))
	})
}

func TestToIssueView_Attachments(t *testing.T) {
	t.Parallel()

	conv := &mock.Converter{
		ToMarkdownFn: func(*jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) { return "", nil },
	}
	noWarn := func(jira4claude.ConversionIssue) {}

	t.Run("carries the issue's attachments", func(t *testing.T) {
		t.Parallel()

		issue := &jira4claude.Issue{
			Key:         "TEST-1",
			Attachments: []*jira4claude.Attachment{{ID: "10000", Filename: "picture.jpg", MimeType: "image/jpeg", Size: 23123}},
		}

		view := jira4claude.ToIssueView(issue, conv, noWarn, "")

		assert.Equal(t, []jira4claude.AttachmentView{
			{ID: "10000", Filename: "picture.jpg", MimeType: "image/jpeg", Size: 23123},
		}, view.Attachments)
	})

	t.Run("JSON has an attachments array when there are attachments", func(t *testing.T) {
		t.Parallel()

		issue := &jira4claude.Issue{
			Key:         "TEST-1",
			Attachments: []*jira4claude.Attachment{{ID: "10000", Filename: "picture.jpg", MimeType: "image/jpeg", Size: 23123}},
		}

		data, err := json.Marshal(jira4claude.ToIssueView(issue, conv, noWarn, ""))
		require.NoError(t, err)

		var got struct {
			Attachments []map[string]any `json:"attachments"`
		}
		require.NoError(t, json.Unmarshal(data, &got))
		require.Len(t, got.Attachments, 1)
		assert.Equal(t, "10000", got.Attachments[0]["id"])
		assert.InDelta(t, 23123, got.Attachments[0]["size"], 0)
	})

	t.Run("JSON omits attachments when there are none, as it does comments", func(t *testing.T) {
		t.Parallel()

		data, err := json.Marshal(jira4claude.ToIssueView(&jira4claude.Issue{Key: "TEST-1"}, conv, noWarn, ""))
		require.NoError(t, err)

		var got map[string]any
		require.NoError(t, json.Unmarshal(data, &got))
		assert.NotContains(t, got, "attachments")
		assert.NotContains(t, got, "comments")
	})
}
