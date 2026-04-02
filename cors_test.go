package xcore

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewCORSMiddleware_DefaultConfig(t *testing.T) {
	mw := NewCORSMiddleware(nil)
	if mw == nil {
		t.Error("NewCORSMiddleware returned nil")
	}
	if mw.config == nil {
		t.Error("config should not be nil")
	}
}

func TestNewCORSMiddleware_CustomConfig(t *testing.T) {
	cfg := &CORSConfig{
		AllowedOrigins:   []string{"https://example.com"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: false,
		MaxAge:           3600,
		ExposedHeaders:   []string{"X-Custom-Header"},
		Enabled:          true,
	}
	mw := NewCORSMiddleware(cfg)
	if mw.config.AllowedOrigins[0] != "https://example.com" {
		t.Error("custom config not applied")
	}
}

func TestNewCORSMiddleware_DisablesWildcardWithCredentials(t *testing.T) {
	cfg := &CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
		Enabled:          true,
	}
	mw := NewCORSMiddleware(cfg)

	if mw.config.AllowedOrigins[0] != "" {
		t.Error("wildcard should be disabled when credentials enabled")
	}
}

func TestCORSMiddleware_Disabled(t *testing.T) {
	cfg := &CORSConfig{
		Enabled: false,
	}
	mw := NewCORSMiddleware(cfg)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	mw.Handler(handler).ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("handler should be called when CORS disabled")
	}
}

func TestCORSMiddleware_Preflight_AllowedOrigin(t *testing.T) {
	cfg := &CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
		Enabled:        true,
	}
	mw := NewCORSMiddleware(cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	mw.Handler(handler).ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Error("Access-Control-Allow-Origin not set correctly")
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("Access-Control-Allow-Methods not set")
	}
	if w.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Error("Access-Control-Allow-Headers not set")
	}
	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}
}

func TestCORSMiddleware_Preflight_Wildcard(t *testing.T) {
	cfg := &CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
		Enabled:        true,
	}
	mw := NewCORSMiddleware(cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	mw.Handler(handler).ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Error("Access-Control-Allow-Origin not set correctly")
	}
}

func TestCORSMiddleware_Preflight_DisallowedOrigin(t *testing.T) {
	cfg := &CORSConfig{
		AllowedOrigins: []string{"https://allowed.com"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
		Enabled:        true,
	}
	mw := NewCORSMiddleware(cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://disallowed.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	mw.Handler(handler).ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("Access-Control-Allow-Origin should not be set for disallowed origin")
	}
	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}
}

func TestCORSMiddleware_Preflight_WithCredentials(t *testing.T) {
	cfg := &CORSConfig{
		AllowedOrigins:   []string{"https://example.com"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
		Enabled:          true,
	}
	mw := NewCORSMiddleware(cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	mw.Handler(handler).ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("Access-Control-Allow-Credentials should be true")
	}
}

func TestCORSMiddleware_Preflight_WithMaxAge(t *testing.T) {
	cfg := &CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
		MaxAge:         3600,
		Enabled:        true,
	}
	mw := NewCORSMiddleware(cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	mw.Handler(handler).ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Max-Age") != "3600" {
		t.Error("Access-Control-Max-Age not set correctly")
	}
}

func TestCORSMiddleware_SimpleRequest_Allowed(t *testing.T) {
	cfg := &CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		Enabled:        true,
	}
	mw := NewCORSMiddleware(cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")

	mw.Handler(handler).ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Error("Access-Control-Allow-Origin not set")
	}
}

func TestCORSMiddleware_SimpleRequest_Disallowed(t *testing.T) {
	cfg := &CORSConfig{
		AllowedOrigins: []string{"https://allowed.com"},
		Enabled:        true,
	}
	mw := NewCORSMiddleware(cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://disallowed.com")

	mw.Handler(handler).ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("Access-Control-Allow-Origin should not be set")
	}
}

func TestCORSMiddleware_SimpleRequest_ExposedHeaders(t *testing.T) {
	cfg := &CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		ExposedHeaders: []string{"X-Custom-Header"},
		Enabled:        true,
	}
	mw := NewCORSMiddleware(cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")

	mw.Handler(handler).ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Expose-Headers") != "X-Custom-Header" {
		t.Error("Access-Control-Expose-Headers not set")
	}
}

func TestCORSMiddleware_MiddlewareFunc(t *testing.T) {
	mw := NewCORSMiddleware(nil)
	fn := mw.MiddlewareFunc()
	if fn == nil {
		t.Error("MiddlewareFunc returned nil")
	}
}

func TestCORSMiddlewareFunc(t *testing.T) {
	cfg := &CORSConfig{
		AllowedOrigins: []string{"*"},
		Enabled:        true,
	}
	fn := CORSMiddlewareFunc(cfg)
	if fn == nil {
		t.Error("CORSMiddlewareFunc returned nil")
	}
}
