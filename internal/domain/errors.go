// Package domain provides domain entities and error types.
package domain

import (
	"errors"
	"fmt"
)

// Common domain errors.
var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidInput  = errors.New("invalid input")
	ErrAlreadyExists = errors.New("already exists")
	ErrProcessing    = errors.New("processing error")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrForbidden     = errors.New("forbidden")
	ErrConflict      = errors.New("conflict")
	ErrInternal      = errors.New("internal error")
)

// AppError represents an application-level error with context.
type AppError struct {
	Op      string // Operation name, e.g., "PersonService.GroupFaces".
	Err     error  // Underlying error.
	Message string // User-friendly message.
	Code    string // Error code for API responses.
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s: %s", e.Op, e.Message)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

// Unwrap implements the errors.Unwrap interface.
func (e *AppError) Unwrap() error {
	return e.Err
}

// Is implements the errors.Is interface for error type comparison.
func (e *AppError) Is(target error) bool {
	if t, ok := target.(*AppError); ok {
		return e.Code == t.Code
	}
	return errors.Is(e.Err, target)
}

// NewAppError creates a new AppError.
func NewAppError(op string, err error, message, code string) *AppError {
	return &AppError{
		Op:      op,
		Err:     err,
		Message: message,
		Code:    code,
	}
}

// NotFoundError creates a not found error.
func NotFoundError(op, resource string) *AppError {
	return &AppError{
		Op:      op,
		Err:     ErrNotFound,
		Message: fmt.Sprintf("%s not found", resource),
		Code:    "NOT_FOUND",
	}
}

// InvalidInputError creates an invalid input error.
func InvalidInputError(op, message string) *AppError {
	return &AppError{
		Op:      op,
		Err:     ErrInvalidInput,
		Message: message,
		Code:    "INVALID_INPUT",
	}
}

// AlreadyExistsError creates an already exists error.
func AlreadyExistsError(op, resource string) *AppError {
	return &AppError{
		Op:      op,
		Err:     ErrAlreadyExists,
		Message: fmt.Sprintf("%s already exists", resource),
		Code:    "ALREADY_EXISTS",
	}
}

// ProcessingError creates a processing error.
func ProcessingError(op string, err error) *AppError {
	return &AppError{
		Op:      op,
		Err:     err,
		Message: "processing failed",
		Code:    "PROCESSING_ERROR",
	}
}

// InternalError creates an internal server error.
func InternalError(op string, err error) *AppError {
	return &AppError{
		Op:      op,
		Err:     err,
		Message: "internal server error",
		Code:    "INTERNAL_ERROR",
	}
}
