// Package http provides an HTTP client for Jira API communication.
//
// This package wraps net/http with Jira-specific authentication (via netrc)
// and error handling.
package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/fwojciec/jira4claude"
	"github.com/jdx/go-netrc"
)

// Client is an HTTP client configured for Jira API requests.
type Client struct {
	baseURL    *url.URL
	username   string
	password   string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*clientConfig)

type clientConfig struct {
	netrcPath  string
	httpClient *http.Client
}

// WithNetrcPath sets a custom path to the netrc file.
func WithNetrcPath(path string) Option {
	return func(c *clientConfig) {
		c.netrcPath = path
	}
}

// WithHTTPClient sets a custom HTTP client for making requests.
func WithHTTPClient(client *http.Client) Option {
	return func(c *clientConfig) {
		c.httpClient = client
	}
}

// NewClient creates a new Client configured for the given Jira server.
// It reads credentials from the netrc file for authentication.
func NewClient(baseURL string, opts ...Option) (*Client, error) {
	cfg := &clientConfig{
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Parse base URL
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Determine netrc path
	netrcPath := cfg.netrcPath
	if netrcPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("could not determine home directory: %w", err)
		}
		netrcPath = filepath.Join(home, ".netrc")
	}

	// Read netrc file
	n, err := netrc.Parse(netrcPath)
	if err != nil {
		return nil, fmt.Errorf("could not read netrc file: %w", err)
	}

	// Find machine entry for the host
	machine := n.Machine(u.Host)
	if machine == nil {
		return nil, fmt.Errorf("no netrc entry for %s", u.Host)
	}

	// Validate credentials are present
	login := machine.Get("login")
	password := machine.Get("password")
	if login == "" {
		return nil, fmt.Errorf("netrc entry for %s: login is required", u.Host)
	}
	if password == "" {
		return nil, fmt.Errorf("netrc entry for %s: password is required", u.Host)
	}

	return &Client{
		baseURL:    u,
		username:   login,
		password:   password,
		httpClient: cfg.httpClient,
	}, nil
}

// Do executes an HTTP request with Jira authentication.
// Relative paths are resolved against the client's base URL.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	// Resolve relative URL against base URL
	reqURL := c.baseURL.ResolveReference(req.URL)
	req.URL = reqURL

	// Set Basic auth
	req.SetBasicAuth(c.username, c.password)

	// Jira API always returns JSON
	req.Header.Set("Accept", "application/json")

	return c.httpClient.Do(req)
}

// ErrorResponse represents a Jira API error response.
type ErrorResponse struct {
	ErrorMessages []string          `json:"errorMessages"`
	Errors        map[string]string `json:"errors"`
}

// ParseErrorResponse parses a Jira API error response body and returns
// a domain error with the appropriate error code based on the HTTP status.
// Returns nil if no errors are present in the response body.
func ParseErrorResponse(statusCode int, body []byte) error {
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return &jira4claude.Error{
			Code:    statusCodeToErrorCode(statusCode),
			Message: "failed to parse error response",
			Inner:   err,
		}
	}

	// Collect all error messages
	messages := make([]string, 0, len(errResp.ErrorMessages)+len(errResp.Errors))
	messages = append(messages, errResp.ErrorMessages...)
	for _, msg := range errResp.Errors {
		messages = append(messages, msg)
	}

	if len(messages) == 0 {
		return nil
	}

	return &jira4claude.Error{
		Code:    statusCodeToErrorCode(statusCode),
		Message: strings.Join(messages, "; "),
	}
}

// statusCodeToErrorCode maps HTTP status codes to domain error codes.
func statusCodeToErrorCode(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return jira4claude.EValidation
	case http.StatusUnauthorized:
		return jira4claude.EUnauthorized
	case http.StatusForbidden:
		return jira4claude.EForbidden
	case http.StatusNotFound:
		return jira4claude.ENotFound
	case http.StatusConflict:
		return jira4claude.EConflict
	case http.StatusTooManyRequests:
		return jira4claude.ERateLimit
	default:
		return jira4claude.EInternal
	}
}
