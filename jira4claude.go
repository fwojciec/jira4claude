// Package jira4claude provides domain types for the jira4claude CLI.
//
// This package contains pure domain entities and service interfaces
// with no external dependencies. Implementations live in subpackages
// named after their dependencies (e.g., jira/ for the API client).
package jira4claude

// ADF represents an Atlassian Document Format document.
// ADF is Jira's native rich text format, stored as a structured JSON object.
type ADF = map[string]any
