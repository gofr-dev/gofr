package http

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

var (
	errUnsupportedType = errors.New("unsupported type for field: expected float64 but got bool")
	errJSON            = errors.New("unexpected end of JSON input")
)

func TestGetFieldName(t *testing.T) {
	tests := []struct {
		desc   string
		field  *reflect.StructField
		key    string
		wantOk bool
	}{
		{
			desc:   "Field with form tag",
			field:  &reflect.StructField{Tag: reflect.StructTag("form:\"name\"")},
			key:    "name",
			wantOk: true,
		},
		{
			desc:   "Field with file tag",
			field:  &reflect.StructField{Tag: reflect.StructTag("file:\"avatar\"")},
			key:    "avatar",
			wantOk: true,
		},
		{
			desc:   "Field with exported name",
			field:  &reflect.StructField{Name: "ID"},
			key:    "ID",
			wantOk: true,
		},
		{
			desc:   "Unexported field with tag",
			field:  &reflect.StructField{Name: "unexported", Tag: reflect.StructTag("form:\"data\""), PkgPath: "unexported"},
			key:    "",
			wantOk: false,
		},
		{
			desc:   "Field with omitted tag",
			field:  &reflect.StructField{},
			key:    "",
			wantOk: false,
		},
	}

	for i, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result, gotOk := getFieldName(tt.field)
			require.Equal(t, tt.key, result, "TestGetFieldName[%d] : %v Failed!", i, tt.desc)
			require.Equal(t, tt.wantOk, gotOk, "TestGetFieldName[%d] : %v Failed!", i, tt.desc)
		})
	}
}

type testValue struct {
	kind  reflect.Kind
	value any
}

func Test_SetFieldValue_Success(t *testing.T) {
	testCases := []struct {
		desc      string
		data      string
		expected  bool
		valueType testValue
	}{
		{"String", "test", true, testValue{reflect.String, "string"}},
		{"Int", "10", true, testValue{reflect.Int, 0}},
		{"Uint", "10", true, testValue{reflect.Uint16, uint16(10)}},
		{"Float64", "3.14", true, testValue{reflect.Float64, 0.0}},
		{"Bool", "true", true, testValue{reflect.Bool, false}},
		{"Slice", "1,2,3,4,5", true, testValue{reflect.Slice, []int{}}},
		{"Array", "1,2,3,4,5", true, testValue{reflect.Array, [5]int{}}},
		{"Struct", `{"name": "John", "age": 30}`, true, testValue{reflect.Struct, struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}{}}},
		{"Interface", "test interface", true, testValue{reflect.Interface, new(any)}},
	}

	for _, tc := range testCases {
		f := &formData{}
		val := reflect.New(reflect.TypeOf(tc.valueType.value)).Elem()

		set, err := f.setFieldValue(val, tc.data)

		require.NoErrorf(t, err, "Unexpected error for value kind %v and data %q", val.Kind(), tc.data)

		require.Equalf(t, tc.expected, set, "Expected set to be %v for value kind %v and data %q", tc.expected, val.Kind(), tc.data)
	}
}

func TestSetFieldValue_InvalidKinds(t *testing.T) {
	uf := &formData{}

	tests := []struct {
		kind reflect.Kind
		data string
		typ  reflect.Type
	}{
		{reflect.Complex64, "foo", reflect.TypeOf(complex64(0))},
		{reflect.Complex128, "bar", reflect.TypeOf(complex128(0))},
		{reflect.Chan, "baz", reflect.TypeOf(make(chan int))},
		{reflect.Func, "qux", reflect.TypeOf(func() {})},
		{reflect.Map, "quux", reflect.TypeOf(map[string]int{})},
		{reflect.UnsafePointer, "grault", reflect.TypeOf(unsafe.Pointer(nil))},
	}

	for _, tt := range tests {
		value := reflect.New(tt.typ).Elem()
		ok, err := uf.setFieldValue(value, tt.data)

		require.False(t, ok, "expected false, got true for kind %v", tt.kind)

		require.NoError(t, err, "expected nil, got %v for kind %v", err, tt.kind)
	}
}

func TestSetSliceOrArrayValue(t *testing.T) {
	type testStruct struct {
		Slice []string
		Array [3]string
	}

	uf := &formData{}

	// Test with a slice
	value := reflect.ValueOf(&testStruct{Slice: nil}).Elem().FieldByName("Slice")

	data := "a,b,c"

	ok, err := uf.setSliceOrArrayValue(value, data)

	require.True(t, ok, "setSliceOrArrayValue failed")

	require.NoError(t, err, "setSliceOrArrayValue failed: %v", err)

	require.Len(t, value.Interface().([]string), 3, "slice not set correctly")

	// Test with an array
	value = reflect.ValueOf(&testStruct{Array: [3]string{}}).Elem().FieldByName("Array")

	data = "a,b,c"

	ok, err = uf.setSliceOrArrayValue(value, data)

	require.True(t, ok, "setSliceOrArrayValue failed")

	require.NoError(t, err, "setSliceOrArrayValue failed: %v", err)
}

func TestSetStructValue_Success(t *testing.T) {
	type testStruct struct {
		Field1 string
		Field2 int
	}

	uf := &formData{}

	tests := []struct {
		name       string
		data       string
		wantField1 string
		wantField2 int
	}{
		{
			name:       "Valid input with correct case",
			data:       `{"Field1":"value1","Field2":123}`,
			wantField1: "value1",
			wantField2: 123,
		},
		{
			name:       "Valid input with case insensitive fields",
			data:       `{"field1":"value2","FIELD2":456}`,
			wantField1: "value2",
			wantField2: 456,
		},
		{
			name:       "Mixed Case and invalid field names",
			data:       `{"FielD1":"value4", "invalidField":"ignored", "FiEld2":789}`,
			wantField1: "value4",
			wantField2: 789,
		},
		{
			name:       "Case-insensitive field name but not in dataMap",
			data:       `{"fIeLd1":"value5", "not_in_dataMap": 123}`,
			wantField1: "value5",
			wantField2: 0, // Field2 should remain unset (default 0)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := reflect.ValueOf(&testStruct{}).Elem()

			ok, err := uf.setStructValue(value, tt.data)

			require.NoError(t, err, "TestSetStructValue_Success Failed.")
			require.True(t, ok, "TestSetStructValue_Success Failed.")
			require.Equal(t, tt.wantField1, value.FieldByName("Field1").String(),
				"TestSetStructValue_Success Failed : Field1 not set correctly")
			require.Equal(t, tt.wantField2, int(value.FieldByName("Field2").Int()),
				"TestSetStructValue_Success Failed : Field2 not set correctly")
		})
	}
}

func TestSetStructValue_Errors(t *testing.T) {
	type testStruct struct {
		Field1 string
		Field2 int
		Field4 float64
	}

	uf := &formData{}

	tests := []struct {
		name string
		data string
		err  error
	}{
		{
			name: "Unexported field",
			data: `{"field3":"value3"}`,
			err:  errFieldsNotSet,
		},
		{
			name: "Unsupported field type",
			data: `{"field2":1,"Field4":true}`,
			err:  fmt.Errorf("%w; %w", nil, errUnsupportedType),
		},
		{
			name: "Invalid JSON",
			data: `{"Field1":"value1", "Field2":123,`,
			err:  errJSON, // JSON parsing error
		},
		{
			name: "Field not settable",
			data: `{"Field1":"value1", "Field2":123, "Field4": "not a float"}`,
			err:  errUnsupportedFieldType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := reflect.ValueOf(&testStruct{}).Elem()

			_, err := uf.setStructValue(value, tt.data)

			require.Error(t, err, "TestSetStructValue_Errors Failed.")
			require.Contains(t, err.Error(), tt.err.Error(), "TestSetStructValue_Errors Failed.")
		})
	}
}
