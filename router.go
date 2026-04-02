package xcore

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

type Router struct {
	router       *mux.Router
	server       *http.Server
	config       *HTTPConfig
	logger       *Logger
	errorHandler *ErrorHandler
}

func NewRouter(cfg *HTTPConfig) *Router {
	if cfg == nil {
		cfg = &HTTPConfig{
			Port: 8080,
		}
	}

	r := &Router{
		router: mux.NewRouter(),
		config: cfg,
	}

	r.server = &http.Server{
		Addr:         getAddr(cfg),
		Handler:      r.router,
		ReadTimeout:  getDuration(cfg.ReadTimeout, 30),
		WriteTimeout: getDuration(cfg.WriteTimeout, 30),
		IdleTimeout:  getDuration(cfg.IdleTimeout, 60),
	}

	return r
}

func (r *Router) WithLogger(logger *Logger) *Router {
	r.logger = logger
	r.errorHandler = NewErrorHandler(logger)
	return r
}

func (r *Router) Use(middleware ...mux.MiddlewareFunc) {
	r.router.Use(middleware...)
}

func (r *Router) UseHandler(h http.Handler) {
	r.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			h.ServeHTTP(w, req)
		})
	})
}

func (r *Router) UseMiddleware(mw func(http.Handler) http.Handler) {
	r.router.Use(mw)
}

func (r *Router) UseRequestLogger() {
	if r.logger == nil {
		return
	}
	r.router.Use(NewRequestLogger(r.logger).Middleware)
}

func (r *Router) UseRecovery() {
	if r.logger == nil {
		return
	}
	r.router.Use(NewRecovery(r.logger).Middleware)
}

func (r *Router) UseRequestID() {
	r.router.Use(NewRequestID().Middleware)
}

func (r *Router) UseCompression(level int) {
	r.router.Use(NewCompression(level).Middleware)
}

func (r *Router) UseBodyParser(maxSize int64) {
	r.router.Use(NewBodyParser(maxSize).Middleware)
}

func (r *Router) UseTimeout(timeout time.Duration) {
	r.router.Use(NewTimeout(timeout).Middleware)
}

func (r *Router) UseRateLimiter(rps, burst int) {
	r.router.Use(NewRateLimiter(rps, burst).Middleware)
}

func (r *Router) UseRateLimiterPerIP(rps, burst int) {
	r.router.Use(NewRateLimiter(rps, burst).EnablePerIP().Middleware)
}

func (r *Router) UseRealIP() {
	r.router.Use(NewRealIP().Middleware)
}

func (r *Router) UseCORS(cfg *CORSConfig) {
	r.router.Use(NewCORSMiddleware(cfg).Handler)
}

func (r *Router) UseMethodOverride() {
	r.router.Use(NewMethodOverride().Middleware)
}

func (r *Router) Static(path, dir string) {
	fs := http.Dir(dir)
	router := r.router.PathPrefix(path).Handler(http.StripPrefix(path, http.FileServer(fs)))
	router.Name("static:" + path)
}

func (r *Router) StaticFS(path string, fs http.FileSystem) {
	r.router.PathPrefix(path).Handler(http.StripPrefix(path, http.FileServer(fs)))
}

func (r *Router) StaticWithOptions(path, dir string, opts StaticOptions) {
	if opts.Index == "" {
		opts.Index = "index.html"
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		filePath := req.URL.Path

		f, err := http.Dir(dir).Open(filePath)
		if err != nil {
			if opts.Fallback != "" {
				http.ServeFile(w, req, opts.Fallback)
				return
			}
			http.NotFound(w, req)
			return
		}
		defer f.Close()

		fi, err := f.Stat()
		if err != nil {
			http.NotFound(w, req)
			return
		}

		if fi.IsDir() {
			indexPath := filePath + "/" + opts.Index
			if _, err := http.Dir(dir).Open(indexPath); err == nil {
				http.ServeFile(w, req, indexPath)
				return
			}

			if opts.Fallback != "" && opts.DirectoryListing {
				http.ServeFile(w, req, opts.Fallback)
				return
			}

			if opts.DirectoryListing {
				http.FileServer(http.Dir(dir)).ServeHTTP(w, req)
				return
			}

			if opts.Fallback != "" {
				http.ServeFile(w, req, opts.Fallback)
				return
			}
			http.NotFound(w, req)
			return
		}

		http.ServeFile(w, req, filePath)
	})

	r.router.PathPrefix(path).Handler(http.StripPrefix(path, handler))
}

type StaticOptions struct {
	Index            string
	Fallback         string
	DirectoryListing bool
}

func (r *Router) Favicon(file string) {
	r.router.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, file)
	})
}

func getAddr(cfg *HTTPConfig) string {
	host := cfg.Host
	if host == "" {
		host = "0.0.0.0"
	}
	port := cfg.Port
	if port == 0 {
		port = 8080
	}
	return host + ":" + strconv.Itoa(port)
}

func getDuration(seconds int, defaultVal int) time.Duration {
	if seconds <= 0 {
		return time.Duration(defaultVal) * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func (r *Router) Router() *mux.Router {
	return r.router
}

func (r *Router) Server() *http.Server {
	return r.server
}

func (r *Router) NotFoundHandler(handler http.HandlerFunc) {
	r.router.NotFoundHandler = handler
}

func (r *Router) ServeHTTP(w http.ResponseWriter, rq *http.Request) {
	r.router.ServeHTTP(w, rq)
}

func (r *Router) Vars(rq *http.Request) map[string]string {
	return mux.Vars(rq)
}

func (r *Router) SetAddress(addr string) {
	r.server.Addr = addr
}

func (r *Router) SetHandler(handler http.Handler) {
	r.server.Handler = handler
}

func (r *Router) SetReadTimeout(timeout int) {
	r.server.ReadTimeout = time.Duration(timeout) * time.Second
}

func (r *Router) SetWriteTimeout(timeout int) {
	r.server.WriteTimeout = time.Duration(timeout) * time.Second
}

func (r *Router) SetIdleTimeout(timeout int) {
	r.server.IdleTimeout = time.Duration(timeout) * time.Second
}

func (r *Router) UseErrorHandler() {
	r.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			c := NewContext(w, req)
			c.logger = r.logger
			next.ServeHTTP(w, req)
		})
	})
}

func (r *Router) handleWithError(w http.ResponseWriter, req *http.Request, handler HandlerFunc) {
	c := NewContext(w, req)
	c.logger = r.logger
	if err := handler(c); err != nil {
		if r.errorHandler != nil {
			r.errorHandler.HandleError(c, err)
		} else {
			NewErrorHandler(r.logger).HandleError(c, err)
		}
	}
}

func (r *Router) HandleContext(path string, handler HandlerFunc) *mux.Route {
	return r.router.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		r.handleWithError(w, req, handler)
	})
}

func (r *Router) GetHandler(path string, handler HandlerFunc) *mux.Route {
	return r.router.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		r.handleWithError(w, req, handler)
	}).Methods(http.MethodGet)
}

func (r *Router) PostHandler(path string, handler HandlerFunc) *mux.Route {
	return r.router.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		r.handleWithError(w, req, handler)
	}).Methods(http.MethodPost)
}

func (r *Router) PutHandler(path string, handler HandlerFunc) *mux.Route {
	return r.router.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		r.handleWithError(w, req, handler)
	}).Methods(http.MethodPut)
}

func (r *Router) PatchHandler(path string, handler HandlerFunc) *mux.Route {
	return r.router.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		r.handleWithError(w, req, handler)
	}).Methods(http.MethodPatch)
}

func (r *Router) DeleteHandler(path string, handler HandlerFunc) *mux.Route {
	return r.router.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		r.handleWithError(w, req, handler)
	}).Methods(http.MethodDelete)
}

func (r *Router) OptionsHandler(path string, handler HandlerFunc) *mux.Route {
	return r.router.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		r.handleWithError(w, req, handler)
	}).Methods(http.MethodOptions)
}

func (r *Router) Group(prefix string, fn func(*Router)) *Router {
	subRouter := r.router.PathPrefix(prefix).Subrouter()
	sub := &Router{
		router:       subRouter,
		config:       r.config,
		logger:       r.logger,
		errorHandler: r.errorHandler,
	}
	fn(sub)
	return r
}

func (r *Router) HandleFunc(path string, handler func(http.ResponseWriter, *http.Request)) *mux.Route {
	return r.router.HandleFunc(path, handler)
}

func (r *Router) Name(name string) *mux.Route {
	return r.router.Name(name)
}
