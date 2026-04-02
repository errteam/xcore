// Package xcore provides health check functionality.
//
// This package provides health check endpoints for monitoring
// the status of the application and its dependencies.
package xcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HealthStatus represents the overall health status of the application.
type HealthStatus struct {
	Status     string                     `json:"status"`
	Timestamp  time.Time                  `json:"timestamp"`
	Components map[string]ComponentHealth `json:"components,omitempty"`
}

// ComponentHealth represents the health status of a specific component.
type ComponentHealth struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type HealthChecker interface {
	Health() ComponentHealth
}

func HealthMiddleware(app *App) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" || r.URL.Path == "/healthz" {
				handleHealth(w, app)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func handleHealth(w http.ResponseWriter, app *App) {
	status := HealthStatus{
		Status:     "ok",
		Timestamp:  time.Now(),
		Components: make(map[string]ComponentHealth),
	}

	if app.database != nil {
		sqlDB, err := app.database.Database()
		if err != nil {
			status.Components["database"] = ComponentHealth{
				Status:  "unhealthy",
				Message: err.Error(),
			}
			status.Status = "degraded"
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := sqlDB.PingContext(ctx); err != nil {
				status.Components["database"] = ComponentHealth{
					Status:  "unhealthy",
					Message: err.Error(),
				}
				status.Status = "degraded"
			} else {
				status.Components["database"] = ComponentHealth{
					Status: "healthy",
				}
			}
		}
	}

	if app.cache != nil {
		status.Components["cache"] = ComponentHealth{
			Status: "healthy",
		}
	}

	if app.websocket != nil {
		hub := app.websocket.Hub()
		if hub != nil {
			hub.mu.RLock()
			connCount := len(hub.connections)
			hub.mu.RUnlock()
			status.Components["websocket"] = ComponentHealth{
				Status:  "healthy",
				Message: fmt.Sprintf("%d active connections", connCount),
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(status)
}
