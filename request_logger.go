package xcore

import (
	"bytes"
	"io"
	"net/http"
	"time"
)

type RequestBodyLogger struct {
	logger   *Logger
	maxBytes int64
}

func NewRequestBodyLogger(logger *Logger) *RequestBodyLogger {
	return &RequestBodyLogger{
		logger:   logger,
		maxBytes: 1024 * 1024,
	}
}

func (l *RequestBodyLogger) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, l.maxBytes))
			if err != nil {
				if l.logger != nil {
					l.logger.Error().Err(err).Msg("failed to read request body")
				}
			} else {
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

				if l.logger != nil {
					bodyStr := string(bodyBytes)
					if len(bodyStr) > 0 {
						l.logger.Debug().
							Str("method", r.Method).
							Str("path", r.URL.Path).
							Str("body", bodyStr).
							Msg("request body")
					}
				}
			}
		}

		wrapper := &responseLogger{ResponseWriter: w, logger: l.logger, request: r}
		next.ServeHTTP(wrapper, r)
	})
}

type responseLogger struct {
	http.ResponseWriter
	logger   *Logger
	request  *http.Request
	status   int
	written  int64
	duration time.Duration
	start    time.Time
}

func (l *responseLogger) WriteHeader(status int) {
	l.status = status
	l.ResponseWriter.WriteHeader(status)
}

func (l *responseLogger) Write(b []byte) (int, error) {
	if l.status == 0 {
		l.status = http.StatusOK
	}
	n, err := l.ResponseWriter.Write(b)
	l.written += int64(n)
	return n, err
}

func (l *responseLogger) Log() {
	if l.logger != nil {
		l.logger.Info().
			Str("method", l.request.Method).
			Str("path", l.request.URL.Path).
			Int("status", l.status).
			Int64("bytes", l.written).
			Str("duration", l.duration.String()).
			Str("remote", l.request.RemoteAddr).
			Msg("request completed")
	}
}

type ResponseLogger struct {
	logger *Logger
}

func NewResponseLogger(logger *Logger) *ResponseLogger {
	return &ResponseLogger{logger: logger}
}

func (l *ResponseLogger) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapper := &responseLogger{
			ResponseWriter: w,
			logger:         l.logger,
			request:        r,
			start:          start,
		}

		next.ServeHTTP(wrapper, r)

		duration := time.Since(start)
		if l.logger != nil {
			l.logger.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", wrapper.status).
				Int64("bytes", wrapper.written).
				Str("duration", duration.String()).
				Str("remote", r.RemoteAddr).
				Msg("request completed")
		}
	})
}

type StructuredLogger struct {
	logger        *Logger
	includeBody   bool
	includeHeader bool
	maxBodyBytes  int64
}

func NewStructuredLogger(logger *Logger) *StructuredLogger {
	return &StructuredLogger{
		logger:        logger,
		includeBody:   false,
		includeHeader: false,
		maxBodyBytes:  4096,
	}
}

func (l *StructuredLogger) WithBody(include bool) *StructuredLogger {
	l.includeBody = include
	return l
}

func (l *StructuredLogger) WithHeader(include bool) *StructuredLogger {
	l.includeHeader = include
	return l
}

func (l *StructuredLogger) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		var requestBody string
		if l.includeBody && r.Body != nil {
			bodyBytes, _ := io.ReadAll(io.LimitReader(r.Body, l.maxBodyBytes))
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			requestBody = string(bodyBytes)
		}

		wrapper := &responseLogger{
			ResponseWriter: w,
			logger:         l.logger,
			request:        r,
			start:          start,
		}

		next.ServeHTTP(wrapper, r)

		duration := time.Since(start)

		if l.logger != nil {
			event := l.logger.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("query", r.URL.RawQuery).
				Int("status", wrapper.status).
				Int64("request_bytes", int64(len(requestBody))).
				Int64("response_bytes", wrapper.written).
				Str("duration_ms", duration.String()).
				Str("remote_addr", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Str("request_id", r.Header.Get("X-Request-ID"))

			if l.includeBody && requestBody != "" {
				event.Str("request_body", requestBody)
			}

			if l.includeHeader {
				for k, v := range r.Header {
					event.Str("header_"+k, v[0])
				}
			}

			event.Msg("request")
		}
	})
}
