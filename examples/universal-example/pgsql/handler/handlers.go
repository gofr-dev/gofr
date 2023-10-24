package handler

import (
	"gofr.dev/examples/universal-example/pgsql/entity"
	"gofr.dev/examples/universal-example/pgsql/store"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

type employee struct {
	store store.Store
}

//nolint:revive //employee should not get accessed, without initialization of store.Employee
func New(s store.Store) employee {
	return employee{
		store: s,
	}
}

func (e employee) Get(c *gofr.Context) (interface{}, error) {
	return e.store.Get(c)
}

func (e employee) Create(c *gofr.Context) (interface{}, error) {
	var emp entity.Employee
	if err := c.Bind(&emp); err != nil {
		return nil, errors.InvalidParam{Param: []string{"body"}}
	}

	err := e.store.Create(c, emp)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
