package xcore

import (
	"encoding/json"
	"net/http"
	"time"
)

type ResponseStatus string

const (
	StatusSuccess ResponseStatus = "success"
	StatusError   ResponseStatus = "error"
	StatusFail    ResponseStatus = "fail"
)

type ResponseMeta struct {
	Page       int       `json:"page,omitempty"`
	PerPage    int       `json:"per_page,omitempty"`
	Total      int64     `json:"total,omitempty"`
	TotalPages int       `json:"total_pages,omitempty"`
	RequestID  string    `json:"request_id,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

type ResponseError struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

type Response struct {
	Status  ResponseStatus  `json:"status"`
	Code    int             `json:"code,omitempty"`
	Message string          `json:"message,omitempty"`
	Data    interface{}     `json:"data,omitempty"`
	Errors  []ResponseError `json:"errors,omitempty"`
	Meta    *ResponseMeta   `json:"meta,omitempty"`
}

func NewResponse() *Response {
	return &Response{
		Status: StatusSuccess,
		Meta:   &ResponseMeta{Timestamp: time.Now()},
		Errors: []ResponseError{},
	}
}

func (r *Response) WithStatus(status ResponseStatus) *Response {
	r.Status = status
	return r
}

func (r *Response) WithCode(code int) *Response {
	r.Code = code
	return r
}

func (r *Response) WithMessage(msg string) *Response {
	r.Message = msg
	return r
}

func (r *Response) WithData(data interface{}) *Response {
	r.Data = data
	return r
}

func (r *Response) WithError(err ResponseError) *Response {
	r.Errors = append(r.Errors, err)
	r.Status = StatusError
	return r
}

func (r *Response) WithErrors(errs []ResponseError) *Response {
	r.Errors = append(r.Errors, errs...)
	r.Status = StatusError
	return r
}

func (r *Response) WithMeta(meta *ResponseMeta) *Response {
	r.Meta = meta
	return r
}

func (r *Response) WithRequestID(id string) *Response {
	if r.Meta == nil {
		r.Meta = &ResponseMeta{Timestamp: time.Now()}
	}
	r.Meta.RequestID = id
	return r
}

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

func (r *Response) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

func (r *Response) Write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	if r.Code > 0 {
		w.WriteHeader(r.Code)
	}
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(r)
}

func Success(data interface{}) *Response {
	return NewResponse().
		WithStatus(StatusSuccess).
		WithCode(http.StatusOK).
		WithData(data)
}

func SuccessMessage(msg string) *Response {
	return NewResponse().
		WithStatus(StatusSuccess).
		WithCode(http.StatusOK).
		WithMessage(msg)
}

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

func Error(msg string) *Response {
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusInternalServerError).
		WithMessage(msg)
}

func ErrorWithCode(code int, msg string) *Response {
	return NewResponse().
		WithStatus(StatusError).
		WithCode(code).
		WithMessage(msg)
}

func BadRequest(msg string) *Response {
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusBadRequest).
		WithMessage(msg)
}

func ValidationErrorResp(errors []ResponseError) *Response {
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusUnprocessableEntity).
		WithMessage("Validation failed").
		WithErrors(errors)
}

func Unauthorized(msg string) *Response {
	if msg == "" {
		msg = "Unauthorized access"
	}
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusUnauthorized).
		WithMessage(msg)
}

func Forbidden(msg string) *Response {
	if msg == "" {
		msg = "Access forbidden"
	}
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusForbidden).
		WithMessage(msg)
}

func NotFound(msg string) *Response {
	if msg == "" {
		msg = "Resource not found"
	}
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusNotFound).
		WithMessage(msg)
}

func Conflict(msg string) *Response {
	if msg == "" {
		msg = "Resource conflict"
	}
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusConflict).
		WithMessage(msg)
}

func TooManyRequests(msg string) *Response {
	if msg == "" {
		msg = "Too many requests"
	}
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusTooManyRequests).
		WithMessage(msg)
}

func ServiceUnavailable(msg string) *Response {
	if msg == "" {
		msg = "Service temporarily unavailable"
	}
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusServiceUnavailable).
		WithMessage(msg)
}

func RequestTimeout(msg string) *Response {
	if msg == "" {
		msg = "Request timeout"
	}
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusRequestTimeout).
		WithMessage(msg)
}

func GatewayTimeout(msg string) *Response {
	if msg == "" {
		msg = "Gateway timeout"
	}
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusGatewayTimeout).
		WithMessage(msg)
}

func MethodNotAllowed(msg string) *Response {
	if msg == "" {
		msg = "Method not allowed"
	}
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusMethodNotAllowed).
		WithMessage(msg)
}

func AlreadyExists(msg string) *Response {
	if msg == "" {
		msg = "Resource already exists"
	}
	return NewResponse().
		WithStatus(StatusError).
		WithCode(http.StatusConflict).
		WithMessage(msg)
}

type Pagination struct {
	Page    int   `json:"page"`
	PerPage int   `json:"per_page"`
	Total   int64 `json:"total"`
}

func Paginate(data interface{}, page, perPage int, total int64) *Response {
	return NewResponse().
		WithStatus(StatusSuccess).
		WithCode(http.StatusOK).
		WithData(data).
		WithPageMeta(page, perPage, total)
}

type ErrorResponse struct {
	Status  ResponseStatus  `json:"status"`
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Errors  []ResponseError `json:"errors,omitempty"`
	Meta    *ResponseMeta   `json:"meta,omitempty"`
}

func NewErrorResponse(code int, msg string) *ErrorResponse {
	return &ErrorResponse{
		Status:  StatusError,
		Code:    code,
		Message: msg,
		Meta:    &ResponseMeta{Timestamp: time.Now()},
	}
}

func (e *ErrorResponse) WithErrors(errors []ResponseError) *ErrorResponse {
	e.Errors = errors
	return e
}

func (e *ErrorResponse) Write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	if e.Code > 0 {
		w.WriteHeader(e.Code)
	}
	return json.NewEncoder(w).Encode(e)
}

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
