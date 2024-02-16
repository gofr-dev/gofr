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
}

func NewMessage() *Message {
	return &Message{ctx: context.Background()}
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
	switch v := i.(type) {
	case string:
		m.Value = []byte(v)
	case []byte:
		m.Value = v
	case json.RawMessage:
		m.Value = v
	case fmt.Stringer:
		m.Value = []byte(v.String())
	default:
		return errUnsupportedBindType
	}

	return nil
}

func (m *Message) HostName() string {
	return ""
}
