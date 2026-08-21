package middleware

import (
	"net/http"
	"net/textproto"
	"strings"
	"sync"
)

const (
	allowedHeaders = "Authorization, Content-Type, x-requested-with, origin, true-client-ip, X-Correlation-ID"

	headerAccessControlAllowMethods = "Access-Control-Allow-Methods"
	headerAccessControlAllowHeaders = "Access-Control-Allow-Headers"
	headerAccessControlAllowOrigin  = "Access-Control-Allow-Origin"
)

// Canonical keys and the shared wildcard value, resolved once at package level
// so no response pays for canonicalization or a value-slice allocation.
//
//nolint:gochecknoglobals // immutable, process-wide header constants.
var (
	canonicalAllowOrigin  = textproto.CanonicalMIMEHeaderKey(headerAccessControlAllowOrigin)
	canonicalAllowMethods = textproto.CanonicalMIMEHeaderKey(headerAccessControlAllowMethods)
	canonicalAllowHeaders = textproto.CanonicalMIMEHeaderKey(headerAccessControlAllowHeaders)
	wildcardOrigin        = sharedValue("*")
)

// sharedValue builds a header value slice that is safe to share across every
// response.
//
// The safety rests entirely on len == cap == 1, which a one-element composite
// literal guarantees: a later Header.Add on the same key sees no spare capacity
// and appends into a NEWLY allocated array instead of writing into this one.
// Every shared value goes through this function so the invariant has a single
// place to be stated, and TestSharedHeaderValuesHaveNoSpareCapacity asserts it
// for all of them -- a slice with room to grow would let one response scribble
// on every other response's headers, process-wide.
func sharedValue(v string) []string {
	return []string{v}
}

// CORS is a middleware that adds CORS (Cross-Origin Resource Sharing) headers to the response.
// It supports multiple allowed origins via comma-separated values in the
// Access-Control-Allow-Origin config. When multiple origins are configured,
// the middleware dynamically matches the request's Origin header and responds
// with the matched origin, adding a Vary: Origin header for correct caching.
func CORS(middlewareConfigs map[string]string, routes *[]string) func(inner http.Handler) http.Handler {
	// Every header this middleware writes, apart from Allow-Origin, is a pure
	// function of the configuration and the registered route set. Both are fixed
	// before the first request, yet they were recomputed on every one: a map
	// literal, a strings.Join over the route list, and an append into the
	// caller's slice.
	//
	// LIFECYCLE CONTRACT: the configuration map AND the route slice are both read
	// exactly once, on the first request, and never again.
	//
	// "On the first request" rather than here, because the route list is still
	// being appended to while handlers register and CORS is constructed before
	// that finishes. GoFr completes registration in httpServerSetup, which runs
	// synchronously before the serve goroutine starts and assigns the final method
	// list, so the set is complete by the time this runs -- the same lifecycle the
	// router's own index relies on.
	//
	// The config is read at the same moment, deliberately. Reading it at
	// construction instead would freeze the two halves of the same configuration at
	// two different instants, which is a difference nothing documents and nobody
	// would expect. One instant, one rule.
	//
	// A caller that mutates either input after the first request served will not
	// see the change. That is not a regression to be fixed by reading them per
	// request: these are read from every serving goroutine, so a caller mutating
	// them concurrently is a data race regardless of when we read. Freezing makes
	// the race impossible instead of merely unlikely. TestCORSLifecycleContract
	// pins all of it.
	var (
		once           sync.Once
		allowedOrigins map[string]bool
		fixed          []headerValue
		methods        []string
	)

	// sync.Once establishes happens-before between this build and every later
	// request that observes it, so the assignments below are safely published to
	// all serving goroutines without a further lock.
	build := func() {
		configs := canonicalizeConfig(middlewareConfigs)
		allowedOrigins = parseOrigins(configs[headerAccessControlAllowOrigin])
		fixed, methods = buildFixedHeaders(configs, *routes)
	}

	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			once.Do(build)
			setMiddlewareHeaders(w, r.Header.Get("Origin"), allowedOrigins, fixed, methods)

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			inner.ServeHTTP(w, r)
		})
	}
}

// headerValue is a precomputed response header, held in the shape net/http
// stores it: a canonical key and a one-element value slice.
//
// Writing it with Header.Set would canonicalize the key and allocate a fresh
// []string on every response. Assigning the map entry directly does neither.
// Sharing the slice across responses is safe because it has len == cap == 1, so
// a later Header.Add appends into a NEW array rather than mutating this one --
// pinned by TestSharedHeaderValueSurvivesAdd.
type headerValue struct {
	key   string
	value []string
}

// buildFixedHeaders computes every header whose value cannot change once routes
// are registered. It returns them alongside the Access-Control-Allow-Methods
// value, which is kept separate only because it is the one derived from routes.
//
// middlewareConfigs arrives already folded onto canonical header keys by
// canonicalizeConfig. That matters for more than tidiness: this function
// classifies entries by key and writes everything it does not recognize into
// fixed, and fixed is applied AFTER the per-request origin negotiation. Matching
// on the raw key would send a config spelled "access-control-allow-origin" to the
// default branch, where it would be canonicalized into the very header the
// allow-list just negotiated and overwrite it.
//
// The method list is joined without appending to the caller's slice: that slice
// is the router's RegisteredRoutes, and appending in place would write into it
// whenever its capacity exceeded its length.
func buildFixedHeaders(middlewareConfigs map[string]string, routes []string) (fixed []headerValue, methods []string) {
	allowMethods := joinAllowedMethods(routes)
	if custom, ok := middlewareConfigs[headerAccessControlAllowMethods]; ok && custom != "" {
		allowMethods = custom
	}

	methods = sharedValue(allowMethods)

	allowHeaders := allowedHeaders
	if custom, ok := middlewareConfigs[headerAccessControlAllowHeaders]; ok && custom != "" {
		allowHeaders = allowedHeaders + ", " + custom
	}

	fixed = append(fixed, headerValue{
		key:   canonicalAllowHeaders,
		value: sharedValue(allowHeaders),
	})

	// Any other configured header is passed through unchanged. Allow-Origin is
	// excluded because it is negotiated per request against the allow-list.
	for header, value := range middlewareConfigs {
		switch header {
		case headerAccessControlAllowOrigin, headerAccessControlAllowMethods, headerAccessControlAllowHeaders:
			continue
		default:
			fixed = append(fixed, headerValue{key: header, value: sharedValue(value)})
		}
	}

	return fixed, methods
}

// joinAllowedMethods renders Access-Control-Allow-Methods: the registered routes
// plus OPTIONS.
func joinAllowedMethods(routes []string) string {
	if len(routes) == 0 {
		return http.MethodOptions
	}

	return strings.Join(routes, ", ") + ", " + http.MethodOptions
}

// setMiddlewareHeaders writes the CORS headers for one response. Only
// Allow-Origin depends on the request; everything else was built once.
//
// The value slices are shared process-wide rather than copied per response, which is safe only
// because each has len == cap == 1: a later Header.Add on the same key appends into a NEWLY
// allocated array instead of writing into the shared one. TestSharedHeaderValueSurvivesAdd pins
// that. Anything added here must keep that property -- a slice with spare capacity would let one
// response scribble on every other response's header.
func setMiddlewareHeaders(w http.ResponseWriter, origin string, allowedOrigins map[string]bool,
	fixed []headerValue, methods []string) {
	header := w.Header()

	if allowedOrigins["*"] {
		header[canonicalAllowOrigin] = wildcardOrigin
	} else if allowedOrigins[origin] {
		// The value varies per request, so this one cannot be shared.
		header.Set(headerAccessControlAllowOrigin, origin)
		header.Add("Vary", "Origin")
	}

	header[canonicalAllowMethods] = methods

	for _, h := range fixed {
		header[h.key] = h.value
	}
}

// canonicalizeConfig folds the caller's configuration onto canonical header keys,
// once, at construction.
//
// Header names are case-insensitive and net/http canonicalizes whatever it is
// given, so two spellings are one header. Without this fold the allow-list read
// by parseOrigins used a raw literal lookup while buildFixedHeaders classified by
// its own raw key, and the two disagreed: a caller spelling the key
// "access-control-allow-origin" -- CORS is exported, so callers do build this map
// -- had it missed by parseOrigins, which fell back to its wildcard default, AND
// routed to buildFixedHeaders' default branch, which canonicalizes it into fixed.
// Since fixed is applied after origin negotiation, that config both lost its
// allow-list and overwrote the negotiated header.
//
// Precedence is explicit rather than left to map iteration order, which would
// otherwise let two spellings of one header resolve differently on different
// requests within a single process: an exactly-canonical spelling always wins,
// and among the rest the lexicographically smallest key does.
func canonicalizeConfig(cfg map[string]string) map[string]string {
	canonical := make(map[string]string, len(cfg))
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
