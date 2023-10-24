package handler

import (
	"gofr.dev/pkg/datastore/pubsub"
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
type handler struct {
	eventHub pubsub.PublisherSubscriber
}

//nolint:revive //handler should not get accessed, without initialization of pubsub.PublisherSubscriber
func New(eve pubsub.PublisherSubscriber) handler {
	return handler{eventHub: eve}
}

func (h handler) Producer(c *gofr.Context) (interface{}, error) {
	id := c.Param("id")

	return nil, h.eventHub.PublishEvent("", Employee{
		ID:    id,
		Name:  "Rohan",
		Phone: "01777",
		Email: "rohan@email.xyz",
		City:  "Berlin",
	}, map[string]string{"headerKey": "headerValue"})
}

func (h handler) Consumer(*gofr.Context) (interface{}, error) {
	emp := map[string]interface{}{}

	msg, err := h.eventHub.Subscribe()
	if err != nil {
		return msg, err
	}

	err = h.eventHub.Bind([]byte(msg.Value), &emp)

	return types.Response{Data: emp, Meta: msg}, err
}
