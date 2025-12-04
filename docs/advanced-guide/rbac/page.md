# Role-Based Access Control (RBAC) in GoFr

Role-Based Access Control (RBAC) is a security mechanism that restricts access to resources based on user roles and permissions. GoFr provides a pure config-based RBAC middleware that supports multiple authentication methods, fine-grained permissions, role inheritance, and hot reloading.

## Overview

- ✅ **Pure Config-Based** - All authorization rules in JSON/YAML files
- ✅ **Two-Level Mapping** - Role→Permission and Route&Method→Permission only
- ✅ **Multiple Auth Methods** - Header-based and JWT-based role extraction
- ✅ **Permission-Based** - Fine-grained permissions (`resource:action` format)
- ✅ **Role Inheritance** - Roles inherit permissions from other roles
- ✅ **Hot Reloading** - Update permissions without restarting

## Quick Start

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/rbac"
)

func main() {
	app := gofr.New()
	
	provider := rbac.NewProvider()
	app.EnableRBAC(provider, "configs/rbac.json") // or "" for default paths
	
	app.GET("/api/users", handler)
	app.Run()
}
```

**Configuration** (`configs/rbac.json`):

```json
{
  "roleHeader": "X-User-Role",
  "roles": [
    {
      "name": "admin",
      "permissions": ["*:*"]
    },
    {
      "name": "editor",
      "permissions": ["users:write", "posts:write"],
      "inheritsFrom": ["viewer"]
    },
    {
      "name": "viewer",
      "permissions": ["users:read", "posts:read"]
    }
  ],
  "endpoints": [
    {
      "path": "/health",
      "methods": ["GET"],
      "public": true
    },
    {
      "path": "/api/users",
      "methods": ["GET"],
      "requiredPermission": "users:read"
    },
    {
      "path": "/api/users",
      "methods": ["POST"],
      "requiredPermission": "users:write"
    }
  ]
}
```

> **⚠️ Security Note**: Header-based RBAC is **not secure** for public APIs. Use JWT-based RBAC for production.

## How It Works

1. **Role Extraction**: Extracts user role from header (`X-User-Role`) or JWT claims
2. **Endpoint Matching**: Matches request method + path to endpoint configuration
3. **Permission Check**: Verifies role has required permission for the endpoint
4. **Authorization**: Allows or denies request based on permission check

The middleware automatically handles all authorization - you just define routes normally.

## Configuration

### Role Extraction

**Header-Based** (for internal/trusted networks):
```json
{
  "roleHeader": "X-User-Role"
}
```

**JWT-Based** (for production/public APIs):
```json
{
  "jwtClaimPath": "role"  // or "roles[0]", "permissions.role", etc.
}
```

**Precedence**: If both are set, JWT takes precedence.

**JWT Claim Path Formats**:
- `"role"` → `{"role": "admin"}`
- `"roles[0]"` → `{"roles": ["admin", "user"]}` (first element)
- `"permissions.role"` → `{"permissions": {"role": "admin"}}`

### Roles and Permissions

```json
{
  "roles": [
    {
      "name": "admin",
      "permissions": ["*:*"]  // Wildcard: all permissions
    },
    {
      "name": "editor",
      "permissions": ["users:write", "posts:write"],  // Only additional permissions
      "inheritsFrom": ["viewer"]  // Inherits viewer's permissions
    },
    {
      "name": "viewer",
      "permissions": ["users:read", "posts:read"]
    }
  ]
}
```

**Note**: When using `inheritsFrom`, only specify additional permissions - inherited ones are automatically included.

### Endpoint Mapping

```json
{
  "endpoints": [
    {
      "path": "/health",
      "methods": ["GET"],
      "public": true  // Bypasses authorization
    },
    {
      "path": "/api/users",
      "methods": ["GET"],
      "requiredPermission": "users:read"
    },
    {
      "path": "/api/users/{id}",
      "methods": ["DELETE"],
      "regex": "^/api/users/\\d+$",  // Regex takes precedence over path
      "requiredPermission": "users:delete"
    },
    {
      "path": "/api/admin/*",  // Wildcard pattern
      "methods": ["*"],  // All methods
      "requiredPermission": "admin:*"
    }
  ]
}
```

**Route Patterns**:
- **Exact**: `"/api/users"` matches exactly `/api/users`
- **Wildcard**: `"/api/*"` matches `/api/users`, `/api/posts`, etc.
- **Regex**: `"^/api/users/\\d+$"` matches `/api/users/123`, etc.

**Hot Reload Configuration** (`configs/rbac.json`):

```json
{
  "roles": [...],
  "endpoints": [...],
  "hotReload": {
    "enabled": true,
    "intervalSeconds": 60
  }
}
```

**Hot Reload Interface**:

```go
type HotReloadSource interface {
    FetchConfig() ([]byte, error)
}
```

Implement this interface to fetch config from any source (Redis, database, HTTP service, etc.).

## Complete Example with Hot Reload

```go
package main

import (
  "context"
  "errors"

  "github.com/redis/go-redis/v9"
  "gofr.dev/pkg/gofr"
  "gofr.dev/pkg/gofr/container"
  "gofr.dev/pkg/gofr/rbac"
)

// RedisHotReloadSource implements HotReloadSource for Redis
type RedisHotReloadSource struct {
  redis container.Redis
  key   string
}

func (r *RedisHotReloadSource) FetchConfig() ([]byte, error) {
  val, err := r.redis.Get(context.Background(), r.key).Result()
  if err != nil {
    if errors.Is(err, redis.Nil) {
      return nil, errors.New("config not found in Redis")
    }
    return nil, err
  }
  return []byte(val), nil
}

func main() {
  app := gofr.New()

  provider := rbac.NewProvider()
  app.EnableRBAC(provider, "configs/rbac.json")

  // Configure hot reload in OnStart
  app.OnStart(func(ctx *gofr.Context) error {
    source := &RedisHotReloadSource{
      redis: ctx.Redis,
      key:   "rbac:config",
    }
    return provider.EnableHotReload(source)
  })

  app.GET("/api/users", func(ctx *gofr.Context) (interface{}, error) {
    // Role is already validated by middleware
    // For JWT-based: use ctx.GetAuthInfo().GetClaims()["role"]
    // For header-based: use ctx.Request.Header().Get("X-User-Role")
    claims := ctx.GetAuthInfo().GetClaims()
    role, _ := claims["role"].(string)
    return map[string]string{"userRole": role}, nil
  })

  app.Run()
}
```

## JWT-Based RBAC

For production/public APIs, use JWT-based role extraction:

```go
app := gofr.New()

// Enable OAuth middleware first (required for JWT validation)
app.EnableOAuth("https://auth.example.com/.well-known/jwks.json", 10)

provider := rbac.NewProvider()
app.EnableRBAC(provider, "configs/rbac.json")
```

**Configuration** (`configs/rbac.json`):

```json
{
  "jwtClaimPath": "role",  // or "roles[0]", "permissions.role", etc.
  "roles": [...],
  "endpoints": [...]
}
```

## Accessing Role in Handlers

For business logic, you can access the user's role from the request context:

**JWT-Based RBAC** (when using JWT role extraction):

```go
func handler(ctx *gofr.Context) (interface{}, error) {
	// Get JWT claims from context
	claims := ctx.GetAuthInfo().GetClaims()
	if claims == nil {
		return nil, errors.New("JWT claims not found")
	}
	
	// Extract role using the same claim path as configured in rbac.json
	// Example: if jwtClaimPath is "role"
	role, _ := claims["role"].(string)
	
	// Use role for business logic (e.g., personalize UI, filter data)
	return map[string]string{"userRole": role}, nil
}
```

**Note**: All authorization is handled automatically by the middleware. Accessing the role in handlers is only for business logic purposes (e.g., personalizing UI, filtering data).

## Permission Naming Conventions

### Recommended Format

Use the format: `resource:action`

- **Resource**: The entity being accessed (e.g., `users`, `posts`, `orders`)
- **Action**: The operation being performed (e.g., `read`, `write`, `delete`, `update`)

### Examples

```
"users:read"      // Read users
"users:write"     // Create/update users
"users:delete"    // Delete users
"posts:read"      // Read posts
"posts:write"     // Create/update posts
"orders:approve"  // Approve orders
"reports:export"  // Export reports
```

**Avoid inconsistent formats**:
- ❌ `"read_users"`, `"writeUsers"`, `"DELETE_POSTS"`
- ✅ `"users:read"`, `"users:write"`, `"posts:delete"`

## Common Patterns

### CRUD Permissions

```json
{
  "roles": [
    {
      "name": "admin",
      "permissions": ["users:create", "users:read", "users:update", "users:delete"]
    },
    {
      "name": "editor",
      "permissions": ["users:create", "users:read", "users:update"],
      "inheritsFrom": ["viewer"]
    },
    {
      "name": "viewer",
      "permissions": ["users:read"]
    }
  ],
  "endpoints": [
    {
      "path": "/api/users",
      "methods": ["POST"],
      "requiredPermission": "users:create"
    },
    {
      "path": "/api/users",
      "methods": ["GET"],
      "requiredPermission": "users:read"
    },
    {
      "path": "/api/users/{id}",
      "methods": ["PUT", "PATCH"],
      "regex": "^/api/users/\\d+$",
      "requiredPermission": "users:update"
    },
    {
      "path": "/api/users/{id}",
      "methods": ["DELETE"],
      "regex": "^/api/users/\\d+$",
      "requiredPermission": "users:delete"
    }
  ]
}
```

### Resource-Specific Permissions

```json
{
  "roles": [
    {
      "name": "admin",
      "permissions": ["own:posts:read", "own:posts:write", "all:posts:read", "all:posts:write"]
    },
    {
      "name": "author",
      "permissions": ["own:posts:read", "own:posts:write"]
    },
    {
      "name": "viewer",
      "permissions": ["own:posts:read", "all:posts:read"]
    }
  ],
  "endpoints": [
    {
      "path": "/api/posts/my-posts",
      "methods": ["GET"],
      "requiredPermission": "own:posts:read"
    },
    {
      "path": "/api/posts",
      "methods": ["GET"],
      "requiredPermission": "all:posts:read"
    }
  ]
}
```

## Best Practices

### Security
- **Never use header-based RBAC for public APIs** - Use JWT-based RBAC
- **Always validate JWT tokens** - Use proper JWKS endpoints with HTTPS
- **Use HTTPS in production** - Protect tokens and headers
- **Monitor audit logs** - Track authorization decisions

### Configuration
- **Use role inheritance** - Avoid duplicating permissions (only specify additional ones)
- **Use consistent naming** - Follow `resource:action` format (e.g., `users:read`, `posts:write`)
- **Group related permissions** - Organize by resource type
- **Version control configs** - Track RBAC changes in git
- **Set appropriate hot reload interval** - 60-300 seconds recommended

## Troubleshooting

**Role not being extracted**
- Ensure `roleHeader` or `jwtClaimPath` is set in config file
- For header-based: check that the header is present in requests
- For JWT-based: ensure OAuth middleware is enabled before RBAC

**Permission checks failing**
- Verify `roles[].permissions` is properly configured
- Check that `endpoints[].requiredPermission` matches your routes correctly
- Ensure role has the required permission (check inherited permissions too)
- Verify route pattern/regex matches exactly
- Check role inheritance - ensure inherited permissions are included

**Permission always denied**
- Check role assignment - verify user's role has the required permission
- Review role permissions - ensure `roles[].permissions` includes the required permission
- Enable debug logging - check audit logs for authorization decisions

**Permission always allowed**
- Check public endpoints - verify endpoint is not marked as `public: true`
- Review endpoint configuration - ensure `endpoints[].requiredPermission` is set correctly
- Verify permission check - check audit logs to see if permission check is being performed

**JWT role extraction failing**
- Ensure OAuth middleware is enabled before RBAC
- Verify JWT claim path is correct

**Config file not found**
- Ensure config file exists at the specified path
- Or use default paths (`configs/rbac.json`, `configs/rbac.yaml`, `configs/rbac.yml`)

## Related Documentation

- [HTTP Authentication](https://gofr.dev/docs/advanced-guide/http-authentication) - Basic Auth, API Keys, OAuth 2.0
- [HTTP Communication](https://gofr.dev/docs/advanced-guide/http-communication) - Inter-service HTTP calls
- [Middlewares](https://gofr.dev/docs/advanced-guide/middlewares) - Custom middleware implementation
