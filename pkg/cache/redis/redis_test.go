package redis

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gofr.dev/pkg/cache"
)

// makeRedisCache initializes the cache and skips the test on connection error
func makeRedisCache(t *testing.T, opts ...Option) cache.Cache {
	t.Helper()
	// Use a different database for testing to avoid conflicts
	allOpts := append([]Option{WithDB(15)}, opts...)
	c, err := NewRedisCache(context.Background(), allOpts...)
	if err != nil {
		t.Skipf("skipping redis tests: could not connect to redis. Error: %v", err)
	}

	// Cleanup hook to clear the database after each test
	t.Cleanup(func() {
		_ = c.Clear(context.Background())
		_ = c.Close(context.Background())
	})

	return c
}

// Test basic Set/Get/Delete/Exists operations
func TestOperations(t *testing.T) {
	ctx := context.Background()
	c := makeRedisCache(t)

	require.NoError(t, c.Set(ctx, "key1", "value10"))

	v, found, err := c.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "value10", v)

	exists, err := c.Exists(ctx, "key1")
	assert.NoError(t, err)
	assert.True(t, exists)

	assert.NoError(t, c.Delete(ctx, "key1"))

	exists, err = c.Exists(ctx, "key1")
	assert.NoError(t, err)
	assert.False(t, exists)
}

// Test Clear method
func TestClear(t *testing.T) {
	ctx := context.Background()
	c := makeRedisCache(t)

	_ = c.Set(ctx, "x", 1)
	_ = c.Set(ctx, "y", 2)

	assert.NoError(t, c.Clear(ctx))

	for _, k := range []string{"x", "y"} {
		exist, err := c.Exists(ctx, k)
		assert.NoError(t, err)
		assert.False(t, exist)
	}
}

// Test TTL expiration
func TestTTLExpiry(t *testing.T) {
	ctx := context.Background()
	c := makeRedisCache(t, WithTTL(50*time.Millisecond))

	_ = c.Set(ctx, "foo", "bar")
	time.Sleep(60 * time.Millisecond)

	_, found, err := c.Get(ctx, "foo")
	assert.NoError(t, err)
	assert.False(t, found, "key should have expired and not be found")
}

// Test overwriting existing key
func TestOverwrite(t *testing.T) {
	ctx := context.Background()
	c := makeRedisCache(t)

	_ = c.Set(ctx, "dupKey", "first")
	_ = c.Set(ctx, "dupKey", "second")

	v, found, err := c.Get(ctx, "dupKey")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "second", v)
}

// Test deleting non-existent key
func TestDeleteNonExistent(t *testing.T) {
	ctx := context.Background()
	c := makeRedisCache(t)

	err := c.Delete(ctx, "ghost")
	assert.NoError(t, err)
}

// Test clearing empty cache
func TestClearEmpty(t *testing.T) {
	ctx := context.Background()
	c := makeRedisCache(t)

	err := c.Clear(ctx)
	assert.NoError(t, err)
}

// Test concurrent Set/Get/Exists
func TestConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	c := makeRedisCache(t)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.Set(ctx, "concurrent", "safe")
			_, _, _ = c.Get(ctx, "concurrent")
			_, _ = c.Exists(ctx, "concurrent")
		}()
	}
	wg.Wait()
}

// Last option wins
func TestMultipleOptions(t *testing.T) {
	ctx := context.Background()
	rc, err := NewRedisCache(ctx,
		WithTTL(30*time.Second),
		WithDB(14),
		WithTTL(60*time.Second),
	)
	require.NoError(t, err)
	
	c := rc.(*redisCache)
	defer c.Close(ctx)

	assert.Equal(t, 60*time.Second, c.ttl)
	assert.Equal(t, 14, c.client.Options().DB)
}

// Test support for different value types - Fixed to handle Redis string conversion
func TestDifferentValueTypes(t *testing.T) {
	ctx := context.Background()
	c := makeRedisCache(t)

	testCases := []struct {
		key      string
		value    interface{}
		expected string
	}{
		{"str", "hello", "hello"},
		{"int", 42, "42"},
		{"flt", 3.14, "3.14"},
		{"bool_true", true, "true"},
		{"bool_false", false, "false"},
		{"int64", int64(123), "123"},
		{"float32", float32(2.5), "2.5"},
	}

	// Set all values
	for _, tc := range testCases {
		require.NoError(t, c.Set(ctx, tc.key, tc.value), "Failed to set key %s", tc.key)
	}

	// Get and verify all values
	for _, tc := range testCases {
		v, found, err := c.Get(ctx, tc.key)
		assert.NoError(t, err, "Failed to get key %s", tc.key)
		assert.True(t, found, "Key %s not found", tc.key)
		assert.Equal(t, tc.expected, v, "Value mismatch for key %s", tc.key)
	}
}

// Using empty key should error
func TestEmptyStringKey(t *testing.T) {
	ctx := context.Background()
	c := makeRedisCache(t)

	err := c.Set(ctx, "", "v")
	assert.ErrorIs(t, err, ErrEmptyKey)

	_, _, err = c.Get(ctx, "")
	assert.ErrorIs(t, err, ErrEmptyKey)

	err = c.Delete(ctx, "")
	assert.ErrorIs(t, err, ErrEmptyKey)

	_, err = c.Exists(ctx, "")
	assert.ErrorIs(t, err, ErrEmptyKey)
}

// Test nil value error
func TestNilValue(t *testing.T) {
	ctx := context.Background()
	c := makeRedisCache(t)

	err := c.Set(ctx, "key", nil)
	assert.ErrorIs(t, err, ErrNilValue)
}

// Concurrent Set on same key
func TestConcurrentSetSameKey(t *testing.T) {
	ctx := context.Background()
	c := makeRedisCache(t)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			_ = c.Set(ctx, "race", val)
		}(i)
	}
	wg.Wait()

	_, found, _ := c.Get(ctx, "race")
	assert.True(t, found)
}

// Test option validation
func TestOptionValidation(t *testing.T) {
	ctx := context.Background()

	// Test invalid DB number
	_, err := NewRedisCache(ctx, WithDB(-1))
	assert.Error(t, err)

	_, err = NewRedisCache(ctx, WithDB(16))
	assert.Error(t, err)

	// Test negative TTL
	_, err = NewRedisCache(ctx, WithTTL(-1*time.Second))
	assert.Error(t, err)

	// Test empty address
	_, err = NewRedisCache(ctx, WithAddr(""))
	assert.Error(t, err)
}

// Test cache hit/miss scenarios
func TestCacheHitMiss(t *testing.T) {
	ctx := context.Background()
	c := makeRedisCache(t)

	// Test cache miss
	_, found, err := c.Get(ctx, "nonexistent")
	assert.NoError(t, err)
	assert.False(t, found)

	// Test cache hit
	require.NoError(t, c.Set(ctx, "existing", "value"))
	val, found, err := c.Get(ctx, "existing")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "value", val)
}

// Test zero TTL (no expiration)
func TestZeroTTL(t *testing.T) {
	ctx := context.Background()
	c := makeRedisCache(t, WithTTL(0))

	require.NoError(t, c.Set(ctx, "permanent", "value"))
	
	// Should still be there after a short wait
	time.Sleep(10 * time.Millisecond)
	val, found, err := c.Get(ctx, "permanent")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "value", val)
}
