package google

import (
	"context"

	"cloud.google.com/go/pubsub"
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
