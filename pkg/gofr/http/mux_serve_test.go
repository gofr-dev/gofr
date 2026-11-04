package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// observe is a handler that reports what it sees of mux's request state.
func observe(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		route := mux.CurrentRoute(r)

		tpl := ""
		if route != nil {
			tpl, _ = route.GetPathTemplate()
		}

		fmt.Fprintf(w, "%s vars=%v nil=%t route=%q", name, vars, vars == nil, tpl) //nolint:gosec // a test handler echoing what it observed
	})
}

func newServeMuxRouter(t *testing.T, notFound bool) *Router {
	t.Helper()
	t.Setenv(RouterEnvVar, MatcherMux)

	r := NewRouter()
	r.Add(http.MethodGet, "/users/{id}", observe("user"))
	r.Add(http.MethodGet, "/plain", observe("plain"))
	r.Router.NewRoute().Path("/nil")
	r.Router.NewRoute().Path("/raw/{name}").Handler(observe("raw"))

	if notFound {
		r.Router.NotFoundHandler = observe("notfound")
	}

	r.Router.Use(func(next http.Handler) http.Handler {
		// mux wraps a route's nil handler too; passing it through keeps that route a 404.
		if next == nil {
			return nil
		}

		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Embedded", "1")
			next.ServeHTTP(w, req)
		})
	})

	return r
}

// serveMux answers every request exactly as mux.Router.ServeHTTP does: the same status, headers
// and body, and the same mux.Vars and mux.CurrentRoute inside the handler.
func TestServeMux_SameAsMux(t *testing.T) {
	for _, notFound := range []bool{false, true} {
		r := newServeMuxRouter(t, notFound)

		for _, tc := range []struct{ method, target string }{
			{http.MethodGet, "/users/42"},
			{http.MethodGet, "/plain"},
			{http.MethodPost, "/plain"},
			{http.MethodGet, "/nope"},
			{http.MethodGet, "/nil"},
			{http.MethodGet, "/raw/x%2Fy"},
			{http.MethodDelete, "/users/7"},
		} {
			t.Run(fmt.Sprintf("notFound=%t %s %s", notFound, tc.method, tc.target), func(t *testing.T) {
				want := httptest.NewRecorder()
				r.Router.ServeHTTP(want, httptest.NewRequestWithContext(t.Context(), tc.method, tc.target, http.NoBody))

				got := httptest.NewRecorder()
				r.ServeHTTP(got, httptest.NewRequestWithContext(t.Context(), tc.method, tc.target, http.NoBody))

				assert.Equal(t, want.Code, got.Code)
				assert.Equal(t, want.Header(), got.Header())
				assert.Equal(t, want.Body.String(), got.Body.String())
			})
		}
	}
}

// Under every mux setting that decides whether a path is served or redirected, and with a
// subrouter and a MethodNotAllowedHandler, serveMux answers as mux.Router.ServeHTTP does,
// including the paths it hands to mux.
func TestServeMux_SameAsMuxUnderSettings(t *testing.T) {
	for _, setting := range []string{"default", "clean", "encoded", "encoded+clean"} {
		r := newServeMuxRouter(t, false)

		switch setting {
		case "clean":
			r.Router.SkipClean(false)
		case "encoded":
			r.Router.UseEncodedPath()
		case "encoded+clean":
			r.Router.UseEncodedPath()
			r.Router.SkipClean(false)
		}

		r.Router.MethodNotAllowedHandler = observe("405")

		sub := r.Router.PathPrefix("/sub").Subrouter()
		sub.Handle("/b/{y}", observe("sub")).Methods(http.MethodPost)
		sub.NotFoundHandler = observe("subnotfound")

		for _, tc := range []struct{ method, target string }{
			{http.MethodGet, "/users/1"}, {http.MethodPost, "/users/1"},
			{http.MethodPost, "/sub/b/2"}, {http.MethodGet, "/sub/b/2"}, {http.MethodGet, "/sub/zz"},
			{http.MethodGet, "/raw/x%2Fy"}, {http.MethodGet, "/raw/x%2F..%2Fy"}, {http.MethodGet, "/raw/%2e%2e"},
			{http.MethodGet, "/zz"},
		} {
			t.Run(fmt.Sprintf("%s %s %s", setting, tc.method, tc.target), func(t *testing.T) {
				// Router.ServeHTTP normalizes the path before matching; give mux the same request.
				req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.target, http.NoBody)
				normalizePath(req)

				want := httptest.NewRecorder()
				r.Router.ServeHTTP(want, req)

				got := httptest.NewRecorder()
				r.ServeHTTP(got, httptest.NewRequestWithContext(t.Context(), tc.method, tc.target, http.NoBody))

				assert.Equal(t, want.Code, got.Code)
				assert.Equal(t, want.Header(), got.Header())
				assert.Equal(t, want.Body.String(), got.Body.String())
			})
		}
	}
}

// A path mux would clean and redirect never reaches serveMux through ServeHTTP, which
// normalizes it first. serveMux called on one hands it to mux, redirect included.
func TestServeMux_UncleanPathGoesToMux(t *testing.T) {
	for _, skipClean := range []bool{true, false} {
		r := newServeMuxRouter(t, false)
		r.Router.SkipClean(skipClean)

		for _, target := range []string{"/users//1", "/users/./1", "/plain/", "/a/../plain", "/raw/x%2Fy/../z"} {
			t.Run(fmt.Sprintf("skipClean=%t %s", skipClean, target), func(t *testing.T) {
				want := httptest.NewRecorder()
				r.Router.ServeHTTP(want, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody))

				got := httptest.NewRecorder()
				r.serveMux(got, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody))

				assert.Equal(t, want.Code, got.Code)
				assert.Equal(t, want.Header(), got.Header())
				assert.Equal(t, want.Body.String(), got.Body.String())
			})
		}
	}
}

// The vars map is the one mux built for the request: the same map on every call, so a write to
// it is seen by a later read, as with mux.
func TestServeMux_VarsMapIsStable(t *testing.T) {
	r := newServeMuxRouter(t, false)

	r.Add(http.MethodGet, "/w/{id}", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mux.Vars(req)["extra"] = "1"
		fmt.Fprint(w, mux.Vars(req)["id"], mux.Vars(req)["extra"]) //nolint:gosec // a test handler echoing what it observed
	}))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/w/9", http.NoBody))

	assert.Equal(t, "91", w.Body.String())
}

type serveMuxKey struct{}

// The context serveMux installs passes every other value, deadline and cancellation through to
// the request's own context, and a context derived from it is canceled with its parent.
func TestServeMux_ContextPassesThrough(t *testing.T) {
	r := newServeMuxRouter(t, false)

	parent, cancel := context.WithCancel(context.WithValue(t.Context(), serveMuxKey{}, "v"))

	r.Add(http.MethodGet, "/ctx", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		child, stop := context.WithCancel(ctx)

		defer stop()

		cancel()
		<-child.Done()

		//nolint:gosec // a test handler echoing what it observed
		fmt.Fprint(w, ctx.Value(serveMuxKey{}), " ", ctx.Err(), " ", context.Cause(child))
	}))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequestWithContext(parent, http.MethodGet, "/ctx", http.NoBody))

	assert.Equal(t, "v context canceled context canceled", w.Body.String())
}

func TestServeMux_KeysProbed(t *testing.T) {
	require.NotNil(t, muxVarsKey, "mux.Vars key found")
	require.NotNil(t, muxRouteKey, "mux.CurrentRoute key found")
	assert.NotEqual(t, muxVarsKey, muxRouteKey)
}

func TestMuxClean(t *testing.T) {
	for p, want := range map[string]bool{
		"/": true, "/a/b": true, "/a/b/": true,
		"": false, "a": false, "//a": false, "/a/./b": false, "/a/../b": false, "/a//": false,
	} {
		assert.Equal(t, want, muxClean(p), p)
	}
}

// serveMux saves three allocations per matched request against mux.Router.ServeHTTP: the
// RouteMatch, one of the two context values and one of the two request copies. A route mux has
// to decide (here, a regexp variable) still gets that much.
func TestServeMux_Allocations(t *testing.T) {
	if raceEnabled {
		t.Skip("allocation counts are not stable under the race detector")
	}

	t.Setenv(RouterEnvVar, MatcherMux)

	r := NewRouter()
	r.Add(http.MethodGet, "/users/{id:[0-9]+}", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/users/1", http.NoBody)
	w := httptest.NewRecorder()

	viaMux := testing.AllocsPerRun(100, func() { r.Router.ServeHTTP(w, req) })
	viaServeMux := testing.AllocsPerRun(100, func() { r.ServeHTTP(w, req) })

	assert.InDelta(t, viaMux-3, viaServeMux, 0.01, "mux %.0f, serveMux %.0f", viaMux, viaServeMux)
}

// A route fastMatch decides costs the muxState and one request copy, and nothing for its vars
// until they are read: no submatch indexes and no map.
func TestServeMux_FastMatchAllocations(t *testing.T) {
	if raceEnabled {
		t.Skip("allocation counts are not stable under the race detector")
	}

	t.Setenv(RouterEnvVar, MatcherMux)

	r := NewRouter()
	r.Add(http.MethodGet, "/users/{id}", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/users/1", http.NoBody)
	w := httptest.NewRecorder()

	viaMux := testing.AllocsPerRun(100, func() { r.Router.ServeHTTP(w, req) })
	viaServeMux := testing.AllocsPerRun(100, func() { r.ServeHTTP(w, req) })

	assert.InDelta(t, 2, viaServeMux, 0.01, "mux %.0f, serveMux %.0f", viaMux, viaServeMux)
}

// Reading a path parameter of a route fastMatch decided builds no vars map.
func TestServeMux_FastPathParamAllocations(t *testing.T) {
	if raceEnabled {
		t.Skip("allocation counts are not stable under the race detector")
	}

	t.Setenv(RouterEnvVar, MatcherMux)

	var id string

	r := NewRouter()
	r.Add(http.MethodGet, "/users/{id}", http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		pr := Request{req: req}
		id = pr.PathParam("id")
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/users/1", http.NoBody)
	w := httptest.NewRecorder()

	assert.InDelta(t, 2, testing.AllocsPerRun(100, func() { r.ServeHTTP(w, req) }), 0.01)
	assert.Equal(t, "1", id)
}
