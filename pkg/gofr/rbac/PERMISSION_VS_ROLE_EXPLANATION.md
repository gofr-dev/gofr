# Permission-Based vs Role-Based Access Control

## The Issue

When you have:

```json
{
  "roles": [
    {
      "name": "admin",
      "permissions": ["*:*"]  // Admin has all permissions
    }
  ],
  "endpoints": [
    {
      "path": "/api/admin/*",
      "methods": ["*"],
      "allowedRoles": ["admin"]  // Why specify this if admin has *:*?
    }
  ]
}
```

## The Problem

**These are two different authorization mechanisms:**

1. **Permission-Based Check** (`requiredPermission`):
   - Checks if the role has the required permission
   - Admin with `*:*` would pass ANY permission check
   - More flexible and scalable

2. **Role-Based Check** (`allowedRoles`):
   - Checks if the role is in the allowed roles list
   - Does NOT check permissions at all
   - Simpler but less flexible

## Current Behavior

When you use `allowedRoles: ["admin"]`:
- ✅ It checks: "Is the user's role == 'admin'?"
- ❌ It does NOT check: "Does admin have the required permission?"

So even though admin has `*:*` permissions, the endpoint with `allowedRoles` doesn't use that information - it only checks the role name.

## The Solution: Use Permission-Based Consistently

Instead of using `allowedRoles`, use `requiredPermission` with a permission that admin has:

```json
{
  "roles": [
    {
      "name": "admin",
      "permissions": ["*:*"]  // Matches any permission
    }
  ],
  "endpoints": [
    {
      "path": "/api/admin/*",
      "methods": ["*"],
      "requiredPermission": "admin:*"  // Or any permission - *:* matches it
    }
  ]
}
```

**How it works:**
- Endpoint requires `admin:*` permission
- Admin role has `*:*` permission
- `*:*` matches `admin:*` (wildcard matching)
- ✅ Access granted

## Better Approach: Use a Specific Permission

For admin-only endpoints, define a specific permission:

```json
{
  "roles": [
    {
      "name": "admin",
      "permissions": ["*:*", "admin:access"]  // Or just *:*
    }
  ],
  "endpoints": [
    {
      "path": "/api/admin/*",
      "methods": ["*"],
      "requiredPermission": "admin:access"  // Explicit permission
    }
  ]
}
```

Or, since `*:*` matches everything, you can use any permission:

```json
{
  "endpoints": [
    {
      "path": "/api/admin/*",
      "methods": ["*"],
      "requiredPermission": "admin:*"  // *:* will match this
    }
  ]
}
```

## When to Use Each Approach

### Use `requiredPermission` (Recommended)
- ✅ Consistent with permission-based model
- ✅ Works with `*:*` wildcard
- ✅ More flexible and scalable
- ✅ Better for audit trails

**Example:**
```json
{
  "endpoints": [
    {
      "path": "/api/admin/*",
      "methods": ["*"],
      "requiredPermission": "admin:*"
    }
  ]
}
```

### Use `allowedRoles` (Simple Cases)
- ✅ Quick setup for simple APIs
- ✅ Good for role-only scenarios
- ❌ Doesn't leverage permissions
- ❌ Less flexible

**Example:**
```json
{
  "endpoints": [
    {
      "path": "/api/admin/*",
      "methods": ["*"],
      "allowedRoles": ["admin"]
    }
  ]
}
```

## Recommended Configuration

For your use case, use permission-based consistently:

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
      "requiredPermission": "admin:*"  // *:* matches this
    }
  ],
  "defaultRole": "viewer"
}
```

**Why this works:**
- Admin has `*:*` which matches `admin:*` (and any other permission)
- Consistent permission-based approach
- No need to specify `allowedRoles` separately
- More maintainable and scalable

## Summary

**Your question is valid!** If you're using permission-based access control, you should use `requiredPermission` consistently. The `*:*` permission will automatically grant access to any endpoint that requires a permission, so you don't need to specify `allowedRoles` separately.

Use `allowedRoles` only when you want simple role-based checks without permissions, or when you need both checks (permission AND role) for maximum security.

