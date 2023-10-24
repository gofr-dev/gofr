package log

import (
	"reflect"
	"testing"
)

func TestGetLevel(t *testing.T) {
	testcases := []struct {
		input  string
		output level
	}{
		{"INFO", Info},
		{"WARN", Warn},
		{"ERROR", Error},
		{"FATAL", Fatal},
		{"DEBUG", Debug},
		{"test", Info},
	}

	for i, v := range testcases {
		resp := getLevel(v.input)

		if resp != v.output {
			t.Errorf("[TEST CASE %d]Failed. Expected %v\tGot %v\n", i+1, v.output, resp)
		}
	}
}
func TestLevel_String(t *testing.T) {
	testcases := []struct {
		input  level
		output string
	}{
		{Info, "INFO"},
		{Warn, "WARN"},
		{Error, "ERROR"},
		{Fatal, "FATAL"},
		{Debug, "DEBUG"},
	}

	for i, v := range testcases {
		resp := v.input.String()

		if resp != v.output {
			t.Errorf("[TEST CASE %d]Failed. Expected %v\tGot %v\n", i+1, v.output, resp)
		}
	}
}

func TestLevel_MarshalJSON(t *testing.T) {
	testcases := []struct {
		input  level
		output []byte
	}{
		{Info, []byte(`"INFO"`)},
		{Warn, []byte(`"WARN"`)},
		{Error, []byte(`"ERROR"`)},
		{Fatal, []byte(`"FATAL"`)},
		{Debug, []byte(`"DEBUG"`)},
	}

	for i, v := range testcases {
		resp, _ := v.input.MarshalJSON()

		if !reflect.DeepEqual(resp, v.output) {
			t.Errorf("[TEST CASE %d]Failed. Expected %s\tGot %s\n", i+1, v.output, resp)
		}
	}
}
