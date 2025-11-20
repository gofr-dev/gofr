# Enhanced RBAC Middleware for GoFr

This package provides a comprehensive Role-Based Access Control (RBAC) middleware for GoFr applications with support for roles, permissions, hierarchy, caching, and audit logging.

## Features

- ✅ **Framework-Level Integration** - Simple API like other GoFr auth methods
- ✅ **JWT Integration** - Automatic role extraction from JWT claims
- ✅ **YAML & JSON Support** - Multiple configuration formats
- ✅ **Environment Variable Overrides** - Runtime configuration
- ✅ **Hot-Reload** - Automatic configuration reloading
- ✅ **Permission-Based Access Control** - Fine-grained permissions
- ✅ **Role Hierarchy** - Inherited roles (admin > editor > author)
- ✅ **Caching** - Performance optimization for role lookups
- ✅ **Audit Logging** - Comprehensive authorization logging

## Quick Start

### Basic RBAC

```go
package main

import (
    "net/http"
    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/rbac"
)

func main() {
    app := gofr.New()

    // Enable RBAC with custom role extractor
    app.EnableRBAC("configs/rbac.json", func(req *http.Request, args ...any) (string, error) {
        return req.Header.Get("X-User-Role"), nil
    })

    app.GET("/api/users", handler)
    app.Run()
}
```

### JWT-Based RBAC

```go
app := gofr.New()

// Enable OAuth first
app.EnableOAuth("https://auth.example.com/.well-known/jwks.json", 10)

// Enable RBAC with JWT role extraction
app.EnableRBACWithJWT("configs/rbac.json", "role") // "role" is the JWT claim path

app.Run()
```

### Permission-Based RBAC

```go
config, _ := rbac.LoadPermissions("configs/rbac.json")
config.PermissionConfig = &rbac.PermissionConfig{
    Permissions: map[string][]string{
        "users:read": ["admin", "editor", "viewer"],
        "users:write": ["admin", "editor"],
    },
    RoutePermissionMap: map[string]string{
        "GET /api/users": "users:read",
        "POST /api/users": "users:write",
    },
}

app.EnableRBACWithPermissions(config, roleExtractor)
```

## Configuration

### JSON Configuration

```json
{
  "route": {
    "/api/users": ["admin", "editor"],
    "/api/posts": ["admin", "editor", "author"],
    "*": ["viewer"]
  },
  "overrides": {
    "/health": true,
    "/metrics": true
  },
  "defaultRole": "viewer",
  "roleHierarchy": {
    "admin": ["editor", "author", "viewer"],
    "editor": ["author", "viewer"],
    "author": ["viewer"]
  },
  "permissions": {
    "permissions": {
      "users:read": ["admin", "editor", "viewer"],
      "users:write": ["admin", "editor"]
    },
    "routePermissions": {
      "GET /api/users": "users:read",
      "POST /api/users": "users:write"
    }
  },
  "enablePermissions": true
}
```

### YAML Configuration

```yaml
route:
  /api/users:
    - admin
    - editor
  /api/posts:
    - admin
    - editor
    - author

overrides:
  /health: true

defaultRole: viewer

roleHierarchy:
  admin:
    - editor
    - author
    - viewer
  editor:
    - author
    - viewer

permissions:
  permissions:
    users:read:
      - admin
      - editor
      - viewer
    users:write:
      - admin
      - editor
  routePermissions:
    "GET /api/users": users:read
    "POST /api/users": users:write

enablePermissions: true
```

## Environment Variables

```bash
# Override default role
RBAC_DEFAULT_ROLE=viewer

# Override route permissions
RBAC_ROUTE_/api/users=admin,editor

# Override specific routes (public access)
RBAC_OVERRIDE_/health=true
```

## Handler-Level Checks

```go
// Require specific role
app.GET("/admin", gofr.RequireRole("admin", handler))

// Require any of multiple roles
app.GET("/dashboard", gofr.RequireAnyRole([]string{"admin", "editor"}, handler))

// Require permission
app.GET("/users", gofr.RequirePermission("users:read", config.PermissionConfig, handler))
```

## Helper Functions

```go
// Check role in handler
if rbac.HasRole(ctx, "admin") {
    // Admin-only logic
}

// Get user role
role := rbac.GetUserRole(ctx)

// Check permission
if rbac.HasPermission(ctx.Context, "users:write", config.PermissionConfig) {
    // Permission-based logic
}
```

## Advanced Features

### Hot-Reload

```go
// Reload configuration every 30 seconds
app.EnableRBACWithHotReload("configs/rbac.yaml", roleExtractor, 30*time.Second)
```

### Custom Error Handler

```go
config.ErrorHandler = func(w http.ResponseWriter, r *http.Request, role, route string, err error) {
    // Custom error response
    w.WriteHeader(http.StatusForbidden)
    w.Write([]byte(fmt.Sprintf("Access denied for role %s on %s", role, route)))
}
```

### Custom Audit Logger

```go
type MyAuditLogger struct{}

func (l *MyAuditLogger) LogAccess(logger logging.Logger, req *http.Request, role, route string, allowed bool, reason string) {
    // Use GoFr's logger or send to external logging service
    if logger != nil {
        logger.Infof("[RBAC] %s %s - Role: %s - Allowed: %v", req.Method, req.URL.Path, role, allowed)
    }
    logToExternalService(req, role, route, allowed, reason)
}

config.AuditLogger = &MyAuditLogger{}
```

## Examples

See `gofr/examples/rbac/` for complete working examples.

## API Reference

### Framework Methods

- `app.EnableRBAC(permissionsFile, roleExtractor)` - Basic RBAC
- `app.EnableRBACWithJWT(permissionsFile, roleClaim)` - JWT-based RBAC
- `app.EnableRBACWithPermissions(config, roleExtractor)` - Permission-based RBAC
- `app.EnableRBACWithConfig(config)` - Full configuration
- `app.EnableRBACWithHotReload(permissionsFile, roleExtractor, interval)` - Hot-reload

### Helper Functions

- `gofr.RequireRole(role, handler)` - Require specific role
- `gofr.RequireAnyRole(roles, handler)` - Require any of multiple roles
- `gofr.RequirePermission(permission, config, handler)` - Require permission

### Context Helpers

- `rbac.HasRole(ctx, role)` - Check if context has role
- `rbac.GetUserRole(ctx)` - Get user role from context
- `rbac.HasPermission(ctx, permission, config)` - Check permission

## Migration from Old RBAC

The old RBAC API is still supported. To migrate:

**Old:**
```go
rbacConfigs, _ := rbac.LoadPermissions("config.json")
rbacConfigs.RoleExtractorFunc = extractor
app.UseMiddleware(rbac.Middleware(rbacConfigs))
```

**New:**
```go
app.EnableRBAC("config.json", extractor)
```

## See Also

- [Implementation Log](./IMPLEMENTATION_LOG.md) - Detailed implementation history
- [GoFr Documentation](https://gofr.dev/docs) - Complete GoFr documentation

