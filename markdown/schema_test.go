package markdown_test

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
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

	if err := validateADF(node); err != nil {
		data, _ := json.Marshal(node)
		t.Fatalf("%s\nADF JSON:\n%s", err, data)
	}
}

// validateADF is the non-fatal form of requireValidADF, for property checks
// that report a failure instead of stopping the test. The error does not
// include the document.
func validateADF(node *jira4claude.ADFNode) error {
	data, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to marshal ADFNode to JSON: %w", err)
	}

	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("failed to unmarshal ADFNode JSON for validation: %w", err)
	}

	if err := compiledSchema.Validate(v); err != nil {
		return fmt.Errorf("ADF schema validation failed:\n%w", err)
	}
	return nil
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
