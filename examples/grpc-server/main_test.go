package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcExample "gofr.dev/examples/grpc-server/grpc"
)

func TestGRPCServer(t *testing.T) {
	const host = "localhost:9000"

	go main()
	time.Sleep(time.Second * 1)

	client, conn := createGRPCClient(t, host)
	defer conn.Close()

	tests := []struct {
		desc            string
		request         *grpcExample.HelloRequest
		expectedErr     error
		responseMessage string
	}{
		{"SayHello with name", &grpcExample.HelloRequest{Name: "John"},
			nil, "Hello John!"},
		{"SayHello without name", &grpcExample.HelloRequest{},
			nil, "Hello World!"},
		{"Name exceeding limit", &grpcExample.HelloRequest{Name: "This name exceeds the allowed maximum length."},
			nil, "Hello This name exceeds the allowed maximum length.!"},
	}

	for _, tc := range tests {
		resp, err := client.SayHello(context.Background(), tc.request)
		assert.NoError(t, err)
		assert.Equal(t, tc.responseMessage, resp.Message)
	}

	// case of empty request
	_, err := client.SayHello(context.Background(), nil)
	assert.Equal(t, "rpc error: code = Internal desc = grpc: error while marshaling: proto: "+
		"Marshal called with nil", err.Error())

}

func createGRPCClient(t *testing.T, host string) (grpcExample.HelloClient, *grpc.ClientConn) {
	conn, err := grpc.Dial(host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Errorf("did not connect: %s", err)
	}

	return grpcExample.NewHelloClient(conn), conn
}

func TestHelloProtoMethods(t *testing.T) {
	// Test HelloRequest methods
	req := &grpcExample.HelloRequest{Name: "John"}
	assert.Equal(t, "John", req.GetName())
	assert.Equal(t, "name:\"John\"", req.String())

	// Test HelloResponse methods
	resp := &grpcExample.HelloResponse{Message: "Hello World"}
	assert.Equal(t, "Hello World", resp.GetMessage())
	assert.Equal(t, "message:\"Hello World\"", resp.String())
}
