package main

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcExample "gofr.dev/examples/grpc-server/grpc"
	"gofr.dev/pkg/gofr/testutil"
)

func TestGRPCServer(t *testing.T) {
	gRPCPort := testutil.GetFreePort(t)
	t.Setenv("GRPC_PORT", strconv.Itoa(gRPCPort))
	host := fmt.Sprint("localhost:", gRPCPort)

	port := testutil.GetFreePort(t)
	t.Setenv("METRICS_PORT", strconv.Itoa(port))

	go main()
	time.Sleep(100 * time.Millisecond)

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
	_, err = client.SayHello(ctx, &grpcExample.HelloRequest{Name: "Test"})
	assert.Equal(t, "rpc error: code = Canceled desc = context canceled", err.Error())
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
