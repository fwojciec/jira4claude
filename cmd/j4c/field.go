package main

import (
	"encoding/json"
	"strings"

	"github.com/fwojciec/jira4claude"
)

// ParseFieldJSON parses repeated --field-json flags of the form key=<json>
// into a map keyed by field ID. Each entry's value must be syntactically
// valid JSON; ParseFieldJSON does NOT validate the JSON shape against any
// Jira schema (that is the caller's responsibility, or Jira's).
//
// Validation rules:
//   - Each entry MUST contain at least one '='. Splitting is on the FIRST
//     '=' only so values may contain '=' (e.g. JSON like {"a":"b=c"}).
//   - The key (left of '=') must be non-empty.
//   - The value (right of '=') must satisfy json.Valid.
//   - The same key may not appear twice across the slice; duplicate keys
//     return an error rather than silently last-wins.
//
// Returns nil (not an empty map) when raws is empty or nil.
// All validation failures return a *jira4claude.Error with Code=EValidation.
func ParseFieldJSON(raws []string) (map[string]json.RawMessage, error) {
	if len(raws) == 0 {
		return nil, nil
	}

	out := make(map[string]json.RawMessage, len(raws))
	for _, raw := range raws {
		idx := strings.IndexByte(raw, '=')
		if idx < 0 {
			return nil, &jira4claude.Error{
				Code:    jira4claude.EValidation,
				Message: `--field-json must be of the form key=<json>: "` + raw + `"`,
			}
		}
		key := raw[:idx]
		value := raw[idx+1:]
		if key == "" {
			return nil, &jira4claude.Error{
				Code:    jira4claude.EValidation,
				Message: `--field-json key must be non-empty: "` + raw + `"`,
			}
		}
		if !json.Valid([]byte(value)) {
			return nil, &jira4claude.Error{
				Code:    jira4claude.EValidation,
				Message: `--field-json value for "` + key + `" is not valid JSON: ` + value,
			}
		}
		if _, exists := out[key]; exists {
			return nil, &jira4claude.Error{
				Code:    jira4claude.EValidation,
				Message: `--field-json key "` + key + `" specified more than once`,
			}
		}
		out[key] = json.RawMessage(value)
	}
	return out, nil
}
