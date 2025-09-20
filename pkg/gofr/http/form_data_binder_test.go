package http

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_setFieldValueFromData(t *testing.T) {
	t.Run("String Field", func(t *testing.T) {
		var str string
		field := reflect.ValueOf(&str).Elem()

		err := setFieldValueFromData(field, "hello")
		require.NoError(t, err)
		assert.Equal(t, "hello", str)
	})

	t.Run("Int Field", func(t *testing.T) {
		var num int
		field := reflect.ValueOf(&num).Elem()

		err := setFieldValueFromData(field, 42)
		require.NoError(t, err)
		assert.Equal(t, 42, num)
	})

	t.Run("Float Field", func(t *testing.T) {
		var f float64
		field := reflect.ValueOf(&f).Elem()

		err := setFieldValueFromData(field, 3.14)
		require.NoError(t, err)
		assert.InEpsilon(t, 3.14, f, 0.001)
	})

	t.Run("Bool Field", func(t *testing.T) {
		var b bool
		field := reflect.ValueOf(&b).Elem()

		err := setFieldValueFromData(field, true)
		require.NoError(t, err)
		assert.True(t, b)
	})

	t.Run("Unsupported Kind", func(t *testing.T) {
		var m map[string]string
		field := reflect.ValueOf(&m).Elem()
		err := setFieldValueFromData(field, map[string]string{"a": "b"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported type for field")
	})
}
