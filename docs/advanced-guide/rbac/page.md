# Role-Based Access Control (RBAC) in GoFr

Role-Based Access Control (RBAC) is a security mechanism that restricts access to resources based on user roles and permissions. GoFr provides a pure config-based RBAC middleware that supports multiple authentication methods, fine-grained permissions, role inheritance, hot reloading, and audit logging.

## Overview

GoFr's RBAC middleware provides:

- ✅ **Pure Config-Based** - All authorization rules defined in JSON/YAML files
- ✅ **Two-Level Mapping** - Role→Permission and Route&Method→Permission mapping only
- ✅ **Multiple Authentication Methods** - Header-based and JWT-based role extraction
- ✅ **Permission-Based Access Control** - Fine-grained permissions (format: `resource:action`)
- ✅ **Role Inheritance** - Roles can inherit permissions from other roles
- ✅ **Hot Reloading** - Update permissions without restarting (Redis/HTTP service support)
- ✅ **Audit Logging** - Comprehensive authorization logging using GoFr's logger

## Quick Start

GoFr's RBAC follows the same factory function pattern as datasource registration. Just like you use `app.AddMongo(db)` to register MongoDB, you use `app.EnableRBAC(provider, configFile)` to register and configure RBAC.

### Basic RBAC with Header-Based Roles

The simplest way to implement RBAC is using header-based role extraction:

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

	// Enable RBAC - uses default config path (configs/rbac.json)
	// Config file defines roleHeader: "X-User-Role" for automatic header extraction
	app.EnableRBAC(provider, "") // Empty string uses default paths

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
      "permissions": ["users:read", "users:write", "posts:read", "posts:write"],
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
    },
    {
      "path": "/api/admin/*",
      "methods": ["*"],
      "requiredPermission": "admin:*"
    }
  ]
}
```

> **⚠️ Security Note**: Header-based RBAC is **not secure** for public APIs. Use JWT-based RBAC for production applications.

## Complete Flow: How RBAC Works

Understanding the complete flow helps you configure and use RBAC effectively. Here's the step-by-step process from a user's perspective:

### Step 1: Create Configuration File

Create a JSON or YAML file (e.g., `configs/rbac.json`) that defines:
- **Roles**: What permissions each role has
- **Endpoints**: Which permission is required for each route
- **Role Extraction**: How to extract the user's role (header or JWT)

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
      "methods": ["POST"],
      "requiredPermission": "users:write"
    }
  ]
}
```

### Step 2: Initialize RBAC in Your Application

In your `main.go`, create a provider and enable RBAC:

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/rbac"
)

func main() {
	app := gofr.New()

	// Step 2a: Create RBAC provider
	provider := rbac.NewProvider()

	// Step 2b: Enable RBAC (loads config and registers middleware)
	app.EnableRBAC(provider, "configs/rbac.json")
	// Or use default path:
	// app.EnableRBAC(provider, "") // Tries configs/rbac.json, configs/rbac.yaml, configs/rbac.yml

	// Step 2c: Define your routes (authorization is automatic)
	app.GET("/api/users", getAllUsers)
	app.POST("/api/users", createUser)

	app.Run()
}
```

**What happens during `EnableRBAC()`:**
1. **Loads config file** - Reads and parses your JSON/YAML configuration
2. **Processes unified config** - Builds in-memory maps:
   - `rolePermissionsMap`: Maps each role to its permissions (including inherited)
   - `endpointPermissionMap`: Maps "METHOD:/path" to required permission
   - `publicEndpointsMap`: Tracks which endpoints are public
3. **Registers middleware** - Adds RBAC middleware to the HTTP router
4. **Logs initialization** - Confirms RBAC is loaded and ready

### Step 3: Request Flow (What Happens Per Request)

When a request arrives, the RBAC middleware automatically:

1. **Extract Role**:
   - If `jwtClaimPath` is set: Extract role from JWT claims in request context
   - Else if `roleHeader` is set: Extract role from HTTP header (e.g., `X-User-Role: admin`)
   - If no role found: Return `401 Unauthorized`

2. **Match Endpoint**:
   - Find matching endpoint configuration for the request (method + path)
   - Check if endpoint is public (bypasses authorization)
   - If no match found: Return `403 Forbidden` (fail secure)

3. **Check Authorization**:
   - Get required permission from endpoint configuration
   - Get user's role permissions from `rolePermissionsMap`
   - Check if role has the required permission (supports wildcards like `users:*`)
   - If authorized: Continue to handler
   - If not authorized: Return `403 Forbidden`

4. **Store Role in Context**:
   - Store the user's role in request context for use in handlers
   - Your handlers can access it via `rbac.GetUserRole(ctx)`

5. **Audit Logging**:
   - Automatically logs authorization decisions using GoFr's logger
   - Logs include: method, path, role, decision (allowed/denied), reason

### Step 4: Optional - Configure Hot Reload

If you want to update permissions without restarting, configure hot reload:

```go
app.OnStart(func(ctx *gofr.Context) error {
	// Configure hot reload source (Redis or HTTP service)
	source := hotreload.NewRedisSource(ctx.Redis, "rbac:config")
	return provider.EnableHotReload(source)
})
```

**What happens during hot reload:**
1. Periodically fetches updated config from Redis/HTTP service
2. Validates and parses the new config
3. Atomically updates in-memory maps (thread-safe)
4. New requests immediately use the updated permissions

### Complete Example: End-to-End Flow

Here's a complete example showing the full flow:

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/rbac"
	"gofr.dev/pkg/gofr/rbac/hotreload"
)

func main() {
	app := gofr.New()

	// 1. Create provider
	provider := rbac.NewProvider()

	// 2. Enable RBAC (loads config, builds maps, registers middleware)
	app.EnableRBAC(provider, "configs/rbac.json")

	// 3. Optional: Configure hot reload
	app.OnStart(func(ctx *gofr.Context) error {
		if ctx.Redis != nil {
			source := hotreload.NewRedisSource(ctx.Redis, "rbac:config")
			return provider.EnableHotReload(source)
		}
		return nil
	})

	// 4. Define routes (authorization is automatic)
	app.GET("/api/users", getAllUsers)      // Requires: users:read
	app.POST("/api/users", createUser)      // Requires: users:write
	app.DELETE("/api/users/:id", deleteUser) // Requires: users:delete

	app.Run()
}

func getAllUsers(ctx *gofr.Context) (interface{}, error) {
	// Role is already validated by middleware
	// You can access it for business logic if needed
	role := rbac.GetUserRole(ctx)
	
	// Your business logic here
	return []string{"user1", "user2"}, nil
}
```

### Key Points

1. **Pure Config-Based**: All authorization rules are in the config file - no code changes needed
2. **Automatic**: Middleware handles everything - you just define routes normally
3. **Two-Level Mapping**: Role→Permission and Route→Permission (no direct route→role mapping)
4. **Thread-Safe**: Hot reload updates maps atomically without blocking requests
5. **Audit Logging**: All authorization decisions are automatically logged

## RBAC Implementation Patterns

GoFr supports four main RBAC patterns, each suited for different use cases:

### 1. Simple RBAC (Header-Based)

**Best for**: Internal APIs, trusted networks, development environments

**Example**: [Simple RBAC Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/simple)

```go
import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/rbac"
)

app := gofr.New()

// Create RBAC provider
provider := rbac.NewProvider()

// Enable RBAC with default config path
// Config file defines roleHeader for automatic header extraction
app.EnableRBAC(provider, "") // Uses configs/rbac.json by default
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
      "methods": ["POST"],
      "requiredPermission": "users:write"
    },
    {
      "path": "/api/admin/*",
      "methods": ["*"],
      "requiredPermission": "admin:*"
    }
  ]
}
```

**Example**: [Simple RBAC Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/simple)

### 2. JWT-Based RBAC

**Best for**: Public APIs, microservices, OAuth2/OIDC integration

**Example**: [JWT RBAC Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/jwt)

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

// Enable RBAC - JWT role extraction configured in config file
// Config file sets: "jwtClaimPath": "role"
app.EnableRBAC(provider, "configs/rbac.json")
```

**JWT Claim Path (`jwtClaimPath`)**:

The `jwtClaimPath` in the config file specifies the path to the role in JWT claims. It supports multiple formats:

| Format | Example | JWT Claim Structure |
|--------|---------|---------------------|
| **Simple Key** | `"role"` | `{"role": "admin"}` |
| **Array Notation** | `"roles[0]"` | `{"roles": ["admin", "user"]}` - extracts first element |
| **Array Notation** | `"roles[1]"` | `{"roles": ["admin", "user"]}` - extracts second element |
| **Dot Notation** | `"permissions.role"` | `{"permissions": {"role": "admin"}}` |
| **Deeply Nested** | `"user.permissions.role"` | `{"user": {"permissions": {"role": "admin"}}}` |

**Config File Examples**:

```json
{
  "jwtClaimPath": "role",
  "roles": [...],
  "endpoints": [...]
}
```
// JWT: {"role": "admin", "sub": "user123"}

```json
{
  "jwtClaimPath": "roles[0]",
  "roles": [...],
  "endpoints": [...]
}
```
// JWT: {"roles": ["admin", "editor"], "sub": "user123"}

```json
{
  "jwtClaimPath": "permissions.role",
  "roles": [...],
  "endpoints": [...]
}
```
// JWT: {"permissions": {"role": "admin"}, "sub": "user123"}

**Notes**: 
- The extracted value is converted to string automatically
- Array indices must be valid integers (e.g., `[0]`, `[1]`, not `[invalid]`)
- Array indices must be within bounds (e.g., `roles[5]` fails if array has only 2 elements)
- **Precedence**: If both `roleHeader` and `jwtClaimPath` are set, JWT takes precedence

**Example**: [JWT RBAC Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/jwt)

**Related**: [HTTP Authentication - OAuth 2.0](https://gofr.dev/docs/advanced-guide/http-authentication#3-oauth-20)

### 3. Permission-Based RBAC (Header)

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

// Enable RBAC with permissions
// Config file defines roleHeader and all permissions
app.EnableRBAC(provider, "") // Uses configs/rbac.json by default
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
      "permissions": ["users:read", "users:write", "posts:read"],
      "inheritsFrom": ["viewer"]
    },
    {
      "name": "viewer",
      "permissions": ["users:read", "posts:read"]
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

**Example**: [Permission-Based RBAC (Header) Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-header)

### 4. Permission-Based RBAC (JWT)

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

**Example**: [Permission-Based RBAC (JWT) Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-jwt)

**Related**: [HTTP Authentication - OAuth 2.0](https://gofr.dev/docs/advanced-guide/http-authentication#3-oauth-20)

## Configuration

### EnableRBAC API

The `EnableRBAC` function accepts a provider and an optional config file path:

```go
func (a *App) EnableRBAC(provider gofr.RBACProvider, configFile string)
```

**Parameters**:
- `provider` (gofr.RBACProvider): RBAC provider instance. Create using `rbac.NewProvider()`
- `configFile` (string): Path to RBAC config file (JSON or YAML). If empty, tries default paths:
  - `configs/rbac.json`
  - `configs/rbac.yaml`
  - `configs/rbac.yml`

**Role Extraction Configuration**:
Role extraction is configured **entirely in the config file** - no options needed:
- Set `roleHeader` for header-based extraction (e.g., `"X-User-Role"`)
- Set `jwtClaimPath` for JWT-based extraction (e.g., `"role"`, `"roles[0]"`)

**Precedence**: If both `roleHeader` and `jwtClaimPath` are set, **JWT takes precedence** (JWT is more secure).

**Examples**:

```go
import (
    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/rbac"
)

app := gofr.New()
provider := rbac.NewProvider()

// Use default config path (configs/rbac.json)
app.EnableRBAC(provider, "")

// Use custom config path
app.EnableRBAC(provider, "configs/custom-rbac.json")
```

**Config File Example**:
```json
{
  "roleHeader": "X-User-Role",        // Header-based (used if jwtClaimPath not set)
  "jwtClaimPath": "role",              // JWT-based (takes precedence if both set)
  "roles": [...],
  "endpoints": [...]
}
```

### JSON Configuration

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
      "permissions": ["users:read", "users:write", "posts:read", "posts:write"],
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
      "methods": ["POST", "PUT"],
      "requiredPermission": "users:write"
    },
    {
      "path": "/api/users/{id}",
      "methods": ["DELETE"],
      "regex": "^/api/users/\\d+$",
      "requiredPermission": "users:delete"
    }
  ],
  "hotReload": {
    "enabled": true,
    "intervalSeconds": 60
  }
}
```

### YAML Configuration

```yaml
roleHeader: X-User-Role

roles:
  - name: admin
    permissions:
      - "*:*"
  - name: editor
    permissions:
      - users:read
      - users:write
      - posts:read
      - posts:write
    inheritsFrom:
      - viewer
  - name: viewer
    permissions:
      - users:read
      - posts:read

endpoints:
  - path: /health
    methods: [GET]
    public: true
  - path: /api/users
    methods: [GET]
    requiredPermission: users:read
  - path: /api/users
    methods: [POST, PUT]
    requiredPermission: users:write
  - path: /api/users/{id}
    methods: [DELETE]
    regex: "^/api/users/\\d+$"
    requiredPermission: users:delete

hotReload:
  enabled: true
  intervalSeconds: 60
```

## Configuration Fields

### Core Fields

| Field | Type | Description |
|-------|------|-------------|
| `roleHeader` | `string` | HTTP header key for role extraction (e.g., "X-User-Role"). Used only if `jwtClaimPath` is not set |
| `jwtClaimPath` | `string` | JWT claim path for JWT-based role extraction (e.g., "role", "roles[0]", "permissions.role"). **Takes precedence** over `roleHeader` if both are set |
| `roles` | `array` | Array of role definitions with permissions and inheritance |
| `endpoints` | `array` | Array of endpoint mappings with permission requirements |
| `hotReload` | `object` | Hot reload configuration (optional) |

### Role Definition

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Role name (required) |
| `permissions` | `array` | List of permissions for this role (format: "resource:action", e.g., "users:read") |
| `inheritsFrom` | `array` | List of role names to inherit permissions from |

### Endpoint Mapping

| Field | Type | Description |
|-------|------|-------------|
| `path` | `string` | Route path pattern (supports wildcards like `/api/*`) |
| `regex` | `string` | Regular expression pattern (takes precedence over `path`) |
| `methods` | `array` | HTTP methods (GET, POST, PUT, DELETE, PATCH, etc.). Use `["*"]` for all methods |
| `requiredPermission` | `string` | Required permission (format: "resource:action"). **Required** unless `public: true` |
| `public` | `boolean` | If `true`, endpoint bypasses authorization |

### Hot Reload Configuration

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | `boolean` | Enable hot reloading |
| `intervalSeconds` | `number` | Interval in seconds between hot reload checks |
| `source` | `object` | Hot reload source (configured programmatically - see Hot Reload Guide) |

## Route Patterns

GoFr supports flexible route pattern matching in endpoint definitions:

- **Exact Match**: `"/api/users"` matches exactly `/api/users`
- **Wildcard**: `"/api/*"` matches `/api/users`, `/api/posts`, etc.
- **Regex**: `"^/api/users/\\d+$"` matches `/api/users/123`, `/api/users/456`, etc.

## Pure Config-Based Approach

GoFr RBAC uses **only configuration files** - there are no programmatic functions, no REST API endpoints for management, and no code-based authorization rules. This ensures:

- ✅ **Single Source of Truth**: All authorization rules in config files
- ✅ **Version Control**: Track all RBAC changes in git
- ✅ **No Code Changes**: Update authorization without redeploying
- ✅ **Infrastructure as Code**: Declarative RBAC configuration
- ✅ **Audit Compliance**: Complete history of authorization changes

### Two-Level Mapping

RBAC uses a pure two-level mapping approach:

1. **Role → Permission Mapping**: Defined in `roles[].permissions`
2. **Route & Method → Permission Mapping**: Defined in `endpoints[].requiredPermission`

**No direct route-to-role mapping** - all authorization is permission-based for consistency and security.

## Hot Reloading

RBAC supports hot reloading of configuration without restarting the application. This allows you to update permissions and endpoint mappings dynamically from external sources (Redis, HTTP service, database, etc.).

### How It Works

1. **Initial Load**: Config is loaded from file at startup
2. **Hot Reload**: Periodically fetches updated config from your configured source
3. **Thread-Safe Updates**: In-memory maps are atomically updated without blocking requests

### Configuration

Enable hot reload in your config file:

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

### Implementing Hot Reload

To enable hot reload, you need to implement the `gofr.HotReloadSource` interface and configure it in `app.OnStart`:

```go
package main

import (
    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/rbac"
)

// HotReloadSource interface
type HotReloadSource interface {
    // FetchConfig fetches the updated RBAC configuration
    // Returns the config data (JSON or YAML bytes) and error
    FetchConfig() ([]byte, error)
}

func main() {
    app := gofr.New()

    // Create RBAC provider and enable RBAC
    provider := rbac.NewProvider()
    app.EnableRBAC(provider, "configs/rbac.json")

    // Configure hot reload in OnStart hook
    app.OnStart(func(ctx *gofr.Context) error {
        // Implement your own HotReloadSource (Redis, HTTP, database, etc.)
        source := NewYourCustomHotReloadSource(ctx)
        return provider.EnableHotReload(source)
    })

    app.GET("/api/users", getAllUsers)
    app.Run()
}
```

**Example: Custom Hot Reload Source**

```go
// Example: Custom hot reload source that fetches from a database
type DatabaseHotReloadSource struct {
    db *sql.DB
}

func (d *DatabaseHotReloadSource) FetchConfig() ([]byte, error) {
    // Fetch config from database
    var configJSON string
    err := d.db.QueryRow("SELECT config FROM rbac_config WHERE id = 1").Scan(&configJSON)
    if err != nil {
        return nil, err
    }
    return []byte(configJSON), nil
}

// In app.OnStart:
app.OnStart(func(ctx *gofr.Context) error {
    source := &DatabaseHotReloadSource{db: ctx.DB}
    return provider.EnableHotReload(source)
})
```

### Best Practices

1. **Interval**: Set appropriate interval (60-300 seconds recommended)
2. **Error Handling**: Hot reload errors are logged but don't stop the application
3. **Source Reliability**: Ensure your hot reload source is highly available
4. **Config Validation**: Hot reload validates config before applying
5. **Monitoring**: Monitor hot reload success/failure in logs

### Limitations

- Hot reload only updates permissions and endpoint mappings
- Role extraction method (header/JWT) cannot be changed via hot reload
- Error handler and logger cannot be changed via hot reload

## Context Helpers

Access role information in your handlers for business logic (not authorization - that's handled by middleware):

```go
import "gofr.dev/pkg/gofr/rbac"

func handler(ctx *gofr.Context) (interface{}, error) {
	// Check if user has specific role (for business logic only)
	if rbac.HasRole(ctx, "admin") {
		// Admin-only business logic (e.g., show admin panel)
	}

	// Get user's role (for business logic only)
	role := rbac.GetUserRole(ctx)
	
	// Use role for business logic (e.g., personalize UI, filter data)
	return map[string]string{"userRole": role}, nil
}
```

**Note**: These helpers are **read-only** and are for business logic purposes only. All authorization is handled automatically by the RBAC middleware based on your config file. You don't need to check permissions in your handlers - the middleware already did that before your handler was called.

## Advanced Features

### Custom Error Handler

Customize error responses for authorization failures by setting the error handler on the config. Note: This requires accessing the config after loading, which is typically not needed as the default error responses are sufficient.

```go
// Note: This is an advanced use case. The default error responses are usually sufficient.
// To customize, you would need to access the config after LoadPermissions and set ErrorHandler.
// Most users don't need to customize error handling.
```

### Audit Logging

RBAC automatically logs all authorization decisions using GoFr's logger. The logger is set automatically when you call `app.EnableRBAC()` - no configuration needed.

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

## In-Memory Maps

At startup, RBAC builds in-memory maps for efficient lookups:

- **`rolePermissionsMap`**: role → []permissions (includes inherited permissions)
- **`endpointPermissionMap`**: "METHOD:/path" → permission
- **`publicEndpointsMap`**: "METHOD:/path" → true (for public endpoints)

These maps are:
- Built at startup from config file
- Updated atomically during hot reload (thread-safe)
- Used for fast authorization checks in middleware

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
6. **No default role** - Explicit role required for all requests (fail secure)
7. **Pure permission-based** - All authorization is permission-based, no direct role mapping

### 2. Configuration

1. **Use unified format** - Define roles and endpoints in single config
   ```json
   {
     "roles": [
       {
         "name": "admin",
         "permissions": ["*:*"]
       },
       {
         "name": "editor",
         "permissions": ["users:read", "users:write"],
         "inheritsFrom": ["viewer"]
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
2. **Use role inheritance** - Avoid duplicating permissions
3. **Set roleHeader or jwtClaimPath** - Configure role extraction method
4. **Use hot reload** - Update permissions without restarting
5. **Version control configs** - Track RBAC changes in git

### 3. Permission Design

1. **Use consistent naming** - Follow `resource:action` format (e.g., `users:read`, `posts:write`)
2. **Group related permissions** - Organize by resource type
3. **Document permission requirements** - Comment which permissions are needed for each endpoint
4. **Test permission checks** - Write integration tests to verify authorization

### 4. Code Organization

1. **All authorization in config** - No programmatic functions, everything in config files
2. **Use context helpers** - Access role info in handlers when needed (role is stored in context)
3. **Keep configs in files** - All RBAC rules in JSON/YAML files
4. **Version control configs** - Track RBAC configuration changes in git

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

- `app.EnableRBAC(provider gofr.RBACProvider, configFile string)` - Enables RBAC with config file
  
  **Parameters:**
  - `provider` (gofr.RBACProvider): RBAC provider instance. Create using `rbac.NewProvider()`
  - `configFile` (string): Optional path to RBAC config file. If empty, tries default paths:
    - `configs/rbac.json`
    - `configs/rbac.yaml`
    - `configs/rbac.yml`
  
  **Role Extraction:**
  - Configured entirely in the config file (no options needed)
  - Set `roleHeader` for header-based extraction
  - Set `jwtClaimPath` for JWT-based extraction
  - **Precedence**: JWT takes precedence if both are set
  
  **Examples:**
  ```go
  import (
      "gofr.dev/pkg/gofr"
      "gofr.dev/pkg/gofr/rbac"
  )
  
  app := gofr.New()
  provider := rbac.NewProvider()
  
  // Use default config path
  app.EnableRBAC(provider, "")
  
  // Use custom config path
  app.EnableRBAC(provider, "configs/custom-rbac.json")
  ```

### Context Helpers

Access role information in your handlers:

- `rbac.GetUserRole(ctx)` - Get user role from context (stored by middleware)

**Note**: All authorization is handled by middleware based on config. No programmatic authorization functions are provided to ensure a single, consistent way to declare RBAC.

## Troubleshooting

### Common Issues

**Issue**: Role not being extracted
- **Solution**: Ensure `roleHeader` or `jwtClaimPath` is set in config file. For header-based: check that the header is present in requests. For JWT-based: ensure OAuth middleware is enabled before RBAC.

**Issue**: Permission checks failing
- **Solution**: Verify `roles[].permissions` is properly configured and `endpoints[].requiredPermission` matches your routes correctly. Check that role has the required permission.

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
