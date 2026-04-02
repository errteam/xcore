package xcore

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewRouter_DefaultConfig(t *testing.T) {
	r := NewRouter(nil)
	if r == nil {
		t.Error("NewRouter returned nil")
	}
	if r.router == nil {
		t.Error("router should not be nil")
	}
	if r.server == nil {
		t.Error("server should not be nil")
	}
	if r.config.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", r.config.Port)
	}
}

func TestNewRouter_CustomConfig(t *testing.T) {
	cfg := &HTTPConfig{
		Host: "localhost",
		Port: 9090,
	}
	r := NewRouter(cfg)
	if r.config.Port != 9090 {
		t.Errorf("expected port 9090, got %d", r.config.Port)
	}
	if r.config.Host != "localhost" {
		t.Errorf("expected host localhost, got %s", r.config.Host)
	}
}

func TestRouter_WithLogger(t *testing.T) {
	logger, _ := NewLogger(&LoggerConfig{Level: "error"})
	r := NewRouter(nil).WithLogger(logger)
	if r.logger != logger {
		t.Error("logger not set correctly")
	}
	if r.errorHandler == nil {
		t.Error("errorHandler should be created")
	}
}

func TestRouter_Use(t *testing.T) {
	r := NewRouter(nil)
	middlewareCalled := false
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			middlewareCalled = true
			next.ServeHTTP(w, req)
		})
	})

	r.router.HandleFunc("/test", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	r.router.ServeHTTP(w, req)

	if !middlewareCalled {
		t.Error("middleware should be called")
	}
}

func TestRouter_UseHandler(t *testing.T) {
	r := NewRouter(nil)
	handlerCalled := false
	r.UseHandler(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		handlerCalled = true
	}))

	r.router.HandleFunc("/test", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	r.router.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("handler should be called")
	}
}

func TestRouter_UseMiddleware(t *testing.T) {
	r := NewRouter(nil)

	r.router.HandleFunc("/test", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middlewareCalled := false
	r.UseMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			middlewareCalled = true
			next.ServeHTTP(w, req)
		})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	r.router.ServeHTTP(w, req)

	if !middlewareCalled {
		t.Error("middleware should be called")
	}
}

func TestRouter_UseRequestID(t *testing.T) {
	r := NewRouter(nil)
	r.UseRequestID()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	router := r.router
	router.HandleFunc("/test", func(w http.ResponseWriter, req *http.Request) {
		requestID := req.Header.Get("X-Request-ID")
		if requestID == "" {
			t.Error("X-Request-ID should be set")
		}
		w.WriteHeader(http.StatusOK)
	})

	router.ServeHTTP(w, req)
}

func TestRouter_UseCompression(t *testing.T) {
	r := NewRouter(nil)
	r.UseCompression(5)

	r.router.HandleFunc("/test", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	r.router.ServeHTTP(w, req)
}

func TestRouter_UseBodyParser(t *testing.T) {
	r := NewRouter(nil)
	r.UseBodyParser(1024)

	r.router.HandleFunc("/test", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("Content-Type", "application/json")

	r.router.ServeHTTP(w, req)
}

func TestRouter_UseTimeout(t *testing.T) {
	r := NewRouter(nil)
	r.UseTimeout(5 * time.Second)

	r.router.HandleFunc("/test", func(w http.ResponseWriter, req *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	r.router.ServeHTTP(w, req)
}

func TestRouter_UseRealIP(t *testing.T) {
	r := NewRouter(nil)
	r.UseRealIP()

	r.router.HandleFunc("/test", func(w http.ResponseWriter, req *http.Request) {
		ip := req.Context().Value(RealIPKey)
		if ip == nil {
			t.Error("RealIP should be set in context")
		}
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Real-IP", "1.2.3.4")

	r.router.ServeHTTP(w, req)
}

func TestRouter_UseCORS(t *testing.T) {
	r := NewRouter(nil)
	cfg := &CORSConfig{
		AllowedOrigins: []string{"*"},
		Enabled:        true,
	}
	r.UseCORS(cfg)

	r.router.HandleFunc("/test", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")

	r.router.ServeHTTP(w, req)
}

func TestRouter_UseMethodOverride(t *testing.T) {
	r := NewRouter(nil)
	r.UseMethodOverride()

	r.router.HandleFunc("/test", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", req.Method)
		}
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-HTTP-Method-Override", "PUT")

	r.router.ServeHTTP(w, req)
}

func TestRouter_Router(t *testing.T) {
	r := NewRouter(nil)
	if r.Router() == nil {
		t.Error("Router() returned nil")
	}
}

func TestRouter_Server(t *testing.T) {
	r := NewRouter(nil)
	if r.Server() == nil {
		t.Error("Server() returned nil")
	}
}

func TestRouter_Vars(t *testing.T) {
	r := NewRouter(nil)
	r.router.HandleFunc("/users/{id}", func(w http.ResponseWriter, req *http.Request) {
		vars := r.Vars(req)
		if vars["id"] != "123" {
			t.Errorf("expected id 123, got %s", vars["id"])
		}
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/users/123", nil)

	r.router.ServeHTTP(w, req)
}

func TestRouter_SetAddress(t *testing.T) {
	r := NewRouter(nil)
	r.SetAddress("localhost:9999")
	if r.server.Addr != "localhost:9999" {
		t.Errorf("expected addr localhost:9999, got %s", r.server.Addr)
	}
}

func TestRouter_SetHandler(t *testing.T) {
	r := NewRouter(nil)
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {})
	r.SetHandler(handler)
	if r.server.Handler == nil {
		t.Error("handler not set")
	}
}

func TestRouter_SetReadTimeout(t *testing.T) {
	r := NewRouter(nil)
	r.SetReadTimeout(60)
	if r.server.ReadTimeout != 60*time.Second {
		t.Errorf("expected read timeout 60s, got %v", r.server.ReadTimeout)
	}
}

func TestRouter_SetWriteTimeout(t *testing.T) {
	r := NewRouter(nil)
	r.SetWriteTimeout(60)
	if r.server.WriteTimeout != 60*time.Second {
		t.Errorf("expected write timeout 60s, got %v", r.server.WriteTimeout)
	}
}

func TestRouter_SetIdleTimeout(t *testing.T) {
	r := NewRouter(nil)
	r.SetIdleTimeout(120)
	if r.server.IdleTimeout != 120*time.Second {
		t.Errorf("expected idle timeout 120s, got %v", r.server.IdleTimeout)
	}
}

func TestRouter_HandleContext(t *testing.T) {
	r := NewRouter(nil)
	r.HandleContext("/test", func(c *Context) error {
		c.Status(http.StatusOK)
		return nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	r.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRouter_GetHandler(t *testing.T) {
	r := NewRouter(nil)
	r.GetHandler("/get", func(c *Context) error {
		return nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/get", nil)

	r.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRouter_PostHandler(t *testing.T) {
	r := NewRouter(nil)
	r.PostHandler("/post", func(c *Context) error {
		return nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/post", nil)

	r.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRouter_PutHandler(t *testing.T) {
	r := NewRouter(nil)
	r.PutHandler("/put", func(c *Context) error {
		return nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/put", nil)

	r.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRouter_PatchHandler(t *testing.T) {
	r := NewRouter(nil)
	r.PatchHandler("/patch", func(c *Context) error {
		return nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/patch", nil)

	r.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRouter_DeleteHandler(t *testing.T) {
	r := NewRouter(nil)
	r.DeleteHandler("/delete", func(c *Context) error {
		return nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/delete", nil)

	r.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRouter_OptionsHandler(t *testing.T) {
	r := NewRouter(nil)
	r.OptionsHandler("/options", func(c *Context) error {
		return nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/options", nil)

	r.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRouter_Group(t *testing.T) {
	r := NewRouter(nil)
	r.Group("/api", func(sub *Router) {
		sub.GetHandler("/test", func(c *Context) error {
			return nil
		})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/test", nil)

	r.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRouter_HandleFunc(t *testing.T) {
	r := NewRouter(nil)
	r.HandleFunc("/func", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/func", nil)

	r.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRouter_NotFoundHandler(t *testing.T) {
	r := NewRouter(nil)
	r.NotFoundHandler(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/notfound", nil)

	r.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}
