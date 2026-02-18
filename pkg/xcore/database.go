package xcore

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// DatabaseConfig holds the configuration for database connections
type DatabaseConfig struct {
	Driver          string `mapstructure:"driver"`
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	Name            string `mapstructure:"name"`
	DSN             string `mapstructure:"dsn"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	ConnMaxLifetime string `mapstructure:"conn_max_lifetime"`

	// GORM logger settings
	LogLevel             string `mapstructure:"log_level"`
	SlowThreshold        string `mapstructure:"slow_threshold"`
	IgnoreRecordNotFound bool   `mapstructure:"ignore_record_not_found"`
}

// GetDSN returns the appropriate DSN string based on the driver
func (c *DatabaseConfig) GetDSN() string {
	// For SQLite, use the DSN field directly
	if c.Driver == "sqlite" {
		if c.DSN != "" {
			return c.DSN
		}
		return "app.db"
	}

	// For PostgreSQL and MySQL, build connection string
	// Only use DSN field if explicitly provided (not the default app.db)
	if c.DSN != "" && c.DSN != "app.db" {
		return c.DSN
	}

	switch c.Driver {
	case "postgres":
		return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=UTC",
			c.Host, c.User, c.Password, c.Name, c.Port)
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			c.User, c.Password, c.Host, c.Port, c.Name)
	default:
		return "app.db"
	}
}

// GetConnMaxLifetime returns the connection max lifetime as a duration
func (c *DatabaseConfig) GetConnMaxLifetime() time.Duration {
	d, err := time.ParseDuration(c.ConnMaxLifetime)
	if err != nil {
		return time.Hour
	}
	return d
}

// GetSlowThreshold returns the slow query threshold as a duration
func (c *DatabaseConfig) GetSlowThreshold() time.Duration {
	if c.SlowThreshold == "" {
		return 200 * time.Millisecond
	}
	d, err := time.ParseDuration(c.SlowThreshold)
	if err != nil {
		return 200 * time.Millisecond
	}
	return d
}

// DefaultDatabaseConfig returns a default database configuration
func DefaultDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		Driver:               "sqlite",
		Host:                 "localhost",
		Port:                 5432,
		User:                 "postgres",
		Password:             "",
		Name:                 "app",
		MaxIdleConns:         10,
		MaxOpenConns:         100,
		ConnMaxLifetime:      "1h",
		LogLevel:             "warn",
		SlowThreshold:        "200ms",
		IgnoreRecordNotFound: false,
	}
}

// InitializeDatabase connects to the database and returns the GORM DB instance
// It uses the xcore logger for GORM logging
func InitializeDatabase(cfg *DatabaseConfig, logger *zerolog.Logger) (*gorm.DB, error) {
	var dialector gorm.Dialector

	dsn := cfg.GetDSN()

	switch cfg.Driver {
	case "postgres":
		dialector = postgres.Open(dsn)
	case "mysql":
		dialector = mysql.Open(dsn)
	case "sqlite":
		dialector = sqlite.Open(dsn)
	default:
		logger.Warn().Str("driver", cfg.Driver).Msg("Unknown driver, defaulting to SQLite")
		dialector = sqlite.Open(dsn)
	}

	// Create GORM logger with Zerolog using config
	gormCfg := RegisterGormLogger(logger, &GormLoggerConfig{
		LogLevel:                  cfg.LogLevel,
		SlowThreshold:             cfg.GetSlowThreshold(),
		IgnoreRecordNotFoundError: cfg.IgnoreRecordNotFound,
		Colorful:                  false, // zerolog handles formatting
		ParameterizedQueries:      false,
	})

	db, err := gorm.Open(dialector, gormCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL DB for connection pool settings
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying SQL DB: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cfg.GetConnMaxLifetime())

	logger.Info().
		Str("driver", cfg.Driver).
		Str("host", cfg.Host).
		Int("port", cfg.Port).
		Str("name", cfg.Name).
		Int("max_idle_conns", cfg.MaxIdleConns).
		Int("max_open_conns", cfg.MaxOpenConns).
		Msg("Successfully connected to database")

	return db, nil
}

// InitializeDatabaseWithOpts connects to the database with custom GORM logger options
// This provides more flexibility for configuring the GORM logger
func InitializeDatabaseWithOpts(cfg *DatabaseConfig, logger *zerolog.Logger, opts ...GormLoggerOption) (*gorm.DB, error) {
	var dialector gorm.Dialector

	dsn := cfg.GetDSN()

	switch cfg.Driver {
	case "postgres":
		dialector = postgres.Open(dsn)
	case "mysql":
		dialector = mysql.Open(dsn)
	case "sqlite":
		dialector = sqlite.Open(dsn)
	default:
		logger.Warn().Str("driver", cfg.Driver).Msg("Unknown driver, defaulting to SQLite")
		dialector = sqlite.Open(dsn)
	}

	// Create GORM logger with custom options
	gormCfg := RegisterGormLoggerWithOpts(logger, opts...)

	db, err := gorm.Open(dialector, gormCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL DB for connection pool settings
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying SQL DB: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cfg.GetConnMaxLifetime())

	logger.Info().
		Str("driver", cfg.Driver).
		Str("host", cfg.Host).
		Int("port", cfg.Port).
		Str("name", cfg.Name).
		Int("max_idle_conns", cfg.MaxIdleConns).
		Int("max_open_conns", cfg.MaxOpenConns).
		Msg("Successfully connected to database")

	return db, nil
}

// AutoMigrate runs auto migration for given models
func AutoMigrate(db *gorm.DB, logger *zerolog.Logger, models ...interface{}) error {
	if err := db.AutoMigrate(models...); err != nil {
		return fmt.Errorf("failed to auto migrate: %w", err)
	}

	logger.Info().Int("models", len(models)).Msg("Database migration completed successfully")
	return nil
}

// CloseDatabase gracefully closes the database connection
func CloseDatabase(db *gorm.DB, logger *zerolog.Logger) error {
	if db == nil {
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying SQL DB: %w", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}

	logger.Info().Msg("Database connection closed successfully")
	return nil
}

// WithTransaction executes a function within a database transaction
func WithTransaction(db *gorm.DB, fn func(tx *gorm.DB) error) error {
	return db.Transaction(func(tx *gorm.DB) error {
		return fn(tx)
	})
}

// WithTransactionAndErrorHandling executes a function within a transaction with custom error handling
func WithTransactionAndErrorHandling(db *gorm.DB, logger *zerolog.Logger, fn func(tx *gorm.DB) error) (err error) {
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			logger.Error().Interface("panic", r).Msg("Recovered from panic in transaction, rolling back")
		}
	}()

	if err = fn(tx); err != nil {
		if rbErr := tx.Rollback().Error; rbErr != nil {
			logger.Error().Err(rbErr).Msg("Failed to rollback transaction")
		}
		logger.Error().Err(err).Msg("Transaction failed, rolled back")
		return err
	}

	if err = tx.Commit().Error; err != nil {
		logger.Error().Err(err).Msg("Failed to commit transaction")
		return err
	}

	logger.Debug().Msg("Transaction committed successfully")
	return nil
}
