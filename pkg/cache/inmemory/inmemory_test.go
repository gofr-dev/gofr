package inmemory

import (
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"
)

// makeCache initializes the cache and fails the test on error
func makeCache(t *testing.T, opts ...Option) *inMemoryCache {
	t.Helper()
	ci, err := NewInMemoryCache(opts...)
	if err != nil {
		t.Fatalf("failed to initialize cache: %v", err)
	}
	return ci.(*inMemoryCache)
}

// basic Set/Get/Delete/Exists flow and error returns
func TestOperations(t *testing.T) {
	c := makeCache(t, WithName("name"), WithTTL(5*time.Second), WithMaxItems(10))
	defer c.Close()

	// Set and Get
	if err := c.Set("key1", 10); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	v, found, err := c.Get("key1")
	if err != nil || !found || v.(int) != 10 {
		t.Errorf("Get returned (%v, %v, %v), want (10, true, nil)", v, found, err)
	}

	// Exists
	exists, err := c.Exists("key1")
	if err != nil || !exists {
		t.Errorf("Exists returned (%v, %v), want (true, nil)", exists, err)
	}

	// Delete
	if err := c.Delete("key1"); err != nil {
		t.Errorf("Delete failed: %v", err)
	}
	exists, err = c.Exists("key1")
	if err != nil {
		t.Errorf("Exists post-delete error: %v", err)
	}
	if exists {
		t.Error("Expected key1 to be gone after Delete")
	}
}

// Clear removes all items safely
func TestClear(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute), WithMaxItems(10))
	defer c.Close()
	_ = c.Set("x", 1)
	_ = c.Set("y", 2)
	if err := c.Clear(); err != nil {
		t.Errorf("Clear failed: %v", err)
	}
	for _, k := range []string{"x", "y"} {
		exist, err := c.Exists(k)
		if err != nil {
			t.Errorf("Exists(%s) error: %v", k, err)
		}
		if exist {
			t.Errorf("Expected %s to be removed by Clear", k)
		}
	}
}

// items expire after TTL
func TestTTLExpiry(t *testing.T) {
	c := makeCache(t, WithTTL(50*time.Millisecond), WithMaxItems(10))
	defer c.Close()
	_ = c.Set("foo", "bar")
	time.Sleep(60 * time.Millisecond)
	_, found, err := c.Get("foo")
	if err != nil {
		t.Errorf("Get error after TTL: %v", err)
	}
	if found {
		t.Error("Expected 'foo' to expire after TTL")
	}
}

// LRU eviction when capacity exceeded
func TestCapacityEviction(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute), WithMaxItems(2))
	defer c.Close()
	_ = c.Set("k1", 1)
	time.Sleep(time.Millisecond)
	_ = c.Set("k2", 2)
	// Access k1 to keep it recent
	_, _, _ = c.Get("k1")
	_ = c.Set("k3", 3)
	// k2 should be evicted
	exists, err := c.Exists("k2")
	if err != nil {
		t.Errorf("Exists error: %v", err)
	}
	if exists {
		t.Error("Expected 'k2' to be evicted as LRU")
	}
}

// Set on existing key updates value and lastUsed
func TestOverwrite(t *testing.T) {
	c := makeCache(t, WithTTL(5*time.Second), WithMaxItems(10))
	defer c.Close()
	_ = c.Set("dupKey", "first")
	_ = c.Set("dupKey", "second")
	v, found, _ := c.Get("dupKey")
	if !found || v != "second" {
		t.Errorf("Expected 'second', got %v", v)
	}
}

// Delete on missing key is safe
func TestDeleteNonExistent(t *testing.T) {
	c := makeCache(t, WithTTL(5*time.Second), WithMaxItems(10))
	defer c.Close()
	if err := c.Delete("ghost"); err != nil {
		t.Errorf("Delete non-existent returned error: %v", err)
	}
}

// Clear on empty cache is safe
func TestClearEmpty(t *testing.T) {
	c := makeCache(t, WithTTL(5*time.Second), WithMaxItems(10))
	defer c.Close()
	if err := c.Clear(); err != nil {
		t.Errorf("Clear empty returned error: %v", err)
	}
}

// ensure thread-safety for Set/Get/Exists
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

// expired items are cleaned before eviction
func TestEvictionEdgeCase(t *testing.T) {
	c := makeCache(t, WithTTL(100*time.Millisecond), WithMaxItems(2))
	defer c.Close()
	_ = c.Set("a", 1)
	time.Sleep(110 * time.Millisecond)
	_ = c.Set("b", 2)
	_ = c.Set("c", 3)
	// both 'b' and 'c' should exist
	exists, err := c.Exists("b")
	if err != nil {
		t.Errorf("Exists error: %v", err)
	}
	if !exists {
		t.Error("Expected 'b' to exist")
	}
	exists, err = c.Exists("c")
	if err != nil {
		t.Errorf("Exists error: %v", err)
	}
	if !exists {
		t.Error("Expected 'c' to exist")
	}
}

// validate defaults and basic usage
func TestDefaultConfiguration(t *testing.T) {
	ci, err := NewInMemoryCache()
	if err != nil {
		t.Fatalf("NewInMemoryCache error: %v", err)
	}
	c := ci.(*inMemoryCache)
	defer c.Close()
	if c.ttl != time.Minute {
		t.Errorf("Default TTL = %v, want 1m", c.ttl)
	}
	if c.maxItems != 0 {
		t.Errorf("Default maxItems = %d, want 0", c.maxItems)
	}
	_ = c.Set("test", "value")
	v, found, _ := c.Get("test")
	if !found || v != "value" {
		t.Error("Default config failed to store/get")
	}
}

// last option wins for conflicting settings
func TestMultipleOptions(t *testing.T) {
	c := makeCache(t,
		WithTTL(30*time.Second),
		WithMaxItems(5),
		WithTTL(60*time.Second),
	)
	defer c.Close()
	if c.ttl != 60*time.Second {
		t.Errorf("TTL = %v, want 60s", c.ttl)
	}
	if c.maxItems != 5 {
		t.Errorf("maxItems = %d, want 5", c.maxItems)
	}
}

// TTL == 0 should expire immediately
func TestZeroTTL(t *testing.T) {
	c := makeCache(t, WithTTL(0))
	defer c.Close()
	c.Set("immediate", "expire")
	if _, found, _ := c.Get("immediate"); found {
		t.Error("Zero TTL: item should expire immediately")
	}
}

// TTL <=0 should expire immediately
func TestNegativeTTL(t *testing.T) {
	c := makeCache(t, WithTTL(-time.Second))
	defer c.Close()
	c.Set("neg", "ttl")
	if _, found, _ := c.Get("neg"); found {
		t.Error("Negative TTL: item should expire immediately")
	}
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
		exists, _ := c.Exists(string(rune(i)))
		if exists {
			count++
		}
	}
	if count != 500 {
		t.Errorf("Unlimited capacity: got %d items, want 500", count)
	}
}

// capacity=1 evicts correct item
func TestSingleItemCapacity(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute), WithMaxItems(1))
	defer c.Close()
	c.Set("first", 1)
	c.Set("second", 2)
	count := 0
	exists, _ := c.Exists("first")
	if exists {
		count++
	}
	exists, _ = c.Exists("second")
	if exists {
		count++
	}
	if count != 1 {
		t.Errorf("Capacity=1: have %d items, want 1", count)
	}
}

// verifies LRU ordering eviction
func TestLRUEvictionOrder(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute), WithMaxItems(3))
	defer c.Close()
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	c.Get("a") // mark a as recent
	c.Set("d", 4)
	exists, err := c.Exists("b")
	if err != nil {
		t.Errorf("Exists error: %v", err)
	}
	if exists {
		t.Error("LRU eviction: expected 'b' to be evicted")
	}
}

// updating key refreshes lastUsed for eviction
func TestUpdateExistingKeyTiming(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute), WithMaxItems(2))
	defer c.Close()
	c.Set("old", 1)
	c.Set("new", 2)
	c.Set("old", 10) // update old
	c.Set("third", 3)
	exists, err := c.Exists("new")
	if err != nil {
		t.Errorf("Exists error: %v", err)
	}
	if exists {
		t.Error("Update existing: expected 'new' to be evicted")
	}
}

// store and retrieve various Go types
func TestDifferentValueTypes(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute))
	defer c.Close()
	c.Set("str", "hello")
	c.Set("int", 42)
	c.Set("flt", 3.14)
	c.Set("bool", true)
	c.Set("slice", []int{1, 2, 3})
	c.Set("map", map[string]int{"k": 123})
	c.Set("nilval", nil)
	for k, want := range map[string]interface{}{"str": "hello", "int": 42, "flt": 3.14, "bool": true} {
		v, found, _ := c.Get(k)
		if !found || v != want {
			t.Errorf("%s: got %v, want %v", k, v, want)
		}
	}
	_, foundNil, _ := c.Get("nilval")
	if foundNil {
		t.Error("nil value: expected found=false")
	}
}

// using empty string as key is invalid
func TestEmptyStringKey(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute))
	defer c.Close()
	if err := c.Set("", "v"); !errors.Is(err, ErrEmptyKey) {
		t.Errorf("Empty key: expected ErrEmptyKey, got %v", err)
	}
}

// very long keys are supported
func TestLongKey(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute))
	defer c.Close()
	longKey := string(make([]byte, 10000))
	if err := c.Set(longKey, "v"); err != nil {
		t.Errorf("Long key Set error: %v", err)
	}
	v, found, _ := c.Get(longKey)
	if !found || v != "v" {
		t.Error("Long key Get failed")
	}
}

// concurrent Set on same key does not panic
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
	if _, found, _ := c.Get("race"); !found {
		t.Error("Concurrent Set: expected key to exist")
	}
}

// concurrent sets and eviction maintain capacity
func TestConcurrentEviction(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute), WithMaxItems(10))
	defer c.Close()
	// prefill
	for i := 0; i < 10; i++ {
		c.Set(string(rune(i)), i)
	}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			c.Set(string(rune(val+100)), val)
			c.Get(string(rune(val % 10)))
			c.Exists(string(rune(val % 10)))
		}(i)
	}

	wg.Wait()

	count := 0
	for i := 0; i < 200; i++ {
		if exist, _ := c.Exists(string(rune(i))); exist {
			count++
		}
	}

	if count > 10 {
		t.Errorf("Concurrent eviction: %d items, want <=10", count)
	}
}

// cleanup goroutine exits on Close
func TestCleanupGoroutineStops(t *testing.T) {
	before := runtime.NumGoroutine()
	c := makeCache(t, WithTTL(time.Millisecond))
	time.Sleep(10 * time.Millisecond)
	c.Close()
	time.Sleep(50 * time.Millisecond)
	after := runtime.NumGoroutine()
	if after > before {
		t.Errorf("Cleanup goroutine still running: before=%d after=%d", before, after)
	}
}

// multiple Close calls are safe
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
	if !found1 || v1 != "close" {
		t.Error("pre-close value lost")
	}
	v2, found2, _ := c.Get("post")
	if !found2 || v2 != "close" {
		t.Error("post-close Set/Get failed")
	}
}

// background cleanup runs periodically
func TestCleanupFrequency(t *testing.T) {
	c := makeCache(t, WithTTL(100*time.Millisecond))
	defer c.Close()
	c.Set("expire_me", "v")
	time.Sleep(250 * time.Millisecond)
	exists, err := c.Exists("expire_me")
	if err != nil {
		t.Errorf("Exists error: %v", err)
	}
	if exists {
		t.Error("Cleanup did not remove expired item")
	}
}

// Exists removes expired items
func TestExistsWithExpiredItems(t *testing.T) {
	c := makeCache(t, WithTTL(50*time.Millisecond))
	defer c.Close()
	c.Set("short", "v")
	exists, err := c.Exists("short")
	if err != nil {
		t.Errorf("Exists error: %v", err)
	}
	if !exists {
		t.Error("Expected short to exist initially")
	}
	time.Sleep(60 * time.Millisecond)
	exists, err = c.Exists("short")
	if err != nil {
		t.Errorf("Exists error: %v", err)
	}
	if exists {
		t.Error("Expired item still exists in Exists")
	}
}

// cleanup frees space for new items
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
		if err != nil {
			t.Errorf("Exists error: %v", err)
		}
		if !exists {
			t.Errorf("Expected %s to exist after cleanup eviction", k)
		}
	}
}

// Get refreshes LRU timestamp
func TestGetUpdatesLastUsed(t *testing.T) {
	c := makeCache(t, WithTTL(time.Minute), WithMaxItems(2))
	defer c.Close()
	c.Set("x", 1)
	c.Set("y", 2)
	c.Get("x")
	c.Set("z", 3)
	exists, err := c.Exists("y")
	if err != nil {
		t.Errorf("Exists error: %v", err)
	}
	if exists {
		t.Error("Expected 'y' to be evicted after Get on 'x'")
	}
}

// stress test mixed operations
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
	if cnt > 100 {
		t.Errorf("HighVolume: got %d, want <=100", cnt)
	}
}
