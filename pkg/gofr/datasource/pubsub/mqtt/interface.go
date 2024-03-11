package mqtt

import (
	"context"

	"gofr.dev/pkg/gofr/datasource"
)

type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

type Metrics interface {
	IncrementCounter(ctx context.Context, name string, labels ...string)
}

type PubSub interface {
	SubscribeWithFunction(topic string, subscribeFunc SubscribeFunc) error
	Publish(ctx context.Context, topic string, message []byte) error
	Unsubscribe(topic string) error
	Disconnect(waitTime uint)
	Ping() error
	Health() datasource.Health
}
