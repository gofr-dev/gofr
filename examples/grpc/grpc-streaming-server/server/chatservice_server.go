// versions:
// 	gofr-cli v0.6.0
// 	gofr.dev v1.37.0
// 	source: chat.proto

package server

import (
	"fmt"
	"gofr.dev/pkg/gofr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"strings"
	"time"
)

// Register the gRPC service in your app using the following code in your main.go:
//
// server.RegisterChatServiceServerWithGofr(app, &server.NewChatServiceGoFrServer())
//
// ChatServiceGoFrServer defines the gRPC server implementation.
// Customize the struct with required dependencies and fields as needed.

type ChatServiceGoFrServer struct {
	health *healthServer
}

func (s *ChatServiceGoFrServer) ServerStream(ctx *gofr.Context, stream ChatService_ServerStreamServer) error {
	req := Request{}
	err := ctx.Bind(&req)
	if err != nil {
		return err
	}

	for i := 0; i < 5; i++ {
		resp := &Response{Message: fmt.Sprintf("Server stream %d: %s", i, req.Message)}
		if err := stream.Send(resp); err != nil {
			return status.Errorf(codes.Internal, "error sending stream: %v", err)
		}
		time.Sleep(1 * time.Second) // Simulate processing delay
	}
	return nil
}

func (s *ChatServiceGoFrServer) ClientStream(ctx *gofr.Context, stream ChatService_ClientStreamServer) error {
	var messageCount int
	var finalMessage strings.Builder

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&Response{
				Message: fmt.Sprintf("Received %d messages. Final: %s", messageCount, finalMessage.String()),
			})
		}
		if err != nil {
			return err
		}

		messageCount++
		finalMessage.WriteString(req.Message + " ")
	}
}

func (s *ChatServiceGoFrServer) BiDiStream(ctx *gofr.Context, stream ChatService_BiDiStreamServer) error {
	// Handle incoming messages in a goroutine
	errChan := make(chan error)

	go func() {
		for {
			req, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				errChan <- err
				return
			}

			// Process request and send response
			resp := &Response{Message: "Echo: " + req.Message}
			if err := stream.Send(resp); err != nil {
				errChan <- err
				return
			}
		}
		errChan <- nil
	}()

	// Wait for completion
	select {
	case err := <-errChan:
		return err
	case <-stream.Context().Done():
		return status.Error(codes.Canceled, "client disconnected")
	}
}
