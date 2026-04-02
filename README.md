# xcore

A comprehensive Go application runtime framework that provides built-in HTTP server with plans for gRPC and other protocols in future versions.

## Overview

xcore is a production-ready Go framework for building applications. It combines the best Go libraries with integrated lifecycle management, making it easy to build, run, and maintain applications. While HTTP is included as a core built-in feature, the framework is architected as an **application runtime** that can host multiple protocol handlers (HTTP, gRPC, etc.).

**Key Philosophy**: HTTP is a built-in capability, not the only capability. The framework is designed to be extensible for future protocol support including gRPC, WebSocket, and more.

## Features

### Built-in HTTP Server
- RESTful routing with gorilla/mux
- Route groups and middleware composition
- Static file serving with fallbacks
- Context-based handlers with automatic error handling

### WebSocket
- Real-time bidirectional communication
- Room-based messaging for broadcasting
- Connection management with ping/pong
- JSON message encoding support

### Middleware
- Recovery (panic recovery)
- Request ID tracking
- Gzip compression
- Body parsing with size limits
- Request timeout
- Real IP extraction
- Method override (POST to PUT/PATCH/DELETE)
- CORS support
- Rate limiting (global and per-IP)

### Authentication & Security
- JWT authentication (HS256, HS384, HS512, RS256, RS384, RS512)
- CSRF protection
- Session management (in-memory, custom stores)
- Security headers middleware

### Data & Storage
- Database: GORM with PostgreSQL, MySQL, SQLite, SQL Server support
- Connection pooling with configurable limits
- Transaction support
- Cache: Memory, File, and Redis backends
- Cache tagging for grouped invalidation

### Background Processing
- Cron job scheduler with panic recovery
- Custom services with auto start/stop
- Graceful shutdown coordination

### Logging & Monitoring
- Structured logging with zerolog (JSON/console)
- Request logging middleware
- Error file logging
- Metrics collection
- Health checks with component status

### Error Handling
- Structured error types with codes
- HTTP status mapping
- Validation error support
- Error middleware for automatic handling

### Graceful Shutdown
- Signal handling (SIGINT, SIGTERM)
- Coordinated shutdown of all components
- Configurable timeouts for callbacks and servers
- Database, cache, and service cleanup

## Architecture

xcore is designed as an **application runtime** with the following architecture:

```
┌─────────────────────────────────────────────────────────────┐
│                         xcore.App                           │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────┐  ┌─────────┐  ┌──────────┐  ┌──────────┐   │
│  │  HTTP   │  │   Web   │  │ Services │  │  Cron    │   │
│  │ Server  │  │ Socket  │  │          │  │ Jobs     │   │
│  └────┬────┘  └────┬────┘  └────┬─────┘  └────┬─────┘   │
│       │           │            │            │          │
│  ┌────┴────┐  ┌────┴────┐  ┌────┴─────┐  ┌────┴─────┐   │
│  │ Router │  │   WS   │  │ Service │  │ Cron    │   │
│  │         │  │ Manager│  │ Manager │  │ Manager │   │
│  └────┬────┘  └────────┘  └─────────┘  └─────────┘   │
│       │                                                  │
│  ┌────┴──────────────────────────────────────────┐    │
│  │            Graceful Shutdown Handler            │    │
│  └────┬──────────────────────────────────────────┘    │
│       │                                                  │
│  ┌────┴──────────────────────────────────────────┐    │
│  │         Integrated Components                  │    │
│  │  Logger │ Database │ Cache │ ConfigLoader    │    │
│  └──────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

## Installation

```bash
go get github.com/errteam/xcore
```

## Quick Start

```go
package main

import (
    "log"
    "net/http"
    
    "github.com/errteam/xcore"
)

func main() {
    app := xcore.New().
        WithHTTP(&xcore.HTTPConfig{Port: 8080}).
        WithLogger(&xcore.LoggerConfig{Level: "info"}).
        WithDatabase(&xcore.DatabaseConfig{Driver: "sqlite", DBName: "app.db"}).
        WithCache(&xcore.CacheConfig{Driver: "memory"}).
        WithGraceful(&xcore.GracefulConfig{Timeout: 30})
    
    // Register routes
    app.Router().GetHandler("/hello", func(c *xcore.Context) error {
        return c.JSONSuccess(map[string]string{"message": "Hello, World!"})
    })
    
    // Health check
    app.Router().GetHandler("/health", func(c *xcore.Context) error {
        return c.JSONSuccess(app.HealthCheck())
    })
    
    // Run the application
    if err := app.Run(); err != nil {
        log.Fatal(err)
    }
}
```

## Core Concepts

### App (Application Runtime)

The `App` struct is the main entry point for the framework. It orchestrates all components:

```go
app := xcore.New().
    WithHTTP(&xcore.HTTPConfig{Port: 8080}).
    WithLogger(&xcore.LoggerConfig{Level: "debug", Output: "console"}).
    WithDatabase(&xcore.DatabaseConfig{Driver: "postgres", Host: "localhost", Port: 5432, User: "user", Password: "pass", DBName: "app"}).
    WithCache(&xcore.CacheConfig{Driver: "redis", RedisAddr: "localhost:6379"}).
    WithCron(&xcore.CronConfig{Timezone: "UTC", RecoverPan: true}).
    WithWebSocket(&xcore.WebsocketConfig{Enabled: true, PingInterval: 30}).
    WithGraceful(&xcore.GracefulConfig{Timeout: 30})
```

### Accessing Components

```go
app.Logger()      // Structured logger
app.Router()      // HTTP router
app.Database()    // Database connection
app.Cache()       // Cache interface
app.Cron()        // Cron scheduler
app.WebSocket()  // WebSocket manager
app.Graceful()    // Graceful shutdown handler
app.Services()    // Service manager
```

### Router & Context-Based Handlers

The Router provides context-based handlers with automatic error handling:

```go
// GET handler
app.Router().GetHandler("/users", func(c *xcore.Context) error {
    return c.JSONSuccess(users)
})

// POST handler with body binding
app.Router().PostHandler("/users", func(c *xcore.Context) error {
    var req CreateUserRequest
    if err := c.BindJSON(&req); err != nil {
        return err  // Returns 400 Bad Request automatically
    }
    // Process request...
    return c.JSONCreated(newUser, "User created")
})

// Route parameters
app.Router().GetHandler("/users/:id", func(c *xcore.Context) error {
    id := c.Param("id")
    user, err := getUser(id)
    if err != nil {
        return xcore.ErrNotFound("User not found")
    }
    return c.JSONSuccess(user)
})

// Query parameters
app.Router().GetHandler("/search", func(c *xcore.Context) error {
    query := c.QueryParam("q")
    page := c.DefaultQuery("page", "1")
    return c.JSONSuccess(searchResults)
})

// Route groups
app.Router().Group("/api/v1", func(api *xcore.Router) {
    api.UseCORS(corsConfig)
    api.GetHandler("/users", handler)
    api.PostHandler("/users", handler)
    api.GetHandler("/posts", handler)
})
```

### Context

The `Context` provides convenient methods for handling HTTP requests:

```go
// Request data
id := c.Param("id")              // URL parameter
query := c.QueryParam("search")  // Query string
body := c.Body()                 // Request body bytes
c.BindJSON(&obj)                // Parse JSON body
c.BindForm(&obj)                // Parse form data
c.BindQuery(&obj)               // Parse query string
c.GetHeader("Authorization")   // Get header
c.Cookie("session")             // Get cookie
c.ClientIP()                    // Get client IP
c.RequestID()                   // Get request ID
c.RealIP()                      // Get real IP (from proxy headers)
c.UserID()                      // Get authenticated user ID

// Response methods
c.JSON(status, data)              // JSON response
c.JSONSuccess(data)               // 200 OK with standard format
c.JSONCreated(data, msg)          // 201 Created
c.JSONError(status, msg)          // Error response
c.JSONValidationError(errors)     // 422 Unprocessable Entity
c.JSONPaginated(data, page, perPage, total) // Paginated response
c.String(status, format, args...) // Plain text
c.HTML(status, html)              // HTML response
c.File(filepath)                  // Serve file
c.FileInline(filepath, name)      // Download file
c.Redirect(status, url)           // Redirect
c.Stream(status, contentType, reader) // Streaming response
```

### Middleware

Middleware can be applied at the app level or router level:

```go
// App-level (all routes)
app.Use(xcore.NewRecovery(nil).Middleware)
app.Use(xcore.NewCompression(gzip.DefaultCompression).Middleware)

// Router-level
router := app.Router()
router.UseRecovery()
router.UseRequestID()
router.UseCompression(gzip.DefaultCompression)
router.UseTimeout(30 * time.Second)
router.UseRateLimiter(100, 200)        // Global rate limit
router.UseRateLimiterPerIP(10, 20)    // Per-IP rate limit
router.UseCORS(&xcore.CORSConfig{
    AllowedOrigins: []string{"https://example.com"},
    AllowCredentials: true,
})
router.UseRealIP()
router.UseMethodOverride()
```

### Error Handling

Use the structured `XError` type for proper error handling:

```go
// Predefined errors
return xcore.ErrNotFound("User not found")
return xcore.ErrUnauthorized("Invalid credentials")
return xcore.ErrValidation("Invalid input")
return xcore.ErrForbidden("Access denied")
return xcore.ErrBadRequest("Invalid request")
return xcore.ErrTooManyRequests("Rate limit exceeded")

// Create custom errors
return xcore.NewError(xcore.ErrCodeBadRequest, "Invalid request")
return xcore.NewErrorWithStatus(xcore.ErrCodeValidation, "Validation failed", 422)

// With validation errors
return xcore.NewError(xcore.ErrCodeValidation, "Validation failed").
    WithErrors([]xcore.ValidationError{
        {Field: "email", Message: "Invalid email format"},
        {Field: "password", Message: "Password too short"},
    })

// Wrap errors with context
return xcore.ErrDatabase(err)
return xcore.ErrCache(err)
return xcore.ErrExternalAPI(err, "payment-service")

// Add metadata
return xcore.ErrNotFound("Resource not found").
    WithMeta("resource_id", id)
```

### Response Helpers

The framework provides standardized response formats:

```go
// Success responses
return c.JSONSuccess(data)
return c.JSONCreated(newUser, "Created successfully")
return c.JSONPaginated(items, page, perPage, total)

// Error responses
return c.JSONError(http.StatusBadRequest, "Invalid request")
return c.JSONValidationError(errors)

// Using Response builders
resp := xcore.NewResponse().
    WithStatus(xcore.StatusSuccess).
    WithCode(http.StatusOK).
    WithData(data).
    WithMessage("Success")
return c.JSON(http.StatusOK, resp)
```

### Services (Background Workers)

Implement custom background services that are started and stopped with the app:

```go
type MyWorker struct {
    done chan struct{}
}

func (w *MyWorker) Name() string { return "my-worker" }

func (w *MyWorker) Start() error {
    w.done = make(chan struct{})
    go w.run()
    return nil
}

func (w *MyWorker) Stop() error {
    close(w.done)
    return nil
}

func (w *MyWorker) run() {
    for {
        select {
        case <-w.done:
            return
        default:
            // Work...
            time.Sleep(time.Second)
        }
    }
}

app := xcore.New().WithService(&MyWorker{})
```

### Database

```go
// Configuration
dbConfig := &xcore.DatabaseConfig{
    Driver:          "postgres",
    Host:           "localhost",
    Port:           5432,
    User:           "user",
    Password:       "password",
    DBName:         "mydb",
    SSLMode:        "disable",
    MaxOpenConns:   25,
    MaxIdleConns:   5,
    ConnMaxLifetime: 300,
    ConnMaxIdleTime: 60,
}

// Usage (GORM)
app.Database().Create(&user)
app.Database().First(&user, id)
app.Database().Where("active = ?", true).Find(&users)
app.Database().Model(&user).Updates(map[string]interface{}{"name": "new name"})

// Transaction
app.Database().Transaction(ctx, func(ctx context.Context) error {
    if err := app.Database().Create(&order).Error; err != nil {
        return err
    }
    return app.Database().Create(&invoice).Error
})

// Health check
health := app.Database().Health(ctx)
fmt.Println(health.Status, health.Latency)
```

### Cache

```go
// Configuration
cacheConfig := &xcore.CacheConfig{
    Driver:          "redis",
    RedisAddr:       "localhost:6379",
    RedisPassword:   "",
    RedisDB:         0,
    RedisPoolSize:   10,
}

// Or memory cache
cacheConfig := &xcore.CacheConfig{
    Driver:          "memory",
    CleanupInterval: 60,
}

// Usage
ctx := context.Background()
app.Cache().Set(ctx, "user:1", userData, time.Hour)
value, err := app.Cache().Get(ctx, "user:1")
app.Cache().Delete(ctx, "user:1")

// Multi operations
app.Cache().MSet(ctx, map[string]interface{}{
    "key1": value1,
    "key2": value2,
})
results := app.Cache().MGet(ctx, "key1", "key2")

// Tags (memory cache)
app.Cache().Tags().SetTags(ctx, "user:1", "user", "active")
app.Cache().Tags().InvalidateByTag(ctx, "user")
```

### WebSocket

```go
// Configuration
wsConfig := &xcore.WebsocketConfig{
    Enabled:         true,
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    PingInterval:    30,
    PongTimeout:     10,
    MaxMessageSize:  1024 * 1024,
    AllowedOrigins:  []string{"*"}, // or ["https://example.com"]
}

app := xcore.New().WithWebSocket(wsConfig)

// Handler
app.Router().GetHandler("/ws", func(c *xcore.Context) error {
    conn := c.Request.Context().Value("ws_conn").(*xcore.WSConnection)
    
    // Read message
    msg := conn.ReadMessage()
    
    // Write message
    conn.WriteMessage(&xcore.WSMessage{
        Type:    xcore.WSMessageText,
        Payload: []byte("Hello"),
    })
    
    // Broadcast to room
    conn.BroadcastToRoom("room1", &xcore.WSMessage{...})
    
    return nil
})

// In WebSocket handler
hub := app.WebSocket().GetHub()
hub.Broadcast(&xcore.WSMessage{Type: xcore.WSMessageText, Payload: []byte("Hello")})
```

### Cron Jobs

```go
// Configuration
cronConfig := &xcore.CronConfig{
    Timezone:   "UTC",
    MaxJobs:    100,
    RecoverPan: true,
}

// Add cron jobs (cron expression: second minute hour day month weekday)
app.Cron().AddJob("daily-cleanup", "0 0 * * *", func() error {
    // Run daily at midnight
    return cleanupOldRecords()
})

app.Cron().AddJob("hourly-report", "0 */6 * * *", func() error {
    // Run every 6 hours
    return generateReport()
})

app.Cron().AddJob("every-minute", "* * * * *", func() error {
    // Run every minute
    return doSomething()
})

// Manage jobs
entries := app.Cron().Entries()
for _, entry := range entries {
    fmt.Println(entry.Name, entry.Next)
}

app.Cron().Stop() // Graceful stop
```

### JWT Authentication

```go
// Configuration
jwtConfig := xcore.NewJWTConfig("your-secret-key").
    WithAlgorithm("HS256").
    WithExpiration(24 * time.Hour).
    WithCookieName("token").
    WithCookieHTTPOnly(true).
    WithCookieSecure(true).
    WithCookieSameSite(http.SameSiteStrictMode)

// Or RSA
jwtConfig, _ := xcore.NewJWTConfig("").WithRSAPrivateKey(privateKeyPEM)

// Middleware
jwtMiddleware := xcore.NewJWTMiddleware(jwtConfig).
    Exclude("/health", "/metrics", "/login")

app.Use(jwtMiddleware.Middleware)

// Generate token
claims := xcore.NewJWTClaims("user-id", "username", "email@example.com", "admin")
token, err := jwtMiddleware.GenerateToken(claims)

// In handler - get claims
claims := xcore.GetJWTClaims(ctx)
userID := xcore.GetUserIDFromContext(ctx)
userID, username, email := xcore.GetUserFromContext(ctx)

// Set cookie
jwtMiddleware.SetTokenCookie(w, token)

// Clear cookie
jwtMiddleware.ClearTokenCookie(w)
```

### Session Management

```go
// Create session store (in-memory)
store := xcore.NewMemorySessionStore(30 * time.Minute)

// Use in handler
app.Router().PostHandler("/login", func(c *xcore.Context) error {
    // Authenticate...
    
    // Create session
    session, _ := store.Create(c.Context())
    session.Values["user_id"] = userID
    store.Set(c.Context(), session)
    
    // Set cookie
    c.SetCookie(&http.Cookie{
        Name:     "session_id",
        Value:    session.ID,
        HttpOnly: true,
    })
    
    return c.JSONSuccess(map[string]string{"status": "logged in"})
})
```

## Configuration Reference

### HTTP Config

```go
&xcore.HTTPConfig{
    Host:         "0.0.0.0",
    Port:         8080,
    ReadTimeout:  30,
    WriteTimeout: 30,
    IdleTimeout:  60,
    StaticPath:   "/static",
    StaticDir:    "./public",
}
```

### Logger Config

```go
&xcore.LoggerConfig{
    Level:       "info",       // debug, info, warn, error
    Output:      "both",        // console, file, both
    Format:      "console",    // console, json
    FilePath:    "./logs/app.log",
    MaxSize:     100,           // MB
    MaxAge:      30,            // days
    MaxBackups:  3,
    Compress:    true,
    Caller:      false,        // include file:line
}
```

### Database Config

```go
&xcore.DatabaseConfig{
    Driver:          "postgres",
    Host:            "localhost",
    Port:            5432,
    User:            "user",
    Password:        "password",
    DBName:          "mydb",
    SSLMode:         "disable",
    MaxOpenConns:    25,
    MaxIdleConns:    5,
    ConnMaxLifetime: 300,
    ConnMaxIdleTime: 60,
    ConnectTimeout:  10,
    LogLevel:       "error",    // silent, error, warn, info
}
```

### Cache Config

```go
// Memory
&xcore.CacheConfig{
    Driver:          "memory",
    CleanupInterval: 60,
}

// File
&xcore.CacheConfig{
    Driver:   "file",
    FilePath: "./cache",
    TTL:      3600,
}

// Redis
&xcore.CacheConfig{
    Driver:        "redis",
    RedisAddr:     "localhost:6379",
    RedisPassword: "",
    RedisDB:       0,
    RedisPoolSize: 10,
    RedisTLS:      false,
}
```

### WebSocket Config

```go
&xcore.WebsocketConfig{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    PingInterval:    30,
    PongTimeout:     10,
    MaxMessageSize:  1024 * 1024,
    Enabled:         true,
    AllowedOrigins:  []string{"*"}, // or ["https://example.com"]
}
```

### Cron Config

```go
&xcore.CronConfig{
    Timezone:   "UTC",
    MaxJobs:    100,
    RecoverPan: true,
}
```

### Graceful Config

```go
&xcore.GracefulConfig{
    Timeout: 30, // seconds
}
```

### CORS Config

```go
&xcore.CORSConfig{
    AllowedOrigins:   []string{"https://example.com"},
    AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    AllowedHeaders:   []string{"Content-Type", "Authorization"},
    ExposedHeaders:   []string{"X-Request-ID"},
    AllowCredentials: true,
    MaxAge:           86400,
    Enabled:          true,
}
```

## Best Practices

### 1. Use Context-Based Handlers

Always use `HandlerFunc` for proper error handling:

```go
// Good
app.Router().GetHandler("/users", func(c *xcore.Context) error {
    return c.JSONSuccess(users)
})

// Avoid - no automatic error handling
app.Router().HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
    // Manual handling required
})
```

### 2. Return Errors

```go
// Good - automatic error response
app.Router().GetHandler("/users", func(c *xcore.Context) error {
    user, err := getUser(c.Param("id"))
    if err != nil {
        return xcore.ErrNotFound("User not found")
    }
    return c.JSONSuccess(user)
})
```

### 3. Use Structured Errors

```go
// Validation errors with field details
if invalidInput {
    return xcore.NewError(xcore.ErrCodeValidation, "Validation failed").
        WithErrors([]xcore.ValidationError{
            {Field: "email", Message: "Invalid email format"},
            {Field: "age", Message: "Must be 18 or older"},
        })
}
```

### 4. Configure Graceful Shutdown

```go
app := xcore.New().
    WithGraceful(&xcore.GracefulConfig{Timeout: 30}).
    WithDatabase(dbConfig).
    WithCache(cacheConfig).
    WithService(&myWorker)
```

### 5. Database Connection Pooling

```go
&xcore.DatabaseConfig{
    MaxOpenConns:    25,       // Adjust based on load
    MaxIdleConns:    5,
    ConnMaxLifetime: 300,      // 5 minutes
    ConnMaxIdleTime: 60,       // 1 minute
}
```

### 6. Logging

```go
// Structured logging
logger.Info().
    Str("user_id", userID).
    Str("action", "create").
    Msg("operation completed")

// Error logging
logger.Error().Err(err).Str("query", query).Msg("database error")
```

### 7. Rate Limiting

```go
// Global rate limit
router.UseRateLimiter(100, 200)

// Per-IP rate limit
router.UseRateLimiterPerIP(10, 20)
```

### 8. Health Checks

```go
app.Router().GetHandler("/health", func(c *xcore.Context) error {
    return c.JSONSuccess(map[string]string{"status": "ok"})
})

app.Router().GetHandler("/ready", func(c *xcore.Context) error {
    // Check dependencies
    if err := app.Database().Ping(c.Context()); err != nil {
        return c.JSONError(http.StatusServiceUnavailable, "Database not ready")
    }
    return c.JSONSuccess(map[string]string{"status": "ready"})
})
```

### 9. Validation

Use validation tags on structs:

```go
type CreateUserRequest struct {
    Name  string `validate:"required,min=2,max=50"`
    Email string `validate:"required,email"`
    Age   int    `validate:"gte=0,lte=150"`
}

// In handler
func (c *Context) BindJSON(v interface{}) error {
    // Binding and validation happens automatically
}
```

### 10. Security Headers

```go
security := xcore.NewSecurityHeaders()
router.UseMiddleware(security.Middleware())
```

## Testing

```bash
go test -v ./...
go test -cover ./...
go test -race ./...
```

## Project Structure

```
myapp/
├── main.go
├── go.mod
├── config.yaml
├── app/
│   ├── handlers/
│   │   ├── user.go
│   │   └── product.go
│   ├── services/
│   │   └── worker.go
│   └── middleware/
│       └── auth.go
├── models/
│   └── user.go
└── migrations/
```

## Version

Current version: 1.0.0

## Roadmap

- [ ] gRPC support
- [ ] GraphQL support
- [ ] Rate limiting middleware improvements
- [ ] Distributed cache invalidation
- [ ] Metrics export (Prometheus)

## License

MIT License

## Dependencies

- github.com/gorilla/mux - HTTP routing
- github.com/gorilla/websocket - WebSocket support
- gorm.io/gorm - Database ORM
- gorm.io/driver/postgres - PostgreSQL driver
- gorm.io/driver/mysql - MySQL driver
- gorm.io/driver/sqlite - SQLite driver
- github.com/rs/zerolog - Logging
- github.com/golang-jwt/jwt/v5 - JWT authentication
- github.com/robfig/cron/v3 - Cron scheduling
- github.com/spf13/viper - Configuration
- github.com/go-redis/redis/v8 - Redis client
- github.com/go-playground/validator/v10 - Validation
- github.com/google/uuid - UUID generation

## Contributing

Contributions are welcome! Please submit issues and pull requests.