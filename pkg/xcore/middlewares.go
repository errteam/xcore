package xcore

import (
	"context"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

// RequestIDMiddleware adds a unique request ID to each request for tracing
func RequestIDMiddleware() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if request already has an ID (from proxy, etc.)
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = GenerateRequestID()
			}

			// Add to context
			ctx := context.WithValue(r.Context(), ContextKeyRequestID, requestID)
			ctx = context.WithValue(ctx, ContextKeyStartTime, time.Now())

			// Add to response header
			w.Header().Set("X-Request-ID", requestID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetRequestIDFromContext retrieves the request ID from context
func GetRequestIDFromContext(ctx context.Context) string {
	if requestID, ok := ctx.Value(ContextKeyRequestID).(string); ok {
		return requestID
	}
	return ""
}

// GetRequestID retrieves the request ID from the request
func GetRequestID(r *http.Request) string {
	return GetRequestIDFromContext(r.Context())
}

// RecoveryMiddleware recovers from panics and returns a 500 error
func RecoveryMiddleware(logger *zerolog.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					requestID := GetRequestID(r)

					logger.Error().
						Str("request_id", requestID).
						Interface("panic", rvr).
						Str("stack", string(debug.Stack())).
						Msg("Panic recovered")

					rb := NewResponseBuilder(w, r)
					rb.InternalServerError("Internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// TimeoutMiddleware creates a timeout middleware that cancels the context after specified duration
func TimeoutMiddleware(timeout time.Duration) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			r = r.WithContext(ctx)

			done := make(chan struct{})
			go func() {
				next.ServeHTTP(w, r)
				close(done)
			}()

			select {
			case <-done:
				return
			case <-ctx.Done():
				rb := NewResponseBuilder(w, r)
				rb.Timeout("Request timeout")
				return
			}
		})
	}
}

// LoggingMiddleware logs request information
func LoggingMiddleware(logger *zerolog.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rb := NewRequestBuilder(r)
			start := time.Now()
			requestID := GetRequestID(r)

			// Wrap response writer to capture status
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rw, r)

			duration := time.Since(start)

			logger.Info().
				Str("request_id", requestID).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("query", r.URL.RawQuery).
				Int("status", rw.statusCode).
				Dur("duration", duration).
				Str("ip", rb.GetClientIP()).
				Msg("Request completed")
		})
	}
}

// securityMiddleware applies security headers to responses.
func SecurityMiddleware(cfg SecurityConfig) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.XFrameOptions != "" {
				w.Header().Set("X-Frame-Options", cfg.XFrameOptions)
			}
			if cfg.XContentTypeOptions != "" {
				w.Header().Set("X-Content-Type-Options", cfg.XContentTypeOptions)
			}
			if cfg.XXSSProtection != "" {
				w.Header().Set("X-XSS-Protection", cfg.XXSSProtection)
			}
			if cfg.StrictTransportSecurity != "" {
				w.Header().Set("Strict-Transport-Security", cfg.StrictTransportSecurity)
			}
			if cfg.ContentSecurityPolicy != "" {
				w.Header().Set("Content-Security-Policy", cfg.ContentSecurityPolicy)
			}
			if cfg.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", cfg.ReferrerPolicy)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// corsMiddleware applies CORS headers to responses.
func CORSMiddleware(cfg CORSConfig) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := false

			// Check if origin is allowed
			for _, o := range cfg.AllowOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}

			if allowed || len(cfg.AllowOrigins) == 0 {
				if len(cfg.AllowOrigins) == 1 && cfg.AllowOrigins[0] == "*" {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else if allowed {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}

				if len(cfg.AllowMethods) > 0 {
					w.Header().Set("Access-Control-Allow-Methods", joinHeader(cfg.AllowMethods))
				}
				if len(cfg.AllowHeaders) > 0 {
					w.Header().Set("Access-Control-Allow-Headers", joinHeader(cfg.AllowHeaders))
				}
				if len(cfg.ExposeHeaders) > 0 {
					w.Header().Set("Access-Control-Expose-Headers", joinHeader(cfg.ExposeHeaders))
				}
				if cfg.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				if cfg.MaxAge != "" {
					w.Header().Set("Access-Control-Max-Age", cfg.MaxAge)
				}
			}

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitMiddleware creates a rate limiting middleware
func RateLimitMiddleware(cfg RateLimitConfig) mux.MiddlewareFunc {
	if !cfg.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	// Create rate limiter
	rate := limiter.Rate{
		Period: time.Second,
		Limit:  int64(cfg.RequestsPerSecond),
	}

	store := memory.NewStore()
	instance := limiter.New(store, rate)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get client IP
			rb := NewRequestBuilder(r)
			clientIP := rb.GetClientIP()
			key := clientIP

			// Use prefix if configured
			if cfg.Prefix != "" {
				key = cfg.Prefix + ":" + clientIP
			}

			// Check rate limit
			limitContext, err := instance.Get(r.Context(), key)
			if err != nil {
				rb := NewResponseBuilder(w, r)
				rb.InternalServerError("Rate limit error")
				return
			}

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(limitContext.Limit, 10))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(limitContext.Remaining, 10))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(limitContext.Reset, 10))

			// If limit exceeded, return 429
			if limitContext.Reached {
				rb := NewResponseBuilder(w, r)
				rb.RateLimitExceeded("Too many requests")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
