package inmemory

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestOperations(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Second * 5), WithMaxItems(10))

	c.Set("key1", 10)
	v, found := c.Get("key1")
	if !found || v.(int) != 10 {
		t.Errorf("Expected 10, got %v", v)
	}

	if !c.Exists("key1") {
		t.Error("Exists: expected key key1 to exist")
	}

	c.Delete("key1")
	if c.Exists("key1") {
		t.Error("Delete: expected key key1 to be gone")
	}
}

func TestClear(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Minute * 1), WithMaxItems(10))
	c.Set("x", 1)
	c.Set("y", 2)
	c.Clear()
	if c.Exists("x") || c.Exists("y") {
		t.Error("Clear: expected cache empty")
	}
}

func TestTTLExpiry(t *testing.T) {
	// very short TTL
	c := NewInMemoryCache(WithTTL(50 * time.Millisecond), WithMaxItems(10))
	defer c.(*inMemoryCache).Close() // Ensure cleanup goroutine stops
	
	c.Set("foo", "bar")
	time.Sleep(60 * time.Millisecond)

	if _, ok := c.Get("foo"); ok {
		t.Error("Expected foo to expire after TTL")
	}
}

func TestCapacityEviction(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Minute * 1), WithMaxItems(2))
	defer c.(*inMemoryCache).Close() // Ensure cleanup goroutine stops
	
	c.Set("k1", 1)
	c.Set("k2", 2)
	// cache is full; next Set must evict one existing
	c.Set("k3", 3)

	// Exactly 2 keys remain
	count := 0
	for _, key := range []string{"k1", "k2", "k3"} {
		if c.Exists(key) {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("Expected 2 items after eviction; got %d", count)
	}
}

// TestOverwrite checks that setting a key twice overwrites the value.
func TestOverwrite(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Second*5), WithMaxItems(10))
	c.Set("dupKey", "first")
	c.Set("dupKey", "second")
	v, found := c.Get("dupKey")
	if !found || v != "second" {
		t.Errorf("Expected overwritten value 'second', got %v", v)
	}
}

// TestDeleteNonExistent ensures deleting a non-existent key is safe and does not panic.
func TestDeleteNonExistent(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Second * 5), WithMaxItems(10))
	c.Delete("ghost") // Should not panic or error
}

// TestClearEmpty ensures clearing an empty cache is safe and does not panic.
func TestClearEmpty(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Second*5), WithMaxItems(10))
	c.Clear() // Should not panic or error
}

// TestConcurrentAccess tests thread safety under concurrent Set/Get/Exists operations.
func TestConcurrentAccess(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Second*5), WithMaxItems(10))
	key := "concurrent"
	value := "safe"
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(i int) {
			c.Set(key, value)
			c.Get(key)
			c.Exists(key)
			done <- true
		}(i)
	}
	for i := 0; i < 100; i++ {
		<-done
	}
}

// TestEvictionEdgeCase tests eviction when all items are expired.
func TestEvictionEdgeCase(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Millisecond * 100), WithMaxItems(2))
	c.Set("a", 1)
	time.Sleep(time.Millisecond * 110)
	c.Set("b", 2)
	c.Set("c", 3) // Should not panic even if 'a' is expired
	if !c.Exists("b") && !c.Exists("c") {
		t.Error("Expected at least one of 'b' or 'c' to exist after eviction with expired items")
	}
}

// TestDefaultConfiguration tests that the cache works with default configuration.
func TestDefaultConfiguration(t *testing.T) {
	c := NewInMemoryCache()
	defer c.(*inMemoryCache).Close()
	
	cache := c.(*inMemoryCache)
	if cache.ttl != time.Minute {
		t.Errorf("Expected default TTL of 1 minute, got %v", cache.ttl)
	}
	if cache.maxItems != 0 {
		t.Errorf("Expected default maxItems of 0 (unlimited), got %d", cache.maxItems)
	}
	
	// Test that it works with defaults
	c.Set("test", "value")
	if val, found := c.Get("test"); !found || val != "value" {
		t.Error("Default cache configuration should work")
	}
}

// TestMultipleOptions tests applying multiple options
func TestMultipleOptions(t *testing.T) {
	c := NewInMemoryCache(
		WithTTL(time.Second * 30),
		WithMaxItems(5),
		WithTTL(time.Second * 60), // This should override the first TTL
	)
	defer c.(*inMemoryCache).Close()
	
	cache := c.(*inMemoryCache)
	if cache.ttl != time.Second * 60 {
		t.Errorf("Expected TTL of 60 seconds (last option should win), got %v", cache.ttl)
	}
	if cache.maxItems != 5 {
		t.Errorf("Expected maxItems of 5, got %d", cache.maxItems)
	}
}

func TestZeroTTL(t *testing.T) {
	c := NewInMemoryCache(WithTTL(0))
	defer c.(*inMemoryCache).Close()
	
	c.Set("immediate", "expire")
	// Items with zero TTL should expire immediately
	if _, found := c.Get("immediate"); found {
		t.Error("Items with zero TTL should expire immediately")
	}
}

func TestNegativeTTL(t *testing.T) {
	c := NewInMemoryCache(WithTTL(-time.Second))
	defer c.(*inMemoryCache).Close()
	
	c.Set("negative", "ttl")
	// Items with negative TTL should expire immediately
	if _, found := c.Get("negative"); found {
		t.Error("Items with negative TTL should expire immediately")
	}
}

func TestUnlimitedCapacity(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Minute), WithMaxItems(0))
	defer c.(*inMemoryCache).Close()
	
	// Add many items (should not evict)
	for i := 0; i < 1000; i++ {
		c.Set(string(rune(i)), i)
	}
	
	// All items should still exist
	count := 0
	for i := 0; i < 1000; i++ {
		if c.Exists(string(rune(i))) {
			count++
		}
	}
	if count != 1000 {
		t.Errorf("Expected 1000 items with unlimited capacity, got %d", count)
	}
}

// TestSingleItemCapacity tests cache with capacity of 1
func TestSingleItemCapacity(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Minute), WithMaxItems(1))
	defer c.(*inMemoryCache).Close()
	
	c.Set("first", 1)
	c.Set("second", 2)
	
	// Only one item should exist
	count := 0
	if c.Exists("first") {
		count++
	}
	if c.Exists("second") {
		count++
	}
	
	if count != 1 {
		t.Errorf("Expected exactly 1 item with capacity 1, got %d", count)
	}
}

// TestLRUEvictionOrder tests that LRU eviction works correctly
func TestLRUEvictionOrder(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Minute), WithMaxItems(3))
	defer c.(*inMemoryCache).Close()
	
	// Add 3 items
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	
	// Access 'a' to make it recently used
	c.Get("a")
	
	// Add another item, should evict 'b' (least recently used)
	c.Set("d", 4)
	
	if !c.Exists("a") || !c.Exists("c") || !c.Exists("d") {
		t.Error("Expected a, c, d to exist after LRU eviction")
	}
	if c.Exists("b") {
		t.Error("Expected b to be evicted (least recently used)")
	}
}

// TestUpdateExistingKeyTiming tests that updating an existing key updates lastUsed
func TestUpdateExistingKeyTiming(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Minute), WithMaxItems(2))
	defer c.(*inMemoryCache).Close()
	
	c.Set("old", 1)
	c.Set("new", 2)
	
	// Update the old key
	c.Set("old", 10)
	
	// Add third item, should evict 'new' since 'old' was just updated
	c.Set("third", 3)
	
	if !c.Exists("old") || !c.Exists("third") {
		t.Error("Expected old and third to exist after updating old key")
	}
	if c.Exists("new") {
		t.Error("Expected new to be evicted after old key was updated")
	}
}

// TestDifferentValueTypes tests storing different types of values
func TestDifferentValueTypes(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Minute))
	defer c.(*inMemoryCache).Close()
	
	// Test various types
	c.Set("string", "hello")
	c.Set("int", 42)
	c.Set("float", 3.14)
	c.Set("bool", true)
	c.Set("slice", []int{1, 2, 3})
	c.Set("map", map[string]int{"key": 123})
	c.Set("nil", nil)
	
	// Verify all types
	if val, found := c.Get("string"); !found || val != "hello" {
		t.Error("String value not stored correctly")
	}
	if val, found := c.Get("int"); !found || val != 42 {
		t.Error("Int value not stored correctly")
	}
	if val, found := c.Get("float"); !found || val != 3.14 {
		t.Error("Float value not stored correctly")
	}
	if val, found := c.Get("bool"); !found || val != true {
		t.Error("Bool value not stored correctly")
	}
	if val, found := c.Get("nil"); !found || val != nil {
		t.Error("Nil value not stored correctly")
	}
}

// TestEmptyStringKey tests using empty string as key
func TestEmptyStringKey(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Minute))
	defer c.(*inMemoryCache).Close()
	
	c.Set("", "empty_key_value")
	if val, found := c.Get(""); !found || val != "empty_key_value" {
		t.Error("Empty string key should be valid")
	}
}

// TestLongKey tests using long keys
func TestLongKey(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Minute))
	defer c.(*inMemoryCache).Close()
	
	longKey := string(make([]byte, 10000))
	c.Set(longKey, "long_key_value")
	if val, found := c.Get(longKey); !found || val != "long_key_value" {
		t.Error("Long key should be handled correctly")
	}
}

// TestConcurrentSetSameKey tests concurrent sets to the same key
func TestConcurrentSetSameKey(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Minute))
	defer c.(*inMemoryCache).Close()
	
	var wg sync.WaitGroup
	key := "race_key"
	
	// Multiple goroutines setting the same key
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			c.Set(key, val)
		}(i)
	}
	
	wg.Wait()
	
	// Should have some value without panic
	if _, found := c.Get(key); !found {
		t.Error("Concurrent sets should not lose the key")
	}
}

// TestConcurrentEviction tests concurrent operations during eviction
func TestConcurrentEviction(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Minute), WithMaxItems(10))
	defer c.(*inMemoryCache).Close()
	
	var wg sync.WaitGroup
	
	// Fill cache to capacity
	for i := 0; i < 10; i++ {
		c.Set(string(rune(i)), i)
	}
	
	// Concurrent operations that should trigger eviction
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			c.Set(string(rune(val + 100)), val)
			c.Get(string(rune(val % 10)))
			c.Exists(string(rune(val % 10)))
		}(i)
	}
	
	wg.Wait()
	
	// Should not panic and should maintain capacity limit
	count := 0
	for i := 0; i < 200; i++ {
		if c.Exists(string(rune(i))) {
			count++
		}
	}
	
	if count > 10 {
		t.Errorf("Expected at most 10 items after concurrent eviction, got %d", count)
	}
}

// TestCleanupGoroutineStops tests that cleanup goroutine stops properly
func TestCleanupGoroutineStops(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()
	
	c := NewInMemoryCache(WithTTL(time.Millisecond))
	
	// Wait a bit for goroutine to start
	time.Sleep(time.Millisecond * 10)
	
	c.(*inMemoryCache).Close()
	
	// Wait for cleanup goroutine to stop
	time.Sleep(time.Millisecond * 50)
	
	finalGoroutines := runtime.NumGoroutine()
	
	if finalGoroutines > initialGoroutines {
		t.Errorf("Cleanup goroutine may not have stopped. Initial: %d, Final: %d", initialGoroutines, finalGoroutines)
	}
}

// TestMultipleClose tests calling Close multiple times
func TestMultipleClose(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Minute))
	cache := c.(*inMemoryCache)
	
	// Should not panic
	cache.Close()
	cache.Close()
	cache.Close()
}

// TestOperationsAfterClose tests that operations work after Close
func TestOperationsAfterClose(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Minute))
	cache := c.(*inMemoryCache)
	
	c.Set("before", "close")
	cache.Close()
	
	// Operations should still work, just cleanup goroutine stops
	c.Set("after", "close")
	if val, found := c.Get("before"); !found || val != "close" {
		t.Error("Should still be able to get values after close")
	}
	if val, found := c.Get("after"); !found || val != "close" {
		t.Error("Should still be able to set/get values after close")
	}
}

// TestCleanupFrequency tests that cleanup runs at appropriate intervals
func TestCleanupFrequency(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Millisecond * 100))
	defer c.(*inMemoryCache).Close()
	
	// Add item that will expire
	c.Set("expire_me", "value")
	
	// Wait for cleanup cycles
	time.Sleep(time.Millisecond * 250)
	
	// Item should be cleaned up by now
	if c.Exists("expire_me") {
		t.Error("Expired item should be cleaned up by background goroutine")
	}
}

// TestExistsWithExpiredItems tests Exists method with expired items
func TestExistsWithExpiredItems(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Millisecond * 50))
	defer c.(*inMemoryCache).Close()
	
	c.Set("short_lived", "value")
	
	// Should exist initially
	if !c.Exists("short_lived") {
		t.Error("Item should exist initially")
	}
	
	// Wait for expiration
	time.Sleep(time.Millisecond * 60)
	
	// Should not exist after expiration
	if c.Exists("short_lived") {
		t.Error("Expired item should not exist")
	}
}

// TestPartialEvictionWithExpiredItems tests eviction when some items are expired
func TestPartialEvictionWithExpiredItems(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Millisecond * 100), WithMaxItems(3))
	defer c.(*inMemoryCache).Close()
	
	// Add items
	c.Set("a", 1)
	c.Set("b", 2)
	
	// Wait for first items to expire
	time.Sleep(time.Millisecond * 110)
	
	// Add new items
	c.Set("c", 3)
	c.Set("d", 4)
	c.Set("e", 5) // This should trigger cleanup of expired items
	
	// Should have room for all new items since old ones expired
	if !c.Exists("c") || !c.Exists("d") || !c.Exists("e") {
		t.Error("All new items should exist after expired items are cleaned up")
	}
}

// TestGetUpdatesLastUsed tests that Get updates the lastUsed timestamp
func TestGetUpdatesLastUsed(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Minute), WithMaxItems(2))
	defer c.(*inMemoryCache).Close()
	
	c.Set("first", 1)
	c.Set("second", 2)
	
	// Get first item to update its lastUsed
	c.Get("first")
	
	// Add third item, should evict second (not first, since first was accessed)
	c.Set("third", 3)
	
	if !c.Exists("first") || !c.Exists("third") {
		t.Error("First and third should exist")
	}
	if c.Exists("second") {
		t.Error("Second should be evicted")
	}
}

// TestHighVolumeOperations tests cache under high load
func TestHighVolumeOperations(t *testing.T) {
	c := NewInMemoryCache(WithTTL(time.Minute), WithMaxItems(1000))
	defer c.(*inMemoryCache).Close()
	
	var wg sync.WaitGroup
	operations := 10000
	
	// High volume mixed operations
	for i := 0; i < operations; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := string(rune(id % 1000))
			c.Set(key, id)
			c.Get(key)
			c.Exists(key)
			if id%10 == 0 {
				c.Delete(key)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Should not panic and should maintain some items
	count := 0
	for i := 0; i < 1000; i++ {
		if c.Exists(string(rune(i))) {
			count++
		}
	}
	
	// Should have some items but not more than capacity
	if count > 1000 {
		t.Errorf("Should not exceed capacity of 1000, got %d", count)
	}
}
