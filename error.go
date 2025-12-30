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
