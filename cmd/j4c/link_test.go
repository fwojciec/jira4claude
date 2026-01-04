package main_test

import (
	"context"
	"testing"

	"github.com/fwojciec/jira4claude"
	main "github.com/fwojciec/jira4claude/cmd/j4c"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LinkListCmd tests

func TestLinkListCmd(t *testing.T) {
	t.Parallel()

	t.Run("lists links for an issue", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
				require.Equal(t, "TEST-123", key)
				return &jira4claude.Issue{
					Key:     "TEST-123",
					Summary: "Test issue",
					Links: []*jira4claude.IssueLink{
						{
							Type: jira4claude.IssueLinkType{
								Name:    "Blocks",
								Outward: "blocks",
								Inward:  "is blocked by",
							},
							OutwardIssue: &jira4claude.LinkedIssue{
								Key:     "TEST-456",
								Summary: "Blocked issue",
								Status:  "To Do",
							},
						},
						{
							Type: jira4claude.IssueLinkType{
								Name:    "Blocks",
								Outward: "blocks",
								Inward:  "is blocked by",
							},
							InwardIssue: &jira4claude.LinkedIssue{
								Key:     "TEST-789",
								Summary: "Blocking issue",
								Status:  "Done",
							},
						},
					},
				}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.LinkContext{
			Service: svc,
			Printer: printer,
			Config:  &jira4claude.Config{Project: "TEST"},
		}

		cmd := main.LinkListCmd{Key: "TEST-123"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.Len(t, printer.LinksCalls, 1)
		assert.Equal(t, "TEST-123", printer.LinksCalls[0].Key)
		assert.Len(t, printer.LinksCalls[0].Links, 2)
		assert.Equal(t, "blocks", printer.LinksCalls[0].Links[0].Type)
		assert.Equal(t, "outward", printer.LinksCalls[0].Links[0].Direction)
		assert.Equal(t, "TEST-456", printer.LinksCalls[0].Links[0].IssueKey)
		assert.Equal(t, "is blocked by", printer.LinksCalls[0].Links[1].Type)
		assert.Equal(t, "inward", printer.LinksCalls[0].Links[1].Direction)
		assert.Equal(t, "TEST-789", printer.LinksCalls[0].Links[1].IssueKey)
	})

	t.Run("handles issue with no links", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{
					Key:     "TEST-123",
					Summary: "Test issue",
					Links:   []*jira4claude.IssueLink{},
				}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.LinkContext{
			Service: svc,
			Printer: printer,
			Config:  &jira4claude.Config{Project: "TEST"},
		}

		cmd := main.LinkListCmd{Key: "TEST-123"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.Len(t, printer.LinksCalls, 1)
		assert.Equal(t, "TEST-123", printer.LinksCalls[0].Key)
		assert.Empty(t, printer.LinksCalls[0].Links)
	})

	t.Run("returns error when service fails", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
				return nil, &jira4claude.Error{Code: jira4claude.ENotFound, Message: "Issue not found"}
			},
		}

		printer := &mock.Printer{}
		ctx := &main.LinkContext{
			Service: svc,
			Printer: printer,
			Config:  &jira4claude.Config{Project: "TEST"},
		}

		cmd := main.LinkListCmd{Key: "INVALID-123"}
		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Issue not found")
		assert.Empty(t, printer.LinksCalls)
	})
}
