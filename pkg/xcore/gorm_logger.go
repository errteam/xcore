package xcore

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

// GormLoggerConfig holds configuration for the GORM logger
type GormLoggerConfig struct {
	// LogLevel is the minimum log level to record (silent, error, warn, info)
	LogLevel string `mapstructure:"log_level"`
	// SlowThreshold is the threshold for slow query logging
	SlowThreshold time.Duration `mapstructure:"slow_threshold"`
	// IgnoreRecordNotFoundError determines whether to ignore ErrRecordNotFound errors
	IgnoreRecordNotFoundError bool `mapstructure:"ignore_record_not_found"`
	// Colorful enables colorful logging
	Colorful bool `mapstructure:"colorful"`
	// ParameterizedQueries enables parameterized queries in logs
	ParameterizedQueries bool `mapstructure:"parameterized_queries"`
}

// DefaultGormLoggerConfig returns a default GORM logger configuration
func DefaultGormLoggerConfig() *GormLoggerConfig {
	return &GormLoggerConfig{
		LogLevel:                  "warn",
		SlowThreshold:             200 * time.Millisecond,
		IgnoreRecordNotFoundError: false,
		Colorful:                  true,
		ParameterizedQueries:      false,
	}
}

// gormLogger implements gorm/logger.Interface using zerolog
type gormLogger struct {
	config               GormLoggerConfig
	logger               *zerolog.Logger
	level                gormlogger.LogLevel
	slowThreshold        time.Duration
	ignoreRecordNotFound bool
	colorful             bool
}

// NewGormLogger creates a new GORM logger that uses the xcore logger
func NewGormLogger(log *zerolog.Logger, cfg *GormLoggerConfig) gormlogger.Interface {
	level := parseGormLogLevel(cfg.LogLevel)

	return &gormLogger{
		config:               *cfg,
		logger:               log,
		level:                level,
		slowThreshold:        cfg.SlowThreshold,
		ignoreRecordNotFound: cfg.IgnoreRecordNotFoundError,
		colorful:             cfg.Colorful,
	}
}

// parseGormLogLevel converts a string log level to gorm logger level
func parseGormLogLevel(level string) gormlogger.LogLevel {
	switch level {
	case "silent":
		return gormlogger.Silent
	case "error":
		return gormlogger.Error
	case "warn":
		return gormlogger.Warn
	case "info":
		return gormlogger.Info
	default:
		return gormlogger.Warn
	}
}

// LogMode sets the log level for the GORM logger
func (l *gormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.level = level
	return &newLogger
}

// Info logs info messages
func (l *gormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.level >= gormlogger.Info {
		l.logger.Info().Ctx(ctx).Msgf(msg, data...)
	}
}

// Warn logs warning messages
func (l *gormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.level >= gormlogger.Warn {
		l.logger.Warn().Ctx(ctx).Msgf(msg, data...)
	}
}

// Error logs error messages
func (l *gormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.level >= gormlogger.Error {
		l.logger.Error().Ctx(ctx).Msgf(msg, data...)
	}
}

// Trace logs SQL queries and execution details
func (l *gormLogger) Trace(
	ctx context.Context,
	begin time.Time,
	fc func() (sql string, rowsAffected int64),
	err error,
) {
	if l.level <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	log := l.logger.With().Ctx(ctx).Logger()

	// Add request ID if available
	requestID := GetRequestIDFromContext(ctx)
	if requestID != "" {
		log = *LoggerWithField(&log, "request_id", requestID)
	}

	// Get SQL and rows affected
	sql, rows := fc()

	// Check for errors
	if err != nil {
		// Ignore record not found if configured
		if l.ignoreRecordNotFound && err == gorm.ErrRecordNotFound {
			return
		}

		log.Error().
			Err(err).
			Str("sql", sql).
			Int64("rows", rows).
			Str("duration", FormatDuration(elapsed)).
			Msg("SQL error")
		return
	}

	// Check for slow query
	if l.slowThreshold > 0 && elapsed > l.slowThreshold {
		log.Warn().
			Str("sql", sql).
			Int64("rows", rows).
			Str("duration", FormatDuration(elapsed)).
			Msgf("Slow SQL query (>%s)", l.slowThreshold)
		return
	}

	// Log at info level if enabled
	if l.level >= gormlogger.Info {
		log.Info().
			Str("sql", sql).
			Int64("rows", rows).
			Str("duration", FormatDuration(elapsed)).
			Msg("SQL executed")
	}
}

// GormLoggerOption is a function type for configuring GORM logger
type GormLoggerOption func(*gormlogger.Config)

// WithGormLogLevel sets the GORM logger level
func WithGormLogLevel(level string) GormLoggerOption {
	return func(cfg *gormlogger.Config) {
		cfg.LogLevel = parseGormLogLevel(level)
	}
}

// WithGormSlowThreshold sets the slow query threshold
func WithGormSlowThreshold(threshold time.Duration) GormLoggerOption {
	return func(cfg *gormlogger.Config) {
		cfg.SlowThreshold = threshold
	}
}

// WithGormColorful enables or disables colorful logging
func WithGormColorful(colorful bool) GormLoggerOption {
	return func(cfg *gormlogger.Config) {
		cfg.Colorful = colorful
	}
}

// WithGormIgnoreRecordNotFound configures whether to ignore ErrRecordNotFound
func WithGormIgnoreRecordNotFound(ignore bool) GormLoggerOption {
	return func(cfg *gormlogger.Config) {
		cfg.IgnoreRecordNotFoundError = ignore
	}
}

// NewGormLoggerWithOpts creates a GORM logger with options
func NewGormLoggerWithOpts(log *zerolog.Logger, opts ...GormLoggerOption) gormlogger.Interface {
	cfg := gormlogger.Config{
		LogLevel:                  gormlogger.Warn,
		SlowThreshold:             200 * time.Millisecond,
		Colorful:                  true,
		IgnoreRecordNotFoundError: false,
		ParameterizedQueries:      false,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return gormlogger.New(log, cfg)
}

// RegisterGormLogger registers the xcore logger with GORM
// This is a convenience function that returns a GORM config with the custom logger
func RegisterGormLogger(log *zerolog.Logger, cfg *GormLoggerConfig) *gorm.Config {
	return &gorm.Config{
		Logger: NewGormLogger(log, cfg),
	}
}

// RegisterGormLoggerWithOpts registers the xcore logger with GORM using options
func RegisterGormLoggerWithOpts(log *zerolog.Logger, opts ...GormLoggerOption) *gorm.Config {
	return &gorm.Config{
		Logger: NewGormLoggerWithOpts(log, opts...),
	}
}

// LogSQLWithResult is a helper function to log SQL execution results
// This can be used for custom logging in GORM callbacks
func LogSQLWithResult(log *zerolog.Logger, ctx context.Context, begin time.Time, sql string, rows int64, err error) {
	if err != nil {
		log.Error().
			Ctx(ctx).
			Err(err).
			Str("sql", sql).
			Int64("rows", rows).
			Str("duration", FormatDuration(time.Since(begin))).
			Msg("SQL error")
		return
	}

	elapsed := time.Since(begin)
	if elapsed > 200*time.Millisecond {
		log.Warn().
			Ctx(ctx).
			Str("sql", sql).
			Int64("rows", rows).
			Str("duration", FormatDuration(elapsed)).
			Msg("Slow SQL query")
		return
	}

	log.Debug().
		Ctx(ctx).
		Str("sql", sql).
		Int64("rows", rows).
		Str("duration", FormatDuration(elapsed)).
		Msg("SQL executed")
}

// GetCaller returns the caller file and line number for logging
func GetCaller() string {
	return utils.FileWithLineNum()
}

// LogWithCaller logs a message with caller information
func LogWithCaller(log *zerolog.Logger, level zerolog.Level, msg string, fields ...interface{}) {
	event := log.WithLevel(level)
	addFields(event, fields)
	event.Str("caller", GetCaller()).Msg(msg)
}

// DebugWithCaller logs a debug message with caller information
func DebugWithCaller(log *zerolog.Logger, msg string, fields ...interface{}) {
	LogWithCaller(log, zerolog.DebugLevel, msg, fields...)
}

// InfoWithCaller logs an info message with caller information
func InfoWithCaller(log *zerolog.Logger, msg string, fields ...interface{}) {
	LogWithCaller(log, zerolog.InfoLevel, msg, fields...)
}

// WarnWithCaller logs a warning message with caller information
func WarnWithCaller(log *zerolog.Logger, msg string, fields ...interface{}) {
	LogWithCaller(log, zerolog.WarnLevel, msg, fields...)
}

// ErrorWithCaller logs an error message with caller information
func ErrorWithCaller(log *zerolog.Logger, msg string, fields ...interface{}) {
	LogWithCaller(log, zerolog.ErrorLevel, msg, fields...)
}
