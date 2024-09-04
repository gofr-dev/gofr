package http

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
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
			assert.Equal(t, tt.key, result, "TestGetFieldName[%d] : %v Failed!", i, tt.desc)
			assert.Equal(t, tt.wantOk, gotOk, "TestGetFieldName[%d] : %v Failed!", i, tt.desc)
		})
	}
}

type testValue struct {
	kind  reflect.Kind
	value interface{}
}

type interfaceValue struct {
	value interface{}
}

func (iv interfaceValue) String() string {
	return "interface value"
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
	}

	for _, tc := range testCases {
		f := &formData{}
		val := reflect.New(reflect.TypeOf(tc.valueType.value)).Elem()

		set, err := f.setFieldValue(val, tc.data)
		if err != nil {
			t.Errorf("Unexpected error for value kind %v and data %q: %v", val.Kind(), tc.data, err)
		}

		if set != tc.expected {
			t.Errorf("Expected set to be %v for value kind %v and data %q, got %v", tc.expected, val.Kind(), tc.data, set)
		}
	}
}
