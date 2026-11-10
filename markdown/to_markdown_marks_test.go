package markdown_test

import (
	"encoding/json"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mark shorthands for the #159 tests. Each is in canonical order.
func textNode(text string, marks ...string) jira4claude.ADFNode {
	n := jira4claude.ADFNode{Type: "text", Text: text}
	for _, m := range marks {
		switch m {
		case "link":
			n.Marks = append(n.Marks, jira4claude.ADFMark{Type: "link", Attrs: json.RawMessage(`{"href":"https://x.test"}`)})
		case "sub":
			n.Marks = append(n.Marks, jira4claude.ADFMark{Type: "subsup", Attrs: json.RawMessage(`{"type":"sub"}`)})
		default:
			n.Marks = append(n.Marks, jira4claude.ADFMark{Type: m})
		}
	}
	return n
}

// TestToMarkdown_MarkRoundTrip covers marked text whose Markdown delimiters
// meet whitespace, punctuation, tildes or other delimiters (#159). Each line
// must read back as the same text nodes with the same marks, with no issue.
func TestToMarkdown_MarkRoundTrip(t *testing.T) {
	t.Parallel()

	cases := map[string][]jira4claude.ADFNode{
		// Edge whitespace stays inside the mark.
		"strong with a trailing space":   {textNode("bold ", "strong"), textNode("after")},
		"em with a leading space":        {textNode("before"), textNode(" italic", "em")},
		"strong with both edge spaces":   {textNode(" bold text ", "strong")},
		"strong space alone":             {textNode(" ", "strong")},
		"strong space between letters":   {textNode("x"), textNode(" ", "strong"), textNode("y")},
		"strike with two spaces":         {textNode("x"), textNode("  ", "strike"), textNode("y")},
		"strong with a tab edge":         {textNode("a\t", "strong"), textNode("b")},
		"strong with a no-break space":   {textNode("a ", "strong"), textNode("b")},
		"strong-em with edge spaces":     {textNode("x"), textNode(" a ", "em", "strong"), textNode("y")},
		"strike-em with edge spaces":     {textNode("x"), textNode(" a ", "em", "strike"), textNode("y")},
		"underline with a trailing":      {textNode("a ", "underline"), textNode("b")},
		"underline with a leading space": {textNode(" a", "underline")},
		"sub with edge spaces":           {textNode("H"), textNode(" 2 ", "sub"), textNode("O")},
		"link with edge spaces":          {textNode("a"), textNode(" b ", "link"), textNode("c")},
		"underline space alone":          {textNode("a"), textNode(" ", "underline"), textNode("b")},
		"strong space after a break":     {textNode("a"), {Type: "hardBreak"}, textNode(" b", "strong")},
		"strong space before a break":    {textNode("a ", "strong"), {Type: "hardBreak"}, textNode("b")},
		"strong line of spaces at start": {textNode("  ", "strong"), textNode("a")},
		"underlined strong with spaces":  {textNode("x"), textNode(" a ", "strong", "underline"), textNode("y")},

		// Punctuation just inside a delimiter.
		"strong ending in punctuation":    {textNode("a.", "strong"), textNode("a")},
		"strong starting in punctuation":  {textNode("a"), textNode(".a", "strong")},
		"em of punctuation only":          {textNode("a"), textNode(".", "em"), textNode("b")},
		"strike ending in punctuation":    {textNode("a.", "strike"), textNode("b")},
		"strong-strike between letters":   {textNode("x"), textNode("a", "strong", "strike"), textNode("y")},
		"em-strike between letters":       {textNode("x"), textNode("a", "em", "strike"), textNode("y")},
		"strong ending in a symbol":       {textNode("a+", "strong"), textNode("b")},
		"strong in a word":                {textNode("x"), textNode("a", "strong"), textNode("y")},
		"strong ending in a star":         {textNode("a*", "strong"), textNode("b")},
		"em ending in an underscore":      {textNode("a_", "em"), textNode("b")},
		"strong ending in a backslash":    {textNode("a\\", "strong"), textNode("b")},
		"strong starting in an ampersand": {textNode("x"), textNode("&amp;", "strong")},

		// Adjacent marked runs.
		"strong then em":                {textNode("a", "strong"), textNode("b", "em")},
		"em then strong":                {textNode("a", "em"), textNode("b", "strong")},
		"strong then strong-em":         {textNode("a", "strong"), textNode("b", "em", "strong")},
		"strong-em then em":             {textNode("a", "em", "strong"), textNode("b", "em")},
		"em then em-strong then em":     {textNode("a", "em"), textNode("b", "em", "strong"), textNode("c", "em")},
		"three alternating runs":        {textNode("a", "strong"), textNode("b", "em"), textNode("c", "strong")},
		"strike then strike-em":         {textNode("a", "strike"), textNode("b", "em", "strike")},
		"strike-em then strike":         {textNode("a", "em", "strike"), textNode("b", "strike")},
		"strike-em then strike-strong":  {textNode("a", "em", "strike"), textNode("b", "strong", "strike")},
		"em then strike":                {textNode("a", "em"), textNode("b", "strike")},
		"punctuated runs":               {textNode("a.", "strong"), textNode(".b", "em")},
		"runs in a word":                {textNode("x"), textNode("a", "strong"), textNode("b", "em"), textNode("y")},
		"strong then underlined strong": {textNode("a", "strong"), textNode("b", "strong", "underline")},
		"strong then link strong":       {textNode("a", "strong"), textNode("b", "link", "strong")},
		"strong then code":              {textNode("a", "strong"), textNode("b", "code")},
		"spaced runs":                   {textNode("a ", "strong"), textNode(" b", "em")},
		"plain star then em":            {textNode("a*"), textNode("b", "em")},
		"em then plain star":            {textNode("a", "em"), textNode("*b")},
		"plain underscore then em":      {textNode("a_"), textNode("b", "em"), textNode("_c")},

		// Tildes at the edge of struck text, inside or outside.
		"struck tilde":                 {textNode("~", "strike")},
		"struck text ending in tilde":  {textNode("a~", "strike")},
		"struck text inside tildes":    {textNode("~a~", "strike")},
		"struck text ending in tildes": {textNode("a~~", "strike")},
		"struck tildes":                {textNode("~~~", "strike")},
		"plain tilde after strike":     {textNode("a", "strike"), textNode("~b")},
		"plain tilde before strike":    {textNode("b~"), textNode("a", "strike")},
		"plain tildes around strike":   {textNode("~~"), textNode("a", "strike"), textNode("~~")},
		"struck tilde in em":           {textNode("~", "em", "strike")},
		"em tilde then strike":         {textNode("a~", "em"), textNode("b", "strike")},

		// A link written as a bare URL cannot have its edge encoded, so it
		// is written in brackets instead.
		"bare URL then punctuated strong": {textNode("https://x.test", "link"), textNode(".a", "strong")},
		"punctuated strong then bare URL": {textNode("a.", "strong"), textNode("https://x.test", "link")},
		"bare URL then underscore em":     {textNode("a", "em"), textNode("b", "strong"), textNode("https://x.test", "link")},
		// Plain URL text after a delimiter has its first letter encoded,
		// which also keeps it from being linked.
		"punctuated strong then URL text": {textNode("a.", "strong"), textNode("https://x.test")},
	}

	for name, inline := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			conv := markdown.New()
			md, readIssues := conv.ToMarkdown(paragraphDoc(inline...))
			assert.Empty(t, readIssues, "markdown was %q", md)

			back, writeIssues := conv.ToADF(md)
			requireValidADF(t, back)
			assert.Empty(t, writeIssues, "markdown was %q", md)
			assert.Equal(t, paragraphDoc(inline...), back, "markdown was %q", md)
		})
	}
}

// TestToMarkdown_MarkForms pins the Markdown written for a few of the shapes
// in TestToMarkdown_MarkRoundTrip, so a change in form is a visible decision.
func TestToMarkdown_MarkForms(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		inline []jira4claude.ADFNode
		want   string
	}{
		// A blank at the edge of strong, em or strike is a character
		// reference inside the delimiters. A delimiter run with punctuation
		// on its inner side must have whitespace or punctuation on its outer
		// side, so a letter there is a character reference too.
		"trailing space":           {[]jira4claude.ADFNode{textNode("bold ", "strong"), textNode(" after")}, "**bold&#32;** after"},
		"trailing space, a letter": {[]jira4claude.ADFNode{textNode("bold ", "strong"), textNode("after")}, "**bold&#32;**&#97;fter"},
		"leading space":            {[]jira4claude.ADFNode{textNode("before "), textNode(" italic", "em")}, "before *&#32;italic*"},
		"space only":               {[]jira4claude.ADFNode{textNode(" ", "strong")}, "**&#32;**"},
		"punctuation, a letter":    {[]jira4claude.ADFNode{textNode("a.", "strong"), textNode("a")}, "**a.**&#97;"},
		// Tags and links have no flanking rules: their blanks stay as is.
		"underline trailing space": {[]jira4claude.ADFNode{textNode("a ", "underline"), textNode("b")}, "<u>a </u>b"},
		// Adjacent emphasis alternates "*" and "_" so the runs do not join.
		"strong then em":  {[]jira4claude.ADFNode{textNode("a", "strong"), textNode("b", "em")}, "**a**_b_"},
		"em then strong":  {[]jira4claude.ADFNode{textNode("a", "em"), textNode("b", "strong")}, "*a*__b__"},
		"three runs":      {[]jira4claude.ADFNode{textNode("a", "strong"), textNode("b", "em"), textNode("c", "strong")}, "**a**_b_**c**"},
		"emphasis around": {[]jira4claude.ADFNode{textNode("a", "strike"), textNode("b", "em", "strike")}, "~~a~~*~~b~~*"},
		// A tilde next to a strike delimiter is a character reference.
		"struck tilde": {[]jira4claude.ADFNode{textNode("a~", "strike")}, "~~a&#126;~~"},
		"plain tilde":  {[]jira4claude.ADFNode{textNode("a", "strike"), textNode("~b")}, "~~a~~&#126;b"},
		// Nothing changes where no rule applies.
		"plain words": {[]jira4claude.ADFNode{textNode("a "), textNode("b", "strong"), textNode(" c")}, "a **b** c"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			md, issues := markdown.New().ToMarkdown(paragraphDoc(tc.inline...))
			assert.Empty(t, issues)
			assert.Equal(t, tc.want, md)
		})
	}
}

// TestToMarkdown_MarkNextToURL covers URL text (not linked in ADF) right
// before a character reference or a delimiter run the read writes. Linkify
// would run the URL on into the reference or the delimiters, taking them as
// part of the link; instead the URL's first character is a reference, so the
// URL is not linked and the text reads back exactly (#159).
//
// Where no reference or delimiter meets the URL, URL text in marked text is
// still linked on write, as before (TestRoundTrip_BareURLInMarkedText); the
// comparison allows that, and nothing else.
func TestToMarkdown_MarkNextToURL(t *testing.T) {
	t.Parallel()

	cases := map[string][]jira4claude.ADFNode{
		"plain URL before punctuated strong": {textNode("see https://x.test"), textNode(".a", "strong")},
		"plain URL path before strike":       {textNode("see https://x.test/"), textNode("a", "strike")},
		"plain URL path before tilde strike": {textNode("see https://x.test/"), textNode("~a", "strike")},
		"plain www before strike":            {textNode("see www.x.test/"), textNode("a", "strike")},
		"struck URL then tilde":              {textNode("https://x.test/", "strike"), textNode("~")},
		"struck URL then letter":             {textNode("a."), textNode("https://x.test/", "strike"), textNode("b")},
		"struck URL ending in tilde":         {textNode("https://x.test/~", "strike")},
		"struck tilde then URL":              {textNode("~https://x.test/", "strike")},
		"email path ending in blank":         {textNode("a@b.co/ ", "strong")},
		"plain digit before em email":        {textNode("1"), textNode("a@b.co", "em")},
		"em email after em":                  {textNode("x", "em"), textNode("a@b.co")},
		"em star then email":                 {textNode("*a@b.co", "em")},
		"em digit then code email":           {textNode("1", "em"), textNode("a@b.co", "code")},
		"plain URL then em email":            {textNode("http://y.test1"), textNode("a@b.co", "em")},
		"plain URL ending the line in a tab": {textNode("a", "em"), textNode(" https://x.test/\t")},
		"code then plain URL then link":      {textNode("a", "code"), textNode("https://x.test/"), textNode("b", "link", "em")},
		"em parens then URL":                 {textNode("((https://x.test)", "em")},
		"plain email after a tab then em":    {textNode("1\t1&*-a@b.co "), textNode("-", "em", "strong", "underline")},
	}
	for _, mark := range []string{"strong", "em", "strike"} {
		cases[mark+" URL ending in blank"] = []jira4claude.ADFNode{textNode("https://x.test/ ", mark)}
		cases[mark+" URL ending in blank, then text"] = []jira4claude.ADFNode{textNode("https://x.test/ ", mark), textNode("b")}
		cases[mark+" URL ending in tab"] = []jira4claude.ADFNode{textNode("see https://x.test/\t", mark)}
		cases[mark+" www ending in blank"] = []jira4claude.ADFNode{textNode("www.x.test/ ", mark)}
		cases[mark+" blank then URL"] = []jira4claude.ADFNode{textNode("a"), textNode(" https://x.test/", mark), textNode("b")}
		cases[mark+" punctuated URL then letter"] = []jira4claude.ADFNode{textNode("https://x.test/.", mark), textNode("b")}
	}

	for name, inline := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			conv := markdown.New()
			md, readIssues := conv.ToMarkdown(paragraphDoc(inline...))
			assert.Empty(t, readIssues, "markdown was %q", md)

			back, writeIssues := conv.ToADF(md)
			requireValidADF(t, back)
			assert.Empty(t, writeIssues, "markdown was %q", md)
			require.Len(t, back.Content, 1, "markdown was %q", md)
			assert.Equal(t, inline, withoutAutolinks(back.Content[0].Content), "markdown was %q", md)
		})
	}
}

// withoutAutolinks drops link marks whose href is the text they mark (the
// links Linkify adds on write) and merges the text nodes that then have the
// same marks.
func withoutAutolinks(nodes []jira4claude.ADFNode) []jira4claude.ADFNode {
	var out []jira4claude.ADFNode
	for _, n := range nodes {
		var marks []jira4claude.ADFMark
		for _, m := range n.Marks {
			var attrs struct {
				Href string `json:"href"`
			}
			_ = json.Unmarshal(m.Attrs, &attrs)
			if m.Type == "link" && (attrs.Href == n.Text || attrs.Href == "http://"+n.Text) {
				continue
			}
			marks = append(marks, m)
		}
		n.Marks = marks
		if last := len(out) - 1; last >= 0 && n.Type == "text" && out[last].Type == "text" &&
			mustJSON(out[last].Marks) == mustJSON(marks) {
			out[last].Text += n.Text
			continue
		}
		out = append(out, n)
	}
	return out
}

// TestToMarkdown_MarkNextToInlineNode covers delimited text next to inline
// nodes. Cards and media are written in brackets, which fit any delimiter
// run; a mention is a plain label, so its edge letter is encoded like plain
// text (#159).
func TestToMarkdown_MarkNextToInlineNode(t *testing.T) {
	t.Parallel()

	card := jira4claude.ADFNode{Type: "inlineCard", Attrs: json.RawMessage(`{"url":"https://x.test"}`)}
	media := jira4claude.ADFNode{Type: "mediaInline", Attrs: json.RawMessage(`{"id":"1","type":"file"}`)}
	mention := jira4claude.ADFNode{Type: "mention", Attrs: json.RawMessage(`{"id":"1","text":"@Ann"}`)}

	cases := map[string]struct {
		inline []jira4claude.ADFNode
		strong string
	}{
		"strong then card":    {[]jira4claude.ADFNode{textNode("a.", "strong"), card}, "a."},
		"media then strong":   {[]jira4claude.ADFNode{media, textNode(".a", "strong")}, ".a"},
		"mention then strong": {[]jira4claude.ADFNode{mention, textNode(".a", "strong")}, ".a"},
		"strong then mention": {[]jira4claude.ADFNode{textNode("a.", "strong"), mention}, "a."},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			conv := markdown.New()
			md, _ := conv.ToMarkdown(paragraphDoc(tc.inline...))
			back, _ := conv.ToADF(md)
			require.Len(t, back.Content, 1)

			var strong []string
			for _, n := range back.Content[0].Content {
				if len(n.Marks) == 1 && n.Marks[0].Type == "strong" {
					strong = append(strong, n.Text)
				}
			}
			assert.Equal(t, []string{tc.strong}, strong, "markdown was %q", md)
		})
	}
}

// TestToMarkdown_MarkBackstopScope pins what the read-back backstop compares:
// strong, em and strike only. Underlined code loses its underline on write
// (code is exclusive), which is not a delimiter problem, so bold on the same
// line does not make it report.
func TestToMarkdown_MarkBackstopScope(t *testing.T) {
	t.Parallel()

	md, issues := markdown.New().ToMarkdown(paragraphDoc(textNode("a", "strong"), textNode("b", "underline", "code")))
	assert.Empty(t, issues, "markdown was %q", md)
}

// TestToMarkdown_MarkOrderFromJira covers marks from Jira in an order other
// than the writer's: equal mark sets still merge, and come back canonical.
func TestToMarkdown_MarkOrderFromJira(t *testing.T) {
	t.Parallel()

	conv := markdown.New()
	inline := []jira4claude.ADFNode{
		{Type: "text", Text: "a", Marks: []jira4claude.ADFMark{{Type: "strong"}, {Type: "em"}}},
		{Type: "text", Text: "b", Marks: []jira4claude.ADFMark{{Type: "em"}, {Type: "strong"}}},
	}
	md, issues := conv.ToMarkdown(paragraphDoc(inline...))
	assert.Empty(t, issues)

	back, _ := conv.ToADF(md)
	require.Len(t, back.Content, 1)
	assert.Equal(t, []jira4claude.ADFNode{textNode("ab", "em", "strong")}, back.Content[0].Content, "markdown was %q", md)
}

// TestToMarkdown_UnwrittenMarksMerge covers neighbours that differ only in
// marks Markdown does not carry (textColor): they are written as one run, so
// their delimiters do not meet ("~~a~~~~b~~").
func TestToMarkdown_UnwrittenMarksMerge(t *testing.T) {
	t.Parallel()

	color := jira4claude.ADFMark{Type: "textColor", Attrs: json.RawMessage(`{"color":"#ff0000"}`)}
	for _, mark := range []string{"strike", "em", "strong"} {
		t.Run(mark, func(t *testing.T) {
			t.Parallel()

			conv := markdown.New()
			colored := textNode("b", mark)
			colored.Marks = append(colored.Marks, color)
			md, issues := conv.ToMarkdown(paragraphDoc(textNode("a", mark), colored))
			assert.Empty(t, issues)

			back, _ := conv.ToADF(md)
			require.Len(t, back.Content, 1)
			assert.Equal(t, []jira4claude.ADFNode{textNode("ab", mark)}, back.Content[0].Content, "markdown was %q", md)
		})
	}

	// Text with only unwritten marks is plain text: blanks at the start or
	// end of the line are kept as for any plain text.
	for name, text := range map[string]string{"line end": "a \t", "line start": " \ta"} {
		t.Run("edge blanks at "+name, func(t *testing.T) {
			t.Parallel()

			conv := markdown.New()
			colored := textNode(text)
			colored.Marks = []jira4claude.ADFMark{color}
			md, issues := conv.ToMarkdown(paragraphDoc(colored))
			assert.Empty(t, issues)

			back, _ := conv.ToADF(md)
			assert.Equal(t, paragraphDoc(textNode(text)), back, "markdown was %q", md)
		})
	}
}
