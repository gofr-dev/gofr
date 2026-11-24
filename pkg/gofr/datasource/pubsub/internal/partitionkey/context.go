// Package partitionkey carries explicit partition keys without inspecting payloads.
package partitionkey

import "context"

type contextKey int

const partitionKeyContextKey contextKey = 0

// WithKey attaches an immutable key to a derived context.
func WithKey(ctx context.Context, key []byte) context.Context {
	return context.WithValue(ctx, partitionKeyContextKey, string(key))
}

// FromContext returns the explicit message key, or nil for unkeyed publication.
func FromContext(ctx context.Context) []byte {
	key, _ := ctx.Value(partitionKeyContextKey).(string)
	if key == "" {
		return nil
	}

	return []byte(key)
}
