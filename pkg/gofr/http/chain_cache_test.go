package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gorilla/mux"
)

// TestChainCacheBuildsOncePerRoute pins the optimization itself: the middleware
// CONSTRUCTORS run once per route, not once per request.
//
// This is the whole point of the cache -- each constructor returns a fresh
// closure, so running them per request allocated one object per middleware on
// every request.
func TestChainCacheBuildsOncePerRoute(t *testing.T) {
	t.Setenv(RouterEnvVar, MatcherTrie)

	var built atomic.Int64

	r := NewRouter()
	r.Use(func(inner http.Handler) http.Handler {
		built.Add(1)

		return inner
	})

	r.Add(http.MethodGet, "/a", chainOKHandler())
	r.Add(http.MethodGet, "/b", chainOKHandler())

	for range 20 {
		serveBody(t, r, "/a")
	}

	for range 20 {
		serveBody(t, r, "/b")
	}

	// Two routes served: two chains built, regardless of request count.
	if got := built.Load(); got != 2 {
		t.Errorf("middleware constructor ran %d times for 2 routes over 40 requests; want 2", got)
	}
}

// TestChainCacheRunsMiddlewareEveryRequest is the other half, and the one that
// would catch a cache that memoized too much: the middleware BODY must still run
// on every single request. A chain cached as a no-op would silently disable
// tracing, logging, metrics and CORS for every GoFr service.
func TestChainCacheRunsMiddlewareEveryRequest(t *testing.T) {
	t.Setenv(RouterEnvVar, MatcherTrie)

	var ran atomic.Int64

	r := NewRouter()
	r.Use(func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ran.Add(1)
			inner.ServeHTTP(w, req)
		})
	})
	r.Add(http.MethodGet, "/x", chainOKHandler())

	const requests = 25
	for range requests {
		serveBody(t, r, "/x")
	}

	if got := ran.Load(); got != requests {
		t.Errorf("middleware body ran %d times over %d requests; want %d", got, requests, requests)
	}
}

// TestChainCacheOrderIsPreserved pins that memoising does not reorder the chain.
// GoFr's ordering is load-bearing: Tracer must be outermost so the whole request
// sits inside its span, and Logging must wrap Metrics so both share one status
// writer.
func TestChainCacheOrderIsPreserved(t *testing.T) {
	t.Setenv(RouterEnvVar, MatcherTrie)

	var (
		mu    sync.Mutex
		order []string
	)

	record := func(name string) mux.MiddlewareFunc {
		return func(inner http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				mu.Lock()

				order = append(order, name)
				mu.Unlock()

				inner.ServeHTTP(w, req)
			})
		}
	}

	r := NewRouter()
	r.Use(record("first"), record("second"), record("third"))
	r.Add(http.MethodGet, "/o", chainOKHandler())

	// Twice: the second request is served from the cache, and must order the
	// same as the first.
	for range 2 {
		mu.Lock()
		order = order[:0]
		mu.Unlock()

		serveBody(t, r, "/o")

		mu.Lock()
		got := fmt.Sprint(order)
		mu.Unlock()

		if want := "[first second third]"; got != want {
			t.Errorf("chain order %s, want %s", got, want)
		}
	}
}

// TestChainCacheIsPerRoute pins that two routes get their own chains rather than
// one route being served another's handler -- the failure a cache keyed too
// loosely would produce, and one that would route traffic to the wrong handler.
func TestChainCacheIsPerRoute(t *testing.T) {
	t.Setenv(RouterEnvVar, MatcherTrie)

	r := NewRouter()
	r.Use(func(inner http.Handler) http.Handler { return inner })

	for _, name := range []string{"alpha", "beta", "gamma"} {
		body := name
		r.NewRoute().Methods(http.MethodGet).Path("/" + name).
			Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
	}

	// Twice each, interleaved, so a cache collision shows up as a wrong body.
	for range 3 {
		for _, name := range []string{"alpha", "beta", "gamma"} {
			if got := serveBody(t, r, "/"+name); got != name {
				t.Errorf("GET /%s returned %q", name, got)
			}
		}
	}
}

// TestChainCacheConcurrentFirstRequests pins that concurrent first requests to
// the same route do not corrupt the cache or drop middleware. The cache is
// populated on first use, so this is the window where a race would live.
func TestChainCacheConcurrentFirstRequests(t *testing.T) {
	t.Setenv(RouterEnvVar, MatcherTrie)

	var ran atomic.Int64

	r := NewRouter()
	r.Use(func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ran.Add(1)
			inner.ServeHTTP(w, req)
		})
	})
	r.Add(http.MethodGet, "/race", chainOKHandler())

	const n = 64

	var wg sync.WaitGroup

	for range n {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if got := serveBody(t, r, "/race"); got != "ok" {
				t.Errorf("body %q", got)
			}
		}()
	}

	wg.Wait()

	if got := ran.Load(); got != n {
		t.Errorf("middleware body ran %d times over %d concurrent requests; want %d", got, n, n)
	}
}

// TestMuxPathUnaffectedByChainCache pins that the default matcher still behaves
// exactly as before -- the cache lives only on the trie path.
func TestMuxPathUnaffectedByChainCache(t *testing.T) {
	t.Setenv(RouterEnvVar, MatcherMux)

	var ran atomic.Int64

	r := NewRouter()
	r.Use(func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ran.Add(1)
			inner.ServeHTTP(w, req)
		})
	})
	r.Add(http.MethodGet, "/m", chainOKHandler())

	const requests = 10
	for range requests {
		if got := serveBody(t, r, "/m"); got != "ok" {
			t.Fatalf("body %q", got)
		}
	}

	if got := ran.Load(); got != requests {
		t.Errorf("middleware body ran %d times; want %d", got, requests)
	}
}

func chainOKHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
}

func serveBody(t *testing.T, r *Router, path string) string {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	return w.Body.String()
}

// TestChainCacheSkipsRoutesItDoesNotOwn pins the limit of the cache.
//
// Router embeds mux.Router publicly, so a user can register a subrouter. When
// mux matches through one it builds a fresh wrapper around the subrouter's
// handler for THAT request, while match.Route still points at the inner route --
// so the route alone does not identify the handler. Caching on it would pin the
// first wrapper and every later request would run that one instead of its own.
//
// Only routes registered through Add, whose handler is the one Add installed and
// never changes, are eligible.
func TestChainCacheSkipsRoutesItDoesNotOwn(t *testing.T) {
	t.Setenv(RouterEnvVar, MatcherTrie)

	r := NewRouter()

	// Registered straight on the embedded mux.Router, exactly as a user adding a
	// subrouter would: not through Add, therefore not owned.
	unowned := r.NewRoute().Methods(http.MethodGet).Path("/unowned").Handler(chainOKHandler())

	r.Add(http.MethodGet, "/owned", chainOKHandler())

	serveBody(t, r, "/unowned")
	serveBody(t, r, "/owned")

	if _, cached := r.chains.Load(unowned); cached {
		t.Error("a route GoFr did not register must never enter the chain cache")
	}

	var ownedCached bool

	r.chains.Range(func(_, _ any) bool {
		ownedCached = true

		return false
	})

	if !ownedCached {
		t.Error("a route registered through Add should be cached")
	}
}

// TestSubrouterMiddlewareRunsItsOwnInstance is the black-box form of the same
// guarantee, and the one that reproduces the user-visible defect.
//
// mux builds a fresh wrapper around a subrouter's handler for every request,
// while match.Route keeps pointing at the inner route. Caching on the route alone
// therefore pinned the FIRST request's wrapper and every later request ran that
// one instead of its own -- visible here as a sequence number frozen at 1.
func TestSubrouterMiddlewareRunsItsOwnInstance(t *testing.T) {
	t.Setenv(RouterEnvVar, MatcherTrie)

	r := NewRouter()

	var seq atomic.Int64

	sub := r.PathPrefix("/api").Subrouter()
	sub.Use(func(inner http.Handler) http.Handler {
		// Captured per wrapper construction, so a reused wrapper reports a stale
		// number while a fresh one reports its own.
		n := seq.Add(1)

		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Seq", strconv.FormatInt(n, 10))
			inner.ServeHTTP(w, req)
		})
	})
	sub.NewRoute().Methods(http.MethodGet).Path("/thing").Handler(chainOKHandler())

	got := make([]string, 0, 3)

	for range 3 {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/thing", http.NoBody))
		got = append(got, w.Header().Get("X-Seq"))
	}

	if got[0] == got[1] && got[1] == got[2] {
		t.Errorf("every request ran the same subrouter middleware instance (%v); the chain "+
			"cache pinned the first request's wrapper", got)
	}
}
