package handler

import (
	"gofr.dev/examples/using-solr/store"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

type handler struct {
	store store.Customer
}

// New initializes the consumer layer
//
//nolint:revive // handler should not be used without proper initilization with required dependency
func New(s store.Customer) handler {
	return handler{store: s}
}

// List lists the customers based on the parameters sent in the query
func (h handler) List(ctx *gofr.Context) (interface{}, error) {
	id := ctx.Param("id")
	if id == "" {
		return nil, errors.MissingParam{Param: []string{"id"}}
	}

	filter := store.Filter{ID: id, Name: ctx.Param("name")}

	resp, err := h.store.List(ctx, "customer", filter)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// Create creates a document in the customer collection
func (h handler) Create(ctx *gofr.Context) (interface{}, error) {
	var model store.Model

	err := ctx.Bind(&model)
	if err != nil {
		return nil, errors.InvalidParam{Param: []string{"body"}}
	}

	if model.Name == "" {
		return nil, errors.InvalidParam{Param: []string{"name"}}
	}

	return nil, h.store.Create(ctx, "customer", model)
}

// Update updates a document in the customer collection
func (h handler) Update(ctx *gofr.Context) (interface{}, error) {
	var model store.Model

	err := ctx.Bind(&model)
	if err != nil {
		return nil, errors.InvalidParam{Param: []string{"body"}}
	}

	if model.Name == "" {
		return nil, errors.InvalidParam{Param: []string{"name"}}
	}

	if model.ID == 0 {
		return nil, errors.InvalidParam{Param: []string{"id"}}
	}

	return nil, h.store.Update(ctx, "customer", model)
}

// Delete deletes a document in the customer collection
func (h handler) Delete(ctx *gofr.Context) (interface{}, error) {
	var model store.Model

	err := ctx.Bind(&model)
	if err != nil {
		return nil, errors.InvalidParam{Param: []string{"body"}}
	}

	if model.ID == 0 {
		return nil, errors.InvalidParam{Param: []string{"id"}}
	}

	return nil, h.store.Delete(ctx, "customer", model)
}
