package handlers

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/types"
)

type Person struct {
	ID    string `avro:"Id"`
	Name  string `avro:"Name"`
	Email string `avro:"Email"`
}

func Producer(ctx *gofr.Context) (interface{}, error) {
	id := ctx.Param("id")

	return nil, ctx.PublishEvent("", Person{
		ID:    id,
		Name:  "Rohan",
		Email: "rohan@email.xyz",
	}, map[string]string{"test": "test"})
}

func Consumer(ctx *gofr.Context) (interface{}, error) {
	p := map[string]interface{}{}
	message, err := ctx.Subscribe(&p)

	return types.Response{Data: p, Meta: message}, err
}
