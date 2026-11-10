package markdown_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jira4claude "github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/require"
)

// Converter properties (#147).
//
// P1, schema validity: for any Markdown input, ToADF gives ADF that passes
// the embedded ADF schema.
//
// P2, fixed point: Markdown -> ADF (adf1) -> Markdown -> ADF (adf2) gives
// adf1 == adf2, compared as the JSON that Jira receives, with a fixed localId
// source. A difference is excused only when one of the two conversions in
// between reports an issue (dropped or degraded, any node):
//
//   - the read, adf1 -> Markdown: the reader saying "this does not round-trip";
//   - the second write, Markdown -> adf2: the writer saying it could not carry
//     the reader's Markdown over.
//
// Issues from the first write (Markdown -> adf1) never excuse a difference:
// they describe how the input became adf1, not whether adf1 round-trips.
// Silence plus a difference is a bug. Even when a difference is excused, adf2
// must still pass the schema.
//
// Limit of the rule: issues carry no position and are reported once per
// (kind, node type), so any one issue excuses the whole document.
//
// Both properties run as ordinary tests on the seed corpus. Inputs that fail
// today are not seeds; they are in TestPropertyKnownFailures.

// propertySeeds are the P1 and P2 seed inputs, besides testdata/*.md. They
// come from the closed "Write Safety" issues and the shapes named in the
// comments on #147 that pass today.
var propertySeeds = []string{
	// #138 soft breaks
	"line one\nline two",
	"*em*\nnext",
	"[link](https://example.com)\nnext",
	"`code`\nnext",
	// #139 single tildes
	"5~10 and 20~30",
	"a ~b~ c",
	"~~gone~~",
	`a \~ b`,
	// #140 inline HTML
	"pass <word> as the argument",
	"stray </u> close",
	"x <u>a never closed",
	"a <u>b <sub>c</u> d</sub> e",
	"<u>u</u> <sub>s</sub> <sup>p</sup>",
	"> [!NOTE]\n> <u>x</u> y",
	"- [ ] <u>x</u> y",
	// #146 nested details
	"<details>\n<summary>Outer</summary>\n\nouter body\n\n<details>\n<summary>Inner</summary>\n\ninner body\n\n</details>\n\ntail\n\n</details>\n\nafter",
	"<details>\n<summary>Open</summary>\n\nbody",
	// #152 hard break after an inline element
	"**bold**  \nnext",
	"**bold**\\\nnext",
	"[link](https://example.com)  \nnext",
	"`code`\\\nnext",
	// #153 escapes and entities
	`2 \* 3 and snake\_case and \[x\]`,
	"a &amp; b",
	"`a \\* &amp; b`",
	"```\n\\* &amp;\n```",
	`\# not a heading`,
	`a \\ b`,
	// #154 indented code and rules
	"    code\n    more",
	"- item\n\n      code",
	"a\n\n---\n\nb",
	// #155 block HTML
	"<word>",
	"<div>\nsome\ntext\n</div>",
	"<!-- <details> -->\n\n<!--\n</details>\n-->",
	// #156 block content in task items
	"- [ ] task\n\n  ```\n  code\n  ```",
	"- [ ] task\n\n  second paragraph",
	"- [ ] task\n  - nested bullet",
	"- [ ] a\n  - [ ] b",
	// #157 container children on read
	"1. item\n\n   second para",
	"10. item\n\n    second para",
	"> quote\n>\n> <word>\n>\n> **text**",
	"- <div>\n  a\n  </div>\n- next",
	"> ![img](x.png)",
	"- item\n\n  ```\n  code\n  ```",
	"- item\n  - nested",
	// #158 adjacent lists and empty list items
	"+ a\n* b",
	"- a\n  - b\n  + c",
	"- [ ] a\n  - [ ] b\n  + [ ] c",
	"+ [X] a\n* b",
	"* 0)\n  0",
	"+ * *",
	// #147 comment shapes
	"a  \nb",
	"**a.**a",
	"a | b\\\n---|---",
	"[a](b%20c)",
	"[a](b\\)c)",
	"- a\n  - [ ] b",
	"- a\n\n  - [ ] b",
	"- item\n\n  | a | b |\n  |---|---|\n  |   | x |",
	"> [!NOTE]\n> ![x](y.png)",
	// #149 text that looks like protected-node tokens stays text
	"{{mention:1 x}}",
	"a {{!mention:1 x}} b {{mediaInline:2}}",
	"{{mediaSingle:1 shot.png}}\n\n{{futureBlock:2}}",
	"`{{mention:1 x}}` and **{{mention:2 y}}**",
	// Schema-invalid output found and fixed in #147
	"*",
	"* *",
	"```",
	"|\n|-",
	"<details><summary></summary>\n\n</details>",
	"> [!NOTE]",
	"- > [!NOTE]",
	// #160 code spans and code blocks
	"`` a`b ``",
	"` `` `",
	"` `",
	"````\n```",
	"~~~0`",
	"~~~0\t 0",
	"`\n`\n-",
	// #159 marks: edge whitespace, adjacent runs, struck tildes, duplicate
	// and reordered marks
	"**a.**&#97;",
	"<u>a </u>b",
	"*_a_*",
	"**__a__**",
	"**_0_**",
	"~~&#126;~~",
	"~~a&#126;~~",
	"**bold&#32;**&#97;fter",
	"befor&#101;*&#32;italic*",
	"**a**_b_**c**",
	"~~a~~*~~b~~*",
	"~~a~~&#126;b",
	"x**~~a~~**y",
	"**&#104;ttps://x.test/&#32;**",
	"*&#119;ww.x.test/&#9;*b",
	"~~&#104;ttps://x.test/&#126;~~",
	"~~&#104;ttps://x.test/~~&#126;",
	"see &#104;ttps://x.tes&#116;**.a**",
	"see &#104;ttps://x.test/~~a~~",
	"**https://x.test/.**&#98;",
	"1\t1&\\*-a@b.co <u>***-***</u>",
	"**a**[*https://x.test/\\_*](https://x.test/_)",
	"~~a~~[https://x.test/a(](<https://x.test/a(>)",
	// #162 line endings, control characters, invalid UTF-8, two escapes
	"\r#\r0",
	"&#X1;",
	"&#Xd;",
	"a&#13;&#10;b",
	"a&#10;# b",
	"*0 []()\x81*",
	"<A>\n\\ ",
	"\\\n\\|-",
	"a\\\n\\-:",
	"\\|- \n|- ",
	"<https://x.com/a>&#10;b",
	"<https://x.com/a>&#92;\\\nb",
	"<https://x.com/a>[b](https://example.com)",
	"<http://www.x.com/a>;[;](https://example.com)",
	"[https://x.com/a&#10;b](https://example.com)",
	"a@b.co&#92;[;](https://example.com)",
	"<https://x.com/b>\\\\. <http://www.x.com/a@b.co>\\\\~~;~~",
	"&#120;**&#10;a**",
	"**a&#10;**&#120;",
	// #163 blank-only code lines in a list item
	"- item\n\n  ```\n   \n  ```",
	"> - ```\n>     \n>   ```",
	"- ```\n\t \n  ```",
	// #161 links
	"[a](<b c>)",
	"[0](( )",
	"[0000]()",
	"[]()",
	"[a](a&amp;b)",
	"[a](<a\\>b>)",
	"[ http://x.com",
	"&#104;ttp://0.a0",
	"[ http://0.a0",
	"0\\!@0.0",
	"\\(https://0.a",
	"0@0\\.0",
	"a [0](0)",
	"<0@0>",
	"<A0:>",
	"[www.x.com](www.x.com)",
	"*https://0.a",
	"<A0>https://0.a",
	"a<https://x.test>",
	"<https://x.test>a",
	"<https://x.test/*>",
	"a\\![b](c)",
	"[a](x)[b](x)",
	"<https://x.co>`a`",
	"<a@b.co>`a`",
	"<https://x.co/a`b> `c`",
	"_a@b.co x_",
	"\\\\ http://x.co/a<a",
	// Found by fuzzing in #161.
	"0@0.0_",
	"* [X*0*]()",
}

// addPropertySeeds adds propertySeeds and the Markdown fixtures in testdata
// to the fuzz corpus.
func addPropertySeeds(f *testing.F) {
	f.Helper()

	for _, seed := range propertySeeds {
		f.Add(seed)
	}
	files, err := filepath.Glob(filepath.Join("testdata", "*.md"))
	require.NoError(f, err)
	require.NotEmpty(f, files)
	for _, file := range files {
		data, err := os.ReadFile(file)
		require.NoError(f, err)
		f.Add(string(data))
	}
}

// propertyConverter returns a converter with a constant localId source, so
// equal documents give equal ADF. A sequential source would make the ids
// depend on how many were generated for nodes the writer later dropped, which
// is not a difference in content.
func propertyConverter() *markdown.Converter {
	return markdown.New(markdown.WithLocalIDs(func(int) string { return "local-id" }))
}

// checkSchemaValid is P1. It returns a description of the failure, or "".
func checkSchemaValid(md string) string {
	adf, _ := propertyConverter().ToADF(md)
	if err := validateADF(adf); err != nil {
		return fmt.Sprintf("ToADF is not schema-valid\ninput: %q\nadf:   %s\n%s", md, mustJSON(adf), shortError(err))
	}
	return ""
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "marshal error: " + err.Error()
	}
	return string(data)
}

// shortError shortens schema errors, which list every alternative of an anyOf.
func shortError(err error) string {
	const n = 400
	s := err.Error()
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// checkFixedPoint is P2. It returns a description of the failure, or "".
func checkFixedPoint(md string) string {
	conv := propertyConverter()

	adf1, writeIssues := conv.ToADF(md)
	md1, readIssues := conv.ToMarkdown(adf1)
	adf2, rewriteIssues := conv.ToADF(md1)

	if err := validateADF(adf2); err != nil {
		return fmt.Sprintf("second write is not schema-valid\ninput: %q\nread:  %q\nadf2:  %s\n%s", md, md1, mustJSON(adf2), shortError(err))
	}

	json1, err := json.Marshal(adf1)
	if err != nil {
		return fmt.Sprintf("marshal adf1: %s", err)
	}
	json2, err := json.Marshal(adf2)
	if err != nil {
		return fmt.Sprintf("marshal adf2: %s", err)
	}
	if string(json1) == string(json2) {
		return ""
	}
	if len(readIssues) > 0 || len(rewriteIssues) > 0 {
		return ""
	}
	return fmt.Sprintf("Markdown -> ADF -> Markdown -> ADF is not a fixed point, and no issue was reported\n"+
		"input:        %q\n"+
		"write issues: %v (do not excuse)\n"+
		"read:         %q\n"+
		"adf1: %s\n"+
		"adf2: %s",
		md, issueStrings(writeIssues), md1, json1, json2)
}

func issueStrings(issues []jira4claude.ConversionIssue) []string {
	s := make([]string, 0, len(issues))
	for _, i := range issues {
		s = append(s, i.String())
	}
	return s
}

func FuzzToADFSchemaValid(f *testing.F) {
	addPropertySeeds(f)
	f.Fuzz(func(t *testing.T, md string) {
		t.Parallel()
		if failure := checkSchemaValid(md); failure != "" {
			t.Fatal(failure)
		}
	})
}

func FuzzToADFFixedPoint(f *testing.F) {
	addPropertySeeds(f)
	f.Fuzz(func(t *testing.T, md string) {
		t.Parallel()
		if failure := checkFixedPoint(md); failure != "" {
			t.Fatal(failure)
		}
	})
}

// P3, rewrites (#149, jir-fajf, jir-h44x): for an ADF document, read as
// Markdown, written back with ToADF and re-attached against the original:
//
//   - unchanged, the result is the original, identical, and nothing is
//     reported. Reattach puts back the original wherever the edited
//     document holds what the unchanged read writes back, so this holds by
//     construction, for any document, schema-valid or not;
//   - with a paragraph appended to the Markdown, the result is the original
//     plus that paragraph, or the write reports an issue. For a schema-valid
//     original the result is schema-valid, and when the write reports no
//     dropped node every protected node comes back unchanged and in order;
//   - with the first "e" of the Markdown changed, so a block is edited in
//     place and the original is merged into it, the same two hold
//     (checkEditedInPlace); the rest is the edit's and is not compared.
//
// Excused in the second comparison, as changing nothing in Jira: the order
// of a text node's marks, how text with the same marks is split into nodes,
// a hardBreak's attribute text "\n" (see comparable), and a document
// without a version, which the write gives the version 1 ADF requires. A difference is
// also excused by any issue at all, as in P2: issues carry no position.
// Reading itself never excuses one: a read that reports content as dropped
// is a view, and the write must still report or restore it. The whole-
// document identity of every fixture is TestReattach_FixtureIdentity; each
// node, mark and attribute is in TestRewriteInventory.
//
// The seeds are the ADF fixtures in testdata and reattachSeeds.

// reattachSeeds are ADF documents with protected nodes the fixtures lack.
var reattachSeeds = []string{
	`{"type":"doc","version":1,"content":[{"type":"extension","attrs":{"extensionType":"t","extensionKey":"toc"}},` +
		`{"type":"paragraph","content":[{"type":"text","text":"a "},{"type":"inlineExtension","attrs":{"extensionType":"t","extensionKey":"k"}}]}]}`,
	`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"{{mention:1 x}} "},` +
		`{"type":"mention","attrs":{"id":"u","text":"@Ann"}},{"type":"text","text":" {{!mention:1 Ann}}"}]}]}`,
	`{"type":"doc","version":1,"content":[{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableCell","content":[` +
		`{"type":"paragraph","content":[{"type":"text","text":"x"}]},{"type":"mediaSingle","content":[{"type":"media","attrs":{"id":"m","type":"file","collection":"c"}}]}]}]}]}]}`,
	// Found by fuzzing: an unknown block that is all of a task item.
	`{"type":"doc","version":1,"content":[{"type":"taskList","attrs":{"localId":"l"},"content":[{"type":"taskItem","attrs":{"localId":"i","state":"TODO"},"content":[{"type":"futureBlock"}]}]}]}`,
	// Status and date tokens (jir-fajf) in a heading, a table cell and a task
	// item, next to marks, links and each other, with literal token text.
	`{"type":"doc","version":1,"content":[{"type":"heading","attrs":{"level":1},"content":[` +
		`{"type":"status","attrs":{"text":"*DONE*","color":"green","localId":"s"}},{"type":"date","attrs":{"timestamp":"0"}}]},` +
		`{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"strong"}]},` +
		`{"type":"date","attrs":{"timestamp":"1762732800000"}},{"type":"text","text":"y {{status:1 a}}","marks":[{"type":"link","attrs":{"href":"https://a.test"}}]}]},` +
		`{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableCell","content":[` +
		`{"type":"paragraph","content":[{"type":"status","attrs":{"text":"Blocked","color":"red"}}]}]}]}]},` +
		`{"type":"taskList","attrs":{"localId":"l"},"content":[{"type":"taskItem","attrs":{"localId":"i","state":"TODO"},"content":[` +
		`{"type":"text","text":"due "},{"type":"date","attrs":{"timestamp":"x"}}]}]}]}`,
	// Emoji and inline cards beside links that read the same.
	`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[` +
		`{"type":"emoji","attrs":{"shortName":":a:","text":"😄"}},{"type":"inlineCard","attrs":{"url":"https://a.test"}},` +
		`{"type":"text","text":" "},{"type":"inlineCard","attrs":{"url":"https://a.test"}},{"type":"text","text":" "},` +
		`{"type":"text","text":"https://a.test","marks":[{"type":"link","attrs":{"href":"https://a.test"}}]}]},` +
		`{"type":"heading","attrs":{"level":2},"content":[{"type":"inlineCard","attrs":{"url":"https://b.test/x_y"}},` +
		`{"type":"emoji","attrs":{"shortName":":custom:"}}]}]}`,
	// Attributes and marks Markdown cannot show, a layout, a decision list
	// and an empty paragraph (jir-h44x).
	`{"type":"doc","version":1,"content":[{"type":"panel","attrs":{"panelType":"custom","panelColor":"#fff"},"content":[` +
		`{"type":"paragraph","content":[{"type":"text","text":"a","marks":[{"type":"textColor","attrs":{"color":"#ff0000"}}]}]}]},` +
		`{"type":"paragraph"},{"type":"layoutSection","content":[{"type":"layoutColumn","attrs":{"width":50},"content":[` +
		`{"type":"paragraph","content":[{"type":"text","text":"b"}]}]},{"type":"layoutColumn","attrs":{"width":50},"content":[` +
		`{"type":"decisionList","attrs":{"localId":"d"},"content":[{"type":"decisionItem","attrs":{"localId":"i","state":"DECIDED"},"content":[` +
		`{"type":"text","text":"c","marks":[{"type":"annotation","attrs":{"id":"x","annotationType":"inlineComment"}}]}]}]}]}]},` +
		`{"type":"table","attrs":{"layout":"wide","localId":"t"},"content":[{"type":"tableRow","content":[{"type":"tableCell","attrs":{"colspan":2},"content":[` +
		`{"type":"paragraph","content":[{"type":"text","text":"d"}]}]}]},{"type":"tableRow","content":[{"type":"tableCell","content":[` +
		`{"type":"paragraph"}]},{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"e"}]}]}]}]}]}`,
}

// appendedText is the paragraph P3 appends to the Markdown.
const appendedText = "Appended by the property."

func checkReattach(data []byte) string {
	var doc jira4claude.ADFNode
	if err := json.Unmarshal(data, &doc); err != nil || doc.Type != "doc" {
		// Only a document is a description; the writer always makes one.
		return ""
	}
	conv := propertyConverter()
	md, _ := conv.ToMarkdown(&doc)
	adf, _ := conv.ToADF(md)
	got, issues := conv.Reattach(&doc, adf)
	if mustJSON(got) != mustJSON(&doc) || len(issues) > 0 {
		return fmt.Sprintf("an unchanged rewrite does not give the original back, with nothing reported\n"+
			"input:  %s\nread:   %q\nissues: %v\nhave:   %s", data, md, issueStrings(issues), mustJSON(got))
	}

	edited, _ := conv.ToADF(md + "\n\n" + appendedText)
	if n := len(edited.Content); n == 0 || mustJSON(edited.Content[n-1]) != mustJSON(paraOf(plainText(appendedText))) {
		// The paragraph did not read back on its own (it continued a
		// construct of the read), so the edit is not the one P3 checks.
		return ""
	}
	got, issues = conv.Reattach(&doc, edited)
	if validateADF(&doc) == nil {
		// What a guarded update sends must be what Jira accepts.
		if err := validateADF(got); err != nil {
			return fmt.Sprintf("Reattach of a schema-valid document, edited, is not schema-valid\n"+
				"input: %s\nread:  %q\ngot:   %s\n%s", data, md, mustJSON(got), shortError(err))
		}
	}
	if len(jira4claude.DroppedIssues(issues)) == 0 && mustJSON(protectedNodes(&doc)) != mustJSON(protectedNodes(got)) {
		return fmt.Sprintf("protected nodes do not survive an edit elsewhere, and no loss is reported\n"+
			"input:  %s\nread:   %q\nissues: %v\nhave:   %s", data, md, issueStrings(issues), mustJSON(got))
	}
	if failure := checkEditedInPlace(&doc, md); failure != "" {
		return failure
	}
	want := doc
	want.Version = 1 // the writer always writes the version ADF requires
	want.Content = append(append([]jira4claude.ADFNode{}, doc.Content...), paraOf(plainText(appendedText)))
	if canonical(&want) != canonical(got) && len(issues) == 0 {
		return fmt.Sprintf("a rewrite with a paragraph appended changes the original, and nothing is reported\n"+
			"input: %s\nread:  %q\nwant:  %s\nhave:  %s", data, md, canonical(&want), canonical(got))
	}
	return ""
}

// checkEditedInPlace is the third leg of P3: the Markdown of doc, md, with
// its first "e" changed, so the block holding it is edited and Reattach
// merges the original into it. For a schema-valid doc the result is
// schema-valid, and the protected nodes come back unchanged and in order
// unless a dropped node is reported. What else the edit changes is the
// edit's, so it is not compared.
func checkEditedInPlace(doc *jira4claude.ADFNode, md string) string {
	if !strings.Contains(md, "e") {
		return ""
	}
	conv := propertyConverter()
	edited, _ := conv.ToADF(strings.Replace(md, "e", "E", 1))
	got, issues := conv.Reattach(doc, edited)
	if validateADF(doc) == nil {
		if err := validateADF(got); err != nil {
			return fmt.Sprintf("Reattach of a schema-valid document, edited in place, is not schema-valid\n"+
				"input: %s\nread:  %q\ngot:   %s\n%s", mustJSON(doc), md, mustJSON(got), shortError(err))
		}
	}
	if len(jira4claude.DroppedIssues(issues)) == 0 && mustJSON(protectedNodes(doc)) != mustJSON(protectedNodes(got)) {
		return fmt.Sprintf("protected nodes do not survive an edit in place, and no loss is reported\n"+
			"input:  %s\nread:   %q\nissues: %v\nhave:   %s", mustJSON(doc), md, issueStrings(issues), mustJSON(got))
	}
	return ""
}

// canonical is comparable for property checks: doc as JSON with the
// differences that change nothing in Jira left out.
func canonical(doc *jira4claude.ADFNode) string {
	data, err := json.Marshal(normalizeText(*doc))
	if err != nil {
		return "marshal error: " + err.Error()
	}
	var generic any
	if err := json.Unmarshal(data, &generic); err != nil {
		return "unmarshal error: " + err.Error()
	}
	out, _ := json.Marshal(generic)
	return string(out)
}

func FuzzReattachProtectedNodes(f *testing.F) {
	for _, seed := range reattachSeeds {
		f.Add([]byte(seed))
	}
	files, err := filepath.Glob(filepath.Join("testdata", "*.adf.json"))
	require.NoError(f, err)
	require.NotEmpty(f, files)
	for _, file := range files {
		data, err := os.ReadFile(file)
		require.NoError(f, err)
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		t.Parallel()
		if failure := checkReattach(data); failure != "" {
			t.Fatal(failure)
		}
	})
}

// TestPropertyKnownFailures holds minimised inputs that break a property
// today, found by fuzzing (#147). Each case is skipped and names its gap;
// when a gap is fixed, move its input from this table into propertySeeds so
// the fuzz targets check it. Set J4C_KNOWN_FAILURES=1 to run every case and
// see which still fail.
//
// "(read)" or "(write)" in a gap names the side believed to be at fault.
func TestPropertyKnownFailures(t *testing.T) {
	t.Parallel()

	// Every gap found so far is fixed; add new ones here, each named for its
	// cause.
	tests := []struct {
		name  string
		gap   string
		input string
	}{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if os.Getenv("J4C_KNOWN_FAILURES") == "" {
				t.Skip("known gap: " + tt.gap)
			}
			if failure := checkFixedPoint(tt.input); failure != "" {
				t.Error(failure)
			}
		})
	}
}
