package handler

import (
	"github.com/google/uuid"

	"gofr.dev/examples/using-postgres/model"
	"gofr.dev/examples/using-postgres/store"
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
	Customers []model.Customer
}

func (h handler) Get(ctx *gofr.Context) (interface{}, error) {
	resp, err := h.store.Get(ctx)
	if err != nil {
		return nil, err
	}

	r := response{Customers: resp}

	return r, nil
}

func (h handler) GetByID(ctx *gofr.Context) (interface{}, error) {
	i := ctx.PathParam("id")
	if i == "" {
		return nil, errors.MissingParam{Param: []string{"id"}}
	}

	id, err := uuid.Parse(i)
	if err != nil {
		ctx.Logger.Errorf("error in parsing %v", i)
		return nil, errors.InvalidParam{Param: []string{"id"}}
	}

	resp, err := h.store.GetByID(ctx, id)
	if err != nil {
		return nil, errors.EntityNotFound{
			Entity: "customer",
			ID:     i,
		}
	}

	return resp, nil
}

func (h handler) Create(ctx *gofr.Context) (interface{}, error) {
	var cust model.Customer
	if err := ctx.Bind(&cust); err != nil {
		ctx.Logger.Errorf("error in binding: %v", err)
		return nil, errors.InvalidParam{Param: []string{"body"}}
	}

	resp, err := h.store.Create(ctx, cust)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (h handler) Update(ctx *gofr.Context) (interface{}, error) {
	i := ctx.PathParam("id")
	if i == "" {
		return nil, errors.MissingParam{Param: []string{"id"}}
	}

	id, err := uuid.Parse(i)
	if err != nil {
		return nil, errors.InvalidParam{Param: []string{"id"}}
	}

	var cust model.Customer
	if err = ctx.Bind(&cust); err != nil {
		ctx.Logger.Errorf("error in binding: %v", err)
		return nil, errors.InvalidParam{Param: []string{"body"}}
	}

	cust.ID = id

	resp, err := h.store.Update(ctx, cust)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (h handler) Delete(ctx *gofr.Context) (interface{}, error) {
	i := ctx.PathParam("id")
	if i == "" {
		return nil, errors.MissingParam{Param: []string{"id"}}
	}

	id, err := uuid.Parse(i)
	if err != nil {
		ctx.Logger.Errorf("error in parsing %v", i)
		return nil, errors.InvalidParam{Param: []string{"id"}}
	}

	if err := h.store.Delete(ctx, id); err != nil {
		return nil, err
	}

	return "Deleted successfully", nil
}
