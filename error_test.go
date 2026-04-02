package xcore

import (
	"errors"
	"net/http"
	"testing"
)

func TestErrorCode_Constants(t *testing.T) {
	tests := []struct {
		code       ErrorCode
		wantStatus int
	}{
		{ErrCodeInternal, http.StatusInternalServerError},
		{ErrCodeValidation, http.StatusBadRequest},
		{ErrCodeNotFound, http.StatusNotFound},
		{ErrCodeUnauthorized, http.StatusUnauthorized},
		{ErrCodeForbidden, http.StatusForbidden},
		{ErrCodeBadRequest, http.StatusBadRequest},
		{ErrCodeConflict, http.StatusConflict},
		{ErrCodeTooManyRequests, http.StatusTooManyRequests},
		{ErrCodeServiceUnavailable, http.StatusServiceUnavailable},
		{ErrCodeInvalidToken, http.StatusUnauthorized},
		{ErrCodeTokenExpired, http.StatusUnauthorized},
		{ErrCodeRateLimitExceeded, http.StatusTooManyRequests},
		{ErrCodeDatabaseError, http.StatusInternalServerError},
		{ErrCodeCacheError, http.StatusInternalServerError},
		{ErrCodeExternalAPI, http.StatusInternalServerError},
		{ErrCodeTimeout, http.StatusGatewayTimeout},
		{ErrCodeCanceled, http.StatusRequestTimeout},
		{ErrCodeAlreadyExists, http.StatusConflict},
		{ErrCodeInvalidInput, http.StatusBadRequest},
		{ErrCodeGatewayTimeout, http.StatusGatewayTimeout},
		{ErrCodeMethodNotAllowed, http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			if got := mapErrorCodeToHTTP(tt.code); got != tt.wantStatus {
				t.Errorf("mapErrorCodeToHTTP(%s) = %d, want %d", tt.code, got, tt.wantStatus)
			}
		})
	}
}

func TestXError_Error(t *testing.T) {
	e := &XError{
		Code:    ErrCodeNotFound,
		Message: "Resource not found",
	}

	if got := e.Error(); got != "NOT_FOUND: Resource not found" {
		t.Errorf("Error() = %s, want NOT_FOUND: Resource not found", got)
	}
}

func TestXError_Error_WithCause(t *testing.T) {
	cause := errors.New("original error")
	e := &XError{
		Code:    ErrCodeDatabaseError,
		Message: "Database operation failed",
		cause:   cause,
	}

	expected := "DATABASE_ERROR: Database operation failed - original error"
	if got := e.Error(); got != expected {
		t.Errorf("Error() = %s, want %s", got, expected)
	}
}

func TestXError_Unwrap(t *testing.T) {
	cause := errors.New("original error")
	e := &XError{
		Code:    ErrCodeDatabaseError,
		Message: "Database operation failed",
		cause:   cause,
	}

	if got := e.Unwrap(); got != cause {
		t.Errorf("Unwrap() = %v, want %v", got, cause)
	}
}

func TestXError_Is(t *testing.T) {
	e := &XError{
		Code:    ErrCodeNotFound,
		Message: "Resource not found",
	}

	target := &XError{Code: ErrCodeNotFound}
	if !e.Is(target) {
		t.Error("Is() should return true for matching XError target")
	}

	targetUnauthorized := &XError{Code: ErrCodeUnauthorized}
	if e.Is(targetUnauthorized) {
		t.Error("Is() should return false for non-matching XError target")
	}
}

func TestXError_WithMeta(t *testing.T) {
	e := NewError(ErrCodeNotFound, "Resource not found")
	e = e.WithMeta("key", "value")

	if e.Meta == nil {
		t.Error("Meta should not be nil after WithMeta")
	}
	if e.Meta["key"] != "value" {
		t.Errorf("Meta[key] = %v, want value", e.Meta["key"])
	}
}

func TestXError_WithMeta_Chained(t *testing.T) {
	e := NewError(ErrCodeNotFound, "Resource not found")
	e = e.WithMeta("key1", "value1").WithMeta("key2", "value2")

	if e.Meta["key1"] != "value1" || e.Meta["key2"] != "value2" {
		t.Error("chained WithMeta should set both keys")
	}
}

func TestXError_WithErrors(t *testing.T) {
	errs := []ValidationError{
		{Field: "name", Message: "Name is required"},
		{Field: "email", Message: "Invalid email format"},
	}

	e := NewError(ErrCodeValidation, "Validation failed")
	e = e.WithErrors(errs)

	if len(e.Errors) != 2 {
		t.Errorf("Errors length = %d, want 2", len(e.Errors))
	}
	if e.Errors[0].Field != "name" {
		t.Errorf("Errors[0].Field = %s, want name", e.Errors[0].Field)
	}
}

func TestNewError(t *testing.T) {
	e := NewError(ErrCodeNotFound, "Resource not found")

	if e.Code != ErrCodeNotFound {
		t.Errorf("Code = %s, want NOT_FOUND", e.Code)
	}
	if e.Message != "Resource not found" {
		t.Errorf("Message = %s, want 'Resource not found'", e.Message)
	}
	if e.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want %d", e.StatusCode, http.StatusNotFound)
	}
}

func TestNewErrorWithStatus(t *testing.T) {
	e := NewErrorWithStatus(ErrCodeInternal, "Internal error", 500)

	if e.Code != ErrCodeInternal {
		t.Errorf("Code = %s, want INTERNAL_ERROR", e.Code)
	}
	if e.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", e.StatusCode)
	}
}

func TestWrapError(t *testing.T) {
	cause := errors.New("original error")
	e := WrapError(cause, ErrCodeDatabaseError, "Database operation failed")

	if e.Code != ErrCodeDatabaseError {
		t.Errorf("Code = %s, want DATABASE_ERROR", e.Code)
	}
	if e.Message != "Database operation failed" {
		t.Errorf("Message = %s, want 'Database operation failed'", e.Message)
	}
	if e.cause != cause {
		t.Error("cause not preserved")
	}
}

func TestWrapErrorWithStatus(t *testing.T) {
	cause := errors.New("original error")
	e := WrapErrorWithStatus(cause, ErrCodeInternal, "Internal error", 503)

	if e.Code != ErrCodeInternal {
		t.Errorf("Code = %s, want INTERNAL_ERROR", e.Code)
	}
	if e.StatusCode != 503 {
		t.Errorf("StatusCode = %d, want 503", e.StatusCode)
	}
	if e.cause != cause {
		t.Error("cause not preserved")
	}
}

func TestNewValidationError(t *testing.T) {
	ve := NewValidationError("email", "Invalid email format")

	if ve.Field != "email" {
		t.Errorf("Field = %s, want email", ve.Field)
	}
	if ve.Message != "Invalid email format" {
		t.Errorf("Message = %s, want 'Invalid email format'", ve.Message)
	}
}

func TestErrInvalidToken(t *testing.T) {
	e := ErrInvalidToken

	if e.Code != ErrCodeInvalidToken {
		t.Errorf("Code = %s, want INVALID_TOKEN", e.Code)
	}
	if e.StatusCode != http.StatusUnauthorized {
		t.Errorf("StatusCode = %d, want %d", e.StatusCode, http.StatusUnauthorized)
	}
}

func TestErrTokenExpired(t *testing.T) {
	e := ErrTokenExpired

	if e.Code != ErrCodeTokenExpired {
		t.Errorf("Code = %s, want TOKEN_EXPIRED", e.Code)
	}
}

func TestErrServiceUnavailable(t *testing.T) {
	e := ErrServiceUnavailable

	if e.Code != ErrCodeServiceUnavailable {
		t.Errorf("Code = %s, want SERVICE_UNAVAILABLE", e.Code)
	}
}

func TestErrInternal(t *testing.T) {
	e := ErrInternal("Something went wrong")

	if e.Code != ErrCodeInternal {
		t.Errorf("Code = %s, want INTERNAL_ERROR", e.Code)
	}
	if e.Message != "Something went wrong" {
		t.Errorf("Message = %s, want 'Something went wrong'", e.Message)
	}
}

func TestErrValidation(t *testing.T) {
	e := ErrValidation("Invalid input")

	if e.Code != ErrCodeValidation {
		t.Errorf("Code = %s, want VALIDATION_ERROR", e.Code)
	}
	if e.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want %d", e.StatusCode, http.StatusBadRequest)
	}
}

func TestErrDatabase(t *testing.T) {
	cause := errors.New("connection refused")
	e := ErrDatabase(cause)

	if e.Code != ErrCodeDatabaseError {
		t.Errorf("Code = %s, want DATABASE_ERROR", e.Code)
	}
	if e.cause != cause {
		t.Error("cause not preserved")
	}
}
