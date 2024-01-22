package cmd

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRequest_Bind(t *testing.T) {
	// TODO: Only fields starting with Capital letter can be 'bind' right now.
	r := NewRequest([]string{"command", "-Name=gofr", "-Valid=true", "-Value=12", "-test", "--name=Gofr"})

	assert.Equal(t, "gofr", r.Param("Name"))

	assert.Equal(t, "true", r.Param("test"))

	assert.Equal(t, "12", r.Param("Value"))

	assert.Equal(t, "Gofr", r.Param("name"))

	// Testing string, bool, int
	a := struct {
		Name  string
		Valid bool
		Value int
	}{}

	_ = r.Bind(&a)

	if a.Name != "gofr" || a.Valid != true || a.Value != 12 {
		t.Errorf("1. Request Bind error. Got: %v", a)
	}
}
