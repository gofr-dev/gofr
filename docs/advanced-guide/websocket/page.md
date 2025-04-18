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

	app.OverrideWebSocketUpgrader(wsUpgrader)

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
