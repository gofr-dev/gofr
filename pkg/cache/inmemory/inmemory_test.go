package inmemory

import (
	"testing"
	"time"
)

func Test_in_memory_cache(t *testing.T) {
	cache := NewInMemoryCache(time.Minute * 5)

	key := "GOFR"
	value := "VALUE"
	otherKey := "GOFR2"

	t.Run("Set and Get", func(t *testing.T) {
		cache.Set(key, value)
		got, found := cache.Get(key)
		if !found {
			t.Fatalf("Expected to find value for key %s, got false", key)
		}
		if got != value {
			t.Fatalf("Expected value %s, got %v", value, got)
		}
	})

	t.Run("Exists", func(t *testing.T) {
		if !cache.Exists(key) {
			t.Errorf("Expected key %s to exist", key)
		}
		if cache.Exists(otherKey) {
			t.Errorf("Did not expect key %s to exist", otherKey)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		cache.Delete(key)
		if cache.Exists(key) {
			t.Errorf("Expected key %s to be deleted", key)
		}
		_, found := cache.Get(key)
		if found {
			t.Errorf("Expected not to find value for deleted key %s", key)
		}
		// Deleting a non-existent key should not panic or error
		cache.Delete("nonexistent")
	})

	t.Run("Clear", func(t *testing.T) {
		cache.Set("a", 1)
		cache.Set("b", 2)
		cache.Clear()
		if cache.Exists("a") || cache.Exists("b") {
			t.Errorf("Expected cache to be empty after Clear")
		}
		// Clear on already empty cache should not panic
		cache.Clear()
	})
}
