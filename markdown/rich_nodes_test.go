package markdown_test

import (
	"strings"
	"testing"

	jira4claude "github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Rich inline nodes (jir-fajf): status and date carry data their text does
// not (a lozenge's colour, a date's timestamp), so they are protected nodes
// and read as tokens. Emoji and inline cards keep their readable form; a
// write that turns them into text or plain links reports it as degraded,
// unless an inline card comes back in a shape that can only be the card.

const (
	statusJSON = `{"type":"status","attrs":{"text":"In Progress","color":"blue","localId":"s1","style":""}}`
	dateJSON   = `{"type":"date","attrs":{"timestamp":"1762732800000"}}`
	emojiJSON  = `{"type":"emoji","attrs":{"shortName":":smile:","id":"1f604","text":"😄"}}`
	cardURL    = "https://example.atlassian.net/browse/PROJ-1"
	cardJSON   = `{"type":"inlineCard","attrs":{"url":"` + cardURL + `"}}`
)

func linkText(text, href string) jira4claude.ADFNode {
	return jira4claude.ADFNode{Type: "text", Text: text, Marks: []jira4claude.ADFMark{
		{Type: "link", Attrs: []byte(`{"href":"` + href + `"}`)},
	}}
}

// degradedIssues returns the issues of kind degraded.
func degradedIssues(issues []jira4claude.ConversionIssue) []jira4claude.ConversionIssue {
	var out []jira4claude.ConversionIssue
	for _, i := range issues {
		if i.Kind == jira4claude.ConversionDegraded {
			out = append(out, i)
		}
	}
	return out
}

func TestRichNodes_UnchangedRewrite(t *testing.T) {
	t.Parallel()

	t.Run("a status reads as a token and comes back identical", func(t *testing.T) {
		t.Parallel()
		doc := docOf(paraOf(plainText("see "), rawNode(t, statusJSON), plainText(" now")))
		md, readIssues := markdown.New().ToMarkdown(doc)
		assert.Equal(t, "see {{status:1 In Progress}} now", md)
		assert.Empty(t, readIssues)
		_, got, issues := readEditWrite(t, doc, nil)
		assert.Empty(t, issues)
		assert.Equal(t, jsonOf(t, doc), jsonOf(t, got))
	})

	t.Run("a date reads as a token labelled with its UTC day and comes back identical", func(t *testing.T) {
		t.Parallel()
		doc := docOf(paraOf(plainText("due "), rawNode(t, dateJSON), plainText(".")))
		md, readIssues := markdown.New().ToMarkdown(doc)
		assert.Equal(t, "due {{date:1 2025-11-10}}.", md)
		assert.Empty(t, readIssues)
		_, got, issues := readEditWrite(t, doc, nil)
		assert.Empty(t, issues)
		assert.Equal(t, jsonOf(t, doc), jsonOf(t, got))
	})

	t.Run("an emoji reads as its text, quietly, and comes back while its paragraph is unchanged", func(t *testing.T) {
		t.Parallel()
		doc := docOf(paraOf(plainText("great "), rawNode(t, emojiJSON), plainText(" "), rawNode(t, emojiJSON)))
		md, readIssues := markdown.New().ToMarkdown(doc)
		assert.Equal(t, "great 😄 😄", md)
		assert.Empty(t, readIssues, "a read reports nothing")
		_, got, issues := readEditWrite(t, doc, nil)
		assert.Empty(t, issues)
		assert.Equal(t, jsonOf(t, doc), jsonOf(t, got))
	})

	t.Run("an edit of an emoji's paragraph turns it into text and reports it as degraded", func(t *testing.T) {
		t.Parallel()
		doc := docOf(paraOf(plainText("great "), rawNode(t, emojiJSON), plainText(" "), rawNode(t, emojiJSON)))
		_, got, issues := readEditWrite(t, doc, func(md string) string { return strings.Replace(md, "great", "good", 1) })
		assert.Equal(t, jsonOf(t, docOf(paraOf(plainText("good 😄 😄")))), jsonOf(t, got))
		require.Len(t, issues, 1)
		assert.Equal(t, jira4claude.ConversionDegraded, issues[0].Kind)
		assert.Equal(t, "emoji", issues[0].Node)
		assert.Contains(t, issues[0].Detail, "2 emoji become plain text")
		assert.True(t, issues[0].Existing)
		assert.Empty(t, issues[0].Token)
		assert.Empty(t, jira4claude.RemovedNodes(issues), "a degraded issue removes nothing")
	})

	t.Run("an inline card that comes back as its own link is restored", func(t *testing.T) {
		t.Parallel()
		doc := docOf(paraOf(plainText("see "), rawNode(t, cardJSON), plainText(" now")))
		md, readIssues := markdown.New().ToMarkdown(doc)
		assert.Equal(t, "see ["+cardURL+"]("+cardURL+") now", md)
		assert.Empty(t, readIssues)
		_, got, issues := readEditWrite(t, doc, nil)
		assert.Empty(t, issues)
		assert.Equal(t, jsonOf(t, doc), jsonOf(t, got))
	})
}

// editEveryLink edits the text after every link of md, so the paragraphs
// holding them are not the ones read.
func editEveryLink(md string) string {
	return strings.ReplaceAll(md, ")", ") now")
}

func TestRichNodes_InlineCardRestore(t *testing.T) {
	t.Parallel()

	card := func(t *testing.T) jira4claude.ADFNode { return rawNode(t, cardJSON) }

	t.Run("after edits around it", func(t *testing.T) {
		t.Parallel()
		doc := docOf(paraOf(plainText("see "), card(t), plainText(" now")))
		_, got, issues := readEditWrite(t, doc, func(md string) string {
			return strings.Replace(md, "now", "today", 1)
		})
		assert.Empty(t, issues)
		assert.Equal(t, jsonOf(t, docOf(paraOf(plainText("see "), card(t), plainText(" today")))), jsonOf(t, got))
	})

	t.Run("two cards with the same URL and attrs, both kept", func(t *testing.T) {
		t.Parallel()
		doc := docOf(paraOf(card(t), plainText(" and ")), paraOf(card(t)))
		_, got, issues := readEditWrite(t, doc, nil)
		assert.Empty(t, issues)
		assert.Equal(t, jsonOf(t, doc), jsonOf(t, got))
	})

	t.Run("in a heading and a list item", func(t *testing.T) {
		t.Parallel()
		doc := docOf(
			jira4claude.ADFNode{Type: "heading", Attrs: []byte(`{"level":2}`), Content: []jira4claude.ADFNode{card(t)}},
			rawNode(t, `{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[`+
				`{"type":"text","text":"x "},`+cardJSON+`]}]}]}`),
		)
		_, got, issues := readEditWrite(t, doc, nil)
		assert.Empty(t, issues)
		assert.Equal(t, jsonOf(t, doc), jsonOf(t, got))
	})

	t.Run("in a paragraph the edit leaves unchanged, whatever its link reads as", func(t *testing.T) {
		t.Parallel()
		// jir-h44x: a paragraph the edit does not touch comes back whole.
		doc := docOf(paraOf(card(t), card(t)), paraOf(card(t), plainText(" "), linkText(cardURL, cardURL)))
		_, got, issues := readEditWrite(t, doc, func(md string) string { return md + "\n\nnew" })
		assert.Empty(t, issues)
		assert.Equal(t, jsonOf(t, docOf(append(doc.Content, paraOf(plainText("new")))...)), jsonOf(t, got))
	})

	t.Run("not restored, and reported, when", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name  string
			doc   func(t *testing.T) *jira4claude.ADFNode
			edit  func(string) string
			count string
		}{
			{
				name: "its link text is changed",
				doc:  func(t *testing.T) *jira4claude.ADFNode { return docOf(paraOf(plainText("see "), card(t))) },
				edit: func(md string) string {
					return "see [the ticket](" + cardURL + ")"
				},
				count: "1 smart link becomes a plain link",
			},
			{
				name: "its URL is changed",
				doc:  func(t *testing.T) *jira4claude.ADFNode { return docOf(paraOf(plainText("see "), card(t))) },
				edit: func(md string) string {
					return strings.ReplaceAll(md, "PROJ-1", "PROJ-2")
				},
				count: "1 smart link becomes a plain link",
			},
			{
				name: "the original also holds a plain link written the same way",
				doc: func(t *testing.T) *jira4claude.ADFNode {
					return docOf(paraOf(card(t), plainText(" "), linkText(cardURL, cardURL)))
				},
				edit:  editEveryLink,
				count: "1 smart link becomes a plain link",
			},
			{
				name:  "adjacent cards read back as one link",
				doc:   func(t *testing.T) *jira4claude.ADFNode { return docOf(paraOf(card(t), card(t))) },
				edit:  editEveryLink,
				count: "2 smart links become plain links",
			},
			{
				name: "a copy of the link is added",
				doc:  func(t *testing.T) *jira4claude.ADFNode { return docOf(paraOf(plainText("see "), card(t))) },
				edit: func(md string) string {
					return md + "\n\nagain " + md
				},
				count: "1 smart link becomes a plain link",
			},
			{
				name: "cards with the same URL differ in attrs",
				doc: func(t *testing.T) *jira4claude.ADFNode {
					return docOf(paraOf(card(t), plainText(" ")),
						paraOf(rawNode(t, `{"type":"inlineCard","attrs":{"url":"`+cardURL+`","localId":"c2"}}`)))
				},
				edit:  editEveryLink,
				count: "2 smart links become plain links",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				doc := tt.doc(t)
				_, got, issues := readEditWrite(t, doc, tt.edit)
				assert.NotContains(t, jsonOf(t, got), `"inlineCard"`)
				require.Len(t, issues, 1)
				assert.Equal(t, jira4claude.ConversionDegraded, issues[0].Kind)
				assert.Equal(t, "inlineCard", issues[0].Node)
				assert.Contains(t, issues[0].Detail, tt.count)
				assert.True(t, issues[0].Existing)
			})
		}
	})
}

func TestRichNodes_StatusAndDateFollowTokenRules(t *testing.T) {
	t.Parallel()

	doc := docOf(paraOf(plainText("state "), rawNode(t, statusJSON), plainText(" due "), rawNode(t, dateJSON)))

	t.Run("a removed token is dropped and existing", func(t *testing.T) {
		t.Parallel()
		_, _, issues := readEditWrite(t, doc, func(md string) string {
			return strings.Replace(md, " due {{date:2 2025-11-10}}", "", 1)
		})
		require.Len(t, issues, 1)
		assert.Equal(t, jira4claude.ConversionDropped, issues[0].Kind)
		assert.Equal(t, "{{date:2 2025-11-10}}", issues[0].Node)
		assert.True(t, issues[0].Existing)
		assert.Equal(t, []string{"{{date:2 2025-11-10}}"}, jira4claude.RemovedNodes(issues))
	})

	t.Run("swapped tokens swap nodes", func(t *testing.T) {
		t.Parallel()
		_, got, issues := readEditWrite(t, doc, func(md string) string {
			return "{{date:2 x}} then {{status:1 y}}"
		})
		assert.Empty(t, issues)
		want := docOf(paraOf(rawNode(t, dateJSON), plainText(" then "), rawNode(t, statusJSON)))
		assert.Equal(t, jsonOf(t, want), jsonOf(t, got))
	})

	t.Run("Markdown read before the change has shifted numbers and is refused", func(t *testing.T) {
		t.Parallel()
		// v0.3.0 read a status as bold text, so the mention after it was
		// token 1; it is token 2 now, and 1 names the status.
		old := docOf(paraOf(rawNode(t, statusJSON), plainText(" "), mention("u1", "Jo")))
		conv := markdown.New()
		adf, _ := conv.ToADF("**In Progress** {{mention:1 Jo}}")
		_, issues := conv.Reattach(old, adf)
		dropped := jira4claude.DroppedIssues(issues)
		require.Len(t, dropped, 3)
		assert.Equal(t, "{{mention:1 Jo}}", dropped[0].Node, "wrong kind")
		assert.False(t, dropped[0].Existing)
		assert.Equal(t, "{{status:1 In Progress}}", dropped[1].Node)
		assert.Equal(t, "{{mention:2 Jo}}", dropped[2].Node)
	})

	t.Run("FindTokens sees them", func(t *testing.T) {
		t.Parallel()
		conv := markdown.New()
		adf, _ := conv.ToADF("{{status:1 x}} {{date:2 y}}")
		assert.Equal(t, []string{"{{status:1 x}}", "{{date:2 y}}"}, conv.FindTokens(adf))
	})
}

func TestRichNodes_Labels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		node string
		want string
	}{
		{"a status without text", `{"type":"status","attrs":{"text":"","color":"neutral"}}`, "{{status:1}}"},
		{"a status label is escaped", `{"type":"status","attrs":{"text":"*DONE* {x}","color":"green"}}`, `{{status:1 \*DONE\* x}}`},
		{"a date at midnight UTC", `{"type":"date","attrs":{"timestamp":"1712707200000"}}`, "{{date:1 2024-04-10}}"},
		{"a date late in the UTC day keeps its UTC day", `{"type":"date","attrs":{"timestamp":"1712793599999"}}`, "{{date:1 2024-04-10}}"},
		{"a date that does not parse is labelled with its value", `{"type":"date","attrs":{"timestamp":"tomorrow"}}`, "{{date:1 tomorrow}}"},
		{"a date out of the four-digit years is labelled with its value", `{"type":"date","attrs":{"timestamp":"999999999999999999"}}`, "{{date:1 999999999999999999}}"},
		{"a date whose timestamp is not a string has no label", `{"type":"date","attrs":{"timestamp":1712707200000}}`, "{{date:1}}"},
		{"a date without attrs has no label", `{"type":"date"}`, "{{date:1}}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := docOf(paraOf(rawNode(t, tt.node)))
			md, issues := markdown.New().ToMarkdown(doc)
			assert.Empty(t, issues)
			assert.Equal(t, tt.want, md)
			_, got, rIssues := readEditWrite(t, doc, nil)
			assert.Empty(t, rIssues)
			assert.Equal(t, jsonOf(t, doc), jsonOf(t, got))
		})
	}

	t.Run("an emoji with neither text nor short name is dropped on read", func(t *testing.T) {
		t.Parallel()
		doc := docOf(paraOf(plainText("a "), rawNode(t, `{"type":"emoji","attrs":{"id":"abc"}}`)))
		_, issues := markdown.New().ToMarkdown(doc)
		dropped := jira4claude.DroppedIssues(issues)
		require.Len(t, dropped, 1)
		assert.Equal(t, "emoji", dropped[0].Node)
	})
}

func TestRichNodes_Clear(t *testing.T) {
	t.Parallel()

	// A clear (no edited document) removes the emoji and cards with the
	// rest of the text; it reports the status and date it removes.
	doc := docOf(paraOf(rawNode(t, emojiJSON), rawNode(t, cardJSON), rawNode(t, statusJSON), rawNode(t, dateJSON)))
	_, issues := markdown.New().Reattach(doc, nil)
	assert.Empty(t, degradedIssues(issues))
	assert.Equal(t, []string{"{{status:1 In Progress}}", "{{date:2 2025-11-10}}"}, jira4claude.RemovedNodes(issues))
}

func TestRichNodes_EmojiInsideProtectedNodesAreNotCounted(t *testing.T) {
	t.Parallel()

	// An emoji in the content of a node the reader tokenises is restored
	// with that node, so the write changes nothing about it.
	ext := `{"type":"bodiedExtension","attrs":{"extensionType":"t","extensionKey":"k"},"content":[` +
		`{"type":"paragraph","content":[` + emojiJSON + `,` + cardJSON + `]}]}`
	doc := docOf(rawNode(t, ext))
	_, got, issues := readEditWrite(t, doc, nil)
	assert.Empty(t, degradedIssues(issues))
	assert.Empty(t, issues)
	assert.Equal(t, jsonOf(t, doc), jsonOf(t, got))
}
