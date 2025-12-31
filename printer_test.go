package jira4claude_test

import (
	"testing"

	"github.com/fwojciec/jira4claude"
)

// testPrinter is a minimal implementation used only to verify
// that Printer properly embeds all sub-interfaces at compile time.
type testPrinter struct{}

func (testPrinter) Issue(*jira4claude.Issue)                      {}
func (testPrinter) Issues([]*jira4claude.Issue)                   {}
func (testPrinter) Transitions(string, []*jira4claude.Transition) {}
func (testPrinter) Links(string, []*jira4claude.IssueLink)        {}
func (testPrinter) Success(string, ...string)                     {}
func (testPrinter) Error(error)                                   {}

// Compile-time interface compliance checks.
// These verify that Printer embeds all sub-interfaces.
var (
	_ jira4claude.IssuePrinter   = testPrinter{}
	_ jira4claude.LinkPrinter    = testPrinter{}
	_ jira4claude.MessagePrinter = testPrinter{}
	_ jira4claude.Printer        = testPrinter{}
)

func TestPrinterComposition(t *testing.T) {
	t.Parallel()

	// This test verifies that Printer embeds all sub-interfaces.
	// The compile-time checks above ensure the embedding is correct.
	// If any sub-interface is missing from Printer, this file won't compile.

	var p jira4claude.Printer = testPrinter{}

	// Verify Printer can be used as each sub-interface.
	var _ jira4claude.IssuePrinter = p
	var _ jira4claude.LinkPrinter = p
	var _ jira4claude.MessagePrinter = p
}
