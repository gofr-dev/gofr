package google

import (
	"context"

	"cloud.google.com/go/pubsub" //nolint:staticcheck // pubsub v1 is deprecated in favor of v2; the migration is a separate change
)

type Client interface {
	Writer
	Reader

	Close() error
	Topics(ctx context.Context) *pubsub.TopicIterator
	Subscriptions(ctx context.Context) *pubsub.SubscriptionIterator
}

type Writer interface {
	Subscription(id string) *pubsub.Subscription
	CreateSubscription(ctx context.Context, id string, cfg pubsub.SubscriptionConfig) (*pubsub.Subscription, error)
}

type Reader interface {
	Topic(id string) *pubsub.Topic
	CreateTopic(ctx context.Context, topicID string) (*pubsub.Topic, error)
}
