// Package xcore provides rate limiting functionality for the xcore framework.
//
// This package implements token bucket rate limiting for HTTP requests.
// It supports both global rate limiting and per-IP rate limiting.
//
// Rate limiting headers (X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset)
// are automatically added to responses. When rate limit is exceeded, a 429 response is returned.
package xcore

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RateLimiter implements token bucket rate limiting.
// It can operate in global mode (single bucket for all requests) or per-IP mode
// (separate bucket for each client IP).
type RateLimiter struct {
	requestsPerSecond int
	burst             int
	perIP             bool
	mu                sync.Mutex
	tokens            float64
	lastTime          time.Time
	ips               map[string]*ipLimiter
	ipMu              sync.RWMutex
	cleanupInterval   time.Duration
	stopCleanup       chan struct{}
}

// ipLimiter tracks token bucket state for a single IP address.
type ipLimiter struct {
	tokens   float64
	lastTime time.Time
}

// NewRateLimiter creates a new RateLimiter with the specified requests per second and burst.
// Default values are used if rps <= 0 (100) or burst <= 0 (100).
// Use EnablePerIP() to switch to per-IP rate limiting.
func NewRateLimiter(requestsPerSecond, burst int) *RateLimiter {
	if requestsPerSecond <= 0 {
		requestsPerSecond = 100
	}
	if burst <= 0 {
		burst = 100
	}
	return &RateLimiter{
		requestsPerSecond: requestsPerSecond,
		burst:             burst,
		tokens:            float64(burst),
		lastTime:          time.Now(),
		ips:               make(map[string]*ipLimiter),
		cleanupInterval:   5 * time.Minute,
		stopCleanup:       make(chan struct{}),
	}
}

// EnablePerIP enables per-IP rate limiting.
// Each client IP gets its own token bucket.
// Returns the RateLimiter for method chaining.
// Background goroutine starts to clean up stale IP entries (older than 10 minutes).
func (r *RateLimiter) EnablePerIP() *RateLimiter {
	r.perIP = true
	go r.cleanup()
	return r
}

// Stop stops the background cleanup goroutine for per-IP rate limiting.
// Call this when shutting down the server.
func (r *RateLimiter) Stop() {
	if r.perIP {
		select {
		case r.stopCleanup <- struct{}{}:
		default:
		}
	}
}

// Reset clears all per-IP rate limit data.
// This is useful for testing or manually resetting the rate limiter.
func (r *RateLimiter) Reset() {
	r.ipMu.Lock()
	defer r.ipMu.Unlock()
	r.ips = make(map[string]*ipLimiter)
}

// Middleware returns an http.Handler that enforces rate limiting.
// Adds rate limit headers to responses: X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset.
// Returns 429 Too Many Requests when rate limit is exceeded.
func (r *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var allowed bool
		var remaining int
		var resetTime time.Time

		if r.perIP {
			ctx := &Context{Request: req}
			ip := ctx.RealIP()
			if ip == "" {
				ip = req.RemoteAddr
			}
			allowed, remaining, resetTime = r.allowIPWithInfo(ip)
		} else {
			allowed, remaining, resetTime = r.allowWithInfo()
		}

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(r.burst))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))

		if !allowed {
			w.Header().Set("Retry-After", strconv.FormatInt(resetTime.Unix()-time.Now().Unix(), 10))
			resp := TooManyRequests("Rate limit exceeded")
			resp.Write(w)
			return
		}
		next.ServeHTTP(w, req)
	})
}

// allow checks if a request is allowed under global rate limiting.
// Uses token bucket algorithm with refill based on requestsPerSecond.
func (r *RateLimiter) allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastTime).Seconds()
	r.tokens += elapsed * float64(r.requestsPerSecond)
	r.lastTime = now

	if r.tokens > float64(r.burst) {
		r.tokens = float64(r.burst)
	}

	if r.tokens < 1 {
		return false
	}

	r.tokens--
	return true
}

// allowWithInfo checks if a request is allowed and returns additional rate limit info.
// Returns: allowed (bool), remaining tokens (int), reset time (time.Time).
func (r *RateLimiter) allowWithInfo() (bool, int, time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastTime).Seconds()
	r.tokens += elapsed * float64(r.requestsPerSecond)
	r.lastTime = now

	if r.tokens > float64(r.burst) {
		r.tokens = float64(r.burst)
	}

	remaining := int(r.tokens)
	resetAfter := float64(r.burst) - r.tokens
	resetTime := now.Add(time.Duration(resetAfter/float64(r.requestsPerSecond)) * time.Second)

	if r.tokens < 1 {
		return false, 0, resetTime
	}

	r.tokens--
	return true, remaining, resetTime
}

// allowIPWithInfo checks if a request from a specific IP is allowed.
// Creates a new bucket for new IPs. Returns: allowed (bool), remaining tokens (int), reset time (time.Time).
func (r *RateLimiter) allowIPWithInfo(ip string) (bool, int, time.Time) {
	r.ipMu.Lock()
	defer r.ipMu.Unlock()

	now := time.Now()
	limiter, exists := r.ips[ip]
	if !exists {
		r.ips[ip] = &ipLimiter{
			tokens:   float64(r.burst - 1),
			lastTime: now,
		}
		return true, r.burst - 1, now.Add(time.Duration(r.burst) / time.Duration(r.requestsPerSecond) * time.Second)
	}

	elapsed := now.Sub(limiter.lastTime).Seconds()
	limiter.tokens += elapsed * float64(r.requestsPerSecond)
	limiter.lastTime = now

	if limiter.tokens > float64(r.burst) {
		limiter.tokens = float64(r.burst)
	}

	remaining := int(limiter.tokens)
	resetAfter := float64(r.burst) - limiter.tokens
	resetTime := now.Add(time.Duration(resetAfter/float64(r.requestsPerSecond)) * time.Second)

	if limiter.tokens < 1 {
		return false, 0, resetTime
	}

	limiter.tokens--
	return true, remaining, resetTime
}

// cleanup runs periodically to remove stale IP entries.
// Removes IPs that haven't been seen in more than 10 minutes.
// This prevents memory growth from long-running servers.
func (r *RateLimiter) cleanup() {
	ticker := time.NewTicker(r.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.ipMu.Lock()
			now := time.Now()
			for ip, limiter := range r.ips {
				if now.Sub(limiter.lastTime) > 10*time.Minute {
					delete(r.ips, ip)
				}
			}
			r.ipMu.Unlock()
		case <-r.stopCleanup:
			return
		}
	}
}
