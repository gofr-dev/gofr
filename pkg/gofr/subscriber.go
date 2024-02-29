package gofr

import (
	"context"

	"gofr.dev/pkg/gofr/container"
)

type SubscribeFunc func(c *Context) error

type SubscriptionManager struct {
	*container.Container
	subscriptions map[string]SubscribeFunc
}

func newSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		subscriptions: make(map[string]SubscribeFunc),
	}
}

func (s *SubscriptionManager) startSubscriber(ctx context.Context, topic string, handler SubscribeFunc) {
	// continuously subscribe in an infinite loop
	for {
		msg, err := s.Container.GetSubscriber().Subscribe(ctx, topic)
		if msg == nil {
			continue
		}

		if err != nil {
			s.Container.Logger.Errorf("error while reading from Kafka, err: %v", err.Error())
			continue
		}

		ctx := newContext(nil, msg, s.Container)
		err = handler(ctx)

		// commit the message if the subscription function does not return error
		if err == nil {
			msg.Commit()
		} else {
			s.Container.Logger.Errorf("error in handler for topic %s: %v", topic, err)
		}
	}
}
