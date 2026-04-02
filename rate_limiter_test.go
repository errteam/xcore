package xcore

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewRateLimiter_DefaultValues(t *testing.T) {
	rl := NewRateLimiter(0, 0)
	if rl.requestsPerSecond != 100 {
		t.Errorf("expected 100 requestsPerSecond, got %d", rl.requestsPerSecond)
	}
	if rl.burst != 100 {
		t.Errorf("expected 100 burst, got %d", rl.burst)
	}
}

func TestNewRateLimiter_CustomValues(t *testing.T) {
	rl := NewRateLimiter(50, 200)
	if rl.requestsPerSecond != 50 {
		t.Errorf("expected 50 requestsPerSecond, got %d", rl.requestsPerSecond)
	}
	if rl.burst != 200 {
		t.Errorf("expected 200 burst, got %d", rl.burst)
	}
}

func TestNewRateLimiter_NegativeValues(t *testing.T) {
	rl := NewRateLimiter(-10, -20)
	if rl.requestsPerSecond != 100 {
		t.Errorf("expected 100 for negative requestsPerSecond, got %d", rl.requestsPerSecond)
	}
	if rl.burst != 100 {
		t.Errorf("expected 100 for negative burst, got %d", rl.burst)
	}
}

func TestRateLimiter_EnablePerIP(t *testing.T) {
	rl := NewRateLimiter(100, 100)
	result := rl.EnablePerIP()
	if result != rl {
		t.Error("EnablePerIP should return self")
	}
	if !rl.perIP {
		t.Error("perIP should be true after EnablePerIP")
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	rl := NewRateLimiter(100, 100).EnablePerIP()

	rl.allowIPWithInfo("1.2.3.4")
	rl.Reset()

	rl.ipMu.RLock()
	if len(rl.ips) != 0 {
		t.Errorf("expected 0 IPs after reset, got %d", len(rl.ips))
	}
	rl.ipMu.RUnlock()
}

func TestRateLimiter_Stop(t *testing.T) {
	rl := NewRateLimiter(100, 100).EnablePerIP()

	rl.Stop()

	time.Sleep(50 * time.Millisecond)
}

func TestRateLimiter_allow(t *testing.T) {
	rl := NewRateLimiter(100, 100)

	allowed := rl.allow()
	if !allowed {
		t.Error("first request should be allowed")
	}
}

func TestRateLimiter_allow_Exhausted(t *testing.T) {
	rl := NewRateLimiter(1, 1)

	rl.allow()

	allowed := rl.allow()
	if allowed {
		t.Error("second request should be denied when burst exhausted")
	}
}

func TestRateLimiter_allowWithInfo(t *testing.T) {
	rl := NewRateLimiter(100, 100)

	allowed, remaining, resetTime := rl.allowWithInfo()
	if !allowed {
		t.Error("first request should be allowed")
	}
	if remaining != 100 {
		t.Errorf("expected 100 remaining, got %d", remaining)
	}
	if resetTime.IsZero() {
		t.Error("resetTime should not be zero")
	}
}

func TestRateLimiter_allowWithInfo_Exhausted(t *testing.T) {
	rl := NewRateLimiter(1, 1)

	rl.allowWithInfo()

	allowed, remaining, _ := rl.allowWithInfo()
	if allowed {
		t.Error("second request should be denied")
	}
	if remaining != 0 {
		t.Errorf("expected 0 remaining, got %d", remaining)
	}
}

func TestRateLimiter_allowIPWithInfo_NewIP(t *testing.T) {
	rl := NewRateLimiter(100, 100).EnablePerIP()

	allowed, remaining, _ := rl.allowIPWithInfo("1.2.3.4")
	if !allowed {
		t.Error("new IP should be allowed")
	}
	if remaining != 99 {
		t.Errorf("expected 99 remaining, got %d", remaining)
	}
}

func TestRateLimiter_allowIPWithInfo_ExistingIP(t *testing.T) {
	rl := NewRateLimiter(100, 100).EnablePerIP()

	rl.allowIPWithInfo("1.2.3.4")

	allowed, remaining, _ := rl.allowIPWithInfo("1.2.3.4")
	if !allowed {
		t.Error("second request from same IP should be allowed")
	}
	if remaining != 99 {
		t.Errorf("expected 99 remaining, got %d", remaining)
	}
}

func TestRateLimiter_Middleware_AllowsRequest(t *testing.T) {
	rl := NewRateLimiter(100, 100)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	rl.Middleware(handler).ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("handler should be called when request allowed")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRateLimiter_Middleware_RateLimitHeaders(t *testing.T) {
	rl := NewRateLimiter(100, 100)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	rl.Middleware(handler).ServeHTTP(w, req)

	if w.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("X-RateLimit-Limit header should be set")
	}
	if w.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("X-RateLimit-Remaining header should be set")
	}
	if w.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("X-RateLimit-Reset header should be set")
	}
}

func TestRateLimiter_Middleware_DeniesRequest(t *testing.T) {
	rl := NewRateLimiter(1, 1)

	rl.allow()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	rl.Middleware(handler).ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("Retry-After header should be set")
	}
}

func TestRateLimiter_Middleware_PerIP(t *testing.T) {
	rl := NewRateLimiter(1, 1).EnablePerIP()

	allowed, _, _ := rl.allowIPWithInfo("127.0.0.1")
	if !allowed {
		t.Error("first request should be allowed")
	}

	allowed, _, _ = rl.allowIPWithInfo("127.0.0.1")
	if allowed {
		t.Error("second request should be denied")
	}
}
