package store

import (
	"time"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

// Store is an abstraction for the core layer
type Store interface {
	Get(ctx *gofr.Context, key string) (string, error)
	Set(ctx *gofr.Context, key, value string, expiration time.Duration) error
	Delete(ctx *gofr.Context, key string) error
}

// model is the type on which all the core layer's functionality is implemented on
type model struct{}

// New is factory function for store
//
//nolint:revive // model should not be used without proper initilization with required dependency
func New() model {
	return model{}
}

// Get returns the value for a given key, throws an error, if something goes wrong
func (m model) Get(ctx *gofr.Context, key string) (string, error) {
	// fetch the Redis client
	rc := ctx.Redis

	value, err := rc.Get(ctx.Context, key).Result()
	if err != nil {
		return "", errors.DB{Err: err}
	}

	return value, nil
}

// Set accepts a key-value pair, and sets those in Redis, if expiration is non-zero value, it sets a expiration(TTL)
// on those keys, if expiration is 0, then the keys have no expiration time
func (m model) Set(ctx *gofr.Context, key, value string, expiration time.Duration) error {
	// fetch the Redis client
	rc := ctx.Redis

	if err := rc.Set(ctx.Context, key, value, expiration).Err(); err != nil {
		return errors.DB{Err: err}
	}

	return nil
}

// Delete deletes a key from Redis, returns the error if it fails to delete
func (m model) Delete(ctx *gofr.Context, key string) error {
	// fetch the Redis client
	rc := ctx.Redis
	return rc.Del(ctx.Context, key).Err()
}
