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
