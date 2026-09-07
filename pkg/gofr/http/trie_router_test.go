package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file is the differential ("characterization") gate for the trie router.
// The discipline: register an identical route set on the default mux router and
// on the trie router, replay the same requests through both, and require the
// observable outcome — status code, body, and extracted path params — to be
// byte-for-byte identical. mux is the oracle; any divergence is either a bug to
// fix or a consciously documented intentional change.

// routeDef is one route registration used by both routers.
type routeDef struct {
	method  string
	pattern string
}

// echoHandler reports, as JSON, exactly what the router resolved for a request:
// the path params (via mux.Vars, which must work under both routers) and the
// route template. Comparing these across routers proves they matched the same
// route with the same variables.
func echoHandler(tag string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		keys := make([]string, 0, len(vars))
		for k := range vars {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		ordered := make([][2]string, 0, len(keys))
		for _, k := range keys {
			ordered = append(ordered, [2]string{k, vars[k]})
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"tag": tag, "vars": ordered})
	})
}

// buildRouter constructs a Router in the requested mode with the given routes.
func buildRouter(routes []routeDef, useTrie bool) *Router {
	r := NewRouter()
	r.useTrie = useTrie

	for _, rd := range routes {
		r.Add(rd.method, rd.pattern, echoHandler(rd.method+" "+rd.pattern))
	}

	return r
}

// serve runs one request and returns the response status and body.
func serve(router *Router, method, target string) (status int, body string) {
	req := httptest.NewRequestWithContext(context.Background(), method, target, http.NoBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec.Code, rec.Body.String()
}

type reqCase struct {
	method string
	target string
}

// runDifferential registers routes on both a mux and a trie router and asserts
// every request yields an identical (status, body) from both.
func runDifferential(t *testing.T, name string, routes []routeDef, cases []reqCase) {
	t.Helper()

	muxR := buildRouter(routes, false)
	trieR := buildRouter(routes, true)

	for _, c := range cases {
		t.Run(fmt.Sprintf("%s/%s_%s", name, c.method, c.target), func(t *testing.T) {
			muxStatus, muxBody := serve(muxR, c.method, c.target)
			trieStatus, trieBody := serve(trieR, c.method, c.target)

			require.Equalf(t, muxStatus, trieStatus,
				"status differs for %s %s (mux=%d trie=%d)", c.method, c.target, muxStatus, trieStatus)
			assert.Equalf(t, muxBody, trieBody,
				"body differs for %s %s", c.method, c.target)
		})
	}
}

func TestTrieDifferential_StaticAndParams(t *testing.T) {
	routes := []routeDef{
		{http.MethodGet, "/users"},
		{http.MethodGet, "/users/me"},         // static beats param when registered first
		{http.MethodGet, "/users/{id}"},       // single param
		{http.MethodGet, "/users/{id}/posts"}, // nested static after param
		{http.MethodGet, "/orgs/{org}/repos/{repo}"},
		{http.MethodPost, "/users/{id}"}, // same path, different method
		{http.MethodGet, "/items/{id:[0-9]+}"},
	}

	cases := []reqCase{
		{http.MethodGet, "/users"},
		{http.MethodGet, "/users/me"},       // must hit the static route
		{http.MethodGet, "/users/42"},       // must hit {id}
		{http.MethodGet, "/users/42/posts"}, // nested
		{http.MethodGet, "/orgs/gofr/repos/x"},
		{http.MethodPost, "/users/42"},        // POST variant
		{http.MethodGet, "/items/123"},        // regex passes
		{http.MethodGet, "/items/abc"},        // regex fails -> 404
		{http.MethodDelete, "/users/42"},      // no DELETE -> 405
		{http.MethodGet, "/nonexistent"},      // unknown -> 404
		{http.MethodGet, "/users/42/unknown"}, // partial depth -> 404
	}

	runDifferential(t, "static_params", routes, cases)
}

func TestTrieDifferential_OverlapOrder(t *testing.T) {
	// Param registered BEFORE the static — mux tries in registration order, so
	// /cfg/all is served by {key}. The trie must reproduce that exactly.
	routes := []routeDef{
		{http.MethodGet, "/cfg/{key}"},
		{http.MethodGet, "/cfg/all"},
	}

	cases := []reqCase{
		{http.MethodGet, "/cfg/all"}, // {key} wins because registered first
		{http.MethodGet, "/cfg/x"},
	}

	runDifferential(t, "overlap_order", routes, cases)
}

func TestTrieDifferential_TrailingSlashAndNormalization(t *testing.T) {
	routes := []routeDef{
		{http.MethodGet, "/a/b"},
		{http.MethodGet, "/"},
		{http.MethodGet, "/x/{id}"},
	}

	cases := []reqCase{
		{http.MethodGet, "/a/b"},
		{http.MethodGet, "/a/b/"},     // trailing slash normalized
		{http.MethodGet, "//a//b"},    // double slashes normalized
		{http.MethodGet, "/a/./b"},    // dot segment
		{http.MethodGet, "/a/c/../b"}, // dot-dot segment
		{http.MethodGet, "/"},
		{http.MethodGet, "/x/7?a=1&b=2"}, // query ignored, vars intact
	}

	runDifferential(t, "normalization", routes, cases)
}

// TestTrieDifferential_SlashSpanningParams covers routes whose param regex can
// match "/" and therefore span multiple request-path segments — catch-all file
// paths and proxy passthroughs, a standard mux idiom. These must go to the
// fallback list so mux's own matcher resolves them; the trie must not silently
// drop them.
func TestTrieDifferential_SlashSpanningParams(t *testing.T) {
	routes := []routeDef{
		{http.MethodGet, "/files/{path:.*}"},     // classic catch-all
		{http.MethodGet, "/proxy/{rest:.+}"},     // non-empty catch-all
		{http.MethodGet, "/mix/{a}/{b:[a-z/]+}"}, // slash allowed in a class
		{http.MethodGet, "/items/{id:[0-9]+}"},   // slash-free regex: stays fast in trie
		{http.MethodGet, "/plain/{name}"},        // plain param: single segment
	}

	cases := []reqCase{
		{http.MethodGet, "/files/a/b/c.txt"}, // spans 3 segments -> vars{path:a/b/c.txt}
		{http.MethodGet, "/files/single"},
		{http.MethodGet, "/files/"}, // empty catch-all (.* matches "")
		{http.MethodGet, "/proxy/x/y"},
		{http.MethodGet, "/proxy/"},        // .+ needs >=1 char -> 404 in both
		{http.MethodGet, "/mix/one/a/b/c"}, // {b} spans a/b/c
		{http.MethodGet, "/items/42"},      // regex passes
		{http.MethodGet, "/items/4/2"},     // extra segment -> 404 in both
		{http.MethodGet, "/plain/bob"},
		{http.MethodGet, "/plain/bob/extra"}, // extra segment -> 404 in both
	}

	runDifferential(t, "slash_spanning", routes, cases)
}

// mixedSegmentRoutes exercises every single-segment pattern shape mux supports:
// a param with a literal suffix, prefix, or both; regex params with an affix;
// adjacent and multi params; and a pure literal sharing a position with a mixed
// one. All match exactly one path segment, so the trie indexes each as a param
// slot and mux validates the intra-segment pattern.
func mixedSegmentRoutes() []routeDef {
	return []routeDef{
		{http.MethodGet, "/files/{name}.txt"},         // literal suffix
		{http.MethodGet, "/files/pinned.txt"},         // pure literal at the SAME position
		{http.MethodGet, "/user-{id}"},                // literal prefix
		{http.MethodGet, "/pre-{x}-post"},             // literal on both sides
		{http.MethodGet, "/v{ver}/x"},                 // mixed, then a static segment
		{http.MethodGet, "/{a}.{b}"},                  // two params + a literal separator
		{http.MethodGet, "/{lang}-{region}"},          // two params, whole-segment braces
		{http.MethodGet, "/img/{id:[0-9]+}.png"},      // regex param + literal suffix
		{http.MethodGet, "/mix/{y:[a-z]+}.json/tail"}, // regex+suffix, then static
		{http.MethodGet, "/plain/{id}"},               // control: pure param
	}
}

// TestTrieDifferential_MixedLiteralParamSegments covers segments that mix a
// literal with a parameter. These match exactly one path segment, so the trie
// indexes them as a parameter slot (mux validates the exact pattern) — they are
// NOT deferred to the fallback list. Verified byte-identical to mux across a
// battery of affix / regex / adjacency / overlap / negative cases.
func TestTrieDifferential_MixedLiteralParamSegments(t *testing.T) {
	cases := []reqCase{
		{http.MethodGet, "/files/report.txt"}, // vars{name:report}
		{http.MethodGet, "/files/report.csv"}, // wrong suffix -> 404 both
		{http.MethodGet, "/files/pinned.txt"}, // literal wins by registration order
		{http.MethodGet, "/files/a.b.txt"},    // dots inside the param value
		{http.MethodGet, "/user-42"},          // vars{id:42}
		{http.MethodGet, "/user-"},            // empty param -> 404 both
		{http.MethodGet, "/admin-42"},         // wrong prefix -> 404 both
		{http.MethodGet, "/pre-mid-post"},     // vars{x:mid}
		{http.MethodGet, "/v2/x"},             // vars{ver:2}
		{http.MethodGet, "/v/x"},              // empty param -> 404 both
		{http.MethodGet, "/x.y"},              // vars{a:x,b:y}
		{http.MethodGet, "/en-US"},            // vars{lang:en,region:US}
		{http.MethodGet, "/img/123.png"},      // regex passes
		{http.MethodGet, "/img/12a.png"},      // regex fails -> 404 both
		{http.MethodGet, "/mix/abc.json/tail"},
		{http.MethodGet, "/mix/ab1.json/tail"}, // regex fails -> 404 both
		{http.MethodGet, "/plain/9"},
		{http.MethodGet, "/plain/9/extra"}, // extra segment -> 404 both
	}

	runDifferential(t, "mixed_seg", mixedSegmentRoutes(), cases)
}

// TestTrieRouter_MixedSegmentsAreIndexed asserts the mixed/param routes are
// actually trie-indexed (not sitting in the linearly-scanned fallback list), so
// the O(path) property holds for them too.
func TestTrieRouter_MixedSegmentsAreIndexed(t *testing.T) {
	r := buildRouter(mixedSegmentRoutes(), true)

	r.buildIdx.Do(func() {
		idx := newRouteIndex()
		idx.build(&r.Router)
		r.idx = idx
	})

	assert.Empty(t, r.idx.fallback, "all slash-free mixed/param routes must be trie-indexed, not in fallback")
}

// TestTrieDifferential_MethodMismatchParity pins the case that used to be the
// one documented divergence between the routers, and now is not.
//
// mux's 404-vs-405 for a wrong-method request is registration-order dependent.
// In gorilla/mux's Route.Match, a matcher that SUCCEEDS clears any
// ErrMethodMismatch a previous route left behind, and GoFr registers routes as
// Methods(m).Path(p) — method matcher first. So below, the later GET /a/b clears
// the mismatch recorded by POST /a/{id} purely because its method matches, even
// though its path does not, and mux answers 404.
//
// That makes the 405 decision depend on routes whose path does not match the
// request — precisely the routes the trie exists to skip — so no amount of
// bookkeeping over the narrowed candidate set can reproduce it. serveTrie
// therefore does not try: it hands every non-match to mux's own ServeHTTP, which
// resolves it by the full scan. The two routers agree here by construction, not
// by coincidence.
func TestTrieDifferential_MethodMismatchParity(t *testing.T) {
	routes := []routeDef{
		{http.MethodPost, "/a/{id}"}, // only POST on this path
		{http.MethodGet, "/a/b"},     // a later, non-overlapping path
	}

	muxR := buildRouter(routes, false)
	trieR := buildRouter(routes, true)

	// GET /a/xyz: the path matches /a/{id}, the method does not.
	muxStatus, _ := serve(muxR, http.MethodGet, "/a/xyz")
	trieStatus, _ := serve(trieR, http.MethodGet, "/a/xyz")

	assert.Equal(t, http.StatusNotFound, muxStatus, "mux baseline: registration-order 404")
	assert.Equal(t, muxStatus, trieStatus, "trie must reproduce mux's order-dependent status, not a 405")
}

// TestTrieRouter_UnmatchedFallsBackToMux asserts the safety property that the
// delegation buys: a route missing from the trie index is still served.
//
// The index is normally built by serveTrie from the router's own routes, so this
// installs a deliberately blind one — every route diverted to neither the trie
// nor the fallback list — to simulate the worst outcome a classifier bug could
// produce. Without the delegation this request 404s even though the route is
// registered and matches; with it, mux's full scan finds the route and serves it,
// so a classifier bug costs speed rather than correctness.
func TestTrieRouter_UnmatchedFallsBackToMux(t *testing.T) {
	r := NewRouter()
	r.useTrie = true
	r.Add(http.MethodGet, "/users/{id}", echoHandler("real-handler"))

	// Pre-seed an empty index so the lazy build in serveTrie is skipped and the
	// trie can match nothing at all.
	r.buildIdx.Do(func() { r.idx = newRouteIndex() })

	status, body := serve(r, http.MethodGet, "/users/42")

	require.Equal(t, http.StatusOK, status, "an unindexed but registered route must still be served by mux")
	assert.Contains(t, body, "real-handler")
	assert.Contains(t, body, `"42"`, "mux must populate the path params it always would")
}

// TestTrieRouter_SubrouterMethodNotAllowedHandler is the other half of the match-with-error family,
// and it is deliberately the OPPOSITE expectation from the NotFoundHandler case above: here the
// middleware chain MUST run.
//
// A subrouter's MethodNotAllowedHandler also reports a successful match, but mux clears
// ErrMethodMismatch the moment one of the route's matchers succeeds (the else arm of the matcher loop
// in gorilla/mux route.go), so the match arrives with a nil MatchErr and mux builds its chain. A
// blanket "skip the chain whenever the handler came from an error path" would be wrong here, which is
// why the guard keys on MatchErr rather than on the kind of handler.
func TestTrieRouter_SubrouterMethodNotAllowedHandler(t *testing.T) {
	build := func(useTrie bool, mwRuns *int) *Router {
		r := NewRouter()
		r.useTrie = useTrie

		sub := r.Router.PathPrefix("/mna").Subrouter()
		sub.MethodNotAllowedHandler = echoHandler("SUB405")
		sub.Path("/only-post").Methods(http.MethodPost).Handler(echoHandler("mna-post"))

		r.UseMiddleware(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				*mwRuns++

				next.ServeHTTP(w, req)
			})
		})

		return r
	}

	for _, c := range []struct {
		method, target string
	}{
		{http.MethodPost, "/mna/only-post"},   // the real route
		{http.MethodDelete, "/mna/only-post"}, // wrong method -> the subrouter's 405 handler
		{http.MethodGet, "/mna/absent"},       // no inner route at all
	} {
		var muxN, trieN int

		muxStatus, muxBody := serve(build(false, &muxN), c.method, c.target)
		trieStatus, trieBody := serve(build(true, &trieN), c.method, c.target)

		require.Equalf(t, muxStatus, trieStatus, "status differs for %s %s", c.method, c.target)
		assert.Equalf(t, muxBody, trieBody, "handler differs for %s %s", c.method, c.target)
		assert.Equalf(t, muxN, trieN, "middleware count differs for %s %s (mux=%d trie=%d)",
			c.method, c.target, muxN, trieN)
	}
}

// TestTrieDifferential_MiddlewareParity asserts the middleware chain runs the
// same number of times under both routers for matched, 404, and 405 requests.
// mux applies its chain only on a successful match (never around NotFound /
// MethodNotAllowed), so the trie must not run middleware on unmatched requests
// either — otherwise 404s would be logged/measured and CORS-answered, and the
// metrics path label would be populated from a raw unbounded path.
func TestTrieDifferential_MiddlewareParity(t *testing.T) {
	build := func(useTrie bool, counter *int) *Router {
		rt := NewRouter()
		rt.useTrie = useTrie
		rt.Add(http.MethodGet, "/hit", echoHandler("hit"))
		rt.Add(http.MethodPost, "/only-post", echoHandler("op"))
		rt.UseMiddleware(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				*counter++

				next.ServeHTTP(w, r)
			})
		})

		return rt
	}

	cases := []struct {
		name           string
		method, target string
	}{
		{"matched", http.MethodGet, "/hit"},
		{"not_found", http.MethodGet, "/missing"},
		{"method_not_allowed", http.MethodGet, "/only-post"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var muxN, trieN int

			serve(build(false, &muxN), c.method, c.target)
			serve(build(true, &trieN), c.method, c.target)

			assert.Equalf(t, muxN, trieN,
				"middleware invocation count differs for %s %s (mux=%d trie=%d)",
				c.method, c.target, muxN, trieN)
		})
	}
}

func TestTrieDifferential_MethodsAndFallback(t *testing.T) {
	// A header-constrained route (fallback) plus a plain route on the same path
	// exercises registration-order fidelity across the trie/fallback boundary.
	r := func(useTrie bool) *Router {
		rt := NewRouter()
		rt.useTrie = useTrie
		rt.Router.NewRoute().Methods(http.MethodGet).Path("/gated").
			Headers("X-Key", "secret").Handler(echoHandler("gated-hdr"))
		rt.Add(http.MethodGet, "/gated", echoHandler("gated-plain"))

		return rt
	}

	muxR, trieR := r(false), r(true)

	do := func(router *Router, hdr string) (int, string) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/gated", http.NoBody)
		if hdr != "" {
			req.Header.Set("X-Key", hdr)
		}

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		return rec.Code, rec.Body.String()
	}

	for _, hdr := range []string{"secret", "wrong", ""} {
		ms, mb := do(muxR, hdr)
		ts, tb := do(trieR, hdr)
		require.Equalf(t, ms, ts, "status differs for header %q", hdr)
		assert.Equalf(t, mb, tb, "body differs for header %q", hdr)
	}
}

// benchmarkRouterMatch measures per-request route matching as the number of
// registered routes grows, for one router mode. mux scans routes linearly
// (O(n)); the trie narrows to O(path length), so its cost should stay roughly
// flat as n rises.
func benchmarkRouterMatch(b *testing.B, useTrie bool) {
	b.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, n := range []int{1, 10, 20, 50, 100, 200} {
		b.Run(fmt.Sprintf("routes=%d", n), func(b *testing.B) {
			r := NewRouter()
			r.useTrie = useTrie

			for i := 0; i < n; i++ {
				r.Add(http.MethodGet, fmt.Sprintf("/resource-%d/{id}", i), handler)
			}

			// Every GoFr app registers a PathPrefix("/") catch-all, which lands
			// in the fallback list and is therefore scanned on every request.
			// Include it so the numbers reflect a real route table.
			r.Router.NewRoute().PathPrefix("/").Handler(handler)

			// Match the MIDDLE route. mux's scan cost is linear in registration
			// position, so hitting the last route measures its worst case rather
			// than what real traffic sees; the middle route is the average case.
			// The trie is unaffected by position either way.
			target := fmt.Sprintf("/resource-%d/42", n/2)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, http.NoBody)
			w := httptest.NewRecorder()

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				r.ServeHTTP(w, req)
			}
		})
	}
}

// BenchmarkRouterMatchMux is the O(n) baseline (default router).
func BenchmarkRouterMatchMux(b *testing.B) { benchmarkRouterMatch(b, false) }

// BenchmarkRouterMatchTrie is the trie matcher (GOFR_ROUTER=trie); its cost
// should stay flat as the route count grows.
func BenchmarkRouterMatchTrie(b *testing.B) { benchmarkRouterMatch(b, true) }

// This file pins the router edge cases found by adversarial review. Each test
// names the shape that previously diverged from mux, so a regression is
// self-describing.

// TestTrieDifferential_EmptyMatchingParams covers params whose regex can match
// the empty string ("{s:[a-z]*}"). Such a template segment can vanish, so the
// template's segment count no longer equals the request's and a segment-by-
// segment walk cannot locate it — the route must go to the fallback list.
//
// This previously produced a SILENT WRONG ROUTE in the real GoFr shape: with the
// PathPrefix("/") catch-all also registered, a request for "/" returned 200 from
// both routers but ran the catch-all under the trie instead of the user handler.
func TestTrieDifferential_EmptyMatchingParams(t *testing.T) {
	routes := []routeDef{
		{http.MethodGet, "/{s:[a-z]*}"},   // may match ""
		{http.MethodGet, "/n/{n:[0-9]*}"}, // may match ""
		{http.MethodGet, "/o/{a:(?:x)?}"}, // optional group
		{http.MethodGet, "/r/{l:[a-z]{0,2}}"},
		{http.MethodGet, "/ok/{id}"}, // control: cannot match ""
	}

	cases := []reqCase{
		{http.MethodGet, "/"},      // the empty-match case
		{http.MethodGet, "/abc"},   // non-empty
		{http.MethodGet, "/n/"},    // trailing empty after normalization
		{http.MethodGet, "/n/123"}, //
		{http.MethodGet, "/o/"},    //
		{http.MethodGet, "/o/x"},   //
		{http.MethodGet, "/r/ab"},  //
		{http.MethodGet, "/ok/7"},  // control
	}

	runDifferential(t, "empty_param", routes, cases)
}

// TestTrieDifferential_EmptyParamWithCatchAll reproduces the silent-wrong-route
// shape directly: the user route and GoFr's default PathPrefix("/") catch-all
// both match "/", so the status is 200 either way and only the BODY reveals
// which handler ran.
func TestTrieDifferential_EmptyParamWithCatchAll(t *testing.T) {
	build := func(useTrie bool) *Router {
		r := NewRouter()
		r.useTrie = useTrie
		r.Add(http.MethodGet, "/{s:[a-z]*}", echoHandler("user-handler"))
		r.Router.NewRoute().PathPrefix("/").Handler(echoHandler("catch-all"))

		return r
	}

	_, muxBody := serve(build(false), http.MethodGet, "/")
	_, trieBody := serve(build(true), http.MethodGet, "/")

	assert.Equal(t, muxBody, trieBody, "the same handler must run under both routers")
}

// TestTrieRouter_PrefixEndingInLiteralDollar covers a PathPrefix whose template
// ends in a literal "$". mux QuoteMeta-escapes it, so the UNANCHORED prefix
// regexp still ends in the byte "$" — a naive HasSuffix(rx, "$") check would
// wrongly treat it as an exact path and index it.
func TestTrieRouter_PrefixEndingInLiteralDollar(t *testing.T) {
	build := func(useTrie bool) *Router {
		r := NewRouter()
		r.useTrie = useTrie
		r.Router.NewRoute().PathPrefix("/p$").Handler(echoHandler("prefix"))

		return r
	}

	muxStatus, muxBody := serve(build(false), http.MethodGet, "/p$/q")
	trieStatus, trieBody := serve(build(true), http.MethodGet, "/p$/q")

	require.Equal(t, muxStatus, trieStatus, "prefix route ending in a literal $ must still match")
	assert.Equal(t, muxBody, trieBody)
}

// TestTrieRouter_SubrouterMiddlewareRuns guards the security-relevant case: a
// subrouter's own middleware must run under the trie router too. It previously
// did not when the parent prefix was misclassified as an exact path, so auth or
// CORS registered via Subrouter().Use() was silently skipped.
func TestTrieRouter_SubrouterMiddlewareRuns(t *testing.T) {
	build := func(useTrie bool) *Router {
		r := NewRouter()
		r.useTrie = useTrie

		sub := r.Router.PathPrefix("/api$").Subrouter()
		sub.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("X-Sub", "yes")
				next.ServeHTTP(w, req)
			})
		})
		sub.Path("/users").Handler(echoHandler("sub-users"))

		return r
	}

	do := func(router *Router) (int, string) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api$/users", http.NoBody)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		return rec.Code, rec.Header().Get("X-Sub")
	}

	muxCode, muxHdr := do(build(false))
	trieCode, trieHdr := do(build(true))

	require.Equal(t, muxCode, trieCode)
	assert.Equal(t, muxHdr, trieHdr, "subrouter middleware must run under both routers")
}

// TestTrieRouter_SubrouterNotFoundHandler covers a subrouter carrying its own
// NotFoundHandler. mux reports a SUCCESSFUL match with MatchErr == ErrNotFound
// and the subrouter's handler in the match; requiring MatchErr == nil would skip
// it and fall through to an unrelated route.
func TestTrieRouter_SubrouterNotFoundHandler(t *testing.T) {
	build := func(useTrie bool, mwRuns *int) *Router {
		r := NewRouter()
		r.useTrie = useTrie

		sub := r.Router.PathPrefix("/api").Subrouter()
		sub.NotFoundHandler = echoHandler("SUB404")
		sub.Path("/users").Handler(echoHandler("sub-users"))

		r.Add(http.MethodGet, "/api/other", echoHandler("top-other"))

		r.UseMiddleware(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				*mwRuns++

				next.ServeHTTP(w, req)
			})
		})

		return r
	}

	// The middleware count is asserted, not just the status and body. This case is why: a subrouter
	// NotFoundHandler answers 200 through mux and through the trie alike, so status and body agree
	// while the chain does not. mux builds its chain only when MatchErr == nil, and this match carries
	// ErrNotFound, so mux runs NO middleware here. The trie ran it, which meant such a request was
	// logged, traced, metered and CORS-answered when mux would not have -- and with no path template
	// on a PathPrefix route, the metrics label fell back to the raw request path.
	for _, target := range []string{"/api/other", "/api/zzz", "/api/users"} {
		var muxN, trieN int

		muxStatus, muxBody := serve(build(false, &muxN), http.MethodGet, target)
		trieStatus, trieBody := serve(build(true, &trieN), http.MethodGet, target)

		require.Equalf(t, muxStatus, trieStatus, "status differs for %s", target)
		assert.Equalf(t, muxBody, trieBody, "handler differs for %s", target)
		assert.Equalf(t, muxN, trieN, "middleware invocation count differs for %s (mux=%d trie=%d)",
			target, muxN, trieN)
	}
}

// TestTrieRouter_UseEncodedPath covers a router in UseEncodedPath mode, where
// mux matches the ESCAPED path. The decoded and escaped forms can split into
// different segment counts, so the trie walks both and unions the candidates.
func TestTrieRouter_UseEncodedPath(t *testing.T) {
	build := func(useTrie bool) *Router {
		r := NewRouter()
		r.useTrie = useTrie
		r.Router.UseEncodedPath()
		r.Router.NewRoute().Methods(http.MethodGet).Path("/a/{v}").Handler(echoHandler("one"))
		r.Router.NewRoute().Methods(http.MethodGet).Path("/a/{x}/{y}").Handler(echoHandler("two"))

		return r
	}

	muxR, trieR := build(false), build(true)

	for _, target := range []string{"/a/x%2Fy", "/a/plain", "/a/p/q"} {
		muxStatus, muxBody := serve(muxR, http.MethodGet, target)
		trieStatus, trieBody := serve(trieR, http.MethodGet, target)

		require.Equalf(t, muxStatus, trieStatus, "status differs for %s", target)
		assert.Equalf(t, muxBody, trieBody, "handler/vars differ for %s", target)
	}
}

// TestTrieRouter_IndexInvariant is the structural gate behind every other test:
// for every request, no route that mux would match may be absent from the
// candidate set the trie produces. Over-producing candidates is safe (mux
// filters them); dropping one is the failure mode that causes silent 404s and
// wrong handlers. This asserts the invariant directly rather than relying on a
// particular request happening to expose a gap.
func TestTrieRouter_IndexInvariant(t *testing.T) {
	templates := []string{
		"/", "/users", "/users/{id}", "/users/{id}/posts", "/{a}/{b}",
		"/files/{name}.txt", "/user-{id}", "/{a}.{b}", "/img/{id:[0-9]+}.png",
		"/files/{path:.*}", "/proxy/{rest:.+}", "/opt/{s:[a-z]*}", "/n/{n:[0-9]*}",
		"/static/x", "/deep/a/b/c/d",
	}

	r := NewRouter()
	r.useTrie = true

	for i, tpl := range templates {
		r.Add(http.MethodGet, tpl, echoHandler(fmt.Sprintf("h%d", i)))
	}

	r.Router.NewRoute().PathPrefix("/").Handler(echoHandler("catch-all"))

	idx := newRouteIndex()
	idx.build(&r.Router)

	paths := []string{
		"/", "/users", "/users/42", "/users/42/posts", "/a/b", "/files/x.txt",
		"/user-9", "/x.y", "/img/12.png", "/files/a/b/c", "/proxy/z", "/opt/",
		"/opt/abc", "/n/", "/n/5", "/static/x", "/deep/a/b/c/d", "/unknown/path",
	}

	for _, p := range paths {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, p, http.NoBody)
		normalizePath(req)

		// Every route mux itself would match must be among the candidates.
		cands := make([]*routeEntry, 0, len(idx.fallback))

		idx.root.collect(pathTrimForTest(req.URL.Path), &cands)
		cands = append(cands, idx.fallback...)

		inCandidates := make(map[*mux.Route]bool, len(cands))
		for _, c := range cands {
			inCandidates[c.route] = true
		}

		_ = r.Router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
			var m mux.RouteMatch
			if route.Match(req, &m) {
				assert.Truef(t, inCandidates[route],
					"path %q: mux matches a route the trie never offered as a candidate", p)
			}

			return nil
		})
	}
}

func pathTrimForTest(p string) string {
	for p != "" && p[0] == '/' {
		p = p[1:]
	}

	for p != "" && p[len(p)-1] == '/' {
		p = p[:len(p)-1]
	}

	return p
}
