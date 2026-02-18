package xcore

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
)

// ShutdownHandler manages graceful shutdown of the application
type ShutdownHandler struct {
	mu            sync.Mutex
	ctx           context.Context
	cancel        context.CancelFunc
	done          chan struct{}
	shutdownFuncs []ShutdownFunc
	logger        *zerolog.Logger
	timeout       time.Duration
	signals       []os.Signal
	isShuttingDown bool
}

// ShutdownFunc is a function that will be called during shutdown
type ShutdownFunc func(ctx context.Context) error

// ShutdownConfig holds configuration for shutdown handler
type ShutdownConfig struct {
	// Timeout is the maximum time to wait for shutdown
	Timeout time.Duration
	// Signals to listen for (default: SIGINT, SIGTERM)
	Signals []os.Signal
	// Logger for shutdown messages
	Logger *zerolog.Logger
}

// DefaultShutdownConfig returns a default shutdown configuration
func DefaultShutdownConfig() *ShutdownConfig {
	return &ShutdownConfig{
		Timeout: 30 * time.Second,
		Signals: []os.Signal{syscall.SIGINT, syscall.SIGTERM},
		Logger:  nil, // Will use a default logger if nil
	}
}

// NewShutdownHandler creates a new shutdown handler
func NewShutdownHandler(cfg *ShutdownConfig) *ShutdownHandler {
	if cfg == nil {
		cfg = DefaultShutdownConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	logger := cfg.Logger
	if logger == nil {
		// Create a default logger
		defaultLogger := zerolog.New(os.Stderr).With().Timestamp().Logger()
		logger = &defaultLogger
	}

	sh := &ShutdownHandler{
		ctx:      ctx,
		cancel:   cancel,
		done:     make(chan struct{}),
		logger:   logger,
		timeout:  cfg.Timeout,
		signals:  cfg.Signals,
	}

	return sh
}

// OnShutdown registers a function to be called during shutdown
// Functions are called in LIFO order (last registered, first called)
func (sh *ShutdownHandler) OnShutdown(fn ShutdownFunc) {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.shutdownFuncs = append(sh.shutdownFuncs, fn)
}

// Context returns the context that will be cancelled during shutdown
func (sh *ShutdownHandler) Context() context.Context {
	return sh.ctx
}

// Done returns a channel that will be closed when shutdown is complete
func (sh *ShutdownHandler) Done() <-chan struct{} {
	return sh.done
}

// IsShuttingDown returns true if shutdown is in progress
func (sh *ShutdownHandler) IsShuttingDown() bool {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	return sh.isShuttingDown
}

// Listen starts listening for shutdown signals
// This should be called in a goroutine
func (sh *ShutdownHandler) Listen() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, sh.signals...)

	go func() {
		sig := <-sigChan
		sh.logger.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
		sh.Shutdown()
	}()

	sh.logger.Info().Msg("Shutdown handler is listening for signals")
}

// Shutdown initiates a graceful shutdown
func (sh *ShutdownHandler) Shutdown() error {
	sh.mu.Lock()
	if sh.isShuttingDown {
		sh.mu.Unlock()
		return fmt.Errorf("shutdown already in progress")
	}
	sh.isShuttingDown = true
	sh.mu.Unlock()

	sh.logger.Info().Msg("Starting graceful shutdown...")

	// Cancel the context to signal all goroutines to stop
	sh.cancel()

	// Create a context with timeout for shutdown functions
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), sh.timeout)
	defer shutdownCancel()

	// Call shutdown functions in LIFO order
	sh.mu.Lock()
	funcs := make([]ShutdownFunc, len(sh.shutdownFuncs))
	copy(funcs, sh.shutdownFuncs)
	sh.mu.Unlock()

	var lastErr error
	for i := len(funcs) - 1; i >= 0; i-- {
		fn := funcs[i]
		if err := fn(shutdownCtx); err != nil {
			sh.logger.Error().Err(err).Msg("Shutdown function failed")
			lastErr = err
		}
	}

	// Close the done channel to signal shutdown is complete
	close(sh.done)

	if lastErr != nil {
		sh.logger.Error().Err(lastErr).Msg("Graceful shutdown completed with errors")
	} else {
		sh.logger.Info().Msg("Graceful shutdown completed successfully")
	}

	return lastErr
}

// ShutdownWithTimeout initiates a graceful shutdown with a custom timeout
func (sh *ShutdownHandler) ShutdownWithTimeout(timeout time.Duration) error {
	sh.mu.Lock()
	oldTimeout := sh.timeout
	sh.timeout = timeout
	sh.mu.Unlock()

	defer func() {
		sh.mu.Lock()
		sh.timeout = oldTimeout
		sh.mu.Unlock()
	}()

	return sh.Shutdown()
}

// WaitForShutdown waits for shutdown to complete or timeout
func (sh *ShutdownHandler) WaitForShutdown(timeout time.Duration) error {
	select {
	case <-sh.done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("shutdown timed out after %v", timeout)
	}
}

// ServerShutdownFunc creates a shutdown function for an HTTP server
func ServerShutdownFunc(server *http.Server) ShutdownFunc {
	return func(ctx context.Context) error {
		if server == nil {
			return nil
		}
		return server.Shutdown(ctx)
	}
}

// CloseFunc creates a shutdown function for a simple Close() operation
func CloseFunc(closer interface{ Close() error }) ShutdownFunc {
	return func(ctx context.Context) error {
		if closer == nil {
			return nil
		}
		return closer.Close()
	}
}

// ContextCloseFunc creates a shutdown function for a CloseWithContext operation
func ContextCloseFunc(closer interface{ CloseWithContext(context.Context) error }) ShutdownFunc {
	return func(ctx context.Context) error {
		if closer == nil {
			return nil
		}
		return closer.CloseWithContext(ctx)
	}
}

// SetupSignalHandler sets up a simple signal handler and returns a context
// This is a simpler alternative to ShutdownHandler for basic use cases
func SetupSignalHandler() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		fmt.Printf("Received signal: %v, initiating shutdown...\n", sig)
		cancel()
	}()

	return ctx, cancel
}

// WithShutdownTimeout wraps a context with a shutdown timeout
func WithShutdownTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}

// RunWithShutdown runs a function and handles graceful shutdown
func RunWithShutdown(runFunc func(ctx context.Context) error, cfg *ShutdownConfig) error {
	sh := NewShutdownHandler(cfg)
	sh.Listen()

	errChan := make(chan error, 1)

	go func() {
		err := runFunc(sh.Context())
		errChan <- err
	}()

	select {
	case err := <-errChan:
		if err != nil {
			return err
		}
		// Function completed normally, initiate shutdown
		return sh.Shutdown()
	case <-sh.Done():
		// Shutdown signal received, wait for function to complete
		select {
		case err := <-errChan:
			return err
		case <-time.After(cfg.Timeout):
			return fmt.Errorf("shutdown timed out")
		}
	}
}

// ShutdownGroup manages a group of shutdown handlers
type ShutdownGroup struct {
	mu       sync.Mutex
	handlers []*ShutdownHandler
}

// NewShutdownGroup creates a new shutdown group
func NewShutdownGroup() *ShutdownGroup {
	return &ShutdownGroup{
		handlers: make([]*ShutdownHandler, 0),
	}
}

// Add adds a shutdown handler to the group
func (sg *ShutdownGroup) Add(sh *ShutdownHandler) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	sg.handlers = append(sg.handlers, sh)
}

// ShutdownAll shuts down all handlers in the group
func (sg *ShutdownGroup) ShutdownAll() error {
	sg.mu.Lock()
	handlers := make([]*ShutdownHandler, len(sg.handlers))
	copy(handlers, sg.handlers)
	sg.mu.Unlock()

	var lastErr error
	for _, sh := range handlers {
		if err := sh.Shutdown(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}
