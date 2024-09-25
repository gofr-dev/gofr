// Package mqtt provides a client for interacting with MQTT message brokers.This package facilitates interaction with
// MQTT brokers, allowing publishing and subscribing to topics, managing subscriptions, and handling messages.
package mqtt

import (
	"context"

	"gofr.dev/pkg/gofr/datasource"
)

//go:generate go run go.uber.org/mock/mockgen -destination=mock_client.go -package=mqtt github.com/eclipse/paho.mqtt.golang Client
//go:generate go run go.uber.org/mock/mockgen -destination=mock_token.go -package=mqtt github.com/eclipse/paho.mqtt.golang Token
//go:generate go run go.uber.org/mock/mockgen -source=interface.go -destination=mock_interfaces.go -package=mqtt

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
	Disconnect(waitTime uint) error
	Ping() error
	Health() datasource.Health
}
