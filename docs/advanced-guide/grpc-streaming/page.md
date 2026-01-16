# gRPC Streaming with GoFr

GoFr provides comprehensive support for gRPC streaming, enabling efficient real-time communication between services. Streaming is particularly useful for scenarios where you need to send or receive multiple messages over a single connection, such as chat applications, real-time data feeds, or large file transfers.

GoFr supports three types of gRPC streaming:
- **Server-side streaming**: The server sends multiple responses to a single client request
- **Client-side streaming**: The client sends multiple requests and receives a single response
- **Bidirectional streaming**: Both client and server can send multiple messages independently

All streaming methods in GoFr include built-in tracing, metrics, and logging support, ensuring seamless observability for your streaming operations.

## Prerequisites

Before implementing gRPC streaming, ensure you have:

1. **Protocol Buffer Compiler (`protoc`)** installed (version 3+)
2. **Go gRPC plugins** installed:
   ```bash
   go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2
   export PATH="$PATH:$(go env GOPATH)/bin"
   ```
3. **gofr-cli** installed:
   ```bash
   go install gofr.dev/cli/gofr@latest
   ```

For detailed setup instructions, refer to the [gRPC with GoFr documentation](https://gofr.dev/docs/advanced-guide/grpc).

## Defining Streaming RPCs in Protocol Buffers

To use streaming in your gRPC service, define your RPC methods with the `stream` keyword in your `.proto` file:

```protobuf
syntax = "proto3";
option go_package = "path/to/your/proto/file";

message Request {
  string message = 1;
}

message Response {
  string message = 1;
}

service ChatService {
  // Server-side streaming: client sends one request, server sends multiple responses
  rpc ServerStream(Request) returns (stream Response);
  
  // Client-side streaming: client sends multiple requests, server sends one response
  rpc ClientStream(stream Request) returns (Response);
  
  // Bidirectional streaming: both client and server can send multiple messages
  rpc BiDiStream(stream Request) returns (stream Response);
}
```

## Generating gRPC Streaming Server Code

GoFr CLI automatically generates streaming-aware server templates. Use the `gofr wrap grpc server` command:

```bash
gofr wrap grpc server -proto=./path/to/your/proto/file
```

This command generates:
- `<SERVICE_NAME>_server.go`: Template file with streaming method signatures
- `<SERVICE_NAME>_gofr.go`: Generated wrapper with streaming instrumentation
- `request_gofr.go`: Request wrapper for context binding
- `health_gofr.go`: Health check server integration

### Server-Side Streaming Implementation

Server-side streaming allows the server to send multiple responses to a single client request. This is useful for scenarios like real-time notifications or progressive data delivery.

**Example Implementation:**

```go
func (s *ChatServiceGoFrServer) ServerStream(ctx *gofr.Context, stream ChatService_ServerStreamServer) error {
    // Bind the initial request
    req := Request{}
    if err := ctx.Bind(&req); err != nil {
        return status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
    }

    // Send multiple responses
    for i := 0; i < 5; i++ {
        // Check if context is canceled
        select {
        case <-stream.Context().Done():
            return status.Error(codes.Canceled, "client disconnected")
        default:
        }

        resp := &Response{
            Message: fmt.Sprintf("Server stream %d: %s", i, req.Message),
        }
        
        if err := stream.Send(resp); err != nil {
            return status.Errorf(codes.Internal, "error sending stream: %v", err)
        }
        
        time.Sleep(1 * time.Second) // Simulate processing delay
    }
    
    return nil
}
```

**Key Points:**
- Use `ctx.Bind()` to extract the initial request
- Return appropriate gRPC status codes for binding errors
- Check for context cancellation before each send operation
- Call `stream.Send()` to send each response message
- Return `nil` when streaming is complete, or an error if something goes wrong

### Client-Side Streaming Implementation

Client-side streaming allows the client to send multiple requests before receiving a single response. This is useful for batch processing or aggregating data from the client.

**Example Implementation:**

```go
func (s *ChatServiceGoFrServer) ClientStream(ctx *gofr.Context, stream ChatService_ClientStreamServer) error {
    var messageCount int
    var finalMessage strings.Builder

    // Receive multiple messages from client
    for {
        // Check if context is canceled before receiving
        select {
        case <-stream.Context().Done():
            return status.Error(codes.Canceled, "client disconnected")
        default:
        }

        req, err := stream.Recv()
        if err == io.EOF {
            // Client has finished sending, send final response
            return stream.SendAndClose(&Response{
                Message: fmt.Sprintf("Received %d messages. Final: %s", 
                    messageCount, finalMessage.String()),
            })
        }
        if err != nil {
            return status.Errorf(codes.Internal, "error receiving stream: %v", err)
        }

        // Process each message
        messageCount++
        finalMessage.WriteString(req.Message + " ")
    }
}
```

**Key Points:**
- Check for context cancellation before each receive operation
- Use `stream.Recv()` in a loop to receive messages
- Check for `io.EOF` to detect when the client has finished sending
- Return appropriate gRPC status codes for receive errors
- Call `stream.SendAndClose()` to send the final response and close the stream
- Process each message as it arrives

### Bidirectional Streaming Implementation

Bidirectional streaming allows both client and server to send messages independently. This is useful for real-time chat applications or interactive protocols.

**Example Implementation:**

```go
func (s *ChatServiceGoFrServer) BiDiStream(ctx *gofr.Context, stream ChatService_BiDiStreamServer) error {
    errChan := make(chan error)

    // Handle incoming messages in a goroutine
    go func() {
        for {
            // Check if context is canceled
            select {
            case <-stream.Context().Done():
                errChan <- status.Error(codes.Canceled, "client disconnected")
                return
            default:
            }

            req, err := stream.Recv()
            if err == io.EOF {
                break
            }
            if err != nil {
                errChan <- status.Errorf(codes.Internal, "error receiving stream: %v", err)
                return
            }

            // Process request and send response
            resp := &Response{Message: "Echo: " + req.Message}
            if err := stream.Send(resp); err != nil {
                errChan <- status.Errorf(codes.Internal, "error sending stream: %v", err)
                return
            }
        }
        errChan <- nil
    }()

    // Wait for completion or cancellation
    select {
    case err := <-errChan:
        return err
    case <-stream.Context().Done():
        return status.Error(codes.Canceled, "client disconnected")
    }
}
```

**Key Points:**
- Use goroutines to handle concurrent send/receive operations
- Check for context cancellation in the goroutine before receiving
- Use `stream.Recv()` to receive messages
- Use `stream.Send()` to send responses
- Return appropriate gRPC status codes for errors
- Monitor `stream.Context().Done()` to handle client disconnections
- Use channels to coordinate between goroutines

## Generating gRPC Streaming Client Code

Generate the client code using:

```bash
gofr wrap grpc client -proto=./path/to/your/proto/file
```

This generates `<SERVICE_NAME>_client.go` with streaming client interfaces.

### Server-Side Streaming Client Usage

**Example Implementation:**

```go
func (c *ChatHandler) ServerStreamHandler(ctx *gofr.Context) (any, error) {
    // Initiate server stream
    stream, err := c.chatClient.ServerStream(ctx, &client.Request{
        Message: "stream request",
    })
    if err != nil {
        return nil, fmt.Errorf("failed to initiate server stream: %v", err)
    }

    var responses []Response
    
    // Receive all streamed responses
    for {
        res, err := stream.Recv()
        if err != nil {
            if errors.Is(err, io.EOF) {
                break // Stream completed
            }
            return nil, fmt.Errorf("stream receive error: %v", err)
        }
        
        responses = append(responses, res)
        ctx.Logger.Infof("Received: %s", res.Message)
    }

    return responses, nil
}
```

### Client-Side Streaming Client Usage

**Example Implementation:**

```go
func (c *ChatHandler) ClientStreamHandler(ctx *gofr.Context) (any, error) {
    // Initiate client stream
    stream, err := c.chatClient.ClientStream(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to initiate client stream: %v", err)
    }

    // Get messages from request body
    var requests []*client.Request
    if err := ctx.Bind(&requests); err != nil {
        return nil, fmt.Errorf("failed to bind requests: %v", err)
    }

    // Send multiple messages
    for _, req := range requests {
        if err := stream.Send(req); err != nil {
            return nil, fmt.Errorf("failed to send request: %v", err)
        }
    }

    // Close stream and receive final response
    response, err := stream.CloseAndRecv()
    if err != nil {
        return nil, fmt.Errorf("failed to receive final response: %v", err)
    }

    return response, nil
}
```

### Bidirectional Streaming Client Usage

**Example Implementation:**

```go
func (c *ChatHandler) BiDiStreamHandler(ctx *gofr.Context) (any, error) {
    // Initiate bidirectional stream
    stream, err := c.chatClient.BiDiStream(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to initiate bidirectional stream: %v", err)
    }

    respChan := make(chan Response)
    errChan := make(chan error)

    // Receive responses in a goroutine
    go func() {
        for {
            res, err := stream.Recv()
            if err != nil {
                if errors.Is(err, io.EOF) {
                    errChan <- nil
                } else {
                    errChan <- err
                }
                return
            }
            respChan <- res
        }
    }()

    // Send messages
    messages := []string{"message 1", "message 2", "message 3"}
    for _, msg := range messages {
        if err := stream.Send(&client.Request{Message: msg}); err != nil {
            return nil, fmt.Errorf("failed to send message: %v", err)
        }
    }

    // Close send side
    if err := stream.CloseSend(); err != nil {
        return nil, fmt.Errorf("failed to close send: %v", err)
    }

    // Collect responses
    var responses []Response
    for {
        select {
        case err := <-errChan:
            return responses, err
        case resp := <-respChan:
            responses = append(responses, resp)
        case <-time.After(5 * time.Second):
            return nil, errors.New("timeout waiting for responses")
        }
    }
}
```

## Registering Streaming Services

Register your streaming service in `main.go` just like unary services:

```go
package main

import (
    "gofr.dev/examples/grpc/grpc-streaming-server/server"
    "gofr.dev/pkg/gofr"
)

func main() {
    app := gofr.New()

    // Register streaming service
    server.RegisterChatServiceServerWithGofr(app, server.NewChatServiceGoFrServer())

    app.Run()
}
```

## Built-in Observability

GoFr automatically provides observability for all streaming operations:

### Metrics

The following metrics are automatically registered:
- **app_gRPC-Stream_stats**: Histogram tracking stream operation duration (Send, Recv, SendAndClose, CloseSend)
- **app_gRPC-Client-Stream_stats**: Histogram for client-side streaming operations

### Tracing

Each streaming operation (Send, Recv, SendAndClose, CloseSend) automatically creates spans for distributed tracing, allowing you to track the flow of messages through your system.

### Logging

Streaming operations are automatically logged with:
- Operation type (Send, Recv, etc.)
- Method name
- Duration
- Error status (if any)

## Error Handling

### Common Streaming Errors

1. **`io.EOF`**: Indicates the stream has ended normally
   - In client-side streaming: Server should call `SendAndClose()`
   - In server-side/bidirectional streaming: Client has finished sending

2. **Context Cancellation**: Stream was canceled or timed out
   - Check `stream.Context().Done()` for cancellation
   - Return appropriate gRPC status codes

3. **Network Errors**: Connection issues during streaming
   - Handle gracefully and return appropriate error status

**Example Error Handling:**

```go
func (s *ChatServiceGoFrServer) ServerStream(ctx *gofr.Context, stream ChatService_ServerStreamServer) error {
    req := Request{}
    if err := ctx.Bind(&req); err != nil {
        return status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
    }

    for i := 0; i < 5; i++ {
        // Check if context is canceled
        select {
        case <-stream.Context().Done():
            return status.Error(codes.Canceled, "client disconnected")
        default:
        }

        resp := &Response{Message: fmt.Sprintf("Message %d", i)}
        if err := stream.Send(resp); err != nil {
            return status.Errorf(codes.Internal, "error sending stream: %v", err)
        }
    }
    
    return nil
}
```

## Best Practices

1. **Always handle `io.EOF`**: This is the normal way streams end
2. **Monitor context cancellation**: Use `stream.Context().Done()` to detect client disconnections
3. **Use goroutines for bidirectional streams**: Allows concurrent send/receive operations
4. **Close streams properly**: Call `CloseSend()` when done sending in bidirectional streams
5. **Handle errors gracefully**: Return appropriate gRPC status codes
6. **Use timeouts**: Set reasonable timeouts for stream operations
7. **Log important events**: Use `ctx.Logger` to log stream lifecycle events

## Examples

Complete working examples are available in the GoFr repository:
- **Server Example**: `gofr/examples/grpc/grpc-streaming-server`
- **Client Example**: `gofr/examples/grpc/grpc-streaming-client`

These examples demonstrate all three types of streaming with detailed error handling and logging.

## Further Reading

- [gRPC with GoFr](https://gofr.dev/docs/advanced-guide/grpc) - General gRPC documentation
- [gRPC Official Documentation](https://grpc.io/docs/what-is-grpc/introduction/) - Learn more about gRPC streaming concepts
- [GoFr Examples](https://github.com/gofr-dev/gofr/tree/main/examples/grpc) - More gRPC examples

