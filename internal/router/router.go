package router

import (
	"xcore-example/internal/config"
	"xcore-example/internal/handler"
	"xcore-example/pkg/xcore"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func RegisterRoutes(cfg *config.Config, router *xcore.Router, db *gorm.DB, logger *zerolog.Logger) {
	healthHandler := xcore.NewHealthHandler(&xcore.HealthConfig{
		ServiceName:    "crud-api-demo",
		ServiceVersion: "1.0.0",
	})
	sqlDB, _ := db.DB()
	dbChecker := xcore.NewDBHealthChecker(&xcore.DBHealthCheckerConfig{
		Name:  "database",
		DB:    sqlDB,
		Ping:  true,
		Query: "SELECT 1",
	})
	healthHandler.AddChecker(dbChecker)

	metricsCollector := xcore.NewMetricsCollector()
	logger.Info().Msg("Metrics collector initialized")

	router.Use(xcore.MetricsMiddleware(metricsCollector))

	// Register health routes
	xcore.RegisterHealthRoutes(router.Router, healthHandler)

	// Register metrics routes
	xcore.RegisterMetricsRoutes(router.Router, metricsCollector)

	authHandler := &handler.AuthHandler{}

	authHandler.RegisterRoutes(router.Router)
}
