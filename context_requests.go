// Package xcore provides HTTP request handling methods for the Context.
//
// This file extends the Context type with methods for reading and parsing
// request data including headers, query parameters, form data, and body.
package xcore

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// GetHeader returns the value of the specified request header.
func (c *Context) GetHeader(key string) string {
	return c.Request.Header.Get(key)
}

// ContentType returns the Content-Type header of the request.
func (c *Context) ContentType() string {
	return c.Request.Header.Get("Content-Type")
}

// Cookie returns the named cookie from the request.
func (c *Context) Cookie(name string) (*http.Cookie, error) {
	return c.Request.Cookie(name)
}

// Cookies returns all cookies from the request.
func (c *Context) Cookies() []*http.Cookie {
	return c.Request.Cookies()
}

// UserAgent returns the User-Agent header of the request.
func (c *Context) UserAgent() string {
	return c.Request.UserAgent()
}

// RemoteAddr returns the network address of the client.
func (c *Context) RemoteAddr() string {
	return c.Request.RemoteAddr
}

// Method returns the HTTP method of the request (GET, POST, etc.).
func (c *Context) Method() string {
	return c.Request.Method
}

// Host returns the host from the request.
func (c *Context) Host() string {
	return c.Request.Host
}

// URL returns the parsed URL of the request.
func (c *Context) URL() *url.URL {
	return c.Request.URL
}

// IsAjax checks if the request is an AJAX request by checking X-Requested-With header.
func (c *Context) IsAjax() bool {
	return c.GetHeader("X-Requested-With") == "XMLHttpRequest"
}

// IsWebSocket returns the value of the Upgrade header (e.g., "websocket").
func (c *Context) IsWebSocket() string {
	return c.GetHeader("Upgrade")
}

// IsSecure checks if the request was made over HTTPS.
func (c *Context) IsSecure() bool {
	return c.Request.TLS != nil
}

// ClientIP returns the real IP address of the client.
// Checks X-Real-IP, X-Forwarded-For headers, then falls back to RemoteAddr.
func (c *Context) ClientIP() string {
	ip := c.GetHeader("X-Real-IP")
	if ip == "" {
		ip = c.GetHeader("X-Forwarded-For")
		if idx := strings.Index(ip, ","); idx != -1 {
			ip = strings.TrimSpace(ip[:idx])
		}
	}
	if ip == "" {
		ip, _, _ = net.SplitHostPort(c.Request.RemoteAddr)
	}
	return ip
}

// Param returns the value of a URL path parameter.
func (c *Context) Param(key string) string {
	if v, ok := c.Params[key]; ok {
		return v
	}
	return ""
}

// DefaultParam returns the value of a URL parameter or a default if not found.
func (c *Context) DefaultParam(key, defaultValue string) string {
	if v := c.Param(key); v != "" {
		return v
	}
	return defaultValue
}

// QueryParam returns the value of a query string parameter.
func (c *Context) QueryParam(key string) string {
	return c.Query.Get(key)
}

// DefaultQuery returns the value of a query parameter or a default if not found.
func (c *Context) DefaultQuery(key, defaultValue string) string {
	if v := c.Query.Get(key); v != "" {
		return v
	}
	return defaultValue
}

// GetQuery returns both the value and existence flag of a query parameter.
func (c *Context) GetQuery(key string) (string, bool) {
	v := c.Query.Get(key)
	return v, v != ""
}

// PostForm returns the value of a POST form field.
func (c *Context) PostForm(key string) string {
	return c.Request.FormValue(key)
}

// DefaultPostForm returns the value of a POST form field or a default if not found.
func (c *Context) DefaultPostForm(key, defaultValue string) string {
	if v := c.Request.FormValue(key); v != "" {
		return v
	}
	return defaultValue
}

// FormFile returns the uploaded file and its metadata from a multipart form.
func (c *Context) FormFile(key string) (multipart.File, *multipart.FileHeader, error) {
	return c.Request.FormFile(key)
}

// MultipartForm parses and returns the multipart form data.
// Uses 32MB as the max memory size for parsing.
func (c *Context) MultipartForm() (*multipart.Form, error) {
	err := c.Request.ParseMultipartForm(32 << 20)
	if err != nil {
		return nil, err
	}
	return c.Request.MultipartForm, nil
}

// GetBody reads the request body and returns it as bytes.
// The body is re-added to the request so it can be read again.
func (c *Context) GetBody() ([]byte, error) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	return body, nil
}

// Body reads and returns the entire request body.
func (c *Context) Body() ([]byte, error) {
	return io.ReadAll(c.Request.Body)
}

// BindBody allows custom binding of the request body to an object.
func (c *Context) BindBody(bindFunc func([]byte, interface{}) error, obj interface{}) error {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	return bindFunc(body, obj)
}

// GetInt parses and returns an integer from a query parameter.
func (c *Context) GetInt(key string) (int, error) {
	return strconv.Atoi(c.QueryParam(key))
}

// GetInt64 parses and returns an int64 from a query parameter.
func (c *Context) GetInt64(key string) (int64, error) {
	return strconv.ParseInt(c.QueryParam(key), 10, 64)
}

// GetFloat64 parses and returns a float64 from a query parameter.
func (c *Context) GetFloat64(key string) (float64, error) {
	return strconv.ParseFloat(c.QueryParam(key), 64)
}

// GetBool parses and returns a bool from a query parameter.
func (c *Context) GetBool(key string) (bool, error) {
	return strconv.ParseBool(c.QueryParam(key))
}

// BindJSON parses the request body as JSON and validates it.
// Returns an error if the body is not valid JSON or validation fails.
func (c *Context) BindJSON(v interface{}) error {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return NewError(ErrCodeBadRequest, "Failed to read request body")
	}

	if err := json.Unmarshal(body, v); err != nil {
		return NewError(ErrCodeBadRequest, "Invalid JSON format")
	}

	if err := Validate(v); err != nil {
		validationErrs := GetValidationErrors(err)
		return NewErrorWithStatus(
			ErrCodeValidation,
			"Validation failed",
			http.StatusUnprocessableEntity,
		).WithErrors(validationErrs)
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	return nil
}

// BindQuery parses query string parameters into the given struct and validates it.
func (c *Context) BindQuery(v interface{}) error {
	data := make(map[string]interface{})

	for k, vals := range c.Request.URL.Query() {
		if len(vals) > 0 {
			data[k] = vals[0]
		}
	}

	bytesData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(bytesData, v); err != nil {
		return err
	}

	if err := Validate(v); err != nil {
		validationErrs := GetValidationErrors(err)
		return NewErrorWithStatus(
			ErrCodeValidation,
			"Validation failed",
			http.StatusUnprocessableEntity,
		).WithErrors(validationErrs)
	}

	return nil
}

// BindForm parses form data (application/x-www-form-urlencoded or multipart/form-data)
// into the given struct and validates it.
func (c *Context) BindForm(v interface{}) error {
	contentType := c.Request.Header.Get("Content-Type")

	if strings.Contains(contentType, "multipart/form-data") {
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
			return NewError(ErrCodeBadRequest, "Failed to parse multipart form")
		}
	} else {
		if err := c.Request.ParseForm(); err != nil {
			return NewError(ErrCodeBadRequest, "Failed to parse form")
		}
	}

	data := make(map[string]interface{})

	for k, vals := range c.Request.Form {
		if len(vals) > 0 {
			data[k] = vals[0]
		}
	}

	bytesData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(bytesData, v); err != nil {
		return err
	}

	if err := Validate(v); err != nil {
		validationErrs := GetValidationErrors(err)
		return NewErrorWithStatus(
			ErrCodeValidation,
			"Validation failed",
			http.StatusUnprocessableEntity,
		).WithErrors(validationErrs)
	}

	return nil
}

// BindHeader parses HTTP headers into the given struct and validates it.
// Header keys are converted to lowercase for the struct mapping.
func (c *Context) BindHeader(v interface{}) error {
	data := make(map[string]string)

	for k, vals := range c.Request.Header {
		if len(vals) > 0 {
			data[strings.ToLower(k)] = vals[0]
		}
	}

	bytesData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(bytesData, v); err != nil {
		return err
	}

	if err := Validate(v); err != nil {
		validationErrs := GetValidationErrors(err)
		return NewErrorWithStatus(
			ErrCodeValidation,
			"Validation failed",
			http.StatusUnprocessableEntity,
		).WithErrors(validationErrs)
	}

	return nil
}

// BindURI parses URL path parameters into the given struct and validates it.
func (c *Context) BindURI(v interface{}) error {
	vars := c.Params

	bytesData, err := json.Marshal(vars)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(bytesData, v); err != nil {
		return err
	}

	if err := Validate(v); err != nil {
		validationErrs := GetValidationErrors(err)
		return NewErrorWithStatus(
			ErrCodeValidation,
			"Validation failed",
			http.StatusUnprocessableEntity,
		).WithErrors(validationErrs)
	}

	return nil
}

// Set stores a value in the request context.
// The key parameter is intentionally a string for user convenience.
// Users should avoid using keys that could collide with framework keys (RequestIDKey, RealIPKey, UserIDKey).
func (c *Context) Set(key string, value interface{}) {
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), key, value))
}

// Get retrieves a value from the request context.
func (c *Context) Get(key string) interface{} {
	return c.Request.Context().Value(key)
}

// Context returns the underlying request context.
func (c *Context) Context() context.Context {
	return c.Request.Context()
}

// Deadline returns the deadline for the request context.
func (c Context) Deadline() (deadline time.Time, ok bool) {
	return c.Request.Context().Deadline()
}

// Done returns a channel that is closed when the request context is cancelled.
func (c Context) Done() <-chan struct{} {
	return c.Request.Context().Done()
}

// Err returns an error if the request context is cancelled or times out.
func (c Context) Err() error {
	return c.Request.Context().Err()
}

// Value returns the value for the given key in the request context.
func (c Context) Value(key interface{}) interface{} {
	return c.Request.Context().Value(key)
}

// RequestID returns the request ID from the context.
// This is set by the RequestID middleware.
func (c *Context) RequestID() string {
	ctx := c.Request.Context()
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// RealIP returns the real client IP from the context.
// This is set by the RealIP middleware.
func (c *Context) RealIP() string {
	ctx := c.Request.Context()
	if ip, ok := ctx.Value(RealIPKey).(string); ok {
		return ip
	}
	return ""
}

// UserID returns the authenticated user ID from the context.
// This is typically set after JWT authentication.
func (c *Context) UserID() string {
	ctx := c.Request.Context()
	if uid, ok := ctx.Value(UserIDKey).(string); ok {
		return uid
	}
	return ""
}
