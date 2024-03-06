package migration

import "context"

type client interface {
	CreateTopic(context context.Context, name string) error
	DeleteTopic(context context.Context, name string) error
}

type pubsub struct {
	client
}

func newPubSub(p client) *pubsub {
	return &pubsub{client: p}
}

func (s *pubsub) CreateTopic(ctx context.Context, name string) error {
	return s.client.CreateTopic(ctx, name)
}

func (s *pubsub) DeleteTopic(ctx context.Context, name string) error {
	return s.client.DeleteTopic(ctx, name)
}
