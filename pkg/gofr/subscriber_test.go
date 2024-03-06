package gofr

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

var errHandler = errors.New("error in subscribing")

func handleError(err string) error {
	return fmt.Errorf("%w: %s", errHandler, err)
}

var errSubscription = errors.New("subscription error")

func subscriptionError(err string) error {
	return fmt.Errorf("%w: %s", errSubscription, err)
}

type mockSubscriber struct {
}

func (s mockSubscriber) CreateTopic(_ context.Context, _ string) error {
	return nil
}

func (s mockSubscriber) DeleteTopic(_ context.Context, _ string) error {
	return nil
}

func (s mockSubscriber) Health() datasource.Health {
	return datasource.Health{}
}

func (s mockSubscriber) Publish(_ context.Context, _ string, _ []byte) error {
	return nil
}

func (mockSubscriber) Subscribe(_ context.Context, topic string) (*pubsub.Message, error) {
	if topic == "test-topic" {
		return &pubsub.Message{
			Topic: "test-topic",
			Value: []byte(`{"data":{"productId":"123","price":"599"}}`),
		}, nil
	}

	return &pubsub.Message{
		Topic: "test-topic",
		Value: []byte(`{"data":{"productId":"123","price":"599"}}`),
	}, subscriptionError("subscription error")
}

func TestSubscriptionManager_HandlerError(t *testing.T) {
	done := make(chan struct{})

	testLogs := testutil.StderrOutputForFunc(func() {
		mockContainer := container.Container{
			Logger: logging.NewLogger(logging.ERROR),
			PubSub: mockSubscriber{},
		}
		subscriptionManager := newSubscriptionManager(&mockContainer)

		// Run the subscriber in a goroutine
		go func() {
			subscriptionManager.startSubscriber("test-topic",
				func(c *Context) error {
					return handleError("error in test-topic")
				})
		}()

		// this sleep is added to wait for StderrOutputForFunc to collect the logs inside the testLogs
		time.Sleep(1 * time.Millisecond)
	})

	// signal the test to end
	close(done)

	if !strings.Contains(testLogs, "error in handler for topic test-topic: error in subscribing: error in test-topic") {
		t.Error("TestSubscriptionManager_HandlerError Failed! Missing log message about handler error")
	}
}

func TestSubscriptionManager_SubscribeError(t *testing.T) {
	done := make(chan struct{})

	testLogs := testutil.StderrOutputForFunc(func() {
		mockContainer := container.Container{
			Logger: logging.NewLogger(logging.ERROR),
			PubSub: mockSubscriber{},
		}
		subscriptionManager := newSubscriptionManager(&mockContainer)

		// Run the subscriber in a goroutine
		go func() {
			subscriptionManager.startSubscriber("abc",
				func(c *Context) error {
					return handleError("error in abc")
				})
		}()

		// this sleep is added to wait for StderrOutputForFunc to collect the logs inside the testLogs
		time.Sleep(1 * time.Millisecond)
	})

	// signal the test to end
	close(done)

	if !strings.Contains(testLogs, "error while reading from Kafka, err: ") {
		t.Error("TestSubscriptionManager_SubscribeError Failed! Missing log message about subscription error")
	}
}
