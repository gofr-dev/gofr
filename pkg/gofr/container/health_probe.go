package container

import (
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// healthProbe owns everything the health endpoint needs beyond the datasources themselves: the TTL
// cache, the singleflight that collapses concurrent probes into one round of checks, and the bound
// on how long that round may take.
//
// It hangs off Container behind a pointer deliberately, and every method tolerates a nil receiver.
// Container is copied by value on a supported path — injectContainer in package gofr sets a
// container.Container field on a user's gRPC server struct — so a mutex may not live in Container
// itself, and a Container built without Create has no probe at all.
//
// Caching is off unless HEALTH_CACHE_TTL is set. Serving a health body up to a TTL old is a trade an
// operator opts into: inside that window a recovered backend still reports DOWN, and one that has
// just failed still reports UP.
type healthProbe struct {
	// Set once by newHealthProbe and never written again, so get and set read ttl before taking
	// the lock. Making the TTL reloadable at runtime means moving it below and locking those reads.
	group   singleflight.Group
	timeout time.Duration
	ttl     time.Duration

	// Guarded by mu.
	mu       sync.RWMutex
	result   map[string]any
	storedAt time.Time
	running  bool
}

// beginRound reports whether a fresh round of checks may start, and claims the round if so.
//
// It returns false only after a timeout: the leader abandons the outstanding checks rather than
// canceling them — SQL, Redis and PubSub health methods take no context — and singleflight
// releases its key as soon as that leader returns. Without this guard the next probe would start a
// second set of goroutines against the same unanswering backend, and under Kubernetes probe traffic
// that is one more leaked goroutine every probe interval, indefinitely.
func (p *healthProbe) beginRound() bool {
	if p == nil {
		return true
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return false
	}

	p.running = true

	return true
}

// endRound releases the claim taken by beginRound. After a timeout it must not be called until the
// abandoned checks have actually finished, or the guard would let a second set start anyway.
func (p *healthProbe) endRound() {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.running = false
}

// newHealthProbe returns a probe with the given cache TTL and per-round timeout. Zero disables
// either one; a disabled cache is a working cache that never hits, not an absent one.
func newHealthProbe(ttl, timeout time.Duration) *healthProbe {
	if ttl < 0 {
		ttl = 0
	}

	if timeout < 0 {
		timeout = 0
	}

	return &healthProbe{ttl: ttl, timeout: timeout}
}

// get returns the stored map itself rather than a copy. Container.Health is its only caller and
// clones before the result leaves the package; nothing else may read from it.
func (p *healthProbe) get() (map[string]any, bool) {
	if p == nil || p.ttl == 0 {
		return nil, false
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.result == nil || time.Since(p.storedAt) > p.ttl {
		return nil, false
	}

	return p.result, true
}

func (p *healthProbe) set(result map[string]any) {
	if p == nil || p.ttl == 0 {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.result = result
	p.storedAt = time.Now()
}

func (p *healthProbe) clear() {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.result = nil
}
