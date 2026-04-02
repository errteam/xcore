// Package xcore provides graceful shutdown functionality for the xcore framework.
//
// The graceful package handles coordinated shutdown of HTTP servers, databases,
// caches, WebSocket connections, cron jobs, and user-defined services.
// It ensures all resources are properly released and pending requests are completed.
//
// Key features:
//   - Signal handling (SIGINT, SIGTERM)
//   - Configurable shutdown timeout
//   - Callback execution with timeout
//   - Server shutdown with context timeout
package xcore

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// ShutdownCallback is a function type that will be called during graceful shutdown.
// It should perform cleanup operations and return an error if the cleanup failed.
// The callback is called with a limited timeout, so it should be non-blocking.
type ShutdownCallback func() error

// Graceful manages the graceful shutdown process for the application.
// It coordinates shutdown of multiple components including HTTP servers,
// databases, caches, WebSocket connections, and user-defined services.
//
// The Graceful struct is safe for concurrent use and can be configured
// with custom timeouts for overall shutdown and individual callbacks.
type Graceful struct {
	timeout         time.Duration      // Maximum time to wait for shutdown
	callbackTimeout time.Duration      // Maximum time for each callback
	callbacks       []ShutdownCallback // User-defined cleanup callbacks
	logger          *Logger            // Logger for shutdown events
	mu              sync.RWMutex       // Protects callbacks
	started         bool               // Whether Run() has been called
	wg              sync.WaitGroup     // Waits for background goroutines
	signalChan      chan os.Signal     // Receives shutdown signals
}

// NewGraceful creates a new Graceful shutdown handler with the specified timeout.
// The timeout parameter specifies the maximum time to wait for shutdown in seconds.
// If timeout <= 0, defaults to 30 seconds.
// The logger is optional and is used to log shutdown events.
func NewGraceful(timeout int, logger *Logger) *Graceful {
	if timeout <= 0 {
		timeout = 30
	}
	return &Graceful{
		timeout:         time.Duration(timeout) * time.Second,
		callbackTimeout: 10 * time.Second,
		callbacks:       make([]ShutdownCallback, 0),
		logger:          logger,
		signalChan:      make(chan os.Signal, 1),
	}
}

// SetCallbackTimeout sets the maximum time allowed for each shutdown callback to execute.
// If a callback exceeds this timeout, it will be cancelled and the next callback will proceed.
// Default is 10 seconds if not set.
func (g *Graceful) SetCallbackTimeout(timeout time.Duration) *Graceful {
	g.callbackTimeout = timeout
	return g
}

// SetLogger sets the logger for graceful shutdown events.
// If nil is passed, no logging occurs during shutdown.
func (g *Graceful) SetLogger(logger *Logger) {
	g.logger = logger
}

// AddCallback registers a callback function to be called during graceful shutdown.
// Callbacks are executed in the order they are registered.
// Each callback should be non-blocking and complete within the callback timeout.
// If a callback returns an error, it is logged but shutdown continues.
func (g *Graceful) AddCallback(fn func() error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.callbacks = append(g.callbacks, fn)
}

// AddCallbackFunc registers a named callback function to be called during graceful shutdown.
// The name is used for logging purposes.
// Callbacks are executed in the order they are registered.
func (g *Graceful) AddCallbackFunc(name string, fn func() error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.logger != nil {
		g.logger.Info().Str("callback", name).Msg("registered shutdown callback")
	}
	g.callbacks = append(g.callbacks, fn)
}

// AddDatabase registers a database for graceful shutdown.
// The database's Close() method will be called during shutdown.
func (g *Graceful) AddDatabase(db *Database) {
	g.AddCallbackFunc("database", func() error {
		if db != nil {
			return db.Close()
		}
		return nil
	})
}

// AddCache registers a cache for graceful shutdown.
// The cache's Clear() method will be called during shutdown.
func (g *Graceful) AddCache(cache Cache) {
	g.AddCallbackFunc("cache", func() error {
		if cache != nil {
			return cache.Clear(context.Background())
		}
		return nil
	})
}

// AddServer registers an HTTP server for graceful shutdown.
// The server's Shutdown() method will be called during shutdown with a context timeout.
// If server is nil, no action is taken.
func (g *Graceful) AddServer(server *http.Server) {
	g.AddCallbackFunc("http_server", func() error {
		if server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), g.timeout)
			defer cancel()
			return server.Shutdown(ctx)
		}
		return nil
	})
}

// AddWebSocket registers a WebSocket for graceful shutdown.
// The WebSocket's Shutdown() method will be called during shutdown.
func (g *Graceful) AddWebSocket(ws *WebSocket) {
	g.AddCallbackFunc("websocket", func() error {
		if ws != nil {
			ws.Shutdown()
		}
		return nil
	})
}

// Wait blocks until the graceful shutdown process has completed.
// This should be called after Run() to keep the main goroutine alive
// until a shutdown signal is received.
func (g *Graceful) Wait() {
	g.wg.Wait()
}

// Run starts the graceful shutdown handler and listens for shutdown signals.
// It should be called after all components have been registered.
// The servers parameter is a list of additional HTTP servers to shutdown.
// Run returns immediately after setting up signal handling.
func (g *Graceful) Run(servers ...*http.Server) {
	if g.started {
		return
	}
	g.started = true

	signal.Notify(g.signalChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-g.signalChan
		if g.logger != nil {
			g.logger.Info().Str("signal", sig.String()).Msg("received shutdown signal")
		}
		g.shutdown(servers...)
	}()

	if g.logger != nil {
		g.logger.Info().Dur("timeout", g.timeout).Msg("graceful shutdown handler started")
	}
}

// shutdown performs the actual graceful shutdown process.
// It executes all registered callbacks in order, then shuts down HTTP servers.
// This method is called automatically when a shutdown signal is received.
func (g *Graceful) shutdown(servers ...*http.Server) {
	if g.logger != nil {
		g.logger.Info().Msg("starting graceful shutdown")
	}

	ctx, cancel := context.WithTimeout(context.Background(), g.timeout)
	defer cancel()

	g.mu.RLock()
	callbacks := make([]ShutdownCallback, len(g.callbacks))
	copy(callbacks, g.callbacks)
	g.mu.RUnlock()

	for i, cb := range callbacks {
		cbCtx, cbCancel := context.WithTimeout(ctx, g.callbackTimeout)

		done := make(chan error, 1)
		go func() {
			done <- cb()
		}()

		select {
		case err := <-done:
			if err != nil {
				if g.logger != nil {
					g.logger.Error().Err(err).Int("callback", i).Msg("shutdown callback failed")
				}
			}
		case <-cbCtx.Done():
			if g.logger != nil {
				g.logger.Warn().Int("callback", i).Msg("shutdown callback timed out")
			}
		}
		cbCancel()
	}

	for _, server := range servers {
		if server != nil {
			if err := server.Shutdown(ctx); err != nil {
				if g.logger != nil {
					g.logger.Error().Err(err).Msg("server shutdown failed")
				}
			}
		}
	}

	if g.logger != nil {
		g.logger.Info().Msg("graceful shutdown completed")
	}
}

// Shutdown triggers a graceful shutdown manually.
// This is useful for testing or when shutdown needs to be triggered from code
// rather than receiving a signal.
func (g *Graceful) Shutdown() {
	select {
	case g.signalChan <- os.Interrupt:
	default:
	}
}

// SignalChannel returns the signal channel for external monitoring.
// This allows other parts of the application to observe shutdown signals.
func (g *Graceful) SignalChannel() chan os.Signal {
	return g.signalChan
}
