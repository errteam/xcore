package xcore

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// MetricsCollector collects and exposes HTTP metrics
type MetricsCollector interface {
	// RecordRequest records a single request
	RecordRequest(method, path, status string, duration time.Duration)
	// RecordError records an error
	RecordError(method, path, errorType string)
	// GetMetrics returns current metrics
	GetMetrics() *Metrics
	// Reset resets all metrics
	Reset()
}

// Metrics holds all collected metrics
type Metrics struct {
	mu sync.RWMutex

	// Request counts
	TotalRequests    uint64            `json:"total_requests"`
	RequestsByMethod map[string]uint64 `json:"requests_by_method"`
	RequestsByPath   map[string]uint64 `json:"requests_by_path"`
	RequestsByStatus map[string]uint64 `json:"requests_by_status"`

	// Latency metrics (in milliseconds)
	LatencyTotal   float64   `json:"latency_total_ms"`
	LatencyMin     float64   `json:"latency_min_ms"`
	LatencyMax     float64   `json:"latency_max_ms"`
	LatencySamples []float64 `json:"-"` // Internal storage

	// Error counts
	TotalErrors  uint64            `json:"total_errors"`
	ErrorsByType map[string]uint64 `json:"errors_by_type"`

	// Active connections
	ActiveConnections int64 `json:"active_connections"`

	// Start time
	StartTime time.Time `json:"start_time"`
}

// InMemoryMetricsCollector is an in-memory implementation of MetricsCollector
type InMemoryMetricsCollector struct {
	metrics *Metrics
	mu      sync.RWMutex
}

// NewMetricsCollector creates a new in-memory metrics collector
func NewMetricsCollector() *InMemoryMetricsCollector {
	return &InMemoryMetricsCollector{
		metrics: &Metrics{
			RequestsByMethod: make(map[string]uint64),
			RequestsByPath:   make(map[string]uint64),
			RequestsByStatus: make(map[string]uint64),
			ErrorsByType:     make(map[string]uint64),
			LatencySamples:   make([]float64, 0, 1000),
			StartTime:        time.Now(),
		},
	}
}

// RecordRequest records a single request
func (c *InMemoryMetricsCollector) RecordRequest(method, path, status string, duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.metrics.TotalRequests++
	c.metrics.RequestsByMethod[method]++
	c.metrics.RequestsByPath[path]++
	c.metrics.RequestsByStatus[status]++

	// Record latency
	latencyMs := float64(duration.Nanoseconds()) / 1000000.0
	c.metrics.LatencyTotal += latencyMs

	if c.metrics.LatencyMin == 0 || latencyMs < c.metrics.LatencyMin {
		c.metrics.LatencyMin = latencyMs
	}
	if latencyMs > c.metrics.LatencyMax {
		c.metrics.LatencyMax = latencyMs
	}

	// Keep last 1000 samples for percentile calculation
	if len(c.metrics.LatencySamples) < 1000 {
		c.metrics.LatencySamples = append(c.metrics.LatencySamples, latencyMs)
	} else {
		c.metrics.LatencySamples = append(c.metrics.LatencySamples[1:], latencyMs)
	}
}

// RecordError records an error
func (c *InMemoryMetricsCollector) RecordError(method, path, errorType string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.metrics.TotalErrors++
	c.metrics.ErrorsByType[errorType]++
}

// GetMetrics returns current metrics
func (c *InMemoryMetricsCollector) GetMetrics() *Metrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Create a copy to avoid race conditions
	m := &Metrics{
		TotalRequests:     c.metrics.TotalRequests,
		RequestsByMethod:  make(map[string]uint64),
		RequestsByPath:    make(map[string]uint64),
		RequestsByStatus:  make(map[string]uint64),
		LatencyTotal:      c.metrics.LatencyTotal,
		LatencyMin:        c.metrics.LatencyMin,
		LatencyMax:        c.metrics.LatencyMax,
		TotalErrors:       c.metrics.TotalErrors,
		ErrorsByType:      make(map[string]uint64),
		ActiveConnections: c.metrics.ActiveConnections,
		StartTime:         c.metrics.StartTime,
	}

	for k, v := range c.metrics.RequestsByMethod {
		m.RequestsByMethod[k] = v
	}
	for k, v := range c.metrics.RequestsByPath {
		m.RequestsByPath[k] = v
	}
	for k, v := range c.metrics.RequestsByStatus {
		m.RequestsByStatus[k] = v
	}
	for k, v := range c.metrics.ErrorsByType {
		m.ErrorsByType[k] = v
	}

	// Calculate average latency
	if c.metrics.TotalRequests > 0 {
		m.LatencySamples = make([]float64, len(c.metrics.LatencySamples))
		copy(m.LatencySamples, c.metrics.LatencySamples)
	}

	return m
}

// Reset resets all metrics
func (c *InMemoryMetricsCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.metrics.TotalRequests = 0
	c.metrics.RequestsByMethod = make(map[string]uint64)
	c.metrics.RequestsByPath = make(map[string]uint64)
	c.metrics.RequestsByStatus = make(map[string]uint64)
	c.metrics.LatencyTotal = 0
	c.metrics.LatencyMin = 0
	c.metrics.LatencyMax = 0
	c.metrics.LatencySamples = make([]float64, 0, 1000)
	c.metrics.TotalErrors = 0
	c.metrics.ErrorsByType = make(map[string]uint64)
	c.metrics.StartTime = time.Now()
}

// GetLatencyPercentile returns the latency at the given percentile (0-100)
func (m *Metrics) GetLatencyPercentile(p float64) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.LatencySamples) == 0 {
		return 0
	}

	// Sort samples (simple bubble sort for small arrays)
	samples := make([]float64, len(m.LatencySamples))
	copy(samples, m.LatencySamples)
	for i := 0; i < len(samples)-1; i++ {
		for j := 0; j < len(samples)-i-1; j++ {
			if samples[j] > samples[j+1] {
				samples[j], samples[j+1] = samples[j+1], samples[j]
			}
		}
	}

	// Calculate percentile index
	index := int(float64(len(samples)) * p / 100.0)
	if index >= len(samples) {
		index = len(samples) - 1
	}

	return samples[index]
}

// GetAverageLatency returns the average latency in milliseconds
func (m *Metrics) GetAverageLatency() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.TotalRequests == 0 {
		return 0
	}
	return m.LatencyTotal / float64(m.TotalRequests)
}

// GetUptime returns the uptime since metrics collection started
func (m *Metrics) GetUptime() time.Duration {
	return time.Since(m.StartTime)
}

// GetRequestsPerSecond returns the average requests per second
func (m *Metrics) GetRequestsPerSecond() float64 {
	uptime := m.GetUptime().Seconds()
	if uptime == 0 {
		return 0
	}
	return float64(m.TotalRequests) / uptime
}

// MetricsMiddleware creates a middleware that collects HTTP metrics
func MetricsMiddleware(collector MetricsCollector) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Execute request
			next.ServeHTTP(rw, r)

			// Record metrics
			duration := time.Since(start)
			path := normalizePath(r.URL.Path)
			status := strconv.Itoa(rw.statusCode)

			collector.RecordRequest(r.Method, path, status, duration)

			// Record errors for 4xx and 5xx
			if rw.statusCode >= 400 {
				errorType := "client_error"
				if rw.statusCode >= 500 {
					errorType = "server_error"
				}
				collector.RecordError(r.Method, path, errorType)
			}
		})
	}
}

// normalizePath normalizes URL paths for metrics (removes IDs)
// /users/123 -> /users/{id}
// /api/v1/orders/456/items -> /api/v1/orders/{id}/items
func normalizePath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		// Check if part is numeric (likely an ID)
		if _, err := strconv.ParseUint(part, 10, 64); err == nil && part != "" {
			parts[i] = "{id}"
		}
	}
	return strings.Join(parts, "/")
}

// PrometheusMetricsHandler returns a handler that exports metrics in Prometheus format
func PrometheusMetricsHandler(collector MetricsCollector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics := collector.GetMetrics()

		var sb strings.Builder
		sb.WriteString("# HELP http_requests_total Total number of HTTP requests\n")
		sb.WriteString("# TYPE http_requests_total counter\n")
		sb.WriteString("http_requests_total " + strconv.FormatUint(metrics.TotalRequests, 10) + "\n")

		sb.WriteString("# HELP http_request_duration_seconds HTTP request latency in seconds\n")
		sb.WriteString("# TYPE http_request_duration_seconds summary\n")
		sb.WriteString("http_request_duration_seconds_sum " + strconv.FormatFloat(metrics.LatencyTotal/1000.0, 'f', 6, 64) + "\n")
		sb.WriteString("http_request_duration_seconds_count " + strconv.FormatUint(metrics.TotalRequests, 10) + "\n")

		sb.WriteString("# HELP http_errors_total Total number of HTTP errors\n")
		sb.WriteString("# TYPE http_errors_total counter\n")
		sb.WriteString("http_errors_total " + strconv.FormatUint(metrics.TotalErrors, 10) + "\n")

		sb.WriteString("# HELP http_active_connections Current number of active connections\n")
		sb.WriteString("# TYPE http_active_connections gauge\n")
		sb.WriteString("http_active_connections " + strconv.FormatInt(metrics.ActiveConnections, 10) + "\n")

		sb.WriteString("# HELP http_uptime_seconds Service uptime in seconds\n")
		sb.WriteString("# TYPE http_uptime_seconds gauge\n")
		sb.WriteString("http_uptime_seconds " + strconv.FormatFloat(metrics.GetUptime().Seconds(), 'f', 2, 64) + "\n")

		sb.WriteString("# HELP http_requests_per_second Average requests per second\n")
		sb.WriteString("# TYPE http_requests_per_second gauge\n")
		sb.WriteString("http_requests_per_second " + strconv.FormatFloat(metrics.GetRequestsPerSecond(), 'f', 2, 64) + "\n")

		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.Write([]byte(sb.String()))
	}
}

// MetricsHandler returns a handler that exports metrics in JSON format
func MetricsHandler(collector MetricsCollector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics := collector.GetMetrics()

		response := map[string]interface{}{
			"total_requests":      metrics.TotalRequests,
			"total_errors":        metrics.TotalErrors,
			"active_connections":  metrics.ActiveConnections,
			"uptime":              metrics.GetUptime().String(),
			"requests_per_second": metrics.GetRequestsPerSecond(),
			"latency": map[string]interface{}{
				"avg_ms": metrics.GetAverageLatency(),
				"min_ms": metrics.LatencyMin,
				"max_ms": metrics.LatencyMax,
				"p50_ms": metrics.GetLatencyPercentile(50),
				"p95_ms": metrics.GetLatencyPercentile(95),
				"p99_ms": metrics.GetLatencyPercentile(99),
			},
			"requests_by_method": metrics.RequestsByMethod,
			"requests_by_status": metrics.RequestsByStatus,
			"errors_by_type":     metrics.ErrorsByType,
		}

		rb := NewResponseBuilder(w, r)
		rb.JSON(http.StatusOK, Response{
			Code:    "SUCCESS",
			Message: "Metrics collected successfully",
			Data:    response,
			Metadata: Metadata{
				RequestID: GetRequestID(r),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}

// RegisterMetricsRoutes registers metrics routes on the router
func RegisterMetricsRoutes(r *mux.Router, collector MetricsCollector) {
	r.HandleFunc("/metrics", PrometheusMetricsHandler(collector)).Methods(http.MethodGet)
	r.HandleFunc("/metrics/json", MetricsHandler(collector)).Methods(http.MethodGet)
}
