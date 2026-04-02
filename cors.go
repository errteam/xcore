// Package xcore provides CORS (Cross-Origin Resource Sharing) middleware for the xcore framework.
//
// This package provides middleware for handling CORS headers in HTTP responses.
// It supports configurable allowed origins, methods, headers, and credentials.
//
// The middleware handles OPTIONS preflight requests automatically and returns
// the appropriate CORS headers based on the configuration.
package xcore

import (
	"net/http"
	"strconv"
	"strings"
)

// CORSMiddleware handles CORS headers for HTTP requests.
// It validates the Origin header against allowed origins and sets appropriate response headers.
type CORSMiddleware struct {
	config *CORSConfig
}

// NewCORSMiddleware creates a new CORS middleware with the given configuration.
// If config is nil, default configuration is used:
//   - AllowedOrigins: ["*"]
//   - AllowedMethods: [GET, POST, PUT, DELETE, OPTIONS]
//   - AllowedHeaders: [Content-Type, Authorization]
//   - AllowCredentials: false
//   - MaxAge: 0 (no caching)
//   - ExposedHeaders: []
//   - Enabled: true
//
// Note: When AllowCredentials is true, wildcards ("*") in AllowedOrigins are replaced
// with an empty string to comply with CORS specification.
func NewCORSMiddleware(cfg *CORSConfig) *CORSMiddleware {
	if cfg == nil {
		cfg = &CORSConfig{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Content-Type", "Authorization"},
			AllowCredentials: false,
			MaxAge:           0,
			ExposedHeaders:   []string{},
			Enabled:          true,
		}
	}

	if cfg.Enabled && cfg.AllowCredentials {
		for i, origin := range cfg.AllowedOrigins {
			if origin == "*" {
				cfg.AllowedOrigins[i] = ""
				break
			}
		}
	}

	return &CORSMiddleware{config: cfg}
}

// Handler returns an http.Handler that applies CORS rules to requests.
// If CORS is disabled (config.Enabled = false), passes through to next handler unchanged.
// For OPTIONS requests (preflight), returns 204 No Content with appropriate headers.
func (m *CORSMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		allowed := false

		if len(m.config.AllowedOrigins) > 0 {
			for _, o := range m.config.AllowedOrigins {
				if o == "*" {
					allowed = true
					break
				}
				if strings.EqualFold(o, origin) {
					allowed = true
					break
				}
			}
		}

		if r.Method == http.MethodOptions {
			if allowed {
				if origin != "" {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(m.config.AllowedMethods, ", "))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(m.config.AllowedHeaders, ", "))

				if m.config.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				if m.config.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", strconv.Itoa(m.config.MaxAge))
				}
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if allowed && origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)

			if m.config.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if len(m.config.ExposedHeaders) > 0 {
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(m.config.ExposedHeaders, ", "))
			}
		}

		next.ServeHTTP(w, r)
	})
}

// MiddlewareFunc returns the middleware as a function type.
// Convenience method for use with router.Use() or similar.
func (m *CORSMiddleware) MiddlewareFunc() func(http.Handler) http.Handler {
	return m.Handler
}

func CORSMiddlewareFunc(cfg *CORSConfig) func(http.Handler) http.Handler {
	return NewCORSMiddleware(cfg).Handler
}
