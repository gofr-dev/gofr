# Role-Based Access Control (RBAC) in GoFr

Role-Based Access Control (RBAC) is a security mechanism that restricts access to resources based on user roles and permissions. GoFr provides a comprehensive RBAC middleware that supports multiple authentication methods, fine-grained permissions, role hierarchies, and audit logging.

## Overview

GoFr's RBAC middleware provides:

- ✅ **Multiple Authentication Methods** - Header-based, JWT-based, and Database-based role extraction
- ✅ **Permission-Based Access Control** - Fine-grained permissions beyond simple roles
- ✅ **Role Hierarchy** - Inherited roles (admin > editor > author > viewer)
- ✅ **Audit Logging** - Comprehensive authorization logging using GoFr's logger
- ✅ **Framework Integration** - Simple API consistent with other GoFr features
- ✅ **Modular Design** - RBAC is an external module, keeping the core framework lightweight

> **Important**: To use RBAC features, you must import the RBAC module:
> ```go
> import _ "gofr.dev/pkg/gofr/rbac" // Import RBAC module for automatic registration
> ```
> The RBAC module uses automatic registration via package-level variable initialization. When imported, RBAC implementations are automatically registered with core GoFr, allowing you to use `app.EnableRBAC()` and related functions.

> **Note**: Container access is restricted for security. The container is only passed to `RoleExtractorFunc` when `config.RequiresContainer = true` (database-based role extraction). For header-based or JWT-based RBAC, `RequiresContainer = false` (default) and `args` will be empty.

## Quick Start

### Basic RBAC with Header-Based Roles

The simplest way to implement RBAC is using header-based role extraction:

```go
package main

import (
	"net/http"
	"gofr.dev/pkg/gofr"
	_ "gofr.dev/pkg/gofr/rbac" // Import RBAC module for automatic registration
)

func main() {
	app := gofr.New()

	// Enable RBAC with header-based role extraction
	// Note: args will be empty for header-based RBAC (container not needed)
	app.EnableRBAC(
		gofr.WithPermissionsFile("configs/rbac.json"),
		gofr.WithRoleExtractor(func(req *http.Request, args ...any) (string, error) {
			role := req.Header.Get("X-User-Role")
			if role == "" {
				return "", fmt.Errorf("role header not found")
			}
			return role, nil
		}),
	)

	app.GET("/api/users", handler)
	app.Run()
}
```

> **Note**: The RBAC module uses automatic registration via package-level variable initialization (not `init()`). When you import `gofr.dev/pkg/gofr/rbac`, the RBAC implementations are automatically registered with core GoFr. This allows the framework to remain lightweight - RBAC is only included when explicitly imported.

> **⚠️ Security Note**: Header-based RBAC is **not secure** for public APIs. Use JWT-based RBAC for production applications.

## RBAC Implementation Patterns

GoFr supports five main RBAC patterns, each suited for different use cases:

### 1. Simple RBAC (Header-Based)

**Best for**: Internal APIs, trusted networks, development environments

**Example**: [Simple RBAC Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/simple)

```go
import (
	"net/http"
	"gofr.dev/pkg/gofr"
	_ "gofr.dev/pkg/gofr/rbac" // Import RBAC module for automatic registration
)

app.EnableRBAC(
	gofr.WithPermissionsFile("configs/rbac.json"),
	gofr.WithRoleExtractor(func(req *http.Request, args ...any) (string, error) {
		return req.Header.Get("X-User-Role"), nil
	}),
)
```

**Configuration** (`configs/rbac.json`):

```json
{
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
	_ "gofr.dev/pkg/gofr/rbac" // Import RBAC module for automatic registration
)

app := gofr.New()

// Enable OAuth middleware first (required for JWT validation)
app.EnableOAuth("https://auth.example.com/.well-known/jwks.json", 10)

// Enable RBAC with JWT role extraction
app.EnableRBAC(
	gofr.WithPermissionsFile("configs/rbac.json"),
	gofr.WithJWT("role"),
)
```

**JWT Role Claim Parameter (`roleClaim`)**:

The `roleClaim` parameter in `WithJWT()` or `rbac.NewJWTRoleExtractor()` specifies the path to the role in JWT claims. It supports multiple formats:

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
app.EnableRBAC(
	gofr.WithPermissionsFile("configs/rbac.json"),
	gofr.WithJWT("role"),
)
// JWT: {"role": "admin", "sub": "user123"}

// Array notation - extract first role
app.EnableRBAC(
	gofr.WithPermissionsFile("configs/rbac.json"),
	gofr.WithJWT("roles[0]"),
)
// JWT: {"roles": ["admin", "editor"], "sub": "user123"}

// Nested claim
app.EnableRBAC(
	gofr.WithPermissionsFile("configs/rbac.json"),
	gofr.WithJWT("permissions.role"),
)
// JWT: {"permissions": {"role": "admin"}, "sub": "user123"}

// Deeply nested
app.EnableRBAC(
	gofr.WithPermissionsFile("configs/rbac.json"),
	gofr.WithJWT("user.permissions.role"),
)
// JWT: {"user": {"permissions": {"role": "admin"}}, "sub": "user123"}
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
	"fmt"
	"net/http"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/rbac"
)

func main() {
	app := gofr.New()

	// Load base configuration (provides fallback role-based auth and other settings)
	config, _ := rbac.LoadPermissions("configs/rbac.json")
	
	// Configure fine-grained permission-based access control
	config.PermissionConfig = &rbac.PermissionConfig{
		Permissions: map[string][]string{
			"users:read":   {"admin", "editor", "viewer"},
			"users:write":  {"admin", "editor"},
			"users:delete": {"admin"},
		},
		RoutePermissionMap: map[string]string{
			"GET /api/users":    "users:read",
			"POST /api/users":   "users:write",
			"DELETE /api/users": "users:delete",
		},
	}

	app.EnableRBAC(
		gofr.WithConfig(config),
		gofr.WithRoleExtractor(func(req *http.Request, args ...any) (string, error) {
			role := req.Header.Get("X-User-Role")
			if role == "" {
				return "", fmt.Errorf("role header not found")
			}
			return role, nil
		}),
		gofr.WithPermissions(config.PermissionConfig),
	)

	app.Run()
}
```

> **Note**: The config file provides fallback role-based authorization and other settings (overrides, defaultRole, etc.). Fine-grained permissions are defined in code. See [Permission-Based Access Control](https://gofr.dev/docs/advanced-guide/rbac-permissions) for comprehensive documentation.

**Example**: [Permission-Based RBAC (Header) Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-header)

### 4. Permission-Based RBAC (JWT)

**Best for**: Public APIs requiring fine-grained permissions

**Example**: [Permission-Based RBAC (JWT) Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-jwt)

```go
app := gofr.New()

// Enable OAuth middleware
app.EnableOAuth("https://auth.example.com/.well-known/jwks.json", 10)

config, _ := rbac.LoadPermissions("configs/rbac.json")
config.PermissionConfig = &rbac.PermissionConfig{
	Permissions: map[string][]string{
		"users:read": {"admin", "editor", "viewer"},
		"users:write": {"admin", "editor"},
	},
	RoutePermissionMap: map[string]string{
		"GET /api/users": "users:read",
		"POST /api/users": "users:write",
	},
}

	app.EnableRBAC(
		gofr.WithConfig(config),
		gofr.WithJWT("role"),
		gofr.WithPermissions(config.PermissionConfig),
	)
```

**Example**: [Permission-Based RBAC (JWT) Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-jwt)

**Related**: [HTTP Authentication - OAuth 2.0](https://gofr.dev/docs/advanced-guide/http-authentication#3-oauth-20)

### 5. Permission-Based RBAC (Database)

**Best for**: Dynamic roles, multi-tenant applications, admin-managed roles

**Example**: [Permission-Based RBAC (Database) Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-db)

**Important**: Database-based RBAC requires explicit container access. You must set `config.RequiresContainer = true` to enable container access.

```go
import (
	"database/sql"
	"fmt"
	"net/http"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/rbac"
)

// Load base configuration
config, err := rbac.LoadPermissions("configs/rbac.json")
if err != nil {
	app.Logger().Error("Failed to load RBAC config: ", err)
	return
}

// Configure permission-based access control
config.PermissionConfig = &rbac.PermissionConfig{
	Permissions: map[string][]string{
		"users:read":   {"admin", "editor", "viewer"},
		"users:write":  {"admin", "editor"},
		"users:delete": {"admin"},
	},
	RoutePermissionMap: map[string]string{
		"GET /api/users":    "users:read",
		"POST /api/users":   "users:write",
		"DELETE /api/users": "users:delete",
	},
}

// CRITICAL: Set RequiresContainer = true for database-based role extraction
// This enables container access in RoleExtractorFunc
config.RequiresContainer = true

// Database-based role extraction
// Extract user ID from header/token, then query database for role
config.RoleExtractorFunc = func(req *http.Request, args ...any) (string, error) {
	// Extract user ID from header (could be from JWT, session, etc.)
	userID := req.Header.Get("X-User-ID")
	if userID == "" {
		return "", fmt.Errorf("user ID not found in request")
	}

	// Get container from args (only available when RequiresContainer = true)
	// Container is automatically passed as args[0] when RequiresContainer = true
	if len(args) > 0 {
		if cntr, ok := args[0].(*container.Container); ok && cntr != nil && cntr.SQL != nil {
			// Use container.SQL.QueryRowContext() to query database
			var role string
			err := cntr.SQL.QueryRowContext(req.Context(), "SELECT role FROM users WHERE id = ?", userID).Scan(&role)
			if err != nil {
				if err == sql.ErrNoRows {
					return "", fmt.Errorf("user not found")
				}
				return "", err
			}
			return role, nil
		}
	}
	
	// Fallback if container is not available
	return "", fmt.Errorf("database not available")
}

// Enable RBAC with permissions
	app.EnableRBAC(
		gofr.WithConfig(config),
		gofr.WithRoleExtractor(config.RoleExtractorFunc),
		gofr.WithPermissions(config.PermissionConfig),
		gofr.WithRequiresContainer(true),
	)
```

**Container Access Control**:

- **`RequiresContainer = false`** (default): Container is **NOT** passed to `RoleExtractorFunc`
  - Used for: Header-based RBAC, JWT-based RBAC
  - `args` will be empty in `RoleExtractorFunc`
  - Container access is restricted for security

- **`RequiresContainer = true`**: Container **IS** passed to `RoleExtractorFunc`
  - Used for: Database-based RBAC
  - `args[0]` will be `*container.Container` in `RoleExtractorFunc`
  - Allows access to `container.SQL`, `container.Redis`, etc.

**Differentiating Between Extraction Types**:

| Extraction Type | RequiresContainer | Container Passed | args in RoleExtractorFunc |
|----------------|-------------------|------------------|---------------------------|
| Header-based | `false` | ❌ No | Empty |
| JWT-based | `false` | ❌ No | Empty |
| Database-based | `true` (must set) | ✅ Yes | `args[0] = *container.Container` |

**Security Note**: 
- Container access is restricted by default for security
- Only set `RequiresContainer = true` when you actually need database access
- Header/JWT-based extractors cannot access the container even if it's available

**Example**: [Permission-Based RBAC (Database) Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-db)

**Related**: [Connecting MySQL](https://gofr.dev/docs/quick-start/connecting-mysql)

## Configuration

### JSON Configuration

```json
{
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
    "permissions": {
      "users:read": ["admin", "editor", "viewer"],
      "users:write": ["admin", "editor"],
      "users:delete": ["admin"]
    },
    "routePermissions": {
      "GET /api/users": "users:read",
      "POST /api/users": "users:write",
      "DELETE /api/users": "users:delete"
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

## Configuration Fields

| Field | Type | Description |
|-------|------|-------------|
| `route` | `map[string][]string` | Maps route patterns to allowed roles. Supports wildcards (`*`, `/api/*`) |
| `overrides` | `map[string]bool` | Routes that bypass RBAC (public access) |
| `defaultRole` | `string` | Role used when no role can be extracted |
| `roleHierarchy` | `map[string][]string` | Defines role inheritance relationships |
| `permissions` | `object` | Permission-based access control configuration |
| `enablePermissions` | `boolean` | Enable permission-based checks |

## Route Patterns

GoFr supports flexible route pattern matching:

- **Exact Match**: `"/api/users"` matches exactly `/api/users`
- **Wildcard**: `"/api/*"` matches `/api/users`, `/api/posts`, etc.
- **Global Fallback**: `"*"` matches all routes not explicitly defined

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
app.DELETE("/api/users", gofr.RequirePermission("users:delete", config.PermissionConfig, deleteUser))
```

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

app.EnableRBAC(
	gofr.WithConfig(config),
)
```

### Audit Logging

RBAC automatically logs all authorization decisions using GoFr's logger when `config.Logger` is set (which is done automatically by `app.EnableRBAC*()` methods).

**Audit logs include:**
- Request method and path
- User role
- Route being accessed
- Authorization decision (allowed/denied)
- Reason for decision

**Example log output:**
```
[RBAC Audit] GET /api/users - Role: admin - Route: /api/users - allowed - Reason: role-based
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

| Feature | Simple | JWT | Permissions-Header | Permissions-JWT | Permissions-DB |
|---------|--------|-----|-------------------|----------------|----------------|
| **Security** | ⚠️ Low | ✅ High | ⚠️ Low | ✅ High | ✅ High |
| **Flexibility** | ⚠️ Low | ⚠️ Low | ✅ High | ✅ High | ✅✅ Very High |
| **Performance** | ✅ Fast | ✅ Fast | ✅ Fast | ✅ Fast | ⚠️ Slower* |
| **Dynamic Roles** | ❌ No | ❌ No | ❌ No | ❌ No | ✅ Yes |
| **Production Ready** | ❌ No | ✅ Yes | ❌ No | ✅ Yes | ✅ Yes |
| **Setup Complexity** | ✅ Simple | ⚠️ Medium | ⚠️ Medium | ⚠️ Medium | ⚠️ High |

*Consider implementing application-level caching (e.g., Redis) for database-based role extraction

## Migration Path

**Development → Production:**
1. Start with **Simple RBAC** for development
2. Move to **JWT RBAC** for production
3. Add **Permissions** when you need fine-grained control
4. Use **Database-based** when roles need to be dynamic

## Complete Examples

All examples are available in the GoFr repository:

- [Simple RBAC Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/simple) - Header-based role extraction
- [JWT RBAC Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/jwt) - JWT-based role extraction
- [Permission-Based RBAC (Header)](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-header) - Permissions with header roles
- [Permission-Based RBAC (JWT)](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-jwt) - Permissions with JWT roles
- [Permission-Based RBAC (Database)](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-db) - Permissions with database roles

Each example includes:
- Complete working code
- Configuration files
- Integration tests
- Setup instructions in code comments

## Security Best Practices

1. **Never use header-based RBAC for public APIs** - Use JWT-based RBAC instead
2. **Always validate JWT tokens** - Use proper JWKS endpoints with HTTPS
3. **Consider caching for DB-based roles** - Implement application-level caching (e.g., Redis) to reduce database load
4. **Use HTTPS in production** - Protect tokens and headers from interception
5. **Implement rate limiting** - Prevent abuse and brute force attacks
6. **Monitor audit logs** - Track authorization decisions for security analysis
7. **Use role hierarchy wisely** - Don't create overly complex hierarchies
8. **Regular security audits** - Review RBAC configurations periodically

## Understanding Permission Configuration

**Why load from file when defining permissions in code?**

The config file serves multiple purposes:
1. **Fallback Authorization** - `RouteWithPermissions` acts as fallback when permission mappings are missing
2. **Configuration Settings** - Provides overrides, defaultRole, roleHierarchy, and other settings
3. **Management Benefits** - Enables environment variable overrides and version control

See [Permission-Based Access Control](https://gofr.dev/docs/advanced-guide/rbac-permissions) for comprehensive documentation on permissions, including detailed explanations, examples, and best practices.

## Related Documentation

- [HTTP Authentication](https://gofr.dev/docs/advanced-guide/http-authentication) - Basic Auth, API Keys, OAuth 2.0
- [HTTP Communication](https://gofr.dev/docs/advanced-guide/http-communication) - Inter-service HTTP calls
- [Middlewares](https://gofr.dev/docs/advanced-guide/middlewares) - Custom middleware implementation
- [Connecting MySQL](https://gofr.dev/docs/quick-start/connecting-mysql) - Database setup for DB-based RBAC
- [Configuration](https://gofr.dev/docs/quick-start/configuration) - Environment variables and configuration management

## API Reference

### Framework Methods

- `app.EnableRBAC(options ...RBACOption)` - Unified RBAC configuration with options pattern
  
  **Available Options:**
  - `WithPermissionsFile(file string)` - Load RBAC config from a file
  - `WithRoleExtractor(extractor RoleExtractor)` - Set custom role extraction function
  - `WithConfig(config RBACConfig)` - Use a pre-loaded RBAC configuration
  - `WithJWT(roleClaim string)` - Enable JWT-based role extraction
  - `WithPermissions(permissionConfig PermissionConfig)` - Enable permission-based access control
  - `WithRequiresContainer(required bool)` - Indicate if container access is needed
  
  **Examples:**
  ```go
  // Basic RBAC (header-based)
  app.EnableRBAC(
      gofr.WithPermissionsFile("configs/rbac.json"),
      gofr.WithRoleExtractor(roleExtractor),
  )
  
  // JWT-based RBAC
  app.EnableRBAC(
      gofr.WithPermissionsFile("configs/rbac.json"),
      gofr.WithJWT("role"),
  )
  
  // Permission-based RBAC
  app.EnableRBAC(
      gofr.WithConfig(config),
      gofr.WithRoleExtractor(roleExtractor),
      gofr.WithPermissions(permissionConfig),
  )
  
  // Database-based (requires container)
  app.EnableRBAC(
      gofr.WithConfig(config),
      gofr.WithRoleExtractor(roleExtractor),
      gofr.WithRequiresContainer(true),
  )
  ```

### Handler Helpers

- `gofr.RequireRole(role, handler)` - Require specific role
- `gofr.RequireAnyRole(roles, handler)` - Require any of multiple roles
- `gofr.RequirePermission(permission, config, handler)` - Require permission

### Context Helpers

Access role and permission information in your handlers:

- `rbac.HasRole(ctx, role)` - Check if context has specific role
- `rbac.GetUserRole(ctx)` - Get user role from context
- `rbac.HasPermission(ctx, permission, config)` - Check if user has permission

## Troubleshooting

### Common Issues

**Issue**: Role not being extracted
- **Solution**: Ensure role extractor returns an error when role is missing, or set `defaultRole` in config

**Issue**: Permission checks failing
- **Solution**: Verify `enablePermissions` is set to `true` and `PermissionConfig` is properly configured

**Issue**: JWT role extraction failing
- **Solution**: Ensure OAuth middleware is enabled before RBAC, and JWT claim path is correct

**Issue**: Database-based roles slow
- **Solution**: Consider implementing application-level caching (e.g., Redis) for role lookups

## Need Help?

- Check [RBAC Examples](https://github.com/gofr-dev/gofr/tree/main/examples/rbac) for complete working code
- See [GoFr Documentation](https://gofr.dev/docs) for general framework documentation
- Review [Permission-Based Access Control](./rbac-permissions/page.md) for detailed permission documentation

