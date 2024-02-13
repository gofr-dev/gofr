package pubsub

import "context"

type Publisher interface {
	Publish(ctx context.Context, topic string, message interface{}) error
}

type Subscriber interface {
	Subscribe(ctx context.Context, topic string) Message
}

type Logger interface {
	Logf(format string, args ...interface{})
	Log(args ...interface{})
}
