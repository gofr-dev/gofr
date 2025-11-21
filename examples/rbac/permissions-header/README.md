# Permission-Based RBAC (Header) Example

## Overview

This example demonstrates **permission-based access control** with header-based role extraction. Instead of checking roles directly, it checks if the user's role has the required **permission** for the action.

## Use Case

**When to use:**
- Fine-grained access control needed
- Permissions change more frequently than roles
- Multiple roles can have the same permission
- Need to check permissions at the action level (read, write, delete)

**Example scenarios:**
- Content management systems (authors can write posts, editors can edit, admins can delete)
- E-commerce platforms (viewers can read products, managers can update, admins can delete)
- Multi-tenant SaaS applications

## How It Works

1. **Role Extraction**: Role extracted from `X-User-Role` header
2. **Permission Mapping**: Route + HTTP method mapped to required permission
3. **Permission Check**: System checks if user's role has the required permission
4. **Authorization**: Access granted/denied based on permission check

## Configuration

### RBAC Config (`configs/rbac.json`)

```json
{
  "route": {
    "/api/*": ["admin", "editor"]
  },
  "enablePermissions": true
}
```

### Permission Configuration (in code)

```go
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
```

**Configuration Fields:**
- `Permissions`: Maps permission strings to roles that have them
- `RoutePermissionMap`: Maps `"METHOD /path"` to required permission
- `DefaultPermission`: Optional fallback permission

## Setup Instructions

1. **Start the application:**
   ```bash
   go run main.go
   ```

2. **Test with different roles:**
   ```bash
   # Admin can read, write, and delete
   curl -H "X-User-Role: admin" http://localhost:8000/api/users
   curl -X POST -H "X-User-Role: admin" http://localhost:8000/api/users
   curl -X DELETE -H "X-User-Role: admin" http://localhost:8000/api/users
   
   # Editor can read and write, but not delete
   curl -H "X-User-Role: editor" http://localhost:8000/api/users
   curl -X POST -H "X-User-Role: editor" http://localhost:8000/api/users
   curl -X DELETE -H "X-User-Role: editor" http://localhost:8000/api/users  # Should fail
   
   # Viewer can only read
   curl -H "X-User-Role: viewer" http://localhost:8000/api/users
   curl -X POST -H "X-User-Role: viewer" http://localhost:8000/api/users  # Should fail
   ```

## API Endpoints

- `GET /api/users` - Requires: `users:read` permission
- `POST /api/users` - Requires: `users:write` permission
- `DELETE /api/users` - Requires: `users:delete` permission (uses `RequirePermission`)
- `GET /api/posts` - Requires: `posts:read` permission
- `POST /api/posts` - Requires: `posts:write` permission

## Permission Matrix

| Role   | users:read | users:write | users:delete | posts:read | posts:write |
|--------|------------|-------------|--------------|-----------|------------|
| admin  | ✅         | ✅          | ✅           | ✅        | ✅         |
| editor | ✅         | ✅          | ❌           | ❌        | ❌         |
| viewer | ✅         | ❌          | ❌           | ✅        | ❌         |
| author | ❌         | ❌          | ❌           | ✅        | ✅         |

## Features Demonstrated

1. **Permission-Based Access**: Fine-grained control at the action level
2. **Route-Method Mapping**: Different permissions for GET vs POST vs DELETE
3. **Helper Functions**: `RequirePermission()` for explicit permission checks
4. **Flexible Permissions**: Multiple roles can share permissions

## Advantages Over Role-Based

✅ **More Flexible**: Permissions can be assigned to multiple roles  
✅ **Easier to Maintain**: Change permissions without changing route configs  
✅ **Action-Level Control**: Different permissions for different HTTP methods  
✅ **Scalable**: Easy to add new permissions without restructuring routes

## Best Practices

1. **Use descriptive permission names**: `resource:action` format (e.g., `users:read`)
2. **Group related permissions**: `users:read`, `users:write`, `users:delete`
3. **Use wildcards sparingly**: Be specific in route permission mapping
4. **Document permission requirements**: Keep a permission matrix for reference

