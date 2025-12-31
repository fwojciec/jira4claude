package main_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/fwojciec/jira4claude"
	main "github.com/fwojciec/jira4claude/cmd/j4c"
	"github.com/fwojciec/jira4claude/gogh"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueTransitionCmd_InvalidStatusShowsQuotedOptions(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	printer := gogh.NewTextPrinter(io)

	svc := &mock.IssueService{
		TransitionsFn: func(ctx context.Context, key string) ([]*jira4claude.Transition, error) {
			return []*jira4claude.Transition{
				{ID: "21", Name: "In Progress"},
				{ID: "31", Name: "Done"},
			}, nil
		},
	}

	ctx := &main.IssueContext{
		Service: svc,
		Printer: printer,
		Config:  &jira4claude.Config{Project: "TEST"},
	}

	cmd := &main.IssueTransitionCmd{
		Key:    "TEST-123",
		Status: "invalid-status",
	}

	err := cmd.Run(ctx)

	require.Error(t, err)
	errMsg := err.Error()
	// Should quote status names for clarity
	assert.Contains(t, errMsg, `"In Progress"`)
	assert.Contains(t, errMsg, `"Done"`)
}
