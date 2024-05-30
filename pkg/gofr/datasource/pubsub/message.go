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

func (m *Message) Bind(i any) error {
	if reflect.ValueOf(i).Kind() != reflect.Ptr {
		return errNotPointer
	}

	switch v := i.(type) {
	case *string:
		*v = string(m.Value)
		return nil
	case *float64:
		f, err := strconv.ParseFloat(string(m.Value), 64)
		if err != nil {
			return err
		}

		*v = f

		return nil
	case *int:
		in, err := strconv.Atoi(string(m.Value))
		if err != nil {
			return err
		}

		*v = in

		return nil
	case *bool:
		b, err := strconv.ParseBool(string(m.Value))
		if err != nil {
			return err
		}

		*v = b

		return nil
	default:
		err := json.Unmarshal(m.Value, i)
		return err
	}
}

func (m *Message) HostName() string {
	return ""
}
