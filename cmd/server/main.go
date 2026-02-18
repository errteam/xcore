package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"xcore-example/pkg/xcore"
)

func main() {
	// ===========================================
	// 1. Initialize Logger
	// ===========================================
	loggerCfg := &xcore.LoggerConfig{
		Level:  "debug",
		Format: "console",
		Output: []string{"stdout"},
		File: xcore.FileLogConfig{
			Enabled:    true,
			Path:       "logs/app.log",
			MaxSize:    50,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   false,
		},
		Console: xcore.ConsoleConfig{
			Color:      true,
			TimeFormat: time.RFC3339,
		},
		Caller: xcore.CallerConfig{
			Enabled:    true,
			SkipFrames: 2,
		},
	}

	logger := xcore.InitializeLogger(loggerCfg)
	logger.Info().Msg("Starting server...")

	// ===========================================
	// 2. Initialize Cache (Memory)
	// ===========================================
	cache := xcore.NewMemoryCache(&xcore.MemoryCacheConfig{
		MaxSize:         10000,
		CleanupInterval: time.Minute,
	})
	logger.Info().Msg("Cache initialized (memory)")

	// ===========================================
	// 3. Initialize Database (SQLite for demo)
	// ===========================================
	dbCfg := &xcore.DatabaseConfig{
		Driver:               "sqlite",
		DSN:                  "demo.db",
		MaxIdleConns:         10,
		MaxOpenConns:         100,
		ConnMaxLifetime:      "1h",
		LogLevel:             "info",
		SlowThreshold:        "200ms",
		IgnoreRecordNotFound: true,
	}

	db, err := xcore.InitializeDatabase(dbCfg, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize database")
	}
	logger.Info().Msg("Database initialized (SQLite)")

	// Run auto migration for demo models
	if err := db.AutoMigrate(&User{}, &Product{}); err != nil {
		logger.Fatal().Err(err).Msg("Failed to migrate database")
	}
	logger.Info().Msg("Database migrated")

	// ===========================================
	// 4. Initialize Health Handler
	// ===========================================
	healthHandler := xcore.NewHealthHandler(&xcore.HealthConfig{
		ServiceName:    "crud-api-demo",
		ServiceVersion: "1.0.0",
	})

	// Add database health checker
	sqlDB, _ := db.DB()
	dbChecker := xcore.NewDBHealthChecker(&xcore.DBHealthCheckerConfig{
		Name:  "database",
		DB:    sqlDB,
		Ping:  true,
		Query: "SELECT 1",
	})
	healthHandler.AddChecker(dbChecker)

	// Add cache health checker (custom)
	cacheChecker := &cacheHealthChecker{cache: cache}
	healthHandler.AddChecker(cacheChecker)

	logger.Info().Msg("Health check initialized")

	// ===========================================
	// 5. Initialize Metrics Collector
	// ===========================================
	metricsCollector := xcore.NewMetricsCollector()
	logger.Info().Msg("Metrics collector initialized")

	// ===========================================
	// 6. Initialize WebSocket Hub
	// ===========================================
	wsCfg := xcore.DefaultWebSocketConfig()
	wsCfg.PingInterval = 30 * time.Second
	wsCfg.MaxConnections = 1000

	wsHub := xcore.NewWSHub(wsCfg, logger)

	// Set WebSocket event handlers
	wsHub.SetAuthFunc(func(r *http.Request) (string, interface{}, error) {
		// Simple auth: get user_id from query param
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			userID = "anonymous"
		}
		return userID, map[string]interface{}{"user_id": userID, "connected_at": time.Now()}, nil
	})

	wsHub.SetOnConnectFunc(func(conn *xcore.WSConnection) {
		logger.Info().Str("conn_id", conn.ID).Msg("WebSocket client connected")
		// Send welcome message
		conn.SendJSON(xcore.NewWSResponse(true, "Connected to WebSocket", map[string]string{
			"connection_id": conn.ID,
			"timestamp":     time.Now().Format(time.RFC3339),
		}))
	})

	wsHub.SetOnMessageFunc(func(conn *xcore.WSConnection, msg *xcore.WSMessage) {
		logger.Debug().Str("conn_id", conn.ID).Int("type", msg.Type).Msg("WebSocket message received")

		// Echo message back
		conn.SendJSON(xcore.NewWSResponse(true, "Message received", map[string]interface{}{
			"echo":      string(msg.Data),
			"timestamp": time.Now().Format(time.RFC3339),
		}))
	})

	wsHub.SetOnDisconnectFunc(func(conn *xcore.WSConnection, code int, reason string) {
		logger.Info().Str("conn_id", conn.ID).Int("code", code).Str("reason", reason).Msg("WebSocket client disconnected")
	})

	// Start WebSocket hub
	go wsHub.Run()
	logger.Info().Msg("WebSocket hub started")

	// ===========================================
	// 7. Initialize Graceful Shutdown
	// ===========================================
	shutdownHandler := xcore.NewShutdownHandler(&xcore.ShutdownConfig{
		Timeout: 30 * time.Second,
		Logger:  logger,
		Signals: []os.Signal{syscall.SIGINT, syscall.SIGTERM},
	})

	// Register shutdown functions
	shutdownHandler.OnShutdown(func(ctx context.Context) error {
		logger.Info().Msg("Shutting down WebSocket hub...")
		wsHub.Shutdown()
		return nil
	})

	shutdownHandler.OnShutdown(func(ctx context.Context) error {
		logger.Info().Msg("Closing database connection...")
		return xcore.CloseDatabase(db, logger)
	})

	shutdownHandler.OnShutdown(func(ctx context.Context) error {
		logger.Info().Msg("Closing cache...")
		return cache.Close()
	})

	// Start listening for shutdown signals
	shutdownHandler.Listen()
	logger.Info().Msg("Shutdown handler initialized")

	// ===========================================
	// 8. Create Router with Middlewares
	// ===========================================
	serverCfg := &xcore.ServerConfig{
		Port:         8080,
		Mode:         "debug",
		ReadTimeout:  "30s",
		WriteTimeout: "30s",
		IdleTimeout:  "60s",
		CORS: xcore.CORSConfig{
			Enabled:      true,
			AllowOrigins: []string{"*"},
			AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID"},
		},
		RateLimit: xcore.RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: 100,
			Burst:             200,
			Prefix:            "api",
		},
		Security: xcore.SecurityConfig{
			Enabled:                 true,
			XFrameOptions:           "DENY",
			XContentTypeOptions:     "nosniff",
			XXSSProtection:          "1; mode=block",
			StrictTransportSecurity: "max-age=31536000; includeSubDomains",
		},
	}

	router := xcore.NewRouter(serverCfg, logger)

	// Add metrics middleware
	router.Router.Use(xcore.MetricsMiddleware(metricsCollector))

	// Add logger to context middleware
	router.Router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := xcore.ContextWithLogger(r.Context(), logger)
			ctx = xcore.ContextWithLogger(ctx, logger)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	// ===========================================
	// 9. Register Routes
	// ===========================================
	registerRoutes(router.Router, logger, cache, db, wsHub, healthHandler, metricsCollector)

	// Register health routes
	xcore.RegisterHealthRoutes(router.Router, healthHandler)

	// Register metrics routes
	xcore.RegisterMetricsRoutes(router.Router, metricsCollector)

	// ===========================================
	// 10. Register Server Shutdown
	// ===========================================
	shutdownHandler.OnShutdown(xcore.ServerShutdownFunc(router.Server))

	// ===========================================
	// 11. Start Server
	// ===========================================
	logger.Info().
		Int("port", serverCfg.Port).
		Str("mode", serverCfg.Mode).
		Msg("Server starting")

	// Start server in goroutine
	go func() {
		if err := router.Start(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	// Print startup info
	printStartupInfo(logger, serverCfg.Port)

	// ===========================================
	// 12. Wait for Shutdown Signal
	// ===========================================
	<-shutdownHandler.Done()

	// Give server time to shutdown
	time.Sleep(2 * time.Second)

	logger.Info().Msg("Server stopped")
}

// registerRoutes registers all application routes
func registerRoutes(
	r *mux.Router,
	logger *zerolog.Logger,
	cache xcore.Cache,
	db interface{},
	wsHub *xcore.WSHub,
	healthHandler *xcore.HealthHandler,
	metrics *xcore.InMemoryMetricsCollector,
) {
	// API routes
	api := r.PathPrefix("/api").Subrouter()

	// Demo CRUD endpoints
	api.HandleFunc("/users", getUsersHandler(logger, db, cache)).Methods(http.MethodGet).Name("admin.user.list")
	api.HandleFunc("/users", createUserHandler(logger, db, cache)).Methods(http.MethodPost)
	api.HandleFunc("/users/{id}", getUserHandler(logger, db, cache)).Methods(http.MethodGet)
	api.HandleFunc("/users/{id}", updateUserHandler(logger, db, cache)).Methods(http.MethodPut)
	api.HandleFunc("/users/{id}", deleteUserHandler(logger, db, cache)).Methods(http.MethodDelete)

	api.HandleFunc("/products", getProductsHandler(logger, db, cache)).Methods(http.MethodGet)
	api.HandleFunc("/products", createProductHandler(logger, db, cache)).Methods(http.MethodPost)

	// Cache demo endpoints
	api.HandleFunc("/cache/set", cacheSetHandler(logger, cache)).Methods(http.MethodPost)
	api.HandleFunc("/cache/get/{key}", cacheGetHandler(logger, cache)).Methods(http.MethodGet)
	api.HandleFunc("/cache/delete/{key}", cacheDeleteHandler(logger, cache)).Methods(http.MethodDelete)

	// WebSocket endpoint
	r.HandleFunc("/ws", wsHub.WSHandler())

	// Demo endpoints for testing xcore features
	api.HandleFunc("/demo/request-info", requestInfoHandler(logger)).Methods(http.MethodGet)
	api.HandleFunc("/demo/slow", slowHandler(logger)).Methods(http.MethodGet)
	api.HandleFunc("/demo/error", errorHandler(logger)).Methods(http.MethodGet)
	api.HandleFunc("/demo/broadcast", broadcastHandler(logger, wsHub)).Methods(http.MethodPost)

	// Metrics info endpoint
	api.HandleFunc("/metrics/summary", metricsSummaryHandler(metrics)).Methods(http.MethodGet)
}

// ===========================================
// Demo Models
// ===========================================

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	Email     string    `gorm:"size:100;uniqueIndex" json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Product struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:200;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Price       float64   `gorm:"type:decimal(10,2)" json:"price"`
	Stock       int       `json:"stock"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ===========================================
// Handler Helpers
// ===========================================

func requestInfoHandler(logger *zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := xcore.GetRequestID(r)
		clientIP := xcore.GetClientIP(r)
		userAgent := xcore.GetUserAgent(r)

		rb := xcore.NewResponseBuilder(w, r)
		rb.OK(map[string]interface{}{
			"request_id":   requestID,
			"client_ip":    clientIP,
			"user_agent":   userAgent,
			"method":       r.Method,
			"path":         r.URL.Path,
			"query":        r.URL.RawQuery,
			"content_type": r.Header.Get("Content-Type"),
			"timestamp":    time.Now().UTC().Format(time.RFC3339),
		})

		logger.Debug().
			Str("request_id", requestID).
			Str("path", r.URL.Path).
			Msg("Request info")
	}
}

func slowHandler(logger *zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Simulate slow operation
		time.Sleep(2 * time.Second)

		duration := time.Since(start)

		rb := xcore.NewResponseBuilder(w, r)
		rb.OK(map[string]interface{}{
			"message":  "Slow operation completed",
			"duration": xcore.FormatDuration(duration),
		})

		logger.Warn().
			Dur("duration", duration).
			Msg("Slow request completed")
	}
}

func errorHandler(logger *zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rb := xcore.NewResponseBuilder(w, r)
		rb.InternalServerError("This is a demo error")

		logger.Error().Msg("Demo error triggered")
	}
}

func broadcastHandler(logger *zerolog.Logger, wsHub *xcore.WSHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Message string `json:"message"`
		}

		if err := xcore.ParseJSON(r, &req); err != nil {
			rb := xcore.NewResponseBuilder(w, r)
			rb.BadRequest("Invalid JSON")
			return
		}

		// Broadcast to all WebSocket clients
		wsHub.BroadcastJSON(xcore.NewWSResponse(true, "Broadcast message", map[string]string{
			"message":   req.Message,
			"timestamp": time.Now().Format(time.RFC3339),
		}))

		rb := xcore.NewResponseBuilder(w, r)
		rb.OK(map[string]interface{}{
			"message":           "Broadcast sent",
			"connected_clients": wsHub.GetConnectionCount(),
		})

		logger.Info().
			Int("clients", wsHub.GetConnectionCount()).
			Msg("Broadcast sent")
	}
}

func metricsSummaryHandler(metrics *xcore.InMemoryMetricsCollector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := metrics.GetMetrics()

		rb := xcore.NewResponseBuilder(w, r)
		rb.OK(map[string]interface{}{
			"total_requests":      m.TotalRequests,
			"total_errors":        m.TotalErrors,
			"uptime":              m.GetUptime().String(),
			"requests_per_second": m.GetRequestsPerSecond(),
			"latency": map[string]interface{}{
				"avg_ms": m.GetAverageLatency(),
				"min_ms": m.LatencyMin,
				"max_ms": m.LatencyMax,
				"p50_ms": m.GetLatencyPercentile(50),
				"p95_ms": m.GetLatencyPercentile(95),
				"p99_ms": m.GetLatencyPercentile(99),
			},
		})
	}
}

// ===========================================
// CRUD Handlers (Demo)
// ===========================================

func getUsersHandler(logger *zerolog.Logger, db interface{}, cache xcore.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var users []User
		db.(*gorm.DB).Find(&users)

		rb := xcore.NewResponseBuilder(w, r)
		rb.OK(users)

		logger.Info().Int("count", len(users)).Msg("Users retrieved")
	}
}

func getUserHandler(logger *zerolog.Logger, db interface{}, cache xcore.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := xcore.GetParam(r, "id")

		var user User
		if err := db.(*gorm.DB).First(&user, id).Error; err != nil {
			rb := xcore.NewResponseBuilder(w, r)
			rb.NotFound("User not found")
			return
		}

		rb := xcore.NewResponseBuilder(w, r)
		rb.OK(user)
	}
}

func createUserHandler(logger *zerolog.Logger, db interface{}, cache xcore.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var user User
		if err := xcore.ParseJSON(r, &user); err != nil {
			rb := xcore.NewResponseBuilder(w, r)
			rb.BadRequest("Invalid request body")
			return
		}

		if err := db.(*gorm.DB).Create(&user).Error; err != nil {
			rb := xcore.NewResponseBuilder(w, r)
			rb.InternalServerError("Failed to create user")
			return
		}

		rb := xcore.NewResponseBuilder(w, r)
		rb.Created("User created successfully", user)

		logger.Info().Uint("id", user.ID).Str("name", user.Name).Msg("User created")
	}
}

func updateUserHandler(logger *zerolog.Logger, db interface{}, cache xcore.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := xcore.GetParam(r, "id")

		var user User
		if err := db.(*gorm.DB).First(&user, id).Error; err != nil {
			rb := xcore.NewResponseBuilder(w, r)
			rb.NotFound("User not found")
			return
		}

		var req User
		if err := xcore.ParseJSON(r, &req); err != nil {
			rb := xcore.NewResponseBuilder(w, r)
			rb.BadRequest("Invalid request body")
			return
		}

		user.Name = req.Name
		user.Email = req.Email

		if err := db.(*gorm.DB).Save(&user).Error; err != nil {
			rb := xcore.NewResponseBuilder(w, r)
			rb.InternalServerError("Failed to update user")
			return
		}

		rb := xcore.NewResponseBuilder(w, r)
		rb.OK(user)
	}
}

func deleteUserHandler(logger *zerolog.Logger, db interface{}, cache xcore.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := xcore.GetParam(r, "id")

		if err := db.(*gorm.DB).Delete(&User{}, id).Error; err != nil {
			rb := xcore.NewResponseBuilder(w, r)
			rb.InternalServerError("Failed to delete user")
			return
		}

		rb := xcore.NewResponseBuilder(w, r)
		rb.Deleted("User deleted successfully")
	}
}

func getProductsHandler(logger *zerolog.Logger, db interface{}, cache xcore.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var products []Product
		db.(*gorm.DB).Find(&products)

		rb := xcore.NewResponseBuilder(w, r)
		rb.OK(products)
	}
}

func createProductHandler(logger *zerolog.Logger, db interface{}, cache xcore.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var product Product
		if err := xcore.ParseJSON(r, &product); err != nil {
			rb := xcore.NewResponseBuilder(w, r)
			rb.BadRequest("Invalid request body")
			return
		}

		if err := db.(*gorm.DB).Create(&product).Error; err != nil {
			rb := xcore.NewResponseBuilder(w, r)
			rb.InternalServerError("Failed to create product")
			return
		}

		rb := xcore.NewResponseBuilder(w, r)
		rb.Created("Product created successfully", product)
	}
}

// ===========================================
// Cache Handlers
// ===========================================

func cacheSetHandler(logger *zerolog.Logger, cache xcore.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Key   string      `json:"key" validate:"required"`
			Value interface{} `json:"value"`
			TTL   string      `json:"ttl"`
		}

		if err := xcore.ParseJSON(r, &req); err != nil {
			rb := xcore.NewResponseBuilder(w, r)
			rb.BadRequest("Invalid request body")
			return
		}

		ttl := 10 * time.Minute
		if req.TTL != "" {
			if d, err := time.ParseDuration(req.TTL); err == nil {
				ttl = d
			}
		}

		if err := cache.Set(r.Context(), req.Key, req.Value, ttl); err != nil {
			rb := xcore.NewResponseBuilder(w, r)
			rb.InternalServerError("Failed to set cache")
			return
		}

		rb := xcore.NewResponseBuilder(w, r)
		rb.OK(map[string]string{
			"message": "Cache set successfully",
			"key":     req.Key,
			"ttl":     ttl.String(),
		})
	}
}

func cacheGetHandler(logger *zerolog.Logger, cache xcore.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := xcore.GetParam(r, "key")

		value, exists, err := cache.Get(r.Context(), key)
		if err != nil {
			rb := xcore.NewResponseBuilder(w, r)
			rb.InternalServerError("Failed to get cache")
			return
		}

		if !exists {
			rb := xcore.NewResponseBuilder(w, r)
			rb.NotFound("Cache key not found")
			return
		}

		rb := xcore.NewResponseBuilder(w, r)
		rb.OK(map[string]interface{}{
			"key":   key,
			"value": value,
		})
	}
}

func cacheDeleteHandler(logger *zerolog.Logger, cache xcore.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := xcore.GetParam(r, "key")

		if err := cache.Delete(r.Context(), key); err != nil {
			rb := xcore.NewResponseBuilder(w, r)
			rb.InternalServerError("Failed to delete cache")
			return
		}

		rb := xcore.NewResponseBuilder(w, r)
		rb.OK(map[string]string{
			"message": "Cache deleted successfully",
			"key":     key,
		})
	}
}

// ===========================================
// Custom Health Checker
// ===========================================

type cacheHealthChecker struct {
	cache xcore.Cache
}

func (c *cacheHealthChecker) Name() string {
	return "cache"
}

func (c *cacheHealthChecker) Check(ctx context.Context) error {
	count, err := c.cache.Count(ctx)
	if err != nil {
		return err
	}
	_ = count // Use count for health check logic if needed
	return nil
}

// ===========================================
// Startup Info
// ===========================================

func printStartupInfo(logger *zerolog.Logger, port int) {
	logger.Info().Msg(`
╔═══════════════════════════════════════════════════════════╗
║                    Server Started                         ║
╠═══════════════════════════════════════════════════════════╣
║  HTTP Server:  http://localhost:` + fmt.Sprintf("%-6d", port) + `                   ║
║  Health Check: http://localhost:` + fmt.Sprintf("%-6d", port) + `/health                   ║
║  Metrics:      http://localhost:` + fmt.Sprintf("%-6d", port) + `/metrics                  ║
║  WebSocket:    ws://localhost:` + fmt.Sprintf("%-6d", port) + `/ws                       ║
╠═══════════════════════════════════════════════════════════╣
║  Demo Endpoints:                                          ║
║  - GET  /api/demo/request-info                            ║
║  - GET  /api/demo/slow                                    ║
║  - GET  /api/demo/error                                   ║
║  - POST /api/demo/broadcast                               ║
║  - GET  /api/users                                        ║
║  - POST /api/users                                        ║
║  - GET  /api/cache/set                                    ║
╚═══════════════════════════════════════════════════════════╝
`)
}
