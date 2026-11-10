package jira4claude

import "errors"

// Error codes for categorizing errors.
const (
	ENotFound     = "not_found"
	EConflict     = "conflict"
	EUnauthorized = "unauthorized"
	EForbidden    = "forbidden"
	EValidation   = "validation"
	ERateLimit    = "rate_limit"
	EInternal     = "internal"
)

// Error represents an application-level error with a code, message, and optional
// wrapped error. This follows the Ben Johnson error pattern.
type Error struct {
	// Code is an application-defined error code for programmatic handling.
	Code string

	// Message is a human-readable message safe to show to end users.
	Message string

	// Inner is the underlying error, if any, for debugging purposes.
	Inner error
}

// Error returns a human-readable error message.
func (e *Error) Error() string {
	if e.Message != "" {
		if e.Inner != nil {
			return e.Message + ": " + e.Inner.Error()
		}
		return e.Message
	}
	if e.Inner != nil {
		return e.Inner.Error()
	}
	return "an error occurred"
}

// Unwrap returns the inner error for use with errors.Is and errors.As.
func (e *Error) Unwrap() error {
	return e.Inner
}

// ReadBackError is the error of a write that Jira applied but whose result
// could not be read back afterwards. Err is the read's error; ErrorCode and
// ExitCode see through to its code. A caller reports the write as done and
// must not repeat it.
type ReadBackError struct {
	Err error
}

// Error returns the read's error, saying that the write was applied.
func (e *ReadBackError) Error() string {
	return "the write was applied, but reading it back failed: " + e.Err.Error()
}

// Unwrap returns the read's error.
func (e *ReadBackError) Unwrap() error {
	return e.Err
}

// TimeoutError marks a request that got no answer, or stopped moving, within
// the client's timeout. Err is the error to report; ErrorCode, ErrorMessage
// and ExitCode see through to it. A command working through several items
// stops at a timeout: the next request would most likely wait as long.
type TimeoutError struct {
	Err error
}

// Error returns the error to report.
func (e *TimeoutError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the error to report.
func (e *TimeoutError) Unwrap() error {
	return e.Err
}

// IsTimeout reports whether err is, or wraps, a *TimeoutError.
func IsTimeout(err error) bool {
	var t *TimeoutError
	return errors.As(err, &t)
}

// ErrorCode returns the error code from err if it is a jira4claude.Error,
// or EInternal for other errors. Returns empty string for nil errors.
func ErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return EInternal
}

// ErrorMessage returns a human-readable message from err. For jira4claude.Error,
// it uses the same fallback logic as Error() (Message → Inner → generic).
// For other errors, returns err.Error(). Returns empty string for nil errors.
func ErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Error()
	}
	return err.Error()
}

// ExitCode returns a semantic exit code for the given error.
// This allows AI agents to programmatically distinguish error types.
//
// Exit codes:
//   - 0: Success (nil error)
//   - 1: Validation error
//   - 2: Unauthorized (authentication failed)
//   - 3: Forbidden (permission denied)
//   - 4: Not found
//   - 5: Conflict
//   - 6: Rate limit exceeded
//   - 7: Internal error (default for unknown errors)
func ExitCode(err error) int {
	if err == nil {
		return 0
	}

	code := ErrorCode(err)
	switch code {
	case EValidation:
		return 1
	case EUnauthorized:
		return 2
	case EForbidden:
		return 3
	case ENotFound:
		return 4
	case EConflict:
		return 5
	case ERateLimit:
		return 6
	default:
		return 7
	}
}
