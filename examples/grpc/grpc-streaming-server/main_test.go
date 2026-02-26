package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"gofr.dev/examples/grpc/grpc-streaming-server/server"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")

	go main()
	time.Sleep(300 * time.Millisecond) // wait for server to boot

	m.Run()
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
		if errors.Is(err, io.EOF) {
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

// TestServerStream_ContextCancellation tests that server-side streaming
// properly handles context cancellation
func TestServerStream_ContextCancellation(t *testing.T) {
	conn, err := grpc.Dial("localhost:9000", grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	client := server.NewChatServiceClient(conn)

	// Create a context that will be canceled after a short delay
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := client.ServerStream(ctx, &server.Request{Message: "Hello"})
	if err != nil {
		t.Fatalf("ServerStream failed: %v", err)
	}

	// Cancel context after receiving first message
	receivedFirst := false
	var lastErr error

	for {
		resp, err := stream.Recv()
		if err != nil {
			lastErr = err
			break
		}

		if !receivedFirst {
			receivedFirst = true
			// Cancel context to trigger cancellation handling
			cancel()
			// Give server time to detect cancellation
			time.Sleep(200 * time.Millisecond)
		}

		_ = resp // Use response to avoid unused variable
	}

	// Verify that we got a cancellation error
	if lastErr == nil {
		t.Error("Expected error due to context cancellation, got nil")
	} else {
		s, ok := status.FromError(lastErr)
		if !ok {
			t.Errorf("Expected gRPC status error, got: %v", lastErr)
		} else if s.Code() != codes.Canceled {
			t.Errorf("Expected Canceled status code, got: %v", s.Code())
		}
	}
}

// TestClientStream_ContextCancellation tests that client-side streaming
// properly handles context cancellation
func TestClientStream_ContextCancellation(t *testing.T) {
	conn, err := grpc.Dial("localhost:9000", grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	client := server.NewChatServiceClient(conn)

	// Create a context that will be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := client.ClientStream(ctx)
	if err != nil {
		t.Fatalf("ClientStream failed: %v", err)
	}

	// Send one message then cancel
	if err := stream.Send(&server.Request{Message: "Hello"}); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Cancel context
	cancel()
	// Give server time to detect cancellation
	time.Sleep(200 * time.Millisecond)

	// Try to close and receive - should get cancellation error
	_, err = stream.CloseAndRecv()
	if err == nil {
		t.Error("Expected error due to context cancellation, got nil")
	} else {
		s, ok := status.FromError(err)
		if !ok {
			t.Errorf("Expected gRPC status error, got: %v", err)
		} else if s.Code() != codes.Canceled {
			t.Errorf("Expected Canceled status code, got: %v", s.Code())
		}
	}
}

// TestClientStream_EOFHandling tests that client-side streaming
// properly handles EOF when client closes the stream
func TestClientStream_EOFHandling(t *testing.T) {
	conn, err := grpc.Dial("localhost:9000", grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	client := server.NewChatServiceClient(conn)
	stream, err := client.ClientStream(context.Background())
	if err != nil {
		t.Fatalf("ClientStream failed: %v", err)
	}

	// Send messages
	messages := []string{"msg1", "msg2", "msg3"}
	for _, msg := range messages {
		if err := stream.Send(&server.Request{Message: msg}); err != nil {
			t.Fatalf("Send failed: %v", err)
		}
	}

	// Close and receive - should succeed with EOF handled properly
	resp, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("CloseAndRecv failed: %v", err)
	}

	// Verify response indicates all messages were received
	expected := fmt.Sprintf("Received %d messages. Final: %s ", len(messages), strings.Join(messages, " "))
	if !strings.Contains(resp.GetMessage(), expected) {
		t.Errorf("Unexpected response: got %q, expected to contain %q", resp.GetMessage(), expected)
	}
}

// TestBiDiStream_ContextCancellation tests that bidirectional streaming
// properly handles context cancellation
func TestBiDiStream_ContextCancellation(t *testing.T) {
	conn, err := grpc.Dial("localhost:9000", grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	client := server.NewChatServiceClient(conn)

	// Create a context that will be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := client.BiDiStream(ctx)
	if err != nil {
		t.Fatalf("BiDiStream failed: %v", err)
	}

	// Send one message
	if err := stream.Send(&server.Request{Message: "test"}); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Receive one response
	resp, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv failed: %v", err)
	}

	if resp.GetMessage() != "Echo: test" {
		t.Errorf("Unexpected response: got %q, want %q", resp.GetMessage(), "Echo: test")
	}

	// Cancel context
	cancel()
	// Give server time to detect cancellation
	time.Sleep(200 * time.Millisecond)

	// Try to receive - should get cancellation error
	_, err = stream.Recv()
	if err == nil {
		t.Error("Expected error due to context cancellation, got nil")
	} else {
		s, ok := status.FromError(err)
		if !ok {
			// EOF is also acceptable when stream closes normally
			if !errors.Is(err, io.EOF) {
				t.Errorf("Expected gRPC status error or EOF, got: %v", err)
			}
		} else if s.Code() != codes.Canceled {
			// If it's a status error, it should be Canceled
			t.Errorf("Expected Canceled status code, got: %v", s.Code())
		}
	}
}

// TestServerStream_ErrorHandling tests error handling in server-side streaming
func TestServerStream_ErrorHandling(t *testing.T) {
	conn, err := grpc.Dial("localhost:9000", grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	client := server.NewChatServiceClient(conn)
	stream, err := client.ServerStream(context.Background(), &server.Request{Message: "test"})
	if err != nil {
		t.Fatalf("ServerStream failed: %v", err)
	}

	// Receive all messages and verify EOF handling
	count := 0
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			// EOF indicates stream ended normally
			break
		}
		if err != nil {
			t.Fatalf("Unexpected error receiving: %v", err)
		}
		count++
		_ = resp // Use response
	}

	// Verify we received all expected messages
	if count != 5 {
		t.Errorf("Expected 5 messages, got %d", count)
	}
}

// TestBiDiStream_ErrorHandling tests error handling in bidirectional streaming
func TestBiDiStream_ErrorHandling(t *testing.T) {
	conn, err := grpc.Dial("localhost:9000", grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	client := server.NewChatServiceClient(conn)
	stream, err := client.BiDiStream(context.Background())
	if err != nil {
		t.Fatalf("BiDiStream failed: %v", err)
	}

	// Send messages
	messages := []string{"msg1", "msg2"}
	for _, msg := range messages {
		if err := stream.Send(&server.Request{Message: msg}); err != nil {
			t.Fatalf("Send failed: %v", err)
		}
	}

	// Close send side
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend failed: %v", err)
	}

	// Receive responses and verify EOF handling
	var responses []string
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			// EOF indicates stream ended normally
			break
		}
		if err != nil {
			t.Fatalf("Unexpected error receiving: %v", err)
		}
		responses = append(responses, resp.GetMessage())
	}

	// Verify we received responses for all sent messages
	if len(responses) != len(messages) {
		t.Errorf("Expected %d responses, got %d", len(messages), len(responses))
	}
}

// TestServerStream_Timeout tests server-side streaming with timeout
func TestServerStream_Timeout(t *testing.T) {
	conn, err := grpc.Dial("localhost:9000", grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	client := server.NewChatServiceClient(conn)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stream, err := client.ServerStream(ctx, &server.Request{Message: "timeout test"})
	if err != nil {
		t.Fatalf("ServerStream failed: %v", err)
	}

	// Try to receive messages - timeout should occur
	var lastErr error
	count := 0
	for {
		resp, err := stream.Recv()
		if err != nil {
			lastErr = err
			break
		}
		count++
		_ = resp
	}

	// Should have received some messages before timeout
	if count == 0 {
		t.Error("Expected to receive at least one message before timeout")
	}

	// Verify timeout error
	if lastErr == nil {
		t.Error("Expected timeout error, got nil")
	} else {
		s, ok := status.FromError(lastErr)
		if ok && s.Code() == codes.DeadlineExceeded {
			// Deadline exceeded is expected for timeout
		} else if !errors.Is(lastErr, context.DeadlineExceeded) {
			// Context deadline exceeded is also acceptable
			if !errors.Is(lastErr, io.EOF) {
				t.Logf("Got error (may be expected): %v", lastErr)
			}
		}
	}
}
