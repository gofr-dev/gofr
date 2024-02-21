package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
)

var errUnsupportedBindType = fmt.Errorf("unsupported type for binding message")

type Message struct {
	ctx context.Context

	Topic    string
	Value    []byte
	MetaData interface{}

	Committer
}

func NewMessage(ctx context.Context) *Message {
	if ctx == nil {
		return &Message{ctx: context.Background()}
	}

	return &Message{ctx: ctx}
}

func (m *Message) Context() context.Context {
	return m.ctx
}

func (m *Message) Param(p string) string {
	if p == "topic" {
		return m.Topic
	}

	return ""
}

func (m *Message) PathParam(p string) string {
	return m.Param(p)
}

func (m *Message) Bind(i interface{}) error {
	// TODO - implement other binding functionality
	err := json.Unmarshal(m.Value, i)
	return err
}

func (m *Message) HostName() string {
	return ""
}
