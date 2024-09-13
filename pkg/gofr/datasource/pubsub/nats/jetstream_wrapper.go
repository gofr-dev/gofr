//go:build !ignore
// +build !ignore

package nats

import "github.com/nats-io/nats.go"

type jetStreamContextWrapper struct {
	nats.JetStreamContext
}

func newJetStreamContextWrapper(js nats.JetStreamContext) JetStreamContext {
	return &jetStreamContextWrapper{js}
}

func (j *jetStreamContextWrapper) DeleteStream(name string, opts ...nats.JSOpt) error {
	return j.JetStreamContext.DeleteStream(name, opts...)
}
