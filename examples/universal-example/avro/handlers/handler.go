package handlers

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/types"
)

type Employee struct {
	ID    string `avro:"Id"`
	Name  string `avro:"Name"`
	Phone string `avro:"Phone"`
	Email string `avro:"Email"`
	City  string `avro:"City"`
}

func Producer(c *gofr.Context) (interface{}, error) {
	id := c.Param("id")

	return nil, c.PublishEvent("", Employee{
		ID:    id,
		Name:  "Rohan",
		Phone: "01777",
		Email: "rohan@email.xyz",
		City:  "Berlin",
	}, nil)
}

func Consumer(c *gofr.Context) (interface{}, error) {
	emp := Employee{}
	message, err := c.Subscribe(&emp)

	return types.Response{Data: emp, Meta: message}, err
}
