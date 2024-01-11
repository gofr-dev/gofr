package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequest_Bind(t *testing.T) {
	// TODO: Only fields starting with Capital letter can be 'bind' right now.
	r := NewRequest([]string{"command", "-Name=gofr", "-Valid=true", "-Value=12", "-test", "--name=Gofr", ""})

	// Testing string, bool, int
	a := struct {
		Name  string
		Valid bool
		Value int
	}{}

	err := r.Bind(&a)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if a.Name != "gofr" || a.Valid != true || a.Value != 12 {
		t.Errorf("TEST Failed.\nGot: %v\n%s", a, "Request Bind error")
	}
}

func TestRequest_Param(t *testing.T) {
	r := NewRequest([]string{"-name=gofr", "-valid=true", "-value=12", "-test"})

	testCases := []struct {
		param string
		want  string
	}{
		{"name", "gofr"},
		{"valid", "true"},
		{"value", "12"},
		{"test", "true"},
	}

	for i, tc := range testCases {
		resp := r.Param(tc.param)

		assert.Equal(t, tc.want, resp, "TEST[%d], Failed.\n", i)

		resp = r.PathParam(tc.param)

		assert.Equal(t, tc.want, resp, "TEST[%d], Failed.\n", i)
	}
}
