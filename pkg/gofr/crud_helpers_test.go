package gofr

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConvertIDType_Success(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		fieldType reflect.Type
		want      any
	}{
		{"String", "abc123", reflect.TypeOf(""), "abc123"},
		{"Int", "42", reflect.TypeOf(int(0)), int(42)},
		{"Int8", "127", reflect.TypeOf(int8(0)), int8(127)},
		{"Int16", "32767", reflect.TypeOf(int16(0)), int16(32767)},
		{"Int32", "2147483647", reflect.TypeOf(int32(0)), int32(2147483647)},
		{"Int64", "9223372036854775807", reflect.TypeOf(int64(0)), int64(9223372036854775807)},
		{"Uint", "42", reflect.TypeOf(uint(0)), uint(42)},
		{"Uint8", "255", reflect.TypeOf(uint8(0)), uint8(255)},
		{"Uint16", "65535", reflect.TypeOf(uint16(0)), uint16(65535)},
		{"Uint32", "4294967295", reflect.TypeOf(uint32(0)), uint32(4294967295)},
		{"Uint64", "18446744073709551615", reflect.TypeOf(uint64(0)), uint64(18446744073709551615)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertIDType(tt.id, tt.fieldType)
			require.NoError(t, err, "Unexpected error: %v", err)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertIDType() = %v (type %T), want %v (type %T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestConvertIDType_Error(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		fieldType reflect.Type
	}{
		{"Invalid Int", "abc", reflect.TypeOf(int(0))},
		{"Invalid Uint", "-1", reflect.TypeOf(uint(0))},
		{"Unsupported Type", "42", reflect.TypeOf(float64(0))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := convertIDType(tt.id, tt.fieldType)
			if err == nil {
				t.Fatalf("Expected error but got nil")
			}
		})
	}
}
