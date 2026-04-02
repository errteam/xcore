// Package xcore provides a Go web application framework for building HTTP services.
//
// This package offers a comprehensive set of utilities including:
//
//   - HTTP routing with gorilla/mux
//   - Middleware support (recovery, compression, rate limiting, CORS, etc.)
//   - JWT authentication
//   - Graceful shutdown handling
//   - Database and cache integration
//   - Logging with zerolog
//   - WebSocket support
//   - Scheduled jobs (cron)
//
// Example usage:
//
//	app := xcore.New().
//	    WithHTTP(&xcore.HTTPConfig{Port: 8080}).
//	    WithLogger(&xcore.LoggerConfig{Level: "debug"}).
//	    WithDatabase(&xcore.DatabaseConfig{...})
//
//	app.Router().GetHandler("/hello", func(c *xcore.Context) error {
//	    return c.JSON(200, map[string]string{"message": "Hello, World!"})
//	})
//
//	if err := app.Run(); err != nil {
//	    log.Fatal(err)
//	}
package xcore

import (
	"fmt"
	"net/http"
	"sync"
)

// App is the main application struct that orchestrates all components of the xcore framework.
// It manages the lifecycle of HTTP servers, databases, caches, cron jobs, WebSocket connections,
// and user-defined services. App provides a fluent builder pattern for configuration.
//
// The App struct is thread-safe for configuration operations but should not be modified after
// Run() has been called. All setter methods (With*) return the App pointer to enable chaining.
//
// Example:
//
//	app := xcore.New().
//	    WithConfig(cfg).
//	    WithHTTP(&xcore.HTTPConfig{Port: 8080}).
//	    WithLogger(&xcore.LoggerConfig{Level: "info"}).
//	    WithDatabase(dbConfig).
//	    WithCache(cacheConfig).
//	    WithCron(cronConfig)
//
//	// Register routes
//	app.Router().GetHandler("/api/health", func(c *xcore.Context) error {
//	    return c.JSON(200, map[string]string{"status": "ok"})
//	})
//
//	// Run the application (blocking)
//	if err := app.Run(); err != nil {
//	    log.Fatal(err)
//	}
type App struct {
	config      *Config
	logger      *Logger
	router      *Router
	database    *Database
	cache       Cache
	cron        *Cron
	websocket   *WebSocket
	graceful    *Graceful
	services    *ServiceManager
	middlewares []func(http.Handler) http.Handler

	mu      sync.RWMutex
	started bool
	server  *http.Server
}

// New creates a new App instance with default configuration.
// This is the starting point for building an xcore application.
//
// The returned App has default Config and an empty ServiceManager.
// Additional configuration is applied using the With* methods before calling Run().
//
// Example:
//
//	app := xcore.New()
//	app.WithHTTP(&xcore.HTTPConfig{Port: 8080})
func New() *App {
	return &App{
		config:   &Config{},
		services: NewServiceManager(nil),
	}
}

// WithConfig sets the application configuration.
// The config parameter should not be nil; if nil is passed, a default Config is used.
//
// Configuration includes settings for HTTP server, logger, database, cache, cron jobs,
// WebSocket, and graceful shutdown behavior.
func (a *App) WithConfig(cfg *Config) *App {
	a.config = cfg
	return a
}

// WithHTTP configures the HTTP server settings.
// If cfg is nil, default values are applied (Port: 8080).
// This method also initializes the Router if not already set.
func (a *App) WithHTTP(cfg *HTTPConfig) *App {
	if cfg == nil {
		cfg = &HTTPConfig{Port: 8080}
	}
	a.config.HTTP = cfg
	if a.logger != nil {
		a.router = NewRouter(cfg).WithLogger(a.logger)
	} else {
		a.router = NewRouter(cfg)
	}
	return a
}

// WithLogger configures the logger with the given configuration.
// If cfg is nil, default values are applied (Level: "info", Output: "console", Format: "console").
// A fallback logger is created if logger initialization fails.
func (a *App) WithLogger(cfg *LoggerConfig) *App {
	if cfg == nil {
		cfg = &LoggerConfig{Level: "info", Output: "console", Format: "console"}
	}
	a.config.Logger = cfg
	logger, err := NewLogger(cfg)
	if err != nil {
		fmt.Printf("failed to create logger: %v\n", err)
		logger, _ = NewLogger(nil)
	}
	a.logger = logger
	a.services.logger = logger
	return a
}

// WithDatabase initializes the database connection using the provided configuration.
// If cfg is nil, the method returns without changes.
// On failure, the error is logged but the App continues (database is set to nil).
// The database is registered with graceful shutdown handler.
func (a *App) WithDatabase(cfg *DatabaseConfig) *App {
	if cfg == nil {
		return a
	}
	a.config.Database = cfg
	db, err := NewDatabase(cfg, a.logger)
	if err != nil {
		if a.logger != nil {
			a.logger.Error().Err(err).Msg("failed to initialize database")
		}
		return a
	}
	a.database = db
	return a
}

// WithCache initializes the cache using the provided configuration.
// If cfg is nil, default configuration is applied (Driver: "memory").
// Supported drivers: "memory", "file", "redis".
// On failure, the error is logged but the App continues.
func (a *App) WithCache(cfg *CacheConfig) *App {
	if cfg == nil {
		cfg = &CacheConfig{Driver: "memory"}
	}
	a.config.Cache = cfg
	c, err := NewCache(cfg)
	if err != nil {
		if a.logger != nil {
			a.logger.Error().Err(err).Msg("failed to initialize cache")
		}
		return a
	}
	a.cache = c
	return a
}

// WithCron initializes the cron job scheduler with the provided configuration.
// If cfg is nil, default configuration is applied.
// The cron scheduler is started immediately when Run() is called.
func (a *App) WithCron(cfg *CronConfig) *App {
	if cfg == nil {
		cfg = &CronConfig{}
	}
	a.config.Cron = cfg
	a.cron = NewCron(cfg, a.logger)
	return a
}

// WithWebSocket initializes WebSocket support with the given configuration.
// If cfg is nil, default configuration is applied.
func (a *App) WithWebSocket(cfg *WebsocketConfig) *App {
	if cfg == nil {
		cfg = &WebsocketConfig{}
	}
	a.config.Websocket = cfg
	a.websocket = NewWebSocket(cfg, a.logger)
	return a
}

// WithGraceful configures graceful shutdown handling.
// If cfg is nil, default configuration is applied (Timeout: 30 seconds).
// Graceful shutdown ensures proper termination of HTTP servers, databases, caches,
// WebSocket connections, and user services.
func (a *App) WithGraceful(cfg *GracefulConfig) *App {
	if cfg == nil {
		cfg = &GracefulConfig{Timeout: 30}
	}
	a.config.Graceful = cfg
	a.graceful = NewGraceful(cfg.Timeout, a.logger)
	return a
}

// WithService adds a user-defined service to the application.
// Services are started in order during app.Run() and stopped in reverse order during shutdown.
// The service must implement the Service interface (Start, Stop, Name methods).
func (a *App) WithService(service Service) *App {
	a.services.Add(service)
	return a
}

// Use registers a global middleware that will be applied to all routes.
// Middlewares are applied in the order they are registered.
// The middleware function receives the next handler and returns a wrapped handler.
//
// Common middlewares include recovery, compression, rate limiting, CORS, etc.
// Example:
//
//	app.Use(xcore.NewRecovery(nil).Middleware)
//	app.Use(xcore.NewCompression(gzip.DefaultCompression).Middleware)
func (a *App) Use(mw func(http.Handler) http.Handler) *App {
	a.middlewares = append(a.middlewares, mw)
	if a.router != nil {
		a.router.UseMiddleware(mw)
	}
	return a
}

// Logger returns the application's logger instance.
// Returns nil if WithLogger() has not been called.
func (a *App) Logger() *Logger {
	return a.logger
}

// Router returns the application's router instance.
// Creates a new Router with default configuration if not already set.
// The router is used to register HTTP handlers and routes.
func (a *App) Router() *Router {
	if a.router == nil {
		a.router = NewRouter(nil)
	}
	return a.router
}

// Database returns the database instance.
// Returns nil if WithDatabase() has not been called or if initialization failed.
func (a *App) Database() *Database {
	return a.database
}

// Cache returns the cache instance.
// Returns nil if WithCache() has not been called or if initialization failed.
func (a *App) Cache() Cache {
	return a.cache
}

// Cron returns the cron scheduler instance.
// Returns nil if WithCron() has not been called.
func (a *App) Cron() *Cron {
	return a.cron
}

// WebSocket returns the WebSocket manager instance.
// Returns nil if WithWebSocket() has not been called.
func (a *App) WebSocket() *WebSocket {
	return a.websocket
}

// Graceful returns the graceful shutdown handler instance.
// Returns nil if WithGraceful() has not been called.
func (a *App) Graceful() *Graceful {
	return a.graceful
}

func (a *App) Services() *ServiceManager {
	return a.services
}

// Run starts the application and blocks until shutdown.
// It initializes all components, starts the HTTP server (if configured),
// starts all registered services, and waits for shutdown signals.
//
// Run performs the following steps:
//  1. Marks the app as started (prevents multiple calls)
//  2. Creates a default logger if none configured
//  3. Creates a default graceful shutdown handler if none configured
//  4. Registers database, cache, and WebSocket with graceful shutdown
//  5. Starts cron jobs (if configured)
//  6. Starts the HTTP server in a goroutine
//  7. Starts all user-defined services
//  8. Registers services shutdown with graceful handler
//  9. Runs graceful shutdown and waits for completion
//
// Returns an error if the app has already been started.
func (a *App) Run() error {
	if a.started {
		return fmt.Errorf("app already started")
	}

	a.mu.Lock()
	a.started = true
	a.mu.Unlock()

	if a.logger == nil {
		logger, _ := NewLogger(nil)
		a.logger = logger
		a.services.logger = logger
	}

	if a.graceful == nil {
		a.graceful = NewGraceful(30, a.logger)
	}

	if a.database != nil {
		a.graceful.AddDatabase(a.database)
	}
	if a.cache != nil {
		a.graceful.AddCache(a.cache)
	}
	if a.websocket != nil {
		a.graceful.AddWebSocket(a.websocket)
	}

	if a.router != nil {
		a.server = a.router.Server()
	}

	if a.cron != nil {
		a.graceful.AddCallbackFunc("cron", func() error {
			a.cron.Stop()
			return nil
		})
		a.cron.Start()
	}

	if a.server != nil {
		go func() {
			if a.logger != nil {
				a.logger.Info().Str("addr", a.server.Addr).Msg("starting HTTP server")
			}
			if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				if a.logger != nil {
					a.logger.Error().Err(err).Msg("HTTP server error")
				}
			}
		}()
	}

	if err := a.services.StartAll(); err != nil {
		if a.logger != nil {
			a.logger.Error().Err(err).Msg("failed to start services")
		}
		return err
	}

	a.graceful.AddCallbackFunc("services", func() error {
		a.services.StopAll()
		return nil
	})

	var servers []*http.Server
	if a.server != nil {
		servers = append(servers, a.server)
	}

	a.graceful.Run(servers...)
	a.graceful.Wait()

	return nil
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if a.router != nil {
		a.router.ServeHTTP(w, r)
	}
}
