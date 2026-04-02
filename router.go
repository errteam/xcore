// Package xcore provides HTTP routing functionality for the xcore framework.
//
// This package wraps the gorilla/mux router to provide a fluent interface
// for defining HTTP routes with support for:
//   - RESTful route handlers (GET, POST, PUT, PATCH, DELETE, OPTIONS)
//   - Context-based handlers with error handling
//   - Route grouping
//   - Static file serving
//   - Middleware composition
//   - CORS, rate limiting, and other common patterns
package xcore

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// Router wraps the gorilla/mux router and provides additional functionality.
// It manages HTTP server configuration, middleware registration, and route handling.
type Router struct {
	router       *mux.Router
	server       *http.Server
	config       *HTTPConfig
	logger       *Logger
	errorHandler *ErrorHandler
}

// NewRouter creates a new Router with optional configuration.
// If cfg is nil, default configuration is used (Port: 8080).
// The router is initialized with default timeouts: ReadTimeout 30s, WriteTimeout 30s, IdleTimeout 60s.
func NewRouter(cfg *HTTPConfig) *Router {
	if cfg == nil {
		cfg = &HTTPConfig{
			Port: 8080,
		}
	}

	r := &Router{
		router: mux.NewRouter(),
		config: cfg,
	}

	r.server = &http.Server{
		Addr:         getAddr(cfg),
		Handler:      r.router,
		ReadTimeout:  getDuration(cfg.ReadTimeout, 30),
		WriteTimeout: getDuration(cfg.WriteTimeout, 30),
		IdleTimeout:  getDuration(cfg.IdleTimeout, 60),
	}

	return r
}

// WithLogger attaches a logger to the router and initializes the error handler.
// The logger is used for request logging and error reporting.
func (r *Router) WithLogger(logger *Logger) *Router {
	r.logger = logger
	r.errorHandler = NewErrorHandler(logger)
	return r
}

// Use adds one or more gorilla/mux middleware functions to the router.
// Middlewares are executed in the order they are added.
func (r *Router) Use(middleware ...mux.MiddlewareFunc) {
	r.router.Use(middleware...)
}

// UseHandler adds a standard http.Handler to the middleware chain.
func (r *Router) UseHandler(h http.Handler) {
	r.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			h.ServeHTTP(w, req)
		})
	})
}

// UseMiddleware adds a middleware function (func(http.Handler) http.Handler) to the router.
func (r *Router) UseMiddleware(mw func(http.Handler) http.Handler) {
	r.router.Use(mw)
}

// UseRequestLogger adds request logging middleware to the router.
// Requires the router to have a logger attached via WithLogger().
func (r *Router) UseRequestLogger() {
	if r.logger == nil {
		return
	}
	r.router.Use(NewRequestLogger(r.logger).Middleware)
}

// UseRecovery adds panic recovery middleware to the router.
// Catches panics and returns a 500 Internal Server Error response.
// Requires the router to have a logger attached via WithLogger().
func (r *Router) UseRecovery() {
	if r.logger == nil {
		return
	}
	r.router.Use(NewRecovery(r.logger).Middleware)
}

// UseRequestID adds request ID middleware to the router.
// Adds X-Request-ID header to requests.
func (r *Router) UseRequestID() {
	r.router.Use(NewRequestID().Middleware)
}

// UseCompression adds gzip compression middleware to the router.
// The level parameter specifies compression level (gzip.DefaultCompression, etc.).
func (r *Router) UseCompression(level int) {
	r.router.Use(NewCompression(level).Middleware)
}

// UseBodyParser adds body parsing middleware to the router.
// The maxSize parameter specifies the maximum body size in bytes.
func (r *Router) UseBodyParser(maxSize int64) {
	r.router.Use(NewBodyParser(maxSize).Middleware)
}

// UseTimeout adds request timeout middleware to the router.
// The timeout parameter specifies the maximum duration for request processing.
func (r *Router) UseTimeout(timeout time.Duration) {
	r.router.Use(NewTimeout(timeout).Middleware)
}

// UseRateLimiter adds global rate limiting to the router.
// The rps parameter is requests per second, burst is the maximum burst size.
func (r *Router) UseRateLimiter(rps, burst int) {
	r.router.Use(NewRateLimiter(rps, burst).Middleware)
}

// UseRateLimiterPerIP adds per-IP rate limiting to the router.
// Each IP has its own token bucket with the specified rps and burst values.
func (r *Router) UseRateLimiterPerIP(rps, burst int) {
	r.router.Use(NewRateLimiter(rps, burst).EnablePerIP().Middleware)
}

// UseRealIP adds real IP extraction middleware to the router.
// Extracts client IP from X-Real-IP or X-Forwarded-For headers.
func (r *Router) UseRealIP() {
	r.router.Use(NewRealIP().Middleware)
}

// UseCORS adds CORS middleware to the router with the given configuration.
func (r *Router) UseCORS(cfg *CORSConfig) {
	r.router.Use(NewCORSMiddleware(cfg).Handler)
}

// UseMethodOverride adds HTTP method override middleware to the router.
// Allows POST requests to override method via X-HTTP-Method-Override header.
func (r *Router) UseMethodOverride() {
	r.router.Use(NewMethodOverride().Middleware)
}

// Static serves static files from the specified directory at the given URL path.
// The path parameter is the URL prefix, dir is the filesystem directory.
func (r *Router) Static(path, dir string) {
	fs := http.Dir(dir)
	router := r.router.PathPrefix(path).Handler(http.StripPrefix(path, http.FileServer(fs)))
	router.Name("static:" + path)
}

// StaticFS serves static files from a custom FileSystem at the given URL path.
func (r *Router) StaticFS(path string, fs http.FileSystem) {
	r.router.PathPrefix(path).Handler(http.StripPrefix(path, http.FileServer(fs)))
}

// StaticWithOptions serves static files with custom options including index, fallback, and directory listing.
// Options: Index (default "index.html"), Fallback (SPA fallback), DirectoryListing (enable/disable).
func (r *Router) StaticWithOptions(path, dir string, opts StaticOptions) {
	if opts.Index == "" {
		opts.Index = "index.html"
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		filePath := req.URL.Path

		f, err := http.Dir(dir).Open(filePath)
		if err != nil {
			if opts.Fallback != "" {
				http.ServeFile(w, req, opts.Fallback)
				return
			}
			http.NotFound(w, req)
			return
		}
		defer f.Close()

		fi, err := f.Stat()
		if err != nil {
			http.NotFound(w, req)
			return
		}

		if fi.IsDir() {
			indexPath := filePath + "/" + opts.Index
			if _, err := http.Dir(dir).Open(indexPath); err == nil {
				http.ServeFile(w, req, indexPath)
				return
			}

			if opts.Fallback != "" && opts.DirectoryListing {
				http.ServeFile(w, req, opts.Fallback)
				return
			}

			if opts.DirectoryListing {
				http.FileServer(http.Dir(dir)).ServeHTTP(w, req)
				return
			}

			if opts.Fallback != "" {
				http.ServeFile(w, req, opts.Fallback)
				return
			}
			http.NotFound(w, req)
			return
		}

		http.ServeFile(w, req, filePath)
	})

	r.router.PathPrefix(path).Handler(http.StripPrefix(path, handler))
}

// StaticOptions defines options for static file serving with fallback and directory listing.
type StaticOptions struct {
	Index            string
	Fallback         string
	DirectoryListing bool
}

// Favicon serves a favicon file at /favicon.ico.
func (r *Router) Favicon(file string) {
	r.router.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, file)
	})
}

// getAddr constructs the server address from HTTP config.
// Defaults to "0.0.0.0:8080" if not specified.
func getAddr(cfg *HTTPConfig) string {
	host := cfg.Host
	if host == "" {
		host = "0.0.0.0"
	}
	port := cfg.Port
	if port == 0 {
		port = 8080
	}
	return host + ":" + strconv.Itoa(port)
}

// getDuration converts seconds to time.Duration with a default value.
func getDuration(seconds int, defaultVal int) time.Duration {
	if seconds <= 0 {
		return time.Duration(defaultVal) * time.Second
	}
	return time.Duration(seconds) * time.Second
}

// Router returns the underlying gorilla/mux Router instance.
// This allows direct access to mux router features if needed.
func (r *Router) Router() *mux.Router {
	return r.router
}

// Server returns the underlying HTTP server instance.
// Used for graceful shutdown and server configuration.
func (r *Router) Server() *http.Server {
	return r.server
}

// NotFoundHandler sets a custom handler for routes that don't match any registered route.
func (r *Router) NotFoundHandler(handler http.HandlerFunc) {
	r.router.NotFoundHandler = handler
}

// ServeHTTP implements the http.Handler interface.
// Delegates to the underlying gorilla/mux router.
func (r *Router) ServeHTTP(w http.ResponseWriter, rq *http.Request) {
	r.router.ServeHTTP(w, rq)
}

// Vars returns the route variables for the given request.
// Uses gorilla/mux's Vars function.
func (r *Router) Vars(rq *http.Request) map[string]string {
	return mux.Vars(rq)
}

// SetAddress sets the server address (host:port).
func (r *Router) SetAddress(addr string) {
	r.server.Addr = addr
}

// SetHandler sets the HTTP handler for the server.
func (r *Router) SetHandler(handler http.Handler) {
	r.server.Handler = handler
}

// SetReadTimeout sets the HTTP server's read timeout in seconds.
func (r *Router) SetReadTimeout(timeout int) {
	r.server.ReadTimeout = time.Duration(timeout) * time.Second
}

// SetWriteTimeout sets the HTTP server's write timeout in seconds.
func (r *Router) SetWriteTimeout(timeout int) {
	r.server.WriteTimeout = time.Duration(timeout) * time.Second
}

// SetIdleTimeout sets the HTTP server's idle timeout in seconds.
func (r *Router) SetIdleTimeout(timeout int) {
	r.server.IdleTimeout = time.Duration(timeout) * time.Second
}

// UseErrorHandler adds the error handler middleware to the router.
// Creates a Context for each request and attaches the logger.
func (r *Router) UseErrorHandler() {
	r.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			c := NewContext(w, req)
			c.logger = r.logger
			next.ServeHTTP(w, req)
		})
	})
}

// handleWithError wraps a HandlerFunc with error handling.
// Creates a Context and delegates to the error handler if the handler returns an error.
func (r *Router) handleWithError(w http.ResponseWriter, req *http.Request, handler HandlerFunc) {
	c := NewContext(w, req)
	c.logger = r.logger
	if err := handler(c); err != nil {
		if r.errorHandler != nil {
			_ = r.errorHandler.HandleError(c, err)
		} else {
			_ = NewErrorHandler(r.logger).HandleError(c, err)
		}
	}
}

// HandleContext registers a context-based handler for the given path.
// The handler receives a Context and can return an error that is handled by the error handler.
func (r *Router) HandleContext(path string, handler HandlerFunc) *mux.Route {
	return r.router.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		r.handleWithError(w, req, handler)
	})
}

// GetHandler registers a GET handler for the given path.
// The handler receives a Context and can return an error.
func (r *Router) GetHandler(path string, handler HandlerFunc) *mux.Route {
	return r.router.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		r.handleWithError(w, req, handler)
	}).Methods(http.MethodGet)
}

// PostHandler registers a POST handler for the given path.
// The handler receives a Context and can return an error.
func (r *Router) PostHandler(path string, handler HandlerFunc) *mux.Route {
	return r.router.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		r.handleWithError(w, req, handler)
	}).Methods(http.MethodPost)
}

// PutHandler registers a PUT handler for the given path.
// The handler receives a Context and can return an error.
func (r *Router) PutHandler(path string, handler HandlerFunc) *mux.Route {
	return r.router.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		r.handleWithError(w, req, handler)
	}).Methods(http.MethodPut)
}

// PatchHandler registers a PATCH handler for the given path.
// The handler receives a Context and can return an error.
func (r *Router) PatchHandler(path string, handler HandlerFunc) *mux.Route {
	return r.router.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		r.handleWithError(w, req, handler)
	}).Methods(http.MethodPatch)
}

// DeleteHandler registers a DELETE handler for the given path.
// The handler receives a Context and can return an error.
func (r *Router) DeleteHandler(path string, handler HandlerFunc) *mux.Route {
	return r.router.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		r.handleWithError(w, req, handler)
	}).Methods(http.MethodDelete)
}

// OptionsHandler registers an OPTIONS handler for the given path.
// The handler receives a Context and can return an error.
func (r *Router) OptionsHandler(path string, handler HandlerFunc) *mux.Route {
	return r.router.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		r.handleWithError(w, req, handler)
	}).Methods(http.MethodOptions)
}

// Group creates a route group with a common prefix.
// The fn callback receives a sub-router that inherits the parent's configuration.
// Returns the parent router for method chaining.
func (r *Router) Group(prefix string, fn func(*Router)) *Router {
	subRouter := r.router.PathPrefix(prefix).Subrouter()
	sub := &Router{
		router:       subRouter,
		config:       r.config,
		logger:       r.logger,
		errorHandler: r.errorHandler,
	}
	fn(sub)
	return r
}

// HandleFunc registers a standard http.HandlerFunc for the given path.
// This bypasses the Context-based error handling.
func (r *Router) HandleFunc(path string, handler func(http.ResponseWriter, *http.Request)) *mux.Route {
	return r.router.HandleFunc(path, handler)
}

func (r *Router) Name(name string) *mux.Route {
	return r.router.Name(name)
}
