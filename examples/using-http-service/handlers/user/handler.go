package user

import (
	"strings"

	"gofr.dev/examples/using-http-service/services"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

type handler struct {
	service services.User
}

// New is factory function for handler layer
//
//nolint:revive // handler should not be used without proper initialization of the required dependency
func New(service services.User) handler {
	return handler{service: service}
}

// Get retrieves the company details for the given name passed as a path param
func (h handler) Get(ctx *gofr.Context) (interface{}, error) {
	name := ctx.PathParam("name")
	if strings.TrimSpace(name) == "" {
		return nil, errors.MissingParam{Param: []string{"name"}}
	}

	resp, err := h.service.Get(ctx, name)
	if err != nil {
		return nil, err // avoiding partial content response
	}

	return resp, nil
}
