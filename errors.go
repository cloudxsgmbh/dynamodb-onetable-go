/*
Package onetable – error types.

Mirrors JS: OneTableError / OneTableArgError.
*/
package onetable

import "fmt"

// ErrorCode is a well-known error category string.
type ErrorCode string

const (
	ErrArgument   ErrorCode = "ArgumentError"
	ErrValidation ErrorCode = "ValidationError"
	ErrMissing    ErrorCode = "MissingError"
	ErrNonUnique  ErrorCode = "NonUniqueError"
	ErrUnique     ErrorCode = "UniqueError"
	ErrNotFound   ErrorCode = "NotFoundError"
	ErrRuntime    ErrorCode = "RuntimeError"
	ErrType       ErrorCode = "TypeError"
)

// OneTableError is the general runtime error. It carries an optional Code and
// a free-form Context map for extra debugging data.
type OneTableError struct {
	Message string
	Code    ErrorCode
	Context map[string]any
	Cause   error
}

func (e *OneTableError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("[%s] %s", e.Code, e.Message)
	}
	return e.Message
}

func (e *OneTableError) Unwrap() error { return e.Cause }

// NewError constructs a OneTableError.
func NewError(msg string, opts ...func(*OneTableError)) *OneTableError {
	err := &OneTableError{Message: msg}
	for _, o := range opts {
		o(err)
	}
	return err
}

// WithCode sets the error code.
func WithCode(c ErrorCode) func(*OneTableError) {
	return func(e *OneTableError) { e.Code = c }
}

// WithContext attaches a context map.
func WithContext(ctx map[string]any) func(*OneTableError) {
	return func(e *OneTableError) { e.Context = ctx }
}

// WithCause wraps an underlying error.
func WithCause(cause error) func(*OneTableError) {
	return func(e *OneTableError) { e.Cause = cause }
}

// OneTableArgError is for invalid argument / configuration errors.
type OneTableArgError struct {
	Message string
	Code    ErrorCode
	Context map[string]any
}

func (e *OneTableArgError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("[%s] %s", e.Code, e.Message)
	}
	return e.Message
}

// NewArgError constructs a OneTableArgError.
func NewArgError(msg string, code ...ErrorCode) *OneTableArgError {
	c := ErrArgument
	if len(code) > 0 {
		c = code[0]
	}
	return &OneTableArgError{Message: msg, Code: c}
}
