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

type Recovery struct {
	logger *Logger
}

func NewRecovery(logger *Logger) *Recovery {
	return &Recovery{logger: logger}
}

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

type RequestID struct{}

func NewRequestID() *RequestID {
	return &RequestID{}
}

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

type Compression struct {
	level int
}

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

type BodyParser struct {
	maxSize int64
}

func NewBodyParser(maxSize int64) *BodyParser {
	if maxSize <= 0 {
		maxSize = 10 << 20
	}
	return &BodyParser{maxSize: maxSize}
}

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

type Timeout struct {
	timeout time.Duration
	handler http.Handler
	written bool
	mu      sync.Mutex
}

func NewTimeout(timeout time.Duration) *Timeout {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Timeout{timeout: timeout}
}

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
			if !tw.written {
				resp := ServiceUnavailable("Request timeout")
				resp.Write(w)
			}
			<-done
		}
	})
}

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

type RealIP struct{}

func NewRealIP() *RealIP {
	return &RealIP{}
}

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

type MethodOverride struct{}

func NewMethodOverride() *MethodOverride {
	return &MethodOverride{}
}

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
