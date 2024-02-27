package pubsub

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessage_Context(t *testing.T) {
	ctx := context.Background()
	m := NewMessage(ctx)

	out := m.Context()

	assert.Equal(t, ctx, out)
}

func TestMessage_BindError(t *testing.T) {
	m := NewMessage(context.TODO())

	// the value is not in json Format
	m.Value = []byte(``)

	err := m.Bind(struct{}{})

	// check if error is present
	if assert.Error(t, err) {
		// the error should be json syntax error
		assert.IsType(t, &json.SyntaxError{}, err)
	}
}

func TestMessage_BindSuccess(t *testing.T) {
	type order struct {
		OrderID int `json:"orderID"`
	}

	m := NewMessage(context.TODO())

	m.Value = []byte(`{"orderID":123}`)

	var data order

	err := m.Bind(&data)

	assert.Nil(t, err)
	assert.Equal(t, order{OrderID: 123}, data)
}

func TestMessage_Param(t *testing.T) {
	testCases := []struct {
		desc        string
		input       string
		expectedOut string
	}{
		{desc: "topic is fetched", input: "topic", expectedOut: "test-topic"},
		{desc: "any other param is fetched", input: "path", expectedOut: ""},
	}

	m := NewMessage(context.TODO())
	m.Topic = "test-topic"

	for _, tc := range testCases {
		out := m.Param(tc.input)

		assert.Equal(t, tc.expectedOut, out)
	}
}

func TestMessage_PathParam(t *testing.T) {
	testCases := []struct {
		desc        string
		input       string
		expectedOut string
	}{
		{desc: "topic is fetched", input: "topic", expectedOut: "test-topic"},
		{desc: "other path param is fetched", input: "path", expectedOut: ""},
	}

	m := NewMessage(context.TODO())
	m.Topic = "test-topic"

	for _, tc := range testCases {
		out := m.PathParam(tc.input)

		assert.Equal(t, tc.expectedOut, out)
	}
}

func TestMessage_HostName(t *testing.T) {
	m := &Message{}

	out := m.HostName()

	assert.Equal(t, "", out)
}
