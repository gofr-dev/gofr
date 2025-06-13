package gofr

import (
	"context"
	"errors"
	"fmt"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/datasource/pubsub/kafka"
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
