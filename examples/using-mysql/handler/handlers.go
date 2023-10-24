package handler

import (
	"gofr.dev/examples/using-mysql/models"
	"gofr.dev/examples/using-mysql/store"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

type handler struct {
	store store.Store
}

// New is factory function for Handler layer
//
//nolint:revive // handler should not be used without proper initilization with required dependency
func New(s store.Store) handler {
	return handler{store: s}
}

type response struct {
	Employees []models.Employee
}

func (h handler) Get(ctx *gofr.Context) (interface{}, error) {
	resp, err := h.store.Get(ctx)
	if err != nil {
		return nil, err
	}

	return response{Employees: resp}, nil
}

func (h handler) Create(ctx *gofr.Context) (interface{}, error) {
	var emp models.Employee
	if err := ctx.Bind(&emp); err != nil {
		ctx.Logger.Errorf("error in binding: %v", err)
		return nil, errors.InvalidParam{Param: []string{"body"}}
	}

	resp, err := h.store.Create(ctx, emp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
