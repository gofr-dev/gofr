package pubsub

import (
	"context"
	"encoding/json"
)

type Message struct {
	ctx context.Context

	Topic    string
	Value    []byte
	MetaData interface{}
}

func (m *Message) Context() context.Context {
	return m.ctx
}

func (m *Message) Param(_ string) string {
	return ""
}

func (m *Message) PathParam(_ string) string {
	return ""
}

func (m *Message) Bind(i interface{}) error {
	// TODO - implement other binding functionality
	err := json.Unmarshal(m.Value, i)
	return err
}

func (m *Message) HostName() string {
	return ""
}
