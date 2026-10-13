package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRouteTemplate_ResolvesUnderBothRouters is the guard for the accessor the
// tracer and metrics middleware depend on for their route label. It must return
// the route template — not the raw request path — under BOTH routers: the trie
// router records it in the request context (it bypasses mux's ServeHTTP, so
// mux.CurrentRoute is nil there), while the default mux router exposes it via
// mux.CurrentRoute. A silent regression here would degrade the metric label to
// an unbounded raw path without failing any other test.
func TestRouteTemplate_ResolvesUnderBothRouters(t *testing.T) {
	const tmpl = "/users/{id}"

	for _, tc := range []struct {
		name    string
		useTrie bool
	}{
		{"mux", false},
		{"trie", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got string

			r := NewRouter()
			r.useTrie = tc.useTrie
			r.Add(http.MethodGet, tmpl, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				got = RouteTemplate(req)

				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/users/42", http.NoBody)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code, "handler must have run")
			assert.Equal(t, tmpl, got, "route template must resolve under the %s router", tc.name)
		})
	}
}

// TestRouteTemplate_EmptyWhenUnmatched asserts the accessor reports no template
// for a request that matched no route, so callers fall back to the raw path
// rather than reading a stale or bogus label.
func TestRouteTemplate_EmptyWhenUnmatched(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/nothing", http.NoBody)

	assert.Empty(t, RouteTemplate(req), "no template should be reported for an unrouted request")
}

// TestWithRoute_NilIsNoOp verifies that recording no route does not copy the
// request — unmatched requests must not pay for a context allocation.
func TestWithRoute_NilIsNoOp(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", http.NoBody)

	assert.Same(t, req, withRoute(req, nil), "no route must not copy the request")
	assert.NotSame(t, req, withRoute(req, mux.NewRouter().Path("/x/{id}")), "a real route must be recorded")
}

// TestIsPrefixRoute tells GoFr's PathPrefix("/") catch-all apart from an
// application route registered at "/": both have the template "/".
func TestIsPrefixRoute(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	testCases := []struct {
		desc    string
		useTrie bool
		path    string
		want    bool
	}{
		{"mux: route at /", false, "/", false},
		{"trie: route at /", true, "/", false},
		{"mux: catch-all", false, "/nowhere", true},
		{"trie: catch-all", true, "/nowhere", true},
		{"mux: route with a parameter", false, "/users/42", false},
		{"trie: route with a parameter", true, "/users/42", false},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			var got bool

			r := NewRouter()
			r.useTrie = tc.useTrie
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					got = IsPrefixRoute(req)

					next.ServeHTTP(w, req)
				})
			})
			r.Add(http.MethodGet, "/", ok)
			r.Add(http.MethodGet, "/users/{id}", ok)
			r.PathPrefix("/").Handler(ok)

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tc.path, http.NoBody)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.want, got, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}

	t.Run("unrouted request", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)

		assert.False(t, IsPrefixRoute(req))
	})
}
