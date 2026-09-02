package middleware

import (
	"net/http"
	"slices"
	"strings"

	gofrHTTP "gofr.dev/pkg/gofr/http"
)

const (
	allowedHeaders = "Authorization, Content-Type, x-requested-with, origin, true-client-ip, X-Correlation-ID"

	headerAccessControlAllowOrigin  = "Access-Control-Allow-Origin"
	headerAccessControlAllowMethods = "Access-Control-Allow-Methods"
	headerAccessControlAllowHeaders = "Access-Control-Allow-Headers"

	// headerAcceptQuery advertises QUERY (RFC 10008) support and the request
	// media types a QUERY body may use, mirroring how Access-Control-Allow-Methods
	// is derived from the registered routes.
	headerAcceptQuery = "Accept-Query"
	acceptQueryTypes  = "application/json"
)

// CORS is a middleware that adds CORS (Cross-Origin Resource Sharing) headers to the response.
// It supports multiple allowed origins via comma-separated values in the
// Access-Control-Allow-Origin config. When multiple origins are configured,
// the middleware dynamically matches the request's Origin header and responds
// with the matched origin, adding a Vary: Origin header for correct caching.
func CORS(middlewareConfigs map[string]string, routes *[]string) func(inner http.Handler) http.Handler {
	configs := canonicalizeConfig(middlewareConfigs)
	allowedOrigins := parseOrigins(configs[headerAccessControlAllowOrigin])

	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			setMiddlewareHeaders(configs, *routes, w, r.Header.Get("Origin"), allowedOrigins)

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			inner.ServeHTTP(w, r)
		})
	}
}

func setMiddlewareHeaders(middlewareConfigs map[string]string, routes []string,
	w http.ResponseWriter, origin string, allowedOrigins map[string]bool,
) {
	// Advertise QUERY support (RFC 10008) via Accept-Query when a QUERY route is
	// registered — the same registered-routes source that builds Allow-Methods.
	// Read from the caller's routes slice without appending; joinAllowedMethods
	// below handles OPTIONS suffixing safely.
	if slices.Contains(routes, gofrHTTP.MethodQuery) {
		w.Header().Set(headerAcceptQuery, acceptQueryTypes)
	}

	// Handle Access-Control-Allow-Origin separately for dynamic matching.
	if allowedOrigins["*"] {
		w.Header().Set(headerAccessControlAllowOrigin, "*")
	} else if allowedOrigins[origin] {
		w.Header().Set(headerAccessControlAllowOrigin, origin)
		w.Header().Add("Vary", "Origin")
	}

	// The keys arrive canonicalized from canonicalizeConfig, so a differently-cased
	// spelling cannot reach a header it was never checked against — replacing the
	// origin negotiated above, or overwriting the default allow-list instead of
	// extending it.
	var customMethods, customHeaders string

	for key, value := range middlewareConfigs {
		switch key {
		case headerAccessControlAllowOrigin:
			// Always negotiated against the configured allow-list above; never
			// overridable from here.
		case headerAccessControlAllowMethods:
			customMethods = value
		case headerAccessControlAllowHeaders:
			customHeaders = value
		default:
			w.Header().Set(key, value)
		}
	}

	// A configured method list replaces the derived one; configured headers
	// EXTEND the defaults rather than replacing them, so the framework's own
	// required headers (Authorization, Content-Type, X-Correlation-ID, ...)
	// cannot be dropped by adding one of your own.
	if customMethods == "" {
		customMethods = joinAllowedMethods(routes)
	}

	allowHeaderValue := allowedHeaders
	if customHeaders != "" {
		allowHeaderValue = allowedHeaders + ", " + customHeaders
	}

	w.Header().Set(headerAccessControlAllowMethods, customMethods)
	w.Header().Set(headerAccessControlAllowHeaders, allowHeaderValue)
}

// joinAllowedMethods renders the Access-Control-Allow-Methods value: the
// registered routes plus OPTIONS.
//
// It deliberately does not append to routes. That slice shares its backing
// array with the caller's (the router's RegisteredRoutes), so appending in
// place writes "OPTIONS" over the caller's next element whenever cap > len.
func joinAllowedMethods(routes []string) string {
	if len(routes) == 0 {
		return http.MethodOptions
	}

	return strings.Join(routes, ", ") + ", " + http.MethodOptions
}

// canonicalizeConfig folds the caller's configuration onto canonical header keys, once, at setup.
//
// Two things depend on this. The allow-list read by parseOrigins used a raw literal lookup while the
// per-request walk classified by canonical name, so the two disagreed: a caller who spelled the key
// "access-control-allow-origin" had it dropped by the classifier AND missed by parseOrigins, which
// then fell through to its wildcard default — turning a config that restricted the origin into one
// that echoed "*" to an unlisted origin. CORS is exported, so callers do build this map themselves.
//
// The other is determinism. Header names are case-insensitive, so two spellings are one header, and
// a map has no order — resolving the collision during the per-request walk meant the winner was
// whichever key map iteration reached last, and a different value could be sent on different
// requests within a single process. Precedence is explicit here instead: an exactly-canonical
// spelling always wins, and among the rest the lexicographically smallest key does.
func canonicalizeConfig(cfg map[string]string) map[string]string {
	canonical := make(map[string]string, len(cfg))

	// Track which raw key won each canonical slot, so precedence does not depend on iteration order.
	winner := make(map[string]string, len(cfg))

	for key, value := range cfg {
		ck := http.CanonicalHeaderKey(key)

		if prev, ok := winner[ck]; ok && !better(key, prev, ck) {
			continue
		}

		winner[ck] = key
		canonical[ck] = value
	}

	return canonical
}

// better reports whether raw key a should beat b for the canonical slot ck.
func better(a, b, ck string) bool {
	if (a == ck) != (b == ck) {
		return a == ck
	}

	return a < b
}

// parseOrigins splits a comma-separated origin string into a set.
// An empty string defaults to wildcard ("*").
func parseOrigins(raw string) map[string]bool {
	if raw == "" {
		return map[string]bool{"*": true}
	}

	origins := make(map[string]bool)

	for _, o := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			origins[trimmed] = true
		}
	}

	if len(origins) == 0 {
		return map[string]bool{"*": true}
	}

	return origins
}
