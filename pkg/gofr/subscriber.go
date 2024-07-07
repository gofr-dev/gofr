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

func (s *SubscriptionManager) startSubscriber(ctx context.Context, topic string, handler SubscribeFunc) {
	// continuously subscribe in an infinite loop
	for {
		select {
		case <-ctx.Done():
			s.container.Logger.Infof("shutting down subscriber for topic %s", topic)
			return
		default:
			err := s.handleSubscription(ctx, topic, handler)
			if err != nil {
				return
			}
		}
	}
}

func (s *SubscriptionManager) handleSubscription(parentCtx context.Context, topic string, handler SubscribeFunc) error {
	ctx := context.WithoutCancel(parentCtx)

	msg, err := s.container.GetSubscriber().Subscribe(ctx, topic)

	if errors.Is(err, kafka.ErrConsumerGroupNotProvided) {
		s.container.Logger.Error("cannot subscribe as consumer_id is not provided in configs")
		return err
	}

	if err != nil {
		s.container.Logger.Errorf("error while reading from topic %v, err: %v", topic, err.Error())
		return nil
	}

	if msg == nil {
		return nil
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
		return nil
	}

	if msg.Committer != nil {
		msg.Commit()
	}

	return nil
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
