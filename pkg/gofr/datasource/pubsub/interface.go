// Package pubsub provides a foundation for implementing pub/sub clients for various message brokers such as google pub-sub,
// kafka and MQTT. It defines interfaces for publishing and subscribing to messages, managing topics, and handling messages.
package pubsub

import (
	"context"

	"gofr.dev/pkg/gofr/datasource"
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
	Health() datasource.Health

	CreateTopic(context context.Context, name string) error
	DeleteTopic(context context.Context, name string) error
	Query(ctx context.Context, query string, args ...any) ([]byte, error)

	Close() error
}

type Committer interface {
	Commit()
}

type Logger interface {
	Debugf(format string, args ...any)
	Debug(args ...any)
	Logf(format string, args ...any)
	Log(args ...any)
	Errorf(format string, args ...any)
	Error(args ...any)
}
