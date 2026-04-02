package xcore

import (
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewRecovery(t *testing.T) {
	logger, _ := NewLogger(&LoggerConfig{Level: "error"})
	r := NewRecovery(logger)
	if r == nil {
		t.Error("NewRecovery returned nil")
	}
	if r.logger != logger {
		t.Error("logger not set correctly")
	}
}

func TestRecovery_Middleware(t *testing.T) {
	r := NewRecovery(nil)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		panic("test panic")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	r.Middleware(handler).ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("handler was not called")
	}

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
}

func TestNewRequestID(t *testing.T) {
	r := NewRequestID()
	if r == nil {
		t.Error("NewRequestID returned nil")
	}
}

func TestRequestID_Middleware_WithExisting(t *testing.T) {
	r := NewRequestID()

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", "custom-request-id")

	r.Middleware(handler).ServeHTTP(w, req)

	if w.Header().Get("X-Request-ID") != "custom-request-id" {
		t.Errorf("expected custom-request-id, got %s", w.Header().Get("X-Request-ID"))
	}
}

func TestRequestID_Middleware_GeneratesNew(t *testing.T) {
	r := NewRequestID()

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	r.Middleware(handler).ServeHTTP(w, req)

	requestID := w.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Error("X-Request-ID header not set")
	}
}

func TestNewCompression_DefaultLevel(t *testing.T) {
	c := NewCompression(-1)
	if c.level != gzip.DefaultCompression {
		t.Errorf("expected default compression level, got %d", c.level)
	}
}

func TestNewCompression_CustomLevel(t *testing.T) {
	c := NewCompression(gzip.BestCompression)
	if c.level != gzip.BestCompression {
		t.Errorf("expected best compression level, got %d", c.level)
	}
}

func TestNewCompression_InvalidLevel(t *testing.T) {
	c := NewCompression(100)
	if c.level != gzip.DefaultCompression {
		t.Errorf("expected default for invalid level, got %d", c.level)
	}
}

func TestCompression_Middleware_NoGzip(t *testing.T) {
	c := NewCompression(gzip.DefaultCompression)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "")

	c.Middleware(handler).ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("should not set gzip encoding without Accept-Encoding header")
	}
}

func TestNewBodyParser_Default(t *testing.T) {
	p := NewBodyParser(0)
	if p.maxSize != 10<<20 {
		t.Errorf("expected default max size, got %d", p.maxSize)
	}
}

func TestNewBodyParser_Custom(t *testing.T) {
	p := NewBodyParser(1024)
	if p.maxSize != 1024 {
		t.Errorf("expected 1024, got %d", p.maxSize)
	}
}

func TestNewBodyParser_Negative(t *testing.T) {
	p := NewBodyParser(-100)
	if p.maxSize != 10<<20 {
		t.Errorf("expected default for negative, got %d", p.maxSize)
	}
}

func TestBodyParser_Middleware_TooLarge(t *testing.T) {
	p := NewBodyParser(10)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", strings.NewReader("hello world"))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = 100

	p.Middleware(handler).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestNewTimeout_Default(t *testing.T) {
	tm := NewTimeout(0)
	if tm.timeout != 30*time.Second {
		t.Errorf("expected default timeout, got %v", tm.timeout)
	}
}

func TestNewTimeout_Custom(t *testing.T) {
	tm := NewTimeout(5 * time.Second)
	if tm.timeout != 5*time.Second {
		t.Errorf("expected 5s timeout, got %v", tm.timeout)
	}
}

func TestNewTimeout_Negative(t *testing.T) {
	tm := NewTimeout(-10 * time.Second)
	if tm.timeout != 30*time.Second {
		t.Errorf("expected default for negative, got %v", tm.timeout)
	}
}

func TestTimeout_Middleware_CompletesInTime(t *testing.T) {
	tm := NewTimeout(100 * time.Millisecond)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	tm.Middleware(handler).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestTimeout_Middleware_Timeout(t *testing.T) {
	tm := NewTimeout(50 * time.Millisecond)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	tm.Middleware(handler).ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
}

func TestNewRealIP(t *testing.T) {
	ri := NewRealIP()
	if ri == nil {
		t.Error("NewRealIP returned nil")
	}
}

func TestRealIP_Middleware_XRealIP(t *testing.T) {
	ri := NewRealIP()

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ip := req.Context().Value(RealIPKey)
		if ip != "1.2.3.4" {
			t.Errorf("expected 1.2.3.4, got %v", ip)
		}
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "1.2.3.4")

	ri.Middleware(handler).ServeHTTP(w, req)
}

func TestRealIP_Middleware_XForwardedFor(t *testing.T) {
	ri := NewRealIP()

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ip := req.Context().Value(RealIPKey)
		if ip != "1.2.3.4" {
			t.Errorf("expected 1.2.3.4, got %v", ip)
		}
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	req.RemoteAddr = "127.0.0.1:1234"

	ri.Middleware(handler).ServeHTTP(w, req)
}

func TestNewMethodOverride(t *testing.T) {
	m := NewMethodOverride()
	if m == nil {
		t.Error("NewMethodOverride returned nil")
	}
}

func TestMethodOverride_Middleware_Override(t *testing.T) {
	m := NewMethodOverride()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-HTTP-Method-Override", "PUT")

	m.Middleware(handler).ServeHTTP(w, req)
}

func TestMethodOverride_Middleware_NoOverride(t *testing.T) {
	m := NewMethodOverride()

	method := ""
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", nil)

	m.Middleware(handler).ServeHTTP(w, req)

	if method != http.MethodPost {
		t.Errorf("expected POST, got %s", method)
	}
}

func TestMethodOverride_Middleware_InvalidOverride(t *testing.T) {
	m := NewMethodOverride()

	method := ""
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-HTTP-Method-Override", "INVALID")

	m.Middleware(handler).ServeHTTP(w, req)

	if method != http.MethodPost {
		t.Errorf("expected POST, got %s", method)
	}
}
