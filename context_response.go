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

type ResponseData struct {
	Code    int             `json:"code"`
	Status  string          `json:"status"`
	Message string          `json:"message,omitempty"`
	Data    interface{}     `json:"data,omitempty"`
	Errors  []ResponseError `json:"errors,omitempty"`
	Meta    *ResponseMeta   `json:"meta,omitempty"`
}

func (c *Context) Status(code int) {
	c.StatusCode = code
	c.Response.WriteHeader(code)
}

func (c *Context) Header(key, value string) {
	c.Response.Header().Set(key, value)
}

func (c *Context) GetHeaderReal(key string) string {
	return c.Response.Header().Get(key)
}

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

func (c *Context) String(code int, format string, args ...interface{}) error {
	c.Response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Response.WriteHeader(code)

	if len(args) > 0 {
		format = fmt.Sprintf(format, args...)
	}

	_, err := c.Response.Write([]byte(format))
	return err
}

func (c *Context) HTML(code int, html string) error {
	c.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Response.WriteHeader(code)

	_, err := c.Response.Write([]byte(html))
	return err
}

func (c *Context) XML(code int, obj interface{}) error {
	c.Response.Header().Set("Content-Type", "application/xml")
	c.Response.WriteHeader(code)

	encoder := json.NewEncoder(c.Response)
	return encoder.Encode(obj)
}

func (c *Context) NoContent(code int) error {
	c.Response.WriteHeader(code)
	return nil
}

func (c *Context) Redirect(code int, url string) error {
	c.Response.Header().Set("Location", url)
	c.Response.WriteHeader(code)
	return nil
}

func (c *Context) File(filepath string) error {
	return c.fileFromDisk(filepath, false)
}

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

func (c *Context) Data(code int, contentType string, data []byte) error {
	c.Response.Header().Set("Content-Type", contentType)
	c.Response.WriteHeader(code)
	_, err := c.Response.Write(data)
	return err
}

func (c *Context) Bytes(code int, contentType string, data []byte) error {
	return c.Data(code, contentType, data)
}

func (c *Context) JSONSuccess(data interface{}) error {
	return c.JSON(http.StatusOK, Success(data))
}

func (c *Context) JSONCreated(data interface{}, message string) error {
	if message == "" {
		message = "Created successfully"
	}
	return c.JSON(http.StatusCreated, Created(data, message))
}

func (c *Context) JSONError(code int, message string) error {
	return c.JSON(code, Error(message))
}

func (c *Context) JSONValidationError(errors []ResponseError) error {
	return c.JSON(http.StatusUnprocessableEntity, ValidationErrorResp(errors))
}

func (c *Context) JSONPaginated(data interface{}, page, perPage int, total int64) error {
	return c.JSON(http.StatusOK, Paginate(data, page, perPage, total))
}

func (c *Context) ServeFile(file string) {
	http.ServeFile(c.Response, c.Request, file)
}

func (c *Context) SetCookie(cookie *http.Cookie) {
	http.SetCookie(c.Response, cookie)
}

func (c *Context) DeleteCookie(name string, path string, domains ...string) {
	for _, domain := range domains {
		c.Response.Header().Add("Set-Cookie", fmt.Sprintf("%s=; path=%s; domain=%s; expires=Thu, 01 Jan 1970 00:00:00 GMT", name, path, domain))
	}
	c.Response.Header().Add("Set-Cookie", fmt.Sprintf("%s=; path=%s; expires=Thu, 01 Jan 1970 00:00:00 GMT", name, path))
}

func (c *Context) SetCacheControl(seconds int) {
	c.Response.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", seconds))
}

func (c *Context) SetContentType(contentType string) {
	c.Response.Header().Set("Content-Type", contentType)
}

func mimeTypeByExtension(ext string) string {
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return contentType
}

func (c *Context) SendFile(file string, name string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	ext := filepath.Ext(name)
	return c.Data(http.StatusOK, mimeTypeByExtension(ext), data)
}
