package cache

import (
	"context"
	"time"
)

func NewMemoryProvider() Provider {
	return &memoryProvider{storage: make(map[string]string)}
}

type memoryProvider struct {
	storage map[string]string
	ttl     time.Time
}

func (m memoryProvider) Get(ctx context.Context, key string) (string, error) {
	return m.storage[key], nil
}

func (m memoryProvider) Set(ctx context.Context, key, val string) error {
	m.storage[key] = val

	return nil
}

func (m memoryProvider) SetWithTTL(ctx context.Context, key, val string, ttl time.Duration) error {
	m.storage[key] = val

	return nil
}

func (m memoryProvider) Delete(ctx context.Context, key string) error {
	delete(m.storage, key)

	return nil
}
