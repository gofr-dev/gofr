package handler

import (
	"strconv"

	"gofr.dev/examples/using-redis/store"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

type config struct {
	store store.Store
}

// New is factory function for config
//
//nolint:revive // handler should not be used without proper initilization with required dependency
func New(s store.Store) config {
	return config{
		store: s,
	}
}

//nolint:gocognit // SetKey is a handler function of type gofr.Handler, it sets keys
func (c config) SetKey(ctx *gofr.Context) (interface{}, error) {
	const size = 64

	input := make(map[string]string)

	length, err := strconv.ParseFloat(ctx.Header("Content-Length"), size)
	if err != nil {
		length = 0
	}

	err = ctx.Metric.SetGauge(ReqContentLengthGauge, length)
	if err != nil {
		return nil, err
	}

	if err = ctx.Bind(&input); err != nil {
		err = ctx.Metric.IncCounter(InvalidBodyCounter)
		if err != nil {
			return nil, err
		}

		err = ctx.Metric.IncCounter(NumberOfSetsCounter, "failed")
		if err != nil {
			return nil, err
		}

		return nil, invalidBodyErr{}
	}

	for key, value := range input {
		if err = c.store.Set(ctx, key, value, 0); err != nil {
			ctx.Logger.Error("got error: ", err)

			err = ctx.Metric.IncCounter(NumberOfSetsCounter, "failed")
			if err != nil {
				return nil, err
			}

			return nil, invalidInputErr{}
		}
	}

	err = ctx.Metric.IncCounter(NumberOfSetsCounter, "succeeded")
	if err != nil {
		return nil, err
	}

	return "Successful", nil
}

// GetKey is a handler function of type gofr.Handler, it fetches keys
func (c config) GetKey(ctx *gofr.Context) (interface{}, error) {
	// fetch the path parameter as specified in the route
	key := ctx.PathParam("key")
	if key == "" {
		return nil, errors.MissingParam{Param: []string{"key"}}
	}

	value, err := c.store.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	resp := make(map[string]string)
	resp[key] = value

	return resp, nil
}

// DeleteKey is a handler function of type gofr.Handler, it deletes keys
func (c config) DeleteKey(ctx *gofr.Context) (interface{}, error) {
	// fetch the path parameter as specified in the route
	key := ctx.PathParam("key")
	if key == "" {
		return nil, errors.MissingParam{Param: []string{"key"}}
	}

	if err := c.store.Delete(ctx, key); err != nil {
		ctx.Logger.Errorf("err: ", err)
		return nil, deleteErr{}
	}

	return "Deleted successfully", nil
}

type (
	deleteErr       struct{}
	invalidInputErr struct{}
	invalidBodyErr  struct{}
)

func (d deleteErr) Error() string {
	return "error: failed to delete"
}

func (i invalidInputErr) Error() string {
	return "error: invalid input"
}

func (i invalidBodyErr) Error() string {
	return "error: invalid body"
}
