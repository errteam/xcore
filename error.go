// Package xcore provides error handling functionality for the xcore framework.
//
// This package defines the XError type for structured error handling across the framework.
// It includes error codes, HTTP status mapping, validation errors, and error handlers.
//
// Key features:
//   - Custom error codes with HTTP status mapping
//   - Wrapping of underlying errors
//   - Validation error support
//   - Metadata attachment
//   - Middleware for automatic error handling
package xcore

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrorCode represents a category of errors.
// These codes are used for programmatic error handling and categorization.
type ErrorCode string

const (
	ErrCodeInternal           ErrorCode = "INTERNAL_ERROR"
	ErrCodeValidation         ErrorCode = "VALIDATION_ERROR"
	ErrCodeNotFound           ErrorCode = "NOT_FOUND"
	ErrCodeUnauthorized       ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden          ErrorCode = "FORBIDDEN"
	ErrCodeBadRequest         ErrorCode = "BAD_REQUEST"
	ErrCodeConflict           ErrorCode = "CONFLICT"
	ErrCodeTooManyRequests    ErrorCode = "TOO_MANY_REQUESTS"
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeInvalidToken       ErrorCode = "INVALID_TOKEN"
	ErrCodeTokenExpired       ErrorCode = "TOKEN_EXPIRED"
	ErrCodeRateLimitExceeded  ErrorCode = "RATE_LIMIT_EXCEEDED"
	ErrCodeDatabaseError      ErrorCode = "DATABASE_ERROR"
	ErrCodeCacheError         ErrorCode = "CACHE_ERROR"
	ErrCodeExternalAPI        ErrorCode = "EXTERNAL_API_ERROR"
	ErrCodeTimeout            ErrorCode = "TIMEOUT"
	ErrCodeCanceled           ErrorCode = "CANCELED"
	ErrCodeAlreadyExists      ErrorCode = "ALREADY_EXISTS"
	ErrCodeInvalidInput       ErrorCode = "INVALID_INPUT"
	ErrCodeGatewayTimeout     ErrorCode = "GATEWAY_TIMEOUT"
	ErrCodeMethodNotAllowed   ErrorCode = "METHOD_NOT_ALLOWED"
)

// XError is the custom error type used throughout the xcore framework.
// It provides structured error information including code, message, HTTP status, and optional details.
type XError struct {
	Code       ErrorCode              `json:"code"`
	Message    string                 `json:"message"`
	StatusCode int                    `json:"-"`
	Errors     []ValidationError      `json:"errors,omitempty"`
	Meta       map[string]interface{} `json:"meta,omitempty"`
	cause      error
}

// Error implements the error interface.
// Returns a formatted string with the error code and message.
// If there is a cause, it includes the underlying error.
func (e *XError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s - %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause of the error.
// Implements the standard errors.Unwrap interface.
func (e *XError) Unwrap() error {
	return e.cause
}

// Is reports whether this error matches target.
// It matches by comparing error codes, or if target is a string, by checking if
// the error message contains that string.
func (e *XError) Is(target error) bool {
	if xerr, ok := target.(*XError); ok {
		return e.Code == xerr.Code
	}
	return strings.Contains(e.Error(), target.Error())
}

// WithMeta adds metadata key-value pairs to the error.
// This is useful for attaching additional context information.
func (e *XError) WithMeta(key string, value interface{}) *XError {
	if e.Meta == nil {
		e.Meta = make(map[string]interface{})
	}
	e.Meta[key] = value
	return e
}

// WithErrors attaches validation errors to the error.
// Used for field-level validation error reporting.
func (e *XError) WithErrors(errs []ValidationError) *XError {
	e.Errors = errs
	return e
}

// NewError creates a new XError with the given code and message.
// The HTTP status code is automatically determined from the error code.
func NewError(code ErrorCode, message string) *XError {
	return &XError{
		Code:       code,
		Message:    message,
		StatusCode: mapErrorCodeToHTTP(code),
	}
}

// NewErrorWithStatus creates a new XError with a custom HTTP status code.
// Use this when you need to override the default status code mapping.
func NewErrorWithStatus(code ErrorCode, message string, statusCode int) *XError {
	return &XError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// WrapError wraps an existing error with a new code and message.
// The original error is preserved and can be unwrapped.
func WrapError(err error, code ErrorCode, message string) *XError {
	return &XError{
		Code:       code,
		Message:    message,
		StatusCode: mapErrorCodeToHTTP(code),
		cause:      err,
	}
}

// WrapErrorWithStatus wraps an existing error with a custom HTTP status code.
func WrapErrorWithStatus(err error, code ErrorCode, message string, statusCode int) *XError {
	return &XError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		cause:      err,
	}
}

// mapErrorCodeToHTTP maps an ErrorCode to its corresponding HTTP status code.
func mapErrorCodeToHTTP(code ErrorCode) int {
	switch code {
	case ErrCodeValidation, ErrCodeBadRequest, ErrCodeInvalidInput:
		return http.StatusBadRequest
	case ErrCodeUnauthorized, ErrCodeInvalidToken, ErrCodeTokenExpired:
		return http.StatusUnauthorized
	case ErrCodeForbidden:
		return http.StatusForbidden
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodeConflict, ErrCodeAlreadyExists:
		return http.StatusConflict
	case ErrCodeTooManyRequests, ErrCodeRateLimitExceeded:
		return http.StatusTooManyRequests
	case ErrCodeServiceUnavailable:
		return http.StatusServiceUnavailable
	case ErrCodeTimeout, ErrCodeGatewayTimeout:
		return http.StatusGatewayTimeout
	case ErrCodeCanceled:
		return http.StatusRequestTimeout
	case ErrCodeMethodNotAllowed:
		return http.StatusMethodNotAllowed
	default:
		return http.StatusInternalServerError
	}
}

// NewValidationError creates a ValidationError for a specific field.
// Used when reporting validation failures in request data.
func NewValidationError(field, message string) ValidationError {
	return ValidationError{
		Field:   field,
		Message: message,
	}
}

// Predefined errors for common cases.
var (
	ErrInvalidToken       = NewError(ErrCodeInvalidToken, "Invalid token")
	ErrTokenExpired       = NewError(ErrCodeTokenExpired, "Token expired")
	ErrServiceUnavailable = NewError(ErrCodeServiceUnavailable, "Service temporarily unavailable")
)

// ErrInternal creates an internal server error with the given message.
func ErrInternal(msg string) *XError {
	return NewError(ErrCodeInternal, msg)
}

// ErrValidation creates a validation error with the given message.
func ErrValidation(msg string) *XError {
	return NewError(ErrCodeValidation, msg)
}

// ErrDatabase wraps a database error with the appropriate code and message.
func ErrDatabase(err error) *XError {
	return WrapError(err, ErrCodeDatabaseError, "Database operation failed")
}

// ErrCache wraps a cache error with the appropriate code and message.
func ErrCache(err error) *XError {
	return WrapError(err, ErrCodeCacheError, "Cache operation failed")
}

// ErrExternalAPI wraps an external API error with the service name in the message.
func ErrExternalAPI(err error, service string) *XError {
	return WrapError(err, ErrCodeExternalAPI, fmt.Sprintf("External API (%s) failed", service))
}

// ErrTimeout creates a timeout error with the given message.
func ErrTimeout(msg string) *XError {
	return NewError(ErrCodeTimeout, msg)
}

// ErrCanceled creates a canceled operation error with the given message.
func ErrCanceled(msg string) *XError {
	return NewError(ErrCodeCanceled, msg)
}

// ErrAlreadyExists creates an "already exists" error with the given message.
func ErrAlreadyExists(msg string) *XError {
	return NewError(ErrCodeAlreadyExists, msg)
}

// ErrInvalidInput creates an invalid input error with the given message.
func ErrInvalidInput(msg string) *XError {
	return NewError(ErrCodeInvalidInput, msg)
}

// ErrGatewayTimeout creates a gateway timeout error with the given message.
func ErrGatewayTimeout(msg string) *XError {
	return NewError(ErrCodeGatewayTimeout, msg)
}

// ErrMethodNotAllowed creates a method not allowed error with the given message.
func ErrMethodNotAllowed(msg string) *XError {
	return NewError(ErrCodeMethodNotAllowed, msg)
}

// ErrNotFound creates a not found error with an optional message.
// If msg is empty, defaults to "Resource not found".
func ErrNotFound(msg string) *XError {
	if msg == "" {
		msg = "Resource not found"
	}
	return NewError(ErrCodeNotFound, msg)
}

// ErrUnauthorized creates an unauthorized error with an optional message.
// If msg is empty, defaults to "Unauthorized access".
func ErrUnauthorized(msg string) *XError {
	if msg == "" {
		msg = "Unauthorized access"
	}
	return NewError(ErrCodeUnauthorized, msg)
}

// ErrForbidden creates a forbidden error with an optional message.
// If msg is empty, defaults to "Access forbidden".
func ErrForbidden(msg string) *XError {
	if msg == "" {
		msg = "Access forbidden"
	}
	return NewError(ErrCodeForbidden, msg)
}

// ErrBadRequest creates a bad request error with an optional message.
// If msg is empty, defaults to "Bad request".
func ErrBadRequest(msg string) *XError {
	if msg == "" {
		msg = "Bad request"
	}
	return NewError(ErrCodeBadRequest, msg)
}

// ErrConflict creates a conflict error with an optional message.
// If msg is empty, defaults to "Resource conflict".
func ErrConflict(msg string) *XError {
	if msg == "" {
		msg = "Resource conflict"
	}
	return NewError(ErrCodeConflict, msg)
}

// ErrTooManyRequests creates a rate limit error with an optional message.
// If msg is empty, defaults to "Too many requests".
func ErrTooManyRequests(msg string) *XError {
	if msg == "" {
		msg = "Too many requests"
	}
	return NewError(ErrCodeTooManyRequests, msg)
}

// ErrorHandler handles errors and converts them to HTTP responses.
// It uses the logger to log errors and can handle both XError and standard errors.
type ErrorHandler struct {
	logger *Logger
}

// NewErrorHandler creates a new ErrorHandler with an optional logger.
func NewErrorHandler(logger *Logger) *ErrorHandler {
	return &ErrorHandler{logger: logger}
}

// Handle processes an error and returns the HTTP status code and response body.
// For XError, it extracts status code and message. For other errors, returns 500.
func (h *ErrorHandler) Handle(err error) (int, interface{}) {
	if xerr, ok := err.(*XError); ok {
		if h.logger != nil {
			h.logger.Error().
				Err(err).
				Str("code", string(xerr.Code)).
				Int("status", xerr.StatusCode).
				Msg("handled error")
		}

		status := xerr.StatusCode
		if status == 0 {
			status = mapErrorCodeToHTTP(xerr.Code)
		}

		resp := ErrorWithCode(status, xerr.Message)
		if len(xerr.Errors) > 0 {
			var responseErrors []ResponseError
			for _, e := range xerr.Errors {
				responseErrors = append(responseErrors, ResponseError{
					Field:   e.Field,
					Message: e.Message,
					Code:    string(xerr.Code),
				})
			}
			return status, NewResponse().
				WithStatus(StatusError).
				WithCode(status).
				WithMessage(xerr.Message).
				WithErrors(responseErrors)
		}
		return status, resp
	}

	if h.logger != nil {
		h.logger.Error().Err(err).Msg("unhandled error")
	}

	return http.StatusInternalServerError, Error("Internal server error")
}

// Middleware returns a HandlerFunc that wraps the next handler with error handling.
// Catches errors returned by the handler and processes them through HandleError.
func (h *ErrorHandler) Middleware(next HandlerFunc) HandlerFunc {
	return func(c *Context) error {
		err := next(c)
		if err != nil {
			return h.HandleError(c, err)
		}
		return nil
	}
}

// HandleError processes an error and sends an appropriate JSON response to the client.
// If the error is an XError, it extracts status code and message.
// Adds the request ID to the response if available.
func (h *ErrorHandler) HandleError(c *Context, err error) error {
	if xerr, ok := err.(*XError); ok {
		status := xerr.StatusCode
		if status == 0 {
			status = mapErrorCodeToHTTP(xerr.Code)
		}

		if len(xerr.Errors) > 0 {
			var responseErrors []ResponseError
			for _, e := range xerr.Errors {
				responseErrors = append(responseErrors, ResponseError{
					Field:   e.Field,
					Message: e.Message,
					Code:    string(xerr.Code),
				})
			}
			return c.JSON(status, NewResponse().
				WithStatus(StatusError).
				WithCode(status).
				WithMessage(xerr.Message).
				WithErrors(responseErrors).
				WithRequestID(c.RequestID()))
		}

		return c.JSON(status, ErrorWithCode(status, xerr.Message).
			WithRequestID(c.RequestID()))
	}

	if h.logger != nil {
		h.logger.Error().Err(err).Msg("unhandled error")
	}

	return c.JSON(http.StatusInternalServerError, Error("Internal server error"))
}

// IsXError checks if the error is an XError.
// Uses errors.As for proper type checking.
func IsXError(err error) bool {
	var xerr *XError
	return errors.As(err, &xerr)
}

// GetXError extracts an XError from the error chain.
// Returns nil if not found.
func GetXError(err error) *XError {
	var xerr *XError
	if errors.As(err, &xerr) {
		return xerr
	}
	return nil
}

// GetXErrorValidationErrors extracts validation errors from an XError.
// Returns nil if the error is not an XError or has no validation errors.
func GetXErrorValidationErrors(err error) []ValidationError {
	if xerr := GetXError(err); xerr != nil {
		return xerr.Errors
	}
	return nil
}

func AsValidationError(err error) ([]ValidationError, bool) {
	if xerr := GetXError(err); xerr != nil && xerr.Code == ErrCodeValidation {
		return xerr.Errors, true
	}
	return nil, false
}
