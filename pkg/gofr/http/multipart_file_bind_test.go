package http

import (
	"reflect"
	"testing"
	"unsafe"

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
		{reflect.Pointer, "corge", reflect.TypeOf((*int)(nil))},
		{reflect.UnsafePointer, "grault", reflect.TypeOf(unsafe.Pointer(nil))},
	}

	for _, tt := range tests {
		value := reflect.New(tt.typ).Elem()
		ok, err := uf.setFieldValue(value, tt.data)

		assert.False(t, ok, "expected false, got true for kind %v", tt.kind)

		assert.NoError(t, err, "expected nil, got %v for kind %v", err, tt.kind)
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
	if !ok || err != nil {
		t.Errorf("setSliceOrArrayValue failed: %v", err)
	}

	if len(value.Interface().([]string)) != 3 {
		t.Errorf("slice not set correctly")
	}

	// Test with an array
	value = reflect.ValueOf(&testStruct{Array: [3]string{}}).Elem().FieldByName("Array")

	data = "a,b,c"

	ok, err = uf.setSliceOrArrayValue(value, data)
	if !ok || err != nil {
		t.Errorf("setSliceOrArrayValue failed: %v", err)
	}
}

func TestSetStructValue(t *testing.T) {
	type testStruct struct {
		Field1 string
		Field2 int
	}

	uf := &formData{}

	// Test with a valid input string
	value := reflect.ValueOf(&testStruct{}).Elem()

	data := "{\"Field1\":\"value1\",\"Field2\":123}"

	ok, err := uf.setStructValue(value, data)
	if !ok || err != nil {
		t.Errorf("setStructValue failed: %v", err)
	}

	if value.FieldByName("Field1").String() != "value1" || value.FieldByName("Field2").Int() != 123 {
		t.Errorf("struct fields not set correctly")
	}
}
