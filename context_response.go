// Package xcore provides HTTP response methods for the Context.
//
// This file extends the Context type with methods for sending various types
// of HTTP responses including JSON, HTML, XML, files, and streams.
package xcore

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
)

// ResponseData is a structured response data container.
// Deprecated: Use Response struct instead.
type ResponseData struct {
	Code    int             `json:"code"`
	Status  string          `json:"status"`
	Message string          `json:"message,omitempty"`
	Data    interface{}     `json:"data,omitempty"`
	Errors  []ResponseError `json:"errors,omitempty"`
	Meta    *ResponseMeta   `json:"meta,omitempty"`
}

// Status sets the HTTP response status code.
func (c *Context) Status(code int) {
	c.StatusCode = code
	c.Response.WriteHeader(code)
}

// Header sets a response header key-value pair.
func (c *Context) Header(key, value string) {
	c.Response.Header().Set(key, value)
}

// GetHeaderReal returns the value of a response header.
func (c *Context) GetHeaderReal(key string) string {
	return c.Response.Header().Get(key)
}

// JSON sends a JSON response with the given status code and object.
// Automatically sets Content-Type to application/json.
func (c *Context) JSON(code int, obj interface{}) error {
	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(code)

	if obj == nil {
		return nil
	}

	encoder := json.NewEncoder(c.Response)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(obj)
}

// JSONP sends a JSONP response with the given callback function and object.
// The response is wrapped in the callback function call.
func (c *Context) JSONP(callback string, obj interface{}) error {
	c.Response.Header().Set("Content-Type", "application/javascript")
	c.Response.WriteHeader(c.StatusCode)

	output := callback + "("
	if data, err := json.Marshal(obj); err == nil {
		output += string(data) + ");"
	}

	_, err := c.Response.Write([]byte(output))
	return err
}

// String sends a plain text response with the given status code and format.
// Supports printf-style formatting with args.
func (c *Context) String(code int, format string, args ...interface{}) error {
	c.Response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Response.WriteHeader(code)

	if len(args) > 0 {
		format = fmt.Sprintf(format, args...)
	}

	_, err := c.Response.Write([]byte(format))
	return err
}

// HTML sends an HTML response with the given status code.
func (c *Context) HTML(code int, html string) error {
	c.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Response.WriteHeader(code)

	_, err := c.Response.Write([]byte(html))
	return err
}

// XML sends an XML response with the given status code and object.
// Note: Uses JSON encoder for simplicity; for proper XML, use dedicated XML marshaling.
func (c *Context) XML(code int, obj interface{}) error {
	c.Response.Header().Set("Content-Type", "application/xml")
	c.Response.WriteHeader(code)

	encoder := json.NewEncoder(c.Response)
	return encoder.Encode(obj)
}

// NoContent sends a response with no body and the given status code.
// Common use: 204 No Content for successful DELETE operations.
func (c *Context) NoContent(code int) error {
	c.Response.WriteHeader(code)
	return nil
}

// Redirect redirects the client to the given URL with the specified status code.
// Common use: 301 Moved Permanently or 302 Found.
func (c *Context) Redirect(code int, url string) error {
	c.Response.Header().Set("Location", url)
	c.Response.WriteHeader(code)
	return nil
}

// File serves a file from disk at the given filepath.
// Sets appropriate Content-Type based on file extension.
func (c *Context) File(filepath string) error {
	return c.fileFromDisk(filepath, false)
}

// FileInline serves a file from disk with a custom filename in the Content-Disposition header.
// This forces the browser to download the file with the specified name.
func (c *Context) FileInline(filepath string, name string) error {
	c.Response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", name))
	return c.fileFromDisk(filepath, false)
}

func (c *Context) fileFromDisk(filepath string, download bool) error {
	f, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := os.Stat(filepath)
	if err != nil {
		return err
	}

	if download {
		c.Header("Content-Disposition", "attachment")
	}

	http.ServeContent(c.Response, c.Request, fi.Name(), fi.ModTime(), f)
	return nil
}

// Stream sends a streaming response with the given content type.
// The readFunc is called repeatedly to write data to the response.
// If the response writer supports http.Flusher, data is flushed after each write.
func (c *Context) Stream(code int, contentType string, readFunc func(w io.Writer, fl http.Flusher) bool) error {
	c.Response.Header().Set("Content-Type", contentType)
	c.Response.WriteHeader(code)

	fl, ok := c.Response.(http.Flusher)
	if !ok {
		for {
			if !readFunc(c.Response, nil) {
				break
			}
		}
		return nil
	}

	for {
		if !readFunc(c.Response, fl) {
			break
		}
		fl.Flush()
	}

	return nil
}

// Data sends a raw byte response with the given content type and status code.
func (c *Context) Data(code int, contentType string, data []byte) error {
	c.Response.Header().Set("Content-Type", contentType)
	c.Response.WriteHeader(code)
	_, err := c.Response.Write(data)
	return err
}

// Bytes is an alias for Data method.
func (c *Context) Bytes(code int, contentType string, data []byte) error {
	return c.Data(code, contentType, data)
}

// JSONSuccess sends a success response with the given data.
// Uses StatusOK (200) and wraps data in the standard response format.
func (c *Context) JSONSuccess(data interface{}) error {
	return c.JSON(http.StatusOK, Success(data))
}

// JSONCreated sends a created response with the given data and message.
// Uses StatusCreated (201). Default message is "Created successfully".
func (c *Context) JSONCreated(data interface{}, message string) error {
	if message == "" {
		message = "Created successfully"
	}
	return c.JSON(http.StatusCreated, Created(data, message))
}

// JSONError sends an error response with the given status code and message.
func (c *Context) JSONError(code int, message string) error {
	return c.JSON(code, Error(message))
}

// JSONValidationError sends a validation error response with the given errors.
// Uses StatusUnprocessableEntity (422).
func (c *Context) JSONValidationError(errors []ResponseError) error {
	return c.JSON(http.StatusUnprocessableEntity, ValidationErrorResp(errors))
}

// JSONPaginated sends a paginated response with the given data and pagination info.
// Includes page, per_page, total, and total_pages in the meta.
func (c *Context) JSONPaginated(data interface{}, page, perPage int, total int64) error {
	return c.JSON(http.StatusOK, Paginate(data, page, perPage, total))
}

// ServeFile serves the given file using http.ServeFile.
// Convenience method that delegates to the standard library.
func (c *Context) ServeFile(file string) {
	http.ServeFile(c.Response, c.Request, file)
}

// SetCookie sets a cookie on the response.
func (c *Context) SetCookie(cookie *http.Cookie) {
	http.SetCookie(c.Response, cookie)
}

// DeleteCookie deletes a cookie by setting its expiration to the epoch.
// Supports optional domain parameters for domain-scoped cookies.
func (c *Context) DeleteCookie(name string, path string, domains ...string) {
	for _, domain := range domains {
		c.Response.Header().Add("Set-Cookie", fmt.Sprintf("%s=; path=%s; domain=%s; expires=Thu, 01 Jan 1970 00:00:00 GMT", name, path, domain))
	}
	c.Response.Header().Add("Set-Cookie", fmt.Sprintf("%s=; path=%s; expires=Thu, 01 Jan 1970 00:00:00 GMT", name, path))
}

// SetCacheControl sets the Cache-Control header with the given max-age seconds.
func (c *Context) SetCacheControl(seconds int) {
	c.Response.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", seconds))
}

// SetContentType sets the Content-Type header to the given value.
func (c *Context) SetContentType(contentType string) {
	c.Response.Header().Set("Content-Type", contentType)
}

// mimeTypeByExtension returns the MIME type for a given file extension.
// Returns "application/octet-stream" for unknown extensions.
func mimeTypeByExtension(ext string) string {
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return contentType
}

// SendFile reads a file and sends it as a response.
// The filename is used to determine the Content-Type.
func (c *Context) SendFile(file string, name string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	ext := filepath.Ext(name)
	return c.Data(http.StatusOK, mimeTypeByExtension(ext), data)
}
