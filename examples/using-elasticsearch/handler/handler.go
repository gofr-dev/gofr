package handler

import (
	"net/http"

	"gofr.dev/examples/using-elasticsearch/model"
	"gofr.dev/examples/using-elasticsearch/store"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

type handler struct {
	store store.Customer
}

// New is factory function for customer handler
//
//nolint:revive // handler should not be used without proper initilization with required dependency
func New(c store.Customer) handler {
	return handler{store: c}
}

func (h handler) Index(ctx *gofr.Context) (interface{}, error) {
	name := ctx.Param("name")

	resp, err := h.store.Get(ctx, name)
	if err != nil {
		return nil, &errors.Response{StatusCode: http.StatusInternalServerError, Reason: "something unexpected happened"}
	}

	return resp, nil
}

func (h handler) Read(ctx *gofr.Context) (interface{}, error) {
	id := ctx.PathParam("id")

	if id == "" {
		return nil, errors.MissingParam{Param: []string{"id"}}
	}

	resp, err := h.store.GetByID(ctx, id)
	if err != nil {
		return nil, &errors.Response{StatusCode: http.StatusInternalServerError, Reason: "something unexpected happened"}
	}

	return resp, nil
}

func (h handler) Update(ctx *gofr.Context) (interface{}, error) {
	id := ctx.PathParam("id")
	if id == "" {
		return nil, errors.MissingParam{Param: []string{"id"}}
	}

	var c model.Customer

	if err := ctx.Bind(&c); err != nil {
		return nil, errors.InvalidParam{Param: []string{"body"}}
	}

	resp, err := h.store.Update(ctx, c, id)
	if err != nil {
		return nil, &errors.Response{StatusCode: http.StatusInternalServerError, Reason: "something unexpected happened"}
	}

	return resp, nil
}

func (h handler) Create(ctx *gofr.Context) (interface{}, error) {
	var c model.Customer
	if err := ctx.Bind(&c); err != nil {
		return nil, errors.InvalidParam{Param: []string{"body"}}
	}

	resp, err := h.store.Create(ctx, c)
	if err != nil {
		return nil, &errors.Response{StatusCode: http.StatusInternalServerError, Reason: "something unexpected happened"}
	}

	return resp, nil
}
func (h handler) Delete(ctx *gofr.Context) (interface{}, error) {
	id := ctx.PathParam("id")
	if id == "" {
		return nil, errors.MissingParam{Param: []string{"id"}}
	}

	if err := h.store.Delete(ctx, id); err != nil {
		return nil, &errors.Response{StatusCode: http.StatusInternalServerError, Reason: "something unexpected happened"}
	}

	return "Deleted successfully", nil
}
