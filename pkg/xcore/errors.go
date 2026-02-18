package xcore

import (
	"errors"
	"net/http"
)

// NOTE: This error handling code is duplicated in internal/errors/errors.go.
// The internal package currently uses its own copy instead of importing from here.
// Consider refactoring to use this package directly: import "your-module/pkg/xcore"
// and replace internal/errors usage with xcore.AppError, xcore.NewAppError, etc.

// ErrorCode represents a unique error code for programmatic handling
type ErrorCode string

// Application error codes
const (
	// Authentication errors
	ErrUnauthorized       ErrorCode = "UNAUTHORIZED"
	ErrInvalidCredentials ErrorCode = "INVALID_CREDENTIALS"
	ErrTokenExpired       ErrorCode = "TOKEN_EXPIRED"
	ErrTokenInvalid       ErrorCode = "TOKEN_INVALID"
	ErrTokenMissing       ErrorCode = "TOKEN_MISSING"
	ErrUserNotFound       ErrorCode = "USER_NOT_FOUND"

	// Validation errors
	ErrValidation    ErrorCode = "VALIDATION_ERROR"
	ErrInvalidInput  ErrorCode = "INVALID_INPUT"
	ErrMissingField  ErrorCode = "MISSING_FIELD"
	ErrInvalidFormat ErrorCode = "INVALID_FORMAT"

	// User errors
	ErrUserAlreadyExists ErrorCode = "USER_ALREADY_EXISTS"
	ErrEmailExists       ErrorCode = "EMAIL_EXISTS"
	ErrUsernameExists    ErrorCode = "USERNAME_EXISTS"

	// Wallet errors
	ErrWalletNotFound    ErrorCode = "WALLET_NOT_FOUND"
	ErrInsufficientFunds ErrorCode = "INSUFFICIENT_FUNDS"
	ErrInvalidAmount     ErrorCode = "INVALID_AMOUNT"

	// Transaction errors
	ErrTransactionNotFound ErrorCode = "TRANSACTION_NOT_FOUND"
	ErrTransactionFailed   ErrorCode = "TRANSACTION_FAILED"

	// Resource errors
	ErrResourceNotFound ErrorCode = "RESOURCE_NOT_FOUND"
	ErrResourceExists   ErrorCode = "RESOURCE_EXISTS"

	// Database errors
	ErrDatabase ErrorCode = "DATABASE_ERROR"

	// Rate limiting errors
	ErrRateLimitExceeded ErrorCode = "RATE_LIMIT_EXCEEDED"

	// Internal errors
	ErrInternal           ErrorCode = "INTERNAL_ERROR"
	ErrUnknown            ErrorCode = "UNKNOWN_ERROR"
	ErrNotImplemented     ErrorCode = "NOT_IMPLEMENTED"
	ErrServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
)

// AppError is the application error type with rich context
type AppError struct {
	Code       ErrorCode `json:"code"`
	Message    string    `json:"message"`
	Err        error     `json:"-"`
	StatusCode int       `json:"-"`
	Details    any       `json:"details,omitempty"`
	RequestID  string    `json:"request_id,omitempty"`
	Timestamp  string    `json:"timestamp,omitempty"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError creates a new AppError with code and message
func NewAppError(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: getStatusCode(code),
	}
}

// NewAppErrorWithDetails creates a new AppError with additional details
func NewAppErrorWithDetails(code ErrorCode, message string, details any) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: getStatusCode(code),
		Details:    details,
	}
}

// NewAppErrorWithErr creates a new AppError wrapping another error
func NewAppErrorWithErr(code ErrorCode, message string, err error) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		Err:        err,
		StatusCode: getStatusCode(code),
	}
}

// WithRequestID sets the request ID on the error
func (e *AppError) WithRequestID(requestID string) *AppError {
	e.RequestID = requestID
	return e
}

// WithTimestamp sets the timestamp on the error
func (e *AppError) WithTimestamp(timestamp string) *AppError {
	e.Timestamp = timestamp
	return e
}

// getStatusCode maps error codes to HTTP status codes
func getStatusCode(code ErrorCode) int {
	switch code {
	case ErrUnauthorized, ErrInvalidCredentials, ErrTokenExpired, ErrTokenInvalid, ErrTokenMissing:
		return http.StatusUnauthorized
	case ErrUserNotFound, ErrWalletNotFound, ErrResourceNotFound, ErrTransactionNotFound:
		return http.StatusNotFound
	case ErrValidation, ErrInvalidInput, ErrMissingField, ErrInvalidFormat, ErrInvalidAmount:
		return http.StatusBadRequest
	case ErrUserAlreadyExists, ErrEmailExists, ErrUsernameExists, ErrResourceExists:
		return http.StatusConflict
	case ErrInsufficientFunds:
		return http.StatusBadRequest
	case ErrRateLimitExceeded:
		return http.StatusTooManyRequests
	case ErrDatabase:
		return http.StatusInternalServerError
	case ErrInternal, ErrUnknown, ErrTransactionFailed:
		return http.StatusInternalServerError
	case ErrNotImplemented:
		return http.StatusNotImplemented
	case ErrServiceUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// Helper functions for common errors

func ErrUnauthorizedError(message string) *AppError {
	return NewAppError(ErrUnauthorized, message)
}

func ErrInvalidCredentialsError() *AppError {
	return NewAppError(ErrInvalidCredentials, "Invalid email or password")
}

func ErrUserNotFoundError() *AppError {
	return NewAppError(ErrUserNotFound, "User not found")
}

func ErrUserAlreadyExistsError(field string) *AppError {
	return NewAppError(ErrUserAlreadyExists, "User with this "+field+" already exists")
}

func ErrValidationError(message string) *AppError {
	return NewAppError(ErrValidation, message)
}

func ErrValidationErrors(details any) *AppError {
	return NewAppErrorWithDetails(ErrValidation, "Validation failed", details)
}

func ErrWalletNotFoundError() *AppError {
	return NewAppError(ErrWalletNotFound, "Wallet not found")
}

func ErrInsufficientFundsError() *AppError {
	return NewAppError(ErrInsufficientFunds, "Insufficient balance")
}

func ErrInvalidAmountError(message string) *AppError {
	return NewAppError(ErrInvalidAmount, message)
}

func ErrInternalError(message string) *AppError {
	return NewAppError(ErrInternal, message)
}

func ErrInternalErrorWithErr(message string, err error) *AppError {
	return NewAppErrorWithErr(ErrInternal, message, err)
}

func ErrDatabaseError(message string) *AppError {
	return NewAppError(ErrDatabase, message)
}

func ErrDatabaseErrorWithErr(message string, err error) *AppError {
	return NewAppErrorWithErr(ErrDatabase, message, err)
}

func ErrTokenExpiredError() *AppError {
	return NewAppError(ErrTokenExpired, "Token has expired")
}

func ErrTokenInvalidError() *AppError {
	return NewAppError(ErrTokenInvalid, "Invalid token")
}

func ErrTokenMissingError() *AppError {
	return NewAppError(ErrTokenMissing, "Authorization token is required")
}

func ErrResourceNotFoundError(resource string) *AppError {
	return NewAppError(ErrResourceNotFound, resource+" not found")
}

func ErrRateLimitExceededError() *AppError {
	return NewAppError(ErrRateLimitExceeded, "Too many requests. Please try again later.")
}

func ErrNotImplementedError(message string) *AppError {
	return NewAppError(ErrNotImplemented, message)
}

func ErrServiceUnavailableError(message string) *AppError {
	return NewAppError(ErrServiceUnavailable, message)
}

// IsAppError checks if an error is an AppError
func IsAppError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr)
}

// GetAppError extracts AppError from error
func GetAppError(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

// GetStatusCode extracts the HTTP status code from an error
func GetStatusCode(err error) int {
	if appErr, ok := GetAppError(err); ok {
		return appErr.StatusCode
	}
	return http.StatusInternalServerError
}

// GetErrorCode extracts the error code from an error
func GetErrorCode(err error) string {
	if appErr, ok := GetAppError(err); ok {
		return string(appErr.Code)
	}
	return "UNKNOWN_ERROR"
}
