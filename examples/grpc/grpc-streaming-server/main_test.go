package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"gofr.dev/examples/grpc/grpc-unary-server/server"
	"google.golang.org/grpc"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")

	go main()
	time.Sleep(300 * time.Millisecond) // wait for server to boot

	os.Exit(m.Run())
}

func TestServerStream(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.Dial("localhost:9000", grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	client := server.NewChatServiceClient(conn)
	stream, err := client.ServerStream(ctx, &server.Request{Message: "Hello"})
	if err != nil {
		t.Fatalf("ServerStream failed: %v", err)
	}

	count := 0
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Error receiving: %v", err)
		}
		expected := fmt.Sprintf("Server stream %d: Hello", count)
		if resp.GetMessage() != expected {
			t.Errorf("Unexpected message: got %q, want %q", resp.GetMessage(), expected)
		}
		count++
	}
	if count != 5 {
		t.Errorf("Expected 5 messages, got %d", count)
	}
}

func TestClientStream(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.Dial("localhost:9000", grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	client := server.NewChatServiceClient(conn)
	stream, err := client.ClientStream(ctx)
	if err != nil {
		t.Fatalf("ClientStream failed: %v", err)
	}

	messages := []string{"Hello", "from", "client"}
	for _, msg := range messages {
		if err := stream.Send(&server.Request{Message: msg}); err != nil {
			t.Fatalf("Send failed: %v", err)
		}
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("CloseAndRecv failed: %v", err)
	}

	expected := "Received 3 messages. Final: Hello from client "
	if resp.GetMessage() != expected {
		t.Errorf("Unexpected response: got %q, want %q", resp.GetMessage(), expected)
	}
}

func TestBiDiStream(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.Dial("localhost:9000", grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	client := server.NewChatServiceClient(conn)
	stream, err := client.BiDiStream(ctx)
	if err != nil {
		t.Fatalf("BiDiStream failed: %v", err)
	}

	messages := []string{"msg1", "msg2", "msg3"}
	go func() {
		for _, msg := range messages {
			_ = stream.Send(&server.Request{Message: msg})
			time.Sleep(100 * time.Millisecond)
		}
		_ = stream.CloseSend()
	}()

	var responses []string
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv failed: %v", err)
		}
		responses = append(responses, resp.GetMessage())
	}

	for i, msg := range messages {
		expected := "Echo: " + msg
		if strings.TrimSpace(responses[i]) != expected {
			t.Errorf("Unexpected response: got %q, want %q", responses[i], expected)
		}
	}
}
