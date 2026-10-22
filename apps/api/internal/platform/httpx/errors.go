// Package httpx holds the API's HTTP conventions: the JSON envelope, stable error codes,
// strict request decoding, pagination cursors and the middleware chain.
package httpx

import (
	"errors"
	"fmt"
	"net/http"
)

// Code is a stable, documented error code (docs/api/openapi.yaml#/components/schemas/ErrorCode).
type Code string

// Error codes. Clients branch on these, never on messages.
const (
	CodeUnauthenticated    Code = "UNAUTHENTICATED"
	CodeForbidden          Code = "FORBIDDEN"
	CodeNotFound           Code = "NOT_FOUND"
	CodeConflict           Code = "CONFLICT"
	CodeRateLimited        Code = "RATE_LIMITED"
	CodeSessionExpired     Code = "SESSION_EXPIRED"
	CodeUnsupportedCommand Code = "UNSUPPORTED_COMMAND"
	CodeValidationFailed   Code = "VALIDATION_FAILED"
	CodeStaleVersion       Code = "STALE_VERSION"
	CodeInternal           Code = "INTERNAL"
)

// FieldError describes one invalid input field.
type FieldError struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// Error is an error that is safe to show to API clients. Anything that is not an *Error is
// reported as INTERNAL with a generic message, and its details only reach the logs.
type Error struct {
	Status  int
	Code    Code
	Message string
	Details []FieldError
	// RetryAfterSeconds sets the Retry-After header (rate limits).
	RetryAfterSeconds int
	// cause is logged but never sent to the client.
	cause error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap exposes the internal cause to errors.Is/As.
func (e *Error) Unwrap() error { return e.cause }

// WithCause attaches an internal cause for logging.
func (e *Error) WithCause(err error) *Error {
	c := *e
	c.cause = err
	return &c
}

// Constructors for the common cases.

// Unauthenticated reports missing, invalid or expired credentials. The message is deliberately generic.
func Unauthenticated() *Error {
	return &Error{Status: http.StatusUnauthorized, Code: CodeUnauthenticated, Message: "Authentication required."}
}

// Forbidden reports a caller who is authenticated but not allowed.
func Forbidden() *Error {
	return &Error{Status: http.StatusForbidden, Code: CodeForbidden, Message: "You don't have permission to do that."}
}

// NotFound is also used for resources owned by someone else, so IDs can't be probed.
func NotFound(what string) *Error {
	return &Error{Status: http.StatusNotFound, Code: CodeNotFound, Message: what + " not found."}
}

// Conflict reports a state conflict.
func Conflict(message string) *Error {
	return &Error{Status: http.StatusConflict, Code: CodeConflict, Message: message}
}

// Validation reports invalid input.
func Validation(message string, details ...FieldError) *Error {
	return &Error{Status: http.StatusBadRequest, Code: CodeValidationFailed, Message: message, Details: details}
}

// RateLimited reports a rate limit with a retry hint.
func RateLimited(retryAfterSeconds int) *Error {
	return &Error{
		Status: http.StatusTooManyRequests, Code: CodeRateLimited,
		Message: "Too many requests. Try again shortly.", RetryAfterSeconds: retryAfterSeconds,
	}
}

// Internal hides the cause from the client.
func Internal(cause error) *Error {
	return (&Error{Status: http.StatusInternalServerError, Code: CodeInternal, Message: "Something went wrong."}).WithCause(cause)
}

// AsError converts any error into a client-safe *Error.
func AsError(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return Internal(err)
}
