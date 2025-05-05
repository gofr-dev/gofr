package main

import (
	"errors"
	"fmt"
	"gofr.dev/examples/grpc/grpc-client/client"
	"gofr.dev/pkg/gofr"
	"io"
)

func main() {
	app := gofr.New()

	//Create a gRPC client for the Hello service
	helloGRPCClient, err := client.NewHelloGoFrClient(app.Config.Get("GRPC_SERVER_HOST"), app.Metrics())
	if err != nil {
		app.Logger().Errorf("Failed to create Hello gRPC client: %v", err)
		return
	}

	chatClient, err := client.NewChatGoFrClient(app.Config.Get("GRPC_SERVER_HOST"), app.Metrics())
	if err != nil {
		app.Logger().Errorf("Failed to create Chat client: %v", err)
	}

	greet := NewGreetHandler(helloGRPCClient)
	chat := NewChatHandler(chatClient)

	app.GET("/hello", greet.Hello)
	app.GET("/chat/server-stream", chat.ServerStreamHandler)
	app.POST("/chat/client-stream", chat.ClientStreamHandler)
	app.GET("/chat/bidi-stream", chat.BiDiStreamHandler)

	app.Run()
}

type GreetHandler struct {
	helloGRPCClient client.HelloGoFrClient
}

func NewGreetHandler(helloClient client.HelloGoFrClient) *GreetHandler {
	return &GreetHandler{
		helloGRPCClient: helloClient,
	}
}

func (g GreetHandler) Hello(ctx *gofr.Context) (interface{}, error) {
	userName := ctx.Param("name")

	if userName == "" {
		ctx.Log("Name parameter is empty, defaulting to 'World'")
		userName = "World"
	}

	//HealthCheck to SayHello Service.
	//res, err := g.helloGRPCClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{Service: "Hello"})
	//if err != nil {
	//	return nil, err
	//} else if res.Status == grpc_health_v1.HealthCheckResponse_NOT_SERVING {
	//	ctx.Error("Hello Service is down")
	//	return nil, fmt.Errorf("Hello Service is down")
	//}

	// Make a gRPC call to the Hello service
	helloResponse, err := g.helloGRPCClient.SayHello(ctx, &client.HelloRequest{Name: userName})
	if err != nil {
		return nil, err
	}

	return helloResponse, nil
}

// Add ChatHandler struct and methods
type ChatHandler struct {
	chatClient client.ChatGoFrClient
}

func NewChatHandler(chatClient client.ChatGoFrClient) *ChatHandler {
	return &ChatHandler{chatClient: chatClient}
}

func (c *ChatHandler) ServerStreamHandler(ctx *gofr.Context) (interface{}, error) {
	stream, err := c.chatClient.ServerStream(ctx, &client.Request{Message: "stream request"})
	if err != nil {
		return nil, err
	}

	// Handle server streaming
	for {
		res, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		ctx.Logger.Info(res.Message)
	}

	return "Server stream completed", nil
}

func (c *ChatHandler) ClientStreamHandler(ctx *gofr.Context) (interface{}, error) {
	// Get client streaming interface
	stream, err := c.chatClient.ClientStream(ctx)
	if err != nil {
		return nil, err
	}

	// Example: Read multiple messages from request body
	var requests []*client.Request
	if err := ctx.Bind(&requests); err != nil {
		return nil, err
	}

	// Send multiple messages to server
	for _, req := range requests {
		if err := stream.Send(req); err != nil {
			return nil, fmt.Errorf("failed to send request: %v", err)
		}
	}

	// Close the stream and get final response
	response, err := stream.CloseAndRecv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive response: %v", err)
	}

	return response.Message, nil
}

func (c *ChatHandler) BiDiStreamHandler(ctx *gofr.Context) (interface{}, error) {
	// Create bidirectional stream
	stream, err := c.chatClient.BiDiStream(ctx)
	if err != nil {
		return nil, err
	}

	// Channel to collect responses
	respChan := make(chan string)
	errChan := make(chan error)

	// Receive messages from server in goroutine
	go func() {
		for {
			res, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					errChan <- nil
					return
				}
				errChan <- err
				return
			}
			respChan <- res.Message
		}
	}()

	// Send multiple messages to server
	messages := []string{"message 1", "message 2", "message 3"}
	for _, msg := range messages {
		if err := stream.Send(&client.Request{Message: msg}); err != nil {
			return nil, fmt.Errorf("failed to send message: %v", err)
		}
	}

	// Close sending side
	if err := stream.CloseSend(); err != nil {
		return nil, fmt.Errorf("failed to close send: %v", err)
	}

	// Collect responses
	var responses []string
	done := false
	for !done {
		select {
		case err := <-errChan:
			if err != nil {
				return nil, err
			}
			done = true
		case msg := <-respChan:
			responses = append(responses, msg)
			ctx.Logger.Info("Received:", msg)
		}
	}

	return map[string]interface{}{
		"sent_messages":     messages,
		"received_messages": responses,
	}, nil
}
