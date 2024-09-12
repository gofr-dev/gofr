//go:build !ignore
// +build !ignore

package nats

import (
	"github.com/nats-io/nats.go"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

type natsMessageWrapper struct {
	msg    *nats.Msg
	logger pubsub.Logger
}

func newNATSMessageWrapper(msg *nats.Msg, logger pubsub.Logger) *natsMessageWrapper {
	return &natsMessageWrapper{msg: msg, logger: logger}
}

func (w *natsMessageWrapper) Ack() error {
	return w.msg.Ack()
}

func (w *natsMessageWrapper) Data() []byte {
	return w.msg.Data
}

func (w *natsMessageWrapper) Subject() string {
	return w.msg.Subject
}

func (w *natsMessageWrapper) Headers() nats.Header {
	return w.msg.Header
}

func (w *natsMessageWrapper) Commit() {
	if err := w.msg.Ack(); err != nil {
		w.logger.Errorf("unable to acknowledge message on NATS JetStream: %v", err)
	}
}
