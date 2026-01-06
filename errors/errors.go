package errors

import (
	"errors"
	"fmt"
)

// Error is an abstract error type wrapping a given error.
type Error struct {
	err error
}

// New creates a new error type with the given base error.
func New(err error) *Error {
	return &Error{err: err}
}

// NewError creates a new error type with the given error message.
func NewError(msg string) *Error {
	return &Error{err: errors.New(msg)}
}

// Unwrap unwraps the underlying error.
func (e Error) Unwrap() error {
	return e.err
}

// Is compares the underlying error with a target error.
func (e Error) Is(target error) bool {
	return errors.Is(e.err, target)
}

// As casts the underlying error to a target type.
func (e Error) As(target any) bool {
	return errors.As(e.err, target)
}

// Error returns the error message of the underlying error.
func (e Error) Error() string {
	return e.err.Error()
}

// String returns the string representation of the underlying error.
func (e Error) String() string {
	return e.err.Error()
}

// New creates a new specific error instance of the underlying error. The error
// message is created by formatting the given message and arguments appending
// it separated by a hyphen to the underlying error.
func (e Error) New(msg string, args ...any) error {
	args = append([]any{e.err}, args...)

	//nolint:goerr113 // ignore error check.
	return fmt.Errorf("%w - "+msg, args...)
}

// Wrap is a convenience method that wraps a given error instance using the
// underlying error. If no error is provided as final argument, it returns nil.
func (e Error) Wrap(msg string, args ...any) error {
	if args[len(args)-1] == nil {
		return nil
	}

	return e.New(msg, args...)
}
