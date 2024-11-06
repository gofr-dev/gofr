// Package nats connector.go
package nats

import (
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type defaultConnector struct{}

func (*defaultConnector) Connect(serverURL string, opts ...nats.Option) (ConnInterface, error) {
	nc, err := nats.Connect(serverURL, opts...)
	if err != nil {
		return nil, err
	}

	return &natsConnWrapper{nc}, nil
}

type DefaultJetStreamCreator struct{}

func (*DefaultJetStreamCreator) New(conn ConnInterface) (jetstream.JetStream, error) {
	return conn.JetStream()
}
