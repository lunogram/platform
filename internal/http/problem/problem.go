package problem

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrUnimplemented is thrown whenever an unimplemented endpoint has been called.
var ErrUnimplemented = ErrorFunc(WithStatus(NewError("unimplemented endpoint", "the requested endpoint has not yet been implemented"), http.StatusNotImplemented))

// ErrUnauthorized is thrown whenever a user calls an unauthorized endpoint.
var ErrUnauthorized = ErrorFunc(WithStatus(NewError("unauthorized", "you are unauthorized to access the given endpoint"), http.StatusUnauthorized))

// ErrNotFound is thrown whenever the requested resource could not be found.
var ErrNotFound = ErrorFunc(WithStatus(NewError("resource not found", "the requested resource has not been found"), http.StatusNotFound))

// ErrInternal is thrown whenever an internal server error has occurred.
var ErrInternal = ErrorFunc(WithStatus(NewError("internal server error", "an internal server error has occurred"), http.StatusInternalServerError))

// ErrBadRequest is thrown whenever the client has sent a bad request.
var ErrBadRequest = ErrorFunc(WithStatus(NewError("bad request", "the request could not be processed"), http.StatusBadRequest))

// ErrForbidden is thrown whenever the user does not have permission to access a resource.
var ErrForbidden = ErrorFunc(WithStatus(NewError("forbidden", "you do not have permission to access this resource"), http.StatusForbidden))

// ErrConflict is thrown whenever a resource conflicts with an existing one.
var ErrConflict = ErrorFunc(WithStatus(NewError("conflict", "the resource already exists"), http.StatusConflict))

// ErrTooManyRequests is thrown when a client exceeds its rate limit.
var ErrTooManyRequests = ErrorFunc(WithStatus(NewError("too many requests", "the rate limit for this request has been exceeded"), http.StatusTooManyRequests))

// NewError creates a new error with the given title and description.
func NewError(title, description string) error {
	return &withDescription{
		title:       title,
		description: description,
	}
}

type withDescription struct {
	cause       error
	title       string
	description string
}

func (w *withDescription) Error() string {
	return fmt.Sprintf("%s\n\n%s", w.title, w.description)
}

func (w *withDescription) Unwrap() error { return w.cause }

// WithDescription wraps a human readable title and description around the given
// error. The title should represent a short summary of the problem. The details
// should include details on how to resolve the given error. If multiple details
// are given they will be joined using a new line character.
func WithDescription(err error, title string, description ...string) error {
	if err == nil {
		return nil
	}

	return &withDescription{
		cause:       err,
		title:       title,
		description: strings.Join(description, "\n"),
	}
}

// AsDescription sets the error content as the error description.
func AsDescription(err error) error {
	return WithDescription(err, "", err.Error())
}

// GetDescription attempts to unwrap the human readable description of the given
// error. The default HTTP status text for the underlaying probem status is
// returned if no description has been found.
func GetDescription(err error) (title string, description string) {
	if c, ok := err.(*withDescription); ok {
		return c.title, c.description
	}

	if n := errors.Unwrap(err); n != nil {
		return GetDescription(n)
	}

	return strings.ToLower(http.StatusText(int(GetStatus(err)))), ""
}

type withStatus struct {
	cause  error
	status int
}

func (w *withStatus) Error() string { return w.cause.Error() }
func (w *withStatus) Unwrap() error { return w.cause }

// WithStatus wraps the given status around the given error. This status is
// returned to the client defining the error category.
func WithStatus(err error, status int) error {
	return &withStatus{
		cause:  err,
		status: status,
	}
}

// GetStatus returns the underlying error status. Statuses are based on the
// registered IANA statuses. If no status has been found is the default internal
// server error status code returned.
// https://www.iana.org/assignments/http-status-codes/http-status-codes.xhtml
func GetStatus(err error) (status int) {
	if c, ok := err.(*withStatus); ok {
		return c.status
	}

	if n := errors.Unwrap(err); n != nil {
		return GetStatus(n)
	}

	return http.StatusInternalServerError
}

// Option is a functional option for configuring error behavior
type Option func(error) error

// Describe creates an option that adds a description to an error
func Describe(description string) Option {
	return func(err error) error {
		var wd *withDescription
		if errors.As(err, &wd) {
			// If the error already has a withDescription, we need to update the description
			// while preserving the full error chain. We wrap the original error (not just wd.cause)
			// to maintain the status information that might be wrapped around it.
			return &withDescription{
				cause:       err,
				title:       wd.title,
				description: description,
			}
		}
		// Otherwise, wrap it with a new withDescription, preserving the original error in the cause
		// so that Unwrap() can find the status information
		return &withDescription{
			cause:       err,
			title:       "",
			description: description,
		}
	}
}

// ErrorFunc creates a function that can accept options to customize the error
func ErrorFunc(err error) func(...Option) error {
	return func(opts ...Option) error {
		result := err
		for _, opt := range opts {
			result = opt(result)
		}
		return result
	}
}
