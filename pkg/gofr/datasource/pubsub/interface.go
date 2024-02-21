package pubsub

import (
	"context"
)

type Publisher interface {
	Publish(ctx context.Context, topic string, message []byte) error
}

type Subscriber interface {
	Subscribe(ctx context.Context, topic string) (*Message, error)
}

type Client interface {
	Publisher
	Subscriber
}

type Committer interface {
	Commit()
}

type Logger interface {
	Debugf(format string, args ...interface{})
	Debug(args ...interface{})
	Logf(format string, args ...interface{})
	Log(args ...interface{})
	Errorf(format string, args ...interface{})
	Error(args ...interface{})
}
