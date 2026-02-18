# RBAC - Role-Based Access Control

Production-ready role-based access control package with GORM, featuring wildcard permissions, caching, and automatic route registration.

## Features

- ✅ **Permission Namespaces**: `admin.user.list`, `user.wallet.withdraw`
- ✅ **Wildcard Support**: `admin.*`, `user.wallet.*`
- ✅ **Flexible Levels**: 2-level (`auth.login`) or 3-level (`admin.user.list`)
- ✅ **Role Hierarchy**: Default roles (`root`, `user`) with inheritance support
- ✅ **Explicit Deny**: Allow permissions but deny specific ones
- ✅ **Caching**: Automatic permission caching with TTL
- ✅ **Auto-Registration**: Walk routes and auto-register permissions
- ✅ **CRUD API**: Full REST API for managing roles/permissions
- ✅ **Middleware**: HTTP middleware for permission checking

## Quick Start

```go
package main

import (
    "github.com/gorilla/mux"
    "github.com/rs/zerolog"
    "gorm.io/gorm"
    "xcore-example/pkg/xcore/rbac"
)

func main() {
    // Initialize database
    db, _ := gorm.Open(...)
    logger := zerolog.New(os.Stdout)

    // Create RBAC instance
    r, err := rbac.New(db, &rbac.Config{
        EnableCache:  true,
        CacheTTL:     5 * time.Minute,
        AutoRegister: true,
        Logger:       &logger,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Initialize (creates default roles: root, user)
    if err := r.Initialize(); err != nil {
        log.Fatal(err)
    }

    // Create router
    router := mux.NewRouter()

    // Register RBAC API routes (for managing roles/permissions)
    api := rbac.NewAPI(r, &logger)
    api.RegisterRoutes(router.PathPrefix("/admin/rbac").Subrouter())

    // Create route registrar for auto-registration
    registrar := rbac.NewRouteRegistrar(r, &logger, router)

    // Register your application routes WITH NAMES
    router.HandleFunc("/users", getUsersHandler).
        Methods("GET").
        Name("admin.user.list")

    router.HandleFunc("/users", createUserHandler).
        Methods("POST").
        Name("admin.user.create")

    router.HandleFunc("/wallet/withdraw", withdrawHandler).
        Methods("POST").
        Name("user.wallet.withdraw")

    // Auto-register permissions from routes
    registrar.RegisterRoutes(context.Background())

    // Create user provider (extract user ID from request)
    userProvider := func(r *http.Request) (uint, error) {
        // Get from JWT, session, or header
        userIDStr := r.Header.Get("X-User-ID")
        userID, _ := strconv.ParseUint(userIDStr, 10, 32)
        return uint(userID), nil
    }

    // Add permission middleware
    router.Use(rbac.PermissionMiddleware(r, &logger, userProvider))

    // Start server
    http.ListenAndServe(":8080", router)
}
```

## Permission Format

### 2-Level Permissions
```
auth.login
auth.logout
auth.register
```

### 3-Level Permissions
```
admin.user.list
admin.user.create
admin.user.delete
user.wallet.view
user.wallet.withdraw
```

### Wildcards
```
admin.*          // Matches all admin permissions
admin.user.*     // Matches all admin.user permissions
user.wallet.*    // Matches all user.wallet permissions
```

## Default Roles

| Role | Slug | Description | Permissions |
|------|------|-------------|-------------|
| Root | `root` | Super admin - bypasses all checks | All (`*`) |
| User | `user` | Default role for new users | Basic user permissions |

## API Endpoints

### Roles
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/roles` | List all roles |
| POST | `/roles` | Create role |
| GET | `/roles/{id}` | Get role |
| PUT | `/roles/{id}` | Update role |
| DELETE | `/roles/{id}` | Delete role |
| PUT | `/roles/{id}/permissions` | Assign permissions to role |

### Permissions
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/permissions` | List all permissions |
| POST | `/permissions` | Create permission |
| GET | `/permissions/{id}` | Get permission |
| PUT | `/permissions/{id}` | Update permission |
| DELETE | `/permissions/{id}` | Delete permission |

### User Permissions
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/users/{id}/permissions` | Get user permissions |
| PUT | `/users/{id}/role` | Assign role to user |
| POST | `/users/{id}/check` | Check user permission |

## Usage Examples

### Create a Role
```bash
curl -X POST http://localhost:8080/admin/rbac/roles \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Admin",
    "slug": "admin",
    "description": "Administrator role",
    "is_default": false
  }'
```

### Create Permissions
```bash
curl -X POST http://localhost:8080/admin/rbac/permissions \
  -H "Content-Type: application/json" \
  -d '{
    "namespace": "admin",
    "resource": "user",
    "action": "list",
    "description": "List users"
  }'
```

### Assign Permissions to Role
```bash
curl -X PUT http://localhost:8080/admin/rbac/roles/2/permissions \
  -H "Content-Type: application/json" \
  -d '{
    "permission_ids": [1, 2, 3],
    "deny_ids": [4]
  }'
```

### Assign Role to User
```bash
curl -X PUT http://localhost:8080/admin/rbac/users/123/role \
  -H "Content-Type: application/json" \
  -d '{
    "role_id": 2
  }'
```

### Check Permission
```bash
curl -X POST http://localhost:8080/admin/rbac/users/123/check \
  -H "Content-Type: application/json" \
  -d '{
    "permission": "admin.user.list"
  }'
```

## Middleware Usage

### Permission-Based Middleware
```go
// Check permission based on route name
router.Use(rbac.PermissionMiddleware(rbac, logger, userProvider))

// Require specific permission
router.HandleFunc("/admin/dashboard", handler).
    Methods("GET").
    Name("admin.dashboard").
    Middleware(rbac.RequirePermission(rbac, logger, userProvider, "admin.dashboard"))

// Require specific role
router.HandleFunc("/admin/settings", handler).
    Methods("GET").
    Middleware(rbac.RequireRole(rbac, logger, userProvider, "admin"))

// Require any of multiple roles
router.HandleFunc("/premium/content", handler).
    Methods("GET").
    Middleware(rbac.RequireAnyRole(rbac, logger, userProvider, "admin", "premium"))
```

## Route Registration

```go
// Auto-register permissions from routes
registrar := rbac.NewRouteRegistrar(r, logger, router)
if err := registrar.RegisterRoutes(ctx); err != nil {
    log.Fatal(err)
}

// Sync permissions (remove orphaned, add new)
if err := registrar.SyncRoutePermissions(ctx); err != nil {
    log.Fatal(err)
}
```

## Database Models

```go
// Role
type Role struct {
    ID          uint
    Name        string  // "Admin"
    Slug        string  // "admin"
    Description string
    IsDefault   bool
    IsSystem    bool
    Permissions []Permission
}

// Permission
type Permission struct {
    ID          uint
    Name        string  // "admin.user.list"
    Namespace   string  // "admin"
    Resource    string  // "user"
    Action      string  // "list"
    Levels      int     // 3
    Description string
}

// RolePermission (many-to-many with deny support)
type RolePermission struct {
    RoleID       uint
    PermissionID uint
    IsDeny       bool  // Explicit deny
}

// User
type User struct {
    ID       uint
    RoleID   *uint
    Role     *Role
    Username string
    Email    string
}
```

## Caching

Permissions are automatically cached for performance:

- **Cache TTL**: Default 5 minutes
- **Invalidation**: Automatic when roles/permissions change
- **Clear Cache**: `rbac.ClearCache()`

```go
rbac := rbac.New(db, &rbac.Config{
    EnableCache: true,
    CacheTTL:    10 * time.Minute,
})
```

## Best Practices

1. **Name Routes Consistently**: Use `namespace.resource.action` format
2. **Use System Roles**: Mark important roles as `IsSystem` to prevent deletion
3. **Default Role**: Set one role as default for new users
4. **Explicit Deny**: Use deny permissions for exceptions
5. **Cache Wisely**: Adjust TTL based on your permission change frequency
6. **Audit Changes**: Log role/permission changes for security

## Package Structure

```
pkg/xcore/rbac/
├── models.go      # GORM models
├── rbac.go        # Core RBAC logic
├── cache.go       # Permission caching
├── middleware.go  # HTTP middleware
├── api.go         # CRUD API handlers
├── routes.go      # Route registration
└── README.md      # This file
```
