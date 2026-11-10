// Package http provides an HTTP client for Jira API communication.
//
// This package wraps net/http with Jira-specific authentication (via netrc)
// and error handling.
package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fwojciec/jira4claude"
	"github.com/jdx/go-netrc"
)

// Client is an HTTP client configured for Jira API requests.
type Client struct {
	baseURL        *url.URL
	username       string
	password       string
	httpClient     *http.Client
	maxRetries     int
	retryBaseDelay time.Duration
	timeout        time.Duration
	// transfer selects the idle rule of the timeout for this client's
	// requests (see WithTimeout); it is set by forTransfer, for uploads.
	transfer bool
}

// Option configures a Client.
type Option func(*clientConfig)

type clientConfig struct {
	netrcPath      string
	httpClient     *http.Client
	maxRetries     int
	retryBaseDelay time.Duration
	timeout        time.Duration
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

// WithMaxRetries sets the maximum number of retries for transient failures.
// Default is 3. Set to 0 to disable retries.
func WithMaxRetries(n int) Option {
	return func(c *clientConfig) {
		c.maxRetries = n
	}
}

// WithRetryBaseDelay sets the base delay for exponential backoff.
// Default is 100ms. Delays double each retry: baseDelay, 2*baseDelay, 4*baseDelay, etc.
func WithRetryBaseDelay(d time.Duration) Option {
	return func(c *clientConfig) {
		c.retryBaseDelay = d
	}
}

// WithTimeout bounds how long a request waits on the server. Zero, the
// default, means no bound.
//
// An API call (DoRequest) must be answered in full, headers and body, within
// d of each attempt being sent. An attachment transfer (Stream, and Upload)
// has no bound on its total duration: it fails only when no byte moves in
// either direction for d, so d is both the wait for the first byte of the
// answer and the longest pause allowed in the middle of a transfer.
//
// A request that times out is not retried: a server silent for d is not
// having a transient failure, and four attempts would multiply the wait by
// four. A write (any method but GET and HEAD) that times out may have been
// applied, and its error says so.
func WithTimeout(d time.Duration) Option {
	return func(c *clientConfig) {
		c.timeout = d
	}
}

// NewClient creates a new Client configured for the given Jira server.
// It reads credentials from the netrc file for authentication.
func NewClient(baseURL string, opts ...Option) (*Client, error) {
	cfg := &clientConfig{
		httpClient:     http.DefaultClient,
		maxRetries:     3,
		retryBaseDelay: 100 * time.Millisecond,
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
		return nil, &jira4claude.Error{
			Code:    jira4claude.EUnauthorized,
			Message: "could not read netrc file: " + err.Error(),
			Inner:   err,
		}
	}

	// Find the machine entry: host:port first when the URL has a port, then
	// the bare host name, as curl and git match it. The more specific entry
	// wins. Either way the credentials go only to the configured scheme,
	// host and port (see Do and Stream).
	names := []string{u.Host}
	if u.Port() != "" {
		names = append(names, u.Hostname())
	}
	var machine *netrc.Machine
	var name string
	for _, candidate := range names {
		if machine = n.Machine(candidate); machine != nil {
			name = candidate
			break
		}
	}
	if machine == nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EUnauthorized,
			Message: "no netrc entry for " + strings.Join(names, " or ") + " in " + netrcPath,
		}
	}

	// Validate credentials are present
	login := machine.Get("login")
	password := machine.Get("password")
	if login == "" {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EUnauthorized,
			Message: "netrc entry for " + name + ": login is required",
		}
	}
	if password == "" {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EUnauthorized,
			Message: "netrc entry for " + name + ": password is required",
		}
	}

	return &Client{
		baseURL:        u,
		username:       login,
		password:       password,
		httpClient:     cfg.httpClient,
		maxRetries:     cfg.maxRetries,
		retryBaseDelay: cfg.retryBaseDelay,
		timeout:        cfg.timeout,
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

// NewJSONRequest creates an HTTP request with a JSON body.
// It marshals the body to JSON, sets the Content-Type header, and returns the request.
func (c *Client) NewJSONRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to marshal request",
			Inner:   err,
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

// DoRequest executes an HTTP request and handles common response processing.
// It returns the response body on success, or an error if the request fails
// or the status code doesn't match the expected value.
//
// Retry policy depends on whether the method is idempotent:
//   - GET, PUT, DELETE (and any other non-POST method) are retried with
//     exponential backoff on 429, 5xx and connection errors.
//   - POST is retried on 429 only. A 5xx or connection error is returned at
//     once, because the server may already have applied the request and a
//     retry could duplicate it (for example a second issue or comment).
//
// The request body is rewound (via req.GetBody) before every retry.
func (c *Client) DoRequest(req *http.Request, expectedStatus int) ([]byte, error) {
	body, _, err := c.doRequest(req, expectedStatus)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// doRequest is DoRequest, also returning the status code of the last
// response (0 when the last attempt got none), so that a caller can word the
// error of a particular status itself.
func (c *Client) doRequest(req *http.Request, expectedStatus int) ([]byte, int, error) {
	var lastErr error
	lastStatus := 0
	idempotent := req.Method != http.MethodPost

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := c.retryBaseDelay * (1 << (attempt - 1)) // exponential backoff
			select {
			case <-time.After(delay):
			case <-req.Context().Done():
				return nil, lastStatus, &jira4claude.Error{
					Code:    jira4claude.EInternal,
					Message: "request cancelled during retry backoff",
					Inner:   req.Context().Err(),
				}
			}
			if err := rewindBody(req); err != nil {
				return nil, lastStatus, err
			}
		}

		body, statusCode, err := c.doRequestOnce(req)
		var timedOut *attemptTimeout
		if errors.As(err, &timedOut) {
			return nil, 0, c.timeoutError(req, timedOut)
		}
		if err != nil {
			connErr := &jira4claude.Error{
				Code:    jira4claude.EInternal,
				Message: "request failed",
				Inner:   err,
			}
			lastStatus = 0
			if !idempotent {
				return nil, lastStatus, mayHaveBeenApplied(req, connErr)
			}
			lastErr = connErr
			continue
		}

		lastStatus = statusCode
		if statusCode == expectedStatus {
			return body, statusCode, nil
		}

		statusErr := statusError(statusCode, body)

		switch {
		case statusCode == http.StatusTooManyRequests:
			// Rate limited: the request was not applied, so any method may retry.
			lastErr = statusErr
			continue
		case isRetryableStatus(statusCode) && idempotent:
			lastErr = statusErr
			continue
		case isRetryableStatus(statusCode):
			return nil, statusCode, mayHaveBeenApplied(req, statusErr)
		default:
			// The body goes back with the error, for a caller that words
			// the error of this status itself.
			return body, statusCode, statusErr
		}
	}

	return nil, lastStatus, lastErr
}

// withoutRedirects returns a copy of c whose requests never follow a
// redirect: the 3xx response is returned to the caller as it is. A request
// whose body can be rewound (GetBody) would otherwise send that body again to
// the redirect target on a 307 or 308, whatever its host.
func (c *Client) withoutRedirects() *Client {
	hc := *c.httpClient
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	cp := *c
	cp.httpClient = &hc
	return &cp
}

// maxStreamRedirects caps the redirects one Stream attempt follows. The cap
// is per attempt: a retried attempt starts again from the first URL.
const maxStreamRedirects = 5

// maxErrorBodyBytes bounds how much of an error response Stream reads.
const maxErrorBodyBytes = 64 << 10

// Stream GETs path on the configured server and copies a 200 response body
// to w, returning the number of bytes written. It sends Accept: */*, unlike
// Do, which asks for JSON.
//
// Redirects are followed by hand, at most maxStreamRedirects per attempt,
// because net/http's own redirect handling decides by host name alone (it
// ignores the port and allows subdomains) whether to resend the
// Authorization header, and a cookie jar ignores ports too:
//   - credentials are sent only while every URL so far has had the scheme,
//     host and port of the configured server; once a hop leaves it, no later
//     hop carries them, even one that leads back;
//   - no hop sends cookies (the client's jar is not used);
//   - a redirect to a scheme other than http or https is refused, and so is
//     one to plain http when the configured server uses https.
//
// GET is idempotent, so an attempt is retried with backoff, as in DoRequest,
// on a connection error, 429 or 5xx, on any hop, but only until the first
// byte reaches w: w cannot be rewound, so a failure while copying is
// returned as it is. A retried attempt starts again from path and follows
// the redirects again, so a server can see up to maxStreamRedirects+1
// requests per attempt. Too many redirects, or a refused redirect, is not
// transient and is never retried.
//
// No error that leaves Stream carries a URL the server supplied (a media
// URL carries a signed query string). Requests go straight to the
// configured client's transport, not through http.Client, which would parse
// a Location header itself and quote it whole in its error. For a hop after
// a redirect, errors name only the host and the status code: neither the
// transport's error text nor the response body is passed on. A failure
// writing to w is passed on, since it is the caller's own.
func (c *Client) Stream(ctx context.Context, path string, w io.Writer) (int64, error) {
	ref, err := url.Parse(path)
	if err != nil {
		return 0, &jira4claude.Error{Code: jira4claude.EInternal, Message: "invalid request path", Inner: err}
	}
	start := c.baseURL.ResolveReference(ref)

	// The configured client's transport, used without the client: no
	// redirect following, no cookie jar. Its Timeout is kept per attempt.
	rt := c.httpClient.Transport
	if rt == nil {
		rt = http.DefaultTransport
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := c.retryBaseDelay * (1 << (attempt - 1))
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return 0, &jira4claude.Error{
					Code:    jira4claude.EInternal,
					Message: "request cancelled during retry backoff",
					Inner:   ctx.Err(),
				}
			}
		}
		n, retry, err := c.streamAttempt(ctx, rt, start, w)
		if err == nil {
			return n, nil
		}
		if !retry || n > 0 {
			return n, err
		}
		lastErr = err
	}
	return 0, lastErr
}

// streamAttempt makes one Stream attempt, under the configured client's
// Timeout when it has one, and under the idle rule of the client's own
// timeout (see WithTimeout) when that is set: every response and every part
// of a body that arrives starts the countdown again, so a slow download that
// keeps moving is never cut. A timed-out attempt is not retried.
func (c *Client) streamAttempt(ctx context.Context, rt http.RoundTripper, start *url.URL, w io.Writer) (int64, bool, error) {
	if c.httpClient.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
	}
	if c.timeout <= 0 {
		return c.streamOnce(ctx, rt, start, w, func() {})
	}
	parent := ctx
	ctx, wd := startWatchdog(parent, c.timeout)
	defer wd.stop()
	n, retry, err := c.streamOnce(ctx, rt, start, w, wd.kick)
	if err != nil && parent.Err() == nil && errors.Is(context.Cause(ctx), errStalled) {
		return n, false, &jira4claude.TimeoutError{Err: &jira4claude.Error{
			Code: jira4claude.EInternal,
			Message: fmt.Sprintf("download timed out: nothing arrived for %s after %d bytes (--timeout sets the limit)",
				c.timeout, n),
		}}
	}
	return n, retry, err
}

// streamOnce makes one Stream attempt from start. retry reports whether the
// failure is transient and nothing was written.
func (c *Client) streamOnce(ctx context.Context, rt http.RoundTripper, start *url.URL, w io.Writer, kick func()) (n int64, retry bool, err error) {
	target := start
	withAuth := true
	for hop := 0; ; hop++ {
		redirected := hop > 0
		if !sameOrigin(target, c.baseURL) {
			withAuth = false
		}
		// The URL is set directly: building the request from target's
		// string form would parse it again, and a failure would quote it.
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "", nil)
		if err != nil {
			return 0, false, &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to create request"}
		}
		req.URL = target
		req.Host = target.Host
		req.Header.Set("Accept", "*/*")
		if withAuth {
			req.SetBasicAuth(c.username, c.password)
		}

		resp, err := rt.RoundTrip(req)
		if err != nil {
			return 0, true, hopError(ctx, "request to "+target.Host+" failed", err, redirected)
		}
		kick()
		// Every body counts as progress, a redirect's or an error's as well
		// as the file's: draining one must not look like silence.
		resp.Body = &progressReader{rc: resp.Body, kick: kick}

		switch {
		case resp.StatusCode == http.StatusOK:
			sw := &sinkWriter{w: w}
			n, err := io.Copy(sw, resp.Body)
			resp.Body.Close()
			if sw.err != nil {
				return n, false, &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to write the download", Inner: sw.err}
			}
			if err != nil {
				return n, true, hopError(ctx, "download from "+target.Host+" interrupted", err, redirected)
			}
			return n, false, nil

		case isRedirectStatus(resp.StatusCode):
			location := resp.Header.Get("Location")
			drainAndClose(resp.Body)
			if hop >= maxStreamRedirects {
				return 0, false, &jira4claude.Error{Code: jira4claude.EInternal, Message: fmt.Sprintf("too many redirects (more than %d)", maxStreamRedirects)}
			}
			next, err := c.redirectTarget(target, location)
			if err != nil {
				return 0, false, err
			}
			target = next

		case redirected:
			// Another host's body is not Jira's error format and may echo
			// the signed URL: report the status alone.
			drainAndClose(resp.Body)
			return 0, isRetryableStatus(resp.StatusCode), &jira4claude.Error{
				Code:    statusCodeToErrorCode(resp.StatusCode),
				Message: fmt.Sprintf("%s answered status %d", target.Host, resp.StatusCode),
			}

		default:
			body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
			drainAndClose(resp.Body)
			return 0, isRetryableStatus(resp.StatusCode), statusError(resp.StatusCode, body)
		}
	}
}

// hopError builds the error for a failed request or body read without the
// transport's error text, which can quote the server's response (a
// malformed header line, Location and signed URL included). Only causes
// that hold no response bytes are kept: a context error on any hop, and on
// the first hop (the configured host) a network or certificate error, which
// names only addresses and certificate names, or an unexpected end of body.
func hopError(ctx context.Context, msg string, err error, redirected bool) error {
	return &jira4claude.Error{Code: jira4claude.EInternal, Message: msg, Inner: safeCause(ctx, err, redirected)}
}

func safeCause(ctx context.Context, err error, redirected bool) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if redirected {
		return nil
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return opErr
	}
	var certErr *tls.CertificateVerificationError
	if errors.As(err, &certErr) {
		return certErr
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return io.ErrUnexpectedEOF
	}
	return nil
}

// sinkWriter records the error of the writer a download is copied to, so
// that it can be told apart from a failure reading the response.
type sinkWriter struct {
	w   io.Writer
	err error
}

func (s *sinkWriter) Write(p []byte) (int, error) {
	n, err := s.w.Write(p)
	if err != nil {
		s.err = err
	}
	return n, err
}

// redirectTarget resolves a Location header against the current URL and
// checks that it may be followed. Messages name at most the host of a
// location that parsed: media URLs carry signed query strings.
func (c *Client) redirectTarget(current *url.URL, location string) (*url.URL, error) {
	if location == "" {
		return nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "redirect without a Location header"}
	}
	next, err := current.Parse(location)
	if err != nil {
		return nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "the server sent an unusable redirect"}
	}
	scheme := strings.ToLower(next.Scheme)
	switch {
	case scheme != "http" && scheme != "https":
		return nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "refusing a redirect to a URL that is not HTTP or HTTPS"}
	case strings.EqualFold(c.baseURL.Scheme, "https") && scheme != "https":
		return nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "refusing a redirect to a non-HTTPS URL on " + next.Host}
	case next.Host == "":
		return nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "the server sent an unusable redirect"}
	}
	return next, nil
}

// sameOrigin reports whether a and b have the same scheme, host and port,
// with the scheme's default port filled in.
func sameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) &&
		strings.EqualFold(a.Hostname(), b.Hostname()) &&
		effectivePort(a) == effectivePort(b)
}

func effectivePort(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return "443"
	case "http":
		return "80"
	}
	return ""
}

func isRedirectStatus(code int) bool {
	switch code {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	}
	return false
}

// drainAndClose reads a little of body, so the connection can be reused, and
// closes it.
func drainAndClose(body io.ReadCloser) {
	// Errors ignored: the body is being discarded.
	_, _ = io.Copy(io.Discard, io.LimitReader(body, maxErrorBodyBytes))
	_ = body.Close()
}

// statusError builds the domain error for an unexpected status code, using the
// Jira error body when it has one.
func statusError(statusCode int, body []byte) error {
	if apiErr := ParseErrorResponse(statusCode, body); apiErr != nil {
		return apiErr
	}
	return &jira4claude.Error{
		Code:    statusCodeToErrorCode(statusCode),
		Message: fmt.Sprintf("unexpected status: %d", statusCode),
	}
}

// mayHaveBeenApplied wraps err for a non-idempotent request that failed in a
// way that does not tell whether the server applied it. The error code of err
// is kept.
func mayHaveBeenApplied(req *http.Request, err error) error {
	return &jira4claude.Error{
		Code: jira4claude.ErrorCode(err),
		Message: fmt.Sprintf("%s %s failed and was not retried; the request may have been applied, check before retrying",
			req.Method, req.URL.Path),
		Inner: err,
	}
}

// rewindBody resets req.Body for another attempt. Requests without a body or
// without GetBody are left unchanged.
func rewindBody(req *http.Request) error {
	if req.Body == nil || req.GetBody == nil {
		return nil
	}
	body, err := req.GetBody()
	if err != nil {
		return &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to rewind request body for retry",
			Inner:   err,
		}
	}
	req.Body = body
	return nil
}

// doRequestOnce executes a single HTTP request attempt, under the client's
// timeout when it has one (see WithTimeout). A timed-out attempt returns an
// *attemptTimeout.
func (c *Client) doRequestOnce(req *http.Request) ([]byte, int, error) {
	if c.timeout <= 0 {
		return c.exchange(req, func() {})
	}
	var ctx context.Context
	var stop func()
	kick := func() {}
	if c.transfer {
		var wd *watchdog
		ctx, wd = startWatchdog(req.Context(), c.timeout)
		stop, kick = wd.stop, wd.kick
	} else {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeoutCause(req.Context(), c.timeout, errDeadline)
		stop = cancel
	}
	defer stop()

	attempt := req.WithContext(ctx)
	if c.transfer && req.Body != nil && req.Body != http.NoBody {
		attempt.Body = &progressReader{rc: req.Body, kick: kick}
	}
	body, status, err := c.exchange(attempt, kick)
	if err != nil && req.Context().Err() == nil {
		if cause := context.Cause(ctx); errors.Is(cause, errDeadline) || errors.Is(cause, errStalled) {
			return nil, 0, &attemptTimeout{stalled: errors.Is(cause, errStalled)}
		}
	}
	return body, status, err
}

// exchange sends req and reads the whole response body, calling kick when
// the response arrives and for each part of its body.
func (c *Client) exchange(req *http.Request, kick func()) ([]byte, int, error) {
	resp, err := c.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	kick()

	body, err := io.ReadAll(&progressReader{rc: resp.Body, kick: kick})
	if err != nil {
		return nil, 0, err
	}

	return body, resp.StatusCode, nil
}

// Causes of a timed-out attempt's cancellation.
var (
	errDeadline = errors.New("no complete answer within the timeout")
	errStalled  = errors.New("no progress within the timeout")
)

// attemptTimeout is the error of an attempt cut by the client's timeout.
type attemptTimeout struct {
	// stalled is true for the idle rule of a transfer, false for the
	// deadline of an API call.
	stalled bool
}

func (e *attemptTimeout) Error() string {
	if e.stalled {
		return errStalled.Error()
	}
	return errDeadline.Error()
}

// timeoutError is the error of a request whose attempt timed out. Its code
// is EInternal, the code of network failures, which a timeout is one of. A
// write may have reached the server before the answer stopped coming, so
// its error says the request may have been applied.
func (c *Client) timeoutError(req *http.Request, e *attemptTimeout) error {
	what := fmt.Sprintf("no answer from Jira within %s", c.timeout)
	if e.stalled {
		what = fmt.Sprintf("the transfer stalled: nothing moved for %s", c.timeout)
	}
	msg := fmt.Sprintf("%s %s timed out (%s; --timeout sets the limit)", req.Method, req.URL.Path, what)
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		msg += "; it was not retried and may have been applied, check before retrying"
	}
	return &jira4claude.TimeoutError{Err: &jira4claude.Error{Code: jira4claude.EInternal, Message: msg}}
}

// watchdog cancels its context when kick has not been called for d: the idle
// rule of a transfer, which bounds the wait for the first byte and every
// pause after it, but not the total duration.
type watchdog struct {
	d      time.Duration
	timer  *time.Timer
	cancel context.CancelCauseFunc
}

func startWatchdog(parent context.Context, d time.Duration) (context.Context, *watchdog) {
	ctx, cancel := context.WithCancelCause(parent)
	w := &watchdog{d: d, cancel: cancel}
	w.timer = time.AfterFunc(d, func() { cancel(errStalled) })
	return ctx, w
}

// kick records progress: the countdown starts again.
func (w *watchdog) kick() {
	w.timer.Reset(w.d)
}

// stop ends the watchdog and releases its context.
func (w *watchdog) stop() {
	w.timer.Stop()
	w.cancel(nil)
}

// progressReader calls kick for every read that returns bytes.
type progressReader struct {
	rc   io.ReadCloser
	kick func()
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.rc.Read(b)
	if n > 0 {
		p.kick()
	}
	return n, err
}

func (p *progressReader) Close() error {
	return p.rc.Close()
}

// forTransfer returns a copy of c whose requests follow the transfer rule of
// the timeout (see WithTimeout), for an upload.
func (c *Client) forTransfer() *Client {
	cp := *c
	cp.transfer = true
	return &cp
}

// isRetryableStatus returns true if the status code indicates a transient error
// that should be retried.
func isRetryableStatus(statusCode int) bool {
	// 5xx server errors
	if statusCode >= 500 {
		return true
	}
	// 429 Too Many Requests (rate limit)
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	return false
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
	messages, err := errorMessages(body)
	if err != nil {
		return &jira4claude.Error{
			Code:    statusCodeToErrorCode(statusCode),
			Message: "failed to parse error response",
			Inner:   err,
		}
	}
	if len(messages) == 0 {
		return nil
	}

	return &jira4claude.Error{
		Code:    statusCodeToErrorCode(statusCode),
		Message: strings.Join(messages, "; "),
	}
}

// errorMessages returns the messages of a Jira error body: the general ones
// first, then the field errors as "field: message", sorted.
func errorMessages(body []byte) ([]string, error) {
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return nil, err
	}

	// Collect general error messages first.
	messages := make([]string, 0, len(errResp.ErrorMessages)+len(errResp.Errors))
	messages = append(messages, errResp.ErrorMessages...)

	// Add field-specific errors with field name prefix and sort for
	// deterministic output (Go map iteration order is random).
	fieldMessages := make([]string, 0, len(errResp.Errors))
	for field, msg := range errResp.Errors {
		fieldMessages = append(fieldMessages, field+": "+msg)
	}
	sort.Strings(fieldMessages)
	return append(messages, fieldMessages...), nil
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
	case http.StatusRequestEntityTooLarge:
		// The request is too large for the server: the input must change.
		return jira4claude.EValidation
	case http.StatusTooManyRequests:
		return jira4claude.ERateLimit
	default:
		return jira4claude.EInternal
	}
}
