package pubsub

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessage_Context(t *testing.T) {
	ctx := context.Background()
	m := NewMessage(ctx)

	out := m.Context()

	assert.Equal(t, ctx, out)
}

func TestMessage_Bind(t *testing.T) {
	testCases := []struct {
		desc     string
		input    interface{}
		value    []byte
		expected interface{}
		hasError bool
	}{
		{
			desc:     "bind to string",
			input:    new(string),
			value:    []byte("test"),
			expected: "test",
			hasError: false,
		},
		{
			desc:     "bind to float64",
			input:    new(float64),
			value:    []byte("1.23"),
			expected: 1.23,
			hasError: false,
		},
		{
			desc:     "bind to int",
			input:    new(int),
			value:    []byte("123"),
			expected: 123,
			hasError: false,
		},
		{
			desc:     "bind to bool",
			input:    new(bool),
			value:    []byte("true"),
			expected: true,
			hasError: false,
		},
		{
			desc:     "bind to map[string]interface{}",
			input:    &map[string]interface{}{},
			value:    []byte(`{"key":"value"}`),
			expected: &map[string]interface{}{"key": "value"},
			hasError: false,
		},
		{
			desc:     "bind to struct",
			input:    &struct{ Name string }{},
			value:    []byte(`{"Name":"test"}`),
			expected: &struct{ Name string }{Name: "test"},
			hasError: false,
		},
		{
			desc:     "bind to not pointer",
			input:    struct{ Name string }{},
			value:    []byte(`{"Name":"test"}`),
			expected: &struct{ Name string }{},
			hasError: true,
		},
		{
			desc:     "bind to struct with error",
			input:    &struct{ Name string }{},
			value:    []byte(`{"Name":}`),
			expected: &struct{ Name string }{},
			hasError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			m := NewMessage(context.Background())
			m.Value = tc.value

			err := m.Bind(tc.input)

			if tc.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				switch v := tc.input.(type) {
				case *string:
					assert.Equal(t, tc.expected, *v)
				case *float64:
					assert.Equal(t, tc.expected, *v)
				case *int:
					assert.Equal(t, tc.expected, *v)
				case *bool:
					assert.Equal(t, tc.expected, *v)
				default:
					assert.Equal(t, tc.expected, v)
				}
			}
		})
	}
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
