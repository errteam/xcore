package xcore

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	// StatusHealthy indicates the component is healthy
	StatusHealthy HealthStatus = "healthy"
	// StatusUnhealthy indicates the component is unhealthy
	StatusUnhealthy HealthStatus = "unhealthy"
	// StatusDegraded indicates the component is partially working
	StatusDegraded HealthStatus = "degraded"
	// StatusUnknown indicates the component status is unknown
	StatusUnknown HealthStatus = "unknown"
)

// HealthCheck represents a health check result
type HealthCheck struct {
	Status    HealthStatus     `json:"status"`
	Timestamp time.Time        `json:"timestamp"`
	Checks    map[string]Check `json:"checks"`
}

// Check represents a single health check
type Check struct {
	Status  HealthStatus `json:"status"`
	Message string       `json:"message,omitempty"`
	Error   string       `json:"error,omitempty"`
}

// HealthChecker defines the interface for health checks
type HealthChecker interface {
	// Name returns the name of the health checker
	Name() string
	// Check performs the health check
	Check(ctx context.Context) error
}

// HealthHandler manages health checks
type HealthHandler struct {
	mu             sync.RWMutex
	checkers       []HealthChecker
	startTime      time.Time
	serviceName    string
	serviceVersion string
}

// HealthConfig holds configuration for health handler
type HealthConfig struct {
	ServiceName    string
	ServiceVersion string
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(cfg *HealthConfig) *HealthHandler {
	return &HealthHandler{
		startTime:      time.Now(),
		serviceName:    cfg.ServiceName,
		serviceVersion: cfg.ServiceVersion,
		checkers:       make([]HealthChecker, 0),
	}
}

// AddChecker adds a health checker to the handler
func (h *HealthHandler) AddChecker(checker HealthChecker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkers = append(h.checkers, checker)
}

// Check performs all health checks and returns the result
func (h *HealthHandler) Check(ctx context.Context) HealthCheck {
	h.mu.RLock()
	checkers := make([]HealthChecker, len(h.checkers))
	copy(checkers, h.checkers)
	h.mu.RUnlock()

	result := HealthCheck{
		Status:    StatusHealthy,
		Timestamp: time.Now(),
		Checks:    make(map[string]Check),
	}

	// Run system checks
	result.Checks["system"] = h.checkSystem()

	// Run uptime check
	result.Checks["uptime"] = h.checkUptime()

	// Run registered checkers
	for _, checker := range checkers {
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		err := checker.Check(checkCtx)
		check := Check{
			Status: StatusHealthy,
		}

		if err != nil {
			check.Status = StatusUnhealthy
			check.Error = err.Error()
			result.Status = StatusUnhealthy
		}

		result.Checks[checker.Name()] = check
	}

	return result
}

// checkSystem returns system health information
func (h *HealthHandler) checkSystem() Check {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return Check{
		Status: StatusHealthy,
		Message: fmt.Sprintf("goroutines=%d, cpu=%d, alloc_mb=%d, sys_mb=%d",
			runtime.NumGoroutine(),
			runtime.NumCPU(),
			m.Alloc/1024/1024,
			m.Sys/1024/1024),
	}
}

// checkUptime returns uptime information
func (h *HealthHandler) checkUptime() Check {
	uptime := time.Since(h.startTime)
	return Check{
		Status:  StatusHealthy,
		Message: uptime.String(),
	}
}

// ServeHTTP implements http.Handler for health endpoint
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	result := h.Check(ctx)

	status := http.StatusOK
	if result.Status == StatusUnhealthy {
		status = http.StatusServiceUnavailable
	}

	// Add service info
	response := map[string]interface{}{
		"status":    result.Status,
		"timestamp": result.Timestamp.Format(time.RFC3339),
		"service":   h.serviceName,
		"version":   h.serviceVersion,
		"checks":    result.Checks,
	}

	rb := NewResponseBuilder(w, r)
	rb.JSON(status, Response{
		Code:    string(result.Status),
		Message: string(result.Status),
		Data:    response,
		Metadata: Metadata{
			RequestID: GetRequestID(r),
			Timestamp: result.Timestamp.Format(time.RFC3339),
		},
	})
}

// DBHealthChecker is a health checker for database connections
type DBHealthChecker struct {
	db    *sql.DB
	name  string
	ping  bool
	query string
}

// DBHealthCheckerConfig holds configuration for DB health checker
type DBHealthCheckerConfig struct {
	Name  string
	DB    *sql.DB
	Ping  bool
	Query string // Optional query to run (e.g., "SELECT 1")
}

// NewDBHealthChecker creates a new database health checker
func NewDBHealthChecker(cfg *DBHealthCheckerConfig) *DBHealthChecker {
	name := cfg.Name
	if name == "" {
		name = "database"
	}
	return &DBHealthChecker{
		db:    cfg.DB,
		name:  name,
		ping:  cfg.Ping,
		query: cfg.Query,
	}
}

// Name returns the name of the checker
func (c *DBHealthChecker) Name() string {
	return c.name
}

// Check performs the database health check
func (c *DBHealthChecker) Check(ctx context.Context) error {
	if c.db == nil {
		return nil
	}

	if c.ping {
		if err := c.db.PingContext(ctx); err != nil {
			return err
		}
	}

	if c.query != "" {
		var result int
		if err := c.db.QueryRowContext(ctx, c.query).Scan(&result); err != nil {
			return err
		}
	}

	return nil
}

// GORMHealthChecker is a health checker for GORM database connections
type GORMHealthChecker struct {
	db    interface{} // *gorm.DB
	name  string
	ping  bool
	query string
}

// NewGORMHealthChecker creates a new GORM health checker
func NewGORMHealthChecker(db interface{}, name string, ping bool, query string) *GORMHealthChecker {
	if name == "" {
		name = "database"
	}
	return &GORMHealthChecker{
		db:    db,
		name:  name,
		ping:  ping,
		query: query,
	}
}

// Name returns the name of the checker
func (c *GORMHealthChecker) Name() string {
	return c.name
}

// Check performs the GORM database health check
func (c *GORMHealthChecker) Check(ctx context.Context) error {
	if c.db == nil {
		return nil
	}

	// Use reflection to call DB() and Ping()
	// This avoids importing gorm in this package
	type dbInterface interface {
		DB() (*sql.DB, error)
	}

	if db, ok := c.db.(dbInterface); ok {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		if c.ping {
			return sqlDB.PingContext(ctx)
		}
	}

	return nil
}

// RedisHealthChecker is a health checker for Redis connections
type RedisHealthChecker struct {
	client interface{} // *redis.Client or *redis.Client
	name   string
}

// NewRedisHealthChecker creates a new Redis health checker
func NewRedisHealthChecker(client interface{}, name string) *RedisHealthChecker {
	if name == "" {
		name = "redis"
	}
	return &RedisHealthChecker{
		client: client,
		name:   name,
	}
}

// Name returns the name of the checker
func (c *RedisHealthChecker) Name() string {
	return c.name
}

// Check performs the Redis health check
func (c *RedisHealthChecker) Check(ctx context.Context) error {
	if c.client == nil {
		return nil
	}

	// Use reflection to call Ping()
	type redisInterface interface {
		Ping(ctx context.Context) error
	}

	if redis, ok := c.client.(redisInterface); ok {
		return redis.Ping(ctx)
	}

	return nil
}

// RegisterHealthRoutes registers health check routes on the router
func RegisterHealthRoutes(r *mux.Router, handler *HealthHandler) {
	r.HandleFunc("/health", handler.ServeHTTP).Methods(http.MethodGet)
	r.HandleFunc("/health/live", handler.ServeHTTP).Methods(http.MethodGet)
	r.HandleFunc("/health/ready", handler.ServeHTTP).Methods(http.MethodGet)
}

// SimpleHealthHandler creates a simple health handler without custom checkers
func SimpleHealthHandler(serviceName, version string) http.HandlerFunc {
	startTime := time.Now()
	return func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"service":   serviceName,
			"version":   version,
			"uptime":    time.Since(startTime).String(),
		}

		rb := NewResponseBuilder(w, r)
		rb.JSON(http.StatusOK, Response{
			Code:    "HEALTHY",
			Message: "Service is healthy",
			Data:    response,
			Metadata: Metadata{
				RequestID: GetRequestID(r),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}
