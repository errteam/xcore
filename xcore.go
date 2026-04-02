package xcore

import (
	"fmt"
	"net/http"
	"sync"
)

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

func New() *App {
	return &App{
		config:   &Config{},
		services: NewServiceManager(nil),
	}
}

func (a *App) WithConfig(cfg *Config) *App {
	a.config = cfg
	return a
}

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

func (a *App) WithCron(cfg *CronConfig) *App {
	if cfg == nil {
		cfg = &CronConfig{}
	}
	a.config.Cron = cfg
	a.cron = NewCron(cfg, a.logger)
	return a
}

func (a *App) WithWebSocket(cfg *WebsocketConfig) *App {
	if cfg == nil {
		cfg = &WebsocketConfig{}
	}
	a.config.Websocket = cfg
	a.websocket = NewWebSocket(cfg, a.logger)
	return a
}

func (a *App) WithGraceful(cfg *GracefulConfig) *App {
	if cfg == nil {
		cfg = &GracefulConfig{Timeout: 30}
	}
	a.config.Graceful = cfg
	a.graceful = NewGraceful(cfg.Timeout, a.logger)
	return a
}

func (a *App) WithService(service Service) *App {
	a.services.Add(service)
	return a
}

func (a *App) Use(mw func(http.Handler) http.Handler) *App {
	a.middlewares = append(a.middlewares, mw)
	if a.router != nil {
		a.router.UseMiddleware(mw)
	}
	return a
}

func (a *App) Logger() *Logger {
	return a.logger
}

func (a *App) Router() *Router {
	if a.router == nil {
		a.router = NewRouter(nil)
	}
	return a.router
}

func (a *App) Database() *Database {
	return a.database
}

func (a *App) Cache() Cache {
	return a.cache
}

func (a *App) Cron() *Cron {
	return a.cron
}

func (a *App) WebSocket() *WebSocket {
	return a.websocket
}

func (a *App) Graceful() *Graceful {
	return a.graceful
}

func (a *App) Services() *ServiceManager {
	return a.services
}

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
