package migration

import "context"

type client interface {
	CreateTopic(context context.Context, name string) error
}

type pubsub struct {
	client
}

func newPubSub(p client) *pubsub {
	return &pubsub{client: p}
}

func (s *pubsub) CreateTopic(context context.Context, name string) error {
	return s.client.CreateTopic(context, name)
}
