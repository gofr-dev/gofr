package http

import (
	"gofr.dev/examples/sample-grpc/handler/grpc"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

func GetUserDetails(ctx *gofr.Context) (interface{}, error) {
	if ctx.Param("id") == "1" {
		resp := grpc.Response{
			FirstName:  "Henry",
			SecondName: "Marc",
		}

		return &resp, nil
	}

	return nil, errors.EntityNotFound{Entity: "name", ID: "2"}
}
