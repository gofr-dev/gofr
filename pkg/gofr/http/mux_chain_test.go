package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/internal/stablemw"
)

// tracingMW returns a middleware that appends name to the X-Order header and counts its
// constructor calls. stable says whether its handler is a stablemw.Handler.
func tracingMW(name string, stable bool, built *atomic.Int64) mux.MiddlewareFunc {
	return func(inner http.Handler) http.Handler {
		built.Add(1)

		f := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Order", name)
			inner.ServeHTTP(w, r)
		}

		if stable {
			return stablemw.Handler(f)
		}

		return http.HandlerFunc(f)
	}
}

func newMuxModeRouter(t *testing.T) *Router {
	t.Helper()
	t.Setenv(RouterEnvVar, MatcherMux)

	r := NewRouter()
	r.Add(http.MethodGet, "/users/{id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The route is only in the context when mux served the request, not for a handler taken
		// from Match and called directly.
		if route := mux.CurrentRoute(r); route != nil {
			tpl, _ := route.GetPathTemplate()
			w.Header().Set("X-Route", mux.Vars(r)["id"]+" "+tpl)
		}
	}))

	return r
}

func serveMux(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody))

	return w
}

// Framework layers are built once per route; a user middleware between them is still built on
// every request, and the order is the registration order throughout.
func TestMuxChain_StableBuiltOnceUserPerRequest(t *testing.T) {
	r := newMuxModeRouter(t)

	var outer, user, inner atomic.Int64

	r.Use(tracingMW("outer", true, &outer), tracingMW("user", false, &user))
	r.Use(tracingMW("inner", true, &inner))

	for range 5 {
		w := serveMux(t, r, "/users/42")
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, []string{"outer", "user", "inner"}, w.Header().Values("X-Order"))
		assert.Equal(t, "42 /users/{id}", w.Header().Get("X-Route"), "mux.Vars and mux.CurrentRoute still work")
	}

	assert.Equal(t, int64(5), user.Load(), "a user constructor runs on every request, as under mux")
	assert.LessOrEqual(t, inner.Load(), int64(1), "the inner framework layer is built once")
	assert.LessOrEqual(t, outer.Load(), int64(2), "the outer framework layer is built once, and once more over the per-request layer")
}

func TestMuxChain_AllStableIsBuiltOnce(t *testing.T) {
	r := newMuxModeRouter(t)

	var a, b atomic.Int64

	r.Use(tracingMW("a", true, &a), tracingMW("b", true, &b))

	for range 5 {
		assert.Equal(t, []string{"a", "b"}, serveMux(t, r, "/users/1").Header().Values("X-Order"))
	}

	assert.Equal(t, int64(1), a.Load())
	assert.Equal(t, int64(1), b.Load())
}

// A middleware registered after the route has served still runs from the next request on, as it
// did when mux held the list.
func TestMuxChain_LateUseTakesEffect(t *testing.T) {
	r := newMuxModeRouter(t)

	var a, b atomic.Int64

	r.Use(tracingMW("a", true, &a))
	assert.Equal(t, []string{"a"}, serveMux(t, r, "/users/1").Header().Values("X-Order"))

	r.Use(tracingMW("b", true, &b))
	assert.Equal(t, []string{"a", "b"}, serveMux(t, r, "/users/1").Header().Values("X-Order"))
}

// A route created on the embedded mux.Router is not owned: its chain is composed per request.
func TestMuxChain_UnownedRouteComposedPerRequest(t *testing.T) {
	r := newMuxModeRouter(t)
	r.Router.NewRoute().Path("/raw").Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	var a atomic.Int64

	r.Use(tracingMW("a", true, &a))

	for range 3 {
		assert.Equal(t, []string{"a"}, serveMux(t, r, "/raw").Header().Values("X-Order"))
	}

	assert.Equal(t, int64(3), a.Load())
}

// The promoted Match still returns a handler wrapped in the registered middleware, and 404s are
// still served without it.
func TestMuxChain_MatchAndNotFound(t *testing.T) {
	r := newMuxModeRouter(t)

	var a atomic.Int64

	r.Use(tracingMW("a", false, &a))

	var match mux.RouteMatch

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/users/7", http.NoBody)
	require.True(t, r.Match(req, &match))

	w := httptest.NewRecorder()
	match.Handler.ServeHTTP(w, req)
	assert.Equal(t, []string{"a"}, w.Header().Values("X-Order"))

	w = serveMux(t, r, "/nope")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Empty(t, w.Header().Values("X-Order"))
}

// Add does not wrap a nil handler, so mux still treats the route as having none.
func TestMuxChain_NilHandlerStillNotFound(t *testing.T) {
	t.Setenv(RouterEnvVar, MatcherMux)

	r := NewRouter()
	r.Add(http.MethodGet, "/nil", nil)

	assert.Equal(t, http.StatusNotFound, serveMux(t, r, "/nil").Code)
}

func TestMuxChain_ConcurrentFirstRequests(t *testing.T) {
	r := newMuxModeRouter(t)

	var a, u atomic.Int64

	r.Use(tracingMW("a", true, &a), tracingMW("u", false, &u))

	var wg sync.WaitGroup

	for range 8 {
		wg.Go(func() {
			for range 50 {
				w := serveMux(t, r, "/users/3")
				assert.Equal(t, "a,u", strings.Join(w.Header().Values("X-Order"), ","))
			}
		})
	}

	wg.Wait()

	assert.Equal(t, int64(400), u.Load())
}
