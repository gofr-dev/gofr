package store

import (
	"time"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

// Store is the abstraction of core layer
type Store interface {
	Get(ctx *gofr.Context, key string) (string, error)
	Set(ctx *gofr.Context, key, value string, expiration time.Duration) error
}

// Model is the type on which all the core layer's functionality is implemented
type Model struct{}

// New returns Model core
func New() Model {
	return Model{}
}

// Get returns the value for a given key, throws an error, if something goes wrong.
func (m Model) Get(c *gofr.Context, key string) (string, error) {
	value, err := c.Redis.Get(c.Context, key).Result()
	if err != nil {
		return "", errors.DB{Err: err}
	}

	return value, nil
}

// Set accept key-value pair, and sets those in Redis, if expiration is non-zero value, it set an expiration(TTL)
// on those keys, if expiration is zero, then keys have no expiration time.
func (m Model) Set(c *gofr.Context, key, value string, expiration time.Duration) error {
	err := c.Redis.Set(c.Context, key, value, expiration).Err()
	if err != nil {
		return errors.DB{Err: err}
	}

	return nil
}
