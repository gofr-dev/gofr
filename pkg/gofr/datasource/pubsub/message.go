package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
)

var errNotPointer = errors.New("input should be a pointer to a variable")

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

// Bind binds the message value to the input variable. The input should be a pointer to a variable.
func (m *Message) Bind(i any) error {
	if reflect.ValueOf(i).Kind() != reflect.Ptr {
		return errNotPointer
	}

	switch v := i.(type) {
	case *string:
		return m.bindString(v)
	case *float64:
		return m.bindFloat64(v)
	case *int:
		return m.bindInt(v)
	case *bool:
		return m.bindBool(v)
	default:
		return m.bindStruct(i)
	}
}

func (m *Message) bindString(v *string) error {
	*v = string(m.Value)
	return nil
}

func (m *Message) bindFloat64(v *float64) error {
	f, err := strconv.ParseFloat(string(m.Value), 64)
	if err != nil {
		return err
	}

	*v = f

	return nil
}

func (m *Message) bindInt(v *int) error {
	in, err := strconv.Atoi(string(m.Value))
	if err != nil {
		return err
	}

	*v = in

	return nil
}

func (m *Message) bindBool(v *bool) error {
	b, err := strconv.ParseBool(string(m.Value))
	if err != nil {
		return err
	}

	*v = b

	return nil
}

func (m *Message) bindStruct(i any) error {
	return json.Unmarshal(m.Value, i)
}

func (*Message) HostName() string {
	return ""
}
