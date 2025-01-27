package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"gofr.dev/examples/grpc/grpc-server/server"
	"gofr.dev/pkg/gofr/testutil"
)

func TestGRPCServer(t *testing.T) {
	configs := testutil.NewServerConfigs(t)
	host := configs.GRPCHost

	go main()
	time.Sleep(100 * time.Millisecond)

	client, conn := createGRPCClient(t, host)
	defer conn.Close()

	tests := []struct {
		desc            string
		request         *server.HelloRequest
		expectedErr     error
		responseMessage string
	}{
		{"SayHello with name", &server.HelloRequest{Name: "John"},
			nil, "Hello John!"},
		{"SayHello without name", &server.HelloRequest{},
			nil, "Hello World!"},
		{"Name exceeding limit", &server.HelloRequest{Name: "This name exceeds the allowed maximum length."},
			nil, "Hello This name exceeds the allowed maximum length.!"},
	}

	for _, tc := range tests {
		resp, err := client.SayHello(context.Background(), tc.request)
		require.NoError(t, err)
		assert.Equal(t, tc.responseMessage, resp.Message)
	}

	// case of empty request
	resp, err := client.SayHello(context.Background(), nil)
	assert.Equal(t, "Hello World!", resp.Message)
	require.NoError(t, err)

	// Test context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = client.SayHello(ctx, &server.HelloRequest{Name: "Test"})
	assert.Equal(t, "rpc error: code = Canceled desc = context canceled", err.Error())
}

func createGRPCClient(t *testing.T, host string) (server.HelloClient, *grpc.ClientConn) {
	conn, err := grpc.Dial(host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Errorf("did not connect: %s", err)
	}

	return server.NewHelloClient(conn), conn
}

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
