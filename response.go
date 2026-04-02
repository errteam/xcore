// Package xcore provides response helpers for standardized HTTP responses.
//
// This package defines the Response type and helper functions for creating
// consistent API responses across the application.
package xcore

import (
	"encoding/json"
	"net/http"
	"time"
)

// ResponseStatus represents the status of a response.
type ResponseStatus string

// Standard response statuses.
const (
	StatusSuccess ResponseStatus = "success"
	StatusError   ResponseStatus = "error"
	StatusFail    ResponseStatus = "fail"
)

// ResponseMeta contains metadata about the response including pagination and timestamps.
type ResponseMeta struct {
	Page       int       `json:"page,omitempty"`
	PerPage    int       `json:"per_page,omitempty"`
	Total      int64     `json:"total,omitempty"`
	TotalPages int       `json:"total_pages,omitempty"`
	RequestID  string    `json:"request_id,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// ResponseError represents a field-level error in validation or processing.
type ResponseError struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// Response is the standard response structure for all API responses.
type Response struct {
	Status  ResponseStatus  `json:"status"`
	Code    int             `json:"code,omitempty"`
	Message string          `json:"message,omitempty"`
	Data    interface{}     `json:"data,omitempty"`
	Errors  []ResponseError `json:"errors,omitempty"`
	Meta    *ResponseMeta   `json:"meta,omitempty"`
}

// NewResponse creates a new Response with default values.
// Sets Status to success and initializes empty Errors slice.
func NewResponse() *Response {
	return &Response{
		Status: StatusSuccess,
		Meta:   &ResponseMeta{Timestamp: time.Now()},
		Errors: []ResponseError{},
	}
}

// WithStatus sets the response status (success, error, fail).
func (r *Response) WithStatus(status ResponseStatus) *Response {
	r.Status = status
	return r
}

// WithCode sets the HTTP status code for the response.
func (r *Response) WithCode(code int) *Response {
	r.Code = code
	return r
}

// WithMessage sets a message for the response.
func (r *Response) WithMessage(msg string) *Response {
	r.Message = msg
	return r
}

// WithData sets the data payload for the response.
func (r *Response) WithData(data interface{}) *Response {
	r.Data = data
	return r
}

// WithError adds a single error to the response and sets status to error.
func (r *Response) WithError(err ResponseError) *Response {
	r.Errors = append(r.Errors, err)
	r.Status = StatusError
	return r
}

// WithErrors adds multiple errors to the response and sets status to error.
func (r *Response) WithErrors(errs []ResponseError) *Response {
	r.Errors = append(r.Errors, errs...)
	r.Status = StatusError
	return r
}

// WithMeta sets the metadata for the response.
func (r *Response) WithMeta(meta *ResponseMeta) *Response {
	r.Meta = meta
	return r
}

// WithRequestID sets the request ID in the response metadata.
func (r *Response) WithRequestID(id string) *Response {
	if r.Meta == nil {
		r.Meta = &ResponseMeta{Timestamp: time.Now()}
	}
	r.Meta.RequestID = id
	return r
}

// WithPageMeta sets pagination metadata (page, per_page, total, total_pages).
func (r *Response) WithPageMeta(page, perPage int, total int64) *Response {
	if r.Meta == nil {
		r.Meta = &ResponseMeta{Timestamp: time.Now()}
	}
	r.Meta.Page = page
	r.Meta.PerPage = perPage
	r.Meta.Total = total
	if perPage > 0 {
		r.Meta.TotalPages = int((total + int64(perPage) - 1) / int64(perPage))
	}
	return r
}

// ToJSON converts the Response to JSON bytes.
func (r *Response) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// Write writes the response to the http.ResponseWriter.
// Sets Content-Type header and writes status code if set.
func (r *Response) Write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	if r.Code > 0 {
		w.WriteHeader(r.Code)
	}
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(r)
}

// Success creates a success response with the given data.
// Uses StatusOK (200).
func Success(data interface{}) *Response {
	return NewResponse().
		WithStatus(StatusSuccess).
		WithCode(http.StatusOK).
		WithData(data)
}

// SuccessMessage creates a success response with a message but no data.
func SuccessMessage(msg string) *Response {
	return NewResponse().
		WithStatus(StatusSuccess).
		WithCode(http.StatusOK).
		WithMessage(msg)
}

// Created creates a response for successful resource creation.
// Uses StatusCreated (201). Default message is "Resource created successfully".
func Created(data interface{}, msg string) *Response {
	if msg == "" {
		msg = "Resource created successfully"
	}
	return NewResponse().
		WithStatus(StatusSuccess).
		WithCode(http.StatusCreated).
		WithMessage(msg).
		WithData(data)
}

// Error creates a generic error response.
// Uses StatusInternalServerError (500).
func Error(msg string) *Response {
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusInternalServerError).
		WithMessage(msg)
}

// ErrorWithCode creates an error response with a custom status code.
func ErrorWithCode(code int, msg string) *Response {
	return NewResponse().
		WithStatus(StatusError).
		WithCode(code).
		WithMessage(msg)
}

// BadRequest creates a bad request error response.
// Uses StatusBadRequest (400).
func BadRequest(msg string) *Response {
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusBadRequest).
		WithMessage(msg)
}

// ValidationErrorResp creates a validation error response.
// Uses StatusUnprocessableEntity (422).
func ValidationErrorResp(errors []ResponseError) *Response {
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusUnprocessableEntity).
		WithMessage("Validation failed").
		WithErrors(errors)
}

// Unauthorized creates an unauthorized error response.
// Uses StatusUnauthorized (401). Default message is "Unauthorized access".
func Unauthorized(msg string) *Response {
	if msg == "" {
		msg = "Unauthorized access"
	}
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusUnauthorized).
		WithMessage(msg)
}

// Forbidden creates a forbidden error response.
// Uses StatusForbidden (403). Default message is "Access forbidden".
func Forbidden(msg string) *Response {
	if msg == "" {
		msg = "Access forbidden"
	}
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusForbidden).
		WithMessage(msg)
}

// NotFound creates a not found error response.
// Uses StatusNotFound (404). Default message is "Resource not found".
func NotFound(msg string) *Response {
	if msg == "" {
		msg = "Resource not found"
	}
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusNotFound).
		WithMessage(msg)
}

// Conflict creates a conflict error response.
// Uses StatusConflict (409). Default message is "Resource conflict".
func Conflict(msg string) *Response {
	if msg == "" {
		msg = "Resource conflict"
	}
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusConflict).
		WithMessage(msg)
}

// TooManyRequests creates a rate limit error response.
// Uses StatusTooManyRequests (429). Default message is "Too many requests".
func TooManyRequests(msg string) *Response {
	if msg == "" {
		msg = "Too many requests"
	}
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusTooManyRequests).
		WithMessage(msg)
}

// ServiceUnavailable creates a service unavailable error response.
// Uses StatusServiceUnavailable (503). Default message is "Service temporarily unavailable".
func ServiceUnavailable(msg string) *Response {
	if msg == "" {
		msg = "Service temporarily unavailable"
	}
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusServiceUnavailable).
		WithMessage(msg)
}

// RequestTimeout creates a request timeout error response.
// Uses StatusRequestTimeout (408). Default message is "Request timeout".
func RequestTimeout(msg string) *Response {
	if msg == "" {
		msg = "Request timeout"
	}
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusRequestTimeout).
		WithMessage(msg)
}

// GatewayTimeout creates a gateway timeout error response.
// Uses StatusGatewayTimeout (504). Default message is "Gateway timeout".
func GatewayTimeout(msg string) *Response {
	if msg == "" {
		msg = "Gateway timeout"
	}
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusGatewayTimeout).
		WithMessage(msg)
}

// MethodNotAllowed creates a method not allowed error response.
// Uses StatusMethodNotAllowed (405). Default message is "Method not allowed".
func MethodNotAllowed(msg string) *Response {
	if msg == "" {
		msg = "Method not allowed"
	}
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusMethodNotAllowed).
		WithMessage(msg)
}

// AlreadyExists creates an already exists error response.
// Uses StatusConflict (409). Default message is "Resource already exists".
func AlreadyExists(msg string) *Response {
	if msg == "" {
		msg = "Resource already exists"
	}
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusConflict).
		WithMessage(msg)
}

// Pagination holds pagination parameters for response.
type Pagination struct {
	Page    int   `json:"page"`
	PerPage int   `json:"per_page"`
	Total   int64 `json:"total"`
}

// Paginate creates a paginated response with the given data and pagination info.
func Paginate(data interface{}, page, perPage int, total int64) *Response {
	return NewResponse().
		WithStatus(StatusSuccess).
		WithCode(http.StatusOK).
		WithData(data).
		WithPageMeta(page, perPage, total)
}

// ErrorResponse is a simplified error response structure.
type ErrorResponse struct {
	Status  ResponseStatus  `json:"status"`
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Errors  []ResponseError `json:"errors,omitempty"`
	Meta    *ResponseMeta   `json:"meta,omitempty"`
}

// NewErrorResponse creates a new ErrorResponse with the given code and message.
func NewErrorResponse(code int, msg string) *ErrorResponse {
	return &ErrorResponse{
		Status:  StatusError,
		Code:    code,
		Message: msg,
		Meta:    &ResponseMeta{Timestamp: time.Now()},
	}
}

// WithErrors adds errors to the ErrorResponse.
func (e *ErrorResponse) WithErrors(errors []ResponseError) *ErrorResponse {
	e.Errors = errors
	return e
}

// Write writes the ErrorResponse to the http.ResponseWriter.
func (e *ErrorResponse) Write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	if e.Code > 0 {
		w.WriteHeader(e.Code)
	}
	return json.NewEncoder(w).Encode(e)
}

// StreamResponse represents a streaming response configuration.
type StreamResponse struct {
	ContentType string
	Headers     map[string]string
	Data        interface{}
}

func (s *StreamResponse) Write(w http.ResponseWriter) error {
	for k, v := range s.Headers {
		w.Header().Set(k, v)
	}
	w.Header().Set("Content-Type", s.ContentType)
	return json.NewEncoder(w).Encode(s.Data)
}
