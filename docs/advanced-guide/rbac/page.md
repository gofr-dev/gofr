# Role-Based Access Control (RBAC) in GoFr

Role-Based Access Control (RBAC) is a security mechanism that restricts access to resources based on user roles and permissions. GoFr provides a comprehensive RBAC middleware that supports multiple authentication methods, fine-grained permissions, role hierarchies, and audit logging.

## Overview

GoFr's RBAC middleware provides:

- ✅ **Multiple Authentication Methods** - Header-based and JWT-based role extraction
- ✅ **Permission-Based Access Control** - Fine-grained permissions beyond simple roles
- ✅ **Role Hierarchy** - Inherited roles (admin > editor > author > viewer)
- ✅ **Audit Logging** - Comprehensive authorization logging using GoFr's logger
- ✅ **Framework Integration** - Simple API consistent with other GoFr features
- ✅ **Modular Design** - RBAC is an external module, keeping the core framework lightweight
- ✅ **Role-Centric Permissions** - Intuitive permission model where roles define what they can do

> **Important**: `app.EnableRBAC()` follows the same factory function pattern used throughout GoFr for datasources (e.g., `app.AddMongo()`, `app.AddPostgres()`). It automatically registers RBAC implementations when called. Simply call `app.EnableRBAC()` to enable RBAC features. When using RBAC options (e.g., `&rbac.JWTExtractor{}`), you must import the rbac package: `import "gofr.dev/pkg/gofr/rbac"`.

## Quick Start

GoFr's RBAC follows the same factory function pattern as datasource registration. Just like you use `app.AddMongo(db)` to register MongoDB, you use `app.EnableRBAC()` to register and configure RBAC.

### Basic RBAC with Header-Based Roles

The simplest way to implement RBAC is using header-based role extraction:

```go
package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Enable RBAC - uses default config path (configs/rbac.json)
	// Config file defines roleHeader: "X-User-Role" for automatic header extraction
	// EnableRBAC is a factory function that registers RBAC automatically
	app.EnableRBAC()

	app.GET("/api/users", handler)
	app.Run()
}
```

**Configuration** (`configs/rbac.json`):

```json
{
  "roleHeader": "X-User-Role",
  "route": {
    "/api/users": ["admin", "editor", "viewer"],
    "/api/admin/*": ["admin"],
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
  }
}
```

> **⚠️ Security Note**: Header-based RBAC is **not secure** for public APIs. Use JWT-based RBAC for production applications.

## RBAC Implementation Patterns

GoFr supports four main RBAC patterns, each suited for different use cases:

### 1. Simple RBAC (Header-Based)

**Best for**: Internal APIs, trusted networks, development environments

**Example**: [Simple RBAC Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/simple)

```go
import (
	"gofr.dev/pkg/gofr"
)

app := gofr.New()

// Enable RBAC with default config path
// Config file defines roleHeader for automatic header extraction
// EnableRBAC is a factory function that registers RBAC automatically
app.EnableRBAC() // Uses configs/rbac.json by default
```

**Configuration** (`configs/rbac.json`):

```json
{
  "roleHeader": "X-User-Role",
  "route": {
    "/api/users": ["admin", "editor", "viewer"],
    "/api/admin/*": ["admin"],
    "*": ["viewer"]
  },
  "defaultRole": "viewer",
  "roleHierarchy": {
    "admin": ["editor", "author", "viewer"]
  }
}
```

**Example**: [Simple RBAC Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/simple)

### 2. JWT-Based RBAC

**Best for**: Public APIs, microservices, OAuth2/OIDC integration

**Example**: [JWT RBAC Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/jwt)

```go
import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/rbac" // Import for JWTExtractor type
)

app := gofr.New()

// Enable OAuth middleware first (required for JWT validation)
app.EnableOAuth("https://auth.example.com/.well-known/jwks.json", 10)

// Enable RBAC with JWT role extraction
// EnableRBAC is a factory function that registers RBAC automatically
app.EnableRBAC("configs/rbac.json", &rbac.JWTExtractor{Claim: "role"})
```

**JWT Role Claim Parameter (`roleClaim`)**:

The `roleClaim` parameter in `JWTExtractor` specifies the path to the role in JWT claims. It supports multiple formats:

| Format | Example | JWT Claim Structure |
|--------|---------|---------------------|
| **Simple Key** | `"role"` | `{"role": "admin"}` |
| **Array Notation** | `"roles[0]"` | `{"roles": ["admin", "user"]}` - extracts first element |
| **Array Notation** | `"roles[1]"` | `{"roles": ["admin", "user"]}` - extracts second element |
| **Dot Notation** | `"permissions.role"` | `{"permissions": {"role": "admin"}}` |
| **Deeply Nested** | `"user.permissions.role"` | `{"user": {"permissions": {"role": "admin"}}}` |

**Examples**:

```go
// Simple claim
app.EnableRBAC("configs/rbac.json", &rbac.JWTExtractor{Claim: "role"})
// JWT: {"role": "admin", "sub": "user123"}

// Array notation - extract first role
app.EnableRBAC("configs/rbac.json", &rbac.JWTExtractor{Claim: "roles[0]"})
// JWT: {"roles": ["admin", "editor"], "sub": "user123"}

// Nested claim
app.EnableRBAC("configs/rbac.json", &rbac.JWTExtractor{Claim: "permissions.role"})
// JWT: {"permissions": {"role": "admin"}, "sub": "user123"}
```

**Note**: 
- If `roleClaim` is empty (`""`), it defaults to `"role"`
- The extracted value is converted to string automatically
- Array indices must be valid integers (e.g., `[0]`, `[1]`, not `[invalid]`)
- Array indices must be within bounds (e.g., `roles[5]` fails if array has only 2 elements)

**Example**: [JWT RBAC Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/jwt)

**Related**: [HTTP Authentication - OAuth 2.0](https://gofr.dev/docs/advanced-guide/http-authentication#3-oauth-20)

### 3. Permission-Based RBAC (Header)

**Best for**: Fine-grained access control with header-based roles

**Example**: [Permission-Based RBAC (Header) Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-header)

```go
import (
	"gofr.dev/pkg/gofr"
)

app := gofr.New()

// Enable RBAC with permissions
// Config file defines roleHeader and all permissions
// EnableRBAC is a factory function that registers RBAC automatically
app.EnableRBAC() // Uses configs/rbac.json by default
```

**Configuration** (`configs/rbac.json`):

```json
{
  "roleHeader": "X-User-Role",
  "route": {
    "/api/*": ["admin", "editor"]
  },
  "permissions": {
    "rolePermissions": {
      "admin": ["users:read", "users:write", "users:delete", "posts:read", "posts:write"],
      "editor": ["users:read", "users:write", "posts:read"],
      "viewer": ["users:read", "posts:read"]
    },
    "routePermissionRules": [
      {
        "methods": ["GET"],
        "regex": "^/api/users(/.*)?$",
        "permission": "users:read"
      },
      {
        "methods": ["POST", "PUT"],
        "regex": "^/api/users(/.*)?$",
        "permission": "users:write"
      },
      {
        "methods": ["DELETE"],
        "regex": "^/api/users/\\d+$",
        "permission": "users:delete"
      }
    ]
  }
}
```

**Example**: [Permission-Based RBAC (Header) Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-header)

### 4. Permission-Based RBAC (JWT)

**Best for**: Public APIs requiring fine-grained permissions

**Example**: [Permission-Based RBAC (JWT) Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-jwt)

```go
app := gofr.New()

// Enable OAuth middleware
app.EnableOAuth("https://auth.example.com/.well-known/jwks.json", 10)

// Enable RBAC with JWT and permissions
app.EnableRBAC("", &rbac.JWTExtractor{Claim: "role"})
// Uses default config path: configs/rbac.json
```

**Example**: [Permission-Based RBAC (JWT) Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-jwt)

**Related**: [HTTP Authentication - OAuth 2.0](https://gofr.dev/docs/advanced-guide/http-authentication#3-oauth-20)

## Factory Function Pattern

GoFr's RBAC follows the same factory function pattern used throughout the framework for datasource registration. This provides a consistent API and user experience.

### Options Pattern (Same as HTTP Service)

RBAC options follow the exact same pattern as HTTP service options (`service.Options`):

| Aspect | HTTP Service | RBAC |
|--------|--------------|------|
| **Interface** | `service.Options` | `rbac.Options` (internal) / `gofr.RBACOption` (public) |
| **Method** | `AddOption(h HTTP) HTTP` | `AddOption(config RBACConfig) RBACConfig` |
| **Usage** | `app.AddHTTPService(name, addr, options...)` | `app.EnableRBAC(configFile, options...)` |
| **Composable** | ✅ Yes - options can be chained | ✅ Yes - options can be chained |

**Example Comparison**:

```go
// HTTP Service Options
app.AddHTTPService("payment", "http://localhost:9000",
    &service.RateLimiterConfig{...},
    &service.CircuitBreakerConfig{...},
)

// RBAC Options (same pattern)
app.EnableRBAC("configs/rbac.json",
    &rbac.JWTExtractor{Claim: "role"},
    &rbac.HeaderRoleExtractor{HeaderKey: "X-User-Role"},
)
```

Both patterns use the same interface design where each option implements `AddOption` method, making them composable and order-agnostic.

### Comparison with Datasource Registration

Just like datasources, RBAC uses a factory function pattern:

| Feature | Datasources | RBAC |
|---------|-------------|------|
| **Factory Function** | `app.AddMongo(db)` | `app.EnableRBAC(configFile, options...)` |
| **Pattern** | Single entry point | Single entry point |
| **Setup** | Logger, Metrics, Tracer | Logger, Config, Middleware |
| **User Import** | Import datasource package | Import rbac package (when using options) |
| **Registration** | Direct assignment | Interface-based registration |

### Example Comparison

**Datasource Registration:**
```go
import (
    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/datasource/mongo"
)

app := gofr.New()
mongoDB := mongo.New(...)
app.AddMongo(mongoDB)  // Factory function - sets up logger, metrics, tracer, connects
```

**RBAC Registration:**
```go
import (
    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/rbac"  // Import when using options
)

app := gofr.New()
app.EnableRBAC("configs/rbac.json", &rbac.JWTExtractor{Claim: "role"})  // Factory function - registers, loads config, sets up middleware
```

Both follow the same pattern:
1. **Import the package** when using specific features/options
2. **Call the factory function** to register and configure
3. **Framework handles setup** automatically (logger, metrics, connection/setup)

## Configuration

### EnableRBAC API

The `EnableRBAC` function accepts an optional config file path and variadic options:

```go
func (a *App) EnableRBAC(configFile string, options ...RBACOption)
```

**Parameters**:
- `configFile` (string): Path to RBAC config file (JSON or YAML). If empty, tries default paths:
  - `configs/rbac.json`
  - `configs/rbac.yaml`
  - `configs/rbac.yml`
- `options` (...RBACOption): Optional interface-based options (follows same pattern as `service.Options`):
  - `&rbac.HeaderRoleExtractor{HeaderKey: "X-User-Role"}` - Header-based extraction
  - `&rbac.JWTExtractor{Claim: "role"}` - JWT-based extraction

**Options Pattern**: RBAC options follow the same pattern as HTTP service options (`service.Options`). Each option implements the `AddOption(config RBACConfig) RBACConfig` method, allowing for composable configuration similar to how HTTP services use `AddOption(h HTTP) HTTP`.

**Examples**:

```go
// Use default config path
app.EnableRBAC()

// Use custom config path
app.EnableRBAC("configs/custom-rbac.json")

// Use default path with JWT option
app.EnableRBAC("", &rbac.JWTExtractor{Claim: "role"})

// Use custom path with header extractor
app.EnableRBAC("configs/rbac.json", &rbac.HeaderRoleExtractor{HeaderKey: "X-User-Role"})
```

### JSON Configuration

```json
{
  "roleHeader": "X-User-Role",
  "route": {
    "/api/users": ["admin", "editor", "viewer"],
    "/api/posts": ["admin", "editor", "author"],
    "/api/admin/*": ["admin"],
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
    "rolePermissions": {
      "admin": ["users:read", "users:write", "users:delete"],
      "editor": ["users:read", "users:write"],
      "viewer": ["users:read"]
    },
    "routePermissionRules": [
      {
        "methods": ["GET"],
        "regex": "^/api/users(/.*)?$",
        "permission": "users:read"
      },
      {
        "methods": ["POST", "PUT"],
        "regex": "^/api/users(/.*)?$",
        "permission": "users:write"
      }
    ]
  }
}
```

### YAML Configuration

```yaml
roleHeader: X-User-Role

route:
  /api/users:
    - admin
    - editor
    - viewer
  /api/posts:
    - admin
    - editor
    - author

overrides:
  /health: true
  /metrics: true

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
  rolePermissions:
    admin:
      - users:read
      - users:write
      - users:delete
    editor:
      - users:read
      - users:write
    viewer:
      - users:read
  routePermissionRules:
    - methods: [GET]
      regex: "^/api/users(/.*)?$"
      permission: users:read
    - methods: [POST, PUT]
      regex: "^/api/users(/.*)?$"
      permission: users:write
```

## Configuration Fields

| Field | Type | Description |
|-------|------|-------------|
| `roleHeader` | `string` | HTTP header key for role extraction (e.g., "X-User-Role"). Auto-configures header extractor |
| `route` | `map[string][]string` | Maps route patterns to allowed roles. Supports wildcards (`*`, `/api/*`) |
| `overrides` | `map[string]bool` | Routes that bypass RBAC (public access) |
| `defaultRole` | `string` | ⚠️ Role used when no role can be extracted. **Security Warning**: Can be a security flaw if not carefully considered |
| `roleHierarchy` | `map[string][]string` | Defines role inheritance relationships |
| `permissions` | `object` | Permission-based access control configuration |
| `permissions.rolePermissions` | `map[string][]string` | **Role-centric**: Maps roles to their permissions (e.g., `"admin": ["users:read", "users:write"]`) |
| `permissions.routePermissionRules` | `array` | Structured route-to-permission mapping with regex support |

## Route Patterns

GoFr supports flexible route pattern matching:

- **Exact Match**: `"/api/users"` matches exactly `/api/users`
- **Wildcard**: `"/api/*"` matches `/api/users`, `/api/posts`, etc.
- **Global Fallback**: `"*"` matches all routes not explicitly defined

## Route Permission Rules

The new `routePermissionRules` format provides structured, flexible route-to-permission mapping:

```json
{
  "routePermissionRules": [
    {
      "methods": ["GET"],
      "regex": "^/api/users(/.*)?$",
      "permission": "users:read"
    },
    {
      "methods": ["POST", "PUT"],
      "path": "/api/users",
      "permission": "users:write"
    },
    {
      "methods": ["DELETE"],
      "regex": "^/api/users/\\d+$",
      "permission": "users:delete"
    }
  ]
}
```

**Fields**:
- `methods` (array): HTTP methods (GET, POST, PUT, DELETE, PATCH, etc.). Empty or `["*"]` matches all methods
- `path` (string): Path pattern (supports wildcards). Used when `regex` is not provided
- `regex` (string): Regular expression pattern. Takes precedence over `path` if both are provided
- `permission` (string): Required permission for matching routes

## Handler-Level Authorization

GoFr provides helper functions for handler-level authorization checks:

### Require Specific Role

```go
app.GET("/admin", gofr.RequireRole("admin", adminHandler))
```

### Require Any of Multiple Roles

```go
app.GET("/dashboard", gofr.RequireAnyRole([]string{"admin", "editor"}, dashboardHandler))
```

### Require Permission

```go
// Note: Middleware automatically checks permissions, but you can use this for programmatic checks
app.DELETE("/api/users/:id", gofr.RequirePermission("users:delete", config.PermissionConfig, deleteUser))
```

> **Note**: With the new API, middleware automatically checks permissions based on `routePermissionRules`. You typically don't need `RequirePermission()` at the route level unless you need programmatic checks within handlers.

## Context Helpers

Access role and permission information in your handlers:

```go
import "gofr.dev/pkg/gofr/rbac"

func handler(ctx *gofr.Context) (interface{}, error) {
	// Check if user has specific role
	if rbac.HasRole(ctx, "admin") {
		// Admin-only logic
	}

	// Get user's role
	role := rbac.GetUserRole(ctx)
	
	// Check permission
	if rbac.HasPermission(ctx.Context, "users:write", config.PermissionConfig) {
		// Permission-based logic
	}

	return nil, nil
}
```

## Advanced Features

### Custom Error Handler

Customize error responses for authorization failures:

```go
config.ErrorHandler = func(w http.ResponseWriter, r *http.Request, role, route string, err error) {
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(fmt.Sprintf("Access denied for role %s on %s", role, route)))
}
```

### Audit Logging

RBAC automatically logs all authorization decisions using GoFr's logger when `config.Logger` is set (which is done automatically by `app.EnableRBAC()`).

**Audit logs include:**
- Request method and path
- User role
- Route being accessed
- Authorization decision (allowed/denied)
- Reason for decision

**Example log output:**
```
[RBAC Audit] GET /api/users - Role: admin - Route: /api/users - allowed - Reason: permission-based
[RBAC Audit] GET /api/admin - Role: viewer - Route: /api/admin - denied - Reason: access denied
```

**No configuration needed** - audit logging works automatically when you enable RBAC. GoFr's logger is used internally to log all authorization decisions.

## Environment Variable Overrides

Override configuration at runtime using environment variables:

```bash
# Override default role
RBAC_DEFAULT_ROLE=viewer

# Override route permissions
RBAC_ROUTE_/api/users=admin,editor

# Override specific routes (public access)
RBAC_OVERRIDE_/health=true
```

## Comparison Matrix

| Feature | Simple | JWT | Permissions-Header | Permissions-JWT |
|---------|--------|-----|-------------------|----------------|
| **Security** | ⚠️ Low | ✅ High | ⚠️ Low | ✅ High |
| **Flexibility** | ⚠️ Low | ⚠️ Low | ✅ High | ✅ High |
| **Performance** | ✅ Fast | ✅ Fast | ✅ Fast | ✅ Fast |
| **Production Ready** | ❌ No | ✅ Yes | ❌ No | ✅ Yes |
| **Setup Complexity** | ✅ Simple | ⚠️ Medium | ⚠️ Medium | ⚠️ Medium |

## Migration Path

**Development → Production:**
1. Start with **Simple RBAC** for development
2. Move to **JWT RBAC** for production
3. Add **Permissions** when you need fine-grained control

## Complete Examples

All examples are available in the GoFr repository:

- [Simple RBAC Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/simple) - Header-based role extraction
- [JWT RBAC Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/jwt) - JWT-based role extraction
- [Permission-Based RBAC (Header)](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-header) - Permissions with header roles
- [Permission-Based RBAC (JWT)](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-jwt) - Permissions with JWT roles

Each example includes:
- Complete working code
- Configuration files
- Integration tests
- Setup instructions in code comments

## Best Practices

### 1. Security

1. **Never use header-based RBAC for public APIs** - Use JWT-based RBAC instead
2. **Always validate JWT tokens** - Use proper JWKS endpoints with HTTPS
3. **Use HTTPS in production** - Protect tokens and headers from interception
4. **Implement rate limiting** - Prevent abuse and brute force attacks
5. **Monitor audit logs** - Track authorization decisions for security analysis
6. **Avoid defaultRole in production** - Can be a security flaw if not carefully considered

### 2. Configuration

1. **Use role-centric permissions** - More intuitive than permission-centric model
   ```json
   {
     "rolePermissions": {
       "admin": ["users:read", "users:write", "users:delete"],
       "editor": ["users:read", "users:write"]
     }
   }
   ```
2. **Use structured route rules** - More flexible than string-based mapping
   ```json
   {
     "routePermissionRules": [
       {
         "methods": ["GET"],
         "regex": "^/api/users(/.*)?$",
         "permission": "users:read"
       }
     ]
   }
   ```
3. **Set roleHeader in config** - Auto-configures header extraction
4. **Use default config paths** - Simplifies configuration management

### 3. Permission Design

1. **Use consistent naming** - Follow `resource:action` format (e.g., `users:read`, `posts:write`)
2. **Group related permissions** - Organize by resource type
3. **Document permission requirements** - Comment which permissions are needed for each endpoint
4. **Test permission checks** - Write integration tests to verify authorization

### 4. Code Organization

1. **Let middleware handle checks** - Don't add `RequirePermission()` at route level unless needed for programmatic checks
2. **Use context helpers** - Access role/permission info in handlers when needed
3. **Keep configs in files** - Use JSON/YAML files for route-level config, code for fine-grained permissions
4. **Version control configs** - Track RBAC configuration changes separately from code

### 5. Performance

1. **Cache role lookups** - For high-traffic applications, consider caching roles
2. **Optimize route rules** - Use specific patterns before generic ones
3. **Monitor performance** - Track authorization decision times

### 6. Maintenance

1. **Regular security audits** - Review RBAC configurations periodically
2. **Use role hierarchy wisely** - Don't create overly complex hierarchies
3. **Document role meanings** - Clearly define what each role can do
4. **Keep examples updated** - Maintain working examples for reference

## Related Documentation

- [Permission-Based Access Control](./rbac-permissions/page.md) - Detailed permission documentation
- [HTTP Authentication](https://gofr.dev/docs/advanced-guide/http-authentication) - Basic Auth, API Keys, OAuth 2.0
- [HTTP Communication](https://gofr.dev/docs/advanced-guide/http-communication) - Inter-service HTTP calls
- [Middlewares](https://gofr.dev/docs/advanced-guide/middlewares) - Custom middleware implementation
- [Configuration](https://gofr.dev/docs/quick-start/configuration) - Environment variables and configuration management

## API Reference

### Framework Methods

- `app.EnableRBAC(configFile string, options ...RBACOption)` - Factory function that enables RBAC with config file and options (follows same pattern as `app.AddHTTPService()`)
  
  **Factory Function Behavior:**
  - Automatically registers RBAC implementations on first call
  - No need to import the RBAC module separately
  - Handles registration internally
  
  **Parameters:**
  - `configFile` (string): Optional path to RBAC config file. If empty, tries default paths:
    - `configs/rbac.json`
    - `configs/rbac.yaml`
    - `configs/rbac.yml`
  - `options` (...RBACOption): Optional interface-based options (follows same pattern as `service.Options`):
    - `&rbac.HeaderRoleExtractor{HeaderKey: "X-User-Role"}` - Header-based role extraction
    - `&rbac.JWTExtractor{Claim: "role"}` - JWT-based role extraction
  
  **Examples:**
  ```go
  // Use default config path
  app.EnableRBAC()
  
  // Use custom config path
  app.EnableRBAC("configs/custom-rbac.json")
  
  // Use default path with JWT option
  app.EnableRBAC("", &rbac.JWTExtractor{Claim: "role"})
  
  // Use custom path with header extractor
  app.EnableRBAC("configs/rbac.json", &rbac.HeaderRoleExtractor{HeaderKey: "X-User-Role"})
  ```

### Handler Helpers

- `gofr.RequireRole(role, handler)` - Require specific role
- `gofr.RequireAnyRole(roles, handler)` - Require any of multiple roles
- `gofr.RequirePermission(permission, config, handler)` - Require permission (for programmatic checks)

### Context Helpers

Access role and permission information in your handlers:

- `rbac.HasRole(ctx, role)` - Check if context has specific role
- `rbac.GetUserRole(ctx)` - Get user role from context
- `rbac.HasPermission(ctx, permission, config)` - Check if user has permission

## Troubleshooting

### Common Issues

**Issue**: Role not being extracted
- **Solution**: Ensure `roleHeader` is set in config or use `HeaderRoleExtractor` option. Check that the header is present in requests.

**Issue**: Permission checks failing
- **Solution**: Verify `rolePermissions` is properly configured and `routePermissionRules` match your routes correctly.

**Issue**: JWT role extraction failing
- **Solution**: Ensure OAuth middleware is enabled before RBAC, and JWT claim path is correct.

**Issue**: Config file not found
- **Solution**: Ensure config file exists at the specified path, or use default paths (`configs/rbac.json`, `configs/rbac.yaml`, `configs/rbac.yml`).

**Issue**: RBAC not working / "RBAC module not imported" error
- **Solution**: This should not happen with the factory function pattern. If it does, ensure you're calling `app.EnableRBAC()` and that the rbac package is available in your dependencies. The factory function handles registration automatically.

## Need Help?

- Check [RBAC Examples](https://github.com/gofr-dev/gofr/tree/main/examples/rbac) for complete working code
- See [GoFr Documentation](https://gofr.dev/docs) for general framework documentation
- Review [Permission-Based Access Control](./rbac-permissions/page.md) for detailed permission documentation
- See [RBAC Architecture](./rbac/ARCHITECTURE.md) for code execution flow
