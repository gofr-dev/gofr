package pubsub

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				switch v := tc.input.(type) {
				case *string:
					assert.Equal(t, tc.expected, *v)
				case *float64:
					assert.InEpsilon(t, tc.expected, *v, 0.01)
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

func TestBindString(t *testing.T) {
	m := &Message{Value: []byte("test")}

	var s string
	err := m.bindString(&s)
	require.NoError(t, err)
	assert.Equal(t, "test", s)
}

func TestBindFloat64(t *testing.T) {
	m := &Message{Value: []byte("1.23")}

	var f float64
	err := m.bindFloat64(&f)
	require.NoError(t, err)
	assert.InEpsilon(t, 1.23, f, 0.01)

	m = &Message{Value: []byte("not a float")}

	var f2 float64
	err = m.bindFloat64(&f2)
	require.Error(t, err)
}

func TestBindInt(t *testing.T) {
	m := &Message{Value: []byte("123")}

	var i int
	err := m.bindInt(&i)
	require.NoError(t, err)
	assert.Equal(t, 123, i)

	m = &Message{Value: []byte("not an int")}

	var i2 int
	err = m.bindInt(&i2)
	require.Error(t, err)
}

func TestBindBool(t *testing.T) {
	m := &Message{Value: []byte("true")}

	var b bool
	err := m.bindBool(&b)
	require.NoError(t, err)
	assert.True(t, b)

	m = &Message{Value: []byte("not a bool")}

	var b2 bool
	err = m.bindBool(&b2)
	require.Error(t, err)
}

func TestBindStruct(t *testing.T) {
	m := &Message{Value: []byte(`{"key":"value"}`)}

	var i map[string]interface{}
	err := m.bindStruct(&i)
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"key": "value"}, i)

	m = &Message{Value: []byte(`{"key":}`)}

	var i2 map[string]interface{}
	err = m.bindStruct(&i2)
	require.Error(t, err)
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
