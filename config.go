package xcore

// Package xcore provides configuration structs for the xcore framework.
//
// This package defines all configuration structures used throughout the framework.
// Each config struct maps to a specific component (HTTP, Database, Cache, etc.)
// and can be loaded from configuration files or environment variables.
//
// Configuration structs use mapstructure, yaml, and json tags for flexible loading
// with tools like Viper.

// HTTPConfig defines the configuration for the HTTP server.
type HTTPConfig struct {
	Host         string           `mapstructure:"host" yaml:"host" json:"host"`
	Port         int              `mapstructure:"port" yaml:"port" json:"port"`
	ReadTimeout  int              `mapstructure:"read_timeout" yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout int              `mapstructure:"write_timeout" yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout  int              `mapstructure:"idle_timeout" yaml:"idle_timeout" json:"idle_timeout"`
	Middlewares  []string         `mapstructure:"middlewares" yaml:"middlewares" json:"middlewares"`
	CORS         *CORSConfig      `mapstructure:"cors" yaml:"cors" json:"cors"`
	RateLimit    *RateLimitConfig `mapstructure:"rate_limit" yaml:"rate_limit" json:"rate_limit"`
	StaticPath   string           `mapstructure:"static_path" yaml:"static_path" json:"static_path"`
	StaticDir    string           `mapstructure:"static_dir" yaml:"static_dir" json:"static_dir"`
	EnablePprof  bool             `mapstructure:"enable_pprof" yaml:"enable_pprof" json:"enable_pprof"`
}

// CORSConfig defines Cross-Origin Resource Sharing configuration.
type CORSConfig struct {
	AllowedOrigins   []string `mapstructure:"allowed_origins" yaml:"allowed_origins" json:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods" yaml:"allowed_methods" json:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers" yaml:"allowed_headers" json:"allowed_headers"`
	ExposedHeaders   []string `mapstructure:"exposed_headers" yaml:"exposed_headers" json:"exposed_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials" yaml:"allow_credentials" json:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age" yaml:"max_age" json:"max_age"`
	Enabled          bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
}

// RateLimitConfig defines rate limiting configuration.
type RateLimitConfig struct {
	RequestsPerSecond int `mapstructure:"requests_per_second" yaml:"requests_per_second"`
	Burst             int `mapstructure:"burst" yaml:"burst"`
}

// LoggerConfig defines the configuration for the logging system.
type LoggerConfig struct {
	Level          string               `mapstructure:"level" yaml:"level" json:"level"`
	Output         string               `mapstructure:"output" yaml:"output" json:"output"`
	Format         string               `mapstructure:"format" yaml:"format" json:"format"`
	FilePath       string               `mapstructure:"file_path" yaml:"file_path" json:"file_path"`
	MaxSize        int                  `mapstructure:"max_size" yaml:"max_size" json:"max_size"`
	MaxAge         int                  `mapstructure:"max_age" yaml:"max_age" json:"max_age"`
	MaxBackups     int                  `mapstructure:"max_backups" yaml:"max_backups" json:"max_backups"`
	Compress       bool                 `mapstructure:"compress" yaml:"compress" json:"compress"`
	Caller         bool                 `mapstructure:"caller" yaml:"caller" json:"caller"`
	TimestampField string               `mapstructure:"timestamp_field" yaml:"timestamp_field" json:"timestamp_field"`
	Console        *ConsoleLoggerConfig `mapstructure:"console" yaml:"console" json:"console"`
}

type ConsoleLoggerConfig struct {
	Colorable bool `mapstructure:"colorable" yaml:"colorable" json:"colorable"`
}

// DatabaseConfig defines the configuration for database connections.
type DatabaseConfig struct {
	Driver          string `mapstructure:"driver" yaml:"driver" json:"driver"`
	Host            string `mapstructure:"host" yaml:"host" json:"host"`
	Port            int    `mapstructure:"port" yaml:"port" json:"port"`
	User            string `mapstructure:"user" yaml:"user" json:"user"`
	Password        string `mapstructure:"password" yaml:"password" json:"password"`
	DBName          string `mapstructure:"db_name" yaml:"db_name" json:"db_name"`
	SSLMode         string `mapstructure:"ssl_mode" yaml:"ssl_mode" json:"ssl_mode"`
	Charset         string `mapstructure:"charset" yaml:"charset" json:"charset"`
	Timezone        string `mapstructure:"timezone" yaml:"timezone" json:"timezone"`
	MaxOpenConns    int    `mapstructure:"max_open_conns" yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns" yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime" yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
	ConnMaxIdleTime int    `mapstructure:"conn_max_idle_time" yaml:"conn_max_idle_time" json:"conn_max_idle_time"`
	ConnectTimeout  int    `mapstructure:"connect_timeout" yaml:"connect_timeout" json:"connect_timeout"`
	QueryTimeout    int    `mapstructure:"query_timeout" yaml:"query_timeout" json:"query_timeout"`
	PoolMode        string `mapstructure:"pool_mode" yaml:"pool_mode" json:"pool_mode"`
	ConnectionName  string `mapstructure:"connection_name" yaml:"connection_name" json:"connection_name"`
	LogLevel        string `mapstructure:"log_level" yaml:"log_level" json:"log_level"`
	CustomLogger    bool   `mapstructure:"custom_logger" yaml:"custom_logger" json:"custom_logger"`
	Enabled         bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
}

// CacheConfig defines the configuration for the caching system.
type CacheConfig struct {
	Driver          string `mapstructure:"driver" yaml:"driver" json:"driver"`
	RedisAddr       string `mapstructure:"redis_addr" yaml:"redis_addr" json:"redis_addr"`
	RedisPassword   string `mapstructure:"redis_password" yaml:"redis_password" json:"redis_password"`
	RedisDB         int    `mapstructure:"redis_db" yaml:"redis_db" json:"redis_db"`
	RedisPoolSize   int    `mapstructure:"redis_pool_size" yaml:"redis_pool_size" json:"redis_pool_size"`
	RedisTLS        bool   `mapstructure:"redis_tls" yaml:"redis_tls" json:"redis_tls"`
	FilePath        string `mapstructure:"file_path" yaml:"file_path" json:"file_path"`
	TTL             int    `mapstructure:"ttl" yaml:"ttl" json:"ttl"`
	CleanupInterval int    `mapstructure:"cleanup_interval" yaml:"cleanup_interval" json:"cleanup_interval"`
	Enabled         bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
}

// CronConfig defines the configuration for cron jobs.
type CronConfig struct {
	Timezone   string `mapstructure:"timezone" yaml:"timezone"`
	MaxJobs    int    `mapstructure:"max_jobs" yaml:"max_jobs"`
	RecoverPan bool   `mapstructure:"recover_pan" yaml:"recover_pan"`
}

// WebsocketConfig defines the WebSocket configuration.
type WebsocketConfig struct {
	ReadBufferSize  int      `mapstructure:"read_buffer_size" yaml:"read_buffer_size" json:"read_buffer_size"`
	WriteBufferSize int      `mapstructure:"write_buffer_size" yaml:"write_buffer_size" json:"write_buffer_size"`
	PingInterval    int      `mapstructure:"ping_interval" yaml:"ping_interval" json:"ping_interval"`
	PongTimeout     int      `mapstructure:"pong_timeout" yaml:"pong_timeout" json:"pong_timeout"`
	MaxMessageSize  int64    `mapstructure:"max_message_size" yaml:"max_message_size" json:"max_message_size"`
	Enabled         bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	AllowedOrigins  []string `mapstructure:"allowed_origins" yaml:"allowed_origins" json:"allowed_origins"`
}

// GracefulConfig defines the graceful shutdown configuration.
type GracefulConfig struct {
	Timeout int `mapstructure:"timeout" yaml:"timeout"`
}

// Config is the root configuration struct.
type Config struct {
	HTTP      *HTTPConfig      `mapstructure:"http" yaml:"http"`
	Logger    *LoggerConfig    `mapstructure:"logger" yaml:"logger"`
	Database  *DatabaseConfig  `mapstructure:"database" yaml:"database"`
	Cache     *CacheConfig     `mapstructure:"cache" yaml:"cache"`
	Cron      *CronConfig      `mapstructure:"cron" yaml:"cron"`
	Websocket *WebsocketConfig `mapstructure:"websocket" yaml:"websocket"`
	Graceful  *GracefulConfig  `mapstructure:"graceful" yaml:"graceful"`
}
