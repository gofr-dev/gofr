package cache

import (
	"context"
	"time"
)

type Provider interface {
	Get(ctx context.Context, key string) (string, error)

	Set(ctx context.Context, key, val string) error
	SetWithTTL(ctx context.Context, key, val string, ttl time.Duration) error

	Delete(ctx context.Context, key string) error
}
