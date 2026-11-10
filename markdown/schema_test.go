package markdown_test

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	jira4claude "github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/adf-schema.json
var adfSchemaJSON []byte

// compiledSchema is the ADF JSON Schema compiled once for all tests.
var compiledSchema = func() *jsonschema.Schema {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(adfSchemaJSON))
	if err != nil {
		panic("failed to unmarshal ADF schema: " + err.Error())
	}
	return compileSchema(doc)
}()

// fastSchema accepts exactly the documents compiledSchema accepts, in time
// linear in the document instead of exponential in its depth (jir-3rwv).
//
// Why compiledSchema is exponential: the validator collects every error of
// an anyOf branch that fails, so a branch whose node type is wrong still
// validates the node's whole content. listItem's content is
// anyOf[paragraph, bulletList, orderedList, ...], and bulletList and
// orderedList have the same content schema, so an orderedList nested in a
// list item is validated twice, once by the bulletList branch that then
// fails on its type: twice per level. Measured on valid documents, 16
// nested ordered lists took 9 s and 20 levels of Markdown "1. " 77 s; an
// invalid document is worse (an expand inside a nestedExpand, 6 levels: 5 s).
//
// fastSchema is the same schema with two changes that do not change what it
// accepts. Every definition that names its node type (properties.type) is
// wrapped as allOf[{properties: {type: ...}}, definition], and the root is
// not(not(doc)). Under not, the validator only needs a verdict and stops at
// the first error; its allOf takes its parts in order, so a branch with the
// wrong node type fails on the type before it looks at the content.
var fastSchema = func() *jsonschema.Schema {
	var raw map[string]any
	if err := json.Unmarshal(adfSchemaJSON, &raw); err != nil {
		panic("failed to unmarshal ADF schema: " + err.Error())
	}
	definitions, ok := raw["definitions"].(map[string]any)
	if !ok {
		panic("ADF schema has no definitions")
	}
	for name, d := range definitions {
		def, ok := d.(map[string]any)
		if !ok {
			continue
		}
		props, _ := def["properties"].(map[string]any)
		if typ, ok := props["type"]; ok {
			definitions[name] = map[string]any{"allOf": []any{
				map[string]any{"properties": map[string]any{"type": typ}},
				def,
			}}
		}
	}
	ref, ok := raw["$ref"].(string)
	if !ok {
		panic("ADF schema has no root $ref")
	}
	delete(raw, "$ref")
	raw["not"] = map[string]any{"not": map[string]any{"$ref": ref}}
	data, err := json.Marshal(raw)
	if err != nil {
		panic("failed to marshal the fast ADF schema: " + err.Error())
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		panic("failed to unmarshal the fast ADF schema: " + err.Error())
	}
	return compileSchema(doc)
}()

func compileSchema(doc any) *jsonschema.Schema {
	c := jsonschema.NewCompiler()
	if err := c.AddResource("adf-schema.json", doc); err != nil {
		panic("failed to add ADF schema resource: " + err.Error())
	}
	sch, err := c.Compile("adf-schema.json")
	if err != nil {
		panic("failed to compile ADF schema: " + err.Error())
	}
	return sch
}

// maxReportCost bounds the full report validateADF gives for an invalid
// document. compiledSchema's report costs roughly nodes x 2^depth (see
// fastSchema); measured, a cost of 2048 takes well under a second. A document
// over the bound gets an error without the report.
const maxReportCost = 2048

// requireValidADF marshals an ADFNode to JSON and validates it against the
// embedded ADF JSON schema. It calls t.Fatal on validation failure.
func requireValidADF(t *testing.T, node *jira4claude.ADFNode) {
	t.Helper()

	if err := validateADF(node); err != nil {
		data, _ := json.Marshal(node)
		t.Fatalf("%s\nADF JSON:\n%s", err, data)
	}
}

// validateADF is the non-fatal form of requireValidADF, for property checks
// that report a failure instead of stopping the test. The error does not
// include the document.
//
// The verdict is fastSchema's, linear in the document, so it is bounded for
// any input the fuzz targets generate. An invalid document's error carries
// compiledSchema's full report on the first top-level node that fails by
// itself, when that report's cost is at most maxReportCost.
func validateADF(node *jira4claude.ADFNode) error {
	v, err := asJSONValue(node)
	if err != nil {
		return err
	}
	if fastSchema.Validate(v) == nil {
		return nil
	}

	// Report on the first top-level node that fails by itself, so the cost
	// of the report does not grow with the document's width.
	subject, where := node, "the document"
	if node.Type == "doc" {
		for i := range node.Content {
			alone := &jira4claude.ADFNode{Type: node.Type, Version: node.Version, Content: node.Content[i : i+1]}
			if av, err := asJSONValue(alone); err == nil && fastSchema.Validate(av) != nil {
				subject, where = alone, fmt.Sprintf("top-level node %d (%s)", i, node.Content[i].Type)
				break
			}
		}
	}
	if cost := reportCost(subject); cost > maxReportCost {
		return fmt.Errorf("ADF schema validation failed in %s (no detailed report: %d nodes, %d deep, cost %d over %d)",
			where, nodeCount(subject), nodeDepth(subject), cost, maxReportCost)
	}
	sv, err := asJSONValue(subject)
	if err != nil {
		return err
	}
	if err := compiledSchema.Validate(sv); err != nil {
		return fmt.Errorf("ADF schema validation failed in %s:\n%w", where, err)
	}
	return fmt.Errorf("ADF schema validation failed by the fast schema only; the two schemas disagree, which is a bug in fastSchema")
}

func asJSONValue(node *jira4claude.ADFNode) (any, error) {
	data, err := json.Marshal(node)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ADFNode to JSON: %w", err)
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ADFNode JSON for validation: %w", err)
	}
	return v, nil
}

// reportCost estimates the work of compiledSchema's report on node: its node
// count times 2 to the power of its depth, saturating at maxReportCost+1.
func reportCost(node *jira4claude.ADFNode) int {
	depth := nodeDepth(node)
	if depth >= 30 {
		return maxReportCost + 1
	}
	return min(nodeCount(node)<<depth, maxReportCost+1)
}

// nodeCount is the number of nodes in node's tree, counting node itself.
func nodeCount(node *jira4claude.ADFNode) int {
	n := 1
	for i := range node.Content {
		n += nodeCount(&node.Content[i])
	}
	return n
}

// nodeDepth is the depth of node's tree, counting node itself.
func nodeDepth(node *jira4claude.ADFNode) int {
	deepest := 0
	for i := range node.Content {
		deepest = max(deepest, nodeDepth(&node.Content[i]))
	}
	return deepest + 1
}

func TestSchemaValidation(t *testing.T) {
	t.Parallel()

	t.Run("valid simple document passes", func(t *testing.T) {
		t.Parallel()

		doc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{Type: "text", Text: "Hello, world!"},
					},
				},
			},
		}
		requireValidADF(t, doc)
	})

	t.Run("valid empty document passes", func(t *testing.T) {
		t.Parallel()

		doc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{},
		}
		requireValidADF(t, doc)
	})

	t.Run("valid heading with level passes", func(t *testing.T) {
		t.Parallel()

		doc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type:  "heading",
					Attrs: json.RawMessage(`{"level":2}`),
					Content: []jira4claude.ADFNode{
						{Type: "text", Text: "My Heading"},
					},
				},
			},
		}
		requireValidADF(t, doc)
	})

	t.Run("heading without level fails", func(t *testing.T) {
		t.Parallel()

		doc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "heading",
					Content: []jira4claude.ADFNode{
						{Type: "text", Text: "No level"},
					},
				},
			},
		}

		data, err := json.Marshal(doc)
		require.NoError(t, err)
		var v any
		require.NoError(t, json.Unmarshal(data, &v))

		err = compiledSchema.Validate(v)
		assert.Error(t, err, "heading without level should fail schema validation")
	})

	t.Run("invalid top-level type fails", func(t *testing.T) {
		t.Parallel()

		doc := &jira4claude.ADFNode{
			Type:    "notADoc",
			Version: 1,
			Content: []jira4claude.ADFNode{},
		}

		data, err := json.Marshal(doc)
		require.NoError(t, err)
		var v any
		require.NoError(t, json.Unmarshal(data, &v))

		err = compiledSchema.Validate(v)
		assert.Error(t, err, "non-doc top-level type should fail schema validation")
	})
}

// TestValidateADF_Bounded pins validateADF's verdict to compiledSchema's and
// its running time to the document's size, not 2 to the power of its depth
// (jir-3rwv).
func TestValidateADF_Bounded(t *testing.T) {
	t.Parallel()

	para := `{"type":"paragraph","content":[{"type":"text","text":"x"}]}`
	bad := `{"type":"paragraph","content":[{"type":"text"}]}` // text without its text
	nest := func(levels int, open, leaf, close string) string {
		return `{"type":"doc","version":1,"content":[` +
			strings.Repeat(open, levels) + leaf + strings.Repeat(close, levels) + `]}`
	}
	wide := func(n int, node string) string {
		return `{"type":"doc","version":1,"content":[` + strings.TrimSuffix(strings.Repeat(node+",", n), ",") + `]}`
	}
	ordered := `{"type":"orderedList","attrs":{"order":1},"content":[{"type":"listItem","content":[` + para + `,`
	expand := `{"type":"expand","attrs":{"title":"t"},"content":[{"type":"nestedExpand","attrs":{"title":"n"},"content":[` + para + `,`
	parse := func(t *testing.T, raw string) *jira4claude.ADFNode {
		t.Helper()
		var doc jira4claude.ADFNode
		require.NoError(t, json.Unmarshal([]byte(raw), &doc))
		return &doc
	}
	verdict := func(t *testing.T, doc *jira4claude.ADFNode) error {
		t.Helper()
		v, err := json.Marshal(doc)
		require.NoError(t, err)
		var value any
		require.NoError(t, json.Unmarshal(v, &value))
		return compiledSchema.Validate(value)
	}

	t.Run("agrees with the full schema on fixtures and invalid documents", func(t *testing.T) {
		t.Parallel()

		docs := map[string]string{
			"heading without level":          `{"type":"doc","version":1,"content":[{"type":"heading","content":[{"type":"text","text":"x"}]}]}`,
			"not a doc":                      `{"type":"notADoc","version":1,"content":[]}`,
			"doc without version":            `{"type":"doc","content":[]}`,
			"text without text":              `{"type":"doc","version":1,"content":[` + bad + `]}`,
			"unknown node":                   `{"type":"doc","version":1,"content":[{"type":"bogus"}]}`,
			"expand in a nestedExpand":       nest(2, expand, para, `]}]}`),
			"ordered list, bad leaf":         nest(3, ordered, bad, `]}]}`),
			"ordered list order is a string": `{"type":"doc","version":1,"content":[{"type":"orderedList","attrs":{"order":"1"},"content":[{"type":"listItem","content":[` + para + `]}]}]}`,
			"list item opens with a list":    `{"type":"doc","version":1,"content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"bulletList","content":[{"type":"listItem","content":[` + para + `]}]}]}]}]}`,
			"valid nested ordered lists":     nest(4, ordered, para, `]}]}`),
			"valid expand":                   nest(1, expand, para, `]}]}`),
		}
		fixtures, err := filepath.Glob("testdata/*.adf.json")
		require.NoError(t, err)
		require.NotEmpty(t, fixtures)
		for _, f := range fixtures {
			data, err := os.ReadFile(f)
			require.NoError(t, err)
			docs[f] = string(data)
		}
		for _, md := range []string{"```", "<details></details>", "- ", "> [!NOTE]", "| a |\n|---|\n| |", "1. 1. x", "- [ ] task"} {
			doc, _ := markdown.New().ToADF(md)
			data, err := json.Marshal(doc)
			require.NoError(t, err)
			docs["markdown "+strconv.Quote(md)] = string(data)
		}

		for name, raw := range docs {
			doc := parse(t, raw)
			want := verdict(t, doc)
			got := validateADF(doc)
			assert.Equal(t, want == nil, got == nil, "%s: full schema says %v, validateADF says %v", name, want, got)
		}
	})

	t.Run("an invalid document's error carries the full report on the failing node", func(t *testing.T) {
		t.Parallel()

		doc := parse(t, wide(3, para)[:len(wide(3, para))-2]+`,`+bad+`]}`)
		err := validateADF(doc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "top-level node 3 (paragraph)")
		assert.Contains(t, err.Error(), "/content/0/content/0", "the report locates the bad node")
	})

	t.Run("deep and wide documents validate quickly", func(t *testing.T) {
		t.Parallel()

		// Each of these took seconds to minutes with the full schema alone:
		// 20 nested ordered lists over a minute, the invalid expands longer.
		docs := map[string]struct {
			raw   string
			valid bool
		}{
			"40 nested ordered lists":          {nest(40, ordered, para, `]}]}`), true},
			"40 nested ordered lists, bad":     {nest(40, ordered, bad, `]}]}`), false},
			"20 expands in nestedExpands":      {nest(20, expand, para, `]}]}`), false},
			"30 invalid expand chains":         {wide(30, strings.TrimPrefix(strings.TrimSuffix(nest(2, expand, para, `]}]}`), `]}`), `{"type":"doc","version":1,"content":[`)), false},
			"30 ordered lists with bad leaves": {wide(30, strings.TrimPrefix(strings.TrimSuffix(nest(2, ordered, bad, `]}]}`), `]}`), `{"type":"doc","version":1,"content":[`)), false},
		}
		for name, tc := range docs {
			doc := parse(t, tc.raw)
			start := time.Now()
			err := validateADF(doc)
			elapsed := time.Since(start)
			assert.Equal(t, tc.valid, err == nil, "%s: %v", name, err)
			assert.Less(t, elapsed, 5*time.Second, "%s took %v", name, elapsed)
		}
	})

	t.Run("a document too costly to report on still fails, without the report", func(t *testing.T) {
		t.Parallel()

		err := validateADF(parse(t, nest(20, expand, para, `]}]}`)))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no detailed report")
	})
}
