package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/fwojciec/jira4claude"
	jirahttp "github.com/fwojciec/jira4claude/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueService_Update_ReadBackFails(t *testing.T) {
	t.Parallel()

	// The PUT is accepted and the read that follows fails: the error says
	// the write was applied (jira4claude.ReadBackError) and keeps the read's
	// code.
	var puts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			puts.Add(1)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errorMessages":["no browse permission"]}`))
	}))
	defer server.Close()
	svc := jirahttp.NewIssueService(newTestClient(t, server.URL, "user@example.com", "api-token"))

	summary := "S"
	got, err := svc.Update(context.Background(), "TEST-1", jira4claude.IssueUpdate{Summary: &summary})

	require.Error(t, err)
	assert.Nil(t, got)
	var readBack *jira4claude.ReadBackError
	require.ErrorAs(t, err, &readBack)
	assert.Equal(t, jira4claude.EForbidden, jira4claude.ErrorCode(err))
	assert.Equal(t, int32(1), puts.Load())
}

func TestIssueService_Update_WriteFails(t *testing.T) {
	t.Parallel()

	// A refused PUT is not a read-back failure.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":{"summary":"too long"}}`))
	}))
	defer server.Close()
	svc := jirahttp.NewIssueService(newTestClient(t, server.URL, "user@example.com", "api-token"))

	summary := "S"
	_, err := svc.Update(context.Background(), "TEST-1", jira4claude.IssueUpdate{Summary: &summary})

	require.Error(t, err)
	var readBack *jira4claude.ReadBackError
	assert.NotErrorAs(t, err, &readBack)
	assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
}
