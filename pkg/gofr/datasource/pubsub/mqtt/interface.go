// Package mqtt provides a client for interacting with MQTT message brokers.This package facilitates interaction with
// MQTT brokers, allowing publishing and subscribing to topics, managing subscriptions, and handling messages.
package mqtt

import (
	"context"

	"gofr.dev/pkg/gofr/datasource"
)

type Logger interface {
	Infof(format string, args ...interface{})
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
