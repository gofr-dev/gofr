package gofr

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/datasource/pubsub/kafka"
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

func (mockSubscriber) CreateTopic(_ context.Context, _ string) error {
	return nil
}

func (mockSubscriber) DeleteTopic(_ context.Context, _ string) error {
	return nil
}

func (mockSubscriber) Health() datasource.Health {
	return datasource.Health{}
}

func (mockSubscriber) Publish(_ context.Context, _ string, _ []byte) error {
	return nil
}

func (mockSubscriber) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	msg := pubsub.NewMessage(ctx)
	msg.Topic = topic
	msg.Value = []byte(`{"data":{"productId":"123","price":"599"}}`)

	if topic == "test-topic" {
		return msg, nil
	} else if topic == "test-err" {
		return msg, kafka.ErrConsumerGroupNotProvided
	}

	return msg, subscriptionError("subscription error")
}

func (mockSubscriber) Close() error {
	return nil
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
			err := subscriptionManager.startSubscriber(context.Background(), "test-topic",
				func(*Context) error {
					return handleError("error in test-topic")
				})

			assert.ErrorContains(t, err, "error in test-topic")
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
			err := subscriptionManager.startSubscriber(context.Background(), "abc",
				func(*Context) error {
					return handleError("error in abc")
				})

			assert.Contains(t, err.Error(), subscriptionError("subscription error").Error())
		}()

		// this sleep is added to wait for StderrOutputForFunc to collect the logs inside the testLogs
		time.Sleep(1 * time.Millisecond)
	})

	// signal the test to end
	close(done)

	if !strings.Contains(testLogs, "error while reading from ") {
		t.Error("TestSubscriptionManager_SubscribeError Failed! Missing log message about subscription error")
	}
}

func TestSubscriptionManager_PanicRecovery(t *testing.T) {
	done := make(chan struct{})

	testLogs := testutil.StderrOutputForFunc(func() {
		mockContainer := container.Container{
			Logger: logging.NewLogger(logging.ERROR),
			PubSub: mockSubscriber{},
		}
		subscriptionManager := newSubscriptionManager(&mockContainer)

		// Run the subscriber in a goroutine
		go func() {
			_ = subscriptionManager.startSubscriber(context.Background(), "abc",
				func(*Context) error {
					panic("test panic")
				})
		}()

		// this sleep is added to wait for StderrOutputForFunc to collect the logs inside the testLogs
		time.Sleep(1 * time.Millisecond)
	})

	// signal the test to end
	close(done)

	if !strings.Contains(testLogs, "error while reading from") {
		t.Error("TestSubscriptionManager_SubscribeError Failed! Missing log message about subscription error")
	}
}

func TestSubscriptionManager_ShouldStopOnCtxDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	mockContainer := container.Container{
		Logger: logging.NewLogger(logging.ERROR),
		PubSub: mockSubscriber{},
	}

	subscriptionManager := newSubscriptionManager(&mockContainer)

	// should handle one message and quit due to canceled context
	err := subscriptionManager.startSubscriber(ctx, "test-topic", func(*Context) error {
		cancel()
		return nil
	})

	require.NoError(t, err)
}
