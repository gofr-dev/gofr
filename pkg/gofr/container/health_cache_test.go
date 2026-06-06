package container

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHealthCache_GetSet(t *testing.T) {
	cache := newHealthCache(5 * time.Second)

	_, ok := cache.get()
	assert.False(t, ok, "empty cache should return false")

	cache.set("test-result")

	result, ok := cache.get()
	assert.True(t, ok, "cache should return true after set")
	assert.Equal(t, "test-result", result)
}

func TestHealthCache_Expiry(t *testing.T) {
	cache := newHealthCache(50 * time.Millisecond)

	cache.set("test-result")

	_, ok := cache.get()
	assert.True(t, ok, "cache should be valid before expiry")

	time.Sleep(100 * time.Millisecond)

	_, ok = cache.get()
	assert.False(t, ok, "cache should be expired after TTL")
}

func TestHealthCache_Disabled(t *testing.T) {
	cache := newHealthCache(0)

	cache.set("test-result")

	_, ok := cache.get()
	assert.False(t, ok, "disabled cache should never return cached value")
}

func TestHealthCache_Clear(t *testing.T) {
	cache := newHealthCache(5 * time.Second)

	cache.set("test-result")

	_, ok := cache.get()
	assert.True(t, ok, "cache should have value before clear")

	cache.clear()

	_, ok = cache.get()
	assert.False(t, ok, "cache should be empty after clear")
}

func TestHealthCache_NegativeTTL(t *testing.T) {
	cache := newHealthCache(-1 * time.Second)

	cache.set("test-result")

	_, ok := cache.get()
	assert.False(t, ok, "negative TTL should behave as disabled")
}
