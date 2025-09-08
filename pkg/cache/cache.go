package cache

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

type Cache interface {
	// Get retrieves the value associated with the given key.
	// It returns the value, a boolean indicating if the key was found, and an error if any occurred.
	// If the key is not found, it returns nil, false, nil.
	Get(ctx context.Context, key string) (any, bool, error)

	// Set stores a key-value pair in the cache.
	// If the key already exists, its value is overwritten.
	// It may also set a time-to-live (TTL) for the entry, depending on the implementation.
	Set(ctx context.Context, key string, value any) error

	// Delete removes the key-value pair associated with the given key from the cache.
	// If the key does not exist, it does nothing and returns nil.
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in the cache and has not expired.
	// It returns true if the key exists, and false otherwise.
	Exists(ctx context.Context, key string) (bool, error)

	// Clear removes all key-value pairs from the cache.
	// This operation is destructive and should be used with caution.
	Clear(ctx context.Context) error

	// Close releases any resources used by the cache, such as background goroutines or network connections.
	// After Close is called, the cache may no longer be usable.
	Close(ctx context.Context) error

	UseTracer(tracer trace.Tracer)
}
