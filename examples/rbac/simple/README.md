# Simple RBAC Example

## Overview

This example demonstrates **simple role-based access control (RBAC)** using header-based role extraction. It's the most straightforward way to implement RBAC in GoFr applications.

## Use Case

**When to use:**
- Simple applications with basic role requirements
- Internal APIs where roles can be trusted from headers
- Microservices communicating within a trusted network
- Quick prototyping and development

**Not suitable for:**
- Public-facing APIs (use JWT-based RBAC instead)
- High-security applications (use JWT with proper validation)
- Multi-tenant systems (use DB-based role extraction)

## How It Works

1. **Role Extraction**: The role is extracted from the `X-User-Role` HTTP header
2. **Route Matching**: Routes are matched against patterns in the config file
3. **Authorization**: The extracted role is checked against allowed roles for the route
4. **Audit Logging**: All authorization decisions are automatically logged using GoFr's logger

## Configuration

### RBAC Config (`configs/rbac.json`)

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
  }
}
```

**Configuration Fields:**
- `route`: Maps route patterns to allowed roles
  - `"/api/users"`: Exact route match
  - `"/api/admin/*"`: Wildcard pattern (matches `/api/admin/dashboard`, etc.)
  - `"*"`: Global fallback route
- `overrides`: Routes that bypass RBAC (public access)
- `defaultRole`: Role used when header is missing
- `roleHierarchy`: Defines role inheritance (admin inherits editor, author, viewer permissions)

## Setup Instructions

1. **Start the application:**
   ```bash
   go run main.go
   ```

2. **Test with different roles:**
   ```bash
   # As admin - should succeed
   curl -H "X-User-Role: admin" http://localhost:8000/api/users
   curl -H "X-User-Role: admin" http://localhost:8000/api/admin
   
   # As editor - should succeed for users, fail for admin
   curl -H "X-User-Role: editor" http://localhost:8000/api/users
   curl -H "X-User-Role: editor" http://localhost:8000/api/admin
   
   # As viewer - should succeed for users, fail for admin
   curl -H "X-User-Role: viewer" http://localhost:8000/api/users
   curl -H "X-User-Role: viewer" http://localhost:8000/api/admin
   ```

## API Endpoints

- `GET /api/users` - Accessible by: admin, editor, viewer
- `GET /api/admin` - Accessible by: admin only (uses `RequireRole`)
- `GET /api/dashboard` - Accessible by: admin or editor (uses `RequireAnyRole`)

## Features Demonstrated

1. **Basic RBAC**: Route-based role checking
2. **Helper Functions**: `RequireRole()` and `RequireAnyRole()`
3. **Role Hierarchy**: Admin automatically has editor/author/viewer permissions
4. **Route Overrides**: Health and metrics endpoints bypass RBAC
5. **Default Role**: Missing role header uses "viewer" role

## Security Considerations

⚠️ **Important**: Header-based role extraction is **not secure** for public APIs because:
- Headers can be easily spoofed
- No authentication/authorization validation
- No token verification

**Use this only for:**
- Internal services within a trusted network
- Development/testing environments
- APIs behind an API gateway that validates requests

For production public APIs, use **JWT-based RBAC** instead.

