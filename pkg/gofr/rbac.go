package gofr

import (
	"net/http"

	"go.opentelemetry.io/otel"
)

// RBACProvider is the interface for RBAC implementations.
// External RBAC modules (like gofr.dev/pkg/gofr/rbac) implement this interface.
type RBACProvider interface {
	// UseLogger sets the logger for the provider
	UseLogger(logger any)

	// UseMetrics sets the metrics for the provider
	UseMetrics(metrics any)

	// UseTracer sets the tracer for the provider
	UseTracer(tracer any)

	// LoadPermissions loads RBAC configuration from the stored config path
	LoadPermissions() error

	// ApplyMiddleware returns the middleware function using the stored config
	// The returned function should be compatible with http.Handler middleware pattern
	ApplyMiddleware() func(http.Handler) http.Handler
}

// DefaultRBACConfig is a constant that can be passed to NewProvider to use default config paths.
// When passed, NewProvider will try: configs/rbac.json, configs/rbac.yaml, configs/rbac.yml.
const DefaultRBACConfig = ""

// EnableRBAC enables RBAC by loading configuration from a JSON or YAML file.
// This is a factory function that registers RBAC implementations and sets up the middleware.
// The config file path is stored in the provider (set via NewProvider).
//
// Pure config-based: All authorization rules are defined in the config file using:
// - Roles: role → permission mapping (format: "resource:action")
// - Endpoints: route & method → permission mapping
//
// Example:
//
//	import (
//		"gofr.dev/pkg/gofr"
//		"gofr.dev/pkg/gofr/rbac"
//	)
//
//	app := gofr.New()
//	provider := rbac.NewProvider("configs/rbac.json") // Store config path
//	app.EnableRBAC(provider) // Uses stored path
//
// Role extraction is configured in the config file:
// - Set "roleHeader" for header-based extraction (e.g., "X-User-Role")
// - Set "jwtClaimPath" for JWT-based extraction (e.g., "role", "roles[0]").
func (a *App) EnableRBAC(provider RBACProvider) {
	if provider == nil {
		a.Logger().Error("RBAC provider is required. Create one using: provider := rbac.NewProvider(\"configs/rbac.json\")")
		return
	}

	// Set logger, metrics, and tracer automatically
	provider.UseLogger(a.Logger())
	provider.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-rbac")
	provider.UseTracer(tracer)

	// Load configuration from file using the provider
	// Logger is automatically set on config during LoadPermissions
	if err := provider.LoadPermissions(); err != nil {
		a.Logger().Errorf("Failed to load RBAC config: %v", err)
		return
	}

	a.Logger().Infof("Loaded RBAC config successfully")

	// Apply middleware using the provider
	middlewareFunc := provider.ApplyMiddleware()
	a.httpServer.router.Use(middlewareFunc)
}
