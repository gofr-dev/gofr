package migration

type client interface {
	CreateTopic(name string) error
}

type pubsub struct {
	client
}

func newPubSub(p client) *pubsub {
	return &pubsub{client: p}
}

func (s *pubsub) CreateTopic(name string) error {
	return s.client.CreateTopic(name)
}
