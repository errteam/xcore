package xcore

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

type ServerConfig struct {
	Port         int    `mapstructure:"port"`
	Mode         string `mapstructure:"mode"`
	ReadTimeout  string `mapstructure:"read_timeout"`
	WriteTimeout string `mapstructure:"write_timeout"`
	IdleTimeout  string `mapstructure:"idle_timeout"`

	CORS      CORSConfig      `mapstructure:"cors"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
	Security  SecurityConfig  `mapstructure:"security"`
}

type CORSConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	AllowOrigins     []string `mapstructure:"allow_origins"`
	AllowMethods     []string `mapstructure:"allow_methods"`
	AllowHeaders     []string `mapstructure:"allow_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           string   `mapstructure:"max_age"`
	ExposeHeaders    []string `mapstructure:"expose_headers"`
}

type RateLimitConfig struct {
	Enabled           bool   `mapstructure:"enabled"`
	RequestsPerSecond int    `mapstructure:"requests_per_second"`
	Burst             int    `mapstructure:"burst"`
	Prefix            string `mapstructure:"prefix"`
}

type SecurityConfig struct {
	Enabled                 bool   `mapstructure:"enabled"`
	XFrameOptions           string `mapstructure:"x_frame_options"`
	XContentTypeOptions     string `mapstructure:"x_content_type_options"`
	XXSSProtection          string `mapstructure:"x_xss_protection"`
	StrictTransportSecurity string `mapstructure:"strict_transport_security"`
	ContentSecurityPolicy   string `mapstructure:"content_security_policy"`
	ReferrerPolicy          string `mapstructure:"referrer_policy"`
}

type Router struct {
	Server *http.Server
	Router *mux.Router
}

// NewRouter creates and returns a configured router with the mux router.
// The returned Router struct contains both the configured mux.Router and http.Server
// for use with graceful shutdown.
func NewRouter(cfg *ServerConfig, logger *zerolog.Logger) *Router {
	r := mux.NewRouter()

	// Apply request ID middleware (should be first)
	r.Use(RequestIDMiddleware())

	// Apply recovery middleware (should be early to catch all panics)
	r.Use(RecoveryMiddleware(logger))

	// Apply logging middleware
	r.Use(LoggingMiddleware(logger))

	// Apply security middleware if enabled
	if cfg.Security.Enabled {
		r.Use(SecurityMiddleware(cfg.Security))
	}

	// Apply CORS middleware if enabled
	if cfg.CORS.Enabled {
		r.Use(CORSMiddleware(cfg.CORS))
	}

	// Apply rate limiting middleware if enabled
	if cfg.RateLimit.Enabled {
		r.Use(RateLimitMiddleware(cfg.RateLimit))
	}

	// Register default error handlers (must be after all middleware)
	registerDefaultErrorHandlers(r, logger)

	// Parse timeout durations
	readTimeout := parseDuration(cfg.ReadTimeout)
	writeTimeout := parseDuration(cfg.WriteTimeout)
	idleTimeout := parseDuration(cfg.IdleTimeout)

	// Create HTTP server
	addr := fmt.Sprintf(":%d", cfg.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	return &Router{
		Server: server,
		Router: r,
	}
}

func (rt *Router) Use(r mux.MiddlewareFunc) {
	rt.Router.Use(r)
}

// Start begins listening on the configured port.
func (rt *Router) Start() error {
	return rt.Server.ListenAndServe()
}

// Shutdown gracefully shuts down the server with the given context.
func (rt *Router) Shutdown(ctx context.Context) error {
	return rt.Server.Shutdown(ctx)
}

// registerDefaultErrorHandlers registers default error handlers for 404 and 405
func registerDefaultErrorHandlers(r *mux.Router, logger *zerolog.Logger) {
	// Handle 404 - Not Found
	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Debug().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("Route not found")

		rb := NewResponseBuilder(w, r)
		rb.NotFound("Route not found")
	})

	// Handle 405 - Method Not Allowed
	r.MethodNotAllowedHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Debug().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("Method not allowed")

		rb := NewResponseBuilder(w, r)
		rb.MethodNotAllowed("Method not allowed")
	})
}

type Handler interface {
	RegisterRoutes(*mux.Router)
}

type HandlerFunc func(*ResponseBuilder)

func RouteWrapper(f HandlerFunc) func(http.ResponseWriter,
	*http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		rb := NewResponseBuilder(w, r)
		f(rb)
	}
}
