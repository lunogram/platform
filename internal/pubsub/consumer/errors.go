package consumer

import (
	"errors"
	"fmt"
)

// PermanentError wraps an error to indicate that it is a permanent failure
// and should not be retried. When the router encounters a PermanentError it
// uses msg.Term() to tell NATS JetStream to stop redelivering the message
// instead of the normal msg.Nak() which triggers a retry.
//
// Use Permanent() to wrap errors that will never succeed on retry, such as
// validation failures, missing configuration, or invalid recipient addresses.
type PermanentError struct {
	err error
}

// Permanent wraps an error as a PermanentError.
// Returns nil if err is nil.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &PermanentError{err: err}
}

// Permanentf creates a new PermanentError with a formatted message.
func Permanentf(format string, args ...any) error {
	return &PermanentError{err: fmt.Errorf(format, args...)}
}

func (e *PermanentError) Error() string {
	return e.err.Error()
}

func (e *PermanentError) Unwrap() error {
	return e.err
}

// IsPermanent reports whether err is (or wraps) a PermanentError.
func IsPermanent(err error) bool {
	var pe *PermanentError
	return errors.As(err, &pe)
}
