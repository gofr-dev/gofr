package cache

import (
	"context"
	"github.com/redis/go-redis/v9"
	"time"
)

func NewRedisProvider(client *redis.Client) Provider {
	return &redisProvider{client: client}
}

type redisProvider struct {
	client *redis.Client
}

func (r redisProvider) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

func (r redisProvider) Set(ctx context.Context, key, val string) error {
	return r.client.Set(ctx, key, val, 0).Err()
}

func (r redisProvider) SetWithTTL(ctx context.Context, key, val string, ttl time.Duration) error {
	return r.client.Set(ctx, key, val, ttl).Err()
}

func (r redisProvider) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}
