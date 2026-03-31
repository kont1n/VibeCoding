// Package errors provides structured error types for the application.
package errors

import (
	"fmt"
	"net/http"
)

// ErrorCode represents a machine-readable error code.
type ErrorCode string

// Application error codes.
const (
	ErrUnknown         ErrorCode = "UNKNOWN"
	ErrValidation      ErrorCode = "VALIDATION_ERROR"
	ErrNotFound        ErrorCode = "NOT_FOUND"
	ErrAlreadyExists   ErrorCode = "ALREADY_EXISTS"
	ErrUnauthorized    ErrorCode = "UNAUTHORIZED"
	ErrForbidden       ErrorCode = "FORBIDDEN"
	ErrInternal        ErrorCode = "INTERNAL_ERROR"
	ErrFileUpload      ErrorCode = "FILE_UPLOAD_ERROR"
	ErrProcessing      ErrorCode = "PROCESSING_ERROR"
	ErrSessionNotFound ErrorCode = "SESSION_NOT_FOUND"
	ErrPersonNotFound  ErrorCode = "PERSON_NOT_FOUND"
	ErrInvalidFormat   ErrorCode = "INVALID_FORMAT"
	ErrPathTraversal   ErrorCode = "PATH_TRAVERSAL"
	ErrRateLimited     ErrorCode = "RATE_LIMITED"
)

// AppError represents a structured application error.
type AppError struct {
	// Code is a machine-readable error code.
	Code ErrorCode `json:"code"`
	// Message is a human-readable error message.
	Message string `json:"message"`
	// Details provides additional context (optional).
	Details map[string]any `json:"details,omitempty"`
	// Err is the underlying error (not serialized to JSON).
	Err error `json:"-"`
	// HTTPStatus is the HTTP status code to return.
	HTTPStatus int `json:"-"`
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error for errors.Is/As.
func (e *AppError) Unwrap() error {
	return e.Err
}

// WithDetails adds details to the error.
func (e *AppError) WithDetails(key string, value any) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]any)
	}
	e.Details[key] = value
	return e
}

// WithError sets the underlying error.
func (e *AppError) WithError(err error) *AppError {
	e.Err = err
	return e
}

// New creates a new AppError.
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: http.StatusInternalServerError,
	}
}

// Newf creates a new AppError with formatted message.
func Newf(code ErrorCode, format string, args ...any) *AppError {
	return &AppError{
		Code:       code,
		Message:    fmt.Sprintf(format, args...),
		HTTPStatus: http.StatusInternalServerError,
	}
}

// Wrap wraps an existing error with additional context.
func Wrap(err error, code ErrorCode, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		Err:        err,
		HTTPStatus: http.StatusInternalServerError,
	}
}

// Wrapf wraps an existing error with formatted message.
func Wrapf(err error, code ErrorCode, format string, args ...any) *AppError {
	return &AppError{
		Code:       code,
		Message:    fmt.Sprintf(format, args...),
		Err:        err,
		HTTPStatus: http.StatusInternalServerError,
	}
}

// Predefined error constructors for common cases.

// NewValidation creates a validation error.
func NewValidation(message string, details ...map[string]any) *AppError {
	err := &AppError{
		Code:       ErrValidation,
		Message:    message,
		HTTPStatus: http.StatusBadRequest,
	}
	if len(details) > 0 {
		err.Details = details[0]
	}
	return err
}

// NewNotFound creates a not found error.
func NewNotFound(resource, id string) *AppError {
	return &AppError{
		Code:       ErrNotFound,
		Message:    fmt.Sprintf("%s not found: %s", resource, id),
		HTTPStatus: http.StatusNotFound,
		Details:    map[string]any{"resource": resource, "id": id},
	}
}

// NewUnauthorized creates an unauthorized error.
func NewUnauthorized(message string) *AppError {
	return &AppError{
		Code:       ErrUnauthorized,
		Message:    message,
		HTTPStatus: http.StatusUnauthorized,
	}
}

// NewForbidden creates a forbidden error.
func NewForbidden(message string) *AppError {
	return &AppError{
		Code:       ErrForbidden,
		Message:    message,
		HTTPStatus: http.StatusForbidden,
	}
}

// NewConflict creates a conflict error.
func NewConflict(message string) *AppError {
	return &AppError{
		Code:       ErrAlreadyExists,
		Message:    message,
		HTTPStatus: http.StatusConflict,
	}
}

// NewInternal creates an internal server error.
func NewInternal(message string, err error) *AppError {
	return &AppError{
		Code:       ErrInternal,
		Message:    message,
		Err:        err,
		HTTPStatus: http.StatusInternalServerError,
	}
}

// NewFileUpload creates a file upload error.
func NewFileUpload(message string) *AppError {
	return &AppError{
		Code:       ErrFileUpload,
		Message:    message,
		HTTPStatus: http.StatusBadRequest,
	}
}

// NewPathTraversal creates a path traversal error.
func NewPathTraversal() *AppError {
	return &AppError{
		Code:       ErrPathTraversal,
		Message:    "Invalid path: path traversal detected",
		HTTPStatus: http.StatusBadRequest,
	}
}

// NewRateLimited creates a rate limit error.
func NewRateLimited() *AppError {
	return &AppError{
		Code:       ErrRateLimited,
		Message:    "Too many requests, please try again later",
		HTTPStatus: http.StatusTooManyRequests,
	}
}
