package xcore

import (
	"context"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
)

type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	RealIPKey    contextKey = "real_ip"
	UserIDKey    contextKey = "user_id"
)

type Context struct {
	Request    *http.Request
	Params     map[string]string
	Query      url.Values
	logger     *Logger
	App        *App
	Router     *Router
	Response   http.ResponseWriter
	StatusCode int
	handlers   []HandlerFunc
	index      int
	errors     []error
}

type HandlerFunc func(c *Context) error

func NewContext(w http.ResponseWriter, r *http.Request) *Context {
	return &Context{
		Request:    r,
		Params:     mux.Vars(r),
		Query:      r.URL.Query(),
		Response:   w,
		StatusCode: http.StatusOK,
		index:      -1,
	}
}

func (c *Context) Reset(w http.ResponseWriter, r *http.Request) {
	c.Request = r
	c.Params = mux.Vars(r)
	c.Query = r.URL.Query()
	c.Response = w
	c.StatusCode = http.StatusOK
	c.index = -1
	c.errors = nil
}

func (c *Context) Handler() HandlerFunc {
	if c.index < len(c.handlers) {
		return c.handlers[c.index]
	}
	return nil
}

func (c *Context) Next() error {
	c.index++
	if c.index < len(c.handlers) {
		return c.handlers[c.index](c)
	}
	return nil
}

func (c *Context) Abort() {
	c.index = len(c.handlers)
}

func (c *Context) AbortWithStatus(code int) {
	c.StatusCode = code
	c.Abort()
}

func (c *Context) AbortWithError(code int, err error) error {
	c.StatusCode = code
	c.errors = append(c.errors, err)
	c.Abort()
	return err
}

func (c *Context) SetLogger(logger *Logger) {
	c.logger = logger
}

func (c *Context) Logger() *Logger {
	return c.logger
}

func (c *Context) Loggerf(format string, args ...interface{}) {
	if c.logger != nil {
		c.logger.Info().Msgf(format, args...)
	}
}

func (c *Context) Error() error {
	if len(c.errors) > 0 {
		return c.errors[0]
	}
	return nil
}

func (c *Context) Errors() []error {
	return c.errors
}

func (c *Context) AddError(err error) {
	c.errors = append(c.errors, err)
}

var _ context.Context = (*Context)(nil)
