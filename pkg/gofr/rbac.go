package gofr

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/rbac"
)

var errNoRBACConfig = errors.New("no RBAC config file found at configs/rbac.json, configs/rbac.yaml or configs/rbac.yml")

// EnableRBAC enables RBAC by loading configuration from a JSON or YAML file.
// It loads the config directly and sets up the middleware.
//
// Pure config-based: All authorization rules are defined in the config file using:
// - Roles: role → permission mapping (format: "resource:action")
// - Endpoints: route & method → permission mapping
//
// Usage:
//
//	// Use default paths (configs/rbac.json, configs/rbac.yaml, configs/rbac.yml)
//	// Uses rbac.DefaultConfigPath internally
//	if err := app.EnableRBAC(); err != nil {
//		app.Logger().Fatalf("%v", err)
//	}
//
//	// Or with custom config path
//	if err := app.EnableRBAC("configs/custom-rbac.json"); err != nil {
//		app.Logger().Fatalf("%v", err)
//	}
//
// Role extraction is configured in the config file:
// - Set "roleHeader" for header-based extraction (e.g., "X-User-Role")
// - Set "jwtClaimPath" for JWT-based extraction (e.g., "role", "roles[0]").
//
// It returns an error, and installs no middleware, if the config cannot be found, read, parsed or
// validated. The error is also logged, so an app that ignores it still sees why authorization is
// off; return or exit on it to stop startup instead.
func (a *App) EnableRBAC(configPath ...string) error {
	return a.authDisabled(a.enableRBAC(configPath...), "Authorization", "EnableRBAC")
}

func (a *App) enableRBAC(configPath ...string) error {
	var path string
	if len(configPath) > 0 {
		path = configPath[0]
	} else {
		// Use rbac.DefaultConfigPath (empty string) to trigger default path resolution
		path = rbac.ResolveRBACConfigPath(rbac.DefaultConfigPath)
		if path == "" {
			return errNoRBACConfig
		}
	}

	// Get dependencies
	logger := a.Logger()
	metrics := a.Metrics()
	tracer := otel.GetTracerProvider().Tracer("gofr-rbac")

	// Load configuration directly with dependencies
	config, err := rbac.LoadPermissions(path, logger, metrics, tracer)
	if err != nil {
		return fmt.Errorf("failed to load RBAC config: %w", err)
	}

	a.Logger().Infof("Loaded RBAC config successfully")

	// Apply middleware using the config
	middlewareFunc := rbac.Middleware(config)
	a.UseMiddleware(middlewareFunc)

	a.rbacConfig = config

	return nil
}

// rbacRouteCheckKey selects what the startup route check does on a mismatch. See rbacRouteCheckMode.
const rbacRouteCheckKey = "GOFR_RBAC_ROUTE_CHECK"

// Values of rbacRouteCheckKey.
const (
	rbacRouteCheckWarn = "warn"
	rbacRouteCheckFail = "fail"
	rbacRouteCheckOff  = "off"
)

// rbacRouteCheckMode reads rbacRouteCheckKey. A value it does not know is logged and read as
// rbacRouteCheckWarn: failing closed on a typo in a diagnostic setting would stop an app that has
// nothing wrong with it, and switching the check off would hide the mismatches it was set to show.
func (a *App) rbacRouteCheckMode() string {
	mode := strings.ToLower(strings.TrimSpace(a.Config.GetOrDefault(rbacRouteCheckKey, rbacRouteCheckWarn)))

	switch mode {
	case rbacRouteCheckWarn, rbacRouteCheckFail, rbacRouteCheckOff:
		return mode
	default:
		a.Logger().Errorf("invalid %s=%q: use %q, %q or %q. Using %q.", rbacRouteCheckKey, mode,
			rbacRouteCheckWarn, rbacRouteCheckFail, rbacRouteCheckOff, rbacRouteCheckWarn)

		return rbacRouteCheckWarn
	}
}

// rbacRouteCollector builds the route list the RBAC route check reads, from the router walk
// httpServerSetup already makes. A disabled collector adds nothing.
type rbacRouteCollector struct {
	enabled bool
	seen    map[string]bool
	labels  []string
}

func newRBACRouteCollector(enabled bool) *rbacRouteCollector {
	return &rbacRouteCollector{enabled: enabled, seen: make(map[string]bool)}
}

// add records route as one "METHOD /template" entry per method, "*" for a route with no method
// matcher. A prefix route gets a trailing catch-all segment, so that it stands for every path under
// the prefix. GoFr's PathPrefix("/") catch-all is left out: it answers 404 for every path no other
// route serves, and would otherwise make every rule look live.
func (c *rbacRouteCollector) add(route *mux.Route, methods []string) {
	if !c.enabled {
		return
	}

	tmpl, err := route.GetPathTemplate()
	if err != nil {
		return
	}

	// mux anchors a Path regexp with "$" and leaves a PathPrefix one open.
	pathRegexp, _ := route.GetPathRegexp()
	if !strings.HasSuffix(pathRegexp, "$") {
		if _, isCatchAll := route.GetHandler().(handler); isCatchAll && tmpl == "/" {
			return
		}

		tmpl = strings.TrimSuffix(tmpl, "/") + "/{path:.*}"
	}

	if len(methods) == 0 {
		methods = []string{"*"}
	}

	for _, method := range methods {
		label := strings.ToUpper(method) + " " + tmpl
		if c.seen[label] {
			continue
		}

		c.seen[label] = true
		c.labels = append(c.labels, label)
	}
}
