package main_test

import (
	"encoding/json"
	"testing"

	"github.com/fwojciec/jira4claude"
	main "github.com/fwojciec/jira4claude/cmd/j4c"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFieldJSON(t *testing.T) {
	t.Parallel()

	t.Run("nil input returns nil map and no error", func(t *testing.T) {
		t.Parallel()

		got, err := main.ParseFieldJSON(nil)

		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("empty input returns nil map and no error", func(t *testing.T) {
		t.Parallel()

		got, err := main.ParseFieldJSON([]string{})

		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("happy path: two valid entries produce a 2-key map", func(t *testing.T) {
		t.Parallel()

		got, err := main.ParseFieldJSON([]string{
			`customfield_10801={"value":"High"}`,
			`customfield_10838=[{"value":"Integrations"}]`,
		})

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.JSONEq(t, `{"value":"High"}`, string(got["customfield_10801"]))
		assert.JSONEq(t, `[{"value":"Integrations"}]`, string(got["customfield_10838"]))
	})

	t.Run("value containing = splits on first equals only", func(t *testing.T) {
		t.Parallel()

		got, err := main.ParseFieldJSON([]string{
			`customfield_10001={"a":"b=c"}`,
		})

		require.NoError(t, err)
		require.Len(t, got, 1)
		// The raw value, including the inner '=', must round-trip.
		assert.JSONEq(t, `{"a":"b=c"}`, string(got["customfield_10001"]))
	})

	t.Run("value containing multiple = parses correctly", func(t *testing.T) {
		t.Parallel()

		got, err := main.ParseFieldJSON([]string{
			`key={"x":"a=b=c=d"}`,
		})

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.JSONEq(t, `{"x":"a=b=c=d"}`, string(got["key"]))
	})

	t.Run("missing = returns EValidation quoting the raw input", func(t *testing.T) {
		t.Parallel()

		_, err := main.ParseFieldJSON([]string{"missing-equals-sign"})

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), `"missing-equals-sign"`)
	})

	t.Run("empty key returns EValidation", func(t *testing.T) {
		t.Parallel()

		_, err := main.ParseFieldJSON([]string{`="value"`})

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "non-empty")
	})

	t.Run("invalid JSON value returns EValidation quoting key and bad value", func(t *testing.T) {
		t.Parallel()

		_, err := main.ParseFieldJSON([]string{`customfield_10801=not-json`})

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "customfield_10801")
		assert.Contains(t, err.Error(), "not-json")
	})

	t.Run("duplicate key returns EValidation quoting the key", func(t *testing.T) {
		t.Parallel()

		_, err := main.ParseFieldJSON([]string{
			`customfield_10801={"value":"High"}`,
			`customfield_10801={"value":"Low"}`,
		})

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "customfield_10801")
	})

	t.Run("primitive JSON values are accepted (string, number, bool, null)", func(t *testing.T) {
		t.Parallel()

		got, err := main.ParseFieldJSON([]string{
			`a="hello"`,
			`b=42`,
			`c=true`,
			`d=null`,
		})

		require.NoError(t, err)
		require.Len(t, got, 4)
		assert.JSONEq(t, `"hello"`, string(got["a"]))
		assert.JSONEq(t, `42`, string(got["b"]))
		assert.JSONEq(t, `true`, string(got["c"]))
		assert.JSONEq(t, `null`, string(got["d"]))
	})

	t.Run("result values are raw JSON suitable for re-marshaling", func(t *testing.T) {
		t.Parallel()

		got, err := main.ParseFieldJSON([]string{`customfield_1={"value":"High"}`})
		require.NoError(t, err)

		// Re-marshaling must produce the same JSON (modulo whitespace)
		// — confirms we stored the bytes as-is rather than re-encoding.
		out, err := json.Marshal(got)
		require.NoError(t, err)
		assert.JSONEq(t, `{"customfield_1":{"value":"High"}}`, string(out))
	})
}
