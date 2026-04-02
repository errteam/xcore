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

type ShutdownCallback func() error

type Graceful struct {
	timeout         time.Duration
	callbackTimeout time.Duration
	callbacks       []ShutdownCallback
	logger          *Logger
	mu              sync.RWMutex
	started         bool
	wg              sync.WaitGroup
	signalChan      chan os.Signal
}

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

func (g *Graceful) SetCallbackTimeout(timeout time.Duration) *Graceful {
	g.callbackTimeout = timeout
	return g
}

func (g *Graceful) SetLogger(logger *Logger) {
	g.logger = logger
}

func (g *Graceful) AddCallback(fn func() error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.callbacks = append(g.callbacks, fn)
}

func (g *Graceful) AddCallbackFunc(name string, fn func() error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.logger != nil {
		g.logger.Info().Str("callback", name).Msg("registered shutdown callback")
	}
	g.callbacks = append(g.callbacks, fn)
}

func (g *Graceful) AddDatabase(db *Database) {
	g.AddCallbackFunc("database", func() error {
		if db != nil {
			return db.Close()
		}
		return nil
	})
}

func (g *Graceful) AddCache(cache Cache) {
	g.AddCallbackFunc("cache", func() error {
		if cache != nil {
			return cache.Clear(context.Background())
		}
		return nil
	})
}

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

func (g *Graceful) AddWebSocket(ws *WebSocket) {
	g.AddCallbackFunc("websocket", func() error {
		if ws != nil {
			ws.Shutdown()
		}
		return nil
	})
}

func (g *Graceful) Wait() {
	g.wg.Wait()
}

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

func (g *Graceful) Shutdown() {
	select {
	case g.signalChan <- os.Interrupt:
	default:
	}
}

func (g *Graceful) SignalChannel() chan os.Signal {
	return g.signalChan
}
