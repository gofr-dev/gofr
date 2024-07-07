package gofr

import (
	"context"
	"errors"
	"runtime/debug"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/pubsub"
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

func (s *SubscriptionManager) startSubscriber(ctx context.Context, topic string, handler SubscribeFunc) {
	// continuously subscribe in an infinite loop
	for {
		s.processSubscription(ctx, topic, handler)
	}
}

func (s *SubscriptionManager) processSubscription(parentCtx context.Context, topic string, handler SubscribeFunc) {
	ctx, done := context.WithCancel(parentCtx)
	defer done()

	msg, err := s.container.GetSubscriber().Subscribe(ctx, topic)

	select {
	case <-ctx.Done():
		s.container.Logger.Infof("shutting down subscriber for topic %s", topic)
		return
	default:
		s.handleSubscription(topic, handler, msg, err)
	}
}

func (s *SubscriptionManager) handleSubscription(topic string, handler SubscribeFunc, msg *pubsub.Message, err error) {
	if errors.Is(err, kafka.ErrConsumerGroupNotProvided) {
		s.container.Logger.Errorf("cannot subscribe as consumer_id is not provided in configs")
		return
	}

	if err != nil {
		s.container.Logger.Errorf("error while reading from topic %v, err: %v", topic, err.Error())
		return
	}

	if msg == nil {
		return
	}

	msgCtx := newContext(nil, msg, s.container)
	err = func(ctx *Context) error {
		// TODO : Move panic recovery at central location which will manage for all the different cases.
		defer func() {
			panicRecovery(recover(), ctx.Logger)
		}()

		return handler(ctx)
	}(msgCtx)

	// commit the message if the subscription function does not return error
	if err != nil {
		s.container.Logger.Errorf("error in handler for topic %s: %v", topic, err)
		return
	}

	if msg.Committer != nil {
		msg.Commit()
	}
}

type panicLog struct {
	Error      string `json:"error,omitempty"`
	StackTrace string `json:"stack_trace,omitempty"`
}

func panicRecovery(re any, log logging.Logger) {
	if re == nil {
		return
	}

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
