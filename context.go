// Package xcore provides the Context type for HTTP request handling in the xcore framework.
//
// The Context type wraps the standard http.Request and http.ResponseWriter to provide
// a more convenient interface for handlers. It includes utilities for:
//
//   - Reading path parameters and query strings
//   - Managing the handler chain (Next, Abort)
//   - Error handling and logging
//   - Context value storage
package xcore

import (
	"context"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
)

// contextKey is the type used for context keys in the xcore framework.
type contextKey string

// Context keys for storing request-scoped values.
const (
	RequestIDKey contextKey = "request_id" // Stores the unique request ID
	RealIPKey    contextKey = "real_ip"    // Stores the client real IP address
	UserIDKey    contextKey = "user_id"    // Stores the authenticated user ID
)

// Context wraps http.Request and http.ResponseWriter for handler use.
// It provides a clean interface for handling HTTP requests with support for
// middleware chaining and error propagation.
type Context struct {
	Request    *http.Request
	Params     map[string]string // Route parameters from gorilla/mux
	Query      url.Values        // Query parameters
	logger     *Logger
	App        *App
	Router     *Router
	Response   http.ResponseWriter
	StatusCode int
	handlers   []HandlerFunc // Middleware/handler chain
	index      int           // Current handler index
	errors     []error       // Collected errors
}

// HandlerFunc is the function signature for context-based handlers.
// It receives a Context and returns an error if something goes wrong.
type HandlerFunc func(c *Context) error

// NewContext creates a new Context from an HTTP request and response writer.
// Automatically extracts route parameters and query string values.
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

// Reset reinitializes the Context with a new request/response pair.
// Used for request reuse in HTTP handler pools.
func (c *Context) Reset(w http.ResponseWriter, r *http.Request) {
	c.Request = r
	c.Params = mux.Vars(r)
	c.Query = r.URL.Query()
	c.Response = w
	c.StatusCode = http.StatusOK
	c.index = -1
	c.errors = nil
}

// Handler returns the current handler in the chain.
// Returns nil if the index is past the end of the chain.
func (c *Context) Handler() HandlerFunc {
	if c.index < len(c.handlers) {
		return c.handlers[c.index]
	}
	return nil
}

// Next executes the next handler in the chain.
// Used by middleware to pass control to the next handler.
func (c *Context) Next() error {
	c.index++
	if c.index < len(c.handlers) {
		return c.handlers[c.index](c)
	}
	return nil
}

// Abort stops the handler chain execution.
// Subsequent handlers in the chain will not be executed.
func (c *Context) Abort() {
	c.index = len(c.handlers)
}

// AbortWithStatus sets the status code and stops the handler chain.
func (c *Context) AbortWithStatus(code int) {
	c.StatusCode = code
	c.Abort()
}

// AbortWithError sets the status code, adds an error, and stops the handler chain.
// Returns the error for convenience in handler return values.
func (c *Context) AbortWithError(code int, err error) error {
	c.StatusCode = code
	c.errors = append(c.errors, err)
	c.Abort()
	return err
}

// SetLogger sets the logger for the context.
func (c *Context) SetLogger(logger *Logger) {
	c.logger = logger
}

// Logger returns the context's logger instance.
func (c *Context) Logger() *Logger {
	return c.logger
}

// Loggerf logs a formatted message using the context's logger.
func (c *Context) Loggerf(format string, args ...interface{}) {
	if c.logger != nil {
		c.logger.Info().Msgf(format, args...)
	}
}

// Error returns the first error in the context's error list.
// Returns nil if no errors have been added.
func (c *Context) Error() error {
	if len(c.errors) > 0 {
		return c.errors[0]
	}
	return nil
}

// Errors returns all errors collected in the context.
func (c *Context) Errors() []error {
	return c.errors
}

// AddError adds an error to the context's error list.
// Errors can be retrieved later using Error() or Errors().
func (c *Context) AddError(err error) {
	c.errors = append(c.errors, err)
}

var _ context.Context = Context{}
