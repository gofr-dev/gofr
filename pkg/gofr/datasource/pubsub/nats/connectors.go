package nats

import (
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type DefaultNATSConnector struct{}

func (d *DefaultNATSConnector) Connect(serverURL string, opts ...nats.Option) (ConnInterface, error) {
	nc, err := nats.Connect(serverURL, opts...)
	if err != nil {
		return nil, err
	}
	return &natsConnWrapper{nc}, nil
}

type DefaultJetStreamCreator struct{}

func (d *DefaultJetStreamCreator) New(nc *nats.Conn) (jetstream.JetStream, error) {
	return jetstream.New(nc)
}
