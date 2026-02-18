package xcore

import (
	"encoding/json"
	"net/http"
	"time"
)

// Metadata contains response metadata
type Metadata struct {
	RequestID  string          `json:"request_id"`
	Timestamp  string          `json:"timestamp"`
	Pagination *PaginationMeta `json:"pagination,omitempty"`
	Extra      map[string]any  `json:"extra,omitempty"`
}

// PaginationMeta contains pagination information
type PaginationMeta struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

// Response is the unified API response structure
type Response struct {
	Code     string      `json:"code"`
	Message  string      `json:"message"`
	Data     interface{} `json:"data,omitempty"`
	Errors   interface{} `json:"errors,omitempty"`
	Metadata Metadata    `json:"metadata"`
}

// ValidationErrorDetail represents a single validation error
type ValidationErrorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// ValidationErrorResponse is the response for validation errors
type ValidationErrorResponse struct {
	Fields map[string][]string     `json:"fields"`
	Errors []ValidationErrorDetail `json:"errors"`
}

// ResponseBuilder helps build responses with metadata
type ResponseBuilder struct {
	*RequestBuilder
	response http.ResponseWriter
}

// NewResponseBuilder creates a new response builder
func NewResponseBuilder(w http.ResponseWriter, r *http.Request) *ResponseBuilder {
	rb := NewRequestBuilder(r)
	return &ResponseBuilder{RequestBuilder: rb, response: w}
}

func (b *ResponseBuilder) Request() *http.Request {
	return b.request
}

func (b *ResponseBuilder) Response() http.ResponseWriter {
	return b.response
}

// getMetadata creates metadata from context
func (b *ResponseBuilder) getMetadata() Metadata {
	return Metadata{
		RequestID: GetRequestIDFromContext(b.request.Context()),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Extra:     make(map[string]any),
	}
}

// JSON writes a JSON response with the given status code
func (b *ResponseBuilder) JSON(statusCode int, response Response) {
	b.response.Header().Set("Content-Type", "application/json")
	b.response.WriteHeader(statusCode)

	if err := json.NewEncoder(b.response).Encode(response); err != nil {
		http.Error(b.response, `{"error":"failed to encode response"}`, http.StatusInternalServerError)
	}
}

// Success sends a success response
func (b *ResponseBuilder) Success(statusCode int, message string, data interface{}) {
	b.JSON(statusCode, Response{
		Code:     "SUCCESS",
		Message:  message,
		Data:     data,
		Metadata: b.getMetadata(),
	})
}

// SuccessWithCode sends a success response with a custom code
func (b *ResponseBuilder) SuccessWithCode(statusCode int, message string, data interface{}, code string) {
	b.JSON(statusCode, Response{
		Code:     code,
		Message:  message,
		Data:     data,
		Metadata: b.getMetadata(),
	})
}

// SuccessWithPagination sends a success response with pagination metadata
func (b *ResponseBuilder) SuccessWithPagination(statusCode int, message string, data interface{}, pagination PaginationMeta) {
	metadata := b.getMetadata()
	metadata.Pagination = &pagination
	b.JSON(statusCode, Response{
		Code:     "SUCCESS",
		Message:  message,
		Data:     data,
		Metadata: metadata,
	})
}

// ErrorWithCode sends an error response with a custom code
func (b *ResponseBuilder) ErrorWithCode(statusCode int, message string, errs interface{}, code string) {
	b.JSON(statusCode, Response{
		Code:     code,
		Message:  message,
		Errors:   errs,
		Metadata: b.getMetadata(),
	})
}

// ValidationError sends a validation error response
func (b *ResponseBuilder) ValidationError(err error) {
	responseErrors := ParseValidationErrors(err)

	b.JSON(http.StatusBadRequest, Response{
		Code:     "VALIDATION_ERROR",
		Message:  "Validation failed",
		Errors:   responseErrors,
		Metadata: b.getMetadata(),
	})
}

// NotFound sends a not found response
func (b *ResponseBuilder) NotFound(message string) {
	b.JSON(http.StatusNotFound, Response{
		Code:     "NOT_FOUND",
		Message:  message,
		Metadata: b.getMetadata(),
	})
}

// Unauthorized sends an unauthorized response
func (b *ResponseBuilder) Unauthorized(message string) {
	b.JSON(http.StatusUnauthorized, Response{
		Code:     "UNAUTHORIZED",
		Message:  message,
		Metadata: b.getMetadata(),
	})
}

// Forbidden sends a forbidden response
func (b *ResponseBuilder) Forbidden(message string) {
	b.JSON(http.StatusForbidden, Response{
		Code:     "FORBIDDEN",
		Message:  message,
		Metadata: b.getMetadata(),
	})
}

// InternalServerError sends an internal server error response
func (b *ResponseBuilder) InternalServerError(message string) {
	b.JSON(http.StatusInternalServerError, Response{
		Code:     "INTERNAL_ERROR",
		Message:  message,
		Metadata: b.getMetadata(),
	})
}

// Conflict sends a conflict response
func (b *ResponseBuilder) Conflict(message string) {
	b.JSON(http.StatusConflict, Response{
		Code:     "CONFLICT",
		Message:  message,
		Metadata: b.getMetadata(),
	})
}

// BadRequest sends a bad request response
func (b *ResponseBuilder) BadRequest(message string) {
	b.JSON(http.StatusBadRequest, Response{
		Code:     "BAD_REQUEST",
		Message:  message,
		Metadata: b.getMetadata(),
	})
}

// RateLimitExceeded sends a rate limit exceeded response
func (b *ResponseBuilder) RateLimitExceeded(message string) {
	b.JSON(http.StatusTooManyRequests, Response{
		Code:     "RATE_LIMIT_EXCEEDED",
		Message:  message,
		Metadata: b.getMetadata(),
	})
}

// MethodNotAllowed sends a method not allowed response
func (b *ResponseBuilder) MethodNotAllowed(message string) {
	b.JSON(http.StatusMethodNotAllowed, Response{
		Code:     "METHOD_NOT_ALLOWED",
		Message:  message,
		Metadata: b.getMetadata(),
	})
}

// Timeout sends a timeout response
func (b *ResponseBuilder) Timeout(message string) {
	b.JSON(http.StatusGatewayTimeout, Response{
		Code:     "TIMEOUT",
		Message:  message,
		Metadata: b.getMetadata(),
	})
}

// ServiceUnavailable sends a service unavailable response
func (b *ResponseBuilder) ServiceUnavailable(message string) {
	b.JSON(http.StatusServiceUnavailable, Response{
		Code:     "SERVICE_UNAVAILABLE",
		Message:  message,
		Metadata: b.getMetadata(),
	})
}

// NewPaginationMeta creates pagination metadata
func NewPaginationMeta(page, perPage int, total int64) PaginationMeta {
	totalPages := CalculateTotalPages(int(total), perPage)

	return PaginationMeta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
}

// StatusCodeToCode converts an HTTP status code to our standard code format
func StatusCodeToCode(statusCode int) string {
	switch statusCode {
	case http.StatusOK:
		return "SUCCESS"
	case http.StatusCreated:
		return "CREATED"
	case http.StatusAccepted:
		return "ACCEPTED"
	case http.StatusNoContent:
		return "NO_CONTENT"
	case http.StatusBadRequest:
		return "BAD_REQUEST"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusMethodNotAllowed:
		return "METHOD_NOT_ALLOWED"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusTooManyRequests:
		return "RATE_LIMIT_EXCEEDED"
	case http.StatusInternalServerError:
		return "INTERNAL_ERROR"
	case http.StatusNotImplemented:
		return "NOT_IMPLEMENTED"
	case http.StatusServiceUnavailable:
		return "SERVICE_UNAVAILABLE"
	case http.StatusGatewayTimeout:
		return "TIMEOUT"
	default:
		return "ERROR"
	}
}

// ErrorWithStatus sends an error response using HTTP status code to determine the code
func (b *ResponseBuilder) Error(statusCode int, message string, errs interface{}) {
	b.JSON(statusCode, Response{
		Code:     StatusCodeToCode(statusCode),
		Message:  message,
		Errors:   errs,
		Metadata: b.getMetadata(),
	})
}

// Status sends a response with code derived from HTTP status code
func (b *ResponseBuilder) Status(statusCode int, message string, data interface{}) {
	b.JSON(statusCode, Response{
		Code:     StatusCodeToCode(statusCode),
		Message:  message,
		Data:     data,
		Metadata: b.getMetadata(),
	})
}

// AppErrorHandler handles AppError from services and sends appropriate response
func (b *ResponseBuilder) AppErrorHandler(err error) {
	if appErr, ok := GetAppError(err); ok {
		// It's an AppError, use its details
		b.JSON(appErr.StatusCode, Response{
			Code:     string(appErr.Code),
			Message:  appErr.Message,
			Errors:   appErr.Details,
			Metadata: b.getMetadata(),
		})
		return
	}

	// Not an AppError, treat as internal error
	b.JSON(http.StatusInternalServerError, Response{
		Code:     "INTERNAL_ERROR",
		Message:  "An unexpected error occurred",
		Metadata: b.getMetadata(),
	})
}

// HandleError is a convenience method that handles both AppError and validation errors
func (b *ResponseBuilder) HandleError(err error) {
	if err == nil {
		return
	}

	// Check if it's an AppError first
	if ok := IsAppError(err); ok {
		b.AppErrorHandler(err)
		return
	}

	// Check if it's a validation error
	if validationErrs := ParseValidationErrors(err); validationErrs != nil && !validationErrs.IsEmpty() {
		b.JSON(http.StatusBadRequest, Response{
			Code:     "VALIDATION_ERROR",
			Message:  "Validation failed",
			Errors:   validationErrs,
			Metadata: b.getMetadata(),
		})
		return
	}

	// Default to internal error
	b.JSON(http.StatusInternalServerError, Response{
		Code:     "INTERNAL_ERROR",
		Message:  err.Error(),
		Metadata: b.getMetadata(),
	})
}

// Created sends a 201 Created response
func (b *ResponseBuilder) Created(message string, data interface{}) {
	b.Success(http.StatusCreated, message, data)
}

// NoContent sends a 204 No Content response
func (b *ResponseBuilder) NoContent() {
	b.response.WriteHeader(http.StatusNoContent)
}

// OK sends a 200 OK response with data
func (b *ResponseBuilder) OK(data interface{}) {
	b.Success(http.StatusOK, "OK", data)
}

// Deleted sends a 200 response indicating successful deletion
func (b *ResponseBuilder) Deleted(message string) {
	if message == "" {
		message = "Resource deleted successfully"
	}
	b.Success(http.StatusOK, message, nil)
}

// Updated sends a 200 response indicating successful update
func (b *ResponseBuilder) Updated(message string, data interface{}) {
	if message == "" {
		message = "Resource updated successfully"
	}
	b.Success(http.StatusOK, message, data)
}

// WithExtra adds extra metadata to the response
func (b *ResponseBuilder) WithExtra(key string, value any) *ResponseBuilder {
	metadata := b.getMetadata()
	if metadata.Extra == nil {
		metadata.Extra = make(map[string]any)
	}
	metadata.Extra[key] = value
	return b
}

// WithPagination adds pagination metadata to the response
func (b *ResponseBuilder) WithPagination(page, perPage int, total int64) *ResponseBuilder {
	metadata := b.getMetadata()
	metadata.Pagination = &PaginationMeta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: CalculateTotalPages(int(total), perPage),
		HasNext:    page < CalculateTotalPages(int(total), perPage),
		HasPrev:    page > 1,
	}
	return b
}

// SetHeader sets a response header
func (b *ResponseBuilder) SetHeader(key, value string) {
	b.response.Header().Set(key, value)
}

// SetCacheControl sets the Cache-Control header
func (b *ResponseBuilder) SetCacheControl(value string) {
	b.SetHeader("Cache-Control", value)
}

// SetETag sets the ETag header
func (b *ResponseBuilder) SetETag(value string) {
	b.SetHeader("ETag", value)
}
