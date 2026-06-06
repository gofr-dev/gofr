package container

import (
	"sync"
	"time"
)

const defaultHealthCacheTTL = 5 * time.Second

type healthCacheEntry struct {
	result    any
	timestamp time.Time
}

type healthCache struct {
	mu    sync.RWMutex
	ttl   time.Duration
	entry *healthCacheEntry
}

func newHealthCache(ttl time.Duration) *healthCache {
	if ttl < 0 {
		ttl = 0
	}

	return &healthCache{ttl: ttl}
}

func (hc *healthCache) get() (any, bool) {
	if hc.ttl == 0 {
		return nil, false
	}

	hc.mu.RLock()
	defer hc.mu.RUnlock()

	if hc.entry == nil {
		return nil, false
	}

	if time.Since(hc.entry.timestamp) > hc.ttl {
		return nil, false
	}

	return hc.entry.result, true
}

func (hc *healthCache) set(result any) {
	if hc.ttl == 0 {
		return
	}

	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.entry = &healthCacheEntry{
		result:    result,
		timestamp: time.Now(),
	}
}

func (hc *healthCache) clear() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.entry = nil
}
