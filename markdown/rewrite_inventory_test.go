package markdown_test

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	jira4claude "github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The rewrite inventory (jir-h44x): one schema-valid sample per node type,
// mark type and attribute of testdata/adf-schema.json. Each is read with
// ToMarkdown, written back through ToADF and Reattach, and compared with the
// original as JSON. A rewrite must give the original back, or report the
// difference on the write, naming the node or mark: degraded when the content
// is kept in another form, dropped (which refuses without --force) when
// content or behaviour is lost. A difference with no such report is a silent
// change, which fails.
//
// Each sample is rewritten three ways:
//
//   - unchanged: the Markdown as read;
//   - edited elsewhere: a paragraph appended after the sample;
//   - edited inside: the word Alpha in the sample's text replaced by Omega,
//     for samples that hold it as text Markdown can carry.

// Outcomes of one rewrite.
const (
	outcomeIdentical = "identical"
	outcomeDegraded  = "degraded"
	outcomeDropped   = "dropped"
	// outcomeSilent: the result differs and no issue names the feature.
	outcomeSilent = "silent"
	// outcomeNoisy: the result is identical, but the write reports issues.
	outcomeNoisy = "noisy"
)

// inventoryRow is one sample. feature is the node or mark type an issue must
// name when the rewrite differs; blocks is the document's content as JSON.
// want holds the outcome for each variant that applies, keyed by variant.
type inventoryRow struct {
	name    string
	feature string
	blocks  string
	want    map[string]string
}

// Rewrite variants.
const (
	variantUnchanged = "unchanged"
	variantElsewhere = "edited elsewhere"
	variantInside    = "edited inside"
)

func ident() map[string]string {
	return map[string]string{variantUnchanged: outcomeIdentical, variantElsewhere: outcomeIdentical, variantInside: outcomeIdentical}
}

// keptOnlyUnedited is the outcome of a feature Markdown cannot carry: it
// comes back while its own content is not edited, and an edit inside it is
// reported with the given kind.
func keptOnlyUnedited(kind string) map[string]string {
	return map[string]string{variantUnchanged: outcomeIdentical, variantElsewhere: outcomeIdentical, variantInside: kind}
}

// nestedExpandOutcome is the outcome of a nested expand. The write gives
// it back, but the writer reports that the nested <details> it read has no
// ADF form of its own (it cannot tell that Reattach restores it), so the
// rewrite is identical with a degraded warning the CLI prints.
func nestedExpandOutcome() map[string]string {
	return map[string]string{variantUnchanged: outcomeNoisy, variantElsewhere: outcomeNoisy, variantInside: outcomeDegraded}
}

const alpha = `{"type":"text","text":"Alpha beta"}`

func invPara(inline ...string) string {
	return `{"type":"paragraph","content":[` + strings.Join(inline, ",") + `]}`
}

func invText(marks ...string) string {
	return `{"type":"text","text":"Alpha","marks":[` + strings.Join(marks, ",") + `]}`
}

const (
	mediaFile     = `{"type":"media","attrs":{"type":"file","id":"m-1","collection":"c","alt":"shot","height":10,"width":20,"occurrenceKey":"k","localId":"md-1"}}`
	mediaExternal = `{"type":"media","attrs":{"type":"external","url":"https://img.test/a.png","alt":"a","height":1,"width":2,"localId":"md-2"}}`
	extAttrs      = `"extensionType":"com.atlassian.confluence.macro.core","extensionKey":"toc","parameters":{"macroParams":{}},"text":"Contents","layout":"default","localId":"x-1"`
)

func inventoryRows() []inventoryRow {
	return []inventoryRow{
		// Nodes, with every attribute they can carry.
		{"doc", "doc", invPara(alpha), ident()},
		{"paragraph with localId", "paragraph", `{"type":"paragraph","attrs":{"localId":"p-1"},"content":[` + alpha + `]}`, ident()},
		{"empty paragraph between paragraphs", "paragraph", invPara(alpha) + `,{"type":"paragraph"},` + invPara(`{"type":"text","text":"Gamma"}`), ident()},
		{"text", "text", invPara(alpha), ident()},
		{"heading", "heading", `{"type":"heading","attrs":{"level":3,"localId":"h-1"},"content":[` + alpha + `]}`, ident()},
		{"codeBlock", "codeBlock", `{"type":"codeBlock","attrs":{"language":"go","uniqueId":"u-1","localId":"c-1"},"content":[{"type":"text","text":"Alpha := 1"}]}`, ident()},
		{"codeBlock language with a blank", "codeBlock", `{"type":"codeBlock","attrs":{"language":"go x"},"content":[{"type":"text","text":"Alpha"}]}`, ident()},
		{"blockquote", "blockquote", `{"type":"blockquote","attrs":{"localId":"q-1"},"content":[` + invPara(alpha) + `]}`, ident()},
		{"bulletList", "bulletList", `{"type":"bulletList","attrs":{"localId":"b-1"},"content":[{"type":"listItem","content":[` + invPara(alpha) + `]}]}`, ident()},
		{"listItem", "listItem", `{"type":"bulletList","content":[{"type":"listItem","attrs":{"localId":"li-1"},"content":[` + invPara(alpha) + `]}]}`, ident()},
		{"orderedList order 3", "orderedList", `{"type":"orderedList","attrs":{"order":3,"localId":"o-1"},"content":[{"type":"listItem","content":[` + invPara(alpha) + `]}]}`, ident()},
		{"orderedList order 1", "orderedList", `{"type":"orderedList","attrs":{"order":1},"content":[{"type":"listItem","content":[` + invPara(alpha) + `]}]}`, ident()},
		{"taskList and taskItem", "taskItem", `{"type":"taskList","attrs":{"localId":"tl-1"},"content":[{"type":"taskItem","attrs":{"localId":"ti-1","state":"DONE"},"content":[` + alpha + `]}]}`, ident()},
		{"blockTaskItem", "blockTaskItem", `{"type":"taskList","attrs":{"localId":"tl-1"},"content":[{"type":"blockTaskItem","attrs":{"localId":"bt-1","state":"TODO"},"content":[` + invPara(alpha) + `]}]}`, keptOnlyUnedited(outcomeDegraded)},
		{"decisionList", "decisionList", `{"type":"decisionList","attrs":{"localId":"dl-1"},"content":[{"type":"decisionItem","attrs":{"localId":"di-1","state":"DECIDED"},"content":[` + alpha + `]}]}`, keptOnlyUnedited(outcomeDegraded)},
		{"panel info", "panel", `{"type":"panel","attrs":{"panelType":"info"},"content":[` + invPara(alpha) + `]}`, ident()},
		{"panel tip", "panel", `{"type":"panel","attrs":{"panelType":"tip"},"content":[` + invPara(alpha) + `]}`, ident()},
		{"panel custom", "panel", `{"type":"panel","attrs":{"panelType":"custom","panelIcon":":star:","panelIconId":"2b50","panelIconText":"⭐","panelColor":"#ABF5D1","localId":"pn-1"},"content":[` + invPara(alpha) + `]}`, ident()},
		{"expand", "expand", `{"type":"expand","attrs":{"title":"Outer title","localId":"e-1"},"content":[` + invPara(alpha) + `]}`, ident()},
		{"expand without attrs", "expand", `{"type":"expand","content":[` + invPara(alpha) + `]}`, ident()},
		{"expand title with markup characters", "expand", `{"type":"expand","attrs":{"title":"a <b> & *c*"},"content":[` + invPara(alpha) + `]}`, ident()},
		{"nestedExpand", "nestedExpand", `{"type":"expand","attrs":{"title":"Outer"},"content":[{"type":"nestedExpand","attrs":{"title":"Inner","localId":"ne-1"},"content":[` + invPara(alpha) + `]}]}`, nestedExpandOutcome()},
		{"rule", "rule", invPara(alpha) + `,{"type":"rule","attrs":{"localId":"r-1"}}`, ident()},
		{"hardBreak", "hardBreak", invPara(alpha, `{"type":"hardBreak","attrs":{"text":"\n"}}`, `{"type":"text","text":"Gamma"}`), ident()},
		{"localId on an inline node", "localId", invPara(alpha, `{"type":"hardBreak","attrs":{"localId":"hb-1"}}`, `{"type":"text","text":"Gamma"}`), keptOnlyUnedited(outcomeDegraded)},
		{"hardBreak at the end of a paragraph", "hardBreak", invPara(alpha, `{"type":"hardBreak"}`), keptOnlyUnedited(outcomeDegraded)},
		{"hardBreak in a heading", "hardBreak", `{"type":"heading","attrs":{"level":1},"content":[` + alpha + `,{"type":"hardBreak"},{"type":"text","text":"Gamma"}]}`, keptOnlyUnedited(outcomeDegraded)},
		{"mention", "mention", invPara(alpha, `{"type":"mention","attrs":{"id":"u-1","localId":"mn-1","text":"@Ann","accessLevel":"CONTAINER","userType":"DEFAULT"}}`), ident()},
		{"emoji", "emoji", invPara(alpha, `{"type":"emoji","attrs":{"shortName":":smile:","id":"1f604","text":"😄","localId":"em-1"}}`), keptOnlyUnedited(outcomeDegraded)},
		{"inlineCard with url", "inlineCard", invPara(`{"type":"text","text":"Alpha beta "}`, `{"type":"inlineCard","attrs":{"url":"https://a.test/x","localId":"ic-1"}}`), ident()},
		{"inlineCard with data", "inlineCard", invPara(alpha, `{"type":"inlineCard","attrs":{"data":{"@type":"Document","name":"x"},"localId":"ic-2"}}`), keptOnlyUnedited(outcomeDropped)},
		{"blockCard with url", "blockCard", `{"type":"blockCard","attrs":{"url":"https://a.test/b","localId":"bc-1"}}`, ident()},
		{"blockCard with datasource", "blockCard", `{"type":"blockCard","attrs":{"url":"https://a.test/d","datasource":{"id":"ds","parameters":{},"views":[{"type":"table","properties":{}}]},"width":50,"layout":"wide","localId":"bc-2"}}`, ident()},
		{"blockCard with data", "blockCard", `{"type":"blockCard","attrs":{"data":{"name":"x"},"localId":"bc-3"}}`, ident()},
		{"embedCard", "embedCard", `{"type":"embedCard","attrs":{"url":"https://a.test/e","layout":"center","width":80,"originalHeight":100,"originalWidth":200,"localId":"ec-1"}}`, ident()},
		{"status", "status", invPara(alpha, `{"type":"status","attrs":{"text":"Done","color":"green","localId":"st-1","style":"bold"}}`), ident()},
		{"date", "date", invPara(alpha, `{"type":"date","attrs":{"timestamp":"1762732800000","localId":"dt-1"}}`), ident()},
		{"placeholder", "placeholder", invPara(alpha, `{"type":"placeholder","attrs":{"text":"Type here","localId":"ph-1"}}`), ident()},
		{"media file in mediaSingle", "mediaSingle", `{"type":"mediaSingle","attrs":{"layout":"center","width":50,"widthType":"percentage","localId":"ms-1"},"content":[` + mediaFile + `]}`, ident()},
		{"media external in mediaSingle", "mediaSingle", `{"type":"mediaSingle","attrs":{"layout":"wide","width":300,"widthType":"pixel"},"content":[` + mediaExternal + `]}`, ident()},
		{"caption", "mediaSingle", `{"type":"mediaSingle","attrs":{"layout":"center"},"content":[` + mediaFile + `,{"type":"caption","attrs":{"localId":"cp-1"},"content":[` + alpha + `]}]}`, ident()},
		{"mediaGroup", "mediaGroup", `{"type":"mediaGroup","content":[` + mediaFile + `]}`, ident()},
		{"mediaInline", "mediaInline", invPara(alpha, `{"type":"mediaInline","attrs":{"type":"file","id":"mi-1","collection":"c","alt":"r","occurrenceKey":"k","width":1,"height":2,"data":{},"localId":"mi-1"}}`), ident()},
		{"table", "table", `{"type":"table","attrs":{"displayMode":"fixed","isNumberColumnEnabled":true,"layout":"wide","localId":"t-1","width":760},"content":[` +
			`{"type":"tableRow","content":[{"type":"tableHeader","content":[` + invPara(`{"type":"text","text":"H"}`) + `]}]},` +
			`{"type":"tableRow","content":[{"type":"tableCell","content":[` + invPara(alpha) + `]}]}]}`, ident()},
		{"table without a header row", "table", `{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableCell","content":[` + invPara(alpha) + `]}]}]}`, keptOnlyUnedited(outcomeDegraded)},
		{"tableRow", "tableRow", `{"type":"table","content":[{"type":"tableRow","attrs":{"localId":"tr-1"},"content":[{"type":"tableHeader","content":[` + invPara(alpha) + `]}]}]}`, ident()},
		{"tableHeader attrs", "tableHeader", `{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableHeader","attrs":{"colspan":2,"rowspan":1,"colwidth":[100,200],"background":"#DEEBFF","localId":"th-1"},"content":[` + invPara(alpha) + `]}]},` +
			`{"type":"tableRow","content":[{"type":"tableCell","content":[` + invPara(`{"type":"text","text":"a"}`) + `]},{"type":"tableCell","content":[` + invPara(`{"type":"text","text":"b"}`) + `]}]}]}`, keptOnlyUnedited(outcomeDegraded)},
		{"tableCell attrs", "tableCell", `{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableHeader","content":[` + invPara(`{"type":"text","text":"H"}`) + `]}]},` +
			`{"type":"tableRow","content":[{"type":"tableCell","attrs":{"colspan":1,"rowspan":1,"colwidth":[120],"background":"#FFFAE6","localId":"td-1"},"content":[` + invPara(alpha) + `]}]}]}`, ident()},
		{"tableHeader in a later row", "tableRow", `{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableHeader","content":[` + invPara(`{"type":"text","text":"H"}`) + `]},{"type":"tableHeader","content":[` + invPara(`{"type":"text","text":"I"}`) + `]}]},` +
			`{"type":"tableRow","content":[{"type":"tableHeader","content":[` + invPara(alpha) + `]},{"type":"tableCell","content":[` + invPara(`{"type":"text","text":"x"}`) + `]}]}]}`, keptOnlyUnedited(outcomeDegraded)},
		{"tableCell with several blocks", "tableCell", `{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableHeader","content":[` + invPara(`{"type":"text","text":"H"}`) + `]}]},` +
			`{"type":"tableRow","content":[{"type":"tableCell","content":[` + invPara(alpha) + `,` + invPara(`{"type":"text","text":"Gamma"}`) +
			`,{"type":"bulletList","content":[{"type":"listItem","content":[` + invPara(`{"type":"text","text":"Delta"}`) + `]}]}]}]}]}`, keptOnlyUnedited(outcomeDegraded)},
		{"layoutSection", "layoutSection", `{"type":"layoutSection","attrs":{"localId":"ls-1"},"content":[` +
			`{"type":"layoutColumn","attrs":{"width":50,"localId":"lc-1"},"content":[` + invPara(alpha) + `]},` +
			`{"type":"layoutColumn","attrs":{"width":50},"content":[` + invPara(`{"type":"text","text":"Gamma"}`) + `]}]}`, keptOnlyUnedited(outcomeDegraded)},
		{"extension", "extension", `{"type":"extension","attrs":{` + extAttrs + `}}`, ident()},
		{"bodiedExtension", "bodiedExtension", `{"type":"bodiedExtension","attrs":{` + extAttrs + `},"content":[` + invPara(alpha) + `]}`, ident()},
		{"inlineExtension", "inlineExtension", invPara(alpha, `{"type":"inlineExtension","attrs":{"extensionType":"com.atlassian.jira","extensionKey":"issue","parameters":{},"text":"x","localId":"ie-1"}}`), ident()},
		{"syncBlock", "syncBlock", `{"type":"syncBlock","attrs":{"resourceId":"r","localId":"sb-1"}}`, ident()},
		{"bodiedSyncBlock", "bodiedSyncBlock", `{"type":"bodiedSyncBlock","attrs":{"resourceId":"r","localId":"bsb-1"},"content":[` + invPara(alpha) + `]}`, ident()},

		// Marks, with their attributes.
		{"alignment on a paragraph", "alignment", `{"type":"paragraph","marks":[{"type":"alignment","attrs":{"align":"center"}}],"content":[` + alpha + `]}`, ident()},
		{"alignment on a heading", "alignment", `{"type":"heading","attrs":{"level":2},"marks":[{"type":"alignment","attrs":{"align":"end"}}],"content":[` + alpha + `]}`, ident()},
		{"indentation", "indentation", `{"type":"paragraph","marks":[{"type":"indentation","attrs":{"level":2}}],"content":[` + alpha + `]}`, ident()},
		{"breakout on a codeBlock", "breakout", `{"type":"codeBlock","marks":[{"type":"breakout","attrs":{"mode":"wide","width":900}}],"content":[{"type":"text","text":"Alpha"}]}`, ident()},
		{"breakout on an expand", "breakout", `{"type":"expand","marks":[{"type":"breakout","attrs":{"mode":"full-width"}}],"content":[` + invPara(alpha) + `]}`, ident()},
		{"breakout on a layoutSection", "layoutSection", `{"type":"layoutSection","marks":[{"type":"breakout","attrs":{"mode":"wide"}}],"content":[` +
			`{"type":"layoutColumn","attrs":{"width":50},"content":[` + invPara(alpha) + `]},` +
			`{"type":"layoutColumn","attrs":{"width":50},"content":[` + invPara(`{"type":"text","text":"Gamma"}`) + `]}]}`, keptOnlyUnedited(outcomeDegraded)},
		{"annotation", "annotation", invPara(invText(`{"type":"annotation","attrs":{"id":"a-1","annotationType":"inlineComment"}}`), `{"type":"text","text":" beta"}`), keptOnlyUnedited(outcomeDropped)},
		{"backgroundColor", "backgroundColor", invPara(invText(`{"type":"backgroundColor","attrs":{"color":"#fedec8"}}`), `{"type":"text","text":" beta"}`), keptOnlyUnedited(outcomeDegraded)},
		{"textColor", "textColor", invPara(invText(`{"type":"textColor","attrs":{"color":"#ff5630"}}`), `{"type":"text","text":" beta"}`), keptOnlyUnedited(outcomeDegraded)},
		{"border on media", "mediaSingle", `{"type":"mediaSingle","attrs":{"layout":"center"},"content":[{"type":"media","attrs":{"type":"file","id":"m","collection":"c"},"marks":[{"type":"border","attrs":{"size":2,"color":"#091e4224"}}]}]}`, ident()},
		{"dataConsumer on an extension", "extension", `{"type":"extension","attrs":{"extensionType":"t","extensionKey":"k"},"marks":[{"type":"dataConsumer","attrs":{"sources":["s"]}}]}`, ident()},
		{"fragment on a table", "fragment", `{"type":"table","marks":[{"type":"fragment","attrs":{"localId":"f-1","name":"part"}}],"content":[{"type":"tableRow","content":[{"type":"tableHeader","content":[` + invPara(alpha) + `]}]}]}`, ident()},
		{"code", "code", invPara(invText(`{"type":"code"}`)), ident()},
		{"code holding a line break", "text", invPara(`{"type":"text","text":"Alpha\nbeta","marks":[{"type":"code"}]}`), keptOnlyUnedited(outcomeDegraded)},
		{"em", "em", invPara(invText(`{"type":"em"}`)), ident()},
		{"strong", "strong", invPara(invText(`{"type":"strong"}`)), ident()},
		{"strike", "strike", invPara(invText(`{"type":"strike"}`)), ident()},
		{"underline", "underline", invPara(invText(`{"type":"underline"}`)), ident()},
		{"subsup sub", "subsup", invPara(invText(`{"type":"subsup","attrs":{"type":"sub"}}`)), ident()},
		{"subsup sup", "subsup", invPara(invText(`{"type":"subsup","attrs":{"type":"sup"}}`)), ident()},
		{"marks out of canonical order", "strong", invPara(invText(`{"type":"strong"}`, `{"type":"em"}`)), ident()},
		{"link", "link", invPara(invText(`{"type":"link","attrs":{"href":"https://a.test/l"}}`)), ident()},
		{"link with title, id, collection and occurrenceKey", "link", invPara(invText(`{"type":"link","attrs":{"href":"https://a.test/l","title":"T","id":"l-1","collection":"c","occurrenceKey":"k"}}`)), keptOnlyUnedited(outcomeDegraded)},
		{"link on a mediaSingle", "mediaSingle", `{"type":"mediaSingle","attrs":{"layout":"center"},"marks":[{"type":"link","attrs":{"href":"https://a.test"}}],"content":[` + mediaFile + `]}`, ident()},
		{"text with a control character", "text", invPara(`{"type":"text","text":"Alpha\u0001beta\rgamma"}`), keptOnlyUnedited(outcomeDegraded)},
	}
}

// inventoryResult is one rewrite of one row.
type inventoryResult struct {
	outcome string
	// readOnly: the read named the feature, the write did not.
	readOnly bool
	md       string
	issues   []jira4claude.ConversionIssue
	got      string
	want     string
}

// rewriteRow rewrites a row's document as the variant says. It reports ok
// false for a variant that does not apply.
func rewriteRow(t *testing.T, row inventoryRow, variant string) (inventoryResult, bool) {
	t.Helper()
	data := `{"type":"doc","version":1,"content":[` + row.blocks + `]}`
	var doc jira4claude.ADFNode
	require.NoError(t, json.Unmarshal([]byte(data), &doc))
	require.NoError(t, validateADF(&doc), "the sample must be schema-valid")

	conv := markdown.New(markdown.WithLocalIDs(func(int) string { return "written-id" }))
	md, readIssues := conv.ToMarkdown(&doc)
	expected := doc
	edited := md
	switch variant {
	case variantElsewhere:
		edited = md + "\n\nAppended paragraph."
		expected.Content = append(append([]jira4claude.ADFNode{}, doc.Content...),
			jira4claude.ADFNode{Type: "paragraph", Content: []jira4claude.ADFNode{{Type: "text", Text: "Appended paragraph."}}})
	case variantInside:
		if strings.Count(md, "Alpha") != 1 {
			return inventoryResult{}, false
		}
		edited = strings.Replace(md, "Alpha", "Omega", 1)
		var copied jira4claude.ADFNode
		require.NoError(t, json.Unmarshal([]byte(data), &copied))
		if !replaceText(&copied, "Alpha", "Omega") {
			return inventoryResult{}, false
		}
		expected = copied
	}
	adf, writeIssues := conv.ToADF(edited)
	got, issues := conv.Reattach(&doc, adf)
	issues = append(issues, writeIssues...)

	require.NoError(t, validateADF(got), "%s: the result must be schema-valid\nmarkdown: %q", variant, edited)
	res := inventoryResult{md: md, issues: issues, got: comparable(t, got), want: comparable(t, &expected)}
	switch {
	case res.got == res.want && len(issues) == 0:
		res.outcome = outcomeIdentical
	case res.got == res.want:
		res.outcome = outcomeNoisy
	default:
		res.outcome = outcomeSilent
		for _, i := range issues {
			if !namesFeature(i, row.feature) {
				continue
			}
			if i.Kind == jira4claude.ConversionDropped {
				res.outcome = outcomeDropped
				break
			}
			res.outcome = outcomeDegraded
		}
		if res.outcome == outcomeSilent {
			for _, i := range readIssues {
				res.readOnly = res.readOnly || namesFeature(i, row.feature)
			}
		}
	}
	return res, true
}

// replaceText replaces the first old in the text nodes of n, outside nodes
// the reader writes as tokens, and reports whether it did.
func replaceText(n *jira4claude.ADFNode, old, replacement string) bool {
	switch n.Type {
	case "mention", "status", "date", "mediaSingle", "mediaGroup", "mediaInline", "media",
		"extension", "bodiedExtension", "inlineExtension", "syncBlock", "bodiedSyncBlock",
		"blockCard", "embedCard", "placeholder":
		return false
	case "text":
		if strings.Contains(n.Text, old) {
			n.Text = strings.Replace(n.Text, old, replacement, 1)
			return true
		}
		return false
	}
	for i := range n.Content {
		if replaceText(&n.Content[i], old, replacement) {
			return true
		}
	}
	return false
}

// namesFeature reports whether an issue names the node or mark type
// feature, directly or as the kind of its token.
func namesFeature(i jira4claude.ConversionIssue, feature string) bool {
	if i.Node == feature {
		return true
	}
	kind := strings.TrimLeft(strings.TrimPrefix(i.Node, "{{"), "!")
	return strings.HasPrefix(i.Node, "{{") && strings.HasPrefix(kind, feature+":")
}

// comparable returns doc as JSON for comparison, with three differences
// that change nothing in Jira excused: the marks of a text node are compared
// in type order, neighbouring text nodes with the same marks are compared as
// one, and a hardBreak's attribute text "\n" (the only value the schema
// allows) is compared as no attribute. A rewritten paragraph holds its marks
// in canonical order, its text merged and its hardBreaks bare, whatever form
// Jira sent.
func comparable(t *testing.T, doc *jira4claude.ADFNode) string {
	t.Helper()
	return jsonCanon(t, normalizeText(*doc))
}

func normalizeText(n jira4claude.ADFNode) jira4claude.ADFNode {
	if n.Type == "hardBreak" && string(n.Attrs) == `{"text":"\n"}` {
		n.Attrs = nil
	}
	if len(n.Marks) > 0 && n.Type == "text" {
		marks := append([]jira4claude.ADFMark{}, n.Marks...)
		sort.SliceStable(marks, func(i, j int) bool { return marks[i].Type < marks[j].Type })
		n.Marks = marks
	}
	if n.Content == nil {
		return n
	}
	content := make([]jira4claude.ADFNode, 0, len(n.Content))
	for _, c := range n.Content {
		c = normalizeText(c)
		if last := len(content) - 1; last >= 0 && c.Type == "text" && content[last].Type == "text" &&
			jsonOfMarks(content[last].Marks) == jsonOfMarks(c.Marks) {
			content[last].Text += c.Text
			continue
		}
		content = append(content, c)
	}
	n.Content = content
	return n
}

func jsonOfMarks(marks []jira4claude.ADFMark) string {
	data, _ := json.Marshal(marks)
	return string(data)
}

// jsonCanon returns v as JSON with object keys sorted, so attributes compare
// by value and not by key order.
func jsonCanon(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	var generic any
	require.NoError(t, json.Unmarshal(data, &generic))
	out, err := json.Marshal(generic)
	require.NoError(t, err)
	return string(out)
}

func TestRewriteInventory(t *testing.T) {
	t.Parallel()

	for _, row := range inventoryRows() {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			for _, variant := range []string{variantUnchanged, variantElsewhere, variantInside} {
				res, ok := rewriteRow(t, row, variant)
				if !ok {
					continue
				}
				if os.Getenv("J4C_INVENTORY") != "" {
					fmt.Printf("INVENTORY %-50s | %-16s | %-9s | readOnly=%v\n", row.name, variant, res.outcome, res.readOnly)
				}
				assert.NotEqual(t, outcomeSilent, res.outcome,
					"%s: the rewrite differs and no issue names %s\nmarkdown: %q\nissues: %v\nwant: %s\ngot:  %s",
					variant, row.feature, res.md, issueStrings(res.issues), res.want, res.got)
				if want, ok := row.want[variant]; ok {
					assert.Equal(t, want, res.outcome, "%s\nmarkdown: %q\nissues: %v\nwant: %s\ngot:  %s",
						variant, res.md, issueStrings(res.issues), res.want, res.got)
				}
			}
		})
	}
}

// TestRewriteInventory_CoversTheSchema checks that the inventory holds a row
// for every node and mark type of the schema, and a sample of every
// attribute they can carry, so a schema update cannot leave one untested.
func TestRewriteInventory_CoversTheSchema(t *testing.T) {
	t.Parallel()

	types, attrs := schemaInventory(t)
	sampled := make(map[string]bool)
	for _, row := range inventoryRows() {
		var doc jira4claude.ADFNode
		require.NoError(t, json.Unmarshal([]byte(`{"type":"doc","version":1,"content":[`+row.blocks+`]}`), &doc), row.name)
		collectSampled(&doc, sampled)
	}

	var missing []string
	for typ := range types {
		if !sampled[typ] {
			missing = append(missing, "type "+typ)
		}
	}
	for attr := range attrs {
		if !sampled[attr] {
			missing = append(missing, "attribute "+attr)
		}
	}
	sort.Strings(missing)
	assert.Empty(t, missing, "the inventory has no sample of these")
}

// collectSampled records the node and mark types and the type.attribute
// pairs present in n.
func collectSampled(n *jira4claude.ADFNode, sampled map[string]bool) {
	sampled[n.Type] = true
	addAttrKeys(n.Type, n.Attrs, sampled)
	for _, m := range n.Marks {
		sampled[m.Type] = true
		addAttrKeys(m.Type, m.Attrs, sampled)
	}
	for i := range n.Content {
		collectSampled(&n.Content[i], sampled)
	}
}

func addAttrKeys(typ string, raw json.RawMessage, sampled map[string]bool) {
	if raw == nil {
		return
	}
	var attrs map[string]json.RawMessage
	if json.Unmarshal(raw, &attrs) != nil {
		return
	}
	for k := range attrs {
		sampled[typ+"."+k] = true
	}
}

// schemaInventory returns the node and mark types of the schema and their
// attributes as type.attribute, from every definition that names a type,
// including each alternative of an anyOf.
func schemaInventory(t *testing.T) (map[string]bool, map[string]bool) {
	t.Helper()
	var schema struct {
		Definitions map[string]struct {
			Properties struct {
				Type struct {
					Enum []string `json:"enum"`
				} `json:"type"`
				Attrs struct {
					Properties map[string]json.RawMessage `json:"properties"`
					AnyOf      []struct {
						Properties map[string]json.RawMessage `json:"properties"`
					} `json:"anyOf"`
				} `json:"attrs"`
			} `json:"properties"`
		} `json:"definitions"`
	}
	require.NoError(t, json.Unmarshal(adfSchemaJSON, &schema))
	types := make(map[string]bool)
	attrs := make(map[string]bool)
	for _, def := range schema.Definitions {
		for _, typ := range def.Properties.Type.Enum {
			types[typ] = true
			for k := range def.Properties.Attrs.Properties {
				attrs[typ+"."+k] = true
			}
			for _, alt := range def.Properties.Attrs.AnyOf {
				for k := range alt.Properties {
					attrs[typ+"."+k] = true
				}
			}
		}
	}
	require.NotEmpty(t, types)
	return types, attrs
}
