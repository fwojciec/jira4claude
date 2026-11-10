package jira4claude_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/fwojciec/jira4claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChange_MarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		change jira4claude.Change
		want   string
	}{
		{"strings", jira4claude.Change{From: "Old", To: "New"}, `{"from":"Old","to":"New"}`},
		{"empty is null", jira4claude.Change{From: nil, To: "Jane Doe"}, `{"from":null,"to":"Jane Doe"}`},
		{"lists", jira4claude.Change{From: []string{}, To: []string{"a", "b"}}, `{"from":[],"to":["a","b"]}`},
		{"raw JSON", jira4claude.Change{From: nil, To: json.RawMessage(`{"value":"High"}`)}, `{"from":null,"to":{"value":"High"}}`},
		{"an edit has no from and to", jira4claude.Change{Edit: jira4claude.EditSet}, `{"edit":"set"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := json.Marshal(tc.change)

			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(got))
		})
	}
}

func TestIssueURL(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "https://x.atlassian.net/browse/TEST-1", jira4claude.IssueURL("https://x.atlassian.net", "TEST-1"))
	assert.Empty(t, jira4claude.IssueURL("", "TEST-1"), "no server, no URL")
}

func TestUpdateChanges(t *testing.T) {
	t.Parallel()

	str := func(s string) *string { return &s }

	before := &jira4claude.Issue{
		Key:      "TEST-1",
		Summary:  "Old",
		Priority: "Medium",
		Labels:   []string{"a"},
		ReadCustomFields: map[string]jira4claude.CustomFieldValue{
			"customfield_1": {Name: "Severity", Value: json.RawMessage(`{"value":"Low"}`)},
		},
	}
	after := &jira4claude.Issue{
		Key:      "TEST-1",
		Summary:  "New",
		Priority: "High",
		Assignee: &jira4claude.User{AccountID: "u-1", DisplayName: "Jane Doe"},
		Labels:   nil,
		Parent:   &jira4claude.LinkedIssue{Key: "TEST-9"},
		ReadCustomFields: map[string]jira4claude.CustomFieldValue{
			"customfield_1": {Name: "Severity", Value: json.RawMessage(`{"value":"High"}`)},
		},
	}

	t.Run("reports from and to for every field written", func(t *testing.T) {
		t.Parallel()

		empty := []string{}
		update := jira4claude.IssueUpdate{
			Summary:      str("New"),
			Priority:     str("High"),
			Assignee:     str("u-1"),
			Labels:       &empty,
			Parent:       str("TEST-9"),
			CustomFields: map[string]json.RawMessage{"customfield_1": json.RawMessage(`{"value":"High"}`)},
		}

		got := jira4claude.UpdateChanges(before, after, update)

		assert.Equal(t, map[string]jira4claude.Change{
			"summary":       {From: "Old", To: "New"},
			"priority":      {From: "Medium", To: "High"},
			"assignee":      {From: nil, To: "Jane Doe"},
			"labels":        {From: []string{"a"}, To: []string{}},
			"parent":        {From: nil, To: "TEST-9"},
			"customfield_1": {From: json.RawMessage(`{"value":"Low"}`), To: json.RawMessage(`{"value":"High"}`)},
		}, got)
	})

	t.Run("fields not written are not reported", func(t *testing.T) {
		t.Parallel()

		got := jira4claude.UpdateChanges(before, after, jira4claude.IssueUpdate{Summary: str("New")})

		assert.Equal(t, map[string]jira4claude.Change{"summary": {From: "Old", To: "New"}}, got)
	})

	t.Run("a field written with its current value is still reported", func(t *testing.T) {
		t.Parallel()

		got := jira4claude.UpdateChanges(before, before, jira4claude.IssueUpdate{Summary: str("Old")})

		assert.Equal(t, map[string]jira4claude.Change{"summary": {From: "Old", To: "Old"}}, got)
	})

	t.Run("an unassign reports null", func(t *testing.T) {
		t.Parallel()

		got := jira4claude.UpdateChanges(after, before, jira4claude.IssueUpdate{Assignee: str("")})

		assert.Equal(t, map[string]jira4claude.Change{"assignee": {From: "Jane Doe", To: nil}}, got)
	})

	t.Run("a custom field missing on one side is null", func(t *testing.T) {
		t.Parallel()

		got := jira4claude.UpdateChanges(&jira4claude.Issue{}, &jira4claude.Issue{},
			jira4claude.IssueUpdate{CustomFields: map[string]json.RawMessage{"customfield_2": json.RawMessage(`1`)}})

		assert.Equal(t, map[string]jira4claude.Change{"customfield_2": {From: nil, To: nil}}, got)
	})

	t.Run("a non-custom field set by ID is reported as set, without values the read cannot show", func(t *testing.T) {
		t.Parallel()

		empty := []string{}
		got := jira4claude.UpdateChanges(before, after, jira4claude.IssueUpdate{
			Summary: str("New"),
			Labels:  &empty,
			CustomFields: map[string]json.RawMessage{
				"duedate": json.RawMessage(`"2026-12-01"`),
				"summary": json.RawMessage(`"Other"`),
				"labels":  json.RawMessage(`["x"]`),
			},
		})

		assert.Equal(t, map[string]jira4claude.Change{
			"summary": {From: "Old", To: "New"},
			"labels":  {From: []string{"a"}, To: []string{}},
			"duedate": {Edit: jira4claude.EditSet},
		}, got, "the typed entries are kept")
	})

	descriptionModes := []struct {
		name   string
		update jira4claude.DescriptionUpdate
		edit   string
	}{
		{"set", jira4claude.SetDescription(&jira4claude.ADFNode{Type: "doc"}), jira4claude.EditSet},
		{"append", jira4claude.AppendDescription(&jira4claude.ADFNode{Type: "doc"}, &jira4claude.ADFNode{Type: "doc"}), jira4claude.EditAppended},
		{"clear", jira4claude.ClearDescription(), jira4claude.EditCleared},
	}
	for _, tc := range descriptionModes {
		t.Run("a description "+tc.name+" reports how, not the text", func(t *testing.T) {
			t.Parallel()

			got := jira4claude.UpdateChanges(before, after, jira4claude.IssueUpdate{Description: tc.update})

			assert.Equal(t, map[string]jira4claude.Change{"description": {Edit: tc.edit}}, got)
		})
	}

	t.Run("an update that writes nothing has no changes, not nil", func(t *testing.T) {
		t.Parallel()

		got := jira4claude.UpdateChanges(before, after, jira4claude.IssueUpdate{})

		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
}

func TestPlannedChanges(t *testing.T) {
	t.Parallel()

	str := func(s string) *string { return &s }
	before := &jira4claude.Issue{
		Key:      "TEST-1",
		Summary:  "Old",
		Priority: "Medium",
		Assignee: &jira4claude.User{AccountID: "u-1", DisplayName: "Jane Doe"},
		Labels:   []string{"a"},
		Parent:   &jira4claude.LinkedIssue{Key: "TEST-8"},
		ReadCustomFields: map[string]jira4claude.CustomFieldValue{
			"customfield_1": {Name: "Severity", Value: json.RawMessage(`{"value":"Low"}`)},
		},
	}

	t.Run("from is the read, to is the value sent", func(t *testing.T) {
		t.Parallel()

		labels := []string{"b", "c"}
		got := jira4claude.PlannedChanges(before, jira4claude.IssueUpdate{
			Summary:      str("New"),
			Priority:     str("High"),
			Assignee:     str("u-2"),
			Labels:       &labels,
			Parent:       str("TEST-9"),
			Description:  jira4claude.ClearDescription(),
			CustomFields: map[string]json.RawMessage{"customfield_1": json.RawMessage(`{"value":"High"}`), "duedate": json.RawMessage(`"2026-12-01"`)},
		})

		assert.Equal(t, map[string]jira4claude.Change{
			"summary":       {From: "Old", To: "New"},
			"priority":      {From: "Medium", To: "High"},
			"assignee":      {From: "Jane Doe", To: "u-2"},
			"labels":        {From: []string{"a"}, To: []string{"b", "c"}},
			"parent":        {From: "TEST-8", To: "TEST-9"},
			"description":   {Edit: jira4claude.EditCleared},
			"customfield_1": {From: json.RawMessage(`{"value":"Low"}`), To: json.RawMessage(`{"value":"High"}`)},
			"duedate":       {Edit: jira4claude.EditSet},
		}, got)
	})

	t.Run("clearing reports null, and the read is not changed", func(t *testing.T) {
		t.Parallel()

		empty := []string{}
		got := jira4claude.PlannedChanges(before, jira4claude.IssueUpdate{Assignee: str(""), Labels: &empty, Parent: str("")})

		assert.Equal(t, map[string]jira4claude.Change{
			"assignee": {From: "Jane Doe", To: nil},
			"labels":   {From: []string{"a"}, To: []string{}},
			"parent":   {From: "TEST-8", To: nil},
		}, got)
		assert.Equal(t, "u-1", before.Assignee.AccountID)
		assert.Equal(t, []string{"a"}, before.Labels)
		assert.Equal(t, "TEST-8", before.Parent.Key)
	})

	t.Run("the current assignee keeps the name the read gave", func(t *testing.T) {
		t.Parallel()

		got := jira4claude.PlannedChanges(before, jira4claude.IssueUpdate{Assignee: str("u-1")})

		assert.Equal(t, map[string]jira4claude.Change{"assignee": {From: "Jane Doe", To: "Jane Doe"}}, got)
	})
}

func TestStatusChange(t *testing.T) {
	t.Parallel()

	got := jira4claude.StatusChange(&jira4claude.Issue{Status: "Backlog"}, &jira4claude.Transition{ID: "31", Name: "Finish", To: "Done"})
	assert.Equal(t, jira4claude.Change{From: "Backlog", To: "Done"}, got)

	got = jira4claude.StatusChange(&jira4claude.Issue{Status: "Backlog"}, &jira4claude.Transition{ID: "31", Name: "Finish"})
	assert.Equal(t, jira4claude.Change{From: "Backlog", To: nil}, got, "a target Jira did not name is unknown, null")
}

func TestAssigneeChange(t *testing.T) {
	t.Parallel()

	jane := &jira4claude.Issue{Assignee: &jira4claude.User{AccountID: "u-jane", DisplayName: "Jane Doe"}}
	idOnly := &jira4claude.Issue{Assignee: &jira4claude.User{AccountID: "u-x"}}

	assert.Equal(t, jira4claude.Change{From: nil, To: "Jane Doe"}, jira4claude.AssigneeChange(&jira4claude.Issue{}, jane))
	assert.Equal(t, jira4claude.Change{From: "Jane Doe", To: "u-x"}, jira4claude.AssigneeChange(jane, idOnly),
		"an assignee without a display name falls back to the account ID")
}

func TestUpdatedTimestamp(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "2026-11-09T10:15:30.123Z",
		jira4claude.UpdatedTimestamp(&jira4claude.Issue{Updated: time.Date(2026, 11, 9, 10, 15, 30, 123_000_000, time.UTC)}))
	assert.Empty(t, jira4claude.UpdatedTimestamp(&jira4claude.Issue{}), "an issue without an updated time has none")
	assert.Empty(t, jira4claude.UpdatedTimestamp(nil))
}

func TestRemovedNodes(t *testing.T) {
	t.Parallel()

	issues := []jira4claude.ConversionIssue{
		{Kind: jira4claude.ConversionDropped, Node: "{{mediaSingle:1 shot.png}}", Existing: true},
		{Kind: jira4claude.ConversionDropped, Node: "{{mention:2 Jane}}", Detail: "appears more than once"},
		{Kind: jira4claude.ConversionDropped, Node: "inlineCard", Existing: true},
		{Kind: jira4claude.ConversionDegraded, Node: "emoji", Existing: true},
		{Kind: jira4claude.ConversionDropped, Node: "Image"},
	}

	got := jira4claude.RemovedNodes(issues)

	assert.Equal(t, []string{"{{mediaSingle:1 shot.png}}", "inlineCard"}, got,
		"only dropped content of the existing document is removed")
	assert.Nil(t, jira4claude.RemovedNodes(nil))
}

func TestSelectTransition(t *testing.T) {
	t.Parallel()

	transitions := []*jira4claude.Transition{
		{ID: "11", Name: "Start", To: "In Progress"},
		{ID: "21", Name: "Done", To: "Closed"},
		{ID: "31", Name: "Finish", To: "Done"},
		{ID: "41", Name: "Resolve", To: "Resolved"},
		{ID: "42", Name: "Close", To: "Resolved"},
		{ID: "51", Name: "Reopen", To: "Backlog"},
		{ID: "52", Name: "reopen", To: "To Do"},
	}

	matches := []struct {
		name   string
		status string
		wantID string
	}{
		{"by target status", "In Progress", "11"},
		{"the target status wins over a transition name", "Done", "31"},
		{"by transition name when no status matches", "Start", "11"},
		{"case-insensitively", "in progress", "11"},
	}
	for _, tc := range matches {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := jira4claude.SelectTransition(transitions, tc.status)

			require.NoError(t, err)
			assert.Equal(t, tc.wantID, got.ID)
		})
	}

	t.Run("two transitions to the status are ambiguous and listed", func(t *testing.T) {
		t.Parallel()

		_, err := jira4claude.SelectTransition(transitions, "Resolved")

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		msg := jira4claude.ErrorMessage(err)
		assert.Contains(t, msg, "ambiguous")
		assert.Contains(t, msg, `"Resolve" (id 41) → Resolved`)
		assert.Contains(t, msg, `"Close" (id 42) → Resolved`)
		assert.NotContains(t, msg, "Start")
		assert.Contains(t, msg, "Nothing was written")
		assert.Contains(t, msg, "--id")
	})

	t.Run("two transitions of the name are ambiguous", func(t *testing.T) {
		t.Parallel()

		_, err := jira4claude.SelectTransition(transitions, "Reopen")

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.Contains(t, jira4claude.ErrorMessage(err), `"Reopen" (id 51) → Backlog`)
		assert.Contains(t, jira4claude.ErrorMessage(err), `"reopen" (id 52) → To Do`)
	})

	t.Run("no match lists every transition", func(t *testing.T) {
		t.Parallel()

		_, err := jira4claude.SelectTransition(transitions, "Nope")

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		msg := jira4claude.ErrorMessage(err)
		assert.Contains(t, msg, `status "Nope" not found`)
		assert.Contains(t, msg, `"Start" (id 11) → In Progress`)
		assert.Contains(t, msg, `"Reopen" (id 51) → Backlog`)
	})

	t.Run("a transition without a known target matches by name", func(t *testing.T) {
		t.Parallel()

		got, err := jira4claude.SelectTransition([]*jira4claude.Transition{{ID: "31", Name: "Done"}}, "Done")

		require.NoError(t, err)
		assert.Equal(t, "31", got.ID)
	})
}

func TestTransitionByID(t *testing.T) {
	t.Parallel()

	transitions := []*jira4claude.Transition{{ID: "11", Name: "Start", To: "In Progress"}}

	t.Run("finds the transition", func(t *testing.T) {
		t.Parallel()

		got, err := jira4claude.TransitionByID(transitions, "11")

		require.NoError(t, err)
		assert.Equal(t, "Start", got.Name)
	})

	t.Run("an unavailable id is a validation error listing the choices", func(t *testing.T) {
		t.Parallel()

		_, err := jira4claude.TransitionByID(transitions, "99")

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.Contains(t, jira4claude.ErrorMessage(err), `transition id "99" is not available`)
		assert.Contains(t, jira4claude.ErrorMessage(err), `"Start" (id 11) → In Progress`)
	})
}
