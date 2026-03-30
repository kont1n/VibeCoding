package domain

import (
	"errors"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *AppError
		expected string
	}{
		{
			name: "with message",
			err: &AppError{
				Op:      "Test.Op",
				Message: "something went wrong",
			},
			expected: "Test.Op: something went wrong",
		},
		{
			name: "with underlying error",
			err: &AppError{
				Op:  "Test.Op",
				Err: errors.New("underlying error"),
			},
			expected: "Test.Op: underlying error",
		},
		{
			name: "with message and underlying error",
			err: &AppError{
				Op:      "Test.Op",
				Err:     errors.New("underlying error"),
				Message: "user message",
			},
			expected: "Test.Op: user message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("Error() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	underlying := errors.New("underlying error")
	appErr := &AppError{
		Op:  "Test.Op",
		Err: underlying,
	}

	unwrapped := appErr.Unwrap()
	if !errors.Is(unwrapped, underlying) {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlying)
	}
}

func TestAppError_Is(t *testing.T) {
	err1 := &AppError{Code: "NOT_FOUND"}
	err2 := &AppError{Code: "NOT_FOUND"}
	err3 := &AppError{Code: "INTERNAL_ERROR"}

	if !errors.Is(err1, err2) {
		t.Error("errors with same code should be equal")
	}

	if errors.Is(err1, err3) {
		t.Error("errors with different codes should not be equal")
	}
}

func TestNewAppError(t *testing.T) {
	underlying := errors.New("underlying error")
	err := NewAppError("Test.Op", underlying, "user message", "ERROR_CODE")

	if err.Op != "Test.Op" {
		t.Errorf("Op = %q, want %q", err.Op, "Test.Op")
	}
	if !errors.Is(err.Err, underlying) {
		t.Errorf("Err = %v, want %v", err.Err, underlying)
	}
	if err.Message != "user message" {
		t.Errorf("Message = %q, want %q", err.Message, "user message")
	}
	if err.Code != "ERROR_CODE" {
		t.Errorf("Code = %q, want %q", err.Code, "ERROR_CODE")
	}
}

func TestNotFoundError(t *testing.T) {
	err := NotFoundError("Test.Op", "User")

	if err.Code != "NOT_FOUND" {
		t.Errorf("Code = %q, want %q", err.Code, "NOT_FOUND")
	}
	if err.Message != "User not found" {
		t.Errorf("Message = %q, want %q", err.Message, "User not found")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Error("should be ErrNotFound")
	}
}

func TestInvalidInputError(t *testing.T) {
	err := InvalidInputError("Test.Op", "invalid email format")

	if err.Code != "INVALID_INPUT" {
		t.Errorf("Code = %q, want %q", err.Code, "INVALID_INPUT")
	}
	if err.Message != "invalid email format" {
		t.Errorf("Message = %q, want %q", err.Message, "invalid email format")
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Error("should be ErrInvalidInput")
	}
}

func TestAlreadyExistsError(t *testing.T) {
	err := AlreadyExistsError("Test.Op", "User")

	if err.Code != "ALREADY_EXISTS" {
		t.Errorf("Code = %q, want %q", err.Code, "ALREADY_EXISTS")
	}
	if err.Message != "User already exists" {
		t.Errorf("Message = %q, want %q", err.Message, "User already exists")
	}
	if !errors.Is(err, ErrAlreadyExists) {
		t.Error("should be ErrAlreadyExists")
	}
}

func TestProcessingError(t *testing.T) {
	underlying := errors.New("processing failed")
	err := ProcessingError("Test.Op", underlying)

	if err.Code != "PROCESSING_ERROR" {
		t.Errorf("Code = %q, want %q", err.Code, "PROCESSING_ERROR")
	}
	if !errors.Is(err.Err, underlying) {
		t.Error("should wrap underlying error")
	}
}

func TestInternalError(t *testing.T) {
	underlying := errors.New("internal failure")
	err := InternalError("Test.Op", underlying)

	if err.Code != "INTERNAL_ERROR" {
		t.Errorf("Code = %q, want %q", err.Code, "INTERNAL_ERROR")
	}
	if err.Message != "internal server error" {
		t.Errorf("Message = %q, want %q", err.Message, "internal server error")
	}
}
