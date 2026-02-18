package main

import (
	"context"
	"net/http"
	"os"
	"syscall"
	"time"
	"xcore-example/internal/config"
	"xcore-example/internal/models"
	"xcore-example/internal/router"
	"xcore-example/pkg/xcore"
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		panic(err)
	}
	logger := xcore.InitializeLogger(&cfg.Logger)

	db, err := xcore.InitializeDatabase(&cfg.Database, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize database")
	}
	logger.Info().Msg("Database initialized")

	// Run auto migration for demo models
	if err := db.AutoMigrate(&models.User{}, &models.Wallet{}, &models.Transaction{}); err != nil {
		logger.Fatal().Err(err).Msg("Failed to migrate database")
	}
	logger.Info().Msg("Database migrated")

	shutdownHandler := xcore.NewShutdownHandler(&xcore.ShutdownConfig{
		Timeout: 30 * time.Second,
		Logger:  logger,
		Signals: []os.Signal{syscall.SIGINT, syscall.SIGTERM},
	})

	shutdownHandler.OnShutdown(func(ctx context.Context) error {
		logger.Info().Msg("Closing database connection...")
		return xcore.CloseDatabase(db, logger)
	})

	shutdownHandler.Listen()
	logger.Info().Msg("Shutdown handler initialized")

	r := xcore.NewRouter(&cfg.Server, logger)
	router.RegisterRoutes(cfg, r, db, logger)

	shutdownHandler.OnShutdown(xcore.ServerShutdownFunc(r.Server))

	logger.Info().
		Int("port", cfg.Server.Port).
		Str("mode", cfg.Server.Mode).
		Msg("Server starting")

	// Start server in goroutine
	go func() {
		if err := r.Start(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	<-shutdownHandler.Done()
}
