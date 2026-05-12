package gofr

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/datasource/pubsub/kafka"
	"gofr.dev/pkg/gofr/logging"
)

var errSubscription = errors.New("subscription error")

func subscriptionError(err string) error {
	return fmt.Errorf("%w: %s", errSubscription, err)
}

type mockSubscriber struct {
}

func (mockSubscriber) Query(_ context.Context, _ string, _ ...any) ([]byte, error) {
	return nil, nil
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

const (
	topicHandleSubPanic = "handle-sub-panic"
	topicHandleSubOK    = "handle-sub-ok"
	topicHandleSubErr   = "handle-sub-err"
)

// countingCommitter records Commit calls for tests.
type countingCommitter struct {
	n int
}

func (c *countingCommitter) Commit() {
	c.n++
}

// handleSubscriptionTestPubSub returns messages with a counting committer for selected topics.
// lastCommitter is the spy for the most recent Subscribe matching those topics.
type handleSubscriptionTestPubSub struct {
	mockSubscriber
	lastCommitter *countingCommitter
}

func (h *handleSubscriptionTestPubSub) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	msg := pubsub.NewMessage(ctx)
	msg.Topic = topic
	msg.Value = []byte(`{}`)

	switch topic {
	case topicHandleSubPanic, topicHandleSubOK, topicHandleSubErr:
		h.lastCommitter = &countingCommitter{}
		msg.Committer = h.lastCommitter

		return msg, nil
	default:
		return (&mockSubscriber{}).Subscribe(ctx, topic)
	}
}

func TestSubscriptionManager_handleSubscription_SuccessCommits(t *testing.T) {
	t.Parallel()

	ps := &handleSubscriptionTestPubSub{}
	c := &container.Container{
		Logger: logging.NewMockLogger(logging.ERROR),
		PubSub: ps,
	}
	s := SubscriptionManager{container: c}

	err := s.handleSubscription(t.Context(), topicHandleSubOK, func(*Context) error { return nil })
	require.NoError(t, err)
	require.NotNil(t, ps.lastCommitter)
	require.Equal(t, 1, ps.lastCommitter.n, "successful handler must commit once")
}

func TestSubscriptionManager_handleSubscription_PanicDoesNotCommit(t *testing.T) {
	t.Parallel()

	ps := &handleSubscriptionTestPubSub{}
	c := &container.Container{
		Logger: logging.NewMockLogger(logging.ERROR),
		PubSub: ps,
	}
	s := SubscriptionManager{container: c}

	err := s.handleSubscription(t.Context(), topicHandleSubPanic, func(*Context) error {
		panic("boom")
	})
	require.NoError(t, err)
	require.NotNil(t, ps.lastCommitter)
	require.Zero(t, ps.lastCommitter.n, "panic must not commit the message")
}

func TestSubscriptionManager_handleSubscription_HandlerErrorDoesNotCommit(t *testing.T) {
	t.Parallel()

	ps := &handleSubscriptionTestPubSub{}
	c := &container.Container{
		Logger: logging.NewMockLogger(logging.ERROR),
		PubSub: ps,
	}
	s := SubscriptionManager{container: c}

	handlerErr := errors.New("handler failed")
	err := s.handleSubscription(t.Context(), topicHandleSubErr, func(*Context) error { return handlerErr })
	require.NoError(t, err)
	require.NotNil(t, ps.lastCommitter)
	require.Zero(t, ps.lastCommitter.n, "handler error must not commit")
}
