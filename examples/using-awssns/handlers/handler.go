package handlers

import (
	"gofr.dev/examples/using-awssns/entity"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/types"
)

const (
	version = 1.1
)

func Publisher(c *gofr.Context) (interface{}, error) {
	var message *entity.Message

	err := c.Bind(&message)
	if err != nil {
		return nil, errors.EntityNotFound{}
	}

	attr := map[string]interface{}{
		"email":   "test@abc.com",
		"version": version,
		"key":     []interface{}{1, 1.999, "value"},
	}

	return nil, c.Notifier.Publish(message, attr)
}

func Subscriber(c *gofr.Context) (interface{}, error) {
	data := map[string]interface{}{}
	msg, err := c.Notifier.SubscribeWithResponse(&data)

	if err != nil {
		return nil, err
	}

	return types.Response{Data: data, Meta: msg}, nil
}
