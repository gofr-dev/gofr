# RBAC Examples

This directory contains comprehensive examples demonstrating different RBAC (Role-Based Access Control) implementations in GoFr.

## Examples Overview

### 1. [Simple RBAC](./simple/)
**Header-based role extraction** - The simplest RBAC implementation.

- **Use Case**: Internal APIs, trusted networks, quick prototyping
- **Role Source**: HTTP header (`X-User-Role`)
- **Best For**: Development, internal services, trusted environments

### 2. [JWT RBAC](./jwt/)
**JWT-based role extraction** - Secure RBAC using JWT tokens.

- **Use Case**: Public APIs, microservices, OAuth2/OIDC integration
- **Role Source**: JWT token claims
- **Best For**: Production public APIs, multi-service architectures

### 3. [Permission-Based RBAC (Header)](./permissions-header/)
**Permission-based access control with header roles** - Fine-grained permissions.

- **Use Case**: Fine-grained access control, action-level permissions
- **Role Source**: HTTP header (`X-User-Role`)
- **Best For**: Content management, e-commerce, SaaS platforms

### 4. [Permission-Based RBAC (JWT)](./permissions-jwt/)
**Permission-based access control with JWT roles** - Secure + flexible.

- **Use Case**: Public APIs requiring fine-grained permissions
- **Role Source**: JWT token claims
- **Best For**: Enterprise APIs, SaaS platforms, secure multi-tenant apps

### 5. [Permission-Based RBAC (Database)](./permissions-db/)
**Permission-based access control with database roles** - Dynamic role management.

- **Use Case**: Dynamic roles, multi-tenant, admin-managed roles
- **Role Source**: Database query
- **Best For**: Applications with user management dashboards, dynamic role assignment

## Quick Start

### Choose Your Example

1. **Simple RBAC** - Start here for basic role-based access
2. **JWT RBAC** - For secure, production-ready APIs
3. **Permission-Based** - When you need fine-grained control

### Running Examples

```bash
# Navigate to the example directory
cd examples/rbac/simple  # or jwt, permissions-header, etc.

# Run the example
go run main.go

# Test the endpoints (see individual README files)
curl -H "X-User-Role: admin" http://localhost:8000/api/users
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

*Can be optimized with caching

## Common Patterns

### Pattern 1: Simple Role-Based
```go
app.EnableRBAC("configs/rbac.json", roleExtractor)
```

### Pattern 2: JWT-Based
```go
app.EnableOAuth(jwksEndpoint, refreshInterval)
app.EnableRBACWithJWT("configs/rbac.json", "role")
```

### Pattern 3: Permission-Based
```go
config.PermissionConfig = &rbac.PermissionConfig{...}
app.EnableRBACWithPermissions(config, roleExtractor)
```

## Configuration Files

All examples use JSON or YAML configuration files. See individual example READMEs for:
- Configuration structure
- Route patterns
- Permission mappings
- Environment variable overrides

## Testing

Each example includes integration tests. Run tests with:

```bash
go test ./examples/rbac/... -v
```

## Security Best Practices

1. **Never use header-based RBAC for public APIs** - Use JWT instead
2. **Always validate JWT tokens** - Use proper JWKS endpoints
3. **Enable caching for DB-based roles** - Reduce database load
4. **Use HTTPS in production** - Protect tokens and headers
5. **Implement rate limiting** - Prevent abuse
6. **Monitor audit logs** - Track authorization decisions

## Migration Path

**Development → Production:**
1. Start with `simple` for development
2. Move to `jwt` for production
3. Add `permissions` when you need fine-grained control
4. Use `permissions-db` when roles need to be dynamic

## Need Help?

- Check individual example READMEs for detailed setup
- See [RBAC Documentation](../../../pkg/gofr/rbac/README.md) for API reference
- Review [Implementation Log](../../../pkg/gofr/rbac/IMPLEMENTATION_LOG.md) for technical details

