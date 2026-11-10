package jira4claude_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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

func TestSafeFilename(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("a", 300) + ".txt"
	longMultibyte := strings.Repeat("é", 200) // 400 bytes

	tests := []struct {
		name     string
		filename string
		id       string
		want     string
	}{
		{"plain name is kept", "report.pdf", "10000", "report.pdf"},
		{"spaces and unicode are kept", "débug log.txt", "10000", "débug log.txt"},
		{"traversal keeps the base name", "../../etc/passwd", "10000", "passwd"},
		{"absolute path keeps the base name", "/abs/path", "10000", "path"},
		{"subdirectory keeps the base name", "a/b.txt", "10000", "b.txt"},
		{"windows separators", `..\x`, "10000", "x"},
		{"windows absolute path", `C:\Users\me\evil.bat`, "10000", "evil.bat"},
		{"mixed separators", `a\b/c\d.txt`, "10000", "d.txt"},
		{"dot dot falls back", "..", "10000", "attachment-10000"},
		{"dot falls back", ".", "10000", "attachment-10000"},
		{"empty falls back", "", "10000", "attachment-10000"},
		{"only separators falls back", "///", "10000", "attachment-10000"},
		{"trailing separator falls back", "dir/", "10000", "attachment-10000"},
		{"trailing backslash falls back", `dir\`, "10000", "attachment-10000"},
		{"traversal ending in dot dot falls back", "a/../..", "10000", "attachment-10000"},
		{"NUL is replaced", "a\x00b.txt", "10000", "a_b.txt"},
		{"newline and tab are replaced", "a\nb\tc.txt", "10000", "a_b_c.txt"},
		{"DEL and C1 controls are replaced", "a\x7fb\u0085c", "10000", "a_b_c"},
		{"invalid UTF-8 is replaced", "a\xffb", "10000", "a_b"},
		{"right-to-left override is replaced", "evil\u202etxt.exe", "10000", "evil_txt.exe"},
		{"a lone NUL becomes a placeholder name", "\x00", "10000", "_"},
		{"hidden file is kept", ".bashrc", "10000", ".bashrc"},
		{"fallback sanitises a hostile id", "", "../1", "attachment-___1"},
		{"fallback with an empty id", "", "", "attachment"},
		{"long name is cut to 200 bytes keeping the extension", long, "10000", strings.Repeat("a", 196) + ".txt"},
		{"long multibyte name is cut on a rune boundary", longMultibyte, "10000", strings.Repeat("é", 100)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := jira4claude.SafeFilename(tc.filename, tc.id)

			assert.Equal(t, tc.want, got)
			assert.NotContains(t, got, "/")
			assert.NotContains(t, got, `\`)
			assert.NotEqual(t, "..", got)
			assert.NotEqual(t, ".", got)
			assert.NotEmpty(t, got)
			assert.LessOrEqual(t, len(got), 200)
			assert.True(t, utf8.ValidString(got))
		})
	}
}

func TestLocalNames(t *testing.T) {
	t.Parallel()

	t.Run("each attachment keeps its safe name", func(t *testing.T) {
		t.Parallel()

		names, err := jira4claude.LocalNames([]*jira4claude.Attachment{
			{ID: "1", Filename: "a.txt"},
			{ID: "2", Filename: "../b.txt"},
		})

		require.NoError(t, err)
		assert.Equal(t, []string{"a.txt", "b.txt"}, names)
	})

	t.Run("a repeated name gets the attachment id in every later copy", func(t *testing.T) {
		t.Parallel()

		names, err := jira4claude.LocalNames([]*jira4claude.Attachment{
			{ID: "10", Filename: "report.pdf"},
			{ID: "11", Filename: "x/report.pdf"},
			{ID: "12", Filename: "REPORT.PDF"},
			{ID: "13", Filename: "README"},
			{ID: "14", Filename: "readme"},
			{ID: "15", Filename: ".env"},
			{ID: "16", Filename: ".env"},
		})

		require.NoError(t, err)
		assert.Equal(t, []string{
			"report.pdf", "report-11.pdf", "REPORT-12.PDF",
			"README", "readme-14",
			".env", ".env-16",
		}, names)
	})

	t.Run("a clash that the id suffix cannot resolve is an error", func(t *testing.T) {
		t.Parallel()

		_, err := jira4claude.LocalNames([]*jira4claude.Attachment{
			{ID: "10", Filename: "a.txt"},
			{ID: "11", Filename: "a.txt"},
			{ID: "12", Filename: "a-11.txt"},
		})

		require.Error(t, err)
		assert.Equal(t, jira4claude.EConflict, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "11")
		assert.Contains(t, err.Error(), "12")
	})

	t.Run("the id suffix also fits a name cut to the length limit", func(t *testing.T) {
		t.Parallel()

		long := strings.Repeat("a", 300) + ".txt"
		names, err := jira4claude.LocalNames([]*jira4claude.Attachment{
			{ID: "10", Filename: long},
			{ID: "11", Filename: long},
		})

		require.NoError(t, err)
		require.Len(t, names, 2)
		assert.LessOrEqual(t, len(names[1]), 200)
		assert.True(t, strings.HasSuffix(names[1], "-11.txt"), "name: %q", names[1])
	})

	t.Run("no attachments gives no names", func(t *testing.T) {
		t.Parallel()

		names, err := jira4claude.LocalNames(nil)

		require.NoError(t, err)
		assert.Empty(t, names)
	})
}
