// Package xcore provides logging functionality for the xcore framework.
//
// This package wraps the zerolog library to provide structured logging with
// support for multiple outputs (console, file, both), log levels, and formatting.
// It also includes request logging middleware for HTTP servers.
package xcore

import (
	"io"
	"net/http"
	"os"
	"time"

	"github.com/natefinch/lumberjack"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

// Logger provides a structured logging interface based on zerolog.
// It supports different log levels, output destinations, and formatting options.
type Logger struct {
	logger      zerolog.Logger
	output      string
	format      string
	level       string
	errorLogger *zerolog.Logger
}

// defaultLogger is the global default logger used when no specific logger is configured.
var defaultLogger *Logger

// NewLogger creates a new Logger with the given configuration.
// If cfg is nil, default configuration is used:
//   - Level: "info"
//   - Output: "console"
//   - Format: "console"
//
// Supported output modes: "console", "file", "both"
// Supported formats: "console" (human-readable), "json" (structured)
func NewLogger(cfg *LoggerConfig) (*Logger, error) {
	if cfg == nil {
		cfg = &LoggerConfig{
			Level:  "info",
			Output: "console",
			Format: "console",
		}
	}

	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.ErrorMarshalFunc = pkgerrors.MarshalStack

	var output io.Writer

	switch cfg.Output {
	case "file":
		if cfg.FilePath == "" {
			cfg.FilePath = "./logs/app.log"
		}
		output = &lumberjack.Logger{
			Filename:   cfg.FilePath,
			MaxSize:    cfg.MaxSize,
			MaxAge:     cfg.MaxAge,
			MaxBackups: cfg.MaxBackups,
			Compress:   cfg.Compress,
		}
	case "both":
		if cfg.FilePath == "" {
			cfg.FilePath = "./logs/app.log"
		}
		fileOutput := &lumberjack.Logger{
			Filename:   cfg.FilePath,
			MaxSize:    cfg.MaxSize,
			MaxAge:     cfg.MaxAge,
			MaxBackups: cfg.MaxBackups,
			Compress:   cfg.Compress,
		}
		output = zerolog.MultiLevelWriter(os.Stdout, fileOutput)
	default:
		output = os.Stdout
	}

	var writer io.Writer
	if cfg.Format == "json" {
		writer = output
	} else {
		writer = zerolog.ConsoleWriter{Out: output}
	}

	logger := zerolog.New(writer).With().Timestamp().Logger()

	if cfg.Caller {
		logger = logger.With().Caller().Logger()
	}

	lvl, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	logger = logger.Level(lvl)

	return &Logger{
		logger: logger,
		output: cfg.Output,
		format: cfg.Format,
		level:  cfg.Level,
	}, nil
}

// WithErrorFile adds a separate error log file with its own configuration.
// The error log file captures only error-level logs.
// Default values are used if any parameter is <= 0:
//   - path: "./logs/error.log"
//   - maxSize: 10 MB
//   - maxAge: 30 days
//   - maxBackups: 3
func (l *Logger) WithErrorFile(path string, maxSize, maxAge, maxBackups int, compress bool) error {
	if path == "" {
		path = "./logs/error.log"
	}
	if maxSize <= 0 {
		maxSize = 10
	}
	if maxAge <= 0 {
		maxAge = 30
	}
	if maxBackups <= 0 {
		maxBackups = 3
	}

	errorOutput := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    maxSize,
		MaxAge:     maxAge,
		MaxBackups: maxBackups,
		Compress:   compress,
	}

	var writer io.Writer
	if l.format == "json" {
		writer = errorOutput
	} else {
		writer = zerolog.ConsoleWriter{Out: errorOutput}
	}

	l.errorLogger = newLoggerWithOutput(writer, zerolog.ErrorLevel)
	return nil
}

// newLoggerWithOutput creates a zerolog logger with a specific output and level.
func newLoggerWithOutput(w io.Writer, level zerolog.Level) *zerolog.Logger {
	logger := zerolog.New(w).With().Timestamp().Logger()
	logger = logger.Level(level)
	return &logger
}

// Error returns an event for logging error-level messages.
// If an error logger is configured, logs to the error file.
func (l *Logger) Error() *zerolog.Event {
	if l.errorLogger != nil {
		return l.errorLogger.Error()
	}
	return l.logger.Error()
}

// Debug returns an event for logging debug-level messages.
func (l *Logger) Debug() *zerolog.Event {
	return l.logger.Debug()
}

// Info returns an event for logging info-level messages.
func (l *Logger) Info() *zerolog.Event {
	return l.logger.Info()
}

// Warn returns an event for logging warning-level messages.
func (l *Logger) Warn() *zerolog.Event {
	return l.logger.Warn()
}

// Fatal returns an event for logging fatal-level messages.
// After logging, it calls os.Exit(1).
func (l *Logger) Fatal() *zerolog.Event {
	return l.logger.Fatal()
}

// Panic returns an event for logging panic-level messages.
// After logging, it calls panic().
func (l *Logger) Panic() *zerolog.Event {
	return l.logger.Panic()
}

// Log returns a generic event for logging at any level.
func (l *Logger) Log() *zerolog.Event {
	return l.logger.Log()
}

// With returns a contextual logger with additional fields.
func (l *Logger) With() zerolog.Context {
	return l.logger.With()
}

// Output creates a new Logger with a different output writer.
func (l *Logger) Output(w io.Writer) *Logger {
	return &Logger{
		logger: l.logger.Output(w),
		output: l.output,
		format: l.format,
		level:  l.level,
	}
}

// Level creates a new Logger with a different log level.
func (l *Logger) Level(level string) *Logger {
	lvl, _ := zerolog.ParseLevel(level)
	return &Logger{
		logger: l.logger.Level(lvl),
		output: l.output,
		format: l.format,
		level:  level,
	}
}

// Hook adds a hook to the logger for custom log processing.
func (l *Logger) Hook(h zerolog.Hook) *Logger {
	l.logger.Hook(h)
	return l
}

// SetDefaultLogger sets the global default logger.
// Used when accessing DefaultLogger() without creating a logger.
func SetDefaultLogger(logger *Logger) {
	defaultLogger = logger
}

// DefaultLogger returns the global default logger.
// Creates one if not set.
func DefaultLogger() *Logger {
	if defaultLogger == nil {
		logger, _ := NewLogger(nil)
		defaultLogger = logger
	}
	return defaultLogger
}

// LogLevel returns the current log level as a string.
func (l *Logger) LogLevel() string {
	return l.level
}

// RequestLogger is a middleware that logs HTTP requests.
// It logs method, path, status code, duration, and client IP.
type RequestLogger struct {
	logger *Logger
}

// NewRequestLogger creates a new RequestLogger middleware.
func NewRequestLogger(logger *Logger) *RequestLogger {
	return &RequestLogger{logger: logger}
}

// Middleware returns an http.Handler that logs request details.
func (l *RequestLogger) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapper := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(wrapper, r)

		if l.logger != nil {
			l.logger.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", wrapper.status).
				Str("duration", time.Since(start).String()).
				Str("remote", r.RemoteAddr).
				Msg("request")
		}
	})
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader captures the status code before writing headers.
func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
