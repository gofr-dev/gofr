package main

import (
	"errors"
	"fmt"
	"io"
	"time"

	"gofr.dev/examples/grpc/grpc-streaming-client/client"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Create a gRPC client for the Chat Streaming service
	chatClient, err := client.NewChatServiceGoFrClient(app.Config.Get("GRPC_SERVER_HOST"), app.Metrics())
	if err != nil {
		app.Logger().Errorf("Failed to create Chat client: %v", err)
	}

	chat := NewChatHandler(chatClient)

	app.GET("/chat/server-stream", chat.ServerStreamHandler)
	app.POST("/chat/client-stream", chat.ClientStreamHandler)
	app.GET("/chat/bidi-stream", chat.BiDiStreamHandler)

	app.Run()
}

type ChatHandler struct {
	chatClient client.ChatServiceGoFrClient
}

func NewChatHandler(chatClient client.ChatServiceGoFrClient) *ChatHandler {
	return &ChatHandler{chatClient: chatClient}
}

type StreamResponse struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Direction string    `json:"direction"` // "received" or "sent"
}

// ServerStreamHandler handles server-side streaming with detailed response tracking
func (c *ChatHandler) ServerStreamHandler(ctx *gofr.Context) (any, error) {
	startTime := time.Now()
	var responses []StreamResponse

	// Record initial request
	responses = append(responses, StreamResponse{
		Message:   "initiating server stream request",
		Timestamp: startTime,
		Direction: "sent",
	})

	stream, err := c.chatClient.ServerStream(ctx, &client.Request{Message: "stream request"})
	if err != nil {
		return nil, fmt.Errorf("failed to initiate server stream: %v", err)
	}

	// Handle server streaming
	for {
		res, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("stream receive error: %v", err)
		}

		// Record received message
		response := StreamResponse{
			Message:   res.Message,
			Timestamp: time.Now(),
			Direction: "received",
		}
		responses = append(responses, response)
		ctx.Logger.Infof("Received server stream message: %s at %v", res.Message, response.Timestamp)
	}

	// Return detailed stream information
	return map[string]any{
		"status":          "server stream completed",
		"start_time":      startTime,
		"end_time":        time.Now(),
		"duration_sec":    time.Since(startTime).Seconds(),
		"stream_messages": responses,
	}, nil
}

// ClientStreamHandler handles client-side streaming with detailed tracking
func (c *ChatHandler) ClientStreamHandler(ctx *gofr.Context) (any, error) {
	startTime := time.Now()
	var streamLog []StreamResponse

	// Get client streaming interface
	stream, err := c.chatClient.ClientStream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate client stream: %v", err)
	}

	// Example: Read multiple messages from request body
	var requests []*client.Request
	if err := ctx.Bind(&requests); err != nil {
		return nil, fmt.Errorf("failed to bind requests: %v", err)
	}

	// Send multiple messages to server and log each one
	for i, req := range requests {
		sendTime := time.Now()
		if err := stream.Send(req); err != nil {
			return nil, fmt.Errorf("failed to send request %d: %v", i+1, err)
		}

		streamLog = append(streamLog, StreamResponse{
			Message:   req.Message,
			Timestamp: sendTime,
			Direction: "sent",
		})
		ctx.Logger.Infof("Sent client stream message %d: %s at %v", i+1, req.Message, sendTime)
	}

	// Close the stream and get final response
	response, err := stream.CloseAndRecv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive final response: %v", err)
	}

	// Record final response
	streamLog = append(streamLog, StreamResponse{
		Message:   response.Message,
		Timestamp: time.Now(),
		Direction: "received",
	})

	return map[string]any{
		"final_response": response.Message,
		"start_time":     startTime,
		"end_time":       time.Now(),
		"duration_sec":   time.Since(startTime).Seconds(),
		"stream_log":     streamLog,
	}, nil
}

// BiDiStreamHandler handles bidirectional streaming with detailed tracking
func (c *ChatHandler) BiDiStreamHandler(ctx *gofr.Context) (any, error) {
	startTime := time.Now()
	streamLog := make([]StreamResponse, 0)

	stream, err := c.chatClient.BiDiStream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate bidirectional stream: %v", err)
	}

	respChan, errChan := make(chan StreamResponse), make(chan error)
	go c.receiveBiDiResponses(ctx, stream, respChan, errChan)

	sentMessages, err := c.sendBiDiMessages(ctx, stream, &streamLog)
	if err != nil {
		return nil, err
	}

	if err := stream.CloseSend(); err != nil {
		return nil, fmt.Errorf("failed to close send: %v", err)
	}

	receivedMessages, err := c.collectBiDiResponses(respChan, errChan, &streamLog)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"status":            "bidirectional stream completed",
		"start_time":        startTime,
		"end_time":          time.Now(),
		"duration_sec":      time.Since(startTime).Seconds(),
		"sent_messages":     sentMessages,
		"received_messages": receivedMessages,
		"detailed_log":      streamLog,
	}, nil
}

// receiveBiDiResponses receives messages in a goroutine
func (c *ChatHandler) receiveBiDiResponses(ctx *gofr.Context, stream client.ChatService_BiDiStreamClient, respChan chan<- StreamResponse, errChan chan<- error) {
	for {
		res, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				errChan <- nil
			} else {
				errChan <- fmt.Errorf("receive error: %v", err)
			}
			return
		}
		timestamp := time.Now()
		ctx.Logger.Infof("Received bidirectional message: %s at %v", res.Message, timestamp)
		respChan <- StreamResponse{
			Message:   res.Message,
			Timestamp: timestamp,
			Direction: "received",
		}
	}
}

// sendBiDiMessages sends predefined messages
func (c *ChatHandler) sendBiDiMessages(ctx *gofr.Context, stream client.ChatService_BiDiStreamClient, streamLog *[]StreamResponse) ([]string, error) {
	messages := []string{"message 1", "message 2", "message 3"}

	for _, msg := range messages {
		timestamp := time.Now()
		if err := stream.Send(&client.Request{Message: msg}); err != nil {
			return nil, fmt.Errorf("failed to send message %q: %v", msg, err)
		}
		ctx.Logger.Infof("Sent bidirectional message: %s at %v", msg, timestamp)
		*streamLog = append(*streamLog, StreamResponse{
			Message:   msg,
			Timestamp: timestamp,
			Direction: "sent",
		})
	}

	return messages, nil
}

// collectBiDiResponses waits and aggregates received responses
func (c *ChatHandler) collectBiDiResponses(respChan <-chan StreamResponse, errChan <-chan error, streamLog *[]StreamResponse) ([]string, error) {
	var received []string

	timeout := time.After(5 * time.Second)
	for {
		select {
		case err := <-errChan:
			return received, err
		case resp := <-respChan:
			received = append(received, resp.Message)
			*streamLog = append(*streamLog, resp)
		case <-timeout:
			return nil, errors.New("bidirectional stream timeout")
		}
	}
}
