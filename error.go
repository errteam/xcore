package xcore

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

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

type XError struct {
	Code       ErrorCode              `json:"code"`
	Message    string                 `json:"message"`
	StatusCode int                    `json:"-"`
	Errors     []ValidationError      `json:"errors,omitempty"`
	Meta       map[string]interface{} `json:"meta,omitempty"`
	cause      error
}

func (e *XError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s - %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *XError) Unwrap() error {
	return e.cause
}

func (e *XError) Is(target error) bool {
	if xerr, ok := target.(*XError); ok {
		return e.Code == xerr.Code
	}
	return strings.Contains(e.Error(), target.Error())
}

func (e *XError) WithMeta(key string, value interface{}) *XError {
	if e.Meta == nil {
		e.Meta = make(map[string]interface{})
	}
	e.Meta[key] = value
	return e
}

func (e *XError) WithErrors(errs []ValidationError) *XError {
	e.Errors = errs
	return e
}

func NewError(code ErrorCode, message string) *XError {
	return &XError{
		Code:       code,
		Message:    message,
		StatusCode: mapErrorCodeToHTTP(code),
	}
}

func NewErrorWithStatus(code ErrorCode, message string, statusCode int) *XError {
	return &XError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

func WrapError(err error, code ErrorCode, message string) *XError {
	return &XError{
		Code:       code,
		Message:    message,
		StatusCode: mapErrorCodeToHTTP(code),
		cause:      err,
	}
}

func WrapErrorWithStatus(err error, code ErrorCode, message string, statusCode int) *XError {
	return &XError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		cause:      err,
	}
}

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

func NewValidationError(field, message string) ValidationError {
	return ValidationError{
		Field:   field,
		Message: message,
	}
}

var (
	ErrInvalidToken       = NewError(ErrCodeInvalidToken, "Invalid token")
	ErrTokenExpired       = NewError(ErrCodeTokenExpired, "Token expired")
	ErrServiceUnavailable = NewError(ErrCodeServiceUnavailable, "Service temporarily unavailable")
)

func ErrInternal(msg string) *XError {
	return NewError(ErrCodeInternal, msg)
}

func ErrValidation(msg string) *XError {
	return NewError(ErrCodeValidation, msg)
}

func ErrDatabase(err error) *XError {
	return WrapError(err, ErrCodeDatabaseError, "Database operation failed")
}

func ErrCache(err error) *XError {
	return WrapError(err, ErrCodeCacheError, "Cache operation failed")
}

func ErrExternalAPI(err error, service string) *XError {
	return WrapError(err, ErrCodeExternalAPI, fmt.Sprintf("External API (%s) failed", service))
}

func ErrTimeout(msg string) *XError {
	return NewError(ErrCodeTimeout, msg)
}

func ErrCanceled(msg string) *XError {
	return NewError(ErrCodeCanceled, msg)
}

func ErrAlreadyExists(msg string) *XError {
	return NewError(ErrCodeAlreadyExists, msg)
}

func ErrInvalidInput(msg string) *XError {
	return NewError(ErrCodeInvalidInput, msg)
}

func ErrGatewayTimeout(msg string) *XError {
	return NewError(ErrCodeGatewayTimeout, msg)
}

func ErrMethodNotAllowed(msg string) *XError {
	return NewError(ErrCodeMethodNotAllowed, msg)
}

func ErrNotFound(msg string) *XError {
	if msg == "" {
		msg = "Resource not found"
	}
	return NewError(ErrCodeNotFound, msg)
}

func ErrUnauthorized(msg string) *XError {
	if msg == "" {
		msg = "Unauthorized access"
	}
	return NewError(ErrCodeUnauthorized, msg)
}

func ErrForbidden(msg string) *XError {
	if msg == "" {
		msg = "Access forbidden"
	}
	return NewError(ErrCodeForbidden, msg)
}

func ErrBadRequest(msg string) *XError {
	if msg == "" {
		msg = "Bad request"
	}
	return NewError(ErrCodeBadRequest, msg)
}

func ErrConflict(msg string) *XError {
	if msg == "" {
		msg = "Resource conflict"
	}
	return NewError(ErrCodeConflict, msg)
}

func ErrTooManyRequests(msg string) *XError {
	if msg == "" {
		msg = "Too many requests"
	}
	return NewError(ErrCodeTooManyRequests, msg)
}

type ErrorHandler struct {
	logger *Logger
}

func NewErrorHandler(logger *Logger) *ErrorHandler {
	return &ErrorHandler{logger: logger}
}

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

func (h *ErrorHandler) Middleware(next HandlerFunc) HandlerFunc {
	return func(c *Context) error {
		err := next(c)
		if err != nil {
			return h.HandleError(c, err)
		}
		return nil
	}
}

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

func IsXError(err error) bool {
	var xerr *XError
	return errors.As(err, &xerr)
}

func GetXError(err error) *XError {
	var xerr *XError
	if errors.As(err, &xerr) {
		return xerr
	}
	return nil
}

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
