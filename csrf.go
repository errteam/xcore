// Package xcore provides CSRF (Cross-Site Request Forgery) protection.
//
// This package provides CSRF token generation and validation middleware
// to protect against CSRF attacks in web applications.
package xcore

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

// CSRFConfig defines configuration for CSRF protection.
type CSRFConfig struct {
	TokenLength    int
	TokenName      string
	CookieName     string
	CookiePath     string
	CookieDomain   string
	CookieHTTPOnly bool
	CookieSecure   bool
	CookieSameSite http.SameSite
	HeaderName     string
	FormKeyName    string
	ExpireDuration time.Duration
	IgnoredMethods []string
	TrustedOrigins []string
}

func NewCSRFConfig() *CSRFConfig {
	return &CSRFConfig{
		TokenLength:    32,
		TokenName:      "csrf_token",
		CookieName:     "_csrf",
		CookiePath:     "/",
		CookieHTTPOnly: true,
		CookieSecure:   true,
		CookieSameSite: http.SameSiteStrictMode,
		HeaderName:     "X-CSRF-Token",
		FormKeyName:    "csrf_token",
		ExpireDuration: 24 * time.Hour,
		IgnoredMethods: []string{"GET", "HEAD", "OPTIONS"},
		TrustedOrigins: []string{},
	}
}

type CSRF struct {
	config *CSRFConfig
}

func NewCSRF(cfg *CSRFConfig) *CSRF {
	if cfg == nil {
		cfg = NewCSRFConfig()
	}
	return &CSRF{config: cfg}
}

func (c *CSRF) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.isIgnoredMethod(r.Method) {
			if r.Method == "GET" || r.Method == "HEAD" {
				c.setTokenCookie(w, r)
			}
			next.ServeHTTP(w, r)
			return
		}

		if !c.isTrustedOrigin(r) {
			http.Error(w, "Origin not trusted", http.StatusForbidden)
			return
		}

		cookie, err := r.Cookie(c.config.CookieName)
		if err != nil {
			http.Error(w, "CSRF token missing", http.StatusForbidden)
			return
		}

		token := cookie.Value
		if token == "" {
			http.Error(w, "CSRF token empty", http.StatusForbidden)
			return
		}

		headerToken := r.Header.Get(c.config.HeaderName)
		formToken := r.PostFormValue(c.config.FormKeyName)

		if headerToken == "" && formToken == "" {
			http.Error(w, "CSRF token missing from request", http.StatusForbidden)
			return
		}

		requestToken := headerToken
		if requestToken == "" {
			requestToken = formToken
		}

		if !c.compareTokens(token, requestToken) {
			http.Error(w, "CSRF token mismatch", http.StatusForbidden)
			return
		}

		c.setTokenCookie(w, r)
		next.ServeHTTP(w, r)
	})
}

func (c *CSRF) isIgnoredMethod(method string) bool {
	for _, m := range c.config.IgnoredMethods {
		if method == m {
			return true
		}
	}
	return false
}

func (c *CSRF) isTrustedOrigin(r *http.Request) bool {
	if len(c.config.TrustedOrigins) == 0 {
		return true
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = r.Header.Get("Referer")
	}

	if origin == "" {
		return len(c.config.TrustedOrigins) == 0
	}

	for _, trusted := range c.config.TrustedOrigins {
		if strings.EqualFold(origin, trusted) {
			return true
		}
	}

	return false
}

func (c *CSRF) compareTokens(a, b string) bool {
	return a == b
}

func (c *CSRF) setTokenCookie(w http.ResponseWriter, r *http.Request) {
	token := c.generateToken()
	http.SetCookie(w, &http.Cookie{
		Name:     c.config.CookieName,
		Value:    token,
		Path:     c.config.CookiePath,
		Domain:   c.config.CookieDomain,
		HttpOnly: c.config.CookieHTTPOnly,
		Secure:   c.config.CookieSecure,
		SameSite: c.config.CookieSameSite,
		Expires:  time.Now().Add(c.config.ExpireDuration),
	})
}

func (c *CSRF) generateToken() string {
	b := make([]byte, c.config.TokenLength)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func (c *CSRF) TokenGenerator() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := c.generateToken()
		http.SetCookie(w, &http.Cookie{
			Name:     c.config.CookieName,
			Value:    token,
			Path:     c.config.CookiePath,
			Domain:   c.config.CookieDomain,
			HttpOnly: c.config.CookieHTTPOnly,
			Secure:   c.config.CookieSecure,
			SameSite: c.config.CookieSameSite,
			Expires:  time.Now().Add(c.config.ExpireDuration),
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"` + token + `"}`))
	}
}
