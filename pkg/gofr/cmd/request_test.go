package cmd

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequest_Bind(t *testing.T) {
	// TODO: Only fields starting with Capital letter can be 'bind' right now.
	r := NewRequest([]string{"command", "-Name=gofr", "-Valid=true", "-Value=12", "-test", "--name=Gofr", ""})

	assert.Equal(t, "gofr", r.Param("Name"), "TEST Failed.\n Unable to read param from request")

	assert.Equal(t, "true", r.Param("test"), "TEST Failed.\n Unable to read param from request")

	assert.Equal(t, "12", r.PathParam("Value"), "TEST Failed.\n Unable to read param from request")

	assert.Equal(t, "Gofr", r.PathParam("name"), "TEST Failed.\n Unable to read param from request")

	// Testing string, bool, int
	a := struct {
		Name  string
		Valid bool
		Value int
	}{}

	_ = r.Bind(&a)

	if a.Name != "gofr" || !a.Valid || a.Value != 12 {
		t.Errorf("TEST Failed.\nGot: %v\n%s", a, "Request Bind error")
	}

	hostName := r.HostName()

	ctx := r.Context()

	osHostName, _ := os.Hostname()

	assert.Equal(t, context.Background(), ctx, "TEST Failed.\n context is not context.Background.")

	assert.Equal(t, osHostName, hostName, "TEST Failed.\n Hostname did not match.")
}

func TestRequest_WithOneArg(t *testing.T) {
	r := NewRequest([]string{"-"})

	req := &Request{
		flags:  make(map[string]bool),
		params: make(map[string]string),
	}

	assert.Equal(t, req, r, "TEST Failed.\n Hostname did not match.")
}

func TestHostName(t *testing.T) {
	r := &Request{}

	// Get the hostname using os.Hostname()
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatalf("Error getting hostname: %v", err)
	}

	// Get the hostname from the mock request
	result := r.HostName()

	assert.Equal(t, hostname, result, "TestHostName Failed!")
}
