package cache

type Cache interface {
	// Get returns the value associated with the key, a boolean indicating existence, and an error if any.
	Get(key string) (interface{}, bool, error)

	// Set stores the value for the key or returns an error.
	Set(key string, value interface{}) error

	// Delete removes the key or returns an error.
	Delete(key string) error

	// Exists reports whether the key exists and hasn’t expired, or returns an error.
	Exists(key string) (bool, error)

	// Clear removes all items or returns an error.
	Clear() error

	// Close stops background processes and marks the cache closed; returns error if already closed or on failure.
	Close() error
}
