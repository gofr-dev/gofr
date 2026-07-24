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

// TestTrieDifferential_MixedLiteralParamSegments covers segments that mix a
// literal with a parameter ("/{name}.txt", "/user-{id}", "/v{ver}/x"). The trie
// keys whole segments, so these can't be indexed and must fall through to mux
// via the fallback list — the router must not drop them.
func TestTrieDifferential_MixedLiteralParamSegments(t *testing.T) {
	routes := []routeDef{
		{http.MethodGet, "/files/{name}.txt"}, // param with a literal suffix
		{http.MethodGet, "/user-{id}"},        // literal prefix + param
		{http.MethodGet, "/v{ver}/x"},         // literal+param, then a static segment
		{http.MethodGet, "/{lang}-{region}"},  // two params + a literal separator
		{http.MethodGet, "/pure/{id}"},        // control: pure param stays trie-indexed
	}

	cases := []reqCase{
		{http.MethodGet, "/files/report.txt"}, // vars{name:report}
		{http.MethodGet, "/files/report.csv"}, // wrong suffix -> 404 in both
		{http.MethodGet, "/user-42"},          // vars{id:42}
		{http.MethodGet, "/v2/x"},             // vars{ver:2}
		{http.MethodGet, "/en-US"},            // vars{lang:en,region:US}
		{http.MethodGet, "/pure/9"},
	}

	runDifferential(t, "mixed_seg", routes, cases)
}

// TestTrieDifferential_KnownMethodMismatchDivergence pins the ONE intentional,
// documented behavior difference between the two routers (see trie_router.go and
// the plan's characterization notes).
//
// When a request's method is wrong for a path that DOES exist, mux's status is
// registration-order dependent: a later route whose path does not match resets
// mux's internal ErrMethodMismatch, so mux can report 404 instead of 405. The
// trie decides method-mismatch over the narrowed candidate set and therefore
// reports the more correct 405. This affects only the status code (404 vs 405)
// of wrong-method requests to existing paths; it never changes which handler a
// valid request reaches, nor its params or body. It is inert by default because
// the trie router is opt-in (GOFR_ROUTER=trie).
func TestTrieDifferential_KnownMethodMismatchDivergence(t *testing.T) {
	routes := []routeDef{
		{http.MethodPost, "/a/{id}"}, // only POST on this path
		{http.MethodGet, "/a/b"},     // a later, non-overlapping path
	}

	muxR := buildRouter(routes, false)
	trieR := buildRouter(routes, true)

	// GET /a/xyz: path matches /a/{id} (wrong method). mux resets its
	// method-mismatch via the later /a/b route -> 404; trie -> 405.
	muxStatus, _ := serve(muxR, http.MethodGet, "/a/xyz")
	trieStatus, _ := serve(trieR, http.MethodGet, "/a/xyz")

	assert.Equal(t, http.StatusNotFound, muxStatus, "mux baseline: registration-order 404")
	assert.Equal(t, http.StatusMethodNotAllowed, trieStatus, "trie: more-correct 405 for existing path, wrong method")
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

	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("routes=%d", n), func(b *testing.B) {
			r := NewRouter()
			r.useTrie = useTrie

			for i := 0; i < n; i++ {
				r.Add(http.MethodGet, fmt.Sprintf("/resource-%d/{id}", i), handler)
			}

			// Match the LAST-registered route: worst case for a linear scan,
			// unaffected by position for a trie.
			target := fmt.Sprintf("/resource-%d/42", n-1)
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
