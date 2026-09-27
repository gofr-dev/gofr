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

type formBinderTarget struct {
	Name   string
	Age    int
	Active bool
	secret string //nolint:unused // only reached via reflection, to exercise the unexported-field path
}

func TestFormData_setInterfaceValue(t *testing.T) {
	var settable any

	tests := []struct {
		desc   string
		value  reflect.Value
		data   any
		expOK  bool
		expErr error
	}{
		{desc: "settable interface", value: reflect.ValueOf(&settable).Elem(), data: "hello", expOK: true, expErr: nil},
		{desc: "non settable value", value: reflect.ValueOf(any(1)), data: "hello", expOK: false,
			expErr: errUnsupportedInterfaceType},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ok, err := (&formData{}).setInterfaceValue(tc.value, tc.data)

			assert.Equal(t, tc.expOK, ok)
			require.ErrorIs(t, err, tc.expErr)
		})
	}

	assert.Equal(t, "hello", settable)
}

func TestFormData_setSliceOrArrayValue(t *testing.T) {
	tests := []struct {
		desc   string
		target any
		data   string
		expOK  bool
		expErr error
		expVal any
	}{
		{desc: "slice of ints", target: &[]int{}, data: "1,2,3", expOK: true, expVal: []int{1, 2, 3}},
		{desc: "array within capacity", target: &[3]int{}, data: "1,2", expOK: true, expVal: [3]int{1, 2, 0}},
		{desc: "array capacity exceeded", target: &[2]int{}, data: "1,2,3", expOK: false,
			expErr: errDataLengthExceeded, expVal: [2]int{}},
		{desc: "invalid element", target: &[]int{}, data: "1,x", expOK: false,
			expErr: errSettingValueFailure, expVal: []int{}},
		{desc: "unsupported kind", target: new(int), data: "1", expOK: false,
			expErr: errUnsupportedKind, expVal: 0},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			value := reflect.ValueOf(tc.target).Elem()

			ok, err := (&formData{}).setSliceOrArrayValue(value, tc.data)

			assert.Equal(t, tc.expOK, ok)
			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expVal, value.Interface())
		})
	}
}

func TestFormData_setStructValue(t *testing.T) {
	tests := []struct {
		desc   string
		target any
		data   string
		expOK  bool
		expErr error
		expVal any
	}{
		{desc: "all fields set case-insensitively", target: &formBinderTarget{},
			data: `{"name":"gofr","AGE":10,"active":true}`, expOK: true,
			expVal: formBinderTarget{Name: "gofr", Age: 10, Active: true}},
		{desc: "unexported field reported", target: &formBinderTarget{}, data: `{"Name":"gofr","secret":"x"}`,
			expOK: true, expErr: errUnexportedField, expVal: formBinderTarget{Name: "gofr"}},
		{desc: "mismatched int and bool types reported", target: &formBinderTarget{},
			data: `{"Name":"gofr","Age":"ten","Active":"yes"}`, expOK: true, expErr: errUnsupportedFieldType,
			expVal: formBinderTarget{Name: "gofr"}},
		{desc: "mismatched string type sets no field", target: &formBinderTarget{}, data: `{"Name":1}`,
			expOK: false, expErr: errFieldsNotSet, expVal: formBinderTarget{}},
		{desc: "empty object", target: &formBinderTarget{}, data: `{}`, expOK: false, expErr: errFieldsNotSet,
			expVal: formBinderTarget{}},
		{desc: "not a struct", target: new(int), data: `{"Name":"gofr"}`, expOK: false, expErr: errNotAStruct, expVal: 0},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			value := reflect.ValueOf(tc.target).Elem()

			ok, err := (&formData{}).setStructValue(value, tc.data)

			assert.Equal(t, tc.expOK, ok)
			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expVal, value.Interface())
		})
	}
}

func TestFormData_setStructValue_InvalidJSON(t *testing.T) {
	tests := []struct {
		desc string
		data string
	}{
		{desc: "malformed json", data: `{"Name":`},
		{desc: "json array instead of object", data: `[1,2]`},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var target formBinderTarget

			ok, err := (&formData{}).setStructValue(reflect.ValueOf(&target).Elem(), tc.data)

			assert.False(t, ok)
			require.Error(t, err)
			assert.Equal(t, formBinderTarget{}, target)
		})
	}
}
