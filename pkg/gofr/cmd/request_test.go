package cmd

import "testing"

func TestRequest_Bind(t *testing.T) {
	// TODO: Only fields starting with Capital letter can be 'bind' right now.
	r := NewRequest([]string{"command", "-Name=gofr", "-Valid=true", "-Value=12", "-test"})

	if r.Param("Name") != "gofr" {
		t.Error("Param parse error.")
	}

	if r.Param("test") != "true" {
		t.Error("Param parse error.")
	}

	if r.PathParam("Value") != "12" {
		t.Error("PathParam error.")
	}

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
