package inmemory

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeCache initializes the cache and fails the test on error.
func makeCache(ctx context.Context, t *testing.T, opts ...Option) *inMemoryCache {
	t.Helper()

	ci, err := NewInMemoryCache(ctx, opts...)
	require.NoError(t, err, "failed to initialize cache")

	return ci.(*inMemoryCache)
}

// Test basic Set/Get/Delete/Exists operations.
func TestOperations(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithName("name"), WithTTL(5*time.Second), WithMaxItems(10))
	defer c.Close(ctx)

	require.NoError(t, c.Set(ctx, "key1", 10))

	v, found, err := c.Get(ctx, "key1")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 10, v)

	exists, err := c.Exists(ctx, "key1")
	require.NoError(t, err)
	assert.True(t, exists)

	require.NoError(t, c.Delete(ctx, "key1"))

	exists, err = c.Exists(ctx, "key1")
	require.NoError(t, err)
	assert.False(t, exists)
}

// Test Clear method.
func TestClear(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(time.Minute), WithMaxItems(10))
	defer c.Close(ctx)

	require.NoError(t, c.Set(ctx, "x", 1))
	require.NoError(t, c.Set(ctx, "y", 2))

	require.NoError(t, c.Clear(ctx))

	for _, k := range []string{"x", "y"} {
		exist, err := c.Exists(ctx, k)
		require.NoError(t, err)
		assert.False(t, exist)
	}
}

// Test TTL expiration.
func TestTTLExpiry(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(50*time.Millisecond), WithMaxItems(10))
	defer c.Close(ctx)

	require.NoError(t, c.Set(ctx, "foo", "bar"))
	time.Sleep(60 * time.Millisecond)

	_, found, err := c.Get(ctx, "foo")
	require.NoError(t, err)
	assert.False(t, found)
}

// Test eviction due to capacity.
func TestCapacityEviction(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(time.Minute), WithMaxItems(2))
	defer c.Close(ctx)

	require.NoError(t, c.Set(ctx, "k1", 1))
	time.Sleep(time.Millisecond)
	require.NoError(t, c.Set(ctx, "k2", 2))
	_, _, err := c.Get(ctx, "k1") // Access to keep recent
	require.NoError(t, err)
	require.NoError(t, c.Set(ctx, "k3", 3))

	exists, err := c.Exists(ctx, "k2")
	require.NoError(t, err)
	assert.False(t, exists)
}

// Test overwriting existing key.
func TestOverwrite(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(5*time.Second), WithMaxItems(10))
	defer c.Close(ctx)

	require.NoError(t, c.Set(ctx, "dupKey", "first"))
	require.NoError(t, c.Set(ctx, "dupKey", "second"))

	v, found, err := c.Get(ctx, "dupKey")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "second", v)
}

// Test deleting non-existent key.
func TestDeleteNonExistent(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(5*time.Second), WithMaxItems(10))
	defer c.Close(ctx)

	err := c.Delete(ctx, "ghost")
	require.NoError(t, err)
}

// Test clearing empty cache.
func TestClearEmpty(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(5*time.Second), WithMaxItems(10))
	defer c.Close(ctx)

	err := c.Clear(ctx)
	require.NoError(t, err)
}

// Test concurrent Set/Get/Exists.
func TestConcurrentAccess(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(5*time.Second), WithMaxItems(10))
	defer c.Close(ctx)

	var wg sync.WaitGroup

	f := func() {
		defer wg.Done()
		require.NoError(t, c.Set(ctx, "concurrent", "safe"))
		_, _, err := c.Get(ctx, "concurrent")
		require.NoError(t, err)
		_, err = c.Exists(ctx, "concurrent")
		require.NoError(t, err)
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)

		go f()
	}

	wg.Wait()
}

// Test cleanup removes expired before eviction.
func TestEvictionEdgeCase(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(100*time.Millisecond), WithMaxItems(2))
	defer c.Close(ctx)

	require.NoError(t, c.Set(ctx, "a", 1))
	time.Sleep(110 * time.Millisecond)
	require.NoError(t, c.Set(ctx, "b", 2))
	require.NoError(t, c.Set(ctx, "c", 3))

	existsB, err := c.Exists(ctx, "b")
	require.NoError(t, err)
	assert.True(t, existsB)

	existsC, err := c.Exists(ctx, "c")
	require.NoError(t, err)
	assert.True(t, existsC)
}

// Test default configuration values.
func TestDefaultConfiguration(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()

	ci, err := NewInMemoryCache(ctx)
	require.NoError(t, err)

	c := ci.(*inMemoryCache)
	defer c.Close(ctx)

	assert.Equal(t, time.Minute, c.ttl)
	assert.Equal(t, int(0), c.maxItems)

	require.NoError(t, c.Set(ctx, "test", "value"))
	v, found, err := c.Get(ctx, "test")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "value", v)
}

// Last option wins.
func TestMultipleOptions(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t,
		WithTTL(30*time.Second),
		WithMaxItems(5),
		WithTTL(60*time.Second),
	)
	defer c.Close(ctx)

	assert.Equal(t, 60*time.Second, c.ttl)
	assert.Equal(t, 5, c.maxItems)
}

// TTL=0 should expire immediately.
func TestZeroTTL(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(0))
	defer c.Close(ctx)

	require.NoError(t, c.Set(ctx, "immediate", "expire"))
	_, found, _ := c.Get(ctx, "immediate")
	assert.False(t, found)
}

// TTL<0 should expire immediately.
func TestNegativeTTL(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(-time.Second))
	defer c.Close(ctx)

	require.NoError(t, c.Set(ctx, "neg", "ttl"))
	_, found, _ := c.Get(ctx, "neg")
	assert.False(t, found)
}

// maxItems=0 means unlimited.
func TestUnlimitedCapacity(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(time.Minute), WithMaxItems(0))
	defer c.Close(ctx)

	for i := 0; i < 500; i++ {
		require.NoError(t, c.Set(ctx, string(rune(i)), i))
	}

	count := 0

	for i := 0; i < 500; i++ {
		exist, _ := c.Exists(ctx, string(rune(i)))
		if exist {
			count++
		}
	}

	assert.Equal(t, 500, count)
}

// maxItems=1 should only allow one item.
func TestSingleItemCapacity(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(time.Minute), WithMaxItems(1))
	defer c.Close(ctx)

	require.NoError(t, c.Set(ctx, "first", 1))
	require.NoError(t, c.Set(ctx, "second", 2))

	exist1, _ := c.Exists(ctx, "first")
	exist2, _ := c.Exists(ctx, "second")

	assert.NotEqual(t, exist1, exist2)
}

// Test LRU eviction order.
func TestLRUEvictionOrder(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(time.Minute), WithMaxItems(3))
	defer c.Close(ctx)

	require.NoError(t, c.Set(ctx, "a", 1))
	require.NoError(t, c.Set(ctx, "b", 2))
	require.NoError(t, c.Set(ctx, "c", 3))
	_, _, err := c.Get(ctx, "a")
	require.NoError(t, err)
	require.NoError(t, c.Set(ctx, "d", 4))

	existB, _ := c.Exists(ctx, "b")
	assert.False(t, existB)
}

// Updating key should refresh its usage.
func TestUpdateExistingKeyTiming(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(time.Minute), WithMaxItems(2))
	defer c.Close(ctx)

	require.NoError(t, c.Set(ctx, "old", 1))
	require.NoError(t, c.Set(ctx, "new", 2))
	require.NoError(t, c.Set(ctx, "old", 10))
	require.NoError(t, c.Set(ctx, "third", 3))

	existNew, _ := c.Exists(ctx, "new")
	assert.False(t, existNew)
}

// Support for multiple Go types.
func TestDifferentValueTypes(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(time.Minute))
	defer c.Close(ctx)

	values := map[string]any{
		"str":   "hello",
		"int":   42,
		"flt":   3.14,
		"bool":  true,
		"slice": []int{1, 2, 3},
		"map":   map[string]int{"k": 123},
	}

	err := c.Set(ctx, "nilval", nil)
	require.Error(t, err, "Expected an error when setting a nil value")

	for k, val := range values {
		require.NoError(t, c.Set(ctx, k, val))
	}

	for k, expected := range values {
		v, found, _ := c.Get(ctx, k)
		assert.True(t, found)
		assert.Equal(t, expected, v)
	}

	_, found, _ := c.Get(ctx, "nilval")
	assert.False(t, found)
}

// Using empty key should error.
func TestEmptyStringKey(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(time.Minute))
	defer c.Close(ctx)

	err := c.Set(ctx, "", "v")
	require.ErrorIs(t, err, ErrEmptyKey)
}

// Long keys are supported.
func TestLongKey(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(time.Minute))
	defer c.Close(ctx)

	longKey := string(make([]byte, 10000))
	err := c.Set(ctx, longKey, "v")
	require.NoError(t, err)

	v, found, _ := c.Get(ctx, longKey)
	assert.True(t, found)
	assert.Equal(t, "v", v)
}

// Concurrent Set on same key.
func TestConcurrentEviction(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(time.Minute), WithMaxItems(10))
	defer c.Close(ctx)

	for i := 0; i < 10; i++ {
		require.NoError(t, c.Set(ctx, string(rune(i)), i))
	}

	var wg sync.WaitGroup

	f := func(val int) {
		defer wg.Done()
		require.NoError(t, c.Set(ctx, string(rune(val+100)), val))
		_, _, err := c.Get(ctx, string(rune(val%10)))
		require.NoError(t, err)
		_, err = c.Exists(ctx, string(rune(val%10)))
		require.NoError(t, err)
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)

		go f(i)
	}

	wg.Wait()

	count := 0

	for i := 0; i < 200; i++ {
		if exist, _ := c.Exists(ctx, string(rune(i))); exist {
			count++
		}
	}

	assert.LessOrEqual(t, count, 10)
}

// Cleanup goroutine should stop after Close.
func TestCleanupGoroutineStops(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	before := runtime.NumGoroutine()
	c := makeCache(ctx, t, WithTTL(time.Millisecond))
	time.Sleep(10 * time.Millisecond)
	c.Close(ctx)
	time.Sleep(50 * time.Millisecond)

	after := runtime.NumGoroutine()
	assert.LessOrEqual(t, after, before)
}

// Calling Close multiple times is safe.
func TestMultipleClose(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(time.Minute))
	c.Close(ctx)
	c.Close(ctx)
	c.Close(ctx)
}

// Set/Get still work after Close.
func TestOperationsAfterClose(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(time.Minute))
	require.NoError(t, c.Set(ctx, "pre", "close"))
	c.Close(ctx)
	require.NoError(t, c.Set(ctx, "post", "close"))

	v1, found1, _ := c.Get(ctx, "pre")
	assert.True(t, found1)
	assert.Equal(t, "close", v1)

	v2, found2, _ := c.Get(ctx, "post")
	assert.True(t, found2)
	assert.Equal(t, "close", v2)
}

// Expired items should be cleaned in background.
func TestCleanupFrequency(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(100*time.Millisecond))
	defer c.Close(ctx)

	require.NoError(t, c.Set(ctx, "expire_me", "v"))
	time.Sleep(250 * time.Millisecond)

	exists, err := c.Exists(ctx, "expire_me")
	require.NoError(t, err)
	assert.False(t, exists)
}

// Exists should clean expired keys.
func TestExistsWithExpiredItems(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(50*time.Millisecond))
	defer c.Close(ctx)

	require.NoError(t, c.Set(ctx, "short", "v"))

	exists, err := c.Exists(ctx, "short")
	require.NoError(t, err)
	assert.True(t, exists)

	time.Sleep(60 * time.Millisecond)

	exists, err = c.Exists(ctx, "short")
	require.NoError(t, err)
	assert.False(t, exists)
}

// Cleanup should free space for new items.
func TestPartialEvictionWithExpiredItems(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(100*time.Millisecond), WithMaxItems(3))
	defer c.Close(ctx)

	require.NoError(t, c.Set(ctx, "a", 1))
	require.NoError(t, c.Set(ctx, "b", 2))
	time.Sleep(110 * time.Millisecond)
	require.NoError(t, c.Set(ctx, "c", 3))
	require.NoError(t, c.Set(ctx, "d", 4))
	require.NoError(t, c.Set(ctx, "e", 5))

	for _, k := range []string{"c", "d", "e"} {
		exists, err := c.Exists(ctx, k)
		require.NoError(t, err)
		assert.True(t, exists)
	}
}

// Get should update lastUsed for LRU.
func TestGetUpdatesLastUsed(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(time.Minute), WithMaxItems(2))
	defer c.Close(ctx)

	require.NoError(t, c.Set(ctx, "x", 1))
	require.NoError(t, c.Set(ctx, "y", 2))
	_, _, err := c.Get(ctx, "x")
	require.NoError(t, err)
	require.NoError(t, c.Set(ctx, "z", 3))

	exists, err := c.Exists(ctx, "y")
	require.NoError(t, err)
	assert.False(t, exists)
}

// Stress test with mixed operations.
func TestHighVolumeOperations(t *testing.T) {
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegistry
	})

	ctx := t.Context()
	c := makeCache(ctx, t, WithTTL(time.Minute), WithMaxItems(1000))
	defer c.Close(ctx)

	var wg sync.WaitGroup

	ops := 1000

	f := func(id int) {
		defer wg.Done()

		key := string(rune(id % 100))
		val := id

		require.NoError(t, c.Set(ctx, key, val))
		_, _, err := c.Get(ctx, key)
		require.NoError(t, err)
		_, err = c.Exists(ctx, key)
		require.NoError(t, err)

		if id%10 == 0 {
			require.NoError(t, c.Delete(ctx, key))
		}
	}

	for i := 0; i < ops; i++ {
		wg.Add(1)

		go f(i)
	}

	wg.Wait()

	cnt := 0

	for i := 0; i < 100; i++ {
		if exist, _ := c.Exists(ctx, string(rune(i))); exist {
			cnt++
		}
	}

	assert.LessOrEqual(t, cnt, 100)
}
