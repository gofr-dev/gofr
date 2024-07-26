package gofr

import (
	"context"
	"runtime/debug"

	"gofr.dev/pkg/gofr/container"
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

// startSubscriber continuously subscribes to a topic and handles messages using the provided handler.
func (s *SubscriptionManager) startSubscriber(ctx context.Context, topic string, handler SubscribeFunc) error {
	for {
		select {
		case <-ctx.Done():
			s.container.Logger.Infof("shutting down subscriber for topic %s", topic)
			return nil
		default:
			err := s.handleSubscription(ctx, topic, handler)
			if err != nil {
				s.container.Logger.Errorf("error in subscription for topic %s: %v", topic, err)
			}
		}
	}
}

func (s *SubscriptionManager) handleSubscription(ctx context.Context, topic string, handler SubscribeFunc) error {
	msg, err := s.container.GetSubscriber().Subscribe(ctx, topic)

	if err != nil {
		s.container.Logger.Errorf("error while reading from topic %v, err: %v", topic, err.Error())

		return err
	}

	if msg == nil {
		return nil
	}

	// newContext creates a new context from the msg.Context()
	msgCtx := newContext(nil, msg, s.container)
	err = func(ctx *Context) error {
		// TODO : Move panic recovery at central location which will manage for all the different cases.
		defer func() {
			panicRecovery(recover(), ctx.Logger)
		}()

		return handler(ctx)
	}(msgCtx)

	if err != nil {
		s.container.Logger.Errorf("error in handler for topic %s: %v", topic, err)

		return nil
	}

	if msg.Committer != nil {
		// commit the message if the subscription function does not return error
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
