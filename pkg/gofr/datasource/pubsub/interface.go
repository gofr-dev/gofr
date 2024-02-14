package pubsub

import "context"

type Publisher interface {
	Publish(ctx context.Context, topic string, message []byte) error
}

type Subscriber interface {
	Subscribe(ctx context.Context, topic string) (Message, error)
}

type Client interface {
	Publisher
	Subscriber
}

type Logger interface {
	Logf(format string, args ...interface{})
	Log(args ...interface{})
	Errorf(format string, args ...interface{})
	Error(args ...interface{})
}
