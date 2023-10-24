package request

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestCMD_parseArgs(t *testing.T) {
	testCases := []struct {
		argString string
		key       string
		expected  string
	}{
		{"abc -t", "t", "true"},
		{"abc -t", "abc", ""},

		{"abc -t -b=c", "t", "true"},
		{"abc -t -b=c", "abc", ""},
		{"abc -t -b=c", "b", "c"},

		{"abc -t -b=c=d", "abc", ""},
		{"abc -t -b=c=d", "t", "true"},
		{"abc -t -b=c=d", "b", ""},
		{"abc -t -b=c=d", "c", ""},
	}

	for _, tc := range testCases {
		args := strings.Split(tc.argString, " ")
		c := &CMD{}
		c.parseArgs(args)

		if value := c.Param(tc.key); value != tc.expected {
			t.Errorf("CMD Parsing error for arg: %s Expected: %s Got: %s %v", tc.argString, tc.expected, value, c.params)
		}
	}
}

func TestCMD_Param(t *testing.T) {
	c := new(CMD)
	expected := "val"
	sample := map[string]string{"key": expected}
	c.params = sample

	got := c.Param("key")
	if got != expected {
		t.Errorf("FAILED, expected: %v, got: %v", expected, got)
	}
}

func TestCMD_Header(t *testing.T) {
	c := new(CMD)
	expected := "value"
	sample := map[string]string{"key": expected}
	c.params = sample

	got := c.Header("key")
	if got != expected {
		t.Errorf("FAILED, expected: %v, got: %v", expected, got)
	}
}

func TestCMD_PathParam(t *testing.T) {
	c := new(CMD)
	expected := "value"
	sample := map[string]string{"key": expected}
	c.params = sample

	got := c.PathParam("key")
	if got != expected {
		t.Errorf("FAILED, expected: %v, got: %v", expected, got)
	}
}

func TestCMD_BindStrict(t *testing.T) {
	var s struct {
		Z int
		O string
		P bool
	}

	c := new(CMD)
	c.params = map[string]string{"Z": "100", "O": "abcd", "P": "true"}

	if gotErr := c.BindStrict(&s); gotErr != nil {
		t.Errorf("Binding failed. error: %s", gotErr)
	}

	if s.Z != 100 {
		t.Errorf("Binding of int type failed.")
	}

	if s.O != "abcd" {
		t.Errorf("Binding of string type failed.")
	}

	if s.P != true {
		t.Errorf("Binding of bool type failed.")
	}
}

func TestCMD_Params(t *testing.T) {
	c := new(CMD)
	c.params = map[string]string{"A": "10", "B": "abc", "C": "true", "D": "false"}

	got := c.Params()
	expected := c.params

	if !reflect.DeepEqual(expected, got) {
		t.Errorf("FAILED, Expected: %v, Got: %v", expected, got)
	}
}

func TestCMD_Request(t *testing.T) {
	var (
		c        = new(CMD)
		expected *http.Request
	)

	got := c.Request()
	if got != expected {
		t.Errorf("FAILED, Expected: %v, Got: %v", expected, got)
	}
}
