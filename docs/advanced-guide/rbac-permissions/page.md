# Permission-Based Access Control in GoFr

Permission-Based Access Control (PBAC) extends Role-Based Access Control (RBAC) by providing fine-grained authorization at the permission level. GoFr's RBAC is **purely permission-based** - all authorization is done through permissions, not direct role-to-route mapping.

## Overview

GoFr's permission-based RBAC provides:

- ✅ **Pure Config-Based** - All authorization rules defined in JSON/YAML files
- ✅ **Two-Level Mapping** - Role→Permission and Route&Method→Permission mapping only
- ✅ **Fine-Grained Control** - Define permissions like `users:read`, `users:write`, `users:delete`
- ✅ **Flexible Route Patterns** - Support for path patterns, wildcards, and regex
- ✅ **Role Inheritance** - Roles can inherit permissions from other roles
- ✅ **Automatic Route Protection** - Middleware automatically checks permissions - no route-level wrappers needed
- ✅ **Hot Reloading** - Update permissions without restarting (implement custom HotReloadSource interface)

> **Note**: `app.EnableRBAC(provider, configFile)` follows the same factory function pattern used throughout GoFr for datasource registration. Just like you use `app.AddMongo(db)` to register MongoDB, you use `app.EnableRBAC(provider, configFile)` to register and configure RBAC. You must import the rbac package: `import "gofr.dev/pkg/gofr/rbac"`.

## Quick Start

### Basic Permission Configuration

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/rbac"
)

func main() {
	app := gofr.New()

	// Create RBAC provider
	provider := rbac.NewProvider()

	// Enable RBAC with permissions
	// Config file defines all permissions and route mappings
	app.EnableRBAC(provider, "") // Uses configs/rbac.json by default

	// Example routes - permissions checked automatically by middleware
	app.GET("/api/users", getAllUsers)        // Auto-checked: users:read
	app.POST("/api/users", createUser)       // Auto-checked: users:write
	app.DELETE("/api/users/:id", deleteUser) // Auto-checked: users:delete

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
      "permissions": ["users:read", "users:write", "users:delete"]
    },
    {
      "name": "editor",
      "permissions": ["users:read", "users:write"],
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
      "methods": ["GET"],
      "requiredPermission": "users:read"
    },
    {
      "path": "/api/users",
      "methods": ["POST", "PUT"],
      "requiredPermission": "users:write"
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

## How Permissions Work

### Pure Permission-Based Authorization

GoFr's RBAC middleware implements a **pure permission-based authorization system**:

1. **Extract Role** - Get user's role from header or JWT
2. **Match Endpoint** - Find endpoint configuration for the request
3. **Get Required Permission** - Get permission required for the endpoint
4. **Check Role Permissions** - Verify if role has the required permission
5. **Authorization Decision** - Allow or deny based on permission check

**No fallback to role-based checks** - all authorization is permission-based for consistency and security.

### Permission Flow

1. **Request arrives** → Middleware extracts user role (from header or JWT)
2. **Match endpoint** → Find matching endpoint configuration (method + path)
3. **Get required permission** → Extract `requiredPermission` from endpoint config
4. **Check role permissions** → Verify if role's permissions include the required permission
5. **Authorization decision** → Allow if role has permission, deny otherwise

**Example Flow:**
```
Request: GET /api/users (Header: X-User-Role: editor)
  ↓
Extract role: "editor"
  ↓
Match endpoint: {path: "/api/users", methods: ["GET"], requiredPermission: "users:read"}
  ↓
Get role permissions: ["users:read", "users:write"] (includes inherited from viewer)
  ↓
Check permission: "users:read" in ["users:read", "users:write"] → ✅ Allowed
```

## Configuration

### Unified Configuration Format

GoFr uses a **unified configuration format** that combines roles and endpoints in a single, intuitive structure:

```json
{
  "roleHeader": "X-User-Role",
  "roles": [
    {
      "name": "admin",
      "permissions": ["users:read", "users:write", "users:delete"]
    },
    {
      "name": "editor",
      "permissions": ["users:read", "users:write"],
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
      "methods": ["GET"],
      "requiredPermission": "users:read"
    },
    {
      "path": "/api/users",
      "methods": ["POST", "PUT"],
      "requiredPermission": "users:write"
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

**Why Unified Format?**
- ✅ Single source of truth - all RBAC rules in one place
- ✅ Easy to understand - see roles and endpoints together
- ✅ Role inheritance - roles can inherit permissions from other roles
- ✅ Flexible route matching - supports path patterns, wildcards, and regex

### Role Definition

Roles define what permissions they have:

```json
{
  "roles": [
    {
      "name": "admin",
      "permissions": ["*:*"]  // Wildcard: all permissions
    },
    {
      "name": "editor",
      "permissions": ["users:read", "users:write"],
      "inheritsFrom": ["viewer"]  // Inherits viewer's permissions
    },
    {
      "name": "viewer",
      "permissions": ["users:read"]
    }
  ]
}
```

**Fields**:
- `name` (string, required): Role name
- `permissions` (array): List of permissions for this role (format: "resource:action")
- `inheritsFrom` (array): List of role names to inherit permissions from

### Endpoint Definition

Endpoints define which permission is required for each route:

```json
{
  "endpoints": [
    {
      "path": "/api/users",
      "methods": ["GET"],
      "requiredPermission": "users:read"
    },
    {
      "path": "/api/users",
      "methods": ["POST", "PUT"],
      "requiredPermission": "users:write"
    },
    {
      "path": "/api/users/{id}",
      "methods": ["DELETE"],
      "regex": "^/api/users/\\d+$",
      "requiredPermission": "users:delete"
    },
    {
      "path": "/health",
      "methods": ["GET"],
      "public": true  // Bypasses authorization
    }
  ]
}
```

**Fields**:
- `path` (string): Route path pattern (supports wildcards like `/api/*`)
- `regex` (string): Regular expression pattern (takes precedence over `path`)
- `methods` (array): HTTP methods (GET, POST, PUT, DELETE, etc.). Use `["*"]` for all methods
- `requiredPermission` (string): Required permission (format: "resource:action"). **Required** unless `public: true`
- `public` (boolean): If `true`, endpoint bypasses authorization

**Route Matching Priority**:
1. Exact match: `"/api/users"` matches exactly `/api/users`
2. Wildcard: `"/api/*"` matches `/api/users`, `/api/posts`, etc.
3. Regex: `"^/api/users/\\d+$"` matches `/api/users/123`, `/api/users/456`, etc.

## Permission Naming Conventions

### Recommended Format

Use the format: `resource:action`

- **Resource**: The entity being accessed (e.g., `users`, `posts`, `orders`)
- **Action**: The operation being performed (e.g., `read`, `write`, `delete`, `update`)

### Examples

```go
"users:read"      // Read users
"users:write"     // Create/update users
"users:delete"    // Delete users
"posts:read"      // Read posts
"posts:write"     // Create/update posts
"orders:approve"  // Approve orders
"reports:export"  // Export reports
```

## Automatic Route Protection

Middleware automatically checks permissions based on endpoint configuration. You **don't need** to add any wrappers at the route level:

```go
// ✅ Good: Middleware automatically checks permissions
app.GET("/api/users", getAllUsers)      // Checks: users:read
app.POST("/api/users", createUser)      // Checks: users:write
app.DELETE("/api/users/:id", deleteUser) // Checks: users:delete
```

**All authorization is handled by middleware** - you just define routes normally. The middleware:
1. Extracts the user's role
2. Matches the request to an endpoint configuration
3. Checks if the role has the required permission
4. Allows or denies the request automatically

## Implementation Patterns

### 1. Permission-Based RBAC (Header)

**Best for**: Fine-grained access control with header-based roles

**Example**: [Permission-Based RBAC (Header) Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-header)

```go
import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/rbac"
)

app := gofr.New()

// Create RBAC provider
provider := rbac.NewProvider()

// Enable RBAC - config file defines roleHeader and permissions
app.EnableRBAC(provider, "") // Uses configs/rbac.json by default
```

### 2. Permission-Based RBAC (JWT)

**Best for**: Public APIs requiring fine-grained permissions

**Example**: [Permission-Based RBAC (JWT) Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-jwt)

```go
import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/rbac"
)

app := gofr.New()

// Enable OAuth middleware first (required for JWT validation)
app.EnableOAuth("https://auth.example.com/.well-known/jwks.json", 10)

// Create RBAC provider
provider := rbac.NewProvider()

// Enable RBAC with JWT and permissions
// Config file sets: "jwtClaimPath": "role"
app.EnableRBAC(provider, "") // Uses default config path: configs/rbac.json
```

## Common Patterns

### Pattern 1: CRUD Permissions

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

### Pattern 2: Resource-Specific Permissions

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

### 1. Use Unified Configuration Format

```json
{
  "roles": [
    {
      "name": "admin",
      "permissions": ["users:read", "users:write", "users:delete"]
    },
    {
      "name": "editor",
      "permissions": ["users:read", "users:write"],
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
      "methods": ["GET"],
      "requiredPermission": "users:read"
    }
  ]
}
```

**Benefits**:
- Single source of truth - all RBAC rules in one place
- Easy to see what each role can do
- Role inheritance reduces duplication
- More maintainable

### 2. Use Flexible Route Patterns

```json
{
  "endpoints": [
    {
      "path": "/api/users",
      "methods": ["GET"],
      "requiredPermission": "users:read"
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

**Benefits**:
- Supports multiple HTTP methods per endpoint
- Flexible regex patterns for complex routes
- Path patterns with wildcards
- More readable and maintainable

### 3. Use Consistent Permission Naming

```go
// Good: Consistent format
"users:read"
"users:write"
"users:delete"
"posts:read"
"posts:write"

// Avoid: Inconsistent formats
"read_users"
"writeUsers"
"DELETE_POSTS"
```

### 4. Group Related Permissions

```json
{
  "roles": [
    {
      "name": "admin",
      "permissions": [
        "users:read", "users:write", "users:delete",
        "posts:read", "posts:write", "posts:delete",
        "orders:read", "orders:approve", "orders:cancel"
      ]
    }
  ]
}
```

### 5. Use Role Inheritance

Avoid duplicating permissions by using role inheritance:

```json
{
  "roles": [
    {
      "name": "editor",
      "permissions": ["users:write", "posts:write"],
      "inheritsFrom": ["viewer"]  // Inherits viewer's read permissions
    },
    {
      "name": "viewer",
      "permissions": ["users:read", "posts:read"]
    }
  ]
}
```

This ensures roles automatically get inherited permissions without duplication.

### 6. Document Permission Requirements

Document which permissions are required for each endpoint:

```go
// GET /api/users - Requires: users:read
// POST /api/users - Requires: users:write
// DELETE /api/users/:id - Requires: users:delete
```

### 7. Test Permission Checks

Write integration tests to verify permission checks:

```go
func TestPermissionChecks(t *testing.T) {
    // Test that admin can delete users
    // Test that editor cannot delete users
    // Test that viewer cannot write users
}
```

### 8. Let Middleware Handle Checks

Don't add `RequirePermission()` at route level unless needed for programmatic checks:

```go
// ✅ Good: Middleware automatically checks
app.DELETE("/api/users/:id", deleteUser)

// ❌ Not needed: Middleware already checks
// app.DELETE("/api/users/:id", gofr.RequirePermission("users:delete", config, deleteUser))
```

## Troubleshooting

### Permission Check Not Working

1. **Verify `roles[].permissions` is configured** - Check that roles have permissions assigned
2. **Check endpoint configuration** - Ensure `endpoints[].requiredPermission` matches your routes correctly
3. **Verify role extraction** - Ensure role is being extracted correctly (check `roleHeader` or `jwtClaimPath`)
4. **Check permission mapping** - Ensure route matches an endpoint configuration
5. **Review role inheritance** - Check if inherited permissions are correct

### Permission Always Denied

1. **Check role assignment** - Verify user's role has the required permission
2. **Review role permissions** - Ensure `roles[].permissions` includes the required permission
3. **Check route matching** - Verify route pattern/regex matches exactly
4. **Enable debug logging** - Check audit logs for authorization decisions
5. **Check role inheritance** - Verify inherited permissions are included

### Permission Always Allowed

1. **Check public endpoints** - Verify endpoint is not marked as `public: true`
2. **Review endpoint configuration** - Ensure `endpoints[].requiredPermission` is set correctly
3. **Check route matching** - Ensure route matches the correct endpoint configuration
4. **Verify permission check** - Check audit logs to see if permission check is being performed

## Hot Reloading

RBAC supports hot reloading of configuration without restarting the application. To enable hot reload, implement the `gofr.HotReloadSource` interface:

```go
// HotReloadSource interface
type HotReloadSource interface {
    // FetchConfig fetches the updated RBAC configuration
    // Returns the config data (JSON or YAML bytes) and error
    FetchConfig() ([]byte, error)
}
```

**Example: Custom Hot Reload Implementation**

```go
package main

import (
    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/rbac"
)

// Your custom hot reload source
type CustomHotReloadSource struct {
    // Your implementation details
}

func (c *CustomHotReloadSource) FetchConfig() ([]byte, error) {
    // Fetch config from your source (database, file, API, etc.)
    // Return JSON or YAML bytes
    return []byte(`{"roles":[...],"endpoints":[...]}`), nil
}

func main() {
    app := gofr.New()
    
    provider := rbac.NewProvider()
    app.EnableRBAC(provider, "configs/rbac.json")
    
    // Configure hot reload in OnStart
    app.OnStart(func(ctx *gofr.Context) error {
        source := &CustomHotReloadSource{}
        return provider.EnableHotReload(source)
    })
    
    app.Run()
}
```

**Configuration** (in `configs/rbac.json`):

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

## Related Documentation

- [Role-Based Access Control (RBAC)](./rbac/page.md) - Complete RBAC guide
- [HTTP Authentication](https://gofr.dev/docs/advanced-guide/http-authentication) - Authentication methods
- [Permission-Based RBAC Examples](https://github.com/gofr-dev/gofr/tree/main/examples/rbac) - Working examples

## Examples

- [Permission-Based RBAC (Header)](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-header) - Header-based role extraction with permissions
- [Permission-Based RBAC (JWT)](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-jwt) - JWT-based role extraction with permissions

Each example includes:
- Complete working code
- Configuration files
- Integration tests
- Detailed README with setup instructions
