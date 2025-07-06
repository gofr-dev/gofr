package cache

import "context"

type Cache interface {
	// Get returns the value associated with the key, a boolean indicating existence, and an error if any.
	Get(ctx context.Context, key string) (interface{}, bool, error)

	// Set stores the value for the key or returns an error.
	Set(ctx context.Context, key string, value interface{}) error

	// Delete removes the key or returns an error.
	Delete(ctx context.Context, key string) error

	// Exists reports whether the key exists and hasn’t expired, or returns an error.
	Exists(ctx context.Context, key string) (bool, error)

	// Clear removes all items or returns an error.
	Clear(ctx context.Context,) error

	// Close stops background processes and marks the cache closed; returns error if already closed or on failure.
	Close(ctx context.Context,) error
}
