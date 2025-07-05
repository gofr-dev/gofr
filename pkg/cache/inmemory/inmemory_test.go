package inmemory

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeCache initializes the cache and fails the test on error
func makeCache(t *testing.T, opts ...Option) *inMemoryCache {
	t.Helper()
	ci, err := NewInMemoryCache(opts...)
	require.NoError(t, err, "failed to initialize cache")
	return ci.(*inMemoryCache)
}

// Test basic Set/Get/Delete/Exists operations
func TestOperations(t *testing.T) {
	c := makeCache(t, WithName("name"), WithTTL(5*time.Second), WithMaxItems(10))
	defer c.Close()

	require.NoError(t, c.Set("key1", 10))

	v, found, err := c.Get("key1")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 10, v)

	exists, err := c.Exists("key1")
	assert.NoError(t, err)
	assert.True(t, exists)

	assert.NoError(t, c.Delete("key1"))

	exists, err = c.Exists("key1")
	assert.NoError(t, err)
	assert.False(t, exists)
}

// Test Clear method
func TestClear(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute), WithMaxItems(10))
	defer c.Close()

	_ = c.Set("x", 1)
	_ = c.Set("y", 2)

	assert.NoError(t, c.Clear())

	for _, k := range []string{"x", "y"} {
		exist, err := c.Exists(k)
		assert.NoError(t, err)
		assert.False(t, exist)
	}
}

// Test TTL expiration
func TestTTLExpiry(t *testing.T) {
	c := makeCache(t, WithTTL(50*time.Millisecond), WithMaxItems(10))
	defer c.Close()

	_ = c.Set("foo", "bar")
	time.Sleep(60 * time.Millisecond)

	_, found, err := c.Get("foo")
	assert.NoError(t, err)
	assert.False(t, found)
}

// Test eviction due to capacity
func TestCapacityEviction(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute), WithMaxItems(2))
	defer c.Close()

	_ = c.Set("k1", 1)
	time.Sleep(time.Millisecond)
	_ = c.Set("k2", 2)
	_, _, _ = c.Get("k1") // Access to keep recent
	_ = c.Set("k3", 3)

	exists, err := c.Exists("k2")
	assert.NoError(t, err)
	assert.False(t, exists)
}

// Test overwriting existing key
func TestOverwrite(t *testing.T) {
	c := makeCache(t, WithTTL(5*time.Second), WithMaxItems(10))
	defer c.Close()

	_ = c.Set("dupKey", "first")
	_ = c.Set("dupKey", "second")

	v, found, err := c.Get("dupKey")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "second", v)
}

// Test deleting non-existent key
func TestDeleteNonExistent(t *testing.T) {
	c := makeCache(t, WithTTL(5*time.Second), WithMaxItems(10))
	defer c.Close()

	err := c.Delete("ghost")
	assert.NoError(t, err)
}

// Test clearing empty cache
func TestClearEmpty(t *testing.T) {
	c := makeCache(t, WithTTL(5*time.Second), WithMaxItems(10))
	defer c.Close()

	err := c.Clear()
	assert.NoError(t, err)
}

// Test concurrent Set/Get/Exists
func TestConcurrentAccess(t *testing.T) {
	c := makeCache(t, WithTTL(5*time.Second), WithMaxItems(10))
	defer c.Close()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.Set("concurrent", "safe")
			_, _, _ = c.Get("concurrent")
			_, _ = c.Exists("concurrent")
		}()
	}
	wg.Wait()
}

// Test cleanup removes expired before eviction
func TestEvictionEdgeCase(t *testing.T) {
	c := makeCache(t, WithTTL(100*time.Millisecond), WithMaxItems(2))
	defer c.Close()

	_ = c.Set("a", 1)
	time.Sleep(110 * time.Millisecond)
	_ = c.Set("b", 2)
	_ = c.Set("c", 3)

	existsB, err := c.Exists("b")
	assert.NoError(t, err)
	assert.True(t, existsB)

	existsC, err := c.Exists("c")
	assert.NoError(t, err)
	assert.True(t, existsC)
}

// Test default configuration values
func TestDefaultConfiguration(t *testing.T) {
	ci, err := NewInMemoryCache()
	require.NoError(t, err)
	c := ci.(*inMemoryCache)
	defer c.Close()

	assert.Equal(t, time.Minute, c.ttl)
	assert.Equal(t, int(0), c.maxItems)

	_ = c.Set("test", "value")
	v, found, err := c.Get("test")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "value", v)
}

// Last option wins
func TestMultipleOptions(t *testing.T) {
	c := makeCache(t,
		WithTTL(30*time.Second),
		WithMaxItems(5),
		WithTTL(60*time.Second),
	)
	defer c.Close()

	assert.Equal(t, 60*time.Second, c.ttl)
	assert.Equal(t, 5, c.maxItems)
}

// TTL=0 should expire immediately
func TestZeroTTL(t *testing.T) {
	c := makeCache(t, WithTTL(0))
	defer c.Close()

	_ = c.Set("immediate", "expire")
	_, found, _ := c.Get("immediate")
	assert.False(t, found)
}

// TTL<0 should expire immediately
func TestNegativeTTL(t *testing.T) {
	c := makeCache(t, WithTTL(-time.Second))
	defer c.Close()

	_ = c.Set("neg", "ttl")
	_, found, _ := c.Get("neg")
	assert.False(t, found)
}

// maxItems=0 means unlimited
func TestUnlimitedCapacity(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute), WithMaxItems(0))
	defer c.Close()

	for i := 0; i < 500; i++ {
		c.Set(string(rune(i)), i)
	}
	count := 0
	for i := 0; i < 500; i++ {
		exist, _ := c.Exists(string(rune(i)))
		if exist {
			count++
		}
	}
	assert.Equal(t, 500, count)
}

// maxItems=1 should only allow one item
func TestSingleItemCapacity(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute), WithMaxItems(1))
	defer c.Close()

	c.Set("first", 1)
	c.Set("second", 2)

	exist1, _ := c.Exists("first")
	exist2, _ := c.Exists("second")

	assert.NotEqual(t, exist1, exist2)
}

// Test LRU eviction order
func TestLRUEvictionOrder(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute), WithMaxItems(3))
	defer c.Close()

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	c.Get("a")
	c.Set("d", 4)

	existB, _ := c.Exists("b")
	assert.False(t, existB)
}

// Updating key should refresh its usage
func TestUpdateExistingKeyTiming(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute), WithMaxItems(2))
	defer c.Close()

	c.Set("old", 1)
	c.Set("new", 2)
	c.Set("old", 10)
	c.Set("third", 3)

	existNew, _ := c.Exists("new")
	assert.False(t, existNew)
}

// Support for multiple Go types
func TestDifferentValueTypes(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute))
	defer c.Close()

	values := map[string]interface{}{
		"str":    "hello",
		"int":    42,
		"flt":    3.14,
		"bool":   true,
		"slice":  []int{1, 2, 3},
		"map":    map[string]int{"k": 123},
		"nilval": nil,
	}

	for k, val := range values {
		_ = c.Set(k, val)
	}

	for k, expected := range values {
		v, found, _ := c.Get(k)
		if k == "nilval" {
			assert.False(t, found)
		} else {
			assert.True(t, found)
			assert.Equal(t, expected, v)
		}
	}
}

// Using empty key should error
func TestEmptyStringKey(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute))
	defer c.Close()

	err := c.Set("", "v")
	assert.ErrorIs(t, err, ErrEmptyKey)
}

// Long keys are supported
func TestLongKey(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute))
	defer c.Close()

	longKey := string(make([]byte, 10000))
	err := c.Set(longKey, "v")
	assert.NoError(t, err)

	v, found, _ := c.Get(longKey)
	assert.True(t, found)
	assert.Equal(t, "v", v)
}

// Concurrent Set on same key
func TestConcurrentSetSameKey(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute))
	defer c.Close()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			_ = c.Set("race", val)
		}(i)
	}
	wg.Wait()

	_, found, _ := c.Get("race")
	assert.True(t, found)
}

// Concurrent eviction should maintain capacity
func TestConcurrentEviction(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute), WithMaxItems(10))
	defer c.Close()

	for i := 0; i < 10; i++ {
		c.Set(string(rune(i)), i)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			_ = c.Set(string(rune(val+100)), val)
			_, _, _ = c.Get(string(rune(val % 10)))
			_, _ = c.Exists(string(rune(val % 10)))
		}(i)
	}
	wg.Wait()

	count := 0
	for i := 0; i < 200; i++ {
		if exist, _ := c.Exists(string(rune(i))); exist {
			count++
		}
	}
	assert.LessOrEqual(t, count, 10)
}

// Cleanup goroutine should stop after Close
func TestCleanupGoroutineStops(t *testing.T) {
	before := runtime.NumGoroutine()
	c := makeCache(t, WithTTL(time.Millisecond))
	time.Sleep(10 * time.Millisecond)
	c.Close()
	time.Sleep(50 * time.Millisecond)
	after := runtime.NumGoroutine()

	assert.LessOrEqual(t, after, before)
}

// Calling Close multiple times is safe
func TestMultipleClose(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute))
	c.Close()
	c.Close()
	c.Close()
}

// Set/Get still work after Close
func TestOperationsAfterClose(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute))
	_ = c.Set("pre", "close")
	c.Close()
	_ = c.Set("post", "close")

	v1, found1, _ := c.Get("pre")
	assert.True(t, found1)
	assert.Equal(t, "close", v1)

	v2, found2, _ := c.Get("post")
	assert.True(t, found2)
	assert.Equal(t, "close", v2)
}

// Expired items should be cleaned in background
func TestCleanupFrequency(t *testing.T) {
	c := makeCache(t, WithTTL(100*time.Millisecond))
	defer c.Close()

	c.Set("expire_me", "v")
	time.Sleep(250 * time.Millisecond)

	exists, err := c.Exists("expire_me")
	assert.NoError(t, err)
	assert.False(t, exists)
}

// Exists should clean expired keys
func TestExistsWithExpiredItems(t *testing.T) {
	c := makeCache(t, WithTTL(50*time.Millisecond))
	defer c.Close()

	c.Set("short", "v")

	exists, err := c.Exists("short")
	assert.NoError(t, err)
	assert.True(t, exists)

	time.Sleep(60 * time.Millisecond)

	exists, err = c.Exists("short")
	assert.NoError(t, err)
	assert.False(t, exists)
}

// Cleanup should free space for new items
func TestPartialEvictionWithExpiredItems(t *testing.T) {
	c := makeCache(t, WithTTL(100*time.Millisecond), WithMaxItems(3))
	defer c.Close()

	c.Set("a", 1)
	c.Set("b", 2)
	time.Sleep(110 * time.Millisecond)
	c.Set("c", 3)
	c.Set("d", 4)
	c.Set("e", 5)

	for _, k := range []string{"c", "d", "e"} {
		exists, err := c.Exists(k)
		assert.NoError(t, err)
		assert.True(t, exists)
	}
}

// Get should update lastUsed for LRU
func TestGetUpdatesLastUsed(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute), WithMaxItems(2))
	defer c.Close()

	c.Set("x", 1)
	c.Set("y", 2)
	c.Get("x")
	c.Set("z", 3)

	exists, err := c.Exists("y")
	assert.NoError(t, err)
	assert.False(t, exists)
}

// Stress test with mixed operations
func TestHighVolumeOperations(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute), WithMaxItems(1000))
	defer c.Close()

	var wg sync.WaitGroup
	ops := 1000

	for i := 0; i < ops; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := string(rune(id % 100))
			c.Set(key, id)
			c.Get(key)
			c.Exists(key)
			if id%10 == 0 {
				c.Delete(key)
			}
		}(i)
	}
	wg.Wait()

	cnt := 0
	for i := 0; i < 100; i++ {
		if exist, _ := c.Exists(string(rune(i))); exist {
			cnt++
		}
	}
	assert.LessOrEqual(t, cnt, 100)
}
