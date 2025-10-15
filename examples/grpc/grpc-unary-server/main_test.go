package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/examples/grpc/grpc-unary-server/server"
)

func TestHelloProtoMethods(t *testing.T) {
	// Test HelloRequest methods
	req := &server.HelloRequest{Name: "John"}
	assert.Equal(t, "John", req.GetName())
	assert.Equal(t, "name:\"John\"", req.String())

	// Test HelloResponse methods
	resp := &server.HelloResponse{Message: "Hello World"}
	assert.Equal(t, "Hello World", resp.GetMessage())
	assert.Equal(t, "message:\"Hello World\"", resp.String())
}
