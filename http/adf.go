package http

import (
	"encoding/json"

	"github.com/fwojciec/jira4claude"
)

// textOrADF converts text to ADF using the provided converter, but if the text
// is already a valid ADF JSON document (starts with `{` and has a "type":"doc" field),
// it parses and returns that directly. This allows the CLI to pre-convert markdown
// to ADF and pass it through without double-conversion.
func textOrADF(text string, converter jira4claude.Converter) map[string]any {
	if adf := tryParseADF(text); adf != nil {
		return adf
	}
	// Convert to ADF using the injected converter (plain text is valid input)
	adf, _ := converter.ToADF(text)
	return adf
}

// tryParseADF attempts to parse text as an ADF JSON document.
// Returns the parsed ADF if successful, nil otherwise.
func tryParseADF(text string) map[string]any {
	// Quick check: must start with { to be valid JSON object
	if len(text) == 0 || text[0] != '{' {
		return nil
	}

	var adf map[string]any
	if err := json.Unmarshal([]byte(text), &adf); err != nil {
		return nil
	}

	// Verify this is an ADF document (has "type": "doc")
	if docType, ok := adf["type"].(string); ok && docType == "doc" {
		return adf
	}

	return nil
}
