package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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

// TestWithRouteTemplate_EmptyIsNoOp verifies that recording an empty template
// does not copy the request — unmatched requests must not pay for a context
// allocation.
func TestWithRouteTemplate_EmptyIsNoOp(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", http.NoBody)

	assert.Same(t, req, withRouteTemplate(req, ""), "empty template must not copy the request")
	assert.NotSame(t, req, withRouteTemplate(req, "/x/{id}"), "a real template must be recorded")
}
