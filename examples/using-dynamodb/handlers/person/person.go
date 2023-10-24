package person

import (
	"gofr.dev/examples/using-dynamodb/models"
	"gofr.dev/examples/using-dynamodb/stores"
	"gofr.dev/pkg/gofr"
)

type handler struct {
	store stores.Person
}

// New factory function for handler layer
//
//nolint:revive // handler should not be used without proper initilization with required dependency
func New(s stores.Person) handler {
	return handler{store: s}
}

func (h handler) Create(ctx *gofr.Context) (interface{}, error) {
	var p models.Person

	err := ctx.Bind(&p)
	if err != nil {
		return nil, err
	}

	err = h.store.Create(ctx, p)
	if err != nil {
		return nil, err
	}

	return "Successful", nil
}

func (h handler) GetByID(ctx *gofr.Context) (interface{}, error) {
	id := ctx.PathParam("id")

	person, err := h.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	return person, nil
}

func (h handler) Update(ctx *gofr.Context) (interface{}, error) {
	id := ctx.PathParam("id")

	var p models.Person

	err := ctx.Bind(&p)
	if err != nil {
		return nil, err
	}

	p.ID = id

	err = h.store.Update(ctx, p)
	if err != nil {
		return nil, err
	}

	return "Successful", nil
}

func (h handler) Delete(ctx *gofr.Context) (interface{}, error) {
	id := ctx.PathParam("id")

	return nil, h.store.Delete(ctx, id)
}
