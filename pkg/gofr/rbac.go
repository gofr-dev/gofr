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

// rbacRouteCollector builds the route lists the RBAC route check reads, from the router walk
// httpServerSetup already makes. A disabled collector adds nothing.
type rbacRouteCollector struct {
	enabled bool
	seen    map[string]bool

	// builtInFrom and builtInTo bound, as positions in the router walk, the routes GoFr registers
	// itself; walked is the position of the next route add sees. The routes are told apart by
	// where they were registered rather than by their path, so that an application route under
	// /.well-known/ is still checked.
	builtInFrom, builtInTo, walked int

	labels, builtInLabels []string
}

func newRBACRouteCollector(enabled bool) *rbacRouteCollector {
	return &rbacRouteCollector{enabled: enabled, seen: make(map[string]bool)}
}

// startBuiltIns marks the routes registered on router from now on as GoFr's own, until endBuiltIns.
func (c *rbacRouteCollector) startBuiltIns(router *mux.Router) {
	if c.enabled {
		c.builtInFrom = countRoutes(router)
	}
}

// endBuiltIns ends the run of routes startBuiltIns began.
func (c *rbacRouteCollector) endBuiltIns(router *mux.Router) {
	if c.enabled {
		c.builtInTo = countRoutes(router)
	}
}

// nextIsBuiltIn reports whether the route at the next walk position is one of GoFr's own, and
// moves past it.
func (c *rbacRouteCollector) nextIsBuiltIn() bool {
	position := c.walked
	c.walked++

	return position >= c.builtInFrom && position < c.builtInTo
}

// countRoutes returns how many routes a walk of router visits.
func countRoutes(router *mux.Router) int {
	n := 0

	_ = router.Walk(func(*mux.Route, *mux.Router, []*mux.Route) error {
		n++

		return nil
	})

	return n
}

// add records route as one "METHOD /template" entry per method, "*" for a route with no method
// matcher, among the built-in routes when it falls between startBuiltIns and endBuiltIns. A prefix
// route gets a trailing catch-all segment, so that it stands for every path under the prefix.
// GoFr's PathPrefix("/") catch-all is left out: it answers 404 for every path no other route
// serves, and would otherwise make every rule look live.
func (c *rbacRouteCollector) add(route *mux.Route, methods []string) {
	if !c.enabled {
		return
	}

	builtIn := c.nextIsBuiltIn()

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

		if builtIn {
			c.builtInLabels = append(c.builtInLabels, label)
		} else {
			c.labels = append(c.labels, label)
		}
	}
}
