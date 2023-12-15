package handler

import (
	"github.com/google/uuid"

	"gofr.dev/examples/using-clickhouse/models"
	"gofr.dev/examples/using-clickhouse/store"
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
	Users []models.User
}

func (h handler) Get(ctx *gofr.Context) (interface{}, error) {
	resp, err := h.store.Get(ctx)
	if err != nil {
		return nil, err
	}

	return response{Users: resp}, nil
}

func (h handler) GetByID(ctx *gofr.Context) (interface{}, error) {
	id := ctx.PathParam("id")

	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, errors.InvalidParam{Param: []string{"id"}}
	}

	resp, err := h.store.GetByID(ctx, uid)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (h handler) Create(ctx *gofr.Context) (interface{}, error) {
	var user models.User
	if err := ctx.Bind(&user); err != nil {
		return nil, errors.InvalidParam{Param: []string{"body"}}
	}

	resp, err := h.store.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
