package main_test

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/fwojciec/jira4claude"
	main "github.com/fwojciec/jira4claude/cmd/j4c"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func listTestContext(svc *mock.IssueService, printer *mock.Printer) *main.IssueContext {
	return &main.IssueContext{
		Service:   svc,
		Printer:   printer,
		Converter: mockConverter(),
		Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
	}
}

func readyIssue(key string) *jira4claude.Issue {
	return &jira4claude.Issue{Key: key, Status: "To Do"}
}

func blockedIssue(key string) *jira4claude.Issue {
	return &jira4claude.Issue{
		Key:    key,
		Status: "To Do",
		Links: []*jira4claude.IssueLink{
			{
				Type:        jira4claude.IssueLinkType{Name: "Blocks", Inward: "is blocked by"},
				InwardIssue: &jira4claude.LinkedIssue{Key: "BLOCKER-1", Status: "In Progress"},
			},
		},
	}
}

// pagedService returns a mock that serves pages in order. Page i is returned
// when the request carries the token "page-i" (the first page has no token);
// every page except the last returns the next page's token. It records the
// page token each call received.
func pagedService(pages ...[]*jira4claude.Issue) (*mock.IssueService, func() []string) {
	var mu sync.Mutex
	var tokens []string
	svc := &mock.IssueService{
		ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, string, error) {
			mu.Lock()
			tokens = append(tokens, filter.PageToken)
			mu.Unlock()

			idx := 0
			if filter.PageToken != "" {
				for i := range pages {
					if filter.PageToken == pageToken(i) {
						idx = i
					}
				}
			}
			next := ""
			if idx+1 < len(pages) {
				next = pageToken(idx + 1)
			}
			return pages[idx], next, nil
		},
	}
	return svc, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), tokens...)
	}
}

func pageToken(i int) string {
	return "page-" + strconv.Itoa(i)
}

func TestIssueListCmd_Truncation(t *testing.T) {
	t.Parallel()

	t.Run("warns and marks output when more results exist than limit", func(t *testing.T) {
		t.Parallel()

		svc, calls := pagedService(
			[]*jira4claude.Issue{readyIssue("TEST-1"), readyIssue("TEST-2")},
			[]*jira4claude.Issue{readyIssue("TEST-3")},
		)
		printer := &mock.Printer{}
		cmd := main.IssueListCmd{Project: "TEST", Limit: 2}

		err := cmd.Run(listTestContext(svc, printer))

		require.NoError(t, err)
		assert.Equal(t, []string{""}, calls(), "list must not page past --limit")
		require.Len(t, printer.IssuesCalls, 1)
		assert.True(t, printer.IssuesCalls[0].Truncated)
		assert.Len(t, printer.IssuesCalls[0].Issues, 2)
		require.Len(t, printer.WarningCalls, 1)
		assert.Contains(t, printer.WarningCalls[0], "--limit")
	})

	t.Run("does not suggest raising limit when Jira returned a short page", func(t *testing.T) {
		t.Parallel()

		svc, _ := pagedService(
			[]*jira4claude.Issue{readyIssue("TEST-1")},
			[]*jira4claude.Issue{readyIssue("TEST-2")},
		)
		printer := &mock.Printer{}
		cmd := main.IssueListCmd{Project: "TEST", Limit: 5}

		err := cmd.Run(listTestContext(svc, printer))

		require.NoError(t, err)
		require.Len(t, printer.IssuesCalls, 1)
		assert.True(t, printer.IssuesCalls[0].Truncated)
		require.Len(t, printer.WarningCalls, 1)
		assert.NotContains(t, printer.WarningCalls[0], "--limit")
	})

	t.Run("no notice when results fit", func(t *testing.T) {
		t.Parallel()

		svc, _ := pagedService([]*jira4claude.Issue{readyIssue("TEST-1")})
		printer := &mock.Printer{}
		cmd := main.IssueListCmd{Project: "TEST", Limit: 2}

		err := cmd.Run(listTestContext(svc, printer))

		require.NoError(t, err)
		require.Len(t, printer.IssuesCalls, 1)
		assert.False(t, printer.IssuesCalls[0].Truncated)
		assert.Empty(t, printer.WarningCalls)
	})
}

func TestIssueReadyCmd_Paging(t *testing.T) {
	t.Parallel()

	t.Run("finds a ready issue beyond the first page", func(t *testing.T) {
		t.Parallel()

		svc, calls := pagedService(
			[]*jira4claude.Issue{blockedIssue("TEST-1"), blockedIssue("TEST-2")},
			[]*jira4claude.Issue{readyIssue("TEST-3")},
		)
		printer := &mock.Printer{}
		cmd := main.IssueReadyCmd{Limit: 2}

		err := cmd.Run(listTestContext(svc, printer))

		require.NoError(t, err)
		assert.Equal(t, []string{"", pageToken(1)}, calls(), "second request must carry the first page's token")
		require.Len(t, printer.IssuesCalls, 1)
		items := printer.IssuesCalls[0].Issues
		require.Len(t, items, 1)
		assert.Equal(t, "TEST-3", items[0].Key)
		assert.False(t, printer.IssuesCalls[0].Truncated)
		assert.Empty(t, printer.WarningCalls)
	})

	t.Run("stops requesting pages once it has limit ready issues", func(t *testing.T) {
		t.Parallel()

		svc, calls := pagedService(
			[]*jira4claude.Issue{blockedIssue("TEST-1"), readyIssue("TEST-2")},
			[]*jira4claude.Issue{readyIssue("TEST-3"), readyIssue("TEST-4")},
			[]*jira4claude.Issue{readyIssue("TEST-5")},
		)
		printer := &mock.Printer{}
		cmd := main.IssueReadyCmd{Limit: 2}

		err := cmd.Run(listTestContext(svc, printer))

		require.NoError(t, err)
		assert.Equal(t, []string{"", pageToken(1)}, calls())
		require.Len(t, printer.IssuesCalls, 1)
		items := printer.IssuesCalls[0].Issues
		require.Len(t, items, 2)
		assert.Equal(t, "TEST-2", items[0].Key)
		assert.Equal(t, "TEST-3", items[1].Key)
		assert.True(t, printer.IssuesCalls[0].Truncated)
		require.Len(t, printer.WarningCalls, 1)
		assert.Contains(t, printer.WarningCalls[0], "--limit")
	})

	t.Run("cuts a page that overshoots the limit", func(t *testing.T) {
		t.Parallel()

		svc, calls := pagedService(
			[]*jira4claude.Issue{readyIssue("TEST-1"), readyIssue("TEST-2"), readyIssue("TEST-3")},
		)
		printer := &mock.Printer{}
		cmd := main.IssueReadyCmd{Limit: 2}

		err := cmd.Run(listTestContext(svc, printer))

		require.NoError(t, err)
		assert.Equal(t, []string{""}, calls())
		require.Len(t, printer.IssuesCalls, 1)
		assert.Len(t, printer.IssuesCalls[0].Issues, 2)
		assert.True(t, printer.IssuesCalls[0].Truncated)
	})

	t.Run("not truncated when exactly limit ready issues and results end", func(t *testing.T) {
		t.Parallel()

		svc, _ := pagedService(
			[]*jira4claude.Issue{readyIssue("TEST-1"), blockedIssue("TEST-2"), readyIssue("TEST-3")},
		)
		printer := &mock.Printer{}
		cmd := main.IssueReadyCmd{Limit: 2}

		err := cmd.Run(listTestContext(svc, printer))

		require.NoError(t, err)
		require.Len(t, printer.IssuesCalls, 1)
		assert.Len(t, printer.IssuesCalls[0].Issues, 2)
		assert.False(t, printer.IssuesCalls[0].Truncated)
		assert.Empty(t, printer.WarningCalls)
	})

	t.Run("pages to the end when limit is zero", func(t *testing.T) {
		t.Parallel()

		svc, calls := pagedService(
			[]*jira4claude.Issue{readyIssue("TEST-1")},
			[]*jira4claude.Issue{readyIssue("TEST-2")},
		)
		printer := &mock.Printer{}
		cmd := main.IssueReadyCmd{}

		err := cmd.Run(listTestContext(svc, printer))

		require.NoError(t, err)
		assert.Equal(t, []string{"", pageToken(1)}, calls())
		require.Len(t, printer.IssuesCalls, 1)
		assert.Len(t, printer.IssuesCalls[0].Issues, 2)
		assert.False(t, printer.IssuesCalls[0].Truncated)
	})
}
