# Role-Based Access Control (RBAC) in GoFr

Role-Based Access Control (RBAC) is a security mechanism that restricts access to resources based on user roles and permissions. GoFr provides a pure config-based RBAC middleware that supports multiple authentication methods, fine-grained permissions, and role inheritance.

## Overview

- ✅ **Pure Config-Based** - All authorization rules in JSON/YAML files
- ✅ **Two-Level Authorization Model** - Roles define permissions, endpoints require permissions (no direct role-to-route mapping)
- ✅ **Multiple Auth Methods** - Header-based and JWT-based role extraction
- ✅ **Permission-Based** - Fine-grained permissions
- ✅ **Role Inheritance** - Roles inherit permissions from other roles

## Quick Start

```go
package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()
	
	// Use default paths (configs/rbac.json, configs/rbac.yaml, configs/rbac.yml)
	// Uses rbac.DefaultConfigPath internally (empty string triggers default path resolution)
	// Tries configs/rbac.json, then configs/rbac.yaml, then configs/rbac.yml
	app.EnableRBAC()
	
	// Or with custom config path
	app.EnableRBAC("configs/custom-rbac.json")
	
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
      "permissions": ["users:read", "users:write", "users:delete", "posts:read", "posts:write"]
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
      "requiredPermissions": ["users:read"]
    },
    {
      "path": "/api/users",
      "methods": ["POST"],
      "requiredPermissions": ["users:write"]
    }
  ]
}
```

> **💡 Best Practice**: For production/public APIs, use JWT-based RBAC instead of header-based RBAC for better security.


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

**Precedence**: If both are set, **only JWT is considered**. The header is not checked when `jwtClaimPath` is configured, even if JWT extraction fails.

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
      "permissions": ["users:read", "users:write", "users:delete", "posts:read", "posts:write"]  // Explicit permissions (wildcards not supported)
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
      "requiredPermissions": ["users:read"]
    },
    {
      "path": "/api/users/{id:[0-9]+}",  // Mux pattern with constraint (numeric IDs only)
      "methods": ["DELETE"],
      "requiredPermissions": ["users:delete"]
    },
    {
      "path": "/api/{resource}",  // Single-level pattern - matches /api/users, /api/posts
      "methods": ["GET"],
      "requiredPermissions": ["api:read"]
    },
    {
      "path": "/api/{path:.*}",  // Multi-level pattern - matches /api/users/123, /api/posts/comments
      "methods": ["*"],  // All methods
      "requiredPermissions": ["admin:read", "admin:write"]  // Multiple permissions (OR logic)
    },
    {
      "path": "/api/{category}/posts",  // Middle variable - matches /api/tech/posts, /api/news/posts
      "methods": ["GET"],
      "requiredPermissions": ["posts:read"]
    }
  ]
}
```

### Mux Pattern Syntax

RBAC uses **gorilla/mux route pattern conventions** for endpoint matching. This ensures perfect alignment with how routes are registered in GoFr.

**Important**: The RBAC middleware uses the same router configuration as GoFr's application router (`StrictSlash(false)`), ensuring consistent behavior for trailing slashes. This means `/api/users` and `/api/users/` are treated as the same route in both RBAC authorization checks and actual route matching.

**Pattern Types**:
- **Exact**: `"/api/users"` matches exactly `/api/users`
- **Single Variable**: `"/api/users/{id}"` matches `/api/users/123`, `/api/users/abc` (any single segment)
- **Variable with Constraint**: `"/api/users/{id:[0-9]+}"` matches `/api/users/123` (numeric IDs only)
- **Single-Level Pattern**: `"/api/{resource}"` matches `/api/users`, `/api/posts` (one segment)
- **Multi-Level Pattern**: `"/api/{path:.*}"` matches `/api/users/123`, `/api/posts/comments` (any depth)
- **Middle Variable**: `"/api/{category}/posts"` matches `/api/tech/posts`, `/api/news/posts`

**Common Patterns**:
- Numeric IDs: `"/api/users/{id:[0-9]+}"` (matches `/api/users/123`)
- UUIDs: `"/api/users/{uuid:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}"` (matches `/api/users/550e8400-e29b-41d4-a716-446655440000`)
- Alphanumeric: `"/api/users/{name:[a-zA-Z0-9]+}"` (matches `/api/users/user123`)

**Grouped Endpoints**:

For endpoints that need to match multiple paths, use mux patterns:

- **Single-level wildcard**: Use `"/api/{resource}"` instead of `"/api/*"`
    - Matches: `/api/users`, `/api/posts` (one segment)

- **Multi-level wildcard**: Use `"/api/{path:.*}"` instead of `"/api/*"`
    - Matches: `/api/users/123`, `/api/posts/comments` (any depth)

- **Middle variable**: Use `"/api/{category}/posts"` instead of `"/api/*/posts"`
    - Matches: `/api/tech/posts`, `/api/news/posts`

## JWT-Based RBAC

For production/public APIs, use JWT-based role extraction:

```go
app := gofr.New()

// Enable OAuth middleware first (required for JWT validation)
app.EnableOAuth("https://auth.example.com/.well-known/jwks.json", 10)

// Enable RBAC with config path (or use app.EnableRBAC() for default paths using rbac.DefaultConfigPath)
app.EnableRBAC("configs/rbac.json")
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
import (
	"encoding/json"
	
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/http"
)

// JWTClaims represents the JWT claims structure
type JWTClaims struct {
	Role string `json:"role"`
	Sub  string `json:"sub"`
	// Add other claim fields as needed
}

func handler(ctx *gofr.Context) (interface{}, error) {
	// Get JWT claims from context
	claimsMap := ctx.GetAuthInfo().GetClaims()
	if claimsMap == nil {
		return nil, http.ErrorInvalidParam{Params: []string{"authorization"}}
	}
	
	// Convert map claims to struct (recommended GoFr pattern)
	var claims JWTClaims
	claimsBytes, err := json.Marshal(claimsMap)
	if err != nil {
		return nil, http.ErrorInvalidParam{Params: []string{"claims"}}
	}
	
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, http.ErrorInvalidParam{Params: []string{"claims"}}
	}
	
	// Use role for business logic (e.g., personalize UI, filter data)
	// The role field matches the jwtClaimPath configured in rbac.json
	return map[string]string{"userRole": claims.Role}, nil
}
```

**Note**: All authorization is handled automatically by the middleware. Accessing the role in handlers is only for business logic purposes (e.g., personalizing UI, filtering data).


## Permission Naming Conventions

### Recommended Format

Use the format: `resource:action`

- **Resource**: The entity being accessed (e.g., `users`, `posts`, `orders`)
- **Action**: The operation being performed (e.g., `read`, `write`, `delete`, `update`)


### Examples:

```editorconfig
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

### Wildcards Not Supported

**Important**: Wildcards are **NOT supported** in permissions. Only exact matches are allowed.

- ❌ `"*:*"` - Does not match all permissions
- ❌ `"users:*"` - Does not match all user permissions
- ✅ `"users:read"` - Exact match only
- ✅ `"users:write"` - Exact match only

If you need multiple permissions, specify them explicitly:
```json
{
  "name": "admin",
  "permissions": ["users:read", "users:write", "users:delete", "posts:read", "posts:write"]
}
```

Or use role inheritance to avoid duplication:
```json
{
  "name": "editor",
  "permissions": ["users:write", "posts:write"],
  "inheritsFrom": ["viewer"]  // Inherits viewer's permissions
}
```

## Common Patterns

### CRUD Permissions

```json
{
  "roles": [
    {
      "name": "admin",
      "permissions": ["users:delete"],
      "inheritsFrom": ["editor"]
    },
    {
      "name": "editor",
      "permissions": ["users:create", "users:update"],
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
      "requiredPermissions": ["users:create"]
    },
    {
      "path": "/api/users",
      "methods": ["GET"],
      "requiredPermissions": ["users:read"]
    },
    {
      "path": "/api/users/{id:[0-9]+}",
      "methods": ["PUT", "PATCH"],
      "requiredPermissions": ["users:update"]
    },
    {
      "path": "/api/users/{id:[0-9]+}",
      "methods": ["DELETE"],
      "requiredPermissions": ["users:delete"]
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
      "requiredPermissions": ["own:posts:read"]
    },
    {
      "path": "/api/posts",
      "methods": ["GET"],
      "requiredPermissions": ["all:posts:read"]
    }
  ]
}
```

## Best Practices

### Security
- **Never use header-based RBAC for public APIs** - Use JWT-based RBAC
- **Always validate JWT tokens** - Use proper JWKS endpoints with HTTPS
- **Use HTTPS in production** - Protect tokens and headers
- **Monitor logs** - Track authorization decisions

### Configuration
- **Use role inheritance** - Avoid duplicating permissions (only specify additional ones)
- **Use consistent naming** - Follow `resource:action` format (e.g., `users:read`, `posts:write`)
- **Group related permissions** - Organize by resource type
- **Version control configs** - Track RBAC changes in git

## Troubleshooting

**Role not being extracted**
- Ensure `roleHeader` or `jwtClaimPath` is set in config file
- For header-based: check that the header is present in requests
- For JWT-based: ensure OAuth middleware is enabled before RBAC

**Permission checks failing**
- Verify `roles[].permissions` is properly configured
- Check that `endpoints[].requiredPermissions` matches your routes correctly
- Ensure role has the required permission (check inherited permissions too)
- Verify route pattern matches exactly (mux patterns supported)
- Check role inheritance - ensure inherited permissions are included

**Permission always denied**
- Check role assignment - verify user's role has the required permission
- Review role permissions - ensure `roles[].permissions` includes the required permission
- Enable debug logging - check debug logs for authorization decisions

**Permission always allowed**
- Check if endpoint is in RBAC config - routes not in config are allowed to proceed
- Check public endpoints - verify endpoint is not marked as `public: true`
- Review endpoint configuration - ensure `endpoints[].requiredPermissions` is set correctly
- Verify permission check - check logs to see if permission check is being performed

**JWT role extraction failing**
- Ensure OAuth middleware is enabled before RBAC
- Verify JWT claim path is correct

**Config file not found**
- Ensure config file exists at the specified path
- Or use default paths (`configs/rbac.json`, `configs/rbac.yaml`, `configs/rbac.yml`)

**Route not being protected by RBAC**
- Verify the route is explicitly configured in `endpoints[]` array
- Check that the path pattern matches exactly (case-sensitive)
- Ensure HTTP method matches (or use `["*"]` for all methods)
- Remember: Routes not in RBAC config are allowed to proceed (not blocked)

## How It Works

1. **Role Extraction**: Extracts user role from header (`X-User-Role`) or JWT claims
2. **Endpoint Matching**: Matches request method + path to endpoint configuration
3. **Permission Check**: Verifies role has required permission for the endpoint
4. **Authorization**: Allows or denies request based on permission check

The middleware automatically handles all authorization - you just define routes normally.

### Unmatched Routes Behavior

**Important**: RBAC only enforces authorization for endpoints that are **explicitly configured** in the RBAC config file.

- ✅ **Routes in RBAC config**: Authorization is enforced (requires valid role and permissions)
- ✅ **Routes NOT in RBAC config**: Requests are allowed to proceed to normal route matching
    - If the route exists in your application, it will be handled normally
    - If the route doesn't exist, it will return 404 (route not registered)

**Example**:
```json
{
  "endpoints": [
    {
      "path": "/api/users",
      "methods": ["GET"],
      "requiredPermissions": ["users:read"]
    }
  ]
}
```

In this configuration:
- `GET /api/users` → **RBAC enforced** (requires `users:read` permission)
- `POST /api/users` → **Not in RBAC config** → Allowed to proceed (may return 404 if route doesn't exist)
- `GET /api/posts` → **Not in RBAC config** → Allowed to proceed (may return 404 if route doesn't exist)
- `GET /health` → **Not in RBAC config** → Allowed to proceed (will work if route exists)

This design allows you to:
- Gradually add RBAC protection to specific endpoints
- Keep some routes unprotected (not in RBAC config)
- Let the router handle 404s for non-existent routes

## Security and Privacy

### Telemetry Data Protection

RBAC middleware implements industry-standard security practices to protect sensitive data:

**Traces (OpenTelemetry):**
- ✅ HTTP method and route patterns included
- ✅ Authorization status (allowed/denied) included
- ❌ Roles excluded (privacy protection - roles are PII)
- ❌ Error messages sanitized (prevent information leakage)

**Metrics:**
- ✅ Authorization decision counts included
- ✅ Status (allowed/denied) included
- ❌ Roles excluded (avoid high cardinality and PII concerns)

**Logs:**
- ✅ Roles included (required for compliance: SOC 2, PCI-DSS, NIST)
- ✅ HTTP method, route, status, and reason included
- ❌ No authorization tokens, headers, or request bodies logged
- ❌ No user IDs or personal information logged

### What's Never Logged

RBAC middleware never logs:
- Authorization tokens (Bearer tokens, API keys)
- Request bodies or headers
- User IDs or personal information
- IP addresses in traces/metrics
- Detailed error messages exposing internal details

## Related Documentation

- [HTTP Authentication](https://gofr.dev/docs/advanced-guide/http-authentication) - Basic Auth, API Keys, OAuth 2.0
- [HTTP Communication](https://gofr.dev/docs/advanced-guide/http-communication) - Inter-service HTTP calls
- [Middlewares](https://gofr.dev/docs/advanced-guide/middlewares) - Custom middleware implementation
