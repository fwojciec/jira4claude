package markdown_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sequentialLocalIDs is a deterministic localId source: the n-th localId
// generated in a conversion is "id-n".
func sequentialLocalIDs(n int) string {
	return fmt.Sprintf("id-%d", n)
}

// localIDs returns the localId attributes of a document in document order.
func localIDs(t *testing.T, doc any) []string {
	t.Helper()

	data, err := json.Marshal(doc)
	require.NoError(t, err)
	var v any
	require.NoError(t, json.Unmarshal(data, &v))

	var ids []string
	var walk func(any)
	walk = func(v any) {
		node, ok := v.(map[string]any)
		if !ok {
			return
		}
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if id, ok := attrs["localId"].(string); ok {
				ids = append(ids, id)
			}
		}
		if content, ok := node["content"].([]any); ok {
			for _, c := range content {
				walk(c)
			}
		}
	}
	walk(v)
	return ids
}

func TestWithLocalIDs(t *testing.T) {
	t.Parallel()

	const tasks = "- [ ] one\n- [x] two\n  - [ ] nested"

	t.Run("same task list converted twice gives identical ADF", func(t *testing.T) {
		t.Parallel()

		conv := markdown.New(markdown.WithLocalIDs(sequentialLocalIDs))
		first, _ := conv.ToADF(tasks)
		second, _ := conv.ToADF(tasks)

		firstJSON, err := json.Marshal(first)
		require.NoError(t, err)
		secondJSON, err := json.Marshal(second)
		require.NoError(t, err)
		assert.JSONEq(t, string(firstJSON), string(secondJSON))
	})

	t.Run("each conversion numbers its ids from zero", func(t *testing.T) {
		t.Parallel()

		conv := markdown.New(markdown.WithLocalIDs(sequentialLocalIDs))
		_, _ = conv.ToADF(tasks)
		doc, _ := conv.ToADF(tasks)

		assert.ElementsMatch(t, []string{"id-0", "id-1", "id-2", "id-3", "id-4"}, localIDs(t, doc))
	})

	t.Run("default source gives distinct random ids", func(t *testing.T) {
		t.Parallel()

		doc, _ := markdown.New().ToADF(tasks)
		ids := localIDs(t, doc)

		require.Len(t, ids, 5)
		seen := map[string]bool{}
		for _, id := range ids {
			assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, id)
			seen[id] = true
		}
		assert.Len(t, seen, 5)
	})
}
