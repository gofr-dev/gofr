# RBAC Examples Summary

## Overview

Four comprehensive examples have been created to demonstrate different RBAC use cases and features.

## Examples Created

### 1. `rbac-enhanced/` - Basic RBAC
**Purpose:** Demonstrates basic role-based access control

**Features:**
- Simple RBAC setup
- Header-based role extraction
- Handler-level role checks
- Multiple role support

**Files:**
- `main.go` - Application code
- `configs/rbac.json` - Configuration with role hierarchy
- `README.md` - Usage instructions

### 2. `rbac-jwt/` - JWT-Based RBAC
**Purpose:** Demonstrates JWT token-based role extraction

**Features:**
- OAuth middleware integration
- JWT claim extraction
- Automatic role extraction from tokens

**Files:**
- `main.go` - Application with OAuth and JWT RBAC
- `configs/rbac.json` - Route configuration
- `README.md` - Usage instructions

**Prerequisites:**
- OAuth provider with JWKS endpoint
- JWT tokens with role claims

### 3. `rbac-permissions/` - Permission-Based Access Control
**Purpose:** Demonstrates fine-grained permission-based authorization

**Features:**
- Permission-to-role mapping
- Route-to-permission mapping
- Handler-level permission checks
- Multiple permissions per role

**Files:**
- `main.go` - Application with permission-based RBAC
- `configs/rbac.json` - Permission configuration
- `README.md` - Usage instructions

**Key Concepts:**
- `users:read`, `users:write`, `users:delete` permissions
- Route-to-permission mapping
- Permission-based handler checks

### 4. `rbac-advanced/` - Advanced Features
**Purpose:** Demonstrates all advanced RBAC features

**Features:**
- Hot-reload configuration
- Custom error handlers
- Custom audit logging
- Role caching
- YAML configuration
- Role hierarchy
- Permission-based access control

**Files:**
- `main.go` - Application with all advanced features
- `configs/rbac.yaml` - YAML configuration
- `configs/rbac.json` - JSON configuration (alternative)
- `README.md` - Usage instructions

**Advanced Features:**
- Configuration hot-reload (30 seconds)
- Custom JSON error responses
- Custom audit logger integration
- 5-minute role caching
- Complex role hierarchy
- Combined permissions and roles

## Running Examples

All examples follow the same pattern:

```bash
cd examples/rbac-<name>
go run main.go
```

Then test with curl commands as shown in each example's README.

## Configuration Formats

Examples demonstrate both JSON and YAML configuration formats:

- **JSON:** Simple, easy to read
- **YAML:** More readable for complex configurations

Both formats support all RBAC features.

## Testing Examples

Each example can be tested with different roles:

```bash
# Test as different roles
curl -H "X-User-Role: admin" http://localhost:8000/api/users
curl -H "X-User-Role: editor" http://localhost:8000/api/users
curl -H "X-User-Role: viewer" http://localhost:8000/api/users
```

## Next Steps

1. Start with `rbac-enhanced` for basic usage
2. Try `rbac-jwt` if using OAuth/JWT
3. Use `rbac-permissions` for fine-grained control
4. Explore `rbac-advanced` for production-ready features

