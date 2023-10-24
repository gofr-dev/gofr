package handlers

import (
	"time"

	"gofr.dev/pkg/datastore/pubsub"
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
	start := time.Now()

	err := ctx.PublishEvent("", Person{
		ID:    id,
		Name:  "Rohan",
		Email: "rohan@email.xyz",
	}, map[string]string{"test": "test"})
	if err != nil {
		return nil, err
	}

	err = ctx.Metric.ObserveHistogram(PublishEventHistogram, time.Since(start).Seconds())

	return nil, err
}

func Consumer(ctx *gofr.Context) (interface{}, error) {
	p := Person{}
	start := time.Now()

	message, err := ctx.Subscribe(&p)
	if err != nil {
		return nil, err
	}

	err = ctx.Metric.ObserveSummary(ConsumeEventSummary, time.Since(start).Seconds())

	return types.Response{Data: p, Meta: message}, err
}

func ConsumerWithCommit(ctx *gofr.Context) (interface{}, error) {
	p := Person{}

	count := 0
	message, err := ctx.SubscribeWithCommit(func(message *pubsub.Message) (bool, bool) {
		count++
		ctx.Logger.Infof("Consumed %v message(s), offset: %v, topic: %v", count, message.Offset, message.Topic)

		for count <= 2 {
			return true, true
		}

		for count <= 5 {
			return false, true
		}

		return false, false
	})

	if err != nil {
		return nil, err
	}

	return types.Response{Data: p, Meta: message}, nil
}
