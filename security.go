// Package xcore provides security headers and helpers.
//
// This package provides security-related utilities including
// security headers for HTTP responses and helper functions.
package xcore

import (
	"net/http"
	"strconv"
	"strings"
)

// SecurityHeaders defines security headers for HTTP responses.
// These headers help protect against common web vulnerabilities.
type SecurityHeaders struct {
	ContentTypeNosniff      string
	XFrameOptions           string
	XContentTypeOptions     string
	XXSSProtection          string
	ReferrerPolicy          string
	PermissionsPolicy       string
	StrictTransportSecurity string
	ContentSecurityPolicy   string
}

func NewSecurityHeaders() *SecurityHeaders {
	return &SecurityHeaders{
		ContentTypeNosniff:      "nosniff",
		XFrameOptions:           "SAMEORIGIN",
		XContentTypeOptions:     "nosniff",
		XXSSProtection:          "1; mode=block",
		ReferrerPolicy:          "strict-origin-when-cross-origin",
		PermissionsPolicy:       "",
		StrictTransportSecurity: "max-age=31536000; includeSubDomains",
		ContentSecurityPolicy:   "",
	}
}

func (s *SecurityHeaders) WithCSP(csp string) *SecurityHeaders {
	s.ContentSecurityPolicy = csp
	return s
}

func (s *SecurityHeaders) WithHSTS(maxAge int, includeSubDomains bool) *SecurityHeaders {
	if maxAge > 0 {
		hsts := "max-age=" + strconv.Itoa(maxAge)
		if includeSubDomains {
			hsts += "; includeSubDomains"
		}
		s.StrictTransportSecurity = hsts
	}
	return s
}

func (s *SecurityHeaders) WithXFO(xfo string) *SecurityHeaders {
	s.XFrameOptions = xfo
	return s
}

func (s *SecurityHeaders) WithReferrerPolicy(policy string) *SecurityHeaders {
	s.ReferrerPolicy = policy
	return s
}

func (s *SecurityHeaders) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.ContentTypeNosniff != "" {
			w.Header().Set("X-Content-Type-Options", s.ContentTypeNosniff)
		}

		if s.XFrameOptions != "" {
			w.Header().Set("X-Frame-Options", s.XFrameOptions)
		}

		if s.XContentTypeOptions != "" {
			w.Header().Set("X-Content-Type-Options", s.XContentTypeOptions)
		}

		if s.XXSSProtection != "" {
			w.Header().Set("X-XSS-Protection", s.XXSSProtection)
		}

		if s.ReferrerPolicy != "" {
			w.Header().Set("Referrer-Policy", s.ReferrerPolicy)
		}

		if s.PermissionsPolicy != "" {
			w.Header().Set("Permissions-Policy", s.PermissionsPolicy)
		}

		if s.StrictTransportSecurity != "" && r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", s.StrictTransportSecurity)
		}

		if s.ContentSecurityPolicy != "" {
			w.Header().Set("Content-Security-Policy", s.ContentSecurityPolicy)
		}

		next.ServeHTTP(w, r)
	})
}

func NewHelmet() *SecurityHeaders {
	return NewSecurityHeaders()
}

func HelmetCSPDefault() string {
	return "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self';"
}

func HelmetCSPStrict() string {
	return "default-src 'none'; script-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none';"
}

type SecureHeadersMiddleware struct {
	headers *SecurityHeaders
}

func NewSecureHeadersMiddleware() *SecureHeadersMiddleware {
	return &SecureHeadersMiddleware{
		headers: NewSecurityHeaders(),
	}
}

func (m *SecureHeadersMiddleware) Middleware(next http.Handler) http.Handler {
	return m.headers.Middleware(next)
}

func (m *SecureHeadersMiddleware) WithCSP(csp string) *SecureHeadersMiddleware {
	m.headers.ContentSecurityPolicy = csp
	return m
}

func (m *SecureHeadersMiddleware) WithHSTS(maxAge int, includeSubDomains bool) *SecureHeadersMiddleware {
	if maxAge > 0 {
		hsts := "max-age=" + strconv.Itoa(maxAge)
		if includeSubDomains {
			hsts += "; includeSubDomains"
		}
		m.headers.StrictTransportSecurity = hsts
	}
	return m
}

func (m *SecureHeadersMiddleware) HandlerFunc(w http.ResponseWriter, r *http.Request) {
	m.headers.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(w, r)
}

func NoCache() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			next.ServeHTTP(w, r)
		})
	}
}

func StaticCache(maxAge int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/static/") || strings.HasPrefix(r.URL.Path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(maxAge))
			}
			next.ServeHTTP(w, r)
		})
	}
}
