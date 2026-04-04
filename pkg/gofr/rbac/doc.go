// Package rbac implements Role-Based Access Control for GoFr applications.
//
// It supports file-driven configuration (JSON or YAML) for defining roles
// with permissions and inheritance, mapping endpoints to required permissions,
// and provides HTTP middleware that resolves routes, extracts roles from JWT
// claims or headers, and enforces permission checks with optional audit
// logging and tracing.
package rbac
