package rbac

import (
	"net/http"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/logging"
)

// registerRBAC registers RBAC implementations with core GoFr.
// This is called automatically when the package is imported via package-level variable initialization.
func registerRBAC() bool {
	gofr.RegisterRBAC(
		&rbacLoader{},            // RBACLoader
		&rbacMiddleware{},        // RBACMiddleware
		requireRoleAdapter,       // RequireRole function
		requireAnyRoleAdapter,    // RequireAnyRole function
		requirePermissionAdapter, // RequirePermission function
		ErrAccessDenied,          // ErrAccessDenied
		ErrPermissionDenied,      // ErrPermissionDenied
	)

	return true
}

// _ is a package-level variable that triggers registration when the package is imported.
// This avoids using init() while maintaining automatic registration behavior.
var _ = registerRBAC()

// rbacLoader implements RBACLoader interface.
type rbacLoader struct{}

func (*rbacLoader) LoadPermissions(file string) (gofr.RBACConfig, error) {
	return LoadPermissions(file)
}

func (*rbacLoader) NewConfigLoaderWithLogger(file string, logger any) (gofr.ConfigLoader, error) {
	var log logging.Logger
	if loggerVal, ok := logger.(logging.Logger); ok {
		log = loggerVal
	}

	loader, err := NewConfigLoaderWithLogger(file, log)
	if err != nil {
		return nil, err
	}

	return loader, nil
}

func (*rbacLoader) NewJWTRoleExtractor(claim string) gofr.JWTRoleExtractor {
	// NewJWTRoleExtractor returns JWTRoleExtractorProvider which implements gofr.JWTRoleExtractor
	return NewJWTRoleExtractor(claim)
}

// rbacMiddleware implements RBACMiddleware interface.
type rbacMiddleware struct{}

func (*rbacMiddleware) Middleware(config gofr.RBACConfig, args ...any) func(http.Handler) http.Handler {
	if cfg, ok := config.(*Config); ok {
		return Middleware(cfg, args...)
	}

	return func(handler http.Handler) http.Handler {
		return handler
	}
}

// requireRoleAdapter adapts RequireRole to match interface signature.
func requireRoleAdapter(allowedRole string, handlerFunc gofr.RBACHandlerFunc) gofr.RBACHandlerFunc {
	hf := HandlerFunc(handlerFunc)
	wrapped := RequireRole(allowedRole, hf)

	return gofr.RBACHandlerFunc(wrapped)
}

// requireAnyRoleAdapter adapts RequireAnyRole to match interface signature.
func requireAnyRoleAdapter(allowedRoles []string, handlerFunc gofr.RBACHandlerFunc) gofr.RBACHandlerFunc {
	hf := HandlerFunc(handlerFunc)
	wrapped := RequireAnyRole(allowedRoles, hf)

	return gofr.RBACHandlerFunc(wrapped)
}

// requirePermissionAdapter adapts RequirePermission to match interface signature.
func requirePermissionAdapter(
	requiredPermission string,
	permissionConfig gofr.PermissionConfig,
	handlerFunc gofr.RBACHandlerFunc,
) gofr.RBACHandlerFunc {
	if cfg, ok := permissionConfig.(*PermissionConfig); ok {
		hf := HandlerFunc(handlerFunc)
		wrapped := RequirePermission(requiredPermission, cfg, hf)

		return gofr.RBACHandlerFunc(wrapped)
	}

	return func(_ any) (any, error) {
		return nil, ErrPermissionDenied
	}
}
