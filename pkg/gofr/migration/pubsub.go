package migration

import "context"

// MigrationManger interface is not implemented because it is not possible to run transaction for creating and deleting topics,
// it is open for further development if we can implement it.

type Client interface {
	CreateTopic(context context.Context, name string) error
	DeleteTopic(context context.Context, name string) error
}

type pubsub struct {
	Client
}

func newPubSub(p Client) *pubsub {
	return &pubsub{Client: p}
}

func (s *pubsub) CreateTopic(ctx context.Context, name string) error {
	return s.Client.CreateTopic(ctx, name)
}

func (s *pubsub) DeleteTopic(ctx context.Context, name string) error {
	return s.Client.DeleteTopic(ctx, name)
}
