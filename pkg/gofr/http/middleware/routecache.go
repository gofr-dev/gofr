package middleware

import (
	"net/http"
	"sync"
	"sync/atomic"
)

// routeCacheLimit caps the number of entries any per-route cache in this package
// will hold.
//
// With cacheableMethod filtering the method half of the key, the reachable key
// space is (standard methods x registered routes), which is fixed at startup —
// so this cap is a backstop for a pathologically large route table, not the
// primary bound. It is a backstop that matters: the primary bound is an argument
// about reachability, and an argument is a weaker thing to rest process memory on
// than a number.
const routeCacheLimit = 4096

// cacheableMethod reports whether method may be used as part of a cache key.
//
// This is the load-bearing half of every per-route cache's bound, and it exists
// because the "only bounded route templates are cached" claim does not survive
// contact with a real GoFr app. gofr.go registers a PathPrefix("/") catch-all, so
// mux.CurrentRoute is set for EVERY request and GetPathTemplate() returns "/"
// even for a path no route matched — the templated guard is therefore true
// always, and the route half of the key is not the bound it looks like.
//
// The method half is worse, because it is not drawn from a fixed set at all:
// net/http accepts any RFC 7230 token as a method, so an unauthenticated client
// streaming M00001, M00002, ... mints a permanent cache entry per request and
// grows resident memory without bound. Restricting the key to methods that are
// actually defined turns "methods x routes" back into the fixed quantity the
// caches were designed around.
//
// A request with a non-standard method still works — its per-route data is built
// fresh, exactly as it was before any cache existed. It just never persists.
func cacheableMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodConnect,
		http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

// routeKey identifies a (method, route-template) pair.
type routeKey struct {
	method, route string
}

// routeCache is a bounded, concurrent cache of V keyed on any comparable
// per-route key.
//
// The KEY is `any` because the caches sharing this bound do not share a key
// shape: the tracer keys on (method, route) while the metrics recorders key on
// (path, method, status) and (path, method). What they DO share is the exposure
// - all three are fed from the same attacker-influenced method and path - so the
// bound is what belongs in one place, not the key type.
//
// The VALUE is a type parameter rather than `any` so that callers do not each
// carry a type assertion. A cache is single-consumer, so those assertions could
// never fail, which left every one of them with a branch no test could reach and
// nothing but a comment explaining why. Naming the type here deletes them: the
// single assertion below is the only one left, and store makes it unfalsifiable.
//
// Reads stay on sync.Map's lock-free path. Writes stop at routeCacheLimit rather
// than evicting: the entries are pure functions of their key and are all equally
// valid forever, so there is no staleness to evict for, and refusing to grow is
// both cheaper and easier to reason about than an eviction policy. Past the cap
// the affected requests rebuild their data per request — the optimization stops
// applying, but memory is flat and behavior is unchanged.
type routeCache[V any] struct {
	entries sync.Map
	count   atomic.Int64
}

// load returns the cached value for key, if present.
//
// The assertion cannot fail: store is the only writer and it takes a V.
func (c *routeCache[V]) load(key any) (value V, ok bool) {
	v, ok := c.entries.Load(key)
	if !ok {
		return value, false
	}

	return v.(V), true
}

// store caches value under key unless the cache is already at its limit.
//
// The count is advisory: concurrent stores can race past the cap by the number of
// goroutines in flight, which is bounded and harmless. What matters is that the
// cache cannot grow without limit.
func (c *routeCache[V]) store(key any, value V) {
	if c.count.Load() >= routeCacheLimit {
		return
	}

	if _, loaded := c.entries.LoadOrStore(key, value); !loaded {
		c.count.Add(1)
	}
}

// len reports the number of cached entries. Test-only.
func (c *routeCache[V]) len() int64 {
	return c.count.Load()
}

// reset empties the cache. Test-only: the package-level caches are process-wide,
// so a test that asserts on size has to start from a known state.
func (c *routeCache[V]) reset() {
	c.entries.Range(func(k, _ any) bool {
		c.entries.Delete(k)

		return true
	})
	c.count.Store(0)
}
