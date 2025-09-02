package redis

import (
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/cache"
)

func makeRedisCache(t *testing.T, opts ...Option) cache.Cache {
	t.Helper()

	allOpts := append([]Option{WithDB(15)}, opts...)

	c, err := NewRedisCache(t.Context(), allOpts...)
	if err != nil {
		t.Skipf("skipping redis tests: could not connect to redis. Error: %v", err)
	}

	t.Cleanup(func() {
		_ = c.Clear(t.Context())
		_ = c.Close(t.Context())
	})

	return c
}

func TestOperations(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeRedisCache(t)

	require.NoError(t, c.Set(ctx, "key1", "value10"))

	v, found, err := c.Get(ctx, "key1")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "value10", v)

	exists, err := c.Exists(ctx, "key1")
	require.NoError(t, err)
	assert.True(t, exists)

	require.NoError(t, c.Delete(ctx, "key1"))

	exists, err = c.Exists(ctx, "key1")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestClear(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeRedisCache(t)

	require.NoError(t, c.Set(ctx, "x", 1))
	require.NoError(t, c.Set(ctx, "y", 2))

	require.NoError(t, c.Clear(ctx))

	for _, k := range []string{"x", "y"} {
		exist, err := c.Exists(ctx, k)
		require.NoError(t, err)
		assert.False(t, exist)
	}
}

func TestTTLExpiry(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeRedisCache(t, WithTTL(50*time.Millisecond))

	require.NoError(t, c.Set(ctx, "foo", "bar"))

	time.Sleep(60 * time.Millisecond)

	_, found, err := c.Get(ctx, "foo")
	require.NoError(t, err)
	assert.False(t, found, "key should have expired and not be found")
}

func TestOverwrite(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeRedisCache(t)

	require.NoError(t, c.Set(ctx, "dupKey", "first"))
	require.NoError(t, c.Set(ctx, "dupKey", "second"))

	v, found, err := c.Get(ctx, "dupKey")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "second", v)
}

func TestDeleteNonExistent(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeRedisCache(t)

	err := c.Delete(ctx, "ghost")
	require.NoError(t, err)
}

func TestClearEmpty(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeRedisCache(t)

	err := c.Clear(ctx)
	require.NoError(t, err)
}

func TestConcurrentAccess(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeRedisCache(t)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			assert.NoError(t, c.Set(ctx, "concurrent", "safe"))
			_, _, err := c.Get(ctx, "concurrent")
			assert.NoError(t, err)
			_, err = c.Exists(ctx, "concurrent")
			assert.NoError(t, err)
		}()
	}

	wg.Wait()
}

func TestMultipleOptions(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
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

func TestDifferentValueTypes(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeRedisCache(t)

	testCases := []struct {
		key      string
		value    any
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

	for _, tc := range testCases {
		require.NoError(t, c.Set(ctx, tc.key, tc.value), "Failed to set key %s", tc.key)
	}

	for _, tc := range testCases {
		v, found, err := c.Get(ctx, tc.key)
		require.NoError(t, err, "Failed to get key %s", tc.key)
		assert.True(t, found, "Key %s not found", tc.key)
		assert.Equal(t, tc.expected, v, "Value mismatch for key %s", tc.key)
	}
}

func TestEmptyStringKey(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeRedisCache(t)

	err := c.Set(ctx, "", "v")
	require.ErrorIs(t, err, ErrEmptyKey)

	_, _, err = c.Get(ctx, "")
	require.ErrorIs(t, err, ErrEmptyKey)

	err = c.Delete(ctx, "")
	require.ErrorIs(t, err, ErrEmptyKey)

	_, err = c.Exists(ctx, "")
	require.ErrorIs(t, err, ErrEmptyKey)
}

func TestNilValue(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeRedisCache(t)

	err := c.Set(ctx, "key", nil)
	require.ErrorIs(t, err, ErrNilValue)
}

func TestConcurrentSetSameKey(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeRedisCache(t)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func(val int) {
			defer wg.Done()
			assert.NoError(t, c.Set(ctx, "race", val))
		}(i)
	}

	wg.Wait()

	_, found, _ := c.Get(ctx, "race")
	assert.True(t, found)
}

func TestOptionValidation(t *testing.T) {
	ctx := t.Context()

	t.Run("Invalid DB number (-1)", func(t *testing.T) {
		originalRegistry := prometheus.DefaultRegisterer
		prometheus.DefaultRegisterer = prometheus.NewRegistry()

		t.Cleanup(func() {
			prometheus.DefaultRegisterer = originalRegistry
		})

		_, err := NewRedisCache(ctx, WithDB(-1))
		require.Error(t, err)
	})

	t.Run("Invalid DB number (16)", func(t *testing.T) {
		originalRegistry := prometheus.DefaultRegisterer
		prometheus.DefaultRegisterer = prometheus.NewRegistry()

		t.Cleanup(func() {
			prometheus.DefaultRegisterer = originalRegistry
		})

		_, err := NewRedisCache(ctx, WithDB(16))
		require.Error(t, err)
	})

	t.Run("Negative TTL", func(t *testing.T) {
		originalRegistry := prometheus.DefaultRegisterer
		prometheus.DefaultRegisterer = prometheus.NewRegistry()

		t.Cleanup(func() {
			prometheus.DefaultRegisterer = originalRegistry
		})

		_, err := NewRedisCache(ctx, WithTTL(-1*time.Second))
		require.Error(t, err)
	})

	t.Run("Empty Address", func(t *testing.T) {
		originalRegistry := prometheus.DefaultRegisterer
		prometheus.DefaultRegisterer = prometheus.NewRegistry()

		t.Cleanup(func() {
			prometheus.DefaultRegisterer = originalRegistry
		})

		_, err := NewRedisCache(ctx, WithAddr(""))
		require.Error(t, err)
	})
}

func TestCacheHitMiss(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeRedisCache(t)

	// Test cache miss
	_, found, err := c.Get(ctx, "nonexistent")
	require.NoError(t, err)
	assert.False(t, found)

	// Test cache hit
	require.NoError(t, c.Set(ctx, "existing", "value"))
	val, found, err := c.Get(ctx, "existing")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "value", val)
}

func TestZeroTTL(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeRedisCache(t, WithTTL(0))

	require.NoError(t, c.Set(ctx, "permanent", "value"))

	// Should still be there after a short wait
	time.Sleep(10 * time.Millisecond)

	val, found, err := c.Get(ctx, "permanent")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "value", val)
}
