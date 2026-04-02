// Package xcore provides database functionality using GORM.
//
// This package wraps the GORM ORM to provide database operations with
// support for PostgreSQL, MySQL, SQLite, and SQL Server.
// It includes connection pooling, transaction support, and health checks.
package xcore

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const ctxTxKey contextKey = "TxKey"

// Database wraps a GORM DB instance with configuration and logging.
type Database struct {
	*gorm.DB
	config *DatabaseConfig
	logger *Logger
}

// DBHealth represents the health status of a database connection.
type DBHealth struct {
	Status    string        `json:"status"`
	Latency   time.Duration `json:"latency,omitempty"`
	Error     string        `json:"error,omitempty"`
	ConnStats ConnStats     `json:"conn_stats,omitempty"`
}

// ConnStats holds database connection pool statistics.
type ConnStats struct {
	OpenConns    int           `json:"open_conns"`
	IdleConns    int           `json:"idle_conns"`
	InUseConns   int           `json:"in_use_conns"`
	WaitCount    int64         `json:"wait_count"`
	WaitDuration time.Duration `json:"wait_duration"`
	MaxOpenConns int           `json:"max_open_conns"`
}

// Database returns the underlying sql.DB instance for direct SQL operations.
func (d *Database) Database() (*sql.DB, error) {
	return d.DB.DB()
}

// PoolStats returns connection pool statistics.
func (d *Database) PoolStats(ctx context.Context) (ConnStats, error) {
	sqlDB, err := d.Database()
	if err != nil {
		return ConnStats{}, err
	}

	s := sqlDB.Stats()
	return ConnStats{
		OpenConns:    s.OpenConnections,
		IdleConns:    s.Idle,
		InUseConns:   s.InUse,
		WaitCount:    s.WaitCount,
		WaitDuration: s.WaitDuration,
		MaxOpenConns: s.MaxOpenConnections,
	}, nil
}

// Health checks the database connection and returns health status.
// Includes latency measurement and connection pool stats.
func (d *Database) Health(ctx context.Context) DBHealth {
	start := time.Now()
	sqlDB, err := d.Database()
	if err != nil {
		return DBHealth{Status: "unhealthy", Error: err.Error()}
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		return DBHealth{Status: "unhealthy", Error: err.Error(), Latency: time.Since(start)}
	}

	s := sqlDB.Stats()
	return DBHealth{
		Status:  "healthy",
		Latency: time.Since(start),
		ConnStats: ConnStats{
			OpenConns:    s.OpenConnections,
			IdleConns:    s.Idle,
			InUseConns:   s.InUse,
			WaitCount:    s.WaitCount,
			WaitDuration: s.WaitDuration,
			MaxOpenConns: s.MaxOpenConnections,
		},
	}
}

// Ping pings the database to check connectivity.
func (d *Database) Ping(ctx context.Context) error {
	sqlDB, err := d.Database()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// Close closes the underlying database connection.
func (d *Database) Close() error {
	sqlDB, err := d.Database()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Transaction executes a function within a database transaction.
// If the function returns an error, the transaction is rolled back.
// Otherwise, the transaction is committed.
// The transaction is passed to the function via context (ctxTxKey).
func (d *Database) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return d.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ctx = context.WithValue(ctx, ctxTxKey, tx)
		return fn(ctx)
	})
}

// ConnectWithRetry attempts to connect to the database with retry logic.
// It will retry up to maxRetries times with the specified retryDelay between attempts.
// Returns the connected database or an error if all retries fail.
func (d *Database) ConnectWithRetry(cfg *DatabaseConfig, logger *Logger, maxRetries int, retryDelay time.Duration) (*Database, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		db, err := NewDatabase(cfg, logger)
		if err == nil {
			return db, nil
		}
		lastErr = err
		if logger != nil {
			logger.Warn().Int("attempt", i+1).Err(err).Msg("database connection failed, retrying...")
		}
		time.Sleep(retryDelay)
	}
	return nil, fmt.Errorf("failed to connect after %d retries: %w", maxRetries, lastErr)
}

// NewDatabase creates a new Database instance with the given configuration.
// Supports drivers: postgres, mysql, sqlite, sqlserver.
// Configures connection pooling with default or custom values.
func NewDatabase(cfg *DatabaseConfig, logger *Logger) (*Database, error) {
	if cfg == nil {
		return nil, fmt.Errorf("database config is required")
	}

	if cfg.Driver == "" {
		cfg.Driver = "sqlite"
	}

	dsn := getDSN(cfg)

	var dialector gorm.Dialector

	switch cfg.Driver {
	case "postgres":
		dialector = postgres.Open(dsn)
	case "mysql":
		dialector = mysql.Open(dsn)
	case "sqlite":
		dialector = sqlite.Open(dsn)
	case "sqlserver":
		dialector = sqlserver.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	dbCfg := &gorm.Config{
		NowFunc: time.Now,
	}

	if cfg.CustomLogger && logger != nil {
		dbCfg.Logger = &GormLogger{logger: logger, logLevel: getGormLogLevel(cfg.LogLevel)}
	} else {
		dbCfg.Logger = &GormLogger{logger: logger, logLevel: getGormLogLevel(cfg.LogLevel)}
	}

	db, err := gorm.Open(dialector, dbCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	} else {
		sqlDB.SetMaxOpenConns(25)
	}

	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	} else {
		sqlDB.SetMaxIdleConns(5)
	}

	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	} else {
		sqlDB.SetConnMaxLifetime(300 * time.Second)
	}

	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTime) * time.Second)
	}

	return &Database{
		DB:     db,
		config: cfg,
		logger: logger,
	}, nil
}

// getDSN builds the database connection string (DSN) based on the driver type.
func getDSN(cfg *DatabaseConfig) string {
	switch cfg.Driver {
	case "postgres":
		sslmode := "disable"
		if cfg.SSLMode != "" {
			sslmode = cfg.SSLMode
		}
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, sslmode)
		if cfg.ConnectTimeout > 0 {
			dsn += fmt.Sprintf(" connect_timeout=%d", cfg.ConnectTimeout)
		}
		if cfg.PoolMode != "" {
			dsn += fmt.Sprintf(" pool_mode=%s", cfg.PoolMode)
		}
		return dsn
	case "mysql":
		charset := "utf8mb4"
		if cfg.Charset != "" {
			charset = cfg.Charset
		}
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=%s",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName, charset, cfg.Timezone)
		if cfg.ConnectTimeout > 0 {
			dsn += fmt.Sprintf("&timeout=%ds", cfg.ConnectTimeout)
		}
		if cfg.ConnectionName != "" {
			dsn += fmt.Sprintf("&connectionName=%s", cfg.ConnectionName)
		}
		return dsn
	case "sqlite":
		return cfg.DBName
	case "sqlserver":
		dsn := fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)
		if cfg.ConnectTimeout > 0 {
			dsn += fmt.Sprintf("&connection timeout=%d", cfg.ConnectTimeout)
		}
		return dsn
	default:
		return ""
	}
}

// getGormLogLevel converts a string log level to GORM's log level.
func getGormLogLevel(level string) logger.LogLevel {
	switch level {
	case "silent":
		return logger.Silent
	case "error":
		return logger.Error
	case "warn":
		return logger.Warn
	case "info":
		return logger.Info
	default:
		return logger.Silent
	}
}

// GormLogger wraps the xcore Logger for GORM logging.
type GormLogger struct {
	logger   *Logger
	logLevel logger.LogLevel
	mu       sync.RWMutex
}

func (g *GormLogger) LogMode(level logger.LogLevel) logger.Interface {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.logLevel = level
	return g
}

func (g *GormLogger) Info(ctx context.Context, msg string, args ...interface{}) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.logLevel >= logger.Info {
		g.logger.Info().Msg(msg)
	}
}

func (g *GormLogger) Warn(ctx context.Context, msg string, args ...interface{}) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.logLevel >= logger.Warn {
		g.logger.Warn().Msg(msg)
	}
}

func (g *GormLogger) Error(ctx context.Context, msg string, args ...interface{}) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.logLevel >= logger.Error {
		g.logger.Error().Msg(msg)
	}
}

func (g *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.logLevel >= logger.Info {
		sql, _ := fc()
		g.logger.Debug().Str("sql", sql).Dur("duration", time.Since(begin)).Msg("query")
	}
}
