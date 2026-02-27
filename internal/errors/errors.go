package errors

import (
	"fmt"
	"strings"
)

// Error codes
const (
	EConnectionFailed  = "E001"
	EAuthFailed       = "E002"
	EPermissionDenied = "E003"
	ESchemaTooLarge   = "E004"
	ETimeout          = "E005"
	EOutputWriteFailed = "E006"
)

// Error represents a structured error
type Error struct {
	Code    string
	Message string
	Hint    string
	Err     error
}

// Error implements the error interface
func (e *Error) Error() string {
	msg := fmt.Sprintf("[%s] %s", e.Code, e.Message)
	if e.Err != nil {
		msg += fmt.Sprintf(": %v", e.Err)
	}
	if e.Hint != "" {
		msg += fmt.Sprintf("\nHint: %s", e.Hint)
	}
	return msg
}

// Unwrap returns the underlying error
func (e *Error) Unwrap() error {
	return e.Err
}

// New creates a new structured error
func New(code, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// WithError wraps an existing error
func (e *Error) WithError(err error) *Error {
	e.Err = err
	return e
}

// WithHint adds a hint to the error
func (e *Error) WithHint(hint string) *Error {
	e.Hint = hint
	return e
}

// ConnectionFailed creates an E001 error
func ConnectionFailed(err error) *Error {
	return New(EConnectionFailed, "Connection failed").
		WithError(err).
		WithHint("Check your database URL and ensure the server is running")
}

// AuthFailed creates an E002 error
func AuthFailed(err error) *Error {
	return New(EAuthFailed, "Authentication failed").
		WithError(err).
		WithHint("Verify your username and password")
}

// PermissionDenied creates an E003 error
func PermissionDenied(err error) *Error {
	return New(EPermissionDenied, "Permission denied").
		WithError(err).
		WithHint("Ensure your user has SELECT privileges on the schema")
}

// SchemaTooLarge creates an E004 error
func SchemaTooLarge(tableCount int) *Error {
	return New(ESchemaTooLarge, fmt.Sprintf("Schema too large (%d tables)", tableCount)).
		WithHint("Consider using --ignore-patterns to filter tables")
}

// Timeout creates an E005 error
func Timeout(operation string, err error) *Error {
	return New(ETimeout, fmt.Sprintf("Timeout during %s", operation)).
		WithError(err).
		WithHint("Increase the --timeout value")
}

// OutputWriteFailed creates an E006 error
func OutputWriteFailed(path string, err error) *Error {
	return New(EOutputWriteFailed, fmt.Sprintf("Failed to write to %s", path)).
		WithError(err).
		WithHint("Check file permissions and disk space")
}

// Is checks if the error code matches
func Is(err error, code string) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == code
	}
	return strings.Contains(err.Error(), code)
}
