package markdown_test

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"testing"

	jira4claude "github.com/fwojciec/jira4claude"
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
	c := jsonschema.NewCompiler()
	if err := c.AddResource("adf-schema.json", doc); err != nil {
		panic("failed to add ADF schema resource: " + err.Error())
	}
	sch, err := c.Compile("adf-schema.json")
	if err != nil {
		panic("failed to compile ADF schema: " + err.Error())
	}
	return sch
}()

// requireValidADF marshals an ADFNode to JSON and validates it against the
// embedded ADF JSON schema. It calls t.Fatal on validation failure.
func requireValidADF(t *testing.T, node *jira4claude.ADFNode) {
	t.Helper()

	data, err := json.Marshal(node)
	require.NoError(t, err, "failed to marshal ADFNode to JSON")

	var v any
	require.NoError(t, json.Unmarshal(data, &v), "failed to unmarshal ADFNode JSON for validation")

	err = compiledSchema.Validate(v)
	if err != nil {
		t.Fatalf("ADF schema validation failed:\n%s\nADF JSON:\n%s", err, data)
	}
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
