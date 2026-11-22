---
description: "Build real-time WebSocket endpoints in GoFr. Full-duplex channels for chat, notifications, and live updates with customizable upgrader options included."
nextjs:
  metadata:
    title: "WebSockets in GoFr — Real-Time Bidirectional Channels"
    description: "Build real-time WebSocket endpoints in GoFr. Full-duplex channels for chat, notifications, and live updates with customizable upgrader options included."
---

# Websockets

WebSockets provide a full-duplex communication channel over a single, long-lived connection, making them ideal for 
real-time applications like chat, notifications, and live updates. GoFr provides a convenient way to integrate websockets
into your application. By leveraging GoFr's WebSocket support and customizable upgrader options,
users can efficiently manage real-time communication in your applications.

## Usage in GoFr

Here is a simple example to set up a WebSocket server in GoFr:

```go
package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	app.WebSocket("/ws", WSHandler)

	app.Run()
}

func WSHandler(ctx *gofr.Context) (any, error) {
	var message string

	err := ctx.Bind(&message)
	if err != nil {
		ctx.Logger.Errorf("Error binding message: %v", err)
		return nil, err
	}

	ctx.Logger.Infof("Received message: %s", message)

	return message, nil
}
```

## Configuration Options
GoFr allows us to customize the WebSocket upgrader with several options. We can set these options using the 
`websocket.NewWSUpgrader` function. Here is the list of options we can apply to your websocket upgrader using GoFr.

- `HandshakeTimeout (WithHandshakeTimeout)`: Sets the handshake timeout.
- `ReadBufferSize (WithReadBufferSize)`: Sets the size of the read buffer.
- `WriteBufferSize (WithWriteBufferSize)`: Sets the size of the write buffer.
- `Subprotocols (WithSubprotocols)`: Sets the supported sub-protocols.
- `Error (WithError)`:  Sets a custom error handler.
- `CheckOrigin (WithCheckOrigin)`: Sets a custom origin check function.
- `Compression (WithCompression)`:  Enables compression.

## Writing Messages

GoFr provides the `WriteMessageToSocket` method to send messages to the underlying websocket connection in a thread-safe way. The data parameter can be a string, []byte, or any struct that can be marshaled to JSON.

## Example:
We can configure the Upgrader by creating a chain of option functions provided by GoFr.

```go
package main

import (
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/websocket"
)

func main() {
	app := gofr.New()

	wsUpgrader := websocket.NewWSUpgrader(
		websocket.WithHandshakeTimeout(5*time.Second), // Set handshake timeout
		websocket.WithReadBufferSize(2048),            // Set read buffer size
		websocket.WithWriteBufferSize(2048),           // Set write buffer size
		websocket.WithSubprotocols("chat", "binary"),  // Specify subprotocols
		websocket.WithCompression(),                   // Enable compression
	)

	app.OverrideWebsocketUpgrader(wsUpgrader)

	app.WebSocket("/ws", WSHandler)

	app.Run()
}

func WSHandler(ctx *gofr.Context) (any, error) {
	var message string

	err := ctx.Bind(&message)
	if err != nil {
		ctx.Logger.Errorf("Error binding message: %v", err)
		return nil, err
	}

	ctx.Logger.Infof("Received message: %s", message)

	err = ctx.WriteMessageToSocket("Hello! GoFr")
	if err != nil {
		return nil, err
	}

	return message, nil
}
```
> #### Check out the example on how to read/write through a WebSocket in GoFr: [Visit GitHub](https://github.com/gofr-dev/gofr/blob/main/examples/using-web-socket/main.go)

## Connection Lifecycle

GoFr keeps one long-lived connection per client and calls your handler repeatedly, once per message. A handler
usually reads with `ctx.Bind`, which blocks until the client sends a message or the connection fails. The loop
stops in the following cases:

- **The client closes the connection**, with any close code, for example `ws.close()` in a browser (1005), 1008,
  1011 or an application code in the range 4000-4999.
- **A read fails**, for example the connection drops, the read limit is exceeded, or a read deadline expires. A
  failed read is permanent: every later read returns the same error, so a read deadline cannot be used to poll
  the connection and continue afterwards.
- **A write fails.**
- **A handler keeps failing without reading anything.** If the handler returns an error, or returns a value that
  cannot be serialized, and no message was read, the next pass would repeat it forever. GoFr retries once and
  then closes the connection.

A handler error is never sent to the client, and neither is a response that cannot be serialized: GoFr logs the
error and writes no frame. If the handler read a message before failing, the connection stays open and the handler
is called again for the next message. Otherwise, as described above, the connection is closed after one retry.

Since GoFr owns the read loop, read input through `ctx.Bind`. Reads made through the `websocket.Connection` that
your handler can obtain from the container (`ctx.Container.GetConnectionFromContext(ctx)`) are tracked as well.
Reads made directly on the underlying gorilla connection, such as `ReadJSON`, `NextReader` or
`c.Conn.ReadMessage()`, still work, but GoFr cannot see them, so a handler that only reads that way and returns an
error is treated as having read nothing and its connection is closed after the retry.

## Inter-Service WebSocket Communication

GoFr also supports Inter-Service WebSocket Communication, enabling seamless communication between services using WebSocket connections. 
This feature is particularly useful for microservices architectures where services need to exchange real-time data.

## Key Methods: 

1. **AddWSService**
This method registers a WebSocket service and establishes a persistent connection to the specified service. It also supports automatic reconnection in case of connection failures.

**Parameters:**


- `serviceName (string)`: A unique name for the WebSocket service.
- `url (string)`: The WebSocket URL of the target service.
- `headers ( map[string][]string)`: HTTP headers to include in the WebSocket handshake.
-  `enableReconnection (bool)`: A boolean to enable automatic reconnection.
- `retryInterval (time.Duration)`: The interval between reconnection attempts.

2. **WriteMessageToService**
This method sends a message to a WebSocket connection associated with a specific service.

**Parameters:**

- `serviceName (string)`: The name of the WebSocket service.
- `data (any)`: The message to send. It can be a string, []byte, or any struct that can be marshaled to JSON.

## Usage in GoFr

```go
package main

import (
	"time"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Add a WebSocket service
	err := app.AddWSService("notification-service", "ws://notifications.example.com/ws", nil, true, 5*time.Second)
	if err != nil {
		app.Logger().Errorf("Failed to add WebSocket service: %v", err)
		return
	}

	// Example route to send a message to the notification service
	app.POST("/send-notification", func(ctx *gofr.Context) (any, error) {
		message := map[string]string{
			"title":   "New Message",
			"content": "You have a new notification!",
		}

		err := ctx.WriteMessageToService("notification-service", message)
		if err != nil {
			ctx.Logger.Errorf("Failed to send message: %v", err)
			return nil, err
		}

		return "Notification sent successfully!", nil
	})

	app.Run()
}
```
