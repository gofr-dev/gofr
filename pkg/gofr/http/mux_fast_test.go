package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fastTemplates are the templates the parity tests register, in this order. The decidable ones
// are mixed with shapes fastMatch must leave to mux, and with routes that shadow later ones.
//
//nolint:gochecknoglobals // shared, read-only test table.
var fastTemplates = []struct{ method, tpl string }{
	{http.MethodGet, "/"},
	{http.MethodGet, "/plain"},
	{http.MethodPost, "/plain"},
	{http.MethodGet, "/users/{id}"},
	{http.MethodGet, "/users/me"}, // shadowed by /users/{id}
	{http.MethodPut, "/users/{id}/posts/{post}"},
	{http.MethodGet, "/dup/{a}/{a}"},
	{http.MethodGet, "/file-{name}"},
	{http.MethodGet, "/v{n}/x"},
	{http.MethodGet, "/num/{id:[0-9]+}"},
	{http.MethodGet, "/a.b+c/{x}"},
	{http.MethodGet, "/many/{a}/{b}/{c}/{d}/{e}/{f}"},
	{http.MethodDelete, "/users/{id}"},
	{http.MethodGet, "/trail/"},
	{http.MethodGet, "/ünï/{x}"},
}

//nolint:gochecknoglobals // shared, read-only test table.
var fastTargets = []string{
	"/", "/plain", "/plain/", "/users/42", "/users/me", "/users/", "/users", "/users/42/posts/7",
	"/users/42/posts/", "/dup/1/2", "/file-x", "/file-", "/file-x/y", "/v2/x", "/v/x", "/num/12",
	"/num/ab", "/a.b+c/1", "/aXb+c/1", "/abbc/1", "/many/1/2/3/4/5/6", "/many/1/2/3/4/5", "/trail/",
	"/trail", "/%C3%BCn%C3%AF/1", "/users/a%2Fb", "/users/a%20b", "/users/%0A", "/users/%FF",
	"/users/42?x=1", "/nope", "/users/42/", "/USERS/42",
}

//nolint:gochecknoglobals // shared, read-only test table.
var fastMethods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, "get"}

func fastRouter(t *testing.T, register func(r *Router)) *Router {
	t.Helper()
	t.Setenv(RouterEnvVar, MatcherMux)

	r := NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Chain", "1")
			next.ServeHTTP(w, req)
		})
	})

	for _, rt := range fastTemplates {
		r.Add(rt.method, rt.tpl, observe(rt.method+" "+rt.tpl))
	}

	if register != nil {
		register(r)
	}

	return r
}

// assertSameAsMux serves every target and method through Router.ServeHTTP and through the
// embedded mux router, and requires the same status, headers and body, which carries
// mux.Vars and mux.CurrentRoute as the handler saw them.
func assertSameAsMux(t *testing.T, r *Router) {
	t.Helper()

	for _, target := range fastTargets {
		for _, method := range fastMethods {
			want := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), method, target, http.NoBody)
			normalizePath(req)
			r.Router.ServeHTTP(want, req)

			got := httptest.NewRecorder()
			r.ServeHTTP(got, httptest.NewRequestWithContext(t.Context(), method, target, http.NoBody))

			assert.Equal(t, want.Code, got.Code, "%s %s", method, target)
			assert.Equal(t, want.Header(), got.Header(), "%s %s", method, target)
			assert.Equal(t, want.Body.String(), got.Body.String(), "%s %s", method, target)
		}
	}
}

func TestFastMatch_SameAsMux(t *testing.T) {
	assertSameAsMux(t, fastRouter(t, nil))
}

// Whenever fastMatch decides a request, mux decides it the same way: the same route, the same
// vars, no error. It never decides one mux would not match.
func TestFastMatch_AgreesWithMuxMatch(t *testing.T) {
	r := fastRouter(t, nil)

	decided := 0

	for _, target := range fastTargets {
		for _, method := range fastMethods {
			req := httptest.NewRequestWithContext(t.Context(), method, target, http.NoBody)

			s := &muxState{Context: req.Context()}
			if !r.fastMatch(req, s) {
				continue
			}

			decided++

			var want mux.RouteMatch

			require.True(t, r.Router.Match(req, &want), "%s %s", method, target)
			require.NoError(t, want.MatchErr)
			assert.Same(t, want.Route, s.match.Route, "%s %s", method, target)

			s.buildVars()
			assert.Equal(t, want.Vars, s.match.Vars, "%s %s", method, target)
		}
	}

	assert.GreaterOrEqual(t, decided, 10, "the fast path decides the plain routes")
}

// The shapes fastMatch accepts, and the ones it leaves to mux.
func TestFastMatch_Entries(t *testing.T) {
	r := fastRouter(t, nil)
	l := r.fastList()
	require.NotNil(t, l)
	require.Len(t, l.entries, len(fastTemplates))

	opaque := map[string]bool{"/v{n}/x": false, "/num/{id:[0-9]+}": true, "/ünï/{x}": true}

	for i, rt := range fastTemplates {
		assert.Equal(t, opaque[rt.tpl], l.entries[i] == nil, rt.tpl)
	}
}

// Under any setting or registration fastMatch cannot account for, the router still answers as
// mux does.
func TestFastMatch_SameAsMuxUnderSettings(t *testing.T) {
	for name, register := range map[string]func(r *Router){
		"strict slash after": func(r *Router) {
			r.Router.StrictSlash(true)
			r.Add(http.MethodGet, "/strict/{x}", observe("strict"))
		},
		"encoded path": func(r *Router) { r.Router.UseEncodedPath() },
		"embedded mux middleware": func(r *Router) {
			r.Router.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					w.Header().Set("X-Embedded", "1")
					next.ServeHTTP(w, req)
				})
			})
		},
		"foreign route": func(r *Router) { r.Router.HandleFunc("/plain", observe("foreign").ServeHTTP) },
		"queries":       func(r *Router) { r.Router.Handle("/users/{id}", observe("q")).Queries("x", "1") },
		"host":          func(r *Router) { r.Router.Host("example.com").Handler(observe("host")) },
		"handlers": func(r *Router) {
			r.Router.NotFoundHandler = observe("404")
			r.Router.MethodNotAllowedHandler = observe("405")
		},
		"subrouter": func(r *Router) {
			sub := r.Router.PathPrefix("/users").Subrouter()
			sub.Handle("/{id}", observe("sub")).Methods(http.MethodPatch)
			sub.NotFoundHandler = observe("subnotfound")
		},
		"catch-all": func(r *Router) { r.Router.PathPrefix("/").Handler(observe("catchall")) },
		"build only": func(r *Router) {
			_ = r.Router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
				if tpl, _ := route.GetPathTemplate(); tpl == "/plain" {
					route.BuildOnly()
				}

				return nil
			})
		},
		"late use": func(r *Router) {
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					w.Header().Set("X-Late", "1")
					next.ServeHTTP(w, req)
				})
			})
		},
	} {
		t.Run(name, func(t *testing.T) {
			assertSameAsMux(t, fastRouter(t, register))
		})
	}
}

// A route added after the router has served is matched as mux would match it.
func TestFastMatch_RouteAddedAfterServing(t *testing.T) {
	r := fastRouter(t, nil)
	assertSameAsMux(t, r)

	r.Add(http.MethodGet, "/nope", observe("late"))
	r.Router.PathPrefix("/").Handler(observe("catchall"))

	assertSameAsMux(t, r)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/nope", http.NoBody))
	assert.Contains(t, w.Body.String(), "late")
}

// A route changed in place after serving keeps the route count, so the scan list is not rebuilt
// for it; the route that wins is re-checked instead.
func TestFastMatch_RouteChangedInPlaceAfterServing(t *testing.T) {
	for name, change := range map[string]func(*mux.Route){
		"build only": func(rt *mux.Route) { rt.BuildOnly() },
		"queries":    func(rt *mux.Route) { rt.Queries("x", "1") },
		"headers":    func(rt *mux.Route) { rt.Headers("X-Need", "1") },
		"schemes":    func(rt *mux.Route) { rt.Schemes("https") },
		"methods":    func(rt *mux.Route) { rt.Methods(http.MethodPost) },
		"path":       func(rt *mux.Route) { rt.Path("/other/{id}") },
		"handler":    func(rt *mux.Route) { rt.Handler(observe("replaced")) },
	} {
		t.Run(name, func(t *testing.T) {
			r := fastRouter(t, nil)
			assertSameAsMux(t, r)

			_ = r.Router.Walk(func(rt *mux.Route, _ *mux.Router, _ []*mux.Route) error {
				tpl, _ := rt.GetPathTemplate()
				methods, _ := rt.GetMethods()

				if tpl == "/users/{id}" && len(methods) == 1 && methods[0] == http.MethodGet {
					change(rt)
				}

				return nil
			})

			assertSameAsMux(t, r)
		})
	}
}

// Without any middleware the matched handler is the route's own, as with mux.
func TestFastMatch_NoMiddleware(t *testing.T) {
	t.Setenv(RouterEnvVar, MatcherMux)

	r := NewRouter()
	r.Add(http.MethodGet, "/users/{id}", observe("user"))

	l := r.fastList()
	require.NotNil(t, l)
	require.NotNil(t, l.entries[0])

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/users/3", http.NoBody))
	assert.Equal(t, `user vars=map[id:3] nil=false route="/users/{id}"`, w.Body.String())
}

// The vars map a fast match builds is one map for the request: a write is seen by a later read.
func TestFastMatch_VarsMapIsStable(t *testing.T) {
	r := fastRouter(t, nil)
	r.Add(http.MethodGet, "/w/{id}", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mux.Vars(req)["extra"] = "1"
		fmt.Fprint(w, mux.Vars(req)["id"], mux.Vars(req)["extra"], NewRequest(req).PathParam("id")) //nolint:gosec // test echo
	}))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/w/9", http.NoBody))

	assert.Equal(t, "919", w.Body.String())
}

// Concurrent first reads of the vars map see one map.
func TestFastMatch_ConcurrentVarsRead(t *testing.T) {
	r := fastRouter(t, nil)
	r.Add(http.MethodGet, "/c/{id}", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		done := make(chan map[string]string, 4)

		for range 4 {
			go func() { done <- mux.Vars(req) }()
		}

		first := <-done
		for range 3 {
			m := <-done
			if fmt.Sprintf("%p", m) != fmt.Sprintf("%p", first) {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}

		fmt.Fprint(w, first["id"])
	}))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/c/5", http.NoBody))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "5", w.Body.String())
}

func TestParseFastTemplate(t *testing.T) {
	for tpl, want := range map[string]string{
		"/":              `^/$`,
		"/a/{b}":         `^/a/(?P<v0>[^/]+)$`,
		"/a.{b}/c/{d}":   `^/a\.(?P<v0>[^/]+)/c/(?P<v1>[^/]+)$`,
		"/{a}{b}":        "",
		"/{a}x":          "",
		"/{a:[0-9]+}":    "",
		"/{}":            "",
		"/{a":            "",
		"/a}":            "",
		"/{a{b}}":        "",
		"/a}/{b}":        "",
		"/{a}/b}":        "",
		"/\xff/{a}":      "",
		"/x/{a}/":        `^/x/(?P<v0>[^/]+)/$`,
		"/x/{a}/y/{b}/z": `^/x/(?P<v0>[^/]+)/y/(?P<v1>[^/]+)/z$`,
	} {
		f := parseFastTemplate(tpl)
		if want == "" {
			assert.Nil(t, f, tpl)

			continue
		}

		require.NotNil(t, f, tpl)
		assert.Equal(t, want, f.regexp(), tpl)
	}
}

// FuzzFastMatch checks, for arbitrary paths, that a request fastMatch decides is one mux matches
// to the same route with the same vars.
func FuzzFastMatch(f *testing.F) {
	for _, target := range fastTargets {
		f.Add(http.MethodGet, target)
	}

	f.Fuzz(func(t *testing.T, method, p string) {
		u, err := url.Parse("http://x" + p)
		if err != nil || method == "" {
			return
		}

		r := fastRouter(t, nil)
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
		req.Method, req.URL = method, u

		s := &muxState{Context: req.Context()}
		if !r.fastMatch(req, s) {
			return
		}

		var want mux.RouteMatch

		require.True(t, r.Router.Match(req, &want))
		require.NoError(t, want.MatchErr)
		require.Same(t, want.Route, s.match.Route)

		for _, name := range append([]string{"missing"}, s.fast.names...) {
			v, ok := s.pathParam(name)
			require.True(t, ok)
			require.Equal(t, want.Vars[name], v, name)
		}

		s.buildVars()
		require.Equal(t, want.Vars, s.match.Vars)

		_, ok := s.pathParam("missing")
		require.False(t, ok, "once the map exists it is the answer")
	})
}

// PathParam reads each variable as mux.Vars would, for every route fastMatch decides, before
// and after the vars map is built, and sees a change made through mux.Vars.
func TestFastMatch_PathParamSameAsVars(t *testing.T) {
	for _, target := range fastTargets {
		for _, method := range fastMethods {
			req := httptest.NewRequestWithContext(t.Context(), method, target, http.NoBody)
			normalizePath(req)

			var want mux.RouteMatch
			if !fastRouter(t, nil).Router.Match(req, &want) || want.MatchErr != nil {
				continue
			}

			names := []string{"missing", "id", "post", "a", "name", "n", "x", "f"}
			got := map[string]string{}

			r := NewRouter()
			for _, rt := range fastTemplates {
				r.Add(rt.method, rt.tpl, http.HandlerFunc(func(_ http.ResponseWriter, hr *http.Request) {
					pr := NewRequest(hr)
					for _, n := range names {
						got[n] = pr.PathParam(n)
					}
				}))
			}

			r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), method, target, http.NoBody))

			for _, n := range names {
				assert.Equal(t, want.Vars[n], got[n], "%s %s %s", method, target, n)
			}
		}
	}
}

func TestFastMatch_PathParamSeesVarsChange(t *testing.T) {
	r := fastRouter(t, nil)
	r.Add(http.MethodGet, "/m/{id}", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		pr := NewRequest(req)
		before := pr.PathParam("id")
		mux.Vars(req)["id"] = "changed"

		fmt.Fprint(w, before, " ", pr.PathParam("id")) //nolint:gosec // test echo
	}))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/m/9", http.NoBody))

	assert.Equal(t, "9 changed", w.Body.String())
}
