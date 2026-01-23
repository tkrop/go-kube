package errors

import (
	"errors"
	"fmt"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

var (
	// Is checks whether the given error matches a target error.
	Is = errors.Is
	// As casts the given error to a target type.
	As = errors.As
	// Join combines multiple errors into a single error.
	Join = errors.Join
)

// Kubernetes error checkers.
var (
	// IsNotFound checks whether the given error is a Kubernetes not found
	// error.
	IsNotFound = kerrors.IsNotFound

	// IsAlreadyExists checks whether the given error is a Kubernetes already
	// exists error.
	IsAlreadyExists = kerrors.IsAlreadyExists

	// IsConflict checks whether the given error is a Kubernetes conflict
	// error.
	IsConflict = kerrors.IsConflict

	// IsInvalid checks whether the given error is a Kubernetes invalid error.
	IsInvalid = kerrors.IsInvalid

	// IsGone checks whether the given error is a Kubernetes gone error.
	IsGone = kerrors.IsGone

	// IsResourceExpired checks whether the given error is a Kubernetes
	// resource expired error.
	IsResourceExpired = kerrors.IsResourceExpired

	// IsNotAcceptable checks whether the given error is a Kubernetes not
	// acceptable error.
	IsNotAcceptable = kerrors.IsNotAcceptable

	// IsUnsupportedMediaType checks whether the given error is a Kubernetes
	// unsupported media type error.
	IsUnsupportedMediaType = kerrors.IsUnsupportedMediaType

	// IsMethodNotSupported checks whether the given error is a Kubernetes
	// method not supported error.
	IsMethodNotSupported = kerrors.IsMethodNotSupported

	// IsServiceUnavailable checks whether the given error is a Kubernetes
	// service unavailable error.
	IsServiceUnavailable = kerrors.IsServiceUnavailable

	// IsBadRequest checks whether the given error is a Kubernetes bad request
	// error.
	IsBadRequest = kerrors.IsBadRequest

	// IsUnauthorized checks whether the given error is a Kubernetes
	// unauthorized error.
	IsUnauthorized = kerrors.IsUnauthorized

	// IsForbidden checks whether the given error is a Kubernetes forbidden
	// error.
	IsForbidden = kerrors.IsForbidden

	// IsTimeout checks whether the given error is a Kubernetes timeout error.
	IsTimeout = kerrors.IsTimeout

	// IsServerTimeout checks whether the given error is a Kubernetes server
	// timeout error.
	IsServerTimeout = kerrors.IsServerTimeout

	// IsInternalError checks whether the given error is a Kubernetes internal
	// error.
	IsInternalError = kerrors.IsInternalError

	// IsTooManyRequests checks whether the given error is a Kubernetes too
	// many requests error.
	IsTooManyRequests = kerrors.IsTooManyRequests

	// IsRequestEntityTooLargeError checks whether the given error is a
	// Kubernetes request entity too large error.
	IsRequestEntityTooLargeError = kerrors.IsRequestEntityTooLargeError

	// IsUnexpectedServerError checks whether the given error is a Kubernetes
	// unexpected server error.
	IsUnexpectedServerError = kerrors.IsUnexpectedServerError

	// IsUnexpectedObjectError checks whether the given error is a Kubernetes
	// unexpected object error.
	IsUnexpectedObjectError = kerrors.IsUnexpectedObjectError

	// IsStoreReadError checks whether the given error is a Kubernetes store
	// read error.
	IsStoreReadError = kerrors.IsStoreReadError
)

// Error is an abstract error type wrapping a given error.
type Error struct {
	err error
}

// New creates a new wrapped error type with the given error message.
func New(msg string) *Error {
	return &Error{err: errors.New(msg)}
}

// Wrap creates a wrapped error type with the given base error.
func Wrap(err error) *Error {
	return &Error{err: err}
}

// Unwrap unwraps the underlying error.
func (e *Error) Unwrap() error {
	return e.err
}

// Is compares the underlying error with a target error.
func (e *Error) Is(target error) bool {
	return errors.Is(e.err, target)
}

// As casts the underlying error to a target type.
func (e *Error) As(target any) bool {
	return errors.As(e.err, target)
}

// Error returns the error message of the underlying error.
func (e *Error) Error() string {
	return e.err.Error()
}

// String returns the string representation of the underlying error.
func (e *Error) String() string {
	return e.err.Error()
}

// New creates a new specific error instance of the underlying error. The error
// message is created by formatting the given message and arguments appending
// it separated by a hyphen to the underlying error.
func (e *Error) New(msg string, args ...any) error {
	args = append([]any{e}, args...)

	//nolint:goerr113 // ignore error check.
	return fmt.Errorf("%w - "+msg, args...)
}

// Wrap is a convenience method that wraps a given error instance using the
// underlying error. If no error is provided as final argument, it returns nil.
func (e *Error) Wrap(msg string, args ...any) error {
	if args[len(args)-1] == nil {
		return nil
	}

	return e.New(msg, args...)
}
