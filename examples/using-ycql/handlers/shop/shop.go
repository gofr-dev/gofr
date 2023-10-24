package shop

import (
	"strconv"

	"gofr.dev/examples/using-ycql/models"
	"gofr.dev/examples/using-ycql/stores"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

type handler struct {
	store stores.Shop
}

// New is Factory function for handlers layer
//
//nolint:revive // handler should not be used without proper initilization with required dependency
func New(s stores.Shop) handler {
	return handler{store: s}
}
func (h handler) Get(ctx *gofr.Context) (interface{}, error) {
	var filter models.Shop

	params := ctx.Params()

	filter.ID, _ = strconv.Atoi(params["id"])
	filter.Name = params["name"]
	filter.Location = params["location"]
	filter.State = params["state"]

	return h.store.Get(ctx, filter), nil
}

func (h handler) Create(ctx *gofr.Context) (interface{}, error) {
	var data models.Shop
	// json error
	if err := ctx.Bind(&data); err != nil {
		return nil, err
	}

	records := h.store.Get(ctx, models.Shop{ID: data.ID})

	if len(records) > 0 {
		return nil, errors.EntityAlreadyExists{}
	}

	return h.store.Create(ctx, data)
}

func (h handler) Delete(ctx *gofr.Context) (interface{}, error) {
	var filter models.Shop

	filter.ID, _ = strconv.Atoi(ctx.PathParam("id"))

	id := ctx.PathParam("id")
	// first verify that value exit or not?
	records := h.store.Get(ctx, filter)

	if len(records) == 0 {
		return nil, errors.EntityNotFound{Entity: "shop", ID: id}
	}

	return nil, h.store.Delete(ctx, ctx.PathParam("id"))
}

func (h handler) Update(ctx *gofr.Context) (interface{}, error) {
	var data models.Shop

	if err := ctx.Bind(&data); err != nil {
		return nil, err
	}

	data.ID, _ = strconv.Atoi(ctx.PathParam("id"))
	records := h.store.Get(ctx, models.Shop{ID: data.ID})

	if len(records) == 0 {
		return nil, errors.EntityNotFound{Entity: "shop", ID: ctx.PathParam("id")}
	}

	return h.store.Update(ctx, data)
}
