package xcore

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LoggerConfig holds the configuration for the logger
type LoggerConfig struct {
	Level   string        `mapstructure:"level"`
	Format  string        `mapstructure:"format"`
	Output  []string      `mapstructure:"output"`
	File    FileLogConfig `mapstructure:"file"`
	Console ConsoleConfig `mapstructure:"console"`
	Caller  CallerConfig  `mapstructure:"caller"`
}

// FileLogConfig holds file logging configuration
type FileLogConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Path       string `mapstructure:"path"`
	MaxSize    int    `mapstructure:"max_size"`    // Maximum size in MB
	MaxBackups int    `mapstructure:"max_backups"` // Maximum number of old log files to keep
	MaxAge     int    `mapstructure:"max_age"`     // Maximum age in days
	Compress   bool   `mapstructure:"compress"`    // Enable gzip compression
}

// ConsoleConfig holds console logging configuration
type ConsoleConfig struct {
	Color      bool   `mapstructure:"color"`
	TimeFormat string `mapstructure:"time_format"`
}

// CallerConfig holds caller information configuration
type CallerConfig struct {
	Enabled    bool `mapstructure:"enabled"`
	SkipFrames int  `mapstructure:"skip_frames"` // Number of stack frames to skip
}

// DefaultLoggerConfig returns a default logger configuration
func DefaultLoggerConfig() *LoggerConfig {
	return &LoggerConfig{
		Level:  "info",
		Format: "json",
		Output: []string{"stdout"},
		File: FileLogConfig{
			Enabled:    false,
			Path:       "logs/app.log",
			MaxSize:    100, // 100MB
			MaxBackups: 3,
			MaxAge:     28, // 28 days
			Compress:   true,
		},
		Console: ConsoleConfig{
			Color:      true,
			TimeFormat: time.RFC3339,
		},
		Caller: CallerConfig{
			Enabled:    false,
			SkipFrames: 2,
		},
	}
}

// InitializeLogger creates and configures a zerolog logger based on the provided configuration
func InitializeLogger(cfg *LoggerConfig) *zerolog.Logger {
	// Set log level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Set time format
	timeFormat := getTimeFormat(cfg.Console.TimeFormat)
	zerolog.TimeFieldFormat = timeFormat

	// Create writers
	var writers []io.Writer

	// Console output
	for _, output := range cfg.Output {
		if output == "stdout" {
			if cfg.Format == "console" {
				writers = append(writers, zerolog.ConsoleWriter{
					Out:        os.Stdout,
					TimeFormat: timeFormat,
					NoColor:    !cfg.Console.Color,
				})
			} else {
				writers = append(writers, os.Stdout)
			}
		} else if output == "file" && cfg.File.Enabled {
			writers = append(writers, getFileWriter(cfg.File))
		}
	}

	// If file is enabled but not in output list, still create file writer
	if cfg.File.Enabled && !contains(cfg.Output, "file") {
		writers = append(writers, getFileWriter(cfg.File))
	}

	// If no writers, default to stdout
	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}

	// If multiple writers, use multi writer
	var writer io.Writer
	if len(writers) == 1 {
		writer = writers[0]
	} else {
		writer = zerolog.MultiLevelWriter(writers...)
	}

	// Create logger with timestamp
	log := zerolog.New(writer).With().Timestamp().Logger()

	// Add caller if enabled
	if cfg.Caller.Enabled {
		// Configure caller to show the actual source code location
		zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
			// Extract just the file name and line number for cleaner output
			// You can customize this to show full path if needed
			short := file
			for i := len(file) - 1; i > 0; i-- {
				if file[i] == '/' && file[i-1] != '/' {
					short = file[i+1:]
					break
				}
			}
			return fmt.Sprintf("%s:%d", short, line)
		}
		log = log.With().CallerWithSkipFrameCount(cfg.Caller.SkipFrames).Logger()
	}

	return &log
}

// getFileWriter creates a file writer with rotation support using lumberjack
func getFileWriter(cfg FileLogConfig) io.Writer {
	// Ensure directory exists
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		// Fallback to stdout if can't create directory
		return os.Stdout
	}

	return &lumberjack.Logger{
		Filename:   cfg.Path,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}
}

// LoggerWithField adds a field to the logger
func LoggerWithField(log *zerolog.Logger, key string, value interface{}) *zerolog.Logger {
	logger := log.With().Interface(key, value).Logger()
	return &logger
}

// LoggerWithFields adds multiple fields to the logger
func LoggerWithFields(log *zerolog.Logger, fields map[string]interface{}) *zerolog.Logger {
	ctx := log.With()
	for key, value := range fields {
		ctx = ctx.Interface(key, value)
	}
	logger := ctx.Logger()
	return &logger
}

// LoggerWithContext retrieves a logger from context, or returns the default logger
func LoggerWithContext(ctx context.Context, defaultLog *zerolog.Logger) *zerolog.Logger {
	if logger, ok := ctx.Value(ContextKeyLogger).(*zerolog.Logger); ok {
		return logger
	}
	return defaultLog
}

// ContextWithLogger stores a logger in the context
func ContextWithLogger(ctx context.Context, log *zerolog.Logger) context.Context {
	return context.WithValue(ctx, ContextKeyLogger, log)
}

// LogEvent is a helper type for conditional logging
type LogEvent struct {
	logger *zerolog.Logger
	level  zerolog.Level
	event  *zerolog.Event
}

// NewLogEvent creates a new log event at the specified level
func NewLogEvent(log *zerolog.Logger, level zerolog.Level) *LogEvent {
	return &LogEvent{
		logger: log,
		level:  level,
		event:  log.WithLevel(level),
	}
}

// If conditionally adds a field to the log event if the condition is true
func (le *LogEvent) If(condition bool, key string, value interface{}) *LogEvent {
	if condition {
		le.event = le.event.Interface(key, value)
	}
	return le
}

// Str adds a string field to the log event
func (le *LogEvent) Str(key, value string) *LogEvent {
	le.event = le.event.Str(key, value)
	return le
}

// Int adds an int field to the log event
func (le *LogEvent) Int(key string, value int) *LogEvent {
	le.event = le.event.Int(key, value)
	return le
}

// Int64 adds an int64 field to the log event
func (le *LogEvent) Int64(key string, value int64) *LogEvent {
	le.event = le.event.Int64(key, value)
	return le
}

// Err adds an error field to the log event
func (le *LogEvent) Err(err error) *LogEvent {
	le.event = le.event.Err(err)
	return le
}

// Msg sends the log event with a message
func (le *LogEvent) Msg(msg string) {
	le.event.Msg(msg)
}

// Convenience logging functions

// Debug logs a debug message with fields
func Debug(log *zerolog.Logger, msg string, fields ...interface{}) {
	event := log.Debug()
	addFields(event, fields).Msg(msg)
}

// Info logs an info message with fields
func Info(log *zerolog.Logger, msg string, fields ...interface{}) {
	event := log.Info()
	addFields(event, fields).Msg(msg)
}

// Warn logs a warning message with fields
func Warn(log *zerolog.Logger, msg string, fields ...interface{}) {
	event := log.Warn()
	addFields(event, fields).Msg(msg)
}

// Error logs an error message with fields
func Error(log *zerolog.Logger, msg string, fields ...interface{}) {
	event := log.Error()
	addFields(event, fields).Msg(msg)
}

// Fatal logs a fatal message with fields
func Fatal(log *zerolog.Logger, msg string, fields ...interface{}) {
	event := log.Fatal()
	addFields(event, fields).Msg(msg)
}

// Panic logs a panic message with fields
func Panic(log *zerolog.Logger, msg string, fields ...interface{}) {
	event := log.Panic()
	addFields(event, fields).Msg(msg)
}

// addFields adds key-value pairs to a log event
func addFields(event *zerolog.Event, fields []interface{}) *zerolog.Event {
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			if key, ok := fields[i].(string); ok {
				event = event.Interface(key, fields[i+1])
			}
		}
	}
	return event
}

// WithContext returns a logger with context information
func WithContext(log *zerolog.Logger, ctx context.Context) *zerolog.Logger {
	requestID := GetRequestIDFromContext(ctx)
	if requestID != "" {
		return LoggerWithField(log, "request_id", requestID)
	}
	return log
}

// FormatDuration formats a duration for logging
func FormatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// getTimeFormat returns the time format string for zerolog
func getTimeFormat(format string) string {
	if format == "" {
		return time.RFC3339
	}
	// Handle common format aliases (case-insensitive)
	switch strings.ToLower(format) {
	case "unix":
		return time.UnixDate
	case "rfc1123":
		return time.RFC1123
	case "rfc3339":
		return time.RFC3339
	case "rfc3339nano":
		return time.RFC3339Nano
	case "kitchen":
		return time.Kitchen
	case "ansic":
		return time.ANSIC
	case "rfc822":
		return time.RFC822
	case "rfc822z":
		return time.RFC822Z
	case "rfc850":
		return time.RFC850
	case "rfc1123z":
		return time.RFC1123Z
	case "ruby":
		return time.RubyDate
	default:
		// If no match, return as-is (custom format)
		return format
	}
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
