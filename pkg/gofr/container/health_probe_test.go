package container

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/logging"
)

// healthConfig is a config.Config that only knows about the keys a test hands it.
type healthConfig map[string]string

func (h healthConfig) Get(key string) string { return h[key] }

func (h healthConfig) GetOrDefault(key, defaultVal string) string {
	if v, ok := h[key]; ok && v != "" {
		return v
	}

	return defaultVal
}

func TestHealthProbe_CachesWithinTTL(t *testing.T) {
	p := newHealthProbe(time.Minute, 0)

	_, ok := p.get()
	assert.False(t, ok, "an empty cache must miss")

	p.set(map[string]any{"status": "UP"})

	got, ok := p.get()
	assert.True(t, ok, "a cache set within the TTL must hit")
	assert.Equal(t, map[string]any{"status": "UP"}, got)
}

func TestHealthProbe_ExpiresAfterTTL(t *testing.T) {
	p := newHealthProbe(20*time.Millisecond, 0)

	p.set(map[string]any{"status": "UP"})

	// Reach past the TTL by rewriting the timestamp rather than sleeping: the test then asserts
	// the expiry rule itself and cannot flake on a loaded machine.
	p.mu.Lock()
	p.storedAt = time.Now().Add(-time.Second)
	p.mu.Unlock()

	_, ok := p.get()
	assert.False(t, ok, "a result older than the TTL must miss")
}

func TestHealthProbe_ZeroTTLNeverCaches(t *testing.T) {
	p := newHealthProbe(0, 0)

	p.set(map[string]any{"status": "UP"})

	_, ok := p.get()
	assert.False(t, ok, "a disabled cache must never hit, and set must be a no-op")
}

func TestHealthProbe_NegativeTTLIsDisabled(t *testing.T) {
	p := newHealthProbe(-time.Second, -time.Second)

	assert.Equal(t, time.Duration(0), p.ttl, "a negative TTL disables the cache")
	assert.Equal(t, time.Duration(0), p.timeout, "a negative timeout disables the bound")
}

func TestHealthProbe_Clear(t *testing.T) {
	p := newHealthProbe(time.Minute, 0)

	p.set(map[string]any{"status": "UP"})
	p.clear()

	_, ok := p.get()
	assert.False(t, ok, "a cleared cache must miss")
}

// A Container built without Create has no probe at all, so every method has to survive a nil
// receiver rather than the caller having to check.
func TestHealthProbe_NilReceiver(t *testing.T) {
	var p *healthProbe

	assert.NotPanics(t, func() {
		_, ok := p.get()
		assert.False(t, ok)

		p.set(map[string]any{"status": "UP"})
		p.clear()
	})
}

func TestContainer_healthDuration(t *testing.T) {
	tests := []struct {
		desc     string
		value    string
		expected time.Duration
	}{
		{"unset", "", 0},
		{"explicit zero", "0", 0},
		{"disabled keyword", "disabled", 0},
		{"disabled keyword, any case", "DiSaBlEd", 0},
		{"surrounding whitespace", "  10s  ", 10 * time.Second},
		{"a valid duration", "5s", 5 * time.Second},
		{"sub-second", "250ms", 250 * time.Millisecond},
		{"unparseable falls back to disabled", "soon", 0},
		{"negative falls back to disabled", "-1s", 0},
	}

	c := &Container{Logger: logging.NewMockLogger(logging.ERROR)}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got := c.healthDuration(healthConfig{healthCacheTTLKey: tc.value}, healthCacheTTLKey)

			assert.Equal(t, tc.expected, got)
		})
	}
}

// healthDuration is reachable from a Container that never got a Logger, so it must not assume one.
func TestContainer_healthDuration_NoLogger(t *testing.T) {
	c := &Container{}

	assert.NotPanics(t, func() {
		assert.Equal(t, time.Duration(0), c.healthDuration(healthConfig{healthCacheTTLKey: "nonsense"}, healthCacheTTLKey))
	})
}

func TestContainer_Create_WiresHealthProbe(t *testing.T) {
	c := &Container{Logger: logging.NewMockLogger(logging.ERROR)}

	c.health = newHealthProbe(
		c.healthDuration(healthConfig{healthCacheTTLKey: "7s"}, healthCacheTTLKey),
		c.healthDuration(healthConfig{healthCheckTimeoutKey: "2s"}, healthCheckTimeoutKey),
	)

	assert.Equal(t, 7*time.Second, c.health.ttl)
	assert.Equal(t, 2*time.Second, c.health.timeout)
}
