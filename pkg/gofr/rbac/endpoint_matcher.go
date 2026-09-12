package rbac

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

const (
	// DefaultConfigPath is the default config path value (empty string).
	// When passed to ResolveRBACConfigPath, it will try default paths: configs/rbac.json, configs/rbac.yaml, configs/rbac.yml.
	DefaultConfigPath = ""

	// Default RBAC config paths (tried in order).
	defaultRBACJSONPath = "configs/rbac.json"
	defaultRBACYAMLPath = "configs/rbac.yaml"
	defaultRBACYMLPath  = "configs/rbac.yml"
)

var (
	// errUnbalancedBraces is returned when a mux pattern has unbalanced braces.
	errUnbalancedBraces = errors.New("unbalanced braces in pattern")

	// errInvalidPattern is returned when mux cannot compile a pattern.
	errInvalidPattern = errors.New("invalid mux pattern")
)

// isMuxPattern detects if a pattern contains mux-style variables.
// Returns true if pattern contains { and }.
func isMuxPattern(pattern string) bool {
	return strings.Contains(pattern, "{") && strings.Contains(pattern, "}")
}

// compilePattern compiles an endpoint path into a mux route, once, at load time.
// Returns nil for a literal path, which is cheaper to compare as a string.
//
// Each pattern is compiled onto a router of its own. mux.Router.NewRoute appends to the router it
// is called on, so compiling against a router shared by every request - as this package used to -
// both mutated that router without synchronization and grew it by one entry per rule per request,
// for the life of the process. A router built here is never written to again, and Route.Match only
// reads the route, so the compiled result is safe to share across goroutines.
func compilePattern(pattern string) *mux.Route {
	if pattern == "" || !isMuxPattern(pattern) {
		return nil
	}

	// StrictSlash(false) matches the application router's behavior.
	return mux.NewRouter().StrictSlash(false).NewRoute().Path(pattern)
}

// matchContext carries the values one resolution scan matches its compiled routes against.
//
// mux writes its result into the RouteMatch, so a single one is reused across the scan and reset
// before each use; nothing here reads it. Hoisting it out of the per-rule call keeps the scan's
// allocation count flat instead of growing with the number of rules in the config.
type matchContext struct {
	req   *http.Request
	match mux.RouteMatch
}

// newMatchContext builds the context for resolving one request. mux matchers only read the
// request, so one value serves the whole scan.
func newMatchContext(methodUpper, path string) *matchContext {
	return &matchContext{
		req: &http.Request{
			Method: methodUpper,
			URL: &url.URL{
				Path: path,
			},
		},
	}
}

// matches reports whether the context's request matches a route returned by compilePattern.
// A pattern mux could not compile carries the error on the route and matches nothing, which is the
// unguarded-endpoint case logUncompilablePattern reports at load.
func (m *matchContext) matches(route *mux.Route) bool {
	m.match = mux.RouteMatch{}

	return route.Match(m.req, &m.match)
}

// validateMuxPattern validates mux pattern syntax.
// Ensures balanced braces and validates regex constraints format.
//
// Whether mux can actually compile the pattern is checked separately and non-fatally by
// logUncompilablePattern - see the note there.
func validateMuxPattern(pattern string) error {
	// Check for balanced braces
	openCount := strings.Count(pattern, "{")

	closeCount := strings.Count(pattern, "}")

	if openCount != closeCount {
		return fmt.Errorf("%w: %s", errUnbalancedBraces, pattern)
	}

	// Check that if there are closing braces, there must be opening braces
	// A pattern like "/api/id}" should not be valid
	if closeCount > 0 && openCount == 0 {
		return fmt.Errorf("%w: %s", errUnbalancedBraces, pattern)
	}

	return nil
}

// logUncompilablePattern reports, at error level, an endpoint pattern that mux cannot compile -
// an unterminated character class such as "{id:[}", for example.
//
// It does not fail the load. A pattern like that passes the brace-balance check, loads cleanly,
// and then never matches any request, so the endpoint it was written to govern is left unguarded:
// the same failure shape as an unreachable wildcard-method rule, reached a different way. Refusing
// to start would close that hole, but GoFr's position is to log and stay up rather than abort on a
// config defect (gofr-dev/gofr#2378), and reversing that is not something a bugfix should do.
// Closing it properly needs the fail-closed default for unmatched routes tracked in #3935; until
// then the operator gets a loud line naming the pattern instead of silence.
func (c *Config) logUncompilablePattern(pattern string, index int) {
	err := mux.NewRouter().NewRoute().Path(pattern).GetError()
	if err == nil || c.Logger == nil {
		return
	}

	c.Logger.Errorf("RBAC: endpoint[%d]: %v: %q: %v. This endpoint will never match a request, "+
		"so it is NOT enforced - any route it was meant to govern is currently unguarded.",
		index, errInvalidPattern, pattern, err)
}

// checkEndpointAuthorization checks if the user's role is authorized for the endpoint.
// Pure permission-based: checks if role has ANY of the required permissions (OR logic).
// Uses the endpoint parameter directly instead of re-looking it up.
func checkEndpointAuthorization(role string, endpoint *EndpointMapping, config *Config) (allowed bool, reason string) {
	// Public endpoints are always allowed
	if endpoint.Public {
		return true, "public-endpoint"
	}

	// Get required permissions
	requiredPerms := endpoint.RequiredPermissions

	// If no permission requirement found, deny (fail secure)
	if len(requiredPerms) == 0 {
		return false, ""
	}

	// Get role's permissions (thread-safe)
	rolePerms := config.GetRolePermissions(role)
	if len(rolePerms) == 0 {
		return false, ""
	}

	// Check if role has ANY of the required permissions (OR logic)
	// Only exact matches are supported - wildcards are NOT supported in permissions
	for _, requiredPerm := range requiredPerms {
		for _, perm := range rolePerms {
			// Exact match only - no wildcard support
			if perm == requiredPerm {
				return true, "permission-based"
			}
		}
	}

	return false, ""
}

// getEndpointForRequest finds the matching endpoint configuration for a request.
// This is the primary function used by the middleware to determine authorization requirements.
// Uses optimized maps for O(1) exact matches, falls back to pattern matching for mux patterns.
func getEndpointForRequest(r *http.Request, config *Config) (*EndpointMapping, bool) {
	if len(config.Endpoints) == 0 {
		return nil, false
	}

	return config.resolve(strings.ToUpper(r.Method), r.URL.Path)
}

// ResolveRBACConfigPath resolves the RBAC config file path.
// If configFile is empty, tries default paths in order: configs/rbac.json, configs/rbac.yaml, configs/rbac.yml.
func ResolveRBACConfigPath(configFile string) string {
	// If custom path provided, use it
	if configFile != "" {
		return configFile
	}

	// Try default paths in order
	defaultPaths := []string{
		defaultRBACJSONPath,
		defaultRBACYAMLPath,
		defaultRBACYMLPath,
	}

	for _, path := range defaultPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}
