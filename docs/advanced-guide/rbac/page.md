---
description: "Configure role-based access control in GoFr with the config-driven RBAC middleware. Supports multiple auth methods, fine-grained perms, and role inheritance."
nextjs:
  metadata:
    title: "RBAC in GoFr — Role-Based Access Control Middleware"
    description: "Configure role-based access control in GoFr with the config-driven RBAC middleware. Supports multiple auth methods, fine-grained perms, and role inheritance."
---

# Role-Based Access Control (RBAC) in GoFr

Role-Based Access Control (RBAC) is a security mechanism that restricts access to resources based on user roles and permissions. GoFr provides a pure config-based RBAC middleware that supports multiple authentication methods, fine-grained permissions, and role inheritance.

## Overview

- ✅ **Pure Config-Based** - All authorization rules in JSON/YAML files
- ✅ **Two-Level Authorization Model** - Roles define permissions, endpoints require permissions (no direct role-to-route mapping)
- ✅ **Multiple Auth Methods** - Header-based and JWT-based role extraction
- ✅ **Multi-Value Claims** - A JWT role array, or a `scope`/permissions claim authorized directly
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
	if err := app.EnableRBAC(); err != nil {
		app.Logger().Fatalf("%v", err)
	}
	
	// Or with custom config path
	// if err := app.EnableRBAC("configs/custom-rbac.json"); err != nil { ... }
	
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

### Config Load Failures

`EnableRBAC` returns an error when the config cannot be used: the file is missing or
unreadable, it is not valid JSON/YAML, its extension is not `.json`, `.yaml` or `.yml`, it fails
validation, or no path is given and none of the default files exist. In that case no RBAC
middleware is installed, so handle the error — typically by stopping the app, as above. Starting
anyway would serve every route with no role checks.

It also logs an error saying authorization is **DISABLED**, so an app that ignores the returned
error still shows why every route is unprotected.


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
- `"roles"` → `{"roles": ["admin", "viewer"]}` (every role in the array, see [Role Arrays](#role-arrays))
- `"roles[0]"` → `{"roles": ["admin", "user"]}` (first element)
- `"permissions.role"` → `{"permissions": {"role": "admin"}}`
- `"https://example.com/roles"` → `{"https://example.com/roles": ["admin"]}`

A claim path is first looked up as one whole top-level key. Only if no such key exists is it read
as a dotted path (`a.b.c`). This is what makes namespaced claim names such as
`https://example.com/roles`, which contain dots, work.

### Configuration Reference

| Key | Type | Used in | Meaning |
|---|---|---|---|
| `roleHeader` | string | header mode | HTTP header that carries one role name, e.g. `X-User-Role` |
| `jwtClaimPath` | string | roles mode | JWT claim that holds a role name or an array of role names |
| `permissionsClaimPath` | string | permissions mode | JWT claim that holds the permissions themselves, e.g. `scope` |
| `audience` | array of strings | JWT modes | Accepted `aud` values. **Required** in permissions mode; checked in roles mode when set |
| `roles` | array | header and roles modes | Role → permission mapping (see below) |
| `endpoints` | array | all modes | Route & method → required permission mapping (see below) |

Exactly one mode is active:

- **Permissions mode** when `permissionsClaimPath` is set.
- **Roles mode** when `jwtClaimPath` is set.
- **Header mode** when only `roleHeader` is set.

`EnableRBAC` returns an error at startup if:

- both `jwtClaimPath` and `permissionsClaimPath` are set;
- `permissionsClaimPath` is set without `audience`;
- `audience` is set but neither JWT claim path is;
- an `audience` entry is empty.

At startup RBAC logs one line naming the mode it runs in, for example:

```
RBAC enabled: mode=permissions, claim=scope, audience=[orders-api]
```

### Role Arrays

When the claim named by `jwtClaimPath` is a JSON array, the request holds **every** role in it,
and is allowed if **any** of those roles has a required permission:

```json
{ "jwtClaimPath": "roles" }
```

| Token claim | Roles held |
|---|---|
| `"roles": ["admin", "viewer"]` | `admin` and `viewer` |
| `"roles": ["viewer", "unknown"]` | `viewer` (names not in `roles[]` grant nothing) |
| `"roles": ["", 42, "viewer"]` | `viewer` (empty and non-string entries are ignored) |
| `"role": "Org Admin"` with `jwtClaimPath: "role"` | one role named `Org Admin` (a role string is never split) |
| `"role": 123` or `"role": true` | one role named `123` or `true` (a number or boolean is read as its text) |
| `"role": null` | no role (the request is denied with a 403) |
| `"roles": [...]` with `jwtClaimPath: "roles[0]"` | the first element only |

### Permissions Mode (scope claim)

Many identity providers put what a token may do straight into the token, in a `scope` claim
(`"orders:read orders:write"`) or a `permissions` array. Permissions mode authorizes from that
claim directly, with no `roles[]` section:

```json
{
  "permissionsClaimPath": "scope",
  "audience": ["orders-api"],
  "endpoints": [
    { "path": "/orders", "methods": ["GET"], "requiredPermissions": ["orders:read"] },
    { "path": "/orders", "methods": ["POST"], "requiredPermissions": ["orders:write"] }
  ]
}
```

A request is allowed if the token holds **any** of the endpoint's `requiredPermissions`, compared
by exact string match:

| Token claim | Permissions held |
|---|---|
| `"scope": "orders:read orders:write"` | `orders:read` and `orders:write` (a string is split on spaces) |
| `"permissions": ["orders:read", "orders:write"]` | each entry as is (entries are not split) |
| `"permissions": ["", true, "orders:read"]` | `orders:read` (empty and non-string entries are ignored) |
| `"scope": ""`, `"scope": null`, `"scope": []` or no `scope` claim | nothing (the request is denied with a 403) |

Because the token itself is the grant here, permissions mode also checks two claims:

- **`aud`**: the token's audience (a string or an array) must share at least one value with
  `audience`. A token minted for a different API gets a **401**.
- **`exp`**: the token must carry an expiry. A token without `exp` gets a **401**. (The OAuth
  middleware already rejects tokens whose `exp` is in the past.)

Like every RBAC check, `aud` and `exp` are checked only on routes that have a rule in `endpoints`.
A route with no rule is passed straight through (see [Unmatched Routes Behavior](#unmatched-routes-behavior)),
so a token minted for another API still reaches it.

RBAC does not check who issued the token. Permissions mode trusts what the token says, so pass the
expected issuer to `EnableOAuth`; a token from any other issuer, or with no `iss`, then gets a 401:

```go
app.EnableOAuth(app.Config.Get("JWKS_URL"), 10, jwt.WithIssuer(app.Config.Get("TOKEN_ISSUER")))
```

Permissions mode is only as strict as your identity provider. If the provider grants a client any
scope it asks for, every user holds every permission. Check how your provider decides which scopes
a token gets — some need per-user RBAC or an access policy turned on before scopes are restricted.

### Rule Summary for JWT Modes

| Case | Result |
|---|---|
| No JWT claims are in the request (OAuth not enabled) | 401 |
| Roles mode, the claim is missing or `""` | 401 |
| Roles mode, the claim is `null` | 403 |
| Roles mode, the claim is a number or boolean | read as a role name, e.g. `123` → role `123` |
| Permissions mode, the claim is missing, `null`, `""` or `[]` | 403 |
| The claim is an object, or (permissions mode) a number or boolean | 403, plus a WARN log naming the claim path (never its value) |
| The token's `aud` shares no value with `audience` | 401, `Unauthorized: Token audience not accepted` |
| Permissions mode, token has no `exp` | 401, `Unauthorized: Token has no expiry` |
| No role or permission held grants a required permission | 403 |

These follow [RFC 6750 §3.1](https://www.rfc-editor.org/rfc/rfc6750#section-3.1): a 401 means the
token itself is not acceptable, and a 403 means the token is fine but holds nothing that grants
access. A custom `ErrorHandler` can tell the 401 causes apart with `errors.Is(err,
rbac.ErrAudienceMismatch)` and `errors.Is(err, rbac.ErrMissingExpiry)`; both also match
`rbac.ErrRoleNotFound`.

### Config Checks at Startup for JWT Modes

- An `audience` entry that is empty is rejected — `[""]` would otherwise accept tokens whose `aud`
  is `""`.
- With `permissionsClaimPath` set, a `roles` section is never consulted; RBAC logs a WARN at startup
  if one is present, so a section left over from roles mode is not mistaken for one that applies.

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

### Rule Resolution

More than one endpoint entry can match the same request — a broad pattern and a narrow one, for
example. GoFr resolves this by **most specific wins**, rather than by the order the entries appear
in the config file.

Specificity is compared segment by segment, and the first segment where two patterns differ decides:

1. A literal segment (`users`) is more specific than
2. a constrained variable (`{id:[0-9]+}`), which is more specific than
3. a free variable (`{id}`), which is more specific than
4. a multi-segment catch-all (`{path:.*}`).

If the paths are equally specific, an entry that names its methods explicitly wins over one
declared with `["*"]`.

```json
{
  "endpoints": [
    {
      "path": "/admin/{path:.*}",
      "methods": ["*"],
      "requiredPermissions": ["admin:read"]
    },
    {
      "path": "/admin/orgs/{org_id}",
      "methods": ["DELETE"],
      "requiredPermissions": ["admin:write"]
    }
  ]
}
```

`DELETE /admin/orgs/123` matches both entries, and the second one governs it — so `admin:write` is
required. `GET /admin/settings` matches only the first, so `admin:read` is required.

`GET /admin/orgs/123` is the case to watch. The narrow entry is `["DELETE"]`-only, so it does not
cover a GET at all and drops out on method before specificity is ever considered — the catch-all
governs, and the request needs only `admin:read`. Writing a strict rule for one method does not
protect the other methods on that path; each method needs its own entry, or the broad entry has to
be strict enough to stand on its own.

> **Note**: `"methods": ["*"]` — and omitting `methods` entirely, which means the same thing —
> matches **every** HTTP method, including methods GoFr does not otherwise know about. Since an
> entry states what a caller must have in order to be let through, covering an unrecognized method
> tightens enforcement rather than relaxing it.

#### Where the ordering is not decided

**Two patterns that score identically** — `/{a}/{b}` and `/{x}/{y}` are the same shape, so nothing
about the patterns separates them. One thing still does: an entry that **requires permissions wins
over a public one**, because a tie is a config that did not express an intent either way and
enforcing is the recoverable half of that mistake. Past that, the first entry declared wins. Avoid
writing two entries that overlap without one being plainly narrower than the other.

**A constraint that can span segments** — a constraint containing `/`, such as `{path:[a-z/]+}`,
matches `/files/a/b/c` the way a catch-all does, so it is scored as one: least specific, losing to
any narrower entry it overlaps. That is decided by the `/` appearing in the constraint at all, not
by whether the regex can really produce one — `{id:[0-9]+/}` is scored as a catch-all even though
it is anchored to a single segment. The effect is only ever to move an entry *down* the ordering,
so it can lose to a narrower rule but never shadow one. Write multi-segment matches as `{path:.*}`
or `{path:.+}` and the scoring is exact.

#### Declaring the same path twice

Two entries with the same method and path are one entry: **the last one declared wins, in full**.
That includes the `public` flag — a public entry followed by a protected one for the same key is
protected, not public. Duplicates are not rejected, so it is worth checking for them in a config
assembled from more than one source.

#### A pattern that cannot compile

If a pattern's constraint is not valid regex — `{id:[}`, for example — the config still loads and
the application still starts, but the pattern can never match a request, so **that endpoint is not
enforced**. GoFr logs the pattern at error level on startup:

```
RBAC: endpoint[2]: invalid mux pattern: "/api/{id:[}": ... This endpoint will never match a
request, so it is NOT enforced - any route it was meant to govern is currently unguarded.
```

Treat that line as an open route, not a warning about a typo.

## JWT-Based RBAC

For production/public APIs, use JWT-based role extraction:

```go
app := gofr.New()

// Enable OAuth middleware first (required for JWT validation)
if err := app.EnableOAuth("https://auth.example.com/.well-known/jwks.json", 10); err != nil {
	app.Logger().Fatalf("%v", err)
}

// Enable RBAC with config path (or call app.EnableRBAC() for the default paths)
if err := app.EnableRBAC("configs/rbac.json"); err != nil {
	app.Logger().Fatalf("%v", err)
}
```

**Configuration** (`configs/rbac.json`):

```json
{
  "jwtClaimPath": "role",  // or "roles", "roles[0]", "permissions.role", etc.
  "roles": [...],
  "endpoints": [...]
}
```

**Call order**: call `EnableOAuth` before `EnableRBAC`. The OAuth middleware is what puts the
verified token's claims into the request; without it every request in a JWT mode gets a 401, and
RBAC logs a WARN once, on the first such request, telling you to call `EnableOAuth` first.

For a full permissions-mode app, with tests that sign their own tokens against a local JWKS
server, see [`examples/using-rbac-permissions`](https://github.com/gofr-dev/gofr/tree/development/examples/using-rbac-permissions).


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
- Ensure OAuth middleware is enabled before RBAC (a WARN `call EnableOAuth before EnableRBAC`
  on the first request means it is not)
- Verify JWT claim path is correct
- A WARN naming the claim path means the claim has a type RBAC cannot read (an object, or in
  permissions mode a number or boolean); it must be a string or an array of strings
- A rule on a `/.well-known/*` path always gets a 401: the OAuth middleware does not verify tokens
  there, so the request carries no claims

**Every permissions-mode request gets a 401**
- Check that the token's `aud` contains one of the values in `audience`
- Check that the token has an `exp` claim
- If `EnableOAuth` is given `jwt.WithIssuer`, check that the token's `iss` matches it

**Config file not found**
- Ensure config file exists at the specified path — relative paths resolve against the process's
  working directory, which inside a container is often not where the file was copied
- Or use default paths (`configs/rbac.json`, `configs/rbac.yaml`, `configs/rbac.yml`)
- A startup log line `Authorization is DISABLED` means the app ignored the error from
  `EnableRBAC` and is serving every route without role checks; handle the error so this stops
  startup instead (see [Config Load Failures](#config-load-failures))

**Route not being protected by RBAC**
- Verify the route is explicitly configured in `endpoints[]` array
- Check that the path pattern matches exactly (case-sensitive)
- Ensure HTTP method matches (or use `["*"]` for all methods) — a rule declared for one method does
  not cover the others on that path
- Check the startup logs for `invalid mux pattern` — a pattern whose constraint is not valid regex
  loads but never matches, leaving that endpoint unenforced
- Remember: Routes not in RBAC config are allowed to proceed (not blocked)

**The wrong permission is being required on a path two entries match**
- The more specific pattern governs — see [Rule Resolution](#rule-resolution)
- Check whether the narrower entry actually covers the request's method; if it does not, it drops
  out and the broader entry applies
- If both patterns are equally specific, declaration order decides — rewrite one to be narrower

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
- ✅ `rbac_role_extraction_failures` counts requests whose role or permissions could not be read,
  with a `reason` label: `audience_mismatch` (a token for another API, or a wrong `audience`),
  `no_jwt_claims` (RBAC is not behind OAuth), `missing_expiry`, `missing_role` (no role claim or
  header) or `unreadable_claim` (an IdP or client problem)
- ❌ Roles excluded (avoid high cardinality and PII concerns)

**Logs:**
- ✅ Roles included (required for compliance: SOC 2, PCI-DSS, NIST)
- ✅ HTTP method, route, status, and reason included
- ✅ With a role array or permissions mode, an allowed request logs only the one role or
  permission that granted access (in permissions mode it is logged as `permission`, not `role`), and a denied request logs only how many were held
  (`held_count`), never their names. The `role` passed to a custom `ErrorHandler` on such a
  denial is empty. A single role (header, `"role"` or `"roles[0]"`) is logged as before.
- ❌ Claim values are never logged; an unreadable claim is reported by its path only
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

- [Authentication](https://gofr.dev/docs/advanced-guide/authentication) - Basic Auth, API Keys, OAuth 2.0
- [HTTP Communication](https://gofr.dev/docs/advanced-guide/http-communication) - Inter-service HTTP calls
- [Middlewares](https://gofr.dev/docs/advanced-guide/middlewares) - Custom middleware implementation

## Related production guides

- **Auth in Kubernetes**: [Run RBAC-protected services on Kubernetes](/docs/guides/auth-in-kubernetes) — JWT verification keys and OIDC issuer wiring in cluster.
