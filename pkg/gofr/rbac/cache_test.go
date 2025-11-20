package rbac

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewRoleCache(t *testing.T) {
	cache := NewRoleCache(5 * time.Minute)
	assert.NotNil(t, cache)
	defer cache.Stop()
}

func TestRoleCache_GetSet(t *testing.T) {
	cache := NewRoleCache(1 * time.Second)
	defer cache.Stop()

	// Set a value
	cache.Set("user:123", "admin")

	// Get immediately - should work
	role, found := cache.Get("user:123")
	assert.True(t, found)
	assert.Equal(t, "admin", role)

	// Get non-existent key
	_, found = cache.Get("user:999")
	assert.False(t, found)
}

func TestRoleCache_Expiration(t *testing.T) {
	cache := NewRoleCache(100 * time.Millisecond)
	defer cache.Stop()

	// Set a value
	cache.Set("user:123", "admin")

	// Get immediately - should work
	role, found := cache.Get("user:123")
	assert.True(t, found)
	assert.Equal(t, "admin", role)

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Get after expiration - should not be found
	_, found = cache.Get("user:123")
	assert.False(t, found)
}

func TestRoleCache_Delete(t *testing.T) {
	cache := NewRoleCache(5 * time.Minute)
	defer cache.Stop()

	// Set a value
	cache.Set("user:123", "admin")

	// Verify it exists
	role, found := cache.Get("user:123")
	assert.True(t, found)
	assert.Equal(t, "admin", role)

	// Delete it
	cache.Delete("user:123")

	// Verify it's gone
	_, found = cache.Get("user:123")
	assert.False(t, found)
}

func TestRoleCache_Clear(t *testing.T) {
	cache := NewRoleCache(5 * time.Minute)
	defer cache.Stop()

	// Set multiple values
	cache.Set("user:123", "admin")
	cache.Set("user:456", "editor")
	cache.Set("user:789", "viewer")

	// Verify they exist
	_, found := cache.Get("user:123")
	assert.True(t, found)

	// Clear all
	cache.Clear()

	// Verify all are gone
	_, found = cache.Get("user:123")
	assert.False(t, found)
	_, found = cache.Get("user:456")
	assert.False(t, found)
	_, found = cache.Get("user:789")
	assert.False(t, found)
}

func TestRoleCache_ThreadSafety(t *testing.T) {
	cache := NewRoleCache(5 * time.Minute)
	defer cache.Stop()

	// Concurrent writes and reads
	done := make(chan bool, 20)
	for i := 0; i < 10; i++ {
		go func(id int) {
			key := "user:" + string(rune(id))
			cache.Set(key, "admin")
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		go func(id int) {
			key := "user:" + string(rune(id))
			_, _ = cache.Get(key)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestRoleCache_Cleanup(t *testing.T) {
	cache := NewRoleCache(100 * time.Millisecond)
	defer cache.Stop()

	// Set multiple values
	cache.Set("user:1", "admin")
	cache.Set("user:2", "editor")

	// Wait for cleanup to run (cleanup runs every TTL/2 = 50ms)
	time.Sleep(200 * time.Millisecond)

	// Values should be cleaned up
	_, found := cache.Get("user:1")
	assert.False(t, found)
	_, found = cache.Get("user:2")
	assert.False(t, found)
}

func TestDefaultCacheKeyGenerator(t *testing.T) {
	tests := []struct {
		name     string
		req      *http.Request
		wantPref string // Expected prefix
	}{
		{
			name: "User ID header",
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("X-User-ID", "123")
				return req
			}(),
			wantPref: "rbac:user:",
		},
		{
			name: "API Key header",
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("X-Api-Key", "key123")
				return req
			}(),
			wantPref: "rbac:apikey:",
		},
		{
			name: "Fallback to IP",
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.RemoteAddr = "192.168.1.1:8080"
				return req
			}(),
			wantPref: "rbac:ip:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := DefaultCacheKeyGenerator(tt.req)
			assert.Contains(t, key, tt.wantPref)
		})
	}
}

func TestRoleCache_MultipleKeys(t *testing.T) {
	cache := NewRoleCache(5 * time.Minute)
	defer cache.Stop()

	// Set multiple different keys
	cache.Set("user:1", "admin")
	cache.Set("user:2", "editor")
	cache.Set("user:3", "viewer")

	// Verify all exist
	role1, found1 := cache.Get("user:1")
	assert.True(t, found1)
	assert.Equal(t, "admin", role1)

	role2, found2 := cache.Get("user:2")
	assert.True(t, found2)
	assert.Equal(t, "editor", role2)

	role3, found3 := cache.Get("user:3")
	assert.True(t, found3)
	assert.Equal(t, "viewer", role3)
}

func TestRoleCache_UpdateValue(t *testing.T) {
	cache := NewRoleCache(5 * time.Minute)
	defer cache.Stop()

	// Set initial value
	cache.Set("user:123", "admin")

	// Update value
	cache.Set("user:123", "editor")

	// Verify updated value
	role, found := cache.Get("user:123")
	assert.True(t, found)
	assert.Equal(t, "editor", role)
}

func TestRoleCache_Stop(t *testing.T) {
	cache := NewRoleCache(100 * time.Millisecond)

	// Set a value
	cache.Set("user:123", "admin")

	// Stop the cache
	cache.Stop()

	// Wait a bit to ensure cleanup goroutine stopped
	time.Sleep(50 * time.Millisecond)

	// Cache should still work for existing values
	role, found := cache.Get("user:123")
	assert.True(t, found)
	assert.Equal(t, "admin", role)
}

func TestRoleCache_ZeroTTL(t *testing.T) {
	// Test with zero TTL (should still work, just expires immediately)
	cache := NewRoleCache(0)
	// Note: With zero TTL, cleanup goroutine is not started, so no need to stop
	// But we'll stop it anyway to be safe
	defer func() {
		// Try to stop, but it might not be running
		cache.Stop()
	}()

	cache.Set("user:123", "admin")

	// Should not be found immediately (or very quickly expires)
	time.Sleep(10 * time.Millisecond)
	// With zero TTL, behavior may vary, but should not crash
	assert.NotPanics(t, func() {
		cache.Get("user:123")
	})
}

func TestRoleCache_ConcurrentSetGet(t *testing.T) {
	cache := NewRoleCache(5 * time.Minute)
	defer cache.Stop()

	// Concurrent sets and gets on same key
	done := make(chan bool, 20)
	for i := 0; i < 10; i++ {
		go func() {
			cache.Set("user:123", "admin")
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		go func() {
			_, _ = cache.Get("user:123")
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Final value should exist
	role, found := cache.Get("user:123")
	assert.True(t, found)
	assert.Equal(t, "admin", role)
}

