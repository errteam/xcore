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

type Logger struct {
	logger      zerolog.Logger
	output      string
	format      string
	level       string
	errorLogger *zerolog.Logger
}

var defaultLogger *Logger

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

func newLoggerWithOutput(w io.Writer, level zerolog.Level) *zerolog.Logger {
	logger := zerolog.New(w).With().Timestamp().Logger()
	logger = logger.Level(level)
	return &logger
}

func (l *Logger) Error() *zerolog.Event {
	if l.errorLogger != nil {
		return l.errorLogger.Error()
	}
	return l.logger.Error()
}

func (l *Logger) Debug() *zerolog.Event {
	return l.logger.Debug()
}

func (l *Logger) Info() *zerolog.Event {
	return l.logger.Info()
}

func (l *Logger) Warn() *zerolog.Event {
	return l.logger.Warn()
}

func (l *Logger) Fatal() *zerolog.Event {
	return l.logger.Fatal()
}

func (l *Logger) Panic() *zerolog.Event {
	return l.logger.Panic()
}

func (l *Logger) Log() *zerolog.Event {
	return l.logger.Log()
}

func (l *Logger) With() zerolog.Context {
	return l.logger.With()
}

func (l *Logger) Output(w io.Writer) *Logger {
	return &Logger{
		logger: l.logger.Output(w),
		output: l.output,
		format: l.format,
		level:  l.level,
	}
}

func (l *Logger) Level(level string) *Logger {
	lvl, _ := zerolog.ParseLevel(level)
	return &Logger{
		logger: l.logger.Level(lvl),
		output: l.output,
		format: l.format,
		level:  level,
	}
}

func (l *Logger) Hook(h zerolog.Hook) *Logger {
	l.logger.Hook(h)
	return l
}

func SetDefaultLogger(logger *Logger) {
	defaultLogger = logger
}

func DefaultLogger() *Logger {
	if defaultLogger == nil {
		logger, _ := NewLogger(nil)
		defaultLogger = logger
	}
	return defaultLogger
}

func (l *Logger) LogLevel() string {
	return l.level
}

type RequestLogger struct {
	logger *Logger
}

func NewRequestLogger(logger *Logger) *RequestLogger {
	return &RequestLogger{logger: logger}
}

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

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
