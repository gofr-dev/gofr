package handler

import (
	"gofr.dev/examples/universal-example/redis/store"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

type config struct {
	store store.Store
}

//nolint:revive //config should not get accessed, without initialization of store.Store
func New(c store.Store) config {
	return config{
		store: c,
	}
}

// GetKey is handler of type gofr.Handler, it sets key.
func (m config) GetKey(c *gofr.Context) (interface{}, error) {
	// fetch the path parameter specified in the route
	key := c.PathParam("key")
	if key == "" {
		return nil, errors.MissingParam{Param: []string{"key"}}
	}

	value, err := m.store.Get(c, key)
	if err != nil {
		return nil, err
	}

	resp := make(map[string]string)
	resp[key] = value

	return resp, nil
}

func (m config) SetKey(c *gofr.Context) (interface{}, error) {
	input := make(map[string]string)
	if err := c.Bind(&input); err != nil {
		return nil, invalidBodyErr
	}

	for key, value := range input {
		if err := m.store.Set(c, key, value, 0); err != nil {
			c.Logger.Errorf("Got error:", err)
			return nil, invalidInputErr
		}
	}

	return "Successful", nil
}

const (
	invalidBodyErr  = constError("error: invalid body")
	invalidInputErr = constError("error: invalid input")
)

type constError string

func (err constError) Error() string {
	return string(err)
}
