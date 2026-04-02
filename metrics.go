// Package xcore provides metrics collection functionality.
//
// This package provides basic metrics collection including counters and histograms.
// It can be extended to integrate with Prometheus or other monitoring systems.
package xcore

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// MetricsConfig defines configuration for metrics collection.
type MetricsConfig struct {
	Path             string
	EnableAPIMetrics bool
	EnableDBMetrics  bool
	Buckets          []float64
}

// NewMetricsConfig creates a default MetricsConfig.
func NewMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		Path:             "/metrics",
		EnableAPIMetrics: true,
		EnableDBMetrics:  true,
		Buckets:          []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}
}

var (
	httpRequestsTotal   *MetricCounter
	httpRequestDuration *MetricHistogram
	httpResponsesTotal  *MetricCounter
	activeRequests      *MetricGauge
	dbQueryDuration     *MetricHistogram
	dbQueryTotal        *MetricCounter
	cacheHits           *MetricCounter
	cacheMisses         *MetricCounter
	wsConnections       *MetricGauge
	wsMessagesTotal     *MetricCounter
)

type MetricCounter struct {
	mu     sync.RWMutex
	values map[string]uint64
	labels []string
}

type MetricGauge struct {
	mu     sync.RWMutex
	values map[string]int64
}

type MetricHistogram struct {
	mu      sync.RWMutex
	buckets map[string][]uint64
	count   map[string]uint64
	sum     map[string]float64
	bounds  []float64
}

func NewMetricCounter(name, help string, labels ...string) *MetricCounter {
	return &MetricCounter{
		values: make(map[string]uint64),
		labels: labels,
	}
}

func (c *MetricCounter) Inc(labels ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := strings.Join(labels, ",")
	c.values[key]++
}

func (c *MetricCounter) Add(n uint64, labels ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := strings.Join(labels, ",")
	c.values[key] += n
}

func (c *MetricCounter) GetValue(labels ...string) uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key := strings.Join(labels, ",")
	return c.values[key]
}

func (c *MetricCounter) String() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var sb strings.Builder
	sb.WriteString("# HELP\n")
	sb.WriteString("# TYPE\n")
	for key, val := range c.values {
		sb.WriteString(fmt.Sprintf("%s{labels=\"%s\"} %d\n", key, key, val))
	}
	return sb.String()
}

func NewMetricGauge(name, help string) *MetricGauge {
	return &MetricGauge{
		values: make(map[string]int64),
	}
}

func (g *MetricGauge) Inc(labels ...string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	key := strings.Join(labels, ",")
	g.values[key]++
}

func (g *MetricGauge) Dec(labels ...string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	key := strings.Join(labels, ",")
	g.values[key]--
}

func (g *MetricGauge) Set(val int64, labels ...string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	key := strings.Join(labels, ",")
	g.values[key] = val
}

func (g *MetricGauge) GetValue(labels ...string) int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	key := strings.Join(labels, ",")
	return g.values[key]
}

func NewMetricHistogram(name, help string, bounds []float64) *MetricHistogram {
	return &MetricHistogram{
		buckets: make(map[string][]uint64),
		count:   make(map[string]uint64),
		sum:     make(map[string]float64),
		bounds:  bounds,
	}
}

func (h *MetricHistogram) Observe(val float64, labels ...string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := strings.Join(labels, ",")
	h.count[key]++
	h.sum[key] += val
	for i, bound := range h.bounds {
		if val <= bound {
			h.buckets[key][i]++
		}
	}
}

func (h *MetricHistogram) GetCount(labels ...string) uint64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	key := strings.Join(labels, ",")
	return h.count[key]
}

func initMetrics() {
	httpRequestsTotal = NewMetricCounter("http_requests_total", "Total HTTP requests")
	httpRequestDuration = NewMetricHistogram("http_request_duration_seconds", "HTTP request duration", []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10})
	httpResponsesTotal = NewMetricCounter("http_responses_total", "Total HTTP responses")
	activeRequests = NewMetricGauge("http_active_requests", "Active HTTP requests")
	dbQueryDuration = NewMetricHistogram("db_query_duration_seconds", "Database query duration", []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1})
	dbQueryTotal = NewMetricCounter("db_queries_total", "Total database queries")
	cacheHits = NewMetricCounter("cache_hits_total", "Total cache hits")
	cacheMisses = NewMetricCounter("cache_misses_total", "Total cache misses")
	wsConnections = NewMetricGauge("ws_connections", "Active WebSocket connections")
	wsMessagesTotal = NewMetricCounter("ws_messages_total", "Total WebSocket messages")
}

type MetricsMiddleware struct {
	config *MetricsConfig
}

func NewMetricsMiddleware(cfg *MetricsConfig) *MetricsMiddleware {
	if cfg == nil {
		cfg = NewMetricsConfig()
	}
	if httpRequestsTotal == nil {
		initMetrics()
	}
	return &MetricsMiddleware{config: cfg}
}

func (m *MetricsMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == m.config.Path {
			m.serveMetrics(w, r)
			return
		}

		activeRequests.Inc()
		defer activeRequests.Dec()

		start := time.Now()

		wrapper := &metricsResponseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(wrapper, r)

		duration := time.Since(start).Seconds()

		httpRequestsTotal.Inc(r.Method, r.URL.Path, fmt.Sprintf("%d", wrapper.status))
		httpRequestDuration.Observe(duration, r.Method, r.URL.Path)
		httpResponsesTotal.Inc(r.Method, fmt.Sprintf("%d", wrapper.status))
	})
}

type metricsResponseWriter struct {
	http.ResponseWriter
	status int
}

func (m *metricsResponseWriter) WriteHeader(status int) {
	m.status = status
	m.ResponseWriter.WriteHeader(status)
}

func (m *MetricsMiddleware) serveMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	var sb strings.Builder

	sb.WriteString("# HELP http_requests_total Total number of HTTP requests\n")
	sb.WriteString("# TYPE http_requests_total counter\n")
	httpRequestsTotal.mu.RLock()
	for key, val := range httpRequestsTotal.values {
		sb.WriteString(fmt.Sprintf("http_requests_total{%s} %d\n", key, val))
	}
	httpRequestsTotal.mu.RUnlock()

	sb.WriteString("\n# HELP http_request_duration_seconds HTTP request duration in seconds\n")
	sb.WriteString("# TYPE http_request_duration_seconds histogram\n")
	httpRequestDuration.mu.RLock()
	for key, count := range httpRequestDuration.count {
		sb.WriteString(fmt.Sprintf("http_request_duration_seconds_count{%s} %d\n", key, count))
		sb.WriteString(fmt.Sprintf("http_request_duration_seconds_sum{%s} %.4f\n", key, httpRequestDuration.sum[key]))
	}
	httpRequestDuration.mu.RUnlock()

	sb.WriteString("\n# HELP http_active_requests Number of active HTTP requests\n")
	sb.WriteString("# TYPE http_active_requests gauge\n")
	activeRequests.mu.RLock()
	for key, val := range activeRequests.values {
		sb.WriteString(fmt.Sprintf("http_active_requests{%s} %d\n", key, val))
	}
	activeRequests.mu.RUnlock()

	sb.WriteString("\n# HELP ws_connections Number of active WebSocket connections\n")
	sb.WriteString("# TYPE ws_connections gauge\n")
	sb.WriteString(fmt.Sprintf("ws_connections %d\n", wsConnections.GetValue()))

	w.Write([]byte(sb.String()))
}

func RecordDBQuery(query string, duration time.Duration) {
	if dbQueryTotal != nil {
		dbQueryTotal.Inc(query)
	}
	if dbQueryDuration != nil {
		dbQueryDuration.Observe(duration.Seconds(), query)
	}
}

func RecordCacheHit() {
	if cacheHits != nil {
		cacheHits.Inc()
	}
}

func RecordCacheMiss() {
	if cacheMisses != nil {
		cacheMisses.Inc()
	}
}

func RecordWSConnection() {
	if wsConnections != nil {
		wsConnections.Inc()
	}
}

func RecordWSDisconnection() {
	if wsConnections != nil {
		wsConnections.Dec()
	}
}

func RecordWSMessage() {
	if wsMessagesTotal != nil {
		wsMessagesTotal.Inc()
	}
}

type PrometheusExporter struct {
	mu       sync.RWMutex
	counters map[string]*MetricCounter
	gauges   map[string]*MetricGauge
	histos   map[string]*MetricHistogram
}

func NewPrometheusExporter() *PrometheusExporter {
	return &PrometheusExporter{
		counters: make(map[string]*MetricCounter),
		gauges:   make(map[string]*MetricGauge),
		histos:   make(map[string]*MetricHistogram),
	}
}

func (p *PrometheusExporter) RegisterCounter(name, help string, labels ...string) *MetricCounter {
	p.mu.Lock()
	defer p.mu.Unlock()
	counter := NewMetricCounter(name, help, labels...)
	p.counters[name] = counter
	return counter
}

func (p *PrometheusExporter) RegisterGauge(name, help string) *MetricGauge {
	p.mu.Lock()
	defer p.mu.Unlock()
	gauge := NewMetricGauge(name, help)
	p.gauges[name] = gauge
	return gauge
}

func (p *PrometheusExporter) RegisterHistogram(name, help string, bounds []float64) *MetricHistogram {
	p.mu.Lock()
	defer p.mu.Unlock()
	hist := NewMetricHistogram(name, help, bounds)
	p.histos[name] = hist
	return hist
}

func (p *PrometheusExporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	var sb strings.Builder

	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, c := range p.counters {
		sb.WriteString(c.String())
		sb.WriteString("\n")
	}

	w.Write([]byte(sb.String()))
}
