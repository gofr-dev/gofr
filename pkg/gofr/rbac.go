package gofr

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/internal/rbaccheck"
	"gofr.dev/pkg/gofr/internal/routemark"
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
// - Set "jwtClaimPath" for JWT-based extraction (e.g., "role", "roles", "roles[0]"); an array claim
// holds every role in it.
// - Set "permissionsClaimPath" (e.g., "scope") to authorize from the permissions in the token
// itself. JWT-based modes need EnableOAuth to be called first; validate the token's audience,
// issuer and expiry there, with jwt.WithAudience, jwt.WithIssuer and jwt.WithExpirationRequired.
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

	config, err := a.loadRBACConfig(path)
	if err != nil {
		return err
	}

	// RBAC metrics are opt-in and registered here (like GraphQL/gRPC/cron/LLM), when the first config
	// is installed. EnableRBAC can run more than once, so guard on the first call to avoid a
	// duplicate-metric error; this runs in the single-threaded setup phase, so no synchronization is
	// needed.
	if metrics := a.Metrics(); !a.rbacEnabled && metrics != nil {
		metrics.NewCounter("rbac_role_extraction_failures", "Number of requests whose RBAC role or permissions could not be read")
	}

	a.rbacEnabled = true

	// Apply middleware using the config
	middlewareFunc := rbac.Middleware(config)
	a.UseMiddleware(middlewareFunc)

	a.rbacConfigs = append(a.rbacConfigs, config)

	return nil
}

// loadRBACConfig loads the config at path and logs the startup summary. The load logs through a
// counting logger, so the summary can say that errors were logged instead of reading as a clean
// success. The returned config logs through the app logger, not the counting wrapper.
func (a *App) loadRBACConfig(path string) (*rbac.Config, error) {
	logger := a.Logger()
	loadLogger := &errorCountingLogger{Logger: logger}

	config, err := rbac.LoadPermissions(path, loadLogger, a.Metrics(), otel.GetTracerProvider().Tracer("gofr-rbac"))
	if err != nil {
		return nil, fmt.Errorf("failed to load RBAC config: %w", err)
	}

	config.Logger = logger

	if loadLogger.errors > 0 {
		logger.Errorf("Loaded RBAC config with %d error(s); see the RBAC errors logged above", loadLogger.errors)
	} else {
		logger.Infof("Loaded RBAC config successfully")
	}

	return config, nil
}

// errorCountingLogger counts the error lines logged through it while the RBAC config loads.
type errorCountingLogger struct {
	datasource.Logger
	errors int
}

func (l *errorCountingLogger) Error(args ...any) {
	l.errors++
	l.Logger.Error(args...)
}

func (l *errorCountingLogger) Errorf(format string, args ...any) {
	l.errors++
	l.Logger.Errorf(format, args...)
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

// checkRBACRoutes runs the RBAC route check against the route table httpServerSetup collected, and
// logs one line for each kind of mismatch it finds. It reports whether any mismatch can leave a
// request served without role checks.
func (a *App) checkRBACRoutes() bool {
	endpoints := make([][]rbac.EndpointMapping, len(a.rbacConfigs))
	for i, config := range a.rbacConfigs {
		endpoints[i] = config.Endpoints
	}

	m := rbaccheck.CheckAll(endpoints, a.rbacRoutes)
	logger := a.Logger()

	if len(m.DeadPublicRules) > 0 {
		logger.Warnf("RBAC: public rules match no registered route: %s", strings.Join(m.DeadPublicRules, ", "))
	}

	if len(m.DeadRules) > 0 {
		logger.Errorf("RBAC route check failed: rules match no registered route: %s. These rules protect "+
			"nothing; fix each rule's path or methods, or remove it.", strings.Join(m.DeadRules, ", "))
	}

	if len(m.Uncovered) > 0 {
		logger.Errorf("RBAC route check failed: routes covered by no rule: %s. They are served without role "+
			"checks; add a rule for each, with \"public\": true for a route meant to be open.", strings.Join(m.Uncovered, ", "))
	}

	if len(m.PartlyCovered) > 0 {
		logger.Errorf("RBAC route check failed: routes only partly covered: %s. Requests these rules do not "+
			"match are served without role checks; add a rule that matches each route in full.", strings.Join(m.PartlyCovered, ", "))
	}

	return m.Failed()
}

// rbacRouteCollector builds the route table the RBAC route check reads, from the router walk
// httpServerSetup already makes. A disabled collector adds nothing.
type rbacRouteCollector struct {
	enabled bool
	seen    map[rbaccheck.Route]bool

	// builtInFrom and builtInTo bound, as positions in the router walk, the routes GoFr registers
	// itself; walked is the position of the next route add sees. The routes are told apart by
	// where they were registered rather than by their path, so that an application route under
	// /.well-known/ is still checked.
	builtInFrom, builtInTo, walked int

	routes []rbaccheck.Route
}

func newRBACRouteCollector(enabled bool) *rbacRouteCollector {
	return &rbacRouteCollector{enabled: enabled, seen: make(map[rbaccheck.Route]bool)}
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

// add records route once per method, "*" for a route with no method matcher, as built in when it
// falls between startBuiltIns and endBuiltIns. A prefix route gets a trailing catch-all segment, so
// that it stands for every path under the prefix.
// GoFr's PathPrefix("/") catch-all is left out: it answers 404 for every path no other route
// serves, and would otherwise make every rule look live.
func (c *rbacRouteCollector) add(route *mux.Route, methods []string) {
	if !c.enabled {
		return
	}

	builtIn := c.nextIsBuiltIn()

	if routemark.IsCatchAll(route) {
		return
	}

	tmpl, err := route.GetPathTemplate()
	if err != nil {
		return
	}

	// mux anchors a Path regexp with "$" and leaves a PathPrefix one open.
	pathRegexp, _ := route.GetPathRegexp()
	if !strings.HasSuffix(pathRegexp, "$") {
		tmpl = strings.TrimSuffix(tmpl, "/") + "/{path:.*}"
	}

	if len(methods) == 0 {
		methods = []string{"*"}
	}

	for _, method := range methods {
		r := rbaccheck.Route{Method: strings.ToUpper(method), Path: tmpl, BuiltIn: builtIn}
		if c.seen[r] {
			continue
		}

		c.seen[r] = true
		c.routes = append(c.routes, r)
	}
}
