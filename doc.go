package xcore

// xcore is a Go web framework package that provides wrappers around popular
// libraries with better defaults and integrated lifecycle management.
//
// # Core Features
//
//   - HTTP Server: gorilla/mux router with WebSocket support
//   - Database: GORM wrapper with multiple driver support
//   - Cache: Memory, File, and Redis (coming soon) backends
//   - Logger: zerolog wrapper with JSON/console output
//   - Cron: Job scheduler with graceful shutdown
//   - Services: Custom background services with auto start/stop
//   - Graceful: Automatic shutdown handling for all components
//
// # Quick Start
//
//	app := xcore.New().
//	    WithHTTP(&xcore.HTTPConfig{Port: 8080}).
//	    WithLogger(&xcore.LoggerConfig{Level: "info"}).
//	    WithDatabase(&xcore.DatabaseConfig{Driver: "sqlite", DBName: "app.db"})
//
//	app.Router().Get("/", func(w http.ResponseWriter, r *http.Request) {
//	    fmt.Fprintf(w, "Hello xcore!")
//	})
//
//	app.Run()
//
// # Background Services
//
// Implement the Service interface to add custom background services:
//
//	type MyWorker struct{}
//
//	func (w *MyWorker) Name() string { return "my-worker" }
//	func (w *MyWorker) Start() error { /* start worker */ return nil }
//	func (w *MyWorker) Stop() error  { /* stop worker */ return nil }
//
//	app := xcore.New().WithService(&MyWorker{})
//	app.Run()
//
// Services are automatically started on Run() and stopped on shutdown.
