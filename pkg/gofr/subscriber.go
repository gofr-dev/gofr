package gofr

import (
	"context"
	"errors"
	"runtime/debug"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/pubsub/kafka"
	"gofr.dev/pkg/gofr/logging"
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

		if errors.Is(err, kafka.ErrConsumerGroupNotProvided) {
			s.container.Logger.Errorf("cannot subscribe as consumer_id is not provided in configs")
			return
		} else if err != nil {
			s.container.Logger.Errorf("error while reading from topic %v, err: %v", topic, err.Error())
			continue
		}

		ctx := newContext(nil, msg, s.container)
		err = func(ctx *Context) error {
			// TODO : Move panic recovery at central location which will manage for all the different cases.
			defer panicRecovery(ctx.Logger)
			return handler(ctx)
		}(ctx)

		// commit the message if the subscription function does not return error
		if err == nil {
			msg.Commit()
		} else {
			s.container.Logger.Errorf("error in handler for topic %s: %v", topic, err)
		}
	}
}

type panicLog struct {
	Error      string `json:"error,omitempty"`
	StackTrace string `json:"stack_trace,omitempty"`
}

func panicRecovery(log logging.Logger) {
	re := recover()

	if re != nil {
		var e string
		switch t := re.(type) {
		case string:
			e = t
		case error:
			e = t.Error()
		default:
			e = "Unknown panic type"
		}
		log.Error(panicLog{
			Error:      e,
			StackTrace: string(debug.Stack()),
		})
	}
}
