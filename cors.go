package xcore

import (
	"net/http"
	"strconv"
	"strings"
)

type CORSMiddleware struct {
	config *CORSConfig
}

func NewCORSMiddleware(cfg *CORSConfig) *CORSMiddleware {
	if cfg == nil {
		cfg = &CORSConfig{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Content-Type", "Authorization"},
			AllowCredentials: false,
			MaxAge:           0,
			ExposedHeaders:   []string{},
			Enabled:          true,
		}
	}

	if cfg.Enabled && cfg.AllowCredentials {
		for i, origin := range cfg.AllowedOrigins {
			if origin == "*" {
				cfg.AllowedOrigins[i] = ""
				break
			}
		}
	}

	return &CORSMiddleware{config: cfg}
}

func (m *CORSMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		allowed := false

		if m.config.AllowedOrigins != nil && len(m.config.AllowedOrigins) > 0 {
			for _, o := range m.config.AllowedOrigins {
				if o == "*" {
					allowed = true
					break
				}
				if strings.EqualFold(o, origin) {
					allowed = true
					break
				}
			}
		}

		if r.Method == http.MethodOptions {
			if allowed {
				if origin != "" {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(m.config.AllowedMethods, ", "))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(m.config.AllowedHeaders, ", "))

				if m.config.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				if m.config.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", strconv.Itoa(m.config.MaxAge))
				}
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if allowed && origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)

			if m.config.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if m.config.ExposedHeaders != nil && len(m.config.ExposedHeaders) > 0 {
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(m.config.ExposedHeaders, ", "))
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (m *CORSMiddleware) MiddlewareFunc() func(http.Handler) http.Handler {
	return m.Handler
}

func CORSMiddlewareFunc(cfg *CORSConfig) func(http.Handler) http.Handler {
	return NewCORSMiddleware(cfg).Handler
}
