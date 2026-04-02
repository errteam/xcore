// Package xcore provides HTTP middleware components for the xcore framework.
//
// This package includes various middleware implementations for common HTTP functionality:
//
//   - Recovery: Panics are recovered and logged, returning a 503 response
//   - RequestID: Adds a unique request ID to each request (X-Request-ID header)
//   - Compression: Gzip compression for responses
//   - BodyParser: Limits and parses request body size
//   - Timeout: Adds request timeout with automatic response
//   - RealIP: Extracts the real client IP from proxies
//   - MethodOverride: Allows POST requests to override HTTP method
//
// Each middleware is implemented as a struct with a Middleware() method that
// returns an http.HandlerFunc for use with net/http or gorilla/mux.
package xcore

import (
	"compress/gzip"
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Recovery is a middleware that recovers from panics in the handler chain.
// When a panic occurs, it logs the error and returns a 503 Service Unavailable response.
// The logger is optional and can be nil.
type Recovery struct {
	logger *Logger
}

// NewRecovery creates a new Recovery middleware with an optional logger.
// If logger is nil, panic information will not be logged.
func NewRecovery(logger *Logger) *Recovery {
	return &Recovery{logger: logger}
}

// Middleware returns an http.Handler that recovers from panics in the next handler.
// If a panic occurs, it logs the panic details and returns a 503 response.
func (r *Recovery) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				if r.logger != nil {
					r.logger.Error().
						Interface("panic", err).
						Str("method", req.Method).
						Str("path", req.URL.Path).
						Msg("panic recovered")
				}

				resp := ServiceUnavailable("Service temporarily unavailable")
				resp.Write(w)
			}
		}()
		next.ServeHTTP(w, req)
	})
}

// RequestID is a middleware that adds a unique request ID to each request.
// If the X-Request-ID header is already present, it uses that value.
// Otherwise, it generates a new UUID.
// The request ID is set in the response header and in the request context.
type RequestID struct{}

// NewRequestID creates a new RequestID middleware.
func NewRequestID() *RequestID {
	return &RequestID{}
}

// Middleware returns an http.Handler that adds a request ID to the request.
// The ID is stored in the context (key: RequestIDKey) and response header.
func (r *RequestID) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestID := req.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		ctx := context.WithValue(req.Context(), RequestIDKey, requestID)
		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

// Compression is a middleware that compresses HTTP responses using gzip.
// The compression level is set during initialization (Default, Best, BestSpeed).
// It checks the Accept-Encoding header and only compresses if gzip is supported.
type Compression struct {
	level int
}

// NewCompression creates a new Compression middleware with the specified level.
// Valid levels are gzip.DefaultCompression (6), gzip.BestSpeed (1), gzip.BestCompression (9).
// If an invalid level is provided, it defaults to gzip.DefaultCompression.
func NewCompression(level int) *Compression {
	if level < gzip.DefaultCompression || level > gzip.BestCompression {
		level = gzip.DefaultCompression
	}
	return &Compression{level: level}
}

type gzipResponseWriter struct {
	http.ResponseWriter
	writer      *gzip.Writer
	wroteHeader bool
}

func (g *gzipResponseWriter) WriteHeader(status int) {
	g.wroteHeader = true
	g.ResponseWriter.WriteHeader(status)
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if !g.wroteHeader && g.ResponseWriter.Header().Get("Content-Encoding") == "" {
		g.ResponseWriter.Header().Set("Content-Encoding", "gzip")
	}
	return g.writer.Write(b)
}

// Middleware returns an http.Handler that compresses responses using gzip.
// It only compresses if the client accepts gzip encoding.
func (c *Compression) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gzipWriter := gzip.NewWriter(w)
		defer gzipWriter.Close()

		w.Header().Set("Content-Encoding", "gzip")
		next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, writer: gzipWriter}, r)
	})
}

// BodyParser is a middleware that validates and limits request body size.
// It checks the Content-Length header against the maxSize limit.
// For JSON requests, it also wraps the body reader with http.MaxBytesReader.
type BodyParser struct {
	maxSize int64
}

// NewBodyParser creates a new BodyParser middleware with the specified max size.
// If maxSize <= 0, defaults to 10MB (10 << 20 bytes).
func NewBodyParser(maxSize int64) *BodyParser {
	if maxSize <= 0 {
		maxSize = 10 << 20
	}
	return &BodyParser{maxSize: maxSize}
}

// Middleware returns an http.Handler that limits and validates request body size.
// Only applies to POST, PUT, and PATCH requests with JSON content type.
func (p *BodyParser) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
			if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
				if r.ContentLength > p.maxSize {
					resp := BadRequest("Request body too large")
					resp.Write(w)
					return
				}

				r.Body = http.MaxBytesReader(w, r.Body, p.maxSize)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Timeout is a middleware that adds a timeout to request processing.
// If the request takes longer than the timeout duration, it returns a 503 response.
// Uses context timeout and ensures the handler goroutine completes before returning.
type Timeout struct {
	timeout time.Duration
	handler http.Handler
	written bool
	mu      sync.Mutex
}

// NewTimeout creates a new Timeout middleware with the specified duration.
// If timeout <= 0, defaults to 30 seconds.
func NewTimeout(timeout time.Duration) *Timeout {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Timeout{timeout: timeout}
}

// Middleware returns an http.Handler that enforces a timeout on request processing.
// If the handler doesn't complete within the timeout, a 503 response is returned.
func (t *Timeout) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), t.timeout)
		defer cancel()

		tw := &timeoutWriter{
			ResponseWriter: w,
			timeout:        t.timeout,
			done:           make(chan struct{}),
		}

		done := make(chan struct{})
		go func() {
			next.ServeHTTP(tw, r.WithContext(ctx))
			close(tw.done)
			close(done)
		}()

		select {
		case <-tw.done:
			return
		case <-ctx.Done():
			tw.mu.Lock()
			written := tw.written
			tw.mu.Unlock()
			if !written {
				resp := ServiceUnavailable("Request timeout")
				resp.Write(w)
			}
			<-done
		}
	})
}

// timeoutWriter is a ResponseWriter wrapper that tracks whether a response
// has been written. It is used by the Timeout middleware to detect if a
// response was already sent before returning a timeout error.
type timeoutWriter struct {
	http.ResponseWriter
	timeout time.Duration
	done    chan struct{}
	written bool
	mu      sync.Mutex
}

func (w *timeoutWriter) WriteHeader(status int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.written {
		w.written = true
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w *timeoutWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.written {
		w.written = true
	}
	return w.ResponseWriter.Write(b)
}

// RealIP is a middleware that extracts the real client IP address from request headers.
// It checks X-Real-IP header first, then X-Forwarded-For, and finally falls back to RemoteAddr.
// The extracted IP is stored in the request context (key: RealIPKey).
type RealIP struct{}

// NewRealIP creates a new RealIP middleware.
func NewRealIP() *RealIP {
	return &RealIP{}
}

// Middleware returns an http.Handler that extracts the real client IP.
// The IP is stored in context with key RealIPKey.
func (r *RealIP) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		realIP := req.Header.Get("X-Real-IP")
		if realIP == "" {
			realIP = req.Header.Get("X-Forwarded-For")
			if idx := strings.Index(realIP, ","); idx != -1 {
				realIP = strings.TrimSpace(realIP[:idx])
			}
		}
		if realIP == "" {
			realIP, _, _ = net.SplitHostPort(req.RemoteAddr)
		}

		ctx := context.WithValue(req.Context(), RealIPKey, realIP)
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

// MethodOverride is a middleware that allows clients to override the HTTP method
// using the X-HTTP-Method-Override header. This is useful when clients can only
// send GET and POST requests but need to use PUT, PATCH, or DELETE.
type MethodOverride struct{}

// NewMethodOverride creates a new MethodOverride middleware.
func NewMethodOverride() *MethodOverride {
	return &MethodOverride{}
}

// Middleware returns an http.Handler that allows method override via header.
// Only applies to POST requests with X-HTTP-Method-Override header.
func (m *MethodOverride) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			method := r.Header.Get("X-HTTP-Method-Override")
			if method != "" {
				switch method {
				case http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete:
					r.Method = method
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
