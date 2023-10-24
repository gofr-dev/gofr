// Package cache implements a basic redis cache which provides functionality to interact with redis
package cache

import (
	"context"
	"time"

	"gofr.dev/pkg/datastore"
)

type RedisCacher struct {
	redis datastore.Redis
}

// NewRedisCacher is a factory function that creates and returns an instance of RedisCacher.
func NewRedisCacher(redis datastore.Redis) RedisCacher {
	return RedisCacher{redis: redis}
}

// Get retrieves the cached content associated with the given key from Redis.
func (r RedisCacher) Get(key string) ([]byte, error) {
	resp, err := r.redis.Get(context.Background(), key).Bytes()
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// Set stores the provided content in Redis cache with the given key and duration.
func (r RedisCacher) Set(key string, content []byte, duration time.Duration) error {
	return r.redis.Set(context.Background(), key, content, duration).Err()
}

// Delete removes the cached content associated with the given key from Redis.
func (r RedisCacher) Delete(key string) error {
	return r.redis.Del(context.Background(), key).Err()
}
