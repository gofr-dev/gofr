package gofr

import (
	"context"

	"gofr.dev/pkg/gofr/container"
)

type SubscribeFunc func(c *Context) error

type SubscriptionManager struct {
	container     *container.Container
	subscriptions map[string]SubscribeFunc
}

func newSubscriptionManager(c *container.Container) SubscriptionManager {
	return SubscriptionManager{
		container:     c,
		subscriptions: make(map[string]SubscribeFunc),
	}
}

func (s *SubscriptionManager) startSubscriber(topic string, handler SubscribeFunc) {
	// continuously subscribe in an infinite loop
	for {
		msg, err := s.container.GetSubscriber().Subscribe(context.Background(), topic)
		if msg == nil {
			continue
		}

		if err != nil {
			s.container.Logger.Errorf("error while reading from Kafka, err: %v", err.Error())
			continue
		}

		ctx := newContext(nil, msg, s.container)
		err = handler(ctx)

		// commit the message if the subscription function does not return error
		if err == nil {
			msg.Commit()
		} else {
			s.container.Logger.Errorf("error in handler for topic %s: %v", topic, err)
		}
	}
}
