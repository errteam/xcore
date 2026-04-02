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

func (c *Context) GetHeader(key string) string {
	return c.Request.Header.Get(key)
}

func (c *Context) ContentType() string {
	return c.Request.Header.Get("Content-Type")
}

func (c *Context) Cookie(name string) (*http.Cookie, error) {
	return c.Request.Cookie(name)
}

func (c *Context) Cookies() []*http.Cookie {
	return c.Request.Cookies()
}

func (c *Context) UserAgent() string {
	return c.Request.UserAgent()
}

func (c *Context) RemoteAddr() string {
	return c.Request.RemoteAddr
}

func (c *Context) Method() string {
	return c.Request.Method
}

func (c *Context) Host() string {
	return c.Request.Host
}

func (c *Context) URL() *url.URL {
	return c.Request.URL
}

func (c *Context) IsAjax() bool {
	return c.GetHeader("X-Requested-With") == "XMLHttpRequest"
}

func (c *Context) IsWebSocket() string {
	return c.GetHeader("Upgrade")
}

func (c *Context) IsSecure() bool {
	return c.Request.TLS != nil
}

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

func (c *Context) Param(key string) string {
	if v, ok := c.Params[key]; ok {
		return v
	}
	return ""
}

func (c *Context) DefaultParam(key, defaultValue string) string {
	if v := c.Param(key); v != "" {
		return v
	}
	return defaultValue
}

func (c *Context) QueryParam(key string) string {
	return c.Query.Get(key)
}

func (c *Context) DefaultQuery(key, defaultValue string) string {
	if v := c.Query.Get(key); v != "" {
		return v
	}
	return defaultValue
}

func (c *Context) GetQuery(key string) (string, bool) {
	v := c.Query.Get(key)
	return v, v != ""
}

func (c *Context) PostForm(key string) string {
	return c.Request.FormValue(key)
}

func (c *Context) DefaultPostForm(key, defaultValue string) string {
	if v := c.Request.FormValue(key); v != "" {
		return v
	}
	return defaultValue
}

func (c *Context) FormFile(key string) (multipart.File, *multipart.FileHeader, error) {
	return c.Request.FormFile(key)
}

func (c *Context) MultipartForm() (*multipart.Form, error) {
	err := c.Request.ParseMultipartForm(32 << 20)
	if err != nil {
		return nil, err
	}
	return c.Request.MultipartForm, nil
}

func (c *Context) GetBody() []byte {
	body, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	return body
}

func (c *Context) Body() ([]byte, error) {
	return io.ReadAll(c.Request.Body)
}

func (c *Context) BindBody(bindFunc func([]byte, interface{}) error, obj interface{}) error {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	return bindFunc(body, obj)
}

func (c *Context) GetInt(key string) (int, error) {
	return strconv.Atoi(c.QueryParam(key))
}

func (c *Context) GetInt64(key string) (int64, error) {
	return strconv.ParseInt(c.QueryParam(key), 10, 64)
}

func (c *Context) GetFloat64(key string) (float64, error) {
	return strconv.ParseFloat(c.QueryParam(key), 64)
}

func (c *Context) GetBool(key string) (bool, error) {
	return strconv.ParseBool(c.QueryParam(key))
}

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

func (c *Context) Set(key string, value interface{}) {
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), key, value))
}

func (c *Context) Get(key string) interface{} {
	return c.Request.Context().Value(key)
}

func (c *Context) Context() context.Context {
	return c.Request.Context()
}

func (c *Context) Deadline() (deadline time.Time, ok bool) {
	return c.Request.Context().Deadline()
}

func (c *Context) Done() <-chan struct{} {
	return c.Request.Context().Done()
}

func (c *Context) Err() error {
	return c.Request.Context().Err()
}

func (c *Context) Value(key interface{}) interface{} {
	return c.Request.Context().Value(key)
}

func (c *Context) RequestID() string {
	ctx := c.Request.Context()
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

func (c *Context) RealIP() string {
	ctx := c.Request.Context()
	if ip, ok := ctx.Value(RealIPKey).(string); ok {
		return ip
	}
	return ""
}

func (c *Context) UserID() string {
	ctx := c.Request.Context()
	if uid, ok := ctx.Value(UserIDKey).(string); ok {
		return uid
	}
	return ""
}
