package handler

import (
	"fmt"
	"net/http"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/types"
)

type person struct {
	Username string
	Password string
}

// HelloWorld is a handler function of type gofr.Handler, it responds with a message
func HelloWorld(_ *gofr.Context) (interface{}, error) {
	return "Hello World!", nil
}

func HelloName(ctx *gofr.Context) (interface{}, error) {
	name := ctx.Param("name")

	return types.Response{
		Data: fmt.Sprintf("Hello %s", name),
		Meta: map[string]interface{}{"page": 1, "offset": 0},
	}, nil
}

func PostName(ctx *gofr.Context) (interface{}, error) {
	var p person

	err := ctx.Bind(&p)
	if err != nil {
		return nil, err
	}

	if p.Username == "alreadyExist" {
		return p, errors.EntityAlreadyExists{}
	}

	return p, nil
}

func ErrorHandler(_ *gofr.Context) (interface{}, error) {
	return nil, &errors.Response{StatusCode: http.StatusNotFound}
}

// MultipleErrorHandler returns multiple errors and
// also sets the statusCode to 400 if id is 1 else to 500
func MultipleErrorHandler(ctx *gofr.Context) (interface{}, error) {
	id := ctx.Param("id")

	var statusCode int

	if id == "1" {
		statusCode = http.StatusBadRequest
	}

	return nil, errors.MultipleErrors{
		StatusCode: statusCode,
		Errors: []error{
			errors.InvalidParam{Param: []string{"EmailAddress"}},
			errors.MissingParam{Param: []string{"Address"}},
		}}
}
