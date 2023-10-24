package person

import (
	"strconv"

	"gofr.dev/examples/using-cassandra/models"
	"gofr.dev/examples/using-cassandra/stores"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

type handler struct {
	store stores.Person
}

// New is factory function for person handler
//
//nolint:revive // handler should not be used without proper initilization with required dependency
func New(ps stores.Person) handler {
	return handler{store: ps}
}

func (h handler) Get(ctx *gofr.Context) (interface{}, error) {
	var filter models.Person

	val := ctx.Params()

	filter.ID, _ = strconv.Atoi(val["id"])
	filter.Name = val["name"]
	filter.Age, _ = strconv.Atoi(val["age"])
	filter.State = val["state"]

	return h.store.Get(ctx, filter), nil
}

func (h handler) Create(ctx *gofr.Context) (interface{}, error) {
	var data models.Person
	// json error
	if err := ctx.Bind(&data); err != nil {
		return nil, err
	}

	records := h.store.Get(ctx, models.Person{ID: data.ID})

	if len(records) > 0 {
		return nil, errors.EntityAlreadyExists{}
	}

	results, err := h.store.Create(ctx, data)

	return results, err
}

func (h handler) Delete(ctx *gofr.Context) (interface{}, error) {
	var filter models.Person

	filter.ID, _ = strconv.Atoi(ctx.PathParam("id"))

	id := ctx.PathParam("id")
	// first verify that value exit or not?
	records := h.store.Get(ctx, filter)

	if len(records) == 0 {
		return nil, errors.EntityNotFound{Entity: "person", ID: id}
	}

	err := h.store.Delete(ctx, ctx.PathParam("id"))

	return nil, err
}

func (h handler) Update(ctx *gofr.Context) (interface{}, error) {
	var data models.Person

	if err := ctx.Bind(&data); err != nil {
		return nil, err
	}

	data.ID, _ = strconv.Atoi(ctx.PathParam("id"))
	records := h.store.Get(ctx, models.Person{ID: data.ID})

	if len(records) == 0 {
		return nil, errors.EntityNotFound{Entity: "person", ID: ctx.PathParam("id")}
	}

	return h.store.Update(ctx, data)
}
